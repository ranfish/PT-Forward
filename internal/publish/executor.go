// Package publish 新发布执行器（§59.156 切片 2，2026-08-31）。
//
// §59.147 配置中心消费流：发布链 DB 优先——切读 sites.publish_form_config，
// 替代 §59.146 灰度 flag（tagConfigFromFlag/settings tag_applier_sites 随本切片退役）。
// 复用组件（§59.148 保留清单）：renderDescription / dedupByPiecesHash /
// CheckPublishEligibility / adapter UploadTorrent / pusher 加种链 / ResourceResolver。
//
// 铁律对齐：零拉取（源 .torrent 走下载器 ExportTorrent 本地导出；元数据走
// torrent_metadata 落库数据 + PTGen 缓存兜底）；pre-audit=推数据拿判定（发布动作前置组成）。
package publish

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/ranfish/pt-forward/internal/httpclient"
	"github.com/ranfish/pt-forward/internal/metadata/extract"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/pusher"
	sitepkg "github.com/ranfish/pt-forward/internal/site"
	"github.com/ranfish/pt-forward/internal/titleparser"
	"github.com/ranfish/pt-forward/internal/util"
	"github.com/ranfish/pt-forward/internal/fingerprint"
)

// ExecuteInput 执行入参（API 层构造）。
type ExecuteInput struct {
	InfoHash   string // 簇内任一 hash（ResourceResolver 定位资源）
	TargetSite string
	Anonymous  bool
	// TagOverrides 人工勾选标签（standard_key 形态；追加到推断结果——auto:false 条目只能经此进入）
	TagOverrides []string
	// DryRun 只组装+预检不上传不加种（预检修正循环 / 冒烟验证）
	DryRun bool
	// PushOnly §59.159: 跳过上传/dedup 直接推送指定 TorrentID（补推——发布成功
	// 但推送环节失败/误跳过的修复通道，如 53511-53516 实战）
	PushOnly bool
	// TorrentID PushOnly 模式的目标站种子 ID
	TorrentID string
	// BatchGroupID 批次分组（批量端点生成 UUID——发布日志页按批次分组）
	BatchGroupID string
	// PushClientID/PushSavePath 补推直给（记录回放——发布记录落库值；缺省回落
	// ResolveResource 资源定位）
	PushClientID string
	PushSavePath string
}

// PreAuditDetail 预检明细（幸运官方结构 §59.150）。
type PreAuditDetail struct {
	RuleType string `json:"ruleType"`
	ErrorCode string `json:"errorCode"`
	Message  string `json:"message"`
	Level    string `json:"level"`
}

// PreAuditResult 预检结果。
type PreAuditResult struct {
	Passed     bool             `json:"passed"`
	TotalScore int              `json:"totalScore"`
	Details    []PreAuditDetail `json:"details"`
}

// ExecuteResult 执行结果（状态机：每个短路点一个 Status）。
type ExecuteResult struct {
	Status   string `json:"status"` // disabled/ ineligible/ duplicate/ existing/ uploaded/ uploaded_existing/ pushed/ pushed_existing/ failed/ dry_run_ok
	Message  string `json:"message"`
	PreAudit *PreAuditResult   `json:"pre_audit,omitempty"`
	Form     map[string]string `json:"form,omitempty"` // 组装产物（DryRun 供人工核对）
	Tags     []string          `json:"tags,omitempty"`
	Upload   *model.PublishResponse `json:"upload,omitempty"`
	TargetTorrentURL string     `json:"target_torrent_url,omitempty"`
}

// PublishExecutor 新发布执行器。
type PublishExecutor struct {
	db       *gorm.DB
	logger   *zap.Logger
	pipe     *Pipeline
	resolver *ResourceResolver
	inferer  *MediaTagInferer
}

// NewPublishExecutor 创建执行器（pipe 提供复用组件：siteProvider/clientProvider/pusher/dedup/render）。
func NewPublishExecutor(pipe *Pipeline) *PublishExecutor {
	ex := &PublishExecutor{
		db:       pipe.db,
		logger:   pipe.logger,
		pipe:     pipe,
		resolver: NewResourceResolver(pipe.db),
		inferer:  &MediaTagInferer{},
	}
	return ex
}

// Execute 单站发布主流程。
func (e *PublishExecutor) Execute(ctx context.Context, in ExecuteInput) *ExecuteResult {
	fail := func(status, msg string) *ExecuteResult {
		return &ExecuteResult{Status: status, Message: msg}
	}

	// ① 簇数据（ResourceResolver——数据权威层出口）
	rv := e.resolver.ResolveResource(ctx, in.InfoHash)
	if rv == nil || rv.Meta == nil {
		return fail("ineligible", "资源无元数据（未获取）")
	}
	meta := rv.Meta

	// §59.164: 落库版 fail（上传失败等终态此前不落库——修道院首例实战暴露：
	// 发布日志页看不到失败记录，用户误读为旧记录）。meta 就绪后的失败全部走此路径。
	failRec := func(status, msg string) *ExecuteResult {
		// DryRun 不落库（适配工具语义——失败记录污染发布日志；§59.164 回归审核补）
		if !in.DryRun {
			e.recordResultFull(ctx, in, meta, rv, status, msg, "", "", false)
		}
		return fail(status, msg)
	}

	// ② 目标站配置（C1 处方开关）
	var site model.Site
	if err := e.db.WithContext(ctx).Where("name = ?", in.TargetSite).First(&site).Error; err != nil {
		return failRec("disabled", "目标站不存在")
	}
	cfg := model.ParseFormConfig(site.PublishFormConfig)
	if cfg == nil || !cfg.Enabled {
		return failRec("disabled", "目标站未启用发布配置")
	}

	// §59.159 PushOnly 补推通道：跳过上传/dedup——推指定 TorrentID（SavePath 取资源目录）
	if in.PushOnly {
		if in.TorrentID == "" {
			return failRec("failed", "PushOnly 需 TorrentID")
		}
		if e.pipe.pusher == nil {
			return failRec("failed", "pusher 未注入")
		}
		clientID, savePath := in.PushClientID, in.PushSavePath
		if clientID == "" || savePath == "" {
			clientID, savePath = rv.ClientID, rv.SavePath // 回落资源定位
		}
		pushReq := &pusher.PushRequest{
			ClientID:  clientID,
			SiteName:  in.TargetSite,
			TorrentID: in.TorrentID, // pusher 内部 download.php?id=N 直下
			InfoHash:  in.InfoHash,
			Title:     meta.Title,
			SavePath:  savePath,
		}
		res := &ExecuteResult{Status: "uploaded"}
		if pr := e.pipe.pusher.Push(ctx, pushReq); pr != nil {
			switch {
			case pr.Success:
				res.Status = "pushed"
				var s2 model.Site
			if err := e.db.WithContext(ctx).Where("name = ?", in.TargetSite).First(&s2).Error; err == nil {
				res.TargetTorrentURL = strings.TrimRight(s2.BaseURL, "/") + "/details.php?id=" + in.TorrentID
			}
			case pr.AlreadyExist:
				res.Status = "pushed_existing"
				res.Message = "下载器已有该种子（已做种）"
			default:
				if pr.Error != nil {
					res.Status = "failed"
					res.Message = "加种失败: " + pr.Error.Error()
				}
			}
		}
		// §59.159 回归审核：补推成功回写原记录（同 tid 最新行 Status/Seeded——
		// 否则按钮反复可点[幂等无害但状态不收敛]、发布日志页 seeded 恒 0）
		if res.Status == "pushed" || res.Status == "pushed_existing" {
			// §59.166 实战修复（探针实证两段错位）：
			// ①原 Scan(&lastID) 标量 GORM 不支持恒 0 → 回写永不执行
			// ②改 Pluck 后取"最新行"——同 tid 多行（漏种 uploaded 行 + 二轮复发
			//   pushed 行）时永远命中新行，漏种行留痕不收敛
			// 终案：按条件批量收敛**所有 status=uploaded 的同 tid 行**（幂等——
			// 已 pushed 行不动），回写条数入日志。
			now := time.Now()
			res2 := e.db.WithContext(ctx).Model(&model.PublishResultRecord{}).
				Where("torrent_id = ? AND target_site = ? AND status = ?",
					in.TorrentID, in.TargetSite, "uploaded").
				Updates(map[string]interface{}{
					"status":     "pushed",
					"seeded":     true,
					"seeded_at":  now,
					"seed_error": "",
				})
			if res2.Error != nil {
				e.logger.Warn("repush 回写失败", zap.Error(res2.Error))
			} else if res2.RowsAffected > 0 {
				e.logger.Info("repush 回写收敛", zap.String("tid", in.TorrentID), zap.Int64("rows", res2.RowsAffected))
			}
		}
		return res
	}

	// ③ 源 .torrent 本地导出（零拉取）
	if e.pipe.clientProvider == nil {
		return failRec("failed", "clientProvider 未注入")
	}
	client, err := e.pipe.clientProvider.Get(rv.ClientID)
	if err != nil {
		return failRec("failed", fmt.Sprintf("获取下载器 %s 失败: %v", rv.ClientID, err))
	}
	torrentData, err := client.ExportTorrent(ctx, in.InfoHash)
	if err != nil || len(torrentData) == 0 {
		return failRec("failed", fmt.Sprintf("本地导出种子失败: %v", err))
	}
	// §59.159 源头嗅探：导出数据必须是 bencode（实战排查——.torrent 无效时
	// 站方静默回表单页，错误不可见）
	if torrentData[0] != 'd' && torrentData[0] != 'l' && torrentData[0] != 'i' {
		e.logger.Warn("exported torrent data is not bencode",
			zap.String("client", rv.ClientID), zap.Int("len", len(torrentData)),
			zap.Uint8("first_byte", torrentData[0]))
		return failRec("failed", fmt.Sprintf("导出数据非 bencode 种子（首字节 %q）——路径/内容异常", torrentData[0]))
	}

	// ④ 域值组装（DB 供给——value_mappings 反查）
	// §59.159 用户定案"发布=填满上传页所有内容"：表单键必须是**站方字段名**
	// （FormFields 映射）——此前用域常量做键（imdb_url=…），站方 input name 是 url → 投递丢失
	form := map[string]string{}
	setForm := func(domain, value string) {
		if field, ok := cfg.FormFields[domain]; ok && field != "" && value != "" {
			form[field] = value
		}
	}
	setForm(model.FieldDomainSmallDescr, meta.Subtitle)
	// §59.164: 修道院 cnname 独立中文名（FormFields 未配站点自然跳过）
	setForm(model.FieldDomainCNName, chineseTitleOf(meta.Title))
	setForm(model.FieldDomainDescription, meta.Description) // renderDescription 结果后续覆盖
	setForm(model.FieldDomainTechInfo, meta.MediaInfo)
	setForm(model.FieldDomainIMDBURL, meta.IMDbURL)
	// §59.159: PT-Gen/豆瓣链接（用户实战指认——发布页面完整投递；PTNexus 同款
	// pt_gen 值=豆瓣链接；FormFields 未配站点自然跳过）
	setForm(model.FieldDomainPTGen, meta.DoubanURL)
	setForm(model.FieldDomainDoubanURL, meta.DoubanURL)

	// §59.166 A 层同源化：表单四域（standard/codec/audiocodec/medium）改消费
	// BuildTechProfile（标题+MI+DOM 三源合并——种配页重组器同款，MI 纠错终态），
	// 替代 meta 原始列。五案根治：From S04(WEBRip 媒介跟标题)/宽幅 2K(高度
	// 1080 归一)/天空之城(audio)/Arco(MI 纠错 DDP)。type/team 域保持原源。
	domMedium, domRes, domVideo, domAudio := titleparser.DOMFieldsFromDetailSource(meta.DetailSourceJSON)
	tp := titleparser.BuildTechProfile(meta.Title, meta.MediaInfo, domMedium, domRes, domVideo, domAudio)
	// §59.166 B2：发布标题重组同源——ReassembleFromTechProfile(tp) 为权威（MI 纠错
	// 终态，Arco 案 DTS-HD MA→DDP 发布时自动纠对存量错标题）；空/异常回 meta.Title。
	publishTitle := meta.Title
	if rt := titleparser.ReassembleFromTechProfile(tp, titleparser.V105TitleFormat()); strings.TrimSpace(rt) != "" {
		publishTitle = rt
	}
	jobs := []domainJob{
		{model.FieldDomainType, e.lookupByStdKey(cfg, model.FieldDomainType, meta.Category)},
		{model.FieldDomainStandard, e.lookupByStdKey(cfg, model.FieldDomainStandard, extract.LookupStandardKey("resolution", tp.Resolution))},
		{model.FieldDomainCodec, e.lookupByStdKey(cfg, model.FieldDomainCodec, extract.LookupStandardKey("video_codec", tp.VideoCodec))},
		{model.FieldDomainAudiocodec, e.audioMappingOf(cfg, tp.AudioCodec, tp.AudioTechnology)},
		{model.FieldDomainMedium, e.mediumMappingOf(cfg, tp)},
		{model.FieldDomainTeam, e.teamMapping(cfg, meta)},
	}
	for _, j := range jobs {
		if j.match == nil {
			continue
		}
		if field, ok := cfg.FormFields[j.domain]; ok && field != "" {
			form[field] = j.match.Value
		}
	}

	// ⑤ tags（判据引擎 → form_config 反查 → auto:false 过滤 + 人工 overrides）
	tags := e.assembleTags(cfg, meta, in.TagOverrides)
	tagCfg := &model.SiteTagConfig{
		Mode:     model.TagModeTaglist,
		Tags:     map[string]string{},
		SpanField: cfg.FormFields[model.FieldDomainTags],
	}
	if cfg.TagConfig != nil && cfg.TagConfig.Mode != "" {
		tagCfg.Mode = cfg.TagConfig.Mode
	}
	// Tags map 从 ValueMappings 构造（双形态键——standard_key + canonical）
	for _, m := range cfg.ValueMappings[model.FieldDomainTags] {
		if m.Auto != nil && !*m.Auto {
			continue
		}
		// §59.164 转存发布禁勾"首发"（PT 圈通用铁律：圈首次发布=站管/压制组
		// 官方专属，转载人员不允许使用——用户 2026-09-02 权威定义，全站适用）
		if m.Label == "首发" {
			continue
		}
		for _, k := range m.StandardKeys {
			tagCfg.Tags[k] = m.Value
			if i := strings.Index(k, "."); i > 0 {
				tagCfg.Tags[k[i+1:]] = m.Value
			}
		}
	}
	applier := NewTagApplier(tagCfg)
	applied := []string{}
	for _, t := range tags {
		if applier.IsSupported(t) {
			if v, ok := tagCfg.Tags[t]; ok {
				applied = append(applied, v)
			} else {
				applied = append(applied, t) // taglist 模式直出标准键
			}
		}
	}

	// ⑥ 描述组装——纯本地资产消费（§59.159 白名单：替代 renderDescription
	// [queryPTGen 在线/rehostPoster 图床转存双违规]；零网络依赖）
	if strings.TrimSpace(meta.Title) == "" || strings.TrimSpace(meta.Description) == "" {
		return failRec("ineligible", "资产不完备：name/简介为空（先完成获取-审核）")
	}
	form[cfg.FormFields[model.FieldDomainDescription]] = assembleDescription(meta, cfg)

	// ⑦ pre-audit（§59.159 六轮定案：适配工具非发布门控——结果记录不阻断；
	// 历史：§59.150"passed 才提交"为单站验证期设计，多站架构下门控语义不成立）
	var preAudit *PreAuditResult
	if cfg.PreAuditURL != "" {
		if pa, errMsg := e.callPreAudit(ctx, &site, cfg, meta, form, jobs, applied); pa != nil {
			preAudit = pa
		} else {
			preAudit = &PreAuditResult{Passed: false, TotalScore: -1,
				Details: []PreAuditDetail{{Level: "WARN", Message: "预检请求失败: " + errMsg}}}
		}
	}

	// DryRun 检查点：组装+渲染+预检之后、上传之前（§59.156——完整表单形态验证，
	// 预检未通过不阻断 dry_run：预检修正循环数据源）
	if in.DryRun {
		res := &ExecuteResult{Status: "dry_run_ok", Form: form, Tags: applied, PreAudit: preAudit}
		if preAudit != nil && !preAudit.Passed {
			res.Message = "预检未通过（dry_run 不阻断）"
		}
		return res
	}

	// ⑧ 上传（adapter UploadTorrent 复用）
	// §59.159: 匿名发布取站点默认（form_config.Anonymous——站点配置勾选项）
	if cfg.Anonymous {
		in.Anonymous = true
	}
	pubReq := &model.PublishRequest{
		TorrentData: torrentData,
		FormFields:  form,
		Title:       publishTitle,
		Subtitle:    form[model.FieldDomainSmallDescr],
		Description: form[cfg.FormFields[model.FieldDomainDescription]],
		MediaInfo:   meta.MediaInfo,
		BDInfo:      meta.BDInfo,
		Screenshots: parseScreenshotsCol(meta.Screenshots),
		IMDbLink:    meta.IMDbURL,
		DoubanLink:  meta.DoubanURL,
		Anonymous:   in.Anonymous,
		SourceSite:  meta.SiteName,
		SourceInfoHash: in.InfoHash,
		TargetSite:  in.TargetSite,
		ClientID:    rv.ClientID,
	}
	// tags 写入表单（checkbox 数组=同名字段重复——§59.156 TagArrayFields 通道）
	applier.Apply(tags, func(field, value string) {
		pubReq.TagArrayFields = append(pubReq.TagArrayFields, model.TagKV{Key: field, Value: value})
	})

	adapter, aErr := e.pipe.siteProvider.GetAdapter(ctx, in.TargetSite)
	if aErr != nil {
		return failRec("failed", fmt.Sprintf("获取站点适配器失败: %v", aErr))
	}
	siteConfig, scErr := e.pipe.siteProvider.GetSiteConfig(ctx, in.TargetSite)
	if scErr != nil {
		return failRec("failed", fmt.Sprintf("获取站点配置失败: %v", scErr))
	}

	// dedup 双路（§59.166 本地记忆优先——修道院 18:49 批实战：站方 pieces-hash API
	// 密集查询间歇空返回致 4 种重传（站方复用 tid 无损但应防御）；本地零依赖零延迟，
	// 站方 API 作本地未命中补充。一种多站/一站多种发布链公共生效。
	piecesHash := fingerprintPiecesHash(torrentData)
	if piecesHash != "" {
		// ① 本地记忆（同站同 pieces_hash 已有终态记录→拦；tid 有值则复用展示）
		var rec model.PublishResultRecord
		if err := e.db.WithContext(ctx).
			Where("target_site = ? AND pieces_hash = ? AND status IN ?", in.TargetSite, piecesHash,
				[]string{"pushed", "pushed_existing", "uploaded", "duplicate"}).
			Order("id DESC").First(&rec).Error; err == nil {
			// 本地记忆 tid 补充：站方 API 查一次取真实 tid（失败不阻塞——记忆拦截已成立）
			tidShown := rec.TorrentID
			if tidShown == "" {
				if _, mHash, mTID := e.pipe.dedupByPiecesHashFull(ctx, adapter, siteConfig, torrentData); mTID > 0 {
					_ = mHash
					tidShown = fmt.Sprintf("%d", mTID)
				}
			}
			msg := "本地记忆命中（站上已有同内容种）: " + piecesHash
			e.recordResult(ctx, in, meta, rv, "duplicate", msg, tidShown, detailURLOf(siteConfig, tidShown), piecesHash)
			return fail("duplicate", msg)
		}
	}
	// ② 站方 pieces-hash API（拦截记录带站上 tid——此前 API 返回的 tid 被丢弃）
	if dup, dupMsg, mTID := e.pipe.dedupByPiecesHashFull(ctx, adapter, siteConfig, torrentData); dup {
		msg := "目标站已存在同内容种子: " + dupMsg
		tidShown := ""
		if mTID > 0 {
			tidShown = fmt.Sprintf("%d", mTID)
		}
		e.recordResult(ctx, in, meta, rv, "duplicate", msg, tidShown, detailURLOf(siteConfig, tidShown), piecesHash)
		return fail("duplicate", msg)
	}

	resp, upErr := adapter.UploadTorrent(ctx, siteConfig, pubReq)
	if upErr != nil {
		return failRec("failed", fmt.Sprintf("上传失败: %v", upErr))
	}
	if !resp.Success {
		// §59.159 回归审核：优先消费 ErrorMessage（已存在等语义化失败信息——
		// 旧代码拼 DetailURL[已存在形态为空]致"上传未成功: "空尾巴）
		if resp.ErrorMessage != "" {
			return failRec("failed", resp.ErrorMessage)
		}
		return failRec("failed", "上传未成功: "+resp.DetailURL)
	}

	// §59.159: 已存在分流（NP 302 existed=1 / stderr 文本——站上已有同种）。
	// 加种照做=把站上已有种下回做种（辅种语义），Status 语义化区分新发/已有。
	uploadedStatus := "uploaded"
	if resp.IsExisting {
		if resp.ExistingID == "" && resp.TorrentID == "" {
			// §59.159 existing 终态：文本"已存在"（信文案不信页面 ID——无权威 ID
			// 不推种；定位与推种走辅种业务）
			msg2 := "站上已有同种（未定位站内 ID，未推种——辅种业务范畴）"
			e.recordResult(ctx, in, meta, rv, "existing", msg2, "", "")
			return &ExecuteResult{
				Status: "existing", Message: msg2,
				TargetTorrentURL: resp.DetailURL,
			}
		}
		uploadedStatus = "uploaded_existing"
		if resp.ExistingID != "" && resp.TorrentID == "" {
			resp.TorrentID = resp.ExistingID
		}
	}

	// ⑨ 加种（pusher 复用——SavePath 资源目录）
	result := &ExecuteResult{
		Status: uploadedStatus,
		Upload: resp,
		TargetTorrentURL: resp.DetailURL,
	}
	if e.pipe.pusher != nil {
		pushReq := &pusher.PushRequest{
			ClientID:  rv.ClientID,
			SiteName:  in.TargetSite,
			TorrentID: resp.TorrentID,
			InfoHash:  in.InfoHash, // §59.159: 源种 infohash——已存在辅种快路径
			Title:     meta.Title,
			SavePath:  rv.SavePath,
		}
		if pr := e.pipe.pusher.Push(ctx, pushReq); pr != nil {
			switch {
			case pr.Success:
				if uploadedStatus == "uploaded_existing" {
					result.Status = "pushed_existing"
				} else {
					result.Status = "pushed"
				}
			case pr.AlreadyExist:
				// 下载器已有同种（人工先行导入/跨链重复）——辅种语义达成
				result.Status = "pushed_existing"
				result.Message = "下载器已有该种子（已做种）"
			default:
				if pr.Error != nil {
					result.Message = "加种失败: " + pr.Error.Error()
				}
			}
		}
	}

	// ⑩ 结果落库（发布日志页消费——recordResultFull 统一单点）
	seeded := result.Status == "pushed" || result.Status == "pushed_existing"
	e.recordResultFull(ctx, in, meta, rv, result.Status,
		joinNotes(paNoteOf(preAudit), result.Message),
		resp.TorrentID, result.TargetTorrentURL, seeded, piecesHash)
	return result
}

// paNoteOf 预检摘要（未过时——§59.160 非门控但记录）。
func paNoteOf(pa *PreAuditResult) string {
	if pa != nil && !pa.Passed {
		return fmt.Sprintf("预检 %d 分未过（已继续提交——非门控）", pa.TotalScore)
	}
	return ""
}

// recordResult 短路终态落库（duplicate/existing——§59.160 回归审核补：发布历史完整性）。
func (e *PublishExecutor) recordResult(ctx context.Context, in ExecuteInput, meta *model.TorrentMetadata,
	rv *ResourceView, status, msg, torrentID, url string, piecesHash ...string) {
	if len(piecesHash) > 0 {
		e.recordResultFull(ctx, in, meta, rv, status, msg, torrentID, url, false, piecesHash[0])
		return
	}
	e.recordResultFull(ctx, in, meta, rv, status, msg, torrentID, url, false)
}

// recordResultFull 统一落库单点（piecesHash §59.166 dedup 本地记忆——一种多站/
// 一站多种发布链公共生效：成功/拦截均落，下次发布 dedup 先查本地零依赖站方 API）。
func (e *PublishExecutor) recordResultFull(ctx context.Context, in ExecuteInput, meta *model.TorrentMetadata,
	rv *ResourceView, status, note, torrentID, url string, seeded bool, piecesHash ...string) {
	now := time.Now()
	seedErr := ""
	if strings.Contains(note, "加种失败") {
		seedErr = note
	}
	ph := ""
	if len(piecesHash) > 0 {
		ph = piecesHash[0]
	}
	_ = e.pipe.CreateResult(ctx, &model.PublishResultRecord{
		PiecesHash:     ph,
		TargetSite:     in.TargetSite,
		SourceSite:     meta.SiteName,
		SourceInfoHash: in.InfoHash,
		SavePath:       rv.SavePath,
		TorrentID:      torrentID,
		Status:         model.PublishResultStatus(status),
		SkipReason:     note,
		PublishURL:     url,
		Trigger:        "manual",
		BatchGroupID:   in.BatchGroupID,
		Title:          meta.Title,
		DownloaderID:   rv.ClientID,
		Seeded:         seeded,
		SeedError:      seedErr,
		SeededAt:       func() *time.Time { if seeded { return &now }; return nil }(),
		CompletedAt:    &now,
	})
}

// joinNotes 多段备注拼接（预检摘要+结果消息）。
func joinNotes(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	default:
		return a + "；" + b
	}
}

// domainJob 域装配任务（主流程与 pre-audit 共用）。
type domainJob struct {
	domain string
	match  *model.FormValueMapping
}

// lookupByStdKey standard_key → FormValueMapping 反查（StandardKeys 多键命中任一即中）。
func (e *PublishExecutor) lookupByStdKey(cfg *model.PublishFormConfig, domain, stdKey string) *model.FormValueMapping {
	if stdKey == "" {
		return nil
	}
	for i, m := range cfg.ValueMappings[domain] {
		for _, k := range m.StandardKeys {
			if k == stdKey {
				return &cfg.ValueMappings[domain][i]
			}
		}
	}
	return nil
}

// audioMappingOf §59.166 A 层：TechProfile 源音频映射（组合键优先 §59.150 判据六）。
func (e *PublishExecutor) audioMappingOf(cfg *model.PublishFormConfig, audioCodec, audioTech string) *model.FormValueMapping {
	// §59.166 B1：canonical→词条 standard_key 直取优先（"DD" 词表 exact 失明修复）
	if sk := titleparser.AudioStandardKey(audioCodec); sk != "" {
		if m := e.lookupByStdKey(cfg, model.FieldDomainAudiocodec, sk); m != nil {
			return m
		}
	}
	if audioCodec != "" && audioTech != "" {
		combined := extract.LookupStandardKey("audio_codec", audioCodec+" "+audioTech)
		if m := e.lookupByStdKey(cfg, model.FieldDomainAudiocodec, combined); m != nil {
			return m
		}
	}
	return e.lookupByStdKey(cfg, model.FieldDomainAudiocodec, extract.LookupStandardKey("audio_codec", audioCodec))
}

// audioMapping 音频域（组合键优先：TrueHD+Atmos→"TrueHD Atmos"——§59.150 判据六）。
func (e *PublishExecutor) audioMapping(cfg *model.PublishFormConfig, meta *model.TorrentMetadata) *model.FormValueMapping {
	if meta.AudioCodec != "" && meta.AudioTech != "" {
		combined := extract.LookupStandardKey("audio_codec", meta.AudioCodec+" "+meta.AudioTech)
		if m := e.lookupByStdKey(cfg, model.FieldDomainAudiocodec, combined); m != nil {
			return m
		}
	}
	return e.lookupByStdKey(cfg, model.FieldDomainAudiocodec, extract.LookupStandardKey("audio_codec", meta.AudioCodec))
}

// mediumMappingOf §59.166 A 层：TechProfile 源媒介映射（标题纠错终态——
// WEBRip/WEB-DL/Encode 规格优先 §59.150 二维规则）。
func (e *PublishExecutor) mediumMappingOf(cfg *model.PublishFormConfig, tp titleparser.TechProfile) *model.FormValueMapping {
	stdKey := ""
	switch strings.ToLower(tp.Specification) {
	case "remux":
		stdKey = "medium.remux"
	case "web-dl", "webdl":
		stdKey = "medium.webdl"
	case "hdtv":
		stdKey = "medium.hdtv"
	case "bdrip", "dvdrip", "tvrip", "encode", "webrip":
		// WEBRip=Encode（§59.166 From S04 案站方 TYPE_MISMATCH 对照——源站 WEB-DL
		// 标错、MI 痕迹证 WEBRip，重组已归 WEBRip，媒介跟 Encode 而非 WEB-DL）
		stdKey = "medium.encode"
	}
	if stdKey == "" {
		// §59.166 回归补：IsEncode 铁证（老 mediumMapping 原有——tp 同源化时丢失
		// 致 BluRay 1080p x265 压制误判 Blu-ray 原碟；titleparser.IsEncode 含
		// MI Writing library 铁证+碟源+编码族二维 §59.151）
		if titleparser.IsEncode(tp) {
			stdKey = "medium.encode"
		} else {
			stdKey = extract.LookupStandardKey("medium", tp.SourceType)
		}
	}
	return e.lookupByStdKey(cfg, model.FieldDomainMedium, stdKey)
}

// mediumMapping 媒介域（二维规则 §59.150：规格优先，Encode 判定 IsEncode 铁证 §59.151）。
func (e *PublishExecutor) mediumMapping(cfg *model.PublishFormConfig, meta *model.TorrentMetadata) *model.FormValueMapping {
	spec := strings.ToLower(meta.Specification)
	stdKey := ""
	switch spec {
	case "remux":
		stdKey = "medium.remux"
	case "web-dl", "webdl", "webrip":
		stdKey = "medium.webdl"
	case "hdtv":
		stdKey = "medium.hdtv"
	case "bdrip", "dvdrip", "tvrip", "encode":
		stdKey = "medium.encode"
	}
	if stdKey == "" {
		// Encode 二维：碟转压（片源碟+编码痕迹）——IsEncode 铁证
		if titleparser.IsEncode(titleparser.TechProfile{
			SourceType:    meta.SourceType,
			Specification: meta.Specification,
			VideoCodec:    meta.VideoCodec,
		}) {
			stdKey = "medium.encode"
		} else {
			stdKey = extract.LookupStandardKey("medium", meta.SourceType)
		}
	}
	return e.lookupByStdKey(cfg, model.FieldDomainMedium, stdKey)
}

// teamMapping 组域（R3-6 待做——label 直配制作组名）。
func (e *PublishExecutor) teamMapping(cfg *model.PublishFormConfig, meta *model.TorrentMetadata) *model.FormValueMapping {
	group := util.ExtractGroupName(meta.Title)
	if group == "" {
		return nil
	}
	for i, m := range cfg.ValueMappings[model.FieldDomainTeam] {
		if strings.EqualFold(m.Label, group) {
			return &cfg.ValueMappings[model.FieldDomainTeam][i]
		}
	}
	return nil
}

// assembleTags 标签装配：判据引擎推断 → form_config 值映射 → auto:false 过滤 + 人工 overrides。
func (e *PublishExecutor) assembleTags(cfg *model.PublishFormConfig, meta *model.TorrentMetadata, overrides []string) []string {
	// 判据引擎（§59.151 MI 唯一真相）
	inferred := e.inferer.Infer(meta.MediaInfo, meta.Title, meta.Subtitle)
	// 站方 tags 域 standard_key → value 映射 + auto:false 排除
	allowed := map[string]string{}
	for _, m := range cfg.ValueMappings[model.FieldDomainTags] {
		if m.Auto != nil && !*m.Auto {
			continue // 条件标签（英语/首发/原创/禁转）——只能经 overrides 进入
		}
		for _, k := range m.StandardKeys {
			allowed[k] = m.Value
			// canonical 双形态（Infer 产出无前缀 "dolby_vision"；standard_key
			// 形态 "tag.dolby_vision"——两者同值）
			if i := strings.Index(k, "."); i > 0 {
				allowed[k[i+1:]] = m.Value
			}
		}
	}
	out := []string{}
	seen := map[string]bool{}
	for _, t := range inferred {
		if v, ok := allowed[t]; ok && !seen[t] {
			out = append(out, t)
			seen[t] = true
			_ = v
		}
	}
	for _, t := range overrides {
		if !seen[t] {
			out = append(out, t)
			seen[t] = true
		}
	}
	return out
}

// callPreAudit 幸运官方预检（§59.150 请求结构——dryrun_luck.py 实调验证 166/166）。
func (e *PublishExecutor) callPreAudit(ctx context.Context, site *model.Site, cfg *model.PublishFormConfig,
	meta *model.TorrentMetadata, form map[string]string, jobs []domainJob, appliedTagValues []string) (*PreAuditResult, string) {

	type idName struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	type auditBody struct {
		Name          string   `json:"name"`
		SmallDescr    string   `json:"small_descr"`
		IMDBURL       string   `json:"imdb_url"`
		Description   string   `json:"description"`
		TechnicalInfo string   `json:"technical_info"`
		Type          *idNameWithMode `json:"type,omitempty"`
		Quality       map[string]idName `json:"quality,omitempty"`
		Tags          []idName `json:"tags"`
		UserInfo      map[string]any `json:"user_info,omitempty"`
		ExportTime    string   `json:"export_time,omitempty"`
		PageURL       string   `json:"page_url,omitempty"`
	}
	body := auditBody{
		Name:          meta.Title,
		SmallDescr:    form[model.FieldDomainSmallDescr],
		IMDBURL:       meta.IMDbURL,
		Description:   form[cfg.FormFields[model.FieldDomainDescription]],
		TechnicalInfo: meta.MediaInfo,
		Quality:       map[string]idName{},
		Tags:          []idName{},
		ExportTime:    time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
		PageURL:       strings.TrimRight(site.BaseURL, "/") + "/upload.php",
	}
	for _, j := range jobs {
		if j.match == nil {
			continue
		}
		body.Quality[j.domain] = idName{ID: j.match.Value, Name: j.match.Label}
	}
	// type 域独立结构：mode 从 data-mode 字段名后缀提取（medium_sel[4] → "4"）
	if t, ok := body.Quality[model.FieldDomainType]; ok && t.ID != "" {
		mode := ""
		if mf := cfg.FormFields[model.FieldDomainMedium]; mf != "" {
			if i := strings.Index(mf, "["); i >= 0 && strings.HasSuffix(mf, "]") {
				mode = mf[i+1 : len(mf)-1]
			}
		}
		body.Type = &idNameWithMode{ID: t.ID, Name: t.Name, Mode: mode}
		delete(body.Quality, model.FieldDomainType)
	}
	// tags：值反查 label
	for _, v := range appliedTagValues {
		for _, m := range cfg.ValueMappings[model.FieldDomainTags] {
			if m.Value == v {
				body.Tags = append(body.Tags, idName{ID: m.Value, Name: m.Label})
			}
		}
	}

	if site.UserID > 0 || site.Username != "" {
		body.UserInfo = map[string]any{
			"id": site.UserID, "username": site.Username,
		}
	}
	siteConfig, err := e.pipe.siteProvider.GetSiteConfig(ctx, site.Name)
	if err != nil {
		return nil, err.Error()
	}
	url := strings.TrimRight(site.BaseURL, "/") + cfg.PreAuditURL
	payload, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Cookie", siteConfig.Cookie)
	// §59.156: 站点代理解析公共单点（site.ResolveSiteProxy——与 adapter 同源；
	// "尊重站点配置而非强制代理"）
	proxyURL := sitepkg.ResolveSiteProxy(e.db, ctx, site)
	httpClient := httpclient.NewSiteHTTPClient(httpclient.SiteHTTPConfig{
		ProxyURL:      proxyURL,
		SkipSSLVerify: site.SkipSSLVerify,
		Timeout:       40 * time.Second,
	})
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err.Error()
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	var envelope struct {
		Data *PreAuditResult `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || envelope.Data == nil {
		return nil, fmt.Sprintf("响应解析失败(%d): %.200s", resp.StatusCode, string(raw))
	}
	return envelope.Data, ""
}

// idNameWithMode type 域带 mode（幸运 data-mode='4'）。
type idNameWithMode struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Mode string `json:"mode,omitempty"`
}

// parseScreenshotsCol screenshots 列解析（§59.47 四消费点同款）。
// §59.159 修复：列存储形态为 JSON 数组（["url1","url2"]）——FieldsFunc 硬拆
// 会保留引号 → [img]"url"[/img] 坏格式 → LuckAudit 审核拒绝（实战 86 分 1 错误）。
// JSON 解析优先，剥引号兜底。
func parseScreenshotsCol(s string) []string {
	if s == "" {
		return nil
	}
	var urls []string
	if err := json.Unmarshal([]byte(s), &urls); err == nil {
		return urls
	}
	// fallback：逗号/换行分隔 + 剥引号
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == '\n' || r == '\r' || r == ' ' })
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.Trim(p, "\"")
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// chineseTitleOf 提取标题中文段（§59.164 修道院 cnname——"阴风阵阵.Suspiria.2018"→"阴风阵阵"）。
// 取最长连续中文段（含·间隔与内部空格）；无中文返回空（字段不投递）。
var chineseTitleRe = regexp.MustCompile(`\p{Han}+(?:[\s·・]\p{Han}+)*`)

func chineseTitleOf(title string) string {
	best := ""
	for _, seg := range chineseTitleRe.FindAllString(title, -1) {
		seg = strings.TrimSpace(seg)
		if len([]rune(seg)) > len([]rune(best)) {
			best = seg
		}
	}
	return best
}

// fingerprintPiecesHash 种子数据 pieces hash（dedup 本地记忆键——空数据/解析失败返回空）。
func fingerprintPiecesHash(torrentData []byte) string {
	if len(torrentData) == 0 {
		return ""
	}
	if m, err := fingerprint.ComputeFromTorrent(torrentData); err == nil && m != nil {
		return m.PiecesHash
	}
	return ""
}

// detailURLOf 站内种详情 URL（拦截记录直达链接——tid 空返回空）。
func detailURLOf(cfg *model.SiteConfig, tid string) string {
	if tid == "" {
		return ""
	}
	return strings.TrimRight(cfg.Domain, "/") + "/details.php?id=" + tid
}
