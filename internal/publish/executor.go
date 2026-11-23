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
	"strconv"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/metadata"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/ranfish/pt-forward/internal/httpclient"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/pusher"
	sitepkg "github.com/ranfish/pt-forward/internal/site"
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
	PushClientID uint
	PushSavePath string
}

// PreAuditDetail 预检明细（幸运官方结构 §59.150）。
type PreAuditDetail struct {
	RuleType  string `json:"ruleType"`
	ErrorCode string `json:"errorCode"`
	Message   string `json:"message"`
	Level     string `json:"level"`
}

// PreAuditResult 预检结果。
type PreAuditResult struct {
	Passed     bool             `json:"passed"`
	TotalScore int              `json:"totalScore"`
	Details    []PreAuditDetail `json:"details"`
}

// ExecuteResult 执行结果（状态机：每个短路点一个 Status）。
type ExecuteResult struct {
	Status   string            `json:"status"` // disabled/ ineligible/ duplicate/ existing/ uploaded/ uploaded_existing/ pushed/ pushed_existing/ failed/ dry_run_ok
	Message  string            `json:"message"`
	PreAudit *PreAuditResult   `json:"pre_audit,omitempty"`
	Form     map[string]string `json:"form,omitempty"` // 组装产物（DryRun 供人工核对）
	// §59.310: 字段名→友好展示（"域名 · 值label"）——audiocodec_sel[4]=12 →
	// "音频编码 · DTS"（form 键是站方字段名+值是站方枚举值，用户不可读）
	FormLabels map[string]string `json:"form_labels,omitempty"`
	// §59.311: 标签资产（canonical 键——Tab1 ③ 同源 meta.Tags；区别于 tags 字段
	// 的站方枚举值形态。用户语义定案：弹窗=系统计算资产+按站投递双通道展示）
	TagAssets []string `json:"tag_assets,omitempty"`
	// §59.312: 标签投递可读化——tags 枚举值 → 站方 label（与 tags 索引对齐；
	// 投递 6/10 → 完结/英语——预检人工查看语义）
	TagsLabels       []string               `json:"tags_labels,omitempty"`
	Tags             []string               `json:"tags,omitempty"`
	Upload           *model.PublishResponse `json:"upload,omitempty"`
	LocalAudit       []LocalAuditFinding    `json:"local_audit,omitempty"` // §59.166 内部规范提示（advisory）
	TargetTorrentURL string                 `json:"target_torrent_url,omitempty"`
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
// preflightNoTransfer §59.409 附二: 发布行为前置检查——禁转/互斥/排除
// 家族单点（用户语义定案两层：获取链打标与目标无关；发布链拦"转载/
// 发布行为本身"与目标相关）。原挂 Pipeline.CheckPublishEligibility——
// §59.147 拆自动发布流后零生产调用（死代码），全家族从未在发布页链路
// 生效；用户"一站多种官种混选是否被阻塞"问出系统性缺口，本次搬家激活。
// 粒度=单颗×单目标（批量逐颗独立 Execute 不连坐——官种颗拦名单站，
// 同批非官种颗/官种颗发其它站均正常）：
//
//	① 种子禁转标记（获取链落库 flags）：绝对禁转族拦全部目标站；限时
//	   禁转窗内拦全部、窗外放行；深度站限转名单站永久拦+窗内全站拦
//	   （checkFlagsFromMetadata 单点）
//	② 定向禁转名单（优堡官种→PTTime 等——checkOfficialDirectedNoTransfer 单点）
//	③ 互斥转载站点对（HardcodedExclusionPairs）
//	④ 用户自定义排除规则（双保险——candidate 创建层已查，单发/重放路径）
func (e *PublishExecutor) preflightNoTransfer(ctx context.Context, meta *model.TorrentMetadata, targetSite string) (bool, string) {
	if targetSite == "" || meta == nil {
		return false, ""
	}
	sourceSite := meta.SiteName
	if sourceSite == "" {
		return false, ""
	}
	if blocked, reason := e.pipe.checkFlagsFromMetadata(ctx, meta.InfoHash, sourceSite, targetSite); blocked {
		return true, reason
	}
	if blocked, reason := e.pipe.checkOfficialDirectedNoTransfer(ctx, sourceSite, meta.Title, targetSite); blocked {
		return true, reason
	}
	if IsHardcodedExclusion(sourceSite, targetSite) {
		return true, fmt.Sprintf("源站 %s → 目标站 %s 为互斥站点（禁止互相转载）", sourceSite, targetSite)
	}
	// §59.539: 硬编码单向互斥（深度→{时间,肉丝,家园}·站规定向禁转
	// 第二重防御·用户定案）。
	if IsHardcodedOneWayExclusion(sourceSite, targetSite) {
		return true, fmt.Sprintf("源站 %s → 目标站 %s 为单向禁转站点（站规定向名单）", sourceSite, targetSite)
	}
	var exclusion model.PublishExclusion
	if err := e.db.WithContext(ctx).
		Where("target_site = ? AND source_site = ?", targetSite, sourceSite).
		First(&exclusion).Error; err == nil {
		return true, fmt.Sprintf("源站 %s → 目标站 %s 存在发布排除规则", sourceSite, targetSite)
	}
	// ⑤ compliance 系统禁转（§59.143"红标不隐藏+提交兜底"设计的兜底层——
	// 原挂死代码 Eligibility 从未生效，§59.409 附三同族补全）。nil 跳过
	// （列表层先例——装配点 main 无条件注入，nil 仅测试构造）。判定输入=
	// meta.Title 站方标题（发布消费权威——与列表用下载器名相比更严一格）。
	if e.pipe.complianceChecker != nil {
		// §59.489: 音乐种分流（音乐不存在成人内容·用户定案——School Rumble
		// 案 [YYYY.MM.DD] 日期前缀误判 bracket_date）。禁转词/组保留。
		if r := e.pipe.complianceChecker.CheckWithSite(ctx, meta.Title, sourceSite, metadata.HasMusic(meta.DetailSourceJSON)); r != nil && !r.Passed {
			return true, fmt.Sprintf("系统禁转（compliance %s）：%s", r.Category, r.Reason)
		}
	}
	// ⑥ §59.511 官组种禁回流源站（全站通用——组→mappings(is_official=1)
	// 归属站==目标站→拦截·251 组自动生效。与②定向禁转正交：②=官种→
	// 特定站名单驱动·⑥=组→归属站数据驱动）
	if blocked, reason := e.checkOfficialGroupBackflow(ctx, meta, targetSite); blocked {
		return true, reason
	}
	return false, ""
}

// exportClusterFallback §59.519: 导出失败时按资源键（rv.Hashes=同 client+
// path+name 的全部活跃 hash）逐副本尝试导出——毒副本（Flight/BoyMeetsGirl 案：
// TR 4.0.5 Malformed benc·.torrent 不落盘且 metainfo rpc 空）由簇内健康
// 副本兜底。首字节 'd' 校验与主路径同源（bencode 嗅探）。返回 nil=全败。
func (e *PublishExecutor) exportClusterFallback(ctx context.Context, client model.DownloaderClient, rv *ResourceView, failedHash string) ([]byte, string) {
	for _, alt := range rv.Hashes {
		if alt == failedHash {
			continue
		}
		data, err := client.ExportTorrent(ctx, alt)
		if err != nil || len(data) == 0 || data[0] != 'd' {
			continue
		}
		return data, alt
	}
	return nil, ""
}

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

	// §59.409 附二: 发布行为前置检查（禁转/互斥/排除——拦"转载/发布
	// 行为本身"）。DryRun 同样生效（适配预览可见拦截）；PushOnly 补推
	// 通道同拦（推送也属转载行为）。
	if blocked, reason := e.preflightNoTransfer(ctx, meta, in.TargetSite); blocked {
		return failRec("ineligible", reason)
	}

	// §59.558 PR-5: PushOnly 补推通道抽为 executePushOnly（executor_stages.go）
	if in.PushOnly {
		if res, fs, fm := e.executePushOnly(ctx, in, meta, rv); fs != "" {
			return failRec(fs, fm)
		} else if res != nil {
			return res
		}
	}

	// ③ 源 .torrent 本地导出（零拉取）
	if e.pipe.clientProvider == nil {
		return failRec("failed", "clientProvider 未注入")
	}
	client, err := e.pipe.clientProvider.Get(rv.ClientUID)
	if err != nil {
		return failRec("failed", fmt.Sprintf("获取下载器 %d 失败: %v", rv.ClientUID, err))
	}
	torrentData, err := client.ExportTorrent(ctx, in.InfoHash)
	if err != nil || len(torrentData) == 0 {
		// §59.519 簇内健康副本 fallback：毒副本（TR 4.0.5 Malformed benc——
		// .torrent 不落盘+metainfo 空）导出失败时·用同资源键（client+path+
		// name=文件树同构）其它副本导出。与 pusher「using actual hash」
		// 同族语义（推送链早已按本地实际 hash 推·导出链对齐）。
		alt, altHash := e.exportClusterFallback(ctx, client, rv, in.InfoHash)
		if alt != nil {
			torrentData = alt
			if e.logger != nil {
				e.logger.Info("export fallback: cluster sibling used",
					zap.String("hash", in.InfoHash), zap.String("alt_hash", altHash),
					zap.Uint("client", rv.ClientUID), zap.String("site", in.TargetSite))
			}
		} else {
			return failRec("failed", fmt.Sprintf(
				"本地导出种子失败: %v（簇内共 %d 个副本均不可导出——TR metainfo 异常见 TR 日志）", err, len(rv.Hashes)))
		}
	}
	// §59.159 源头嗅探：导出数据必须是 bencode（实战排查——.torrent 无效时
	// 站方静默回表单页，错误不可见）
	if torrentData[0] != 'd' && torrentData[0] != 'l' && torrentData[0] != 'i' {
		e.logger.Warn("exported torrent data is not bencode",
			zap.Uint("client", rv.ClientUID), zap.Int("len", len(torrentData)),
			zap.Uint8("first_byte", torrentData[0]))
		return failRec("failed", fmt.Sprintf("导出数据非 bencode 种子（首字节 %q）——路径/内容异常", torrentData[0]))
	}

	// §59.485 刀2: 音乐种独立发布分支——表单语义/标题组装与影视链完全
	// 不同（type=408 恒定/name 四段式/无 MI 无截图·影视标题重组对音乐
	// 必挂"MI 缺失"）。提前分流·影视链零触碰。目标站收口：音乐种当前
	// 仅乌托邦（§59.469 附九前端门的后端单点）。
	if mm := musicMetaOfMeta(meta); mm != nil {
		if in.TargetSite != "乌托邦" {
			return failRec("ineligible", "音乐种目标站未适配（当前仅乌托邦）——音乐发布链 §59.485")
		}
		return e.executeMusicPublish(ctx, in, meta, rv, cfg, torrentData, mm)
	}

	// §59.558 PR-5: 表单段抽为 buildExecuteForm（executor_stages.go）
	fb, fs, fm, useRec := e.buildExecuteForm(ctx, in, meta, rv, &site, cfg, torrentData)
	if fb == nil {
		if useRec {
			return failRec(fs, fm)
		}
		return fail(fs, fm)
	}
	form, tp, publishTitle := fb.form, fb.tp, fb.publishTitle
	arbNotes, jobs := fb.arbNotes, fb.jobs
	tags, tagArrayFields, applied := fb.tags, fb.tagArrayFields, fb.applied
	gpwExtra, gpwArtists, gpwSubs := fb.gpwExtra, fb.gpwArtists, fb.gpwSubs

	// ⑦ pre-audit（§59.159 六轮定案：适配工具非发布门控——结果记录不阻断；
	// 历史：§59.150"passed 才提交"为单站验证期设计，多站架构下门控语义不成立）
	var preAudit *PreAuditResult
	if cfg.PreAuditURL != "" {
		if pa, errMsg := e.callPreAudit(ctx, &site, cfg, meta, form, jobs, tagArrayFields, publishTitle); pa != nil {
			preAudit = pa
		} else {
			preAudit = &PreAuditResult{Passed: false, TotalScore: -1,
				Details: []PreAuditDetail{{Level: "WARN", Message: "预检请求失败: " + errMsg}}}
		}
	}

	// §59.166 LocalAudit 内部重组审核规范（advisory 全 WARN 不阻塞——规则未实战
	// 验证前不拦发布，用户定案；提示走 DryRun/成功 message/logs 三出口）
	// §59.504: 删 PublishTitle/AppliedTags 传参（消费者随规则删减归零）
	localFindings := RunLocalAudit(localAuditInput{
		TargetSite:   in.TargetSite,
		TP:           tp,
		Form:         form,
		Cfg:          cfg,
		TagOverrides: in.TagOverrides,
		Meta:         meta,
	})

	// §59.558 PR-5: DryRun 检查点抽为 dryRunCheckpoint（executor_stages.go）
	if in.DryRun {
		return e.dryRunCheckpoint(ctx, in, meta, torrentData, cfg, form, preAudit, localFindings, applied, arbNotes)
	}

	// ⑧ 上传（adapter UploadTorrent 复用）
	// §59.273: 审核门——未审核种子禁止直接发布（一种多站/一站多种/单发全
	// 路径收口在 executor 单点；DryRun 预览不受限。249 案：清除→重获后
	// reviewed 重置为 false，页面显示未审核但发布无门可拦）
	if !meta.Reviewed {
		return failRec("unreviewed", "种子未审核（reviewed=false）——请先在种子配置页完成预览审核")
	}
	// §59.159: 匿名发布取站点默认（form_config.Anonymous——站点配置勾选项）
	if cfg.Anonymous {
		in.Anonymous = true
	}
	pubReq := &model.PublishRequest{
		TorrentData: torrentData,
		FormFields:  form,
		// §59.462 刀⑤: 海豹翻译层产物注入（jobs 段局部变量→此处消费）
		ExtraArtists:   gpwArtists,
		SubtitleLangs:  gpwSubs,
		Title:          publishTitle,
		Subtitle:       meta.Subtitle, // §59.541 附二: 直读素材——原 form 字面量读在缺域/非恒等映射站断流（好学/朱雀缺域·馒头smallDescr/肉丝subtitle/野马shortDesc 非恒等·五站实锤）
		Description:    form[cfg.FormFields[model.FieldDomainDescription]],
		MediaInfo:      meta.MediaInfo,
		BDInfo:         meta.BDInfo,
		Screenshots:    parseScreenshotsCol(meta.Screenshots),
		IMDbLink:       meta.IMDbURL,
		DoubanLink:     meta.DoubanURL,
		Anonymous:      in.Anonymous,
		SourceSite:     meta.SiteName,
		SourceInfoHash: in.InfoHash,
		TargetSite:     in.TargetSite,
		ClientUID:      rv.ClientUID,
	}
	// §59.309: tags 复用前置构建的 TagArrayFields（预检与上传同一产物——
	// checkbox 数组=同名字段重复 §59.156 通道；§59.304 站规注入已在构建段）
	pubReq.TagArrayFields = tagArrayFields
	// §59.451: 馒头 labels 位掩码注入（站方特化——API TorrentUploadForm
	// labels int：bit0=DIY/bit1=中配/bit2=中字，无 tags checkbox 槽——
	// TagArrayFields 空，标签经 ExtraFields 通道翻译投递；adapter
	// uploadViaAPI 透传。0=无标签不写（bit 全零省字段）。
	if in.TargetSite == "馒头" {
		if bits := mteamLabelsBits(tags); bits > 0 {
			if pubReq.ExtraFields == nil {
				pubReq.ExtraFields = map[string]string{}
			}
			pubReq.ExtraFields["labels"] = strconv.Itoa(bits)
		}
	}
	// §59.462 刀⑤: 海豹 ExtraFields 注入（局部变量——非目标站恒 nil 零注入）
	if in.TargetSite == "海豹" && len(gpwExtra) > 0 {
		if pubReq.ExtraFields == nil {
			pubReq.ExtraFields = map[string]string{}
		}
		for k, v := range gpwExtra {
			pubReq.ExtraFields[k] = v
		}
		// §59.462 附七: gpw_image Rehost 转投——源值=dbposter（豆瓣域，
		// 站方抓不到）→默认图床域（系统配置）。失败降级保留原 URL
		// （图床故障不阻塞发布——肉丝同款降级语义）。DryRun 在此前已
		// 返回——预览值为原始 URL（无网络动作）。
		if src := pubReq.ExtraFields["gpw_image"]; src != "" &&
			e.pipe != nil && e.pipe.imageHostMgr != nil {
			if r, rErr := e.pipe.imageHostMgr.Rehost(ctx, src); rErr == nil && r != nil && r.URL != "" {
				pubReq.ExtraFields["gpw_image"] = r.URL
				e.logger.Info("gpw image rehosted", zap.String("to", r.URL))
			} else {
				e.logger.Warn("gpw image rehost failed（降级保留原 URL）", zap.Error(rErr))
			}
		}
	}
	// §59.458: 肉丝 ExtraFields 搬运（form 段注入的显示键→adapter
	// 消费通道；genre=JSON 数组串/region=slug 终态/poster=URL）
	if in.TargetSite == "肉丝" {
		if pubReq.ExtraFields == nil {
			pubReq.ExtraFields = map[string]string{}
		}
		for _, k := range []string{"rs_genre", "rs_region", "rs_poster"} {
			if v := form[k]; v != "" {
				pubReq.ExtraFields[k] = v
			}
		}
	}
	// §59.459: 野马 ExtraFields 搬运（regionList JSON 数组/译名/封面）
	if in.TargetSite == "野马" {
		if pubReq.ExtraFields == nil {
			pubReq.ExtraFields = map[string]string{}
		}
		for _, k := range []string{"yema_regions", "yema_translated", "yema_poster"} {
			if v := form[k]; v != "" {
				pubReq.ExtraFields[k] = v
			}
		}
	}

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
				model.PublishSkipStatuses).
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
	// ③ 站方 check 预查重（§59.515 刀2 D1——好大 API 独有·file 通道清洗重算
	// normalized_infohash 比我们本地指纹更准：源站私有字段清洗后 hash 不同·
	// 本地指纹对不上。read scope 零副作用·网络失败 warning 放行 409 兜底（E3））
	// 注：局部变量 adapter（SiteAdapter 接口）遮蔽包名——用 pkg 引用类型
	if checker, ok := adapter.(hdareaPreChecker); ok {
		pc, pcErr := checker.CheckBeforeUpload(ctx, siteConfig, torrentData)
		if pcErr != nil {
			// E3：check 网络失败 → warning 放行（409 是第二道防线）
			e.logger.Warn("check 预查重失败（放行·409 兜底）", zap.String("site", in.TargetSite), zap.Error(pcErr))
		} else if pc != nil && !pc.CanUpload {
			if pc.IsBanned {
				// E1：banned_duplicate → failed 终态（站方禁发资源·不是重复是禁止）
				msg := "站方禁发此资源: " + pc.Reason
				e.recordResult(ctx, in, meta, rv, "failed", msg, "", "", piecesHash)
				return fail("failed", msg)
			}
			// active_duplicate → duplicate 终态（与①②同语义）
			msg := "站方 check 拦截: " + pc.Reason
			if pc.ExistingName != "" {
				msg += "（" + pc.ExistingName + "）"
			}
			e.recordResult(ctx, in, meta, rv, "duplicate", msg, pc.ExistingID, detailURLOf(siteConfig, pc.ExistingID), piecesHash)
			return fail("duplicate", msg)
		} else if pc != nil && len(pc.ReplaceableDead) > 0 {
			// D2：断种顶替——站方将自动删旧上架·日志透出顶替意图
			e.logger.Info("check 顶替断种（站方将自动删旧上架）",
				zap.String("site", in.TargetSite),
				zap.Strings("dead_tids", pc.ReplaceableDead))
		}
	}

	resp, upErr := adapter.UploadTorrent(ctx, siteConfig, pubReq)
	timeoutRecheckMsg := ""
	if upErr != nil {
		// §59.470: 超时≠失败——站方可能已完成入库（南洋案实证：30s 超时但
		// 站方 09:43 实际入库 tid=225756·记录假失败+人工重试险重复发布）。
		// 超时类错误先站方复查（复用 pieces-hash dedup 通道）·查到同内容种
		// →按站上已有处理（uploaded_existing 语义·推种+pushed_existing）；
		// 复查无 → 真 failed。
		if isUploadTimeoutErr(upErr) {
			// 复查子 ctx：上传已耗尽外层预算大部分——WithoutCancel 保 trace
			// 值+去取消，再派生独立预算（复查=一次 pieces-hash 查询 60s 足够）
			recheckCtx, recheckCancel := context.WithTimeout(context.WithoutCancel(ctx), 60*time.Second)
			dup, dupMsg, mTID := e.pipe.dedupByPiecesHashFull(recheckCtx, adapter, siteConfig, torrentData)
			recheckCancel()
			if dup && mTID > 0 {
				tidShown := fmt.Sprintf("%d", mTID)
				if e.logger != nil {
					e.logger.Info("upload timeout but site recheck hit (treat as existing)",
						zap.String("site", in.TargetSite),
						zap.String("tid", tidShown),
						zap.Error(upErr))
				}
				timeoutRecheckMsg = "上传超时但站方复查确认已发布（tid=" + tidShown + "）: " + dupMsg
				resp = &model.PublishResponse{
					Success:    true,
					IsExisting: true,
					TorrentID:  tidShown,
					DetailURL:  detailURLOf(siteConfig, tidShown),
				}
				upErr = nil
			}
		}
		if upErr != nil {
			// §59.473: 复查通道失效站（海胆 pieces-hash 404）超时兜底文案——
			// 引导手动核验（超时≠失败·站方可能已入库）
			if isUploadTimeoutErr(upErr) {
				return failRec("failed", fmt.Sprintf("上传超时且站方复查不可用（%v）——超时≠失败，请上站核验是否已发布后再决定重试", upErr))
			}
			return failRec("failed", fmt.Sprintf("上传失败: %v", upErr))
		}
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
		Status:           uploadedStatus,
		Upload:           resp,
		TargetTorrentURL: resp.DetailURL,
	}
	// §59.470: 超时复查命中场景的消息注入（uploaded_existing 语义正常路径无 message）
	if timeoutRecheckMsg != "" {
		result.Message = timeoutRecheckMsg
	}
	if e.pipe.pusher != nil {
		pushReq := &pusher.PushRequest{
			ClientUID: rv.ClientUID,
			SiteName:  in.TargetSite,
			TorrentID: resp.TorrentID,
			InfoHash:  in.InfoHash, // §59.159: 源种 infohash——已存在辅种快路径
			Title:     meta.Title,
			SavePath:  rv.SavePath,
			// §59.515 刀4 D2: 站方回传终版种子直传（好大 return_torrent_base64
			// 通道·非空时 pusher 跳过 DownloadTorrent；其他站/未回传=空零影响）
			TorrentData: resp.TorrentData,
		}
		if pr := e.pipe.pusher.Push(ctx, pushReq); pr != nil {
			// §59.471 附: 推种分支消息与超时复查消息拼接（非覆盖——
			// 回归审核发现 AlreadyExist/Error 分支整串替换会丢"上传超时
			// 但站方复查确认已发布"的语义线索）
			mergeMsg := func(m string) string {
				if timeoutRecheckMsg != "" {
					return timeoutRecheckMsg + "；" + m
				}
				return m
			}
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
				result.Message = mergeMsg("下载器已有该种子（已做种）")
			default:
				if pr.Error != nil {
					result.Message = mergeMsg("加种失败: " + pr.Error.Error())
				}
			}
		}
	}

	// ⑩ 结果落库（发布日志页消费——recordResultFull 统一单点）
	seeded := result.Status == "pushed" || result.Status == "pushed_existing"
	// §59.166 LocalAudit 出口：成功态 message 后缀（failed 态不拼防混淆站方错误——
	// 回归审核定案③）；findings JSON 进 logs 列（实战回流分析对照站方结果）
	if len(localFindings) > 0 {
		result.Message = strings.TrimSpace(strings.TrimSuffix(result.Message, "；") +
			"；" + FormatLocalAudit(localFindings))
	}
	e.recordResultFullWithLogs(ctx, in, meta, rv, result.Status,
		joinNotes(paNoteOf(preAudit), result.Message),
		resp.TorrentID, result.TargetTorrentURL, seeded, piecesHash,
		SerializeLocalAudit(localFindings))
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
func (e *PublishExecutor) recordResultFullWithLogs(ctx context.Context, in ExecuteInput, meta *model.TorrentMetadata,
	rv *ResourceView, status, note, torrentID, url string, seeded bool, piecesHash string, logs string) {
	now := time.Now()
	seedErr := ""
	if strings.Contains(note, "加种失败") {
		seedErr = note
	}
	_ = e.pipe.CreateResult(ctx, &model.PublishResultRecord{
		PiecesHash:     piecesHash,
		TargetSite:     in.TargetSite,
		SourceSite:     meta.SiteName,
		SourceInfoHash: in.InfoHash,
		SavePath:       rv.SavePath,
		TorrentID:      torrentID,
		Status:         model.PublishResultStatus(status),
		SkipReason:     note,
		PublishURL:     url,
		Logs:           logs,
		Trigger:        "manual",
		BatchGroupID:   in.BatchGroupID,
		Title:          meta.Title,
		DownloaderID:   rv.ClientUID,
		Seeded:         seeded,
		SeedError:      seedErr,
		SeededAt: func() *time.Time {
			if seeded {
				return &now
			}
			return nil
		}(),
		CompletedAt: &now,
	})
}

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
		DownloaderID:   rv.ClientUID,
		Seeded:         seeded,
		SeedError:      seedErr,
		SeededAt: func() *time.Time {
			if seeded {
				return &now
			}
			return nil
		}(),
		CompletedAt: &now,
	})
}

// domainJob 域装配任务（主流程与 pre-audit 共用）。
type domainJob struct {
	domain string
	match  *model.FormValueMapping
}

// assembleTags 标签装配：判据引擎推断 → form_config 值映射 → auto:false 过滤 + 人工 overrides。
func (e *PublishExecutor) assembleTags(cfg *model.PublishFormConfig, meta *model.TorrentMetadata, overrides []string) []string {
	// §59.319 附二十七定案（用户）：发布只消费，不推断——
	// 推断唯一发生在 fetch 链（InferFull → DB tags 列），发布读 DB
	// 消费。消除双链分叉（光之子案：fetch 推断 chinese_audio ✓，
	// 发布重新 Infer 缺 Description → 零产出 → TAGS_EMPTY）。
	// 用户在种子配置页编辑的 tags 也存 DB——发布尊重用户编辑。
	var storedTags []string
	if meta.Tags != "" {
		if err := json.Unmarshal([]byte(meta.Tags), &storedTags); err != nil {
			storedTags = nil // 坏 JSON 按空处理（§59.28 G 语义）
		}
	}
	// 站方 tags 域 standard_key → value 映射 + auto:false 排除
	allowed := map[string]string{}
	for _, m := range cfg.ValueMappings[model.FieldDomainTags] {
		if m.Auto != nil && !*m.Auto {
			continue // 条件标签（英语/首发/原创/禁转）——只能经 overrides 进入
		}
		for _, k := range m.StandardKeys {
			allowed[k] = m.Value
			// canonical 双形态（stored 无前缀 "dolby_vision"；standard_key
			// 形态 "tag.dolby_vision"——两者同值）
			if i := strings.Index(k, "."); i > 0 {
				allowed[k[i+1:]] = m.Value
			}
		}
	}
	out := []string{}
	seen := map[string]bool{}
	// §59.518: 调整层工作标签放行——complete/collection/ongoing 是
	// adjustTagsForSite（§59.177/§59.518）的转换原料：值集无 complete 的站
	// （优堡）先滤后转=转的原料没了（243 DryRun 复验抓出：tags=['6'] 无合集
	// 20）。放行进调整层转换·产物再走正常映射（miss 自然蒸发=原行为）。
	adjustSourceTags := map[string]bool{
		"complete": true, "collection": true, "ongoing": true,
	}
	for _, t := range storedTags {
		if _, okAll := allowed[t]; (okAll || adjustSourceTags[t]) && !seen[t] {
			out = append(out, t)
			seen[t] = true
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
	meta *model.TorrentMetadata, form map[string]string, jobs []domainJob, tagArrayFields []model.TagKV,
	auditName string) (*PreAuditResult, string) {

	type idName struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	type auditBody struct {
		Name          string            `json:"name"`
		SmallDescr    string            `json:"small_descr"`
		IMDBURL       string            `json:"imdb_url"`
		Description   string            `json:"description"`
		TechnicalInfo string            `json:"technical_info"`
		Type          *idNameWithMode   `json:"type,omitempty"`
		Quality       map[string]idName `json:"quality,omitempty"`
		Tags          []idName          `json:"tags"`
		UserInfo      map[string]any    `json:"user_info,omitempty"`
		ExportTime    string            `json:"export_time,omitempty"`
		PageURL       string            `json:"page_url,omitempty"`
	}
	// §59.309: 预检 name 消费主管线产物（publishTitle 直传——按构造同源）；
	// 空串=主管线重组失败的兜底场景（真实路径已在上游 fail，此处防御降级）
	if auditName == "" {
		fallback, ok := e.preAuditTitle(meta)
		if !ok {
			return nil, "标题重组失败：数据不完整——重新获取数据后再预检"
		}
		auditName = fallback
	}
	body := auditBody{
		Name:          auditName,
		SmallDescr:    meta.Subtitle, // §59.541 附二: 同 Execute :641——audit 展示面同源直读（form 字面量读五站恒空）
		IMDBURL:       meta.IMDbURL,
		Description:   form[cfg.FormFields[model.FieldDomainDescription]],
		TechnicalInfo: TechTextOf(meta),
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
	// §59.309: tags 消费上传真相源 TagArrayFields（同一产物按构造同源）；
	// label 反查 miss 不丢（label=value 兜底——原 applied 表示层静默丢的分叉防）
	for _, kv := range tagArrayFields {
		name := kv.Value
		for _, m := range cfg.ValueMappings[model.FieldDomainTags] {
			if m.Value == kv.Value {
				name = m.Label
				break
			}
		}
		body.Tags = append(body.Tags, idName{ID: kv.Value, Name: name})
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
		TLSPretend:    site.TLSPretend, // §59.333
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

// AssembleTagArrayForSite §59.531: 目标站 tags 组装单源——发布链 Execute ⑤段
// 与站上标签修复工具（tools/fix-site-tags）同源消费（防双链分叉）。
// 返回三元组：tags（canonical 列表——M-Team labels 位等消费）·
// tagArrayFields（上传真相源：field+value·§59.529 同域同值去重）·
// applied（表示层值——DryRun Tags/labels 显示）。
// 行为零变化：段整体自 Execute 迁移（§59.309 构造同源/§59.400 财神喜剧/
// §59.277 幸运英语/§59.273 中字启发/§59.278 HDR 对齐全链随迁）。
func (e *PublishExecutor) AssembleTagArrayForSite(meta *model.TorrentMetadata,
	cfg *model.PublishFormConfig, targetSite, hdrProfile string,
	overrides []string) ([]string, []model.TagKV, []string) {
	tags := adjustTagsForSite(e.assembleTags(cfg, meta, overrides),
		targetSite, meta.Title, meta.Subtitle)
	// §59.273: 幸运中字启发（站方种审判据——MI 无中文字幕但产地为华语区且
	// 副标题带硬字幕/中英字幕说明时仍须选中字；The Furious/I Know Who You
	// Are 八案。站点特化——统一方法待其它站适配时再议，用户定案）
	tags = luckptChineseSubtitleHeuristic(targetSite, meta, tags)
	// §59.278: HDR 标签标题对齐——组装标题的 HDR token（tp.HDR 标题/DOM 源）
	// 与标签勾选一致（幸运种审判据"标题包含 HDR10→勾 HDR10 标签"）。MI 缺
	// HDR 行时推断器（§59.151 MI 唯一真相）不产 HDR 族标签——标题/DOM 源
	// 证据补位；站方无对应标签选项时 applier 自然丢弃（跨站安全）。
	tags = syncHDRTagFromProfile(hdrProfile, tags)
	tagCfg := &model.SiteTagConfig{
		Mode:      model.TagModeTaglist,
		Tags:      map[string]string{},
		SpanField: cfg.FormFields[model.FieldDomainTags],
	}
	if cfg.TagConfig != nil && cfg.TagConfig.Mode != "" {
		tagCfg.Mode = cfg.TagConfig.Mode
	} else if fieldName := cfg.FormFields[model.FieldDomainTags]; strings.Contains(fieldName, "[") {
		// §59.269 兜底: TagConfig 缺失但字段名是数组形态（tags[4][]）→ 推断
		// checkbox_span（推断源=显式 FormFields 数据派生；空配置现状=默认
		// taglist → 字段名硬编码 tagList → 标签全丢，兜底严格更好）
		tagCfg.Mode = model.TagModeCheckboxSpan
		if e.logger != nil {
			e.logger.Info("tag config missing, inferred checkbox_span from field name",
				zap.String("field", fieldName))
		}
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
	// §59.277: 幸运英语标签站规注入——TAGS_MISSING_ENGLISH_AUDIO 判据（无
	// 国语/粤语/中字且英语音轨→必须选英语）。§59.151 附7 的 lucky_english_audio
	// 通道被"英语选项 auto:false × 无 standard_keys"双卡断（allowed/tagCfg.Tags
	// 均按 StandardKeys 构建，英语选项零键恒 miss）——站规强制属 auto:false
	// 例外路径，Label 直配注入 applied。
	if v := luckptEnglishTagValue(cfg, targetSite, meta, tags); v != "" {
		dup := false
		for _, a := range applied {
			if a == v {
				dup = true
				break
			}
		}
		if !dup {
			applied = append(applied, v)
		}
	}
	// §59.400: 财神题材标签注入（喜剧——◎类别行同源消费）
	if v := csptGenreTagValue(cfg, targetSite, meta); v != "" {
		dup := false
		for _, a := range applied {
			if a == v {
				dup = true
				break
			}
		}
		if !dup {
			applied = append(applied, v)
		}
	}
	// §59.309: TagArrayFields（上传 tags 真相源——预检与 pubReq 同一产物，
	// 按构造同源。原预检消费 applied 表示层（值反查 label miss 静默丢）+上传独立
	// applier.Apply=双链分叉三案病根：§59.304 英语注入、§59.308 标题、本次审计）
	// §59.529: 同域同值去重——双 canonical 映射同 value（财神双 11：dolby_vision
	// 与 dolby_atmos 都命中「杜比」词条）时只投一次。站方表单对重复 checkbox 值
	// 会整栏作废（Ip Man 实证：标签全丢+Bot 拒）——投递层兜底防御，键层修正之外
	// 的第二道防线。
	var tagArrayFields []model.TagKV
	tagValueSeen := map[string]bool{}
	applier.Apply(tags, func(field, value string) {
		dedup := field + "\x00" + value
		if tagValueSeen[dedup] {
			return
		}
		tagValueSeen[dedup] = true
		tagArrayFields = append(tagArrayFields, model.TagKV{Key: field, Value: value})
	})
	if v := luckptEnglishTagValue(cfg, targetSite, meta, tags); v != "" {
		dup := false
		for _, kv := range tagArrayFields {
			if kv.Value == v {
				dup = true
				break
			}
		}
		if field := cfg.FormFields[model.FieldDomainTags]; !dup && field != "" {
			tagArrayFields = append(tagArrayFields, model.TagKV{Key: field, Value: v})
		}
	}
	// §59.400: 财神题材标签注入（喜剧）——上传真相源同构
	if v := csptGenreTagValue(cfg, targetSite, meta); v != "" {
		dup := false
		for _, kv := range tagArrayFields {
			if kv.Value == v {
				dup = true
				break
			}
		}
		if field := cfg.FormFields[model.FieldDomainTags]; !dup && field != "" {
			tagArrayFields = append(tagArrayFields, model.TagKV{Key: field, Value: v})
		}
	}
	return tags, tagArrayFields, applied
}
