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

	// ② 目标站配置（C1 处方开关）
	var site model.Site
	if err := e.db.WithContext(ctx).Where("name = ?", in.TargetSite).First(&site).Error; err != nil {
		return fail("disabled", "目标站不存在")
	}
	cfg := model.ParseFormConfig(site.PublishFormConfig)
	if cfg == nil || !cfg.Enabled {
		return fail("disabled", "目标站未启用发布配置")
	}

	// §59.159 PushOnly 补推通道：跳过上传/dedup——推指定 TorrentID（SavePath 取资源目录）
	if in.PushOnly {
		if in.TorrentID == "" {
			return fail("failed", "PushOnly 需 TorrentID")
		}
		if e.pipe.pusher == nil {
			return fail("failed", "pusher 未注入")
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
			now := time.Now()
			// GORM Updates 不支持 Order/Limit——子查询定位最新行主键再更新
			var lastID int64
			e.db.WithContext(ctx).Model(&model.PublishResultRecord{}).
				Where("torrent_id = ? AND target_site = ?", in.TorrentID, in.TargetSite).
				Select("id").Order("id DESC").Limit(1).Scan(&lastID)
			if lastID > 0 {
				e.db.WithContext(ctx).Model(&model.PublishResultRecord{}).
					Where("id = ?", lastID).
					Updates(map[string]interface{}{
						"status":     "pushed",
						"seeded":     true,
						"seeded_at":  now,
						"seed_error": "",
					})
			}
		}
		return res
	}

	// ③ 源 .torrent 本地导出（零拉取）
	if e.pipe.clientProvider == nil {
		return fail("failed", "clientProvider 未注入")
	}
	client, err := e.pipe.clientProvider.Get(rv.ClientID)
	if err != nil {
		return fail("failed", fmt.Sprintf("获取下载器 %s 失败: %v", rv.ClientID, err))
	}
	torrentData, err := client.ExportTorrent(ctx, in.InfoHash)
	if err != nil || len(torrentData) == 0 {
		return fail("failed", fmt.Sprintf("本地导出种子失败: %v", err))
	}
	// §59.159 源头嗅探：导出数据必须是 bencode（实战排查——.torrent 无效时
	// 站方静默回表单页，错误不可见）
	if torrentData[0] != 'd' && torrentData[0] != 'l' && torrentData[0] != 'i' {
		e.logger.Warn("exported torrent data is not bencode",
			zap.String("client", rv.ClientID), zap.Int("len", len(torrentData)),
			zap.Uint8("first_byte", torrentData[0]))
		return fail("failed", fmt.Sprintf("导出数据非 bencode 种子（首字节 %q）——路径/内容异常", torrentData[0]))
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
	setForm(model.FieldDomainDescription, meta.Description) // renderDescription 结果后续覆盖
	setForm(model.FieldDomainTechInfo, meta.MediaInfo)
	setForm(model.FieldDomainIMDBURL, meta.IMDbURL)
	// §59.159: PT-Gen/豆瓣链接（用户实战指认——发布页面完整投递；PTNexus 同款
	// pt_gen 值=豆瓣链接；FormFields 未配站点自然跳过）
	setForm(model.FieldDomainPTGen, meta.DoubanURL)
	setForm(model.FieldDomainDoubanURL, meta.DoubanURL)

	jobs := []domainJob{
		{model.FieldDomainType, e.lookupByStdKey(cfg, model.FieldDomainType, meta.Category)},
		{model.FieldDomainStandard, e.lookupByStdKey(cfg, model.FieldDomainStandard, extract.LookupStandardKey("resolution", meta.Resolution))},
		{model.FieldDomainCodec, e.lookupByStdKey(cfg, model.FieldDomainCodec, extract.LookupStandardKey("video_codec", meta.VideoCodec))},
		{model.FieldDomainAudiocodec, e.audioMapping(cfg, meta)},
		{model.FieldDomainMedium, e.mediumMapping(cfg, meta)},
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
		return fail("ineligible", "资产不完备：name/简介为空（先完成获取-审核）")
	}
	form[cfg.FormFields[model.FieldDomainDescription]] = assembleDescription(meta)

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
		Title:       meta.Title,
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
		return fail("failed", fmt.Sprintf("获取站点适配器失败: %v", aErr))
	}
	siteConfig, scErr := e.pipe.siteProvider.GetSiteConfig(ctx, in.TargetSite)
	if scErr != nil {
		return fail("failed", fmt.Sprintf("获取站点配置失败: %v", scErr))
	}

	// dedup（复用组件——pieces_hash 目标站查重）
	if dup, dupMsg := e.pipe.dedupByPiecesHash(ctx, adapter, siteConfig, torrentData); dup {
		return fail("duplicate", "目标站已存在同内容种子: "+dupMsg)
	}

	resp, upErr := adapter.UploadTorrent(ctx, siteConfig, pubReq)
	if upErr != nil {
		return fail("failed", fmt.Sprintf("上传失败: %v", upErr))
	}
	if !resp.Success {
		// §59.159 回归审核：优先消费 ErrorMessage（已存在等语义化失败信息——
		// 旧代码拼 DetailURL[已存在形态为空]致"上传未成功: "空尾巴）
		if resp.ErrorMessage != "" {
			return fail("failed", resp.ErrorMessage)
		}
		return fail("failed", "上传未成功: "+resp.DetailURL)
	}

	// §59.159: 已存在分流（NP 302 existed=1 / stderr 文本——站上已有同种）。
	// 加种照做=把站上已有种下回做种（辅种语义），Status 语义化区分新发/已有。
	uploadedStatus := "uploaded"
	if resp.IsExisting {
		if resp.ExistingID == "" && resp.TorrentID == "" {
			// §59.159 existing 终态：文本"已存在"（信文案不信页面 ID——无权威 ID
			// 不推种；定位与推种走辅种业务）
			return &ExecuteResult{
				Status: "existing", Message: "站上已有同种（未定位站内 ID，未推种——辅种业务范畴）",
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

	// ⑩ 结果落库（发布日志页消费）
	now := time.Now()
	paNote := ""
	if preAudit != nil && !preAudit.Passed {
		paNote = fmt.Sprintf("预检 %d 分未过（已继续提交——§59.159 非门控）", preAudit.TotalScore)
	}
	_ = e.pipe.CreateResult(ctx, &model.PublishResultRecord{
		TargetSite:   in.TargetSite,
		SourceSite:   meta.SiteName,
		SourceInfoHash: in.InfoHash,
		SavePath:      rv.SavePath,
		TorrentID:    resp.TorrentID,
		Status:       model.PublishResultStatus(result.Status),
		SkipReason:   joinNotes(paNote, result.Message),
		PublishURL:   result.TargetTorrentURL,
		Trigger:      "manual",
		BatchGroupID: in.BatchGroupID,
		Title:        meta.Title,
		DownloaderID: rv.ClientID,
		Seeded:       result.Status == "pushed" || result.Status == "pushed_existing", // §59.159: 辅种加种同为加种
		SeedError:    func() string { if strings.Contains(result.Message, "加种失败") { return result.Message }; return "" }(),
		SeededAt:     func() *time.Time { if result.Status == "pushed" { return &now }; return nil }(),
		CompletedAt:  &now,
	})
	return result
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
	inferred := e.inferer.Infer(meta.MediaInfo, meta.Title)
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
