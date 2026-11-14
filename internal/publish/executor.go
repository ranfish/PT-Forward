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
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/adapter"
	"github.com/ranfish/pt-forward/internal/metadata"
	"github.com/ranfish/pt-forward/internal/ptgen"

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
	PushClientID uint
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
	// §59.310: 字段名→友好展示（"域名 · 值label"）——audiocodec_sel[4]=12 →
	// "音频编码 · DTS"（form 键是站方字段名+值是站方枚举值，用户不可读）
	FormLabels map[string]string `json:"form_labels,omitempty"`
	// §59.311: 标签资产（canonical 键——Tab1 ③ 同源 meta.Tags；区别于 tags 字段
	// 的站方枚举值形态。用户语义定案：弹窗=系统计算资产+按站投递双通道展示）
	TagAssets []string `json:"tag_assets,omitempty"`
	// §59.312: 标签投递可读化——tags 枚举值 → 站方 label（与 tags 索引对齐；
	// 投递 6/10 → 完结/英语——预检人工查看语义）
	TagsLabels []string `json:"tags_labels,omitempty"`
	Tags     []string          `json:"tags,omitempty"`
	Upload   *model.PublishResponse `json:"upload,omitempty"`
	LocalAudit []LocalAuditFinding  `json:"local_audit,omitempty"` // §59.166 内部规范提示（advisory）
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
// preflightNoTransfer §59.409 附二: 发布行为前置检查——禁转/互斥/排除
// 家族单点（用户语义定案两层：获取链打标与目标无关；发布链拦"转载/
// 发布行为本身"与目标相关）。原挂 Pipeline.CheckPublishEligibility——
// §59.147 拆自动发布流后零生产调用（死代码），全家族从未在发布页链路
// 生效；用户"一站多种官种混选是否被阻塞"问出系统性缺口，本次搬家激活。
// 粒度=单颗×单目标（批量逐颗独立 Execute 不连坐——官种颗拦名单站，
// 同批非官种颗/官种颗发其它站均正常）：
//  ① 种子禁转标记（获取链落库 flags）：绝对禁转族拦全部目标站；限时
//     禁转窗内拦全部、窗外放行；深度站限转名单站永久拦+窗内全站拦
//     （checkFlagsFromMetadata 单点）
//  ② 定向禁转名单（优堡官种→PTTime 等——checkOfficialDirectedNoTransfer 单点）
//  ③ 互斥转载站点对（HardcodedExclusionPairs）
//  ④ 用户自定义排除规则（双保险——candidate 创建层已查，单发/重放路径）
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

	// §59.159 PushOnly 补推通道：跳过上传/dedup——推指定 TorrentID（SavePath 取资源目录）
	if in.PushOnly {
		if in.TorrentID == "" {
			return failRec("failed", "PushOnly 需 TorrentID")
		}
		if e.pipe.pusher == nil {
			return failRec("failed", "pusher 未注入")
		}
		clientUID, savePath := in.PushClientID, in.PushSavePath
		if clientUID == 0 || savePath == "" {
			clientUID, savePath = rv.ClientUID, rv.SavePath // 回落资源定位
		}
		pushReq := &pusher.PushRequest{
			ClientUID: clientUID,
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
			e.repushConverge(ctx, in, time.Now())
		}
		return res
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
	// §59.510: 海报外链 URL 直填（瓷器 cover 表单域——PTGen 简介首行
	// [img]URL[/img] 剥壳得裸地址·hdwing.md 站方文档「海报外链 URL 直填
	// （非 BBCode）」·未配 cover 域的站自然跳过零影响）
	setForm(model.FieldDomainCover, coverURLOfDescription(meta.Description))
	setForm(model.FieldDomainDescription, meta.Description) // renderDescription 结果后续覆盖
	// §59.319 P2: 原盘 tech 文本源 BDInfo 优先（表单 techinfo 域/推断/审计同源）
	setForm(model.FieldDomainTechInfo, TechTextOf(meta))
	setForm(model.FieldDomainIMDBURL, meta.IMDbURL)
	// §59.159: PT-Gen/豆瓣链接（用户实战指认——发布页面完整投递；PTNexus 同款
	// pt_gen 值=豆瓣链接；FormFields 未配站点自然跳过）
	setForm(model.FieldDomainPTGen, meta.DoubanURL)
	setForm(model.FieldDomainDoubanURL, meta.DoubanURL)
	// §59.399: 编号形态字段（学校站 imdbid/doubanid 仅收编号 tt4154756/24773958——
	// 站方特别要求）——从完整 URL 提取尾部编号；URL 空/形态不匹配不填（站方
	// 非必填不阻断）；DB 值已是纯编号形态直出兼容（Tab4 用户手输 §59.341）。
	// 与 imdb_url/dburl 完整链接域并存——域语义=站方要什么形态配什么域。
	setForm(model.FieldDomainIMDbID, extractIMDbID(meta.IMDbURL))
	setForm(model.FieldDomainDoubanID, extractDoubanID(meta.DoubanURL))

	// §59.166 A 层同源化：表单四域（standard/codec/audiocodec/medium）改消费
	// BuildTechProfile（标题+MI+DOM 三源合并——种配页重组器同款，MI 纠错终态），
	// 替代 meta 原始列。五案根治：From S04(WEBRip 媒介跟标题)/宽幅 2K(高度
	// 1080 归一)/天空之城(audio)/Arco(MI 纠错 DDP)。type/team 域保持原源。
	domMedium, domRes, domVideo, domAudio := titleparser.DOMFieldsFromDetailSource(meta.DetailSourceJSON)
	tp := titleparser.BuildTechProfile(meta.Title, TechTextOf(meta), domMedium, domRes, domVideo, domAudio)
	// §59.514 附八: tp 终态化走 FinalizeTitle 共享单点（PTGen+仲裁+归一+
	// 重组——t0/recompute/发布三链结构性同源·原 :349-357 三段收编）
	finalTitle, arbNotes := e.FinalizeTitle(ctx, meta, &tp, torrentData)
	if len(arbNotes) > 0 && e.logger != nil {
		e.logger.Info("torrent-arbitration", zap.String("info_hash", meta.InfoHash), zap.Strings("notes", arbNotes))
	}
	// §59.424: 系统级禁发剧集分集（用户定案——所有站点仅允许合集/完结/
	// 全集形态）。挂载点=归一后（tp.SeasonEpisode 已是归一终态）+PushOnly
	// 分支之后（P1 定案：补推豁免——形态恒定检查与时变检查（flags）挂载
	// 策略不同，补推重放重查只误伤存量加种）。DryRun 同拦（预览可见）。
	if blocked, reason := episodeReleaseBlocked(meta, &tp); blocked {
		return failRec("ineligible", reason)
	}
	// §59.425: 碟粉站点级双检查（五年规则+禁欧美——用户定案）。挂载
	// PushOnly 分支后（恒定检查同 §59.424 P1 补推豁免）；DryRun 同拦可见。
	// **站点门控（回归修复：首版漏门控——碟粉规则拦了所有站，时光
	// DryRun 被"碟粉站点规则"拦实战抓出）**：仅 targetSite==碟粉 触发。
	if in.TargetSite == "碟粉" {
		if blocked, reason := discfanReleaseBlocked(meta, &tp); blocked {
			return failRec("ineligible", reason)
		}
	}
	// §59.453: 好学站点级拦截（用户定案）——仅纪录片（410 特别通道）可发，
	// 普通影视资源不适用本站（学习资料站）。挂载同碟粉（PushOnly 分支后
	// 恒定检查+DryRun 同拦可见+站点门控——§59.426 形态差异清单）。
	if in.TargetSite == "好学" {
		if blocked, reason := hxptOnlyDocumentary(meta); blocked {
			return failRec("ineligible", reason)
		}
	}
	// §59.166 B2：发布标题重组同源——ReassembleFromTechProfile(tp) 为权威（MI 纭错
	// 终态，Arco 案 DTS-HD MA→DDP 发布时自动纠对存量错标题）。
	// §59.254 项4 D 终案：重组空=四层全灭（三源+PTGen 兜底后仍缺损）——拒发+精准引导
	// （预览链展示且警示不拒——预览本身即发布前检查工具）。
	// §59.514 附八（用户定案）: 标题锚定列——列在场时以列为准（列=发布·
	// 用户 Tab1 看到的=发出的·结构性一致）；列空（存量/降级场景）fallback
	// FinalizeTitle 现算产物。逻辑升级后 recompute 刷列（§59.500 刀4 运维
	// 模式配套）——升级窗口内列=旧逻辑产物·发布随列（用户定案接受）。
	publishTitle := ""
	if finalTitle != "" {
		publishTitle = finalTitle
	} else {
		if !tp.MIHasVideo {
			return fail("failed", "标题重组失败：MI 缺失或视频不可读——检查下载器文件可访问性后重新获取数据")
		}
		return fail("failed", "标题重组失败：标题不可解析且 PTGen 无英文名——重新获取数据后再发布")
	}
	if meta.ReassembledTitle != "" {
		if meta.ReassembledTitle != publishTitle && e.logger != nil {
			e.logger.Info("publish title anchored to stored column (diff with live finalize)",
				zap.String("info_hash", meta.InfoHash),
				zap.String("column", meta.ReassembledTitle),
				zap.String("live", publishTitle))
		}
		publishTitle = meta.ReassembledTitle
	}
	jobs := []domainJob{
		{model.FieldDomainType, e.typeMappingOf(cfg, in.TargetSite, meta, tp)},
		{model.FieldDomainStandard, e.lookupByStdKey(cfg, model.FieldDomainStandard, extract.LookupStandardKey("resolution", tp.Resolution))},
		{model.FieldDomainCodec, e.codecMappingOf(cfg, tp)},
		{model.FieldDomainAudiocodec, e.audioMappingOf(cfg, model.FieldDomainAudiocodec, tp.AudioCodec, tp.AudioTechnology)},
		{model.FieldDomainMedium, e.mediumMappingOf(cfg, in.TargetSite, meta, tp)},
		{model.FieldDomainTeam, e.teamMapping(cfg, meta)},
		// §59.471: 地区/风格（HDVideo 必填——站特化路由+兜底；其它站未配
		// value_mappings 域时 lookup nil 跳过零影响）
		{model.FieldDomainRegion, e.regionMappingOf(cfg, in.TargetSite, meta)},
		{model.FieldDomainStyle, e.styleMappingOf(cfg, in.TargetSite, meta)},
	}
	for _, j := range jobs {
		if j.match == nil {
			continue
		}
		if field, ok := cfg.FormFields[j.domain]; ok && field != "" {
			form[field] = j.match.Value
		}
	}
	// §59.421: 候选发布站点默认（form_config.Offer——站点配置勾选项）：
	// 开启则提交时勾选站方候选 checkbox（优堡 asoffer/家园 offers——字段名
	// 走 form_fields["offer"] 映射；默认不勾=直接发布）
	if cfg.Offer {
		if fn, ok := cfg.FormFields[model.FieldDomainOffer]; ok && fn != "" {
			form[fn] = "yes"
		}
	}
	// §59.472: 海胆季/集/合集注入（未配置 season 键的站零动作）
	haidanSeasonEpisodeInject(form, cfg, meta, &tp)
	// §59.511: 瓷器固定字段注入（share_rule=3 欢迎/board=1 普通区/
	// is3d=Stereo3D 勾——未配置键零动作）
	hdwingPublishInject(form, cfg, meta, tp)
	// §59.289 fail-fast: 站方表单要求 type（FormFields 有域）但映射不出
	// （元数据 category 无对应选项）——注定失败的上传不发出去，本地精准
	// 拒发（修道院纪录片单复数错位案：type 不发→站方 Invalid integer
	// →未知响应三连）。缺省站（FormFields 无 type 域）不受影响。
	if tf, ok := cfg.FormFields[model.FieldDomainType]; ok && tf != "" {
		if _, filled := form[tf]; !filled {
			return failRec("failed", fmt.Sprintf(
				"类型映射缺失：站点表单要求 type（%s），元数据 category=%q 无对应选项——检查站点发布表单配置 type 域映射", tf, meta.Category))
		}
	}
	// §59.471 附: region/style 必填域同款 fail-fast（回归审核补——配置异常
	// （兜底项被手改删除）时静默不发→站方"请选择地区"错误循环重蹈；正常
	// 路径有兜底（28/6）不会触发）。
	for _, domain := range []string{model.FieldDomainRegion, model.FieldDomainStyle} {
		if f, ok := cfg.FormFields[domain]; ok && f != "" {
			if _, filled := form[f]; !filled {
				return failRec("failed", fmt.Sprintf(
					"%s 映射缺失：站点表单要求必填域 %s（%s）但映射不出（含兜底）——检查站点发布表单配置 %s 域 value_mappings（含兜底项）", domain, domain, f, domain))
			}
		}
	}

	// §59.453: 好学 410 纪录片特别通道四槽注入（年级恒其它/版本恒其它/
	// 年份三级链/格式容器判定——用户逐点定案；jobs 通用链零配键 miss 不填
	// 的槽由特化覆盖——非纪录片已在上游 hxptOnlyDocumentary 拦截）。
	// 挂载点=jobs 段后 DryRun 检查点前（首版误挂 pubReq 装配段在 DryRun
	// return 之后——DryRun 看不到注入，形态一实测年级/版本/年份全 None 抓出）。
	if in.TargetSite == "好学" {
		hxptDocumentaryPrefill(form, cfg, meta, tp)
	}
	// §59.454: 城市站特化注入（题材/字幕四 checkbox/3D/bigname 双填/
	// codec 音频类切换——jobs 段后 DryRun 检查点前，§59.453 挂载教训）。
	// tags 产物依赖（chnize/sub_hard 消费 chinese_subtitle）——城市注入
	// 分两段：标签无关项在 jobs 段后（此处），标签依赖项在 tags 产出后
	// （hdcityTagDependentFill）。
	// §59.458: 肉丝特化注入（genre/region/poster——attributes 数据源；
	// genre/region 非发布链标准域走 form 显示键+ExtraFields 双通道——
	// form 键 DryRun 可见 §59.453 挂载教训；ExtraFields adapter 消费）。
	// §59.462 刀⑤: 海豹（GazellePW）发布翻译层——内容拦截+建组八字段+
	// 版本层特化+剧组+subtitle_type+EditionInfo+8 特性 checkbox。
	// 挂载模式：肉丝/野马同款站点特化（jobs 段后 DryRun 检查点前）。
	// §59.462 附八: 产物走 Execute 局部变量（同函数内两段传递）——
	// 旧 struct 字段桥接在批量并行/HTTP 并发下跨 goroutine 串数据
	// （executor 单例）——回归审核 P1。
	var gpwExtra map[string]string
	var gpwArtists []model.PublishArtist
	var gpwSubs []string
	if in.TargetSite == "海豹" {
		// 2d-① 内容拦截（纯电影站——好学模式+animation S/E 特判）
		if blocked, reason := gpwOnlyMovies(meta); blocked {
			return failRec("ineligible", reason)
		}
		// 2d-② 截图三要件门 §59.462 附九（格式 PNG/图床白名单/≥3 张——
		// 站规 2.2.1 硬性要求，质量门=拦截+行动指引文案；DryRun 同拦）
		if blocked, reason := gpwScreenshotGate(parseScreenshotsCol(meta.Screenshots)); blocked {
			return failRec("ineligible", reason)
		}
		// 翻译注入（三层通道：form 特化覆盖/ExtraFields gpw_/ExtraArtists+SubtitleLangs）
		gpwExtra = map[string]string{}
		gpwSize := e.gpwLookupSize(ctx, in.InfoHash)
		gpwPublishInject(cfg, form, meta, tp, gpwSize, gpwExtra, &gpwArtists, &gpwSubs)
		// DryRun 检查点输出（gpw_ 键进 form 便于预览；§59.462 附八:
		// 剧组/字幕数量一并预览——ExtraArtists/SubtitleLangs 不进 form
		// 主体，无此键 DryRun 看不到剧组组装结果）
		for k, v := range gpwExtra {
			form["[gpw]"+k] = v
		}
		form["[gpw]gpw_artists_n"] = strconv.Itoa(len(gpwArtists))
		if len(gpwSubs) > 0 {
			form["[gpw]gpw_subtitles"] = strings.Join(gpwSubs, ",")
		}
	}

	if in.TargetSite == "肉丝" {
		rousiPublishInject(form, meta)
	}
	// §59.459: 野马特化注入（regionList 产区数组≤3/中文译名/封面 URL）
	if in.TargetSite == "野马" {
		yemaPublishInject(form, meta)
	}
	if in.TargetSite == "城市" {
		hdcityPrefill(form, cfg, meta, tp)
		// codec 音频类切换（§2.7 编码-媒介对应表——音频类查 codec 域
		// 的 audio.* 键，覆盖通用链视频键 miss 的空值）
		if m := e.hdcityCodecOption(cfg, meta, tp); m != nil {
			if f := cfg.FormFields[model.FieldDomainCodec]; f != "" {
				form[f] = m.Value
			}
		}
	}

	// ⑤ tags（判据引擎 → 站点调整层 → form_config 反查 → auto:false 过滤 + 人工 overrides）
	tags := adjustTagsForSite(e.assembleTags(cfg, meta, in.TagOverrides),
		in.TargetSite, meta.Title, meta.Subtitle)
	// §59.273: 幸运中字启发（站方种审判据——MI 无中文字幕但产地为华语区且
	// 副标题带硬字幕/中英字幕说明时仍须选中字；The Furious/I Know Who You
	// Are 八案。站点特化——统一方法待其它站适配时再议，用户定案）
	tags = luckptChineseSubtitleHeuristic(in.TargetSite, meta, tags)
	// §59.278: HDR 标签标题对齐——组装标题的 HDR token（tp.HDR 标题/DOM 源）
	// 与标签勾选一致（幸运种审判据"标题包含 HDR10→勾 HDR10 标签"）。MI 缺
	// HDR 行时推断器（§59.151 MI 唯一真相）不产 HDR 族标签——标题/DOM 源
	// 证据补位；站方无对应标签选项时 applier 自然丢弃（跨站安全）。
	tags = syncHDRTagFromProfile(tp.HDR, tags)
	// §59.454: 城市站标签依赖注入（chnize 消费 meta.Tags 的
	// chinese_subtitle——不经 assembleTags 过滤：城市站无 tags 域配置
	// → allowed 空 → 全部标签被滤（首版传 tags 变量恒空实测抓出）；
	// 挂 DryRun 检查点前——§59.453 挂载教训）。
	if in.TargetSite == "城市" {
		hdcityTagDependentFill(form, meta.Tags)
	}
	tagCfg := &model.SiteTagConfig{
		Mode:     model.TagModeTaglist,
		Tags:     map[string]string{},
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
	if v := luckptEnglishTagValue(cfg, in.TargetSite, meta, tags); v != "" {
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
	if v := csptGenreTagValue(cfg, in.TargetSite, meta); v != "" {
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

	// ⑥ 描述组装——纯本地资产消费（§59.159 白名单：替代 renderDescription
	// [queryPTGen 在线/rehostPoster 图床转存双违规]；零网络依赖）
	if strings.TrimSpace(meta.Title) == "" || strings.TrimSpace(meta.Description) == "" {
		return failRec("ineligible", "资产不完备：name/简介为空（先完成获取-审核）")
	}
	form[cfg.FormFields[model.FieldDomainDescription]] = assembleDescription(meta, cfg)

	// §59.309: TagArrayFields 前置构建（上传 tags 真相源——预检与 pubReq 同一产物，
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
	if v := luckptEnglishTagValue(cfg, in.TargetSite, meta, tags); v != "" {
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
	if v := csptGenreTagValue(cfg, in.TargetSite, meta); v != "" {
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

	// DryRun 检查点：组装+渲染+预检之后、上传之前（§59.156——完整表单形态验证，
	// 预检未通过不阻断 dry_run：预检修正循环数据源）
	if in.DryRun {
		res := &ExecuteResult{Status: "dry_run_ok", Form: form, Tags: applied, PreAudit: preAudit, LocalAudit: localFindings,
			FormLabels: buildFormLabels(cfg, form),
			TagAssets: parseTagsAssets(meta.Tags),
			TagsLabels: buildTagsLabels(cfg, applied)}
		// §59.515 刀2 E4：check 预查重结果透出（发布前核对闭环核心价值——
		// read scope 零副作用·网络失败降级为提示不阻断）
		if e.pipe.siteProvider != nil {
			if dryAdapter, daErr := e.pipe.siteProvider.GetAdapter(ctx, in.TargetSite); daErr == nil {
				if dryConfig, dcErr := e.pipe.siteProvider.GetSiteConfig(ctx, in.TargetSite); dcErr == nil {
					if checker, ok := dryAdapter.(hdareaPreChecker); ok {
						if pc, pcErr := checker.CheckBeforeUpload(ctx, dryConfig, torrentData); pcErr == nil && pc != nil {
							if !pc.CanUpload {
								if pc.IsBanned {
									res.Message = "⚠ 站方禁发此资源: " + pc.Reason
								} else {
									res.Message = "⚠ 站方已有同内容种 tid=" + pc.ExistingID + "（" + pc.ExistingName + "）可辅种"
								}
							} else if len(pc.ReplaceableDead) > 0 {
								res.Message = "将顶替断种 [" + strings.Join(pc.ReplaceableDead, ",") + "]（站方自动删旧上架）"
							}
						}
					}
				}
			}
		}
		if preAudit != nil && !preAudit.Passed {
			res.Message = "预检未通过（dry_run 不阻断）"
		}
		// §59.514 刀2: 仲裁记录透出（DryRun 人工核对通道——标题与实际
		// 文件一致性的修正明细）
		if len(arbNotes) > 0 {
			res.Message = strings.TrimSpace(res.Message + "\n.torrent 仲裁: " + strings.Join(arbNotes, "；"))
		}
		return res
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
		ExtraArtists: gpwArtists,
		SubtitleLangs: gpwSubs,
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
		ClientUID: rv.ClientUID,
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
		Status: uploadedStatus,
		Upload: resp,
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
		SeededAt:       func() *time.Time { if seeded { return &now }; return nil }(),
		CompletedAt:    &now,
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

// luckptChineseSubtitleHeuristic §59.273→§59.278 终版: 幸运中字启发式补标。
// 判据（用户定案演进）：§59.273 华语区产地门 → §59.278 硬字幕无条件化 →
// 终版产地忽略——副标题含 中字/中英字幕/硬字幕 任一即补 chinese_subtitle
// （MI 无中文字幕轨也补——硬字幕内嵌不体现为字幕轨）；"无中字"否定式剥除。
func luckptChineseSubtitleHeuristic(siteName string, meta *model.TorrentMetadata, tags []string) []string {
	if siteName != "幸运" || meta == nil {
		return tags
	}
	for _, t := range tags {
		if t == "chinese_subtitle" {
			return tags
		}
	}
	// §59.278 终版（用户定案：产地忽略）——副标题含 中字/中英字幕/硬字幕/
	// 特效字幕/特效中字/简繁/简中/繁中/简体/繁体 任一即推断中字（PT 语境
	// 发布者声明的特效字幕默认中文——§59.151 保守假设零案例支撑被推翻；
	// 简繁族=中文字形变体描述。裸"特效"不命中——§59.73 防误判保持）。
	// "无中字"先剥除（§59.277 Deep Water 案语义：否定式声明不得误配 中字 子串）。
	sub := strings.ReplaceAll(meta.Subtitle, "无中字", "")
	for _, kw := range []string{"中字", "中英字幕", "硬字幕", "特效字幕", "特效中字", "简繁", "简中", "繁中", "简体", "繁体"} {
		if strings.Contains(sub, kw) {
			return append(tags, "chinese_subtitle")
		}
	}
	return tags
}

// syncHDRTagFromProfile §59.278: HDR 标签与组装标题 HDR token 对齐。
// tp.HDR（标题/DOM 源——重组 hdr 槽数据源）非空而 MI 推断未产 HDR 族标签时
// 补位（十三猎杀案：标题 HDR10 重组输出含 HDR10，MI 无 HDR 行标签缺失——
// 幸运种审判据"标题包含 HDR10→勾 HDR10"）。已有 HDR 族标签（MI 铁证优先）
// 不覆盖；站方无对应选项时 applier 自然丢弃（跨站安全）。
func syncHDRTagFromProfile(hdr string, tags []string) []string {
	if hdr == "" || hdr == "SDR" {
		return tags
	}
	hdrFamily := map[string]bool{
		"hdr10": true, "hdr10_plus": true, "hdr_vivid": true, "hlg": true, "dolby_vision": true,
	}
	for _, t := range tags {
		if hdrFamily[t] {
			return tags
		}
	}
	var key string
	switch {
	case strings.Contains(hdr, "Vivid"):
		key = "hdr_vivid"
	case strings.Contains(hdr, "DoVi") || strings.Contains(hdr, "DV"):
		key = "dolby_vision"
	case strings.Contains(hdr, "HDR10+"):
		key = "hdr10_plus"
	case strings.Contains(hdr, "HDR"):
		key = "hdr10"
	case strings.Contains(hdr, "HLG"):
		key = "hlg"
	default:
		return tags
	}
	return append(tags, key)
}

// luckptEnglishTagValue §59.277: 幸运英语标签站规——无国语/粤语/中字且英语
// 音轨（english_audio/lucky_english_audio 推断 或 副标题"英语/无中字"声明）
// → 返回英语选项 value（Label 直配——auto:false 无 standard_keys 例外路径）。
// Deep Water 2022 AMZN WEB-DL 种审案（MI 无中文字幕 + 副标题[英语/无中字]）。
func luckptEnglishTagValue(cfg *model.PublishFormConfig, siteName string, meta *model.TorrentMetadata, tags []string) string {
	if siteName != "幸运" || cfg == nil {
		return ""
	}
	// §59.326 附三补2: 证据判定读 stored 原始推断（meta.Tags）+ 过滤后双源
	// ——assembleTags 按 auto:false 滤掉 english_audio 后再问"有英语吗"
	// 恒 false（Alice 三案：tags=[dolby_vision,hdr10] 英语证据不可见）。
	stored := []string{}
	if meta != nil && meta.Tags != "" {
		if err := json.Unmarshal([]byte(meta.Tags), &stored); err != nil {
			stored = nil
		}
	}
	merged := append(append([]string{}, tags...), stored...)
	// §59.326 附七（用户三规则定案，取代附五"无条件注入"）：
	// 规则 1（人工审核）+规则 3（用户提出，机器实证支持——造雨人
	// 中字+英语同选 passed）：**音轨与字幕物理语义**——国语/粤语音轨
	// 是版本属性（国配/粤配版与英语原声版互斥）→ 硬排斥英语标签；
	// 中文字幕是叠加属性（英语原声+中字=最常见原声中字版，正交）→
	// 不排斥。注入条件=英语证据在场 ∧ 无 chinese_audio ∧ 无
	// cantonese_audio（附五全拦→全放的精细回归：只拦音轨放行字幕）。
	// §59.350（站方规则收紧——243 全检 61 簇冲突+用户实测审核脚本
	// 原文推翻附七字幕正交实证）：中文字幕同样排斥——"英语标签仅在
	// 既没有国语、粤语音轨，也没有中文字幕时才能勾选"。语义回归
	// auto=False 条件标签本义：英语=纯英文无中文内容专版标记（生肉版）。
	for _, t := range merged {
		if t == "chinese_audio" || t == "cantonese_audio" || t == "chinese_subtitle" {
			return "" // 国/粤语音轨或中文字幕在场——站方规则禁勾英语（§59.350）
		}
	}
	hasEnglish := false
	for _, t := range merged {
		if t == "english_audio" || t == "lucky_english_audio" {
			hasEnglish = true
			break
		}
	}
	// 副标题声明兜底：MI 无 Language 行时 english_audio 可能未推断，
	// "英语/无中字"声明是站方判据源
	if !hasEnglish && meta != nil {
		sub := meta.Subtitle
		if strings.Contains(sub, "英语") || strings.Contains(sub, "无中字") {
			hasEnglish = true
		}
	}
	if !hasEnglish {
		return ""
	}
	for _, m := range cfg.ValueMappings[model.FieldDomainTags] {
		if m.Label == "英语" {
			return m.Value
		}
	}
	return ""
}

// csptGenreTagValue §59.400: 财神题材标签站规——description ◎类　　别
// 行含"喜剧"→注入财神"喜剧"标签（Label 直配——label-only 无
// standard_keys 例外路径，luckptEnglishTagValue 站规注入同款先例）。
// 缩小人生案（243 财神种审报"标签与简介类别不一致(喜剧)"）：喜剧是
// PTGen 题材细分（category 列只存大类 movie、canonical 无 tag.comedy
// 键——fetch 推断永不产出）；站方审种脚本用简介◎类别行校验——发布
// 同源消费（确定性正则提取非语义推断，§59.319 附二十七"只消费不
// 推断"合规）。儿童(24)题材标签零案例不预改（按需生长纪律）。
// 正则注：类/别间是全角空格（U+3000）——Go \s 只匹配 ASCII 空白，
// 字符类显式含　。
var reGenreLine = regexp.MustCompile(`◎类[　\s]*别[　\s]*([^\n]+)`)

func csptGenreTagValue(cfg *model.PublishFormConfig, siteName string, meta *model.TorrentMetadata) string {
	if siteName != "财神" || cfg == nil || meta == nil || meta.Description == "" {
		return ""
	}
	m := reGenreLine.FindStringSubmatch(meta.Description)
	if len(m) < 2 || !strings.Contains(m[1], "喜剧") {
		return ""
	}
	for _, t := range cfg.ValueMappings[model.FieldDomainTags] {
		if t.Label == "喜剧" {
			return t.Value
		}
	}
	return ""
}

// lookupOtherOption §59.273: 域内 Other 选项兜底（Label 判定——站方 Other
// 普遍无 standard_key，键查必落空；幸运 audio/codec/team 三域 Other 全无键
// 实证，修道院 codec Other 有键属例外。Label 兼容 Other/其它/其他。
// §59.417 前缀变体增强：站方 Other 项带后缀（我的PT "Other/转载"）——
// 精确匹配 miss 后 HasPrefix("other") 兜底扫描（通用增强未来站自动生效）。
func (e *PublishExecutor) lookupOtherOption(cfg *model.PublishFormConfig, domain string) *model.FormValueMapping {
	for i, m := range cfg.ValueMappings[domain] {
		lower := strings.ToLower(m.Label)
		switch lower {
		case "other", "其它", "其他":
			return &cfg.ValueMappings[domain][i]
		}
		if strings.HasPrefix(lower, "other") {
			return &cfg.ValueMappings[domain][i]
		}
	}
	// §59.511 附: 复合词形 Contains 兜底——hdwing "其他(Other)" 双语
	// 复合形态两形态均 miss（精确枚举+other 前缀）→ team 槽缺失（DryRun
	// 2046 案实证）；对齐同包 fallbackByOtherLabel 的 Contains 语义。
	for i, m := range cfg.ValueMappings[domain] {
		lower := strings.ToLower(m.Label)
		if strings.Contains(lower, "其它") || strings.Contains(lower, "其他") || strings.Contains(lower, "other") {
			return &cfg.ValueMappings[domain][i]
		}
	}
	return nil
}

// codecMappingOf §59.268: 视频编码域映射——miss 折叠兜底。
// 编码器实现归标准（x264→H.264 / x265→H.265——站方表单普遍只有标准名选项；
// 修道院 224/288 未选案实证：x264×216+x265×6+AV1×2，BluRay/WEB-DL x264 通杀）。
// 无对应标准的新编码（AV1/VVC/AVS2）→ Other 兜底（站方无 Other 选项时维持
// nil 不强填）。
// §59.380 百川案定案（用户）：压制形态联动——媒介判定 encode 且站点表单
// 提供 CLI 压制形态选项（百川 §5.2 站方语义：原盘/Remux→H.26x、压制/
// Encode→X26x）时优先 CLI 选项。媒介判定单源 mediumStdKeyOf（防双实现）。
// 通用语义：站方提供 X26x 选项=站方有此区分维度（NP 惯例），压制种选
// CLI 形态更贴站方；无该选项的站（龙之家/壹吧）行为零变化。
func (e *PublishExecutor) codecMappingOf(cfg *model.PublishFormConfig, tp titleparser.TechProfile) *model.FormValueMapping {
	// §59.436: 原盘编码分维特化（PTFans §4.1 站方明文：原盘/Remux→
	// Bluray(AVC/HEVC/VC-1) 项、压制/WEB→H.26x 项——编码×媒介双维）
	if v := blurayCodecOption(cfg, tp); v != nil {
		return v
	}
	stdKey := extract.LookupStandardKey("video_codec", tp.VideoCodec)
	m := e.lookupByStdKey(cfg, model.FieldDomainCodec, stdKey)
	if m == nil {
		// 折叠路径（x264/x265 CLI 形态键→H 标准键——§59.268 归标准）
		if folded := titleparser.FoldStandardKey("video_codec", stdKey); folded != stdKey {
			m = e.lookupByStdKey(cfg, model.FieldDomainCodec, folded)
		}
	}
	if m == nil {
		// §59.273: Other 兜底改 Label 判定（幸运 codec Other 无 standard_key——
		// 键查在无键站落空，Opus 类未选音频案同族）
		return e.lookupOtherOption(cfg, model.FieldDomainCodec)
	}
	// §59.380: 压制形态联动（两命中路径统一——直查/折叠都过）——encode
	// 媒介 + 站点有 CLI 形态选项优先（百川 §5.2 站方语义）
	if mediumStdKeyOf(tp, "") == "medium.encode" {
		if cli := lookupCLIVariant(cfg, stdKey); cli != nil {
			return cli
		}
	}
	return m
}

// blurayCodecOption §59.436: 原盘编码分维特化——站方 codec 区分原盘
// （Bluray(AVC)/Bluray(HEVC)/Bluray(VC-1)）与压制（H.264/H.265）两
// 体系（PTFans §4.1 站方明文：原盘/Remux→Bluray 项、压制/WEB→H.26x）。
// **动态检测非站名硬编码**：codec 槽存在 label 前缀 "Bluray(" 项才触
// 发（未来同形态站自动生效，§59.407 diyMediumOption 同款通用投资）；
// 仅原盘族媒介（含 Remux——站方语义 Remux 归原盘编码项）分发，
// 压制/WEB-DL 走通用链零变化。
func blurayCodecOption(cfg *model.PublishFormConfig, tp titleparser.TechProfile) *model.FormValueMapping {
	if cfg == nil {
		return nil
	}
	switch mediumStdKeyOf(tp, "") {
	case "medium.bluray", "medium.uhd_bluray", "medium.hddvd",
		"medium.remux", "medium.uhd_remux":
	default:
		return nil // 压制/WEB/CD 等走通用链
	}
	stdKey := extract.LookupStandardKey("video_codec", tp.VideoCodec)
	var want string
	switch stdKey {
	case "video.h264", "video.x264":
		want = "AVC"
	case "video.h265", "video.x265":
		want = "HEVC"
	case "video.vc1":
		want = "VC-1"
	default:
		return nil // AV1/MPEG-2 等无原盘分维项
	}
	items := cfg.ValueMappings[model.FieldDomainCodec]
	for i := range items {
		if strings.HasPrefix(items[i].Label, "Bluray(") && strings.Contains(items[i].Label, want) {
			return &items[i]
		}
	}
	return nil
}

// lookupCLIVariant §59.380: 视频编码 CLI 压制形态选项查找（百川 §5.2
// 站方语义——压制种选 X26x）。label 精确匹配（大小写两形态）；仅
// video.h264/video.h265 有 CLI 惯例变体（AV1/VVC 无）。
func lookupCLIVariant(cfg *model.PublishFormConfig, stdKey string) *model.FormValueMapping {
	variants := map[string][]string{
		"video.h264": {"X264", "x264", "X.264", "x.264"},
		"video.h265": {"X265", "x265", "X.265", "x.265"},
		// CLI 原始键形态（LookupStandardKey 对 x265 可能产出独立键——
		// 折叠前直查即命中的站走此形态）
		"video.x264": {"X264", "x264", "X.264", "x.264"},
		"video.x265": {"X265", "x265", "X.265", "x.265"},
	}
	labels, ok := variants[stdKey]
	if !ok {
		return nil
	}
	for i, m := range cfg.ValueMappings[model.FieldDomainCodec] {
		for _, l := range labels {
			if m.Label == l {
				return &cfg.ValueMappings[model.FieldDomainCodec][i]
			}
		}
	}
	return nil
}

// audioMappingOf §59.166 A 层：TechProfile 源音频映射。
// §59.175 顺序定案：表单组合键【最优先】——§59.150 判据六（TrueHD+Atmos→
// 站点表单选项 "TrueHD Atmos"）。B1 直取曾置于组合键之前把组合键短路
// （→裸 audio.truehd → 幸运 LuckAudit AUDIO_MISMATCH 十连拒实证：
// "标题显示为TrueHD Atmos，但选择了TrueHD"）。组合键未命中自然落 B1
// （DD 场景行为不变）。注：表单选项的组合词条与标题形态无关——标题
// 规范（v1.05）Atmos 属对象信息在声道数后（TrueHD 7.1 Atmos），不变。
// domain 参数 §59.454 附: 城市站音频编码键配在 codec 域（混音视频单槽
// ——hdcityCodecOption 切换数据源复用本链）；常规站传 audiocodec 域。
func (e *PublishExecutor) audioMappingOf(cfg *model.PublishFormConfig, domain, audioCodec, audioTech string) *model.FormValueMapping {
	if audioCodec != "" && audioTech != "" {
		combined := extract.LookupStandardKey("audio_codec", audioCodec+" "+audioTech)
		if m := e.lookupByStdKey(cfg, domain, combined); m != nil {
			return m
		}
	}
	// §59.166 B1：canonical→词条 standard_key 直取（"DD" 词表 exact 失明修复）
	if sk := titleparser.AudioStandardKey(audioCodec); sk != "" {
		if m := e.lookupByStdKey(cfg, domain, sk); m != nil {
			return m
		}
	}
	m := e.lookupByStdKey(cfg, domain, extract.LookupStandardKey("audio_codec", audioCodec))
	if m != nil {
		return m
	}
	// §59.273: 音频域 Other 兜底（Opus/ALAC/DSD 等站方无选项——幸运种审
	// "未选择音频" Robot Dreams AV1 Opus 案）
	return e.lookupOtherOption(cfg, domain)
}

// audioMapping 音频域（组合键优先：TrueHD+Atmos→"TrueHD Atmos"——§59.150 判据六）。

// mediumMappingOf §59.166 A 层：TechProfile 源媒介映射（标题纠错终态——
// WEBRip/WEB-DL/Encode 规格优先 §59.150 二维规则）。

// hasMinBDToken §59.325: 标题 MinBD 词形检测（MinBD/MiniBD/MINBD——
// §59.213 变体归一，lower contains 单词覆盖）——站点映射层特例：
// 有 MiniBD 精确选项的站点优先映射（True.Grit MinBD1080p 案站方
// TYPE_MISMATCH——canonical 判 Encode 正确，站点选项更精确）。
func hasMinBDToken(rawTitle string) bool {
	lower := strings.ToLower(rawTitle)
	// MinBD（minbd）与 MiniBD（minibd）互不为子串——双词形检测
	return strings.Contains(lower, "minbd") || strings.Contains(lower, "minibd")
}

// §59.427 签名扩展：siteName+meta（好多油 TV 双轨分发——站方媒介
// 电影/剧集分立项，分发依据 category=tv_series）。
func (e *PublishExecutor) mediumMappingOf(cfg *model.PublishFormConfig, siteName string, meta *model.TorrentMetadata, tp titleparser.TechProfile) *model.FormValueMapping {
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
	// §59.380: stdKey 推导抽单源（codec 压制形态联动复用——防双实现漂移）
	stdKey = mediumStdKeyOf(tp, stdKey)
	// §59.427: 好多油 TV 双轨分发——站方媒介 Remux/Encode/WEB-DL 各带
	// TV 后缀分立项（Remux TV 12/UHD Remux TV 16/Encode TV 14/WEB-DL TV
	// 13）：剧集（tv_series）种分发 TV 项（电影形态主键归 Remux/Encode/
	// WEB-DL——TV 项 label-only 特化独占，直查永不命中防撞键）。
	// 分发依据=stdKey（媒介单源同源）；非 tv_series 零变化。
	if v := hduptTVMediumOption(cfg, siteName, meta, stdKey); v != nil {
		return v
	}
	// §59.325: MinBD 特例优先——有 MiniBD 精确选项的站点映 medium.minibd，
	// miss 时 fallback 原链（无该选项的站点走 Encode/原盘原语义）
	if hasMinBDToken(tp.RawTitle) {
		if m := e.lookupByStdKey(cfg, model.FieldDomainMedium, "medium.minibd"); m != nil {
			return m
		}
	}
	// §59.407: DIY 原盘媒介联动——原盘族（bluray/uhd_bluray）+ DIY 信号在场
	// 且站方有 DIY 媒介细分项时优先匹配（六站受益：库非/壹吧/咖啡/五二/
	// 冬樱/海的宝；触发条件动态检测非站名硬编码——未来站点有 DIY 项自动
	// 生效零新代码，§59.380 联动同款通用投资）。DIY 信号=tags 列 tag.diy
	// OR 标题 DIY 词形（双源）；DIY Remux 不触发（用户定案——六站均无
	// Remux DIY 项，信号+Remux 键照旧选 Remux）；无 DIY 项/无信号/
	// 非原盘媒介三重零触发面。
	if v := diyMediumOption(cfg, tp, stdKey); v != nil {
		return v
	}
	m := e.lookupByStdKey(cfg, model.FieldDomainMedium, stdKey)
	// §59.419: 压制媒介按分辨率细分匹配（直查 miss 后 fallback——
	// 站方按分辨率细分压制项：红豆饭 20/24/25/26 式四分）
	if m == nil {
		m = resolutionEncodeOption(cfg, tp, stdKey)
	}
	return m
}

// diyMediumOption §59.407: DIY 原盘媒介细分匹配（站方 DIY 项 label 直达）。
// 匹配形态：各站 label 词形不一（"Blu-ray DIY"/"BD DIY"/"Blu-ray/DIY"/
// "UHD Blu-ray/DIY"/"4K UHD DIY"）——统一按 UHD/4K 关键词分两族：
// label 含 (UHD 或 4K) → UHD DIY 族；其余 → BD DIY 族；族内取首项
// （数组序即站方表单序）。种子媒介键 uhd_bluray→UHD 族 / bluray→
// BD 族。nil=不触发（零信号/非原盘族/站方无 DIY 项任一）。
func diyMediumOption(cfg *model.PublishFormConfig, tp titleparser.TechProfile, stdKey string) *model.FormValueMapping {
	if cfg == nil || (stdKey != "medium.bluray" && stdKey != "medium.uhd_bluray") {
		return nil
	}
	// DIY 信号双源：标题 DIY 词形 OR RawTitle（TechProfile 无 tags 列——
	// tags 由调用方 meta 传入场景少；标题词形是稳定主源，fetch 推断
	// tag.diy 的上游依据也是标题/副标题词形——双源在标题侧收敛）
	if !strings.Contains(strings.ToUpper(tp.RawTitle), "DIY") {
		return nil
	}
	wantUHD := stdKey == "medium.uhd_bluray"
	items := cfg.ValueMappings[model.FieldDomainMedium]
	for i := range items {
		if !strings.Contains(strings.ToUpper(items[i].Label), "DIY") {
			continue
		}
		up := strings.ToUpper(items[i].Label)
		isUHDFamily := strings.Contains(up, "UHD") || strings.Contains(up, "4K")
		if isUHDFamily == wantUHD {
			return &items[i]
		}
	}
	return nil
}

// resolutionEncodeOption §59.419: 压制媒介按分辨率细分匹配（站方表单
// 按分辨率细分压制项——红豆饭 §5.1 站方权威规则：Encode+1080p→
// "1080P/i压制"/720p→"720P压制"/SD→MiniSD/UHD(4K)→"UHD压制"）。
// §59.387 库非挂账（resolution×medium 联动零案例不预实施）第二站
// 实施——站方文档规则锚+通用动态检测（label 压制/Encode+分辨率词形），
// 未来站自动生效（§59.407 DIY 联动同款通用投资）。
// 挂载位：mediumMappingOf 直查 miss 后 fallback（站方有通用 Encode 项
// 的站直查已命中不进此函数语义；双保险再排除 label 精确 "Encode" 项）。
// 分发：r2160p/r4320p→UHD 族；r1080p/r1080i→1080 族；r720p→720 族；
// sd/其余→SD/MiniSD 族；目标族缺失按 UHD→1080→720→SD 降级链兜底
// （族内取首项——数组序=站方表单序）。
// nil=不触发（无细分项站零变化——库非 18 通用兜底不受影响）。
func resolutionEncodeOption(cfg *model.PublishFormConfig, tp titleparser.TechProfile, stdKey string) *model.FormValueMapping {
	if cfg == nil || stdKey != "medium.encode" {
		return nil
	}
	resKey := extract.LookupStandardKey("resolution", tp.Resolution)
	if resKey == "" {
		return nil
	}
	items := cfg.ValueMappings[model.FieldDomainMedium]
	for i := range items {
		if strings.EqualFold(strings.TrimSpace(items[i].Label), "Encode") {
			return nil // 站方有通用 Encode 项——通用兜底优先，特例不触发
		}
	}
	var uhdOpt, opt1080, opt720, sdOpt *model.FormValueMapping
	for i := range items {
		up := strings.ToUpper(items[i].Label)
		if !strings.Contains(up, "压制") && !strings.Contains(up, "ENCODE") &&
			!strings.Contains(up, "MINISD") && !strings.EqualFold(strings.TrimSpace(up), "SD") {
			continue
		}
		if strings.Contains(up, "DIY") { // 压制族不含 DIY 项（原盘 DIY 联动另管）
			continue
		}
		switch {
		case strings.Contains(up, "UHD") || strings.Contains(up, "4K"):
			if uhdOpt == nil {
				uhdOpt = &items[i]
			}
		case strings.Contains(up, "1080"):
			if opt1080 == nil {
				opt1080 = &items[i]
			}
		case strings.Contains(up, "720"):
			if opt720 == nil {
				opt720 = &items[i]
			}
		case strings.Contains(up, "MINISD") || strings.Contains(up, "SD"):
			if sdOpt == nil {
				sdOpt = &items[i]
			}
		}
	}
	if uhdOpt == nil && opt1080 == nil && opt720 == nil && sdOpt == nil {
		return nil // 非细分站（无任何压制分辨率细分项）零触发
	}
	var want string
	switch resKey {
	case "resolution.r2160p", "resolution.r4320p":
		want = "uhd"
	case "resolution.r1080p", "resolution.r1080i":
		want = "1080"
	case "resolution.r720p":
		want = "720"
	default:
		want = "sd"
	}
	// 目标族优先，缺失按 UHD→1080→720→SD 降级链兜底
	for _, cand := range []struct {
		key string
		opt *model.FormValueMapping
	}{
		{"uhd", uhdOpt}, {"1080", opt1080}, {"720", opt720}, {"sd", sdOpt},
	} {
		if cand.opt != nil && cand.key == want {
			return cand.opt
		}
	}
	for _, cand := range []*model.FormValueMapping{uhdOpt, opt1080, opt720, sdOpt} {
		if cand != nil {
			return cand
		}
	}
	return nil
}

// mediumStdKeyOf §59.380: 媒介 canonical 键推导单源（mediumMappingOf 主链
// 与 codec 压制形态联动共用）。preset 非空（规格优先路径）直接返回；
// 空则 IsEncode 铁证（§59.166 回归补：MI Writing library+碟源+编码族
// 二维 §59.151）→ encode；否则 SourceType 归一。
func mediumStdKeyOf(tp titleparser.TechProfile, preset string) string {
	if preset != "" {
		return preset
	}
	if titleparser.IsEncode(tp) {
		return "medium.encode"
	}
	return extract.LookupStandardKey("medium", tp.SourceType)
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
	// §59.325: MinBD 特例优先（同 mediumMappingOf）
	if hasMinBDToken(meta.Title) {
		if m := e.lookupByStdKey(cfg, model.FieldDomainMedium, "medium.minibd"); m != nil {
			return m
		}
	}
	if stdKey == "" {
		// Encode 二维：碟转压（片源碟+编码痕迹）——IsEncode 铁证
		if titleparser.IsEncode(titleparser.TechProfile{
			SourceType:    meta.SourceType,
			Specification: meta.Specification,
			VideoCodec:    meta.VideoCodec,
			// §59.319 附三十二: RawTitle 必传（重编码排除扫原始标题——
			// 雀起乡 Blu-ray.x265 连字符压制写法发布链错映原盘）
			RawTitle: meta.Title,
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
	// §59.273: 制作组域 Other 兜底（组名存在但站方无选项——统一"无可选项
	// 均选 Other"用户定案；组名空仍 nil 不强填）
	return e.lookupOtherOption(cfg, model.FieldDomainTeam)
}

// TechTextOf §59.319 P2: 原盘 tech 文本源统一——BDInfo 优先，普通种子
// MediaInfo（SourceMediaInfo 兜底由调用方语境决定，发布链只消费本地列——
// 与 FetchAndStore 落库语义一致）。表单 techinfo 域/BuildTechProfile/标签
// 推断/审计 body/描述 quote 五消费点同源。ExtractMediaInfo 格式分流
// （QUICK SUMMARY 锚）保证 BDInfo 文本自动走 BD 解析器。
// §59.324: 源站采集的 BDInfo/MI 文本常裹 [code] BBCode 块（True.Grit
// MinBD 案——站方 MEDIAINFO_CONTAINS_BBCODE 拒收）——发布链输出剥
// code 标记对（描述域的 [code] 是合法 BBCode 不受影响，仅此处剥）。
func TechTextOf(meta *model.TorrentMetadata) string {
	raw := ""
	if meta.BDInfo != "" {
		raw = meta.BDInfo
	} else {
		raw = meta.MediaInfo
	}
	if strings.Contains(raw, "[code]") || strings.Contains(raw, "[/code]") {
		raw = strings.ReplaceAll(raw, "[code]", "")
		raw = strings.ReplaceAll(raw, "[/code]", "")
	}
	return raw
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
		SmallDescr:    form[model.FieldDomainSmallDescr],
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

// §59.399: IMDb/豆瓣编号提取（学校站 imdbid/doubanid 编号形态字段）。
// IMDb ID=tt+数字（URL 任意位置匹配——尾部锚定不必要，tt 前缀即无歧义锚）；
// 豆瓣 ID=subject/ 后数字（路径锚定防误匹配非条目页）。DB 值已是纯编号
// 形态（Tab4 用户手输 §59.341）直出兼容；形态不匹配返回空串（setForm
// skip 不填——站方非必填不阻断）。
var reIMDbID = regexp.MustCompile(`tt[0-9]+`)
var reDoubanID = regexp.MustCompile(`subject/([0-9]+)`)

func extractIMDbID(url string) string {
	url = strings.TrimSpace(url)
	if m := reIMDbID.FindString(url); m != "" {
		return m
	}
	return ""
}

func extractDoubanID(url string) string {
	url = strings.TrimSpace(url)
	if m := reDoubanID.FindStringSubmatch(url); len(m) > 1 {
		return m[1]
	}
	// 纯数字直出（用户手输纯编号形态——无 URL 壳）
	if url != "" {
		for _, r := range url {
			if r < '0' || r > '9' {
				return ""
			}
		}
		return url
	}
	return ""
}

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
	// §59.491: 协议根修——裸 Domain 拼接产出无协议 URL（publish_url 落库
	// "www.musopia.vip/..."）·前端 href 相对路径拼系统地址。PT 站全 https·
	// 已带协议的 Domain 原样（防御双形态）。
	d := strings.TrimRight(cfg.Domain, "/")
	if !strings.HasPrefix(d, "http://") && !strings.HasPrefix(d, "https://") {
		d = "https://" + d
	}
	return d + "/details.php?id=" + tid
}

// preAuditTitle §59.166 B2 补：预检 name 与上传标题同源（tp 重组 MI 纠错终态
// ——Arco 案预检层旧标题与表单 DDP 对照错位残留）。
// §59.254 项4：重组空返回 ok=false（DryRun 同拒——预检正是发现问题的工具）。
func (e *PublishExecutor) preAuditTitle(meta *model.TorrentMetadata) (string, bool) {
	// §59.514 附八: 优先消费重组标题资产列（§59.304 双链分叉防——主管线
	// 锚定列·兜底链同锚定·列空才降级现算）
	if meta.ReassembledTitle != "" {
		return meta.ReassembledTitle, true
	}
	domMedium, domRes, domVideo, domAudio := titleparser.DOMFieldsFromDetailSource(meta.DetailSourceJSON)
	tp := titleparser.BuildTechProfile(meta.Title, TechTextOf(meta), domMedium, domRes, domVideo, domAudio)
	e.normalizeSeasonPackOf(meta, &tp) // §59.308: 预检 name 与上传标题同归一（双链分叉防）
	if rt := titleparser.ReassembleFromTechProfile(tp, titleparser.V105TitleFormat()); strings.TrimSpace(rt) != "" {
		return rt, true
	}
	return "", false
}

// parseTagsAssets §59.311: meta.Tags JSON 数组 → canonical 键列表（Tab1 ③ 同源）。
func parseTagsAssets(raw string) []string {
	if raw == "" {
		return nil
	}
	var arr []string
	if err := json.Unmarshal([]byte(raw), &arr); err != nil {
		return nil
	}
	return arr
}

// buildTagsLabels §59.312: 标签投递值→站方 label（ValueMappings 反查；
// miss 兜底原值——与 tags 索引对齐）。
func buildTagsLabels(cfg *model.PublishFormConfig, applied []string) []string {
	if cfg == nil || len(applied) == 0 {
		return nil
	}
	out := make([]string, 0, len(applied))
	for _, v := range applied {
		label := v
		for _, m := range cfg.ValueMappings[model.FieldDomainTags] {
			if m.Value == v {
				label = m.Label
				break
			}
		}
		out = append(out, label)
	}
	return out
}

// buildFormLabels §59.310: DryRun 表单组装可读化——字段名（站方 input name）反查
// 域中文名 + 值（站方枚举 value）反查 label（ValueMappings 全域扫）。
// "audiocodec_sel[4]"="12" → "音频编码 · DTS"；文本域（无枚举）只带域名。
func buildFormLabels(cfg *model.PublishFormConfig, form map[string]string) map[string]string {
	if cfg == nil || len(form) == 0 {
		return nil
	}
	// 域中文名（domain → 中文）
	domainNames := map[string]string{
		model.FieldDomainType: "类型", model.FieldDomainStandard: "规格",
		model.FieldDomainCodec: "编码", model.FieldDomainAudiocodec: "音频编码",
		model.FieldDomainTeam: "制作组", model.FieldDomainMedium: "媒介",
		model.FieldDomainSmallDescr: "副标题", model.FieldDomainDescription: "简介",
		model.FieldDomainTechInfo: "技术信息", model.FieldDomainIMDBURL: "IMDb",
		model.FieldDomainPTGen: "豆瓣链接", model.FieldDomainDoubanURL: "豆瓣链接",
		model.FieldDomainIMDbID: "IMDb编号", model.FieldDomainDoubanID: "豆瓣编号",
		model.FieldDomainCNName: "中文名", model.FieldDomainUplver: "匿名发布", model.FieldDomainOffer: "候选发布",
	}
	// 字段名 → 域名（FormFields 反查）
	fieldToDomain := map[string]string{}
	for domain, field := range cfg.FormFields {
		if field != "" {
			fieldToDomain[field] = domain
		}
	}
	out := make(map[string]string, len(form))
	for field, value := range form {
		domain, ok := fieldToDomain[field]
		if !ok {
			// 站方特有字段无域名（如 url_[r]/DryRun [gpw] 前缀预览键）——
			// 跳过不进 formLabels（Form 主体仍原键可见，此处无需重复）
			continue
		}
		dn := domainNames[domain]
		if dn == "" {
			dn = domain
		}
		// 值 → label（下拉域）
		if ms, ok := cfg.ValueMappings[domain]; ok {
			for _, m := range ms {
				if m.Value == value {
					out[field] = dn + " · " + m.Label
					break
				}
			}
		}
		if out[field] == "" {
			out[field] = dn
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// normalizeSeasonPackOf §59.308: 完结季包季集归一单点——A 通道（meta.Tags 含
// complete——副标题"全N集"/源站 tag 提取，完结声明权威在标签域）或 B 通道
// （PTGen episodes=区间末集）命中时 E01 起始集区间归一纯季号（v1.05 单季合集=S01；
// 海的开始案 TAGS_COMPLETE_NO_SERIES_ALLOWED 根治）。主管线与 preAuditTitle 共用。
func (e *PublishExecutor) normalizeSeasonPackOf(meta *model.TorrentMetadata, tp *titleparser.TechProfile) {
	if meta == nil || tp == nil {
		return
	}
	// §59.514 通道 C: 副标「全N集/N集全」并入 tagsComplete 同权（信号集
	// 不对称消除——副标词形此前只在 §59.424 门放行不进归一，Book 型
	// episode_split 误标场景 E01 形态漏网）
	tagsComplete := strings.Contains(meta.Tags, "\"complete\"") || subtitleCompleteSignal(meta.Subtitle)
	ptgenEps := 0
	if src, err := metadata.UnmarshalPTGenSource(meta.PTGenSourceJSON); err == nil && src != nil {
		if v, perr := strconv.Atoi(strings.TrimSpace(src.Episodes)); perr == nil {
			ptgenEps = v
		}
	}
	tp.SeasonEpisode = titleparser.NormalizeSeasonPack(tp.SeasonEpisode, tagsComplete, ptgenEps)
}

// crossSeasonTagRe §59.177: 跨季标题形态（S01-S02 / 第N-M季）。
var crossSeasonTagRe = regexp.MustCompile(`(?i)(?:[.\s]S\d{1,2}-S?\d{1,2}\b|第\d+-\d+季)`)

// multiWorkCollectionRe §59.177: 多作品合集信号（副标题）。
var multiWorkCollectionRe = regexp.MustCompile(`(合集|集全|期全)`)

// completeSignalRe §59.177: 完结信号（副标题）。§59.424 扩英文词形
// （用户定案中英双语——\bcomplete\b 词边界防误匹配 incomplete）。
var completeSignalRe = regexp.MustCompile(`(\b(?i:COMPLETE)\b|全集|全季|已完结|完结|全\d+集|全剧终)`)

// mainTitleCompleteRe §59.424: 主标题完结信号（中英双语词形——
// Complete Seasons/Series/Collection 全形态经 \bcomplete\b 命中）。
var mainTitleCompleteRe = regexp.MustCompile(`(\b(?i:COMPLETE)\b|完结|全集|全季)`)

// seasonPackFormRe §59.424: 合集形态季集（纯季 S01 / 跨季 S01-S02 /
// 第N-M季——无 E 后缀）。归一通道（§59.308）命中的完整季已是纯季号。
var seasonPackFormRe = regexp.MustCompile(`^S\d{1,2}(-S?\d{1,2})?$`)

// singleEpisodeRe §59.424: 单集形态（S01E02——无区间后缀）。§59.514
// 刀1: 裸 Exx 兼容（(?:S\d{1,2})? 可选——Quacks E01 案形态；无信号裸
// 单集=真分集该拦·此前裸形态提取失明走「无 S/E 信号」豁免漏拦）。
var singleEpisodeRe = regexp.MustCompile(`^(?:S\d{1,2})?E(\d{1,4})$`)

// haidanCrossSeasonRe §59.472: 跨季形态（S01-S02 / S01-02 尾季无 S 短形态·
// 单位数 S1-2）——海胆 collages 合集勾选判定。
var haidanCrossSeasonRe = regexp.MustCompile(`^S\d{1,2}-S?\d{1,2}$`)

// episodeReleaseBlocked §59.424: 系统级禁发剧集分集（用户定案——所有
// 站点仅允许合集/完结/全集形态发布；与"转存发布禁勾首发"同层行为铁律，
// 纯代码规则零配置）。
// 范围：category ∈ {tv_series, animation, tv_shows}（②定案；动漫电影
// 形态无 S/E 信号天然豁免；短剧/音乐/MV/体育不在范围——边界定案④）。
// 放行（任一）：
//   - 无分集形态（无 S/E 信号，或 S01 纯季/跨季合集形态——归一通道
//     §59.308 命中的完整季此时已是纯季号）
//   - 完结信号任一（①定案宁放勿拦）：meta.Tags 含 complete（源站采集
//     标签①）∨ 主标题中英词形② ∨ 副标题中英词形直查③（P2 定案：
//     直查不依赖 fetch 推断产物——历史种/推断 miss 兜底）
//   - 全集一集：单集 E01 ∧ PTGen episodes=1（BBC 单剧形态——定案不禁）
// 拦截：单 E / 归一后仍是区间（非 E01 起或无完结信号的 E01 起区间）且
// 无任何完结信号。
// 边界（漏判放行·定案）：S01 标题权威（内容半季不判读文件清单）/
// 多区间复杂形态按第一组锚/源站误标 movie 等范围外豁免。
func episodeReleaseBlocked(meta *model.TorrentMetadata, tp *titleparser.TechProfile) (bool, string) {
	if meta == nil || tp == nil {
		return false, ""
	}
	switch meta.Category {
	case "category.tv_series", "category.animation", "category.tv_shows":
	default:
		return false, "" // 范围外（电影/短剧/音乐/MV/体育等）——豁免
	}
	se := tp.SeasonEpisode
	if se == "" {
		return false, "" // 无 S/E 信号（含动漫电影/OVA 形态）——豁免
	}
	if seasonPackFormRe.MatchString(se) {
		return false, "" // S01 纯季 / S01-S02 跨季——合集形态放行
	}
	// 单集/区间形态：完结信号集（任一即放——宁放勿拦）
	if strings.Contains(meta.Tags, `"complete"`) { // ①源站采集标签（归一 tag.complete）
		return false, ""
	}
	if mainTitleCompleteRe.MatchString(meta.Title) { // ②主标题中英词形
		return false, ""
	}
	if completeSignalRe.MatchString(meta.Subtitle) { // ③副标题中英词形直查（P2）
		return false, ""
	}
	// 全集一集：单集 E01 ∧ PTGen episodes=1（整部剧就一集——完整全集）
	if m := singleEpisodeRe.FindStringSubmatch(se); m != nil && m[1] == "01" {
		if src, err := metadata.UnmarshalPTGenSource(meta.PTGenSourceJSON); err == nil && src != nil {
			if v, perr := strconv.Atoi(strings.TrimSpace(src.Episodes)); perr == nil && v == 1 {
				return false, ""
			}
		}
	}
	return true, fmt.Sprintf("剧集分集形态（%s）——系统规则仅允许合集/完结/全集发布（放行信号：源站完结标签 / 主·副标题 Complete·完结词形 / 全集数对照）", se)
}

// crossSeasonCollectionSites §59.177: 覆写组——跨季=合集（Session Pack 语义）。
// 藏宝阁: 多季合集规范（S01-SXX 专门命名规范）; 优堡: 整季包/跨季包;
// 野马 §59.518 用户定案: 完结(12)=单季完结状态·完结全集(18)=多季打包。
// 新站实测冲突时加条目（两层架构——通用=幸运，差异按站扩展）。
// §59.518 键修正："ubits"（英文）→"优堡"（name 列中文真相）——原键自
// v0.0.895 起恒 miss，覆写组在优堡从未生效（Quacks/Ambassadors 站方审核
// 「完结剧集请添加合集标签」实锤）。
var crossSeasonCollectionSites = map[string]bool{
	"藏宝阁": true,
	"优堡":   true,
	"野马":   true,
}

// adjustTagsForSite §59.177: 发布时标签按目标站调整（两层架构的站点扩展层）。
// 通用规则=幸运语义：合集=多部不同作品的集合；S01-S02=单作品系列→完结。
// 覆写组（藏宝阁/优堡/野马）：S01-S02=多季合集→合集（去完结）。
// 用户意图保护：副标题含"合集/集全/期全"（真多作品合集）时调整层不动。
//
// §59.518 优堡特化：站方无「完结」标签（值集只有连载23/合集20）——合集=
// Session Pack（整季包/跨季包），完结剧一律转合集。complete 产出词形=剧终
// 语境（全集/全季/完结/全N集/全剧终/COMPLETE），电影副标不含→不误触发。
func adjustTagsForSite(tags []string, siteName, title, subtitle string) []string {
	has := func(t string) bool {
		for _, x := range tags {
			if x == t {
				return true
			}
		}
		return false
	}
	remove := func(t string) []string {
		out := make([]string, 0, len(tags))
		for _, x := range tags {
			if x != t {
				out = append(out, x)
			}
		}
		return out
	}

	// 用户意图保护——真多作品合集不调整（§59.518 前置：原在跨季闸门后，
	// 前置后非跨季场景语义等价——两分支都不动；优堡单季分支同受保护）
	if multiWorkCollectionRe.MatchString(subtitle) {
		return tags
	}

	// §59.518 优堡特化——完结剧→合集（站方审核实锤+值集无完结）。
	// 跨季+complete 在场也走此分支（去完结加合集·与覆写组结果收敛）；
	// 跨季+complete 缺席则落到下方覆写组补合集——双路径收敛。
	if siteName == "优堡" && has("complete") {
		adjusted := remove("complete")
		if !has("collection") {
			adjusted = append(adjusted, "collection")
		}
		return adjusted
	}

	if !crossSeasonTagRe.MatchString(title + " " + subtitle) {
		return tags
	}

	if crossSeasonCollectionSites[siteName] {
		// 覆写组：跨季=合集（去完结，加合集）
		adjusted := remove("complete")
		if !has("collection") {
			adjusted = append(adjusted, "collection")
		}
		return adjusted
	}
	// 通用（幸运语义）：跨季=单作品系列（去合集，加完结）
	adjusted := remove("collection")
	if !has("complete") && completeSignalRe.MatchString(subtitle) {
		adjusted = append(adjusted, "complete")
	}
	return adjusted
}

// mergePTGenSource §59.254 项1: 第四源准备与合并（executor 侧）。
// 端点感知链：PTGenSourceJSON 内 aka 启发式（零请求）→ NeedsIMDbFallback
// 触发时 &imdb 二连（e.pipe.ptgen 独享豆影）→ MergePTGenInto。
// EnglishName 同时回写 meta.EnglishTitle（二连触发时权威值覆盖）。
func (e *PublishExecutor) mergePTGenSource(ctx context.Context, tp *titleparser.TechProfile, meta *model.TorrentMetadata) {
	if meta == nil {
		return
	}
	src, err := metadata.UnmarshalPTGenSource(meta.PTGenSourceJSON)
	if err != nil || src == nil {
		return // 无 PTGen 数据（增强层缺失回退三源现状）
	}
	englishName := ptgen.ExtractEnglishName(&src.PTGenResult, src.Source)
	if ptgen.NeedsIMDbFallback(tp.MainTitle, englishName) && e.pipe != nil && e.pipe.ptgen != nil && meta.DoubanURL != "" {
		if en, _ := e.pipe.ptgen.QueryIMDbForEnglish(ctx, meta.DoubanURL); en != "" {
			englishName = en
			// 权威值覆盖落库（EnglishTitle 资产）
			if err := e.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
				Where("info_hash = ?", meta.InfoHash).
				Update("english_title", en).Error; err != nil {
				e.logger.Warn("english_title persist failed", zap.Error(err))
			}
		}
	}
	// 常规 EnglishTitle 尽力落库（aka 启发式——未触发二连时）
	if englishName != "" && meta.EnglishTitle == "" {
		_ = e.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
			Where("info_hash = ?", meta.InfoHash).
			Update("english_title", englishName).Error
	}
	titleparser.MergePTGenInto(tp, titleparser.PTGenMergeInput{
		EnglishName: englishName,
		Year:        src.Year,
	}, func(msg string, kv ...any) {
		// 可观测日志（info 级——year_corrected/main_title_backfilled/title_mismatch）
		fields := []zap.Field{zap.String("info_hash", meta.InfoHash)}
		for i := 0; i+1 < len(kv); i += 2 {
			if k, ok := kv[i].(string); ok {
				fields = append(fields, zap.Any(k, kv[i+1]))
			}
		}
		e.logger.Info(msg, fields...)
	})
}

// typeMappingOf §59.391: type 槽映射统一入口（产区站/媒介细分站特化+通用链）。
// 葡萄产区×类型双维分类（站方 type 27 项——电影按产区三分/剧集四分/
// 综艺四分）无法用 form_config 单键单值表达——产区类 label-only（防
// lookupByStdKey 撞错），本入口按 PTGen 产区现算（§59.380 codec 联动
// 同款跨槽特化先例）；非产区站/非产区类回退通用链零变化。
// §59.405: 好大媒介细分站（type 18 项——电影按媒介/分辨率细分 10 项）
// 同入口第二站；非电影类回退通用链。
func (e *PublishExecutor) typeMappingOf(cfg *model.PublishFormConfig, siteName string, meta *model.TorrentMetadata, tp titleparser.TechProfile) *model.FormValueMapping {
	if v := sjtuRegionCategoryValue(cfg, siteName, meta); v != nil {
		return v
	}
	if v := hdareaCategoryValue(cfg, siteName, meta, tp); v != nil {
		return v
	}
	if v := cbgCategoryValue(cfg, siteName, meta, tp); v != nil {
		return v
	}
	if v := hdhomeCategoryValue(cfg, siteName, meta, tp); v != nil {
		return v
	}
	if v := hdskyCategoryValue(cfg, siteName, meta); v != nil {
		return v
	}
	if v := discfanCategoryValue(cfg, siteName, meta); v != nil {
		return v
	}
	if v := hdtimeCategoryValue(cfg, siteName, meta, tp); v != nil {
		return v
	}
	if v := itzmxCategoryValue(cfg, siteName, meta, tp); v != nil {
		return v
	}
	if v := e.mteamCategoryValue(cfg, siteName, meta, tp); v != nil {
		return v
	}
	// §59.511: 瓷器 type 混维路由（第十一站——4K 大桶/电影媒介分桶/
	// 剧集产区轴·直映射类回退通用链）
	if v := e.hdwingCategoryValue(cfg, siteName, meta, tp); v != nil {
		return v
	}
	return e.lookupByStdKey(cfg, model.FieldDomainType, meta.Category)
}

// mteamCategoryValue §59.451: 馒头（api.m-team.cc）type 槽三维融合路由
// （typeMappingOf 第十站——好大模式同族：category 树=类型×媒介×分辨率
// 融合，22 非成人叶中影视族 9 叶按媒介/分辨率细分）。API 值集 2026-11-02
// 实时拉取（List API 八端点——team 3 增 1 删外其余零漂移）：
//   - 电影族（parent=100）：401 SD/419 HD/420 DVDiSo/421 BluRay/439 Remux
//   - 影剧/综艺族（parent=105，含剧集）：402 HD/403 SD/435 DVDiSo/438 BluRay
//     ——影剧族无 Remux 叶（Remux 归 402 HD——站方树实态）
//   - 动画 405+453 BluRay（原盘特化——discCategoryRoute 单类形态）
//   - 其余 13 类标准键直配（通用链：404 纪录/406 演唱/407 运动/434 无损/
//     409 其他/422 软件/427 电子书/442 有声书/451 教育/423 PC 游戏）。
// 媒介判定源=mediumMappingOf 结果的 StandardKeys（§59.380 单源同判定链
// ——首版误用 mediumStdKeyOf 弱判定：SourceType 归一损失致 REMUX 标题判
// bluray→421 误路由（龙PT 归一损失同款），DryRun 形态三/四抓出即修）：
//   原盘族（bluray/uhd_bluray/hddvd 键）→电影 421/影剧 438；dvd/dvdr→
//   420/435；remux/uhd_remux→439/402（影剧无 Remux 叶归 HD）；
//   分发形态（encode/webdl/webrip/hdtv/其它/miss）→分辨率 sd→401/403，
//   ≥720p→419/402（default 兜底）。
// 影视族 9 叶+453 全 label-only（路由独占——通用链永不命中影视叶）；成人
// 15 叶不录（发布层成人拦截在前 §59.222 不可达）。
func (e *PublishExecutor) mteamCategoryValue(cfg *model.PublishFormConfig, siteName string, meta *model.TorrentMetadata, tp titleparser.TechProfile) *model.FormValueMapping {
	if siteName != "馒头" || cfg == nil || meta == nil {
		return nil
	}
	// 媒介族判定：mediumMappingOf 判定结果（与 medium 槽同源——归一损失
	// 免疫：REMUX 规格优先路径正确判 remux）
	mediumKey := ""
	if m := e.mediumMappingOf(cfg, siteName, meta, tp); m != nil {
		for _, k := range m.StandardKeys {
			if strings.HasPrefix(k, "medium.") {
				mediumKey = k
				break
			}
		}
	}
	// 动画×原盘族→453 BluRay（非原盘动画回退通用链 405）
	switch meta.Category {
	case "category.animation":
		switch mediumKey {
		case "medium.bluray", "medium.uhd_bluray", "medium.hddvd":
			for i := range cfg.ValueMappings[model.FieldDomainType] {
				if cfg.ValueMappings[model.FieldDomainType][i].Value == "453" {
					return &cfg.ValueMappings[model.FieldDomainType][i]
				}
			}
		}
		return nil
	}
	// 影视族×媒介×分辨率
	isMovie := meta.Category == "category.movie"
	isTV := meta.Category == "category.tv_series" || meta.Category == "category.tv_shows"
	if !isMovie && !isTV {
		return nil
	}
	var target string
	switch mediumKey {
	case "medium.bluray", "medium.uhd_bluray", "medium.hddvd":
		target = "421" // BluRay 原盘
		if isTV {
			target = "438"
		}
	case "medium.dvd", "medium.dvdr":
		target = "420" // DVDiSo
		if isTV {
			target = "435"
		}
	case "medium.remux", "medium.uhd_remux":
		target = "439" // Remux（影剧族无 Remux 叶→402 HD）
		if isTV {
			target = "402"
		}
	default:
		// 分发形态（encode/webdl/webrip/hdtv/其它/miss）——分辨率二分
		target = "419" // HD
		if isTV {
			target = "402"
		}
		if extract.LookupStandardKey("resolution", tp.Resolution) == "resolution.sd" {
			target = "401" // SD
			if isTV {
				target = "403"
			}
		}
	}
	for i := range cfg.ValueMappings[model.FieldDomainType] {
		if cfg.ValueMappings[model.FieldDomainType][i].Value == target {
			return &cfg.ValueMappings[model.FieldDomainType][i]
		}
	}
	return nil
}

// hdtimeCategoryValue §59.426: 时光（hdtime.org）type 槽原盘形态路由
// （typeMappingOf 第七站——媒介维度决定 type，好大模式同族）。站方 424
// Blu-Ray原盘为独立分类（非质量选项）——站内实态仲裁（424 列表页 62 种
// 全原盘标题实证：Blu-ray AVC 原盘资源发 424 而非 401+medium Blu-ray）。
// 路由：影视类 ∧ 原盘族媒介（bluray/uhd_bluray/hddvd——mediumStdKeyOf
// §59.380 单源同款；Remux/Encode 非原盘不触发）→424；其余回退通用链
// （401 电影/402 剧集/... 标准键直配）。音乐 CD/Track 等非"原盘形态"
// 不触发（CD=音乐介质）。424 label-only（特化独占——通用链永不命中）。
// discCategoryRoute §59.428: 原盘独立分类公共路由（时光 424/分享站 414——
// 两站同构形态：站方"原盘/蓝光"为独立 type 分类，站内实态全原盘标题）。
// 判定：影视类 ∧ 原盘族（bluray/uhd_bluray/hddvd——mediumStdKeyOf §59.380
// 单源；Remux/Encode 非原盘不触发）→ target 分类；其余回退通用链。
func discCategoryRoute(cfg *model.PublishFormConfig, meta *model.TorrentMetadata, tp titleparser.TechProfile, target string) *model.FormValueMapping {
	if cfg == nil || meta == nil {
		return nil
	}
	switch meta.Category {
	case "category.movie", "category.tv_series", "category.animation",
		"category.documentary", "category.documentaries", "category.tv_shows",
		"category.sports", "category.mv", "category.music_video":
	default:
		return nil // 非影视类（音乐/游戏/软件/文档等）回退通用链
	}
	switch mediumStdKeyOf(tp, "") {
	case "medium.bluray", "medium.uhd_bluray", "medium.hddvd":
		// 原盘族（含 DIY 原盘——站方无 DIY 媒介细分；Remux 非原盘不触发）
	default:
		return nil
	}
	for i := range cfg.ValueMappings[model.FieldDomainType] {
		if cfg.ValueMappings[model.FieldDomainType][i].Value == target {
			return &cfg.ValueMappings[model.FieldDomainType][i]
		}
	}
	return nil
}

// mteamLabelsBits §59.451: 馒头 labels 位掩码翻译（tags canonical 键 →
// int 位掩码——站方 API 文档 TorrentUploadForm.labels 定义：bit0=DIY/
// bit1=中配（国语配音）/bit2=中字）。键集白名单精确匹配（diy/
// chinese_audio/chinese_subtitle）——其余标签（HDR/DV 等）馒头无对应
// bit 不翻译（tags 自定义字符串字段本发布链不投递）。
func mteamLabelsBits(tags []string) int {
	bits := 0
	for _, t := range tags {
		switch t {
		case "diy":
			bits |= 1 << 0
		case "chinese_audio":
			bits |= 1 << 1
		case "chinese_subtitle":
			bits |= 1 << 2
		}
	}
	return bits
}

// hdcityPrefill §59.454: 城市站（hdcity.city）jobs 段特化注入
// （标签无关项）——两步上传站的表单特化（用户逐点讨论定案）。
//
// ① bigname/small_descr 双填副标题（站方实态——t-68751/t-68754 双行
// 同值实证；空态拼装退化"译名 | MainTitle | Year"——PT 圈副标题
// 共识 0% 场景纯防御）
// ② tag1ing 题材注入（hdcityTag1ingValue——◎类别 行 100% 必有
// （29 库 315/315 实证），财神 csptGenreTagValue 增强版——27 项
// 题材映射表+行内多题材按序取首个命中）
// ③ processing_sel 3D 特化（hdcityProcessingOption——HOU→1/
// HSBS→2/全幅→5 Alt/空→不投递；交织(3)/红蓝(4)无提取词形预览人工）
//
// 标签依赖项（chnize/sub_hard/sub_soft/sub_incl）在 tags 产出后
// hdcityTagDependentFill 处理。
// yemaRegionID §59.459: 野马 region PTGen 产区中文→数字值全表
// （wiki 枚举权威 8 项——大陆/港/台分立/欧洲聚合/其它兜底）。
var yemaRegionID = map[string]string{
	"大陆": "1", "中国大陆": "1", "香港": "2", "中国香港": "2",
	"台湾": "3", "中国台湾": "3", "美国": "4", "日本": "6", "韩国": "7",
	// 欧洲聚合（英法德意西俄等→EU=5）
	"英国": "5", "法国": "5", "德国": "5", "意大利": "5", "西班牙": "5",
	"俄罗斯": "5", "新西兰": "5", "加拿大": "5", "澳大利亚": "5",
}

// yemaPublishInject §59.459-②③: 野马 attributes 数据源注入——
// regionList（PTGen 产区合拍多产区全投 ≤3——野马数组语义比肉丝单选
// 表达力强；miss 不投）+translated（中文译名 shortDesc 兜底）+poster。
func yemaPublishInject(form map[string]string, meta *model.TorrentMetadata) {
	regions := []string{}
	if src, err := metadata.UnmarshalPTGenSource(meta.PTGenSourceJSON); err == nil && src != nil {
		for _, raw := range src.Region {
			for _, part := range strings.Split(raw, "/") {
				w := strings.TrimSpace(part)
				if id, ok := yemaRegionID[w]; ok {
					dup := false
					for _, e := range regions {
						if e == id {
							dup = true
							break
						}
					}
					if !dup {
						regions = append(regions, id)
					}
				}
				if len(regions) >= 3 {
					break
				}
			}
			if len(regions) >= 3 {
				break
			}
		}
	}
	if len(regions) > 0 {
		if b, err := json.Marshal(regions); err == nil {
			form["yema_regions"] = string(b)
		}
	}
	if src, err := metadata.UnmarshalPTGenSource(meta.PTGenSourceJSON); err == nil && src != nil && len(src.TranslatedTitles) > 0 {
		form["yema_translated"] = string(src.TranslatedTitles)
	}
	form["yema_poster"] = meta.Poster
}

// rousiRegionSlug §59.458: 肉丝 region label→slug 全表（categories API
// 2026-11-03 实时权威 18 项）。
var rousiRegionSlug = map[string]string{
	"大陆": "mainland-china", "香港": "hong-kong", "台湾": "taiwan",
	"日本": "japan", "韩国": "south-korea", "美国": "usa", "英国": "uk",
	"法国": "france", "德国": "germany", "意大利": "italy", "西班牙": "spain",
	"俄罗斯": "russia", "新西兰": "new-zealand", "加拿大": "canada",
	"印度": "india", "泰国": "thailand", "澳大利亚": "australia",
	"其它": "other",
}

// rousiGenreSet §59.458: 站方 genre 词表（29 项中文——categories API
// 权威；与豆瓣题材词表同源，实测 60 样本 214 词全命中）。
var rousiGenreSet = map[string]bool{
	"剧情": true, "喜剧": true, "动作": true, "爱情": true, "科幻": true,
	"悬疑": true, "惊悚": true, "恐怖": true, "犯罪": true, "动画": true,
	"奇幻": true, "冒险": true, "灾难": true, "战争": true, "传记": true,
	"历史": true, "运动": true, "音乐": true, "歌舞": true, "家庭": true,
	"儿童": true, "纪录": true, "短片": true, "真人秀": true, "脱口秀": true,
	"西部": true, "武侠": true, "古装": true, "其它": true,
}

// rousiPublishInject §59.458-②③: 肉丝 attributes 数据源注入——
// genre（PTGen 题材数组剥 miss 后 JSON；全空兜"其它"——站方必填）
// +region（PTGen 产地 norm 三条+合拍首值优先（§59.391 葡萄先例同数据
// 源同策略）+label→slug 映射；miss 兜 other）+poster（封面 URL——
// adapter 窄例外下载 base64）。form 显示键（DryRun 可见）。
func rousiPublishInject(form map[string]string, meta *model.TorrentMetadata) {
	// genre
	genres := []string{}
	if src, err := metadata.UnmarshalPTGenSource(meta.PTGenSourceJSON); err == nil && src != nil {
		for _, g := range src.Genre {
			g = strings.TrimSpace(g)
			if g != "" && rousiGenreSet[g] {
				genres = append(genres, g)
			}
		}
	}
	if len(genres) == 0 {
		genres = []string{"其它"}
	}
	if b, err := json.Marshal(genres); err == nil {
		form["rs_genre"] = string(b)
	}

	// region（PTGen Region=[]string 合拍串形态——元素内再按 / 切）
	region := "other"
	if src, err := metadata.UnmarshalPTGenSource(meta.PTGenSourceJSON); err == nil && src != nil {
	outer:
		for _, raw := range src.Region {
			for _, part := range strings.Split(raw, "/") {
				w := strings.TrimSpace(part)
				if w == "" {
					continue
				}
				// norm 三条：中国大陆→大陆/中国香港→香港/中国台湾→台湾
				switch {
				case strings.HasPrefix(w, "中国大陆"):
					w = "大陆"
				case strings.HasPrefix(w, "中国香港"):
					w = "香港"
				case strings.HasPrefix(w, "中国台湾"):
					w = "台湾"
				}
				if slug, ok := rousiRegionSlug[w]; ok {
					region = slug
					break outer // 首值优先——命中即止（§59.391 葡萄先例）
				}
			}
		}
	}
	form["rs_region"] = region

	// poster（封面 URL——adapter §59.458 窄例外下载）
	form["rs_poster"] = meta.Poster
}

func hdcityPrefill(form map[string]string, cfg *model.PublishFormConfig, meta *model.TorrentMetadata, tp titleparser.TechProfile) {
	if form == nil || cfg == nil || meta == nil {
		return
	}
	// ① bigname/small_descr 双填副标题（空态拼装退化）
	subtitle := strings.TrimSpace(meta.Subtitle)
	if subtitle == "" {
		subtitle = hdcitySubtitleFallback(meta, tp)
	}
	if subtitle != "" {
		form["bigname"] = subtitle
		form["small_descr"] = subtitle
	}
	// ② tag1ing 题材注入
	if v := hdcityTag1ingValue(cfg, meta); v != "" {
		form["tag1ing"] = v
	}
	// ③ processing 3D（城市裸名无域常量——好学 processing_sel[4] 先例）
	if m := hdcityProcessingOption(cfg, tp); m != nil {
		form["processing_sel"] = m.Value
	}
}

// hdcitySubtitleFallback 副标题空态拼装退化（"译名 | MainTitle | Year"
// ——68754"骨湖 | Bone Lake | 2024"形态；译名空退化"MainTitle | Year"）。
func hdcitySubtitleFallback(meta *model.TorrentMetadata, tp titleparser.TechProfile) string {
	parts := []string{}
	if cn := chineseTitleOf(meta.Title); cn != "" {
		parts = append(parts, cn)
	}
	if tp.MainTitle != "" {
		parts = append(parts, tp.MainTitle)
	}
	if tp.Year != "" {
		parts = append(parts, tp.Year)
	}
	return strings.Join(parts, " | ")
}

// hdcityCodecOption §59.454: codec 域音频类切换（好大 blurayCodecOption
// 同款站方特化）——城市 codec_sel 混视频+音频 17 项单槽（§2.7 站方
// 编码-媒介对应表权威锚：FLAC→CD/Track、TrueHD/Atmos→BD/Remux/Encode）。
// 影视类走通用视频编码链零变化；音频类（type=408 Audio 或 medium 键
// CD/Track 族）改用音频编码数据源查同一 codec 域值集。
// hdcityCodecOption §59.454 附: codec 域音频类切换实现（e 方法——
// audioMappingOf 域参数化复用：城市 codec_sel 混音视频 17 项，音频类
// （category.music/lossless_music——408 Audio）改用音频编码数据源查
// 同一 codec 域；影视类返回 nil 走通用视频链零变化。
// 首版为残废函数（恒 nil）——回归审核抓出，AGENTS.md/docs 虚报即正。
func (e *PublishExecutor) hdcityCodecOption(cfg *model.PublishFormConfig, meta *model.TorrentMetadata, tp titleparser.TechProfile) *model.FormValueMapping {
	if cfg == nil || meta == nil {
		return nil
	}
	if meta.Category != "category.music" && meta.Category != "category.lossless_music" {
		return nil
	}
	return e.audioMappingOf(cfg, model.FieldDomainCodec, tp.AudioCodec, tp.AudioTechnology)
}

// hdcityTag1ingValue §59.454: 城市 tag1ing 题材注入（财神
// csptGenreTagValue 增强版）——◎类别 行 100% 必有（29 库 315/315），
// 行内多题材（"剧情 / 喜剧 / 动作"）按序取首个命中映射表的题材。
// PTGen 题材词→城市 tag1ing label 映射（27 项——"奇幻"→"魔幻/Fantasy"
// 语义对齐；"恐怖"城市无对应项跳过取行内下一词）。
var hdcityGenreMap = map[string]string{
	"动作": "动作/Action", "科幻": "科幻/Sci-Fi", "爱情": "爱情/Romance",
	"剧情": "剧情/Drama", "喜剧": "喜剧/Comedy", "家庭": "家庭/Family",
	"伦理": "伦理/Ethics", "惊悚": "惊悚/Thriller", "文艺": "文艺/Artistic",
	"冒险": "冒险/Adventure", "奇幻": "魔幻/Fantasy", "灾难": "灾难/Disaster",
	"犯罪": "犯罪/Crime", "悬疑": "悬疑/Mystery", "纪录": "记录/Documentary",
	"战争": "战争/War", "历史": "历史/History", "传记": "传记/Biography",
	"歌舞": "歌舞/Music", "动画": "动画/Animation", "西部": "西部/Western",
	"古装": "古装/Costume", "武侠": "武侠/仙侠", "运动": "运动/Sports",
	"微电影": "微电影/MiniVideo", "少儿不宜": "少儿不宜/Adult",
}

func hdcityTag1ingValue(cfg *model.PublishFormConfig, meta *model.TorrentMetadata) string {
	if meta == nil || meta.Description == "" {
		return ""
	}
	m := reGenreLine.FindStringSubmatch(meta.Description)
	if len(m) < 2 {
		return ""
	}
	// 行内题材词序列（"剧情 / 喜剧 / 动作"→[剧情,喜剧,动作]）
	for _, word := range strings.FieldsFunc(m[1], func(r rune) bool {
		return r == '/' || r == '、' || r == ' ' || r == '　' || r == ',' || r == '，'
	}) {
		word = strings.TrimSpace(word)
		if label, ok := hdcityGenreMap[word]; ok {
			// label → 城市 value（tag1ing 值=label 本身——字符串 value 下拉）
			return label
		}
	}
	return ""
}

// hdcityProcessingOption §59.454: 城市 processing_sel=3D 类型特化
// （用户定案）——tp.Stereo3D 映射：HOU→1(3D H-OU)/HSBS→2(3D H-SBS)/
// SBS·OU 全幅→5(3D Alt)/空→不投递（站方默认不选）。
// 交织(3)/红蓝(4)无标题提取词形——预览人工（罕见封装）。
func hdcityProcessingOption(cfg *model.PublishFormConfig, tp titleparser.TechProfile) *model.FormValueMapping {
	if cfg == nil {
		return nil
	}
	var target string
	switch tp.Stereo3D {
	case "HOU":
		target = "1"
	case "HSBS":
		target = "2"
	case "SBS", "OU":
		target = "5"
	default:
		return nil
	}
	for i := range cfg.ValueMappings["processing"] {
		if cfg.ValueMappings["processing"][i].Value == target {
			return &cfg.ValueMappings["processing"][i]
		}
	}
	return nil
}

// hdcityTagDependentFill §59.454: 城市站标签依赖注入——字幕
// chnize checkbox（用户定案：消费 Tab1 中字标签）。
// 直读 meta.Tags JSON（不经 assembleTags allowed 过滤——城市站无
// tags 域配置全滤实测首版抓出）。
func hdcityTagDependentFill(form map[string]string, tagsJSON string) {
	if form == nil || tagsJSON == "" {
		return
	}
	var tags []string
	if err := json.Unmarshal([]byte(tagsJSON), &tags); err != nil {
		return
	}
	for _, t := range tags {
		if t == "chinese_subtitle" {
			form["chnize"] = "1"
			return
		}
	}
}
func hxptLabelOption(items []model.FormValueMapping, label string) *model.FormValueMapping {
	for i := range items {
		if items[i].Label == label {
			return &items[i]
		}
	}
	return nil
}

// hxptOnlyDocumentary §59.453: 好学站点级拦截（用户定案）——学习资料站
// 仅纪录片（410 特别通道）可发；非纪录片 ineligible（DryRun 同拦预览可见
// ——碟粉模式同款）。分叉族双形态（单/复数）都放行。
func hxptOnlyDocumentary(meta *model.TorrentMetadata) (bool, string) {
	if meta == nil {
		return false, ""
	}
	switch meta.Category {
	case "category.documentary", "category.documentaries":
		return false, ""
	}
	return true, "好学仅接受纪录片（410 特别通道）——普通影视资源不适用本站"
}

// hxptDocumentaryPrefill §59.453: 好学（www.hxpt.org）410 纪录片特别通道
// 四槽注入（用户逐点定案讨论终案）：
//   - 年级恒"其他"（audiocodec_sel——站方语义=教育年级幼儿园~高三，
//     纪录片无年级概念；字段名走 form_fields 域键）
//   - 版本恒"其他"（processing_sel——站方语义=教材出版社，纪录片无
//     教材概念；无域常量直写站方字段名 [4] 形态——好学特化站字段名稳定）
//   - 年份三级链（source_sel——站方语义=发布年份）：tp.Year（Tab1 同源
//     标题+PTGen 校正）精确 label 匹配 → 早于 2010→"2010年前" →
//     空/未来年份站方未加项→不填（非必填不硬造）
//   - 格式（codec_sel——站方语义=文件格式）：MI General 容器规范名
//     （Matroska→MKV/MPEG-4→MP4）+标题词形兜底→其它一律"其他"。
//
// value_mappings 的 source/processing 槽 label 全录（值集完备——站方
// HTML 对照）+label 动态匹配消费；codec/audiocodec 槽零 standard_keys
// （jobs 通用链 miss 不填，特化覆盖）。
func hxptDocumentaryPrefill(form map[string]string, cfg *model.PublishFormConfig, meta *model.TorrentMetadata, tp titleparser.TechProfile) {
	if form == nil || cfg == nil {
		return
	}
	// ① 年级恒其它（audiocodec 域键取字段名）
	if f := cfg.FormFields[model.FieldDomainAudiocodec]; f != "" {
		if m := hxptLabelOption(cfg.ValueMappings[model.FieldDomainAudiocodec], "其他"); m != nil {
			form[f] = m.Value
		}
	}
	// ② 版本恒其它（直写站方字段名）
	if m := hxptLabelOption(cfg.ValueMappings["processing"], "其他"); m != nil {
		form["processing_sel[4]"] = m.Value
	}
	// ③ 年份三级链（直写站方字段名）。站方值集 2011 起有独立项——2010
	// 及更早一律"2010年前"（n<=2010：值集无 2010 独立 label，精确匹配必
	// miss——裸对修正时同步校准）。
	if y := strings.TrimSpace(tp.Year); y != "" {
		if n, err := strconv.Atoi(y); err == nil && n > 0 {
			target := y
			if n <= 2010 {
				target = "2010年前"
			}
			if m := hxptLabelOption(cfg.ValueMappings["source"], target); m != nil {
				form["source_sel[4]"] = m.Value
			}
		}
	}
	// ④ 格式：MI 容器（§59.453 透出）→标题词形兜底→"其他"
	if f := cfg.FormFields[model.FieldDomainCodec]; f != "" {
		label := "其他"
		raw := strings.ToLower(tp.RawTitle)
		switch {
		case strings.EqualFold(tp.ContainerFormat, "Matroska"):
			label = "MKV"
		case strings.EqualFold(tp.ContainerFormat, "MPEG-4"):
			label = "MP4"
		case strings.Contains(raw, "mkv"):
			label = "MKV"
		case strings.Contains(raw, "mp4"):
			label = "MP4"
		}
		if m := hxptLabelOption(cfg.ValueMappings[model.FieldDomainCodec], label); m != nil {
			form[f] = m.Value
		}
	}
}

// hdtimeCategoryValue §59.426（实现收敛 discCategoryRoute 公共——§59.428
// 分享站同构复用抽公共，两站 target 值差异参数化）。
func hdtimeCategoryValue(cfg *model.PublishFormConfig, siteName string, meta *model.TorrentMetadata, tp titleparser.TechProfile) *model.FormValueMapping {
	if siteName != "时光" {
		return nil
	}
	return discCategoryRoute(cfg, meta, tp, "424")
}

// itzmxCategoryValue §59.428: 分享站（pt.itzmx.com）type 槽原盘独立分类
// 路由（typeMappingOf 第八站——414 蓝光独立分类，站内实态仲裁：414 列表页
// 123 种全原盘标题 Blu-ray AVC/UHD Blu-ray DoVi——时光 424 同构形态，
// discCategoryRoute 公共复用）。414 label-only（特化独占）。
func itzmxCategoryValue(cfg *model.PublishFormConfig, siteName string, meta *model.TorrentMetadata, tp titleparser.TechProfile) *model.FormValueMapping {
	if siteName != "分享站" {
		return nil
	}
	return discCategoryRoute(cfg, meta, tp, "414")
}

// discfanRegionWords §59.425: 碟粉电影产区细分词表（比 classifyRegionWords
// §59.391 更细粒度——港/台分立、日/韩分立、泰单列：站方电影按地区 7 分）。
var (
	discfanMainlandWr = []string{"中国大陆", "大陆", "内地"}
	discfanHKWr       = []string{"中国香港", "香港"}
	discfanTWWr       = []string{"中国台湾", "台湾"}
	discfanJapanWr    = []string{"日本"}
	discfanKoreaWr    = []string{"韩国"}
	discfanThailandWr = []string{"泰国"}
)

// discfanCategoryValue §59.425: 碟粉（discfan.net）type 槽电影产区细分路由
// （typeMappingOf 第六站——用户定案）。站方电影按地区 7 分（大陆 401/港 404/
// 台 405/泰 402/日 403/韩 406/世界 410）——产区数据源=PTGenSourceJSON→
// src.Region（首值优先——葡萄定案）；列举地区 miss→410 世界兜底（亚洲小国
// 合法归 410——欧美已被 discfanPreflight 禁欧美上游拦，到路由的 410 承接
// =亚洲其它+罕见产区）。电影 7 地区+410 全部 label-only（必须经产区路由，
// 通用链不猜产区）；非电影类型（剧集/音乐/纪录/综艺/体育/动漫）回退通用
// 链（标准键直配——产区维度由 preflight 禁欧美统一把关）。
func discfanCategoryValue(cfg *model.PublishFormConfig, siteName string, meta *model.TorrentMetadata) *model.FormValueMapping {
	if siteName != "碟粉" || cfg == nil || meta == nil {
		return nil
	}
	if meta.Category != "category.movie" {
		return nil // 非电影回退通用链
	}
	var regions []string
	if meta.PTGenSourceJSON != "" {
		if src, err := metadata.UnmarshalPTGenSource(meta.PTGenSourceJSON); err == nil && src != nil {
			regions = src.Region
		}
	}
	// 产区匹配（首值优先+全数组兜底——葡萄定案）
	pick := func(words []string) bool {
		if len(regions) > 0 && containsAnyWord(regions[0], words) {
			return true
		}
		for _, r := range regions {
			if containsAnyWord(r, words) {
				return true
			}
		}
		return false
	}
	var want string
	switch {
	case pick(discfanMainlandWr):
		want = "401"
	case pick(discfanHKWr):
		want = "404"
	case pick(discfanTWWr):
		want = "405"
	case pick(discfanJapanWr):
		want = "403"
	case pick(discfanKoreaWr):
		want = "406"
	case pick(discfanThailandWr):
		want = "402"
	default:
		want = "410" // 世界兜底（欧美上游已拦——此处承接亚洲小国等）
	}
	for i := range cfg.ValueMappings[model.FieldDomainType] {
		if cfg.ValueMappings[model.FieldDomainType][i].Value == want {
			return &cfg.ValueMappings[model.FieldDomainType][i]
		}
	}
	return nil
}

// discfanReleaseBlocked §59.425: 碟粉站点级发布前置双检查（用户定案）。
// ①五年规则（站规硬规则——仅允许五年以前的资源）：
//   year ≤ 当前年-5 放行；year > 当前年-5 拦；无年份放行（宁放——
//   29 库实证占比 0+审核门无 year 字段按需生长）。
//   年份信号=tp.Year（标题+PTGen §59.254 校正终态——Tab1 同源）。
// ②禁欧美（用户策略性定案——欧美资源一颗不发连候选流程都不走；
//   站规原文"欧美需候选"升级为我方直接禁）：
//   classifyRegionWords 复用——Western 拦/亚洲四类（大陆/港台/日韩/
//   亚洲其它）放/无产区防御性放行（29 库 315/315 Region 全覆盖实证
//   ——实际不触发）。
// 挂载：Execute PushOnly 分支后（恒定检查同 §59.424 P1——补推豁免）。
func discfanReleaseBlocked(meta *model.TorrentMetadata, tp *titleparser.TechProfile) (bool, string) {
	if meta == nil || tp == nil {
		return false, ""
	}
	// ①五年规则
	if y, err := strconv.Atoi(strings.TrimSpace(tp.Year)); err == nil && y > time.Now().Year()-5 {
		return true, fmt.Sprintf("碟粉五年规则：%d 年资源（仅允许 %d 年及以前）——五年内资源禁止发布", y, time.Now().Year()-5)
	}
	// ②禁欧美
	var regions []string
	if meta.PTGenSourceJSON != "" {
		if src, err := metadata.UnmarshalPTGenSource(meta.PTGenSourceJSON); err == nil && src != nil {
			regions = src.Region
		}
	}
	if len(regions) > 0 {
		cls := classifyRegionWords(regions[:1])
		if cls == regionNone {
			cls = classifyRegionWords(regions)
		}
		// classifyRegionWords 只分亚洲四类（§59.391——"其余=欧美"系消费端
		// 默认）：有产区证据但不中任何亚洲词表（regionNone）= 欧美/海外 → 拦。
		// regions 空 = 无产区证据 → 防御性放行（29 库 315/315 全覆盖实证不触发）。
		if cls == regionNone {
			return true, "碟粉站点规则：欧美产区资源禁发（用户策略——亚洲资源直发/欧美不走候选）"
		}
	}
	return false, ""
}

// hdskyCategoryValue §59.423: 天空（hdsky.me）type 槽分集/合集×国产/海外
// 双维路由（typeMappingOf 第五站——用户定案：恒合集+产区路由）。站规
// 明文"剧集分集仅官方发布员可发"+我方发布恒整季合集（v1.05）——
// 402/412/414 分集项 label-only 防撞；本特化只服务合集侧产区路由：
//   剧集 tv_series：陆港澳台→411 国产合集 ｜ 其余/无产区→413 海外合集
//   综艺 tv_shows：陆港澳台→403 国产综艺 ｜ 其余/无产区→415 海外合集
// 产区数据源=meta.PTGenSourceJSON→src.Region（Tab1"作品信息-产地"同源
// ——葡萄先例复用；首值优先+全数组兜底）。无产区证据默认海外（海外
// 剧集大盘——预览可手改）。403 配 [tv_shows] 键：综艺特化 miss 时
// 通用链兜底命中国产综艺（错分好过 miss——预览可手改）；411/413
// label-only（必须经产区路由，通用链不猜国籍）。
// 电影/纪录/动漫/短剧等其余类型回退通用链。
func hdskyCategoryValue(cfg *model.PublishFormConfig, siteName string, meta *model.TorrentMetadata) *model.FormValueMapping {
	if siteName != "天空" || cfg == nil || meta == nil {
		return nil
	}
	var want string
	switch meta.Category {
	case "category.tv_series":
		want = "413"
		if hdskyIsDomesticRegion(meta) {
			want = "411"
		}
	case "category.tv_shows":
		want = "415"
		if hdskyIsDomesticRegion(meta) {
			want = "403"
		}
	default:
		return nil
	}
	for i := range cfg.ValueMappings[model.FieldDomainType] {
		if cfg.ValueMappings[model.FieldDomainType][i].Value == want {
			return &cfg.ValueMappings[model.FieldDomainType][i]
		}
	}
	return nil
}

// hdskyIsDomesticRegion 陆港澳台=国产判定（天空双维路由消费——
// classifyRegionWords §59.391 复用：regionMainland+regionHKTW）。
func hdskyIsDomesticRegion(meta *model.TorrentMetadata) bool {
	var regions []string
	if meta.PTGenSourceJSON != "" {
		if src, err := metadata.UnmarshalPTGenSource(meta.PTGenSourceJSON); err == nil && src != nil {
			regions = src.Region
		}
	}
	if len(regions) == 0 {
		return false
	}
	cls := classifyRegionWords(regions[:1]) // 首值优先（葡萄定案）
	if cls == regionNone {
		cls = classifyRegionWords(regions)
	}
	return cls == regionMainland || cls == regionHKTW
}

// hdhomeCategoryValue §59.420: 家园（hdhome.org）type 槽三维融合特化
// （typeMappingOf 第四站——好大模式第三例放大版）。站方 type 种子区
// 63 项=类型×媒介×分辨率三维编码（label 结构 "{Prefix} {Medium/Res}"
// 统一），form_config 单键单值不可表达——**label 词形路由**（非硬编码
// value：站方改值不破+错拼容错 "TVSereis"(453)+"TVShows"(452) 复数+
// "Sport 8K"(511) 单数）。
// prefix（category 键→归一大写前缀）：
//   movie→MOVIES ｜ tv_series→TVSER ｜ tv_shows→TVSHOW ｜ documentary
//   (单复数)→DOC ｜ animation→ANIME ｜ music/lossless_music→MUSICS
//   （音频格式词 FLAC/APE 子路由）｜ sports→SPORT ｜ mv/music_video→
//   MUSICS+MV ｜ 其余（education/ebook/software/game/concert 等）→nil
//   （杂项站方概不接受/无独立项——通用链兜底）。
// suffix 路由矩阵（medium 单源+resolution 归一，label 归一去连字符
// 大写后 Contains 匹配）：
//   uhd_bluray: 8K(r4320p)→"8K"+"UHD BD" ｜ 其余→"UHD"+"BLURAY"
//   bluray→"BLURAY"（排除 UHD/8K）
//   uhd_remux: 8K→"8K"+"REMUX" ｜ 其余→"UHD"+"REMUX"
//   remux→"REMUX"（排除 UHD/8K）
//   encode/webdl/webrip/hdtv/minibd/hddvd/dvd/dvdr/other+分辨率：
//     r4320p→"4320P" ｜ r2160p→"2160P" ｜ r1080p→"1080P" ｜ r1080i→
//     "1080I" ｜ r720p→"720P" ｜ sd→"SD" ｜ 空分辨率→nil
//   music 类 cd/track 媒介→音频词形 FLAC/APE（tp.AudioCodec）
// 匹配：prefix 子集内 suffix 词形首中；REMUX 项由 remux 族分支独占
// （原盘分支排除 REMUX 词防 529 "Doc 8K UHD BD REMUX" 撞 8K 原盘）。
// nil=非家园/无 prefix/无分辨率（encode 族）——通用链零变化。
func hdhomeCategoryValue(cfg *model.PublishFormConfig, siteName string, meta *model.TorrentMetadata, tp titleparser.TechProfile) *model.FormValueMapping {
	if siteName != "家园" || cfg == nil || meta == nil {
		return nil
	}
	cat := meta.Category
	prefix, isMusic := "", false
	switch cat {
	case "category.movie":
		prefix = "MOVIES"
	case "category.tv_series":
		prefix = "TVSER" // 容错站方错拼 TVSereis(453)
	case "category.tv_shows":
		prefix = "TVSHOW" // 匹配单数 TVShow+复数 TVShows(452)
	case "category.documentary", "category.documentaries":
		prefix = "DOC"
	case "category.animation":
		prefix = "ANIME"
	case "category.music", "category.lossless_music":
		prefix, isMusic = "MUSICS", true
	case "category.sports":
		prefix = "SPORT" // 匹配单复数
	case "category.mv", "category.music_video":
		prefix = "MUSICS" // +MV 词形
	default:
		return nil
	}
	mediumKey := mediumStdKeyOf(tp, "")
	resKey := extract.LookupStandardKey("resolution", tp.Resolution)
	norm := func(s string) string {
		return strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(s, "-", ""), " ", ""))
	}
	// 路由目标词形（norm 后 Contains 匹配）
	type wordMatch struct{ words []string; exclude []string }
	var m wordMatch
	isMV := cat == "category.mv" || cat == "category.music_video"
	switch {
	case isMV:
		m = wordMatch{words: []string{"MV"}, exclude: nil}
	case isMusic && (mediumKey == "medium.cd" || mediumKey == "medium.track"):
		// 音乐音频格式子路由（FLAC/APE）
		ac := strings.ToUpper(tp.AudioCodec)
		switch {
		case strings.Contains(ac, "FLAC"):
			m = wordMatch{words: []string{"FLAC"}}
		case strings.Contains(ac, "APE"):
			m = wordMatch{words: []string{"APE"}}
		default:
			return nil
		}
	case mediumKey == "medium.uhd_bluray":
		if resKey == "resolution.r4320p" {
			m = wordMatch{words: []string{"8K", "UHDBD"}, exclude: []string{"REMUX"}}
		} else {
			m = wordMatch{words: []string{"UHDBLURAY"}, exclude: []string{"REMUX"}}
		}
	case mediumKey == "medium.bluray":
		m = wordMatch{words: []string{"BLURAY"}, exclude: []string{"UHD", "8K", "REMUX"}}
	case mediumKey == "medium.uhd_remux", mediumKey == "medium.remux":
		// 归一损失 UHD 维度（"UHD REMUX"→remux）——按分辨率分发：
		// 8K(4320p)→8K REMUX（529 Doc）；2160p→UHD REMUX（518/531）；
		// 其它→REMUX（415 等 BD Remux）
		switch resKey {
		case "resolution.r4320p":
			m = wordMatch{words: []string{"8K", "REMUX"}}
		case "resolution.r2160p":
			m = wordMatch{words: []string{"UHD", "REMUX"}, exclude: []string{"8K"}}
		default:
			m = wordMatch{words: []string{"REMUX"}, exclude: []string{"UHD", "8K"}}
		}
	default:
		// encode/webdl/webrip/hdtv/minibd/hddvd/dvd/dvdr/cd/track/other+分辨率
		switch resKey {
		case "resolution.r4320p":
			m = wordMatch{words: []string{"4320P"}}
		case "resolution.r2160p":
			m = wordMatch{words: []string{"2160P"}}
		case "resolution.r1080p":
			m = wordMatch{words: []string{"1080P"}}
		case "resolution.r1080i":
			m = wordMatch{words: []string{"1080I"}}
		case "resolution.r720p":
			m = wordMatch{words: []string{"720P"}}
		case "resolution.sd", "resolution.r480p", "resolution.r576p":
			m = wordMatch{words: []string{"SD"}}
		default:
			return nil // encode 族无分辨率不路由（预览人工）
		}
	}
	items := cfg.ValueMappings[model.FieldDomainType]
	for i := range items {
		label := norm(items[i].Label)
		if !strings.HasPrefix(label, prefix) {
			continue
		}
		ok, excluded := true, false
		for _, w := range m.words {
			if !strings.Contains(label, w) {
				ok = false
				break
			}
		}
		for _, x := range m.exclude {
			if strings.Contains(label, x) {
				excluded = true
				break
			}
		}
		if ok && !excluded {
			return &items[i]
		}
	}
	return nil
}

// cbgCategoryValue §59.411: 藏宝阁（cangbao.ge）type 槽动画/动漫分立
// 特化（typeMappingOf 第三站）。站方 404 动画（单集语义——电影/单集
// OVA）与 405 动漫（多集语义）分立且**双向规则校验**（动画分类检出
// 多集信号→站方错误/动漫分类无多集信号→站方错误）——form_config 单键
// 单值不可表达，404 配 animation 键仅覆盖单集形态，多集形态由本特化
// 分发：SeasonEpisode 非空（S01/E 多集信号）→ 405 动漫；空 → 404
// 动画（通用链兜底）。非藏宝阁/非动画类回退通用链零变化。
func cbgCategoryValue(cfg *model.PublishFormConfig, siteName string, meta *model.TorrentMetadata, tp titleparser.TechProfile) *model.FormValueMapping {
	if siteName != "藏宝阁" || cfg == nil || meta == nil {
		return nil
	}
	if meta.Category != "category.animation" || tp.SeasonEpisode == "" {
		return nil // 单集动画走通用链（404 配 animation 键兜底）
	}
	for i := range cfg.ValueMappings[model.FieldDomainType] {
		if cfg.ValueMappings[model.FieldDomainType][i].Value == "405" {
			return &cfg.ValueMappings[model.FieldDomainType][i]
		}
	}
	return nil
}

// hdareaCategoryValue §59.405: 好大（hdarea.club）type 槽媒介细分特化
// （用户五定案 2026-11-01）。站方 type 18 项=电影按媒介/分辨率细分
// 10 项（type×medium×resolution 三维融合——form_config 单键单值不可
// 表达，葡萄产区模式第二站）。**仅 category.movie 走特化**——非电影
// 类（动画 3D 归 405/MV/剧集等）回退通用链直配（416 命名 *Movies* 3D
// 只服务电影）。
// 路由矩阵（判定优先级自上而下，用户定案）：
//  ① Stereo3D → 416（定案1：3D 最优先）
//  ② uhd_bluray → 300 ｜ ③ bluray → 401 ｜ ④ remux/uhd_remux → 415
//  ⑤ dvdr/dvd → 414 ｜ ⑥ webdl/webrip → 412（定案4：不看分辨率——
//     4K WEB 也进，分辨率由 standard_sel 表达）｜ ⑦ hdtv → 413
//  ⑧ 兜底（encode/minibd/hddvd/ipad 等）按分辨率（定案2/3）：
//     2160p → 300 ｜ 1080p/1080i → 410 ｜ 720p → 411 ｜ SD/其它 → 414
//     （414=DVD 分辨率宽语义=原盘/镜像/DVDRip/SD 重编码——用户定义）
// 数据源：mediumStdKeyOf 媒介单源+tp.Resolution+tp.Stereo3D（§59.380
// 联动同源）。
func hdareaCategoryValue(cfg *model.PublishFormConfig, siteName string, meta *model.TorrentMetadata, tp titleparser.TechProfile) *model.FormValueMapping {
	if siteName != "好大" || cfg == nil || meta == nil {
		return nil
	}
	if meta.Category != "category.movie" {
		return nil // 非电影类回退通用链（B 组直配——用户定案5）
	}
	var target string
	mediumKey := mediumStdKeyOf(tp, "")
	switch mediumKey {
	case "medium.uhd_bluray":
		target = "300"
	case "medium.bluray":
		target = "401"
	case "medium.remux", "medium.uhd_remux":
		target = "415"
	case "medium.dvdr", "medium.dvd":
		target = "414"
	case "medium.webdl", "medium.webrip":
		target = "412"
	case "medium.hdtv":
		target = "413"
	default:
		// 兜底按分辨率（encode/minibd/hddvd/ipad/其它——iPad 与
		// MiniBD 无专属分类，SD 并入 414 DVD 分辨率族——用户定案2/3）
		switch extract.LookupStandardKey("resolution", tp.Resolution) {
		case "resolution.r2160p", "resolution.r4320p":
			target = "300"
		case "resolution.r1080p", "resolution.r1080i":
			target = "410"
		case "resolution.r720p":
			target = "411"
		default:
			target = "414"
		}
	}
	if tp.Stereo3D != "" {
		target = "416" // 定案1：3D 最优先（Movies 3D——电影类专属）
	}
	for i := range cfg.ValueMappings[model.FieldDomainType] {
		if cfg.ValueMappings[model.FieldDomainType][i].Value == target {
			return &cfg.ValueMappings[model.FieldDomainType][i]
		}
	}
	return nil
}

// regionClass 产区归类（§59.391 用户定案规则）：
// 陆/港/澳/台→华语（细分大陆 vs 港台——葡萄剧集/综艺站方粒度）；
// 日韩独立（综艺站方粒度）；其余亚洲国→亚洲其它；再其余→欧美兜底。
type regionClass int

const (
	regionNone regionClass = iota
	regionMainland
	regionHKTW
	regionJapanKorea
	regionAsianOther
	regionWestern
)

var (
	regionMainlandWords = []string{"中国大陆", "大陆"}
	regionHKTWWords     = []string{"中国香港", "中国台湾", "中国澳门", "香港", "台湾", "澳门"}
	regionJapanKoreaWr  = []string{"日本", "韩国", "日韩"}
	regionAsianOtherWr  = []string{"印度", "泰国", "越南", "新加坡", "马来西亚", "印度尼西亚",
		"印尼", "菲律宾", "缅甸", "柬埔寨", "尼泊尔", "斯里兰卡", "巴基斯坦", "孟加拉国",
		"孟加拉", "哈萨克斯坦", "乌兹别克斯坦", "伊朗", "土耳其", "以色列", "阿联酋",
		"沙特", "蒙古", "亚洲"}
)

// classifyRegionWords 产区归类（元素级 contains——PTGen Region 元素可能
// 本身是"中国大陆 / 中国香港"复合串，splitBySlash 已拆但保守 contains）。
func classifyRegionWords(regions []string) regionClass {
	for _, r := range regions {
		if containsAnyWord(r, regionMainlandWords) {
			return regionMainland
		}
		if containsAnyWord(r, regionHKTWWords) {
			return regionHKTW
		}
		if containsAnyWord(r, regionJapanKoreaWr) {
			return regionJapanKorea
		}
		if containsAnyWord(r, regionAsianOtherWr) {
			return regionAsianOther
		}
	}
	return regionNone
}

func containsAnyWord(s string, words []string) bool {
	for _, w := range words {
		if strings.Contains(s, w) {
			return true
		}
	}
	return false
}

// sjtuRegionCategoryValue §59.391: 葡萄产区×类型映射。产区数据源=
// meta.PTGenSourceJSON→src.Region（Tab1"作品信息-产地"同源——
// §59.151 regionLabelsOfMeta 幸运判据消费先例）。**首值优先**
// （Region[0]=豆瓣展示的主产地序——复合产区二义消解，用户定案）；
// 首值 miss 再扫全数组；无产区证据 fallback 欧美（402/410/413——
// 资源主形态，预览可手改）。
func sjtuRegionCategoryValue(cfg *model.PublishFormConfig, siteName string, meta *model.TorrentMetadata) *model.FormValueMapping {
	if siteName != "葡萄" || cfg == nil || meta == nil {
		return nil
	}
	var regions []string
	if meta.PTGenSourceJSON != "" {
		if src, err := metadata.UnmarshalPTGenSource(meta.PTGenSourceJSON); err == nil && src != nil {
			regions = src.Region
		}
	}
	cls := regionNone
	if len(regions) > 0 {
		if c := classifyRegionWords(regions[:1]); c != regionNone { // 首值优先
			cls = c
		} else {
			cls = classifyRegionWords(regions) // 全数组兜底
		}
	}
	if cls == regionNone {
		cls = regionWestern // 无产区证据→欧美兜底
	}
	var want string
	switch meta.Category {
	case "category.movie":
		// 华语(401)/亚洲(403)/欧美(402)——日韩并入亚洲（电影无日韩项）
		switch cls {
		case regionMainland, regionHKTW:
			want = "401"
		case regionJapanKorea, regionAsianOther:
			want = "403"
		default:
			want = "402"
		}
	case "category.tv_series":
		// 港台(407)/亚洲(408)/大陆(409)/欧美(410)四分
		switch cls {
		case regionMainland:
			want = "409"
		case regionHKTW:
			want = "407"
		case regionJapanKorea, regionAsianOther:
			want = "408"
		default:
			want = "410"
		}
	case "category.tv_shows":
		// 大陆(411)/港台(412)/欧美(413)/日韩(414)四分——其它亚洲归欧美
		// （站方无"亚洲综艺"项）
		switch cls {
		case regionMainland:
			want = "411"
		case regionHKTW:
			want = "412"
		case regionJapanKorea:
			want = "414"
		default:
			want = "413"
		}
	default:
		return nil // 非产区类（动画/纪录/MV/体育…）→通用链
	}
	for i := range cfg.ValueMappings[model.FieldDomainType] {
		if cfg.ValueMappings[model.FieldDomainType][i].Value == want {
			return &cfg.ValueMappings[model.FieldDomainType][i]
		}
	}
	return nil
}

// hduptTVMediumOption §59.427: 好多油 TV 双轨分发（剧集媒介 TV 后缀项）。
func hduptTVMediumOption(cfg *model.PublishFormConfig, siteName string, meta *model.TorrentMetadata, stdKey string) *model.FormValueMapping {
	if siteName != "好多油" || cfg == nil || meta == nil {
		return nil
	}
	if meta.Category != "category.tv_series" {
		return nil // 电影/综艺等走通用键（电影形态主键）
	}
	var want string
	switch stdKey {
	case "medium.remux":
		want = "12" // Remux TV
	case "medium.uhd_remux":
		want = "16" // UHD Remux TV
	case "medium.encode":
		want = "14" // Encode TV
	case "medium.webdl", "medium.webrip":
		want = "13" // WEB-DL/WEBRip TV
	default:
		return nil // 原盘/CD 等 TV 无分立项——通用链
	}
	for i := range cfg.ValueMappings[model.FieldDomainMedium] {
		if cfg.ValueMappings[model.FieldDomainMedium][i].Value == want {
			return &cfg.ValueMappings[model.FieldDomainMedium][i]
		}
	}
	return nil
}


// isUploadTimeoutErr §59.470: 上传超时类错误判定——超时≠失败（站方可能
// 已完成入库·南洋案实证），触发站方复查通道。
func isUploadTimeoutErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	s := err.Error()
	return strings.Contains(s, "Client.Timeout") ||
		strings.Contains(s, "context deadline exceeded") ||
		strings.Contains(s, "Timeout exceeded") ||
		strings.Contains(s, "timeout")
}

// ptgenRegionGenreWords §59.471: PTGen 产区/风格词提取（region/style 域
// 数据源——HDVideo 必填两域消费）。
func ptgenRegionGenreWords(meta *model.TorrentMetadata, domain string) []string {
	if meta == nil || meta.PTGenSourceJSON == "" {
		return nil
	}
	src, err := metadata.UnmarshalPTGenSource(meta.PTGenSourceJSON)
	if err != nil || src == nil {
		return nil
	}
	if domain == model.FieldDomainRegion {
		return src.Region
	}
	return src.Genre
}

// fallbackByValue §59.471: 域内按站方值找兜底项（region miss→"28"其他地区
// /style miss→"6"剧情——用户定案 a+c·碟粉 410 兜底同族先例）。
func fallbackByValue(mappings []model.FormValueMapping, value string) *model.FormValueMapping {
	for i := range mappings {
		if mappings[i].Value == value {
			return &mappings[i]
		}
	}
	return nil
}

// regionMappingOf §59.471: 地区槽映射——PTGen 产区词 canonical 化后按
// standard_keys 匹配（港台双键承接·朱雀先例）；miss 兜底"28"其他地区
// （HDVideo 站方值·当前唯一配置 region 域的站）。未配置域返回 nil 跳过。
func (e *PublishExecutor) regionMappingOf(cfg *model.PublishFormConfig, siteName string, meta *model.TorrentMetadata) *model.FormValueMapping {
	mappings := cfg.ValueMappings[model.FieldDomainRegion]
	if len(mappings) == 0 {
		return nil
	}
	_ = siteName
	for _, raw := range ptgenRegionGenreWords(meta, model.FieldDomainRegion) {
		if k := titleparser.LookupDictKey("region", raw); k != "" {
			if m := e.lookupByStdKey(cfg, model.FieldDomainRegion, k); m != nil {
				return m
			}
		}
	}
	// §59.494: 兜底站自适应——"28"为 HDVideo 值集（其它站无 28 值会
	// fallback nil→region 缺→站方"质量区域未选择"·优堡案）。miss 时按
	// label 词形找"其它/其他"条目（各站其它桶通用形态）。
	if m := fallbackByValue(mappings, "28"); m != nil {
		return m
	}
	return fallbackByOtherLabel(mappings)
}

// fallbackByOtherLabel §59.494: 地区兜底"其它"桶 label 词形匹配
// （优堡"其它(Other)"/HDVideo"其他地区"双形态）。
func fallbackByOtherLabel(mappings []model.FormValueMapping) *model.FormValueMapping {
	for i := range mappings {
		if strings.Contains(mappings[i].Label, "其它") || strings.Contains(mappings[i].Label, "其他") {
			return &mappings[i]
		}
	}
	return nil
}

// styleMappingOf §59.471: 风格槽映射——PTGen 类型词 canonical 化匹配；
// miss 兜底"6"剧情（最泛）。style_sel[4][] 多选形态单值勾选（首命中）。
func (e *PublishExecutor) styleMappingOf(cfg *model.PublishFormConfig, siteName string, meta *model.TorrentMetadata) *model.FormValueMapping {
	mappings := cfg.ValueMappings[model.FieldDomainStyle]
	if len(mappings) == 0 {
		return nil
	}
	_ = siteName
	for _, raw := range ptgenRegionGenreWords(meta, model.FieldDomainStyle) {
		if k := titleparser.LookupDictKey("genre", raw); k != "" {
			if m := e.lookupByStdKey(cfg, model.FieldDomainStyle, k); m != nil {
				return m
			}
		}
	}
	return fallbackByValue(mappings, "6")
}

// haidanSeasonEpisodeInject §59.472: 海胆季/集/合集三字段注入（From S04 案——
// 纯季包 §59.424 合集形态放行但海胆站方要求剧集类必填季集信息·集数 0=全季）。
// 数据源 tp.SeasonEpisode 归一终态（§59.424 同源——挂载点同在归一后）：
//   S04 纯季 → season=4 episode=0（全季）；S01-S02 跨季 → 0/0+勾合集；
//   S04E01 单集（eps=1 全集豁免形态）→ season=4 episode=1；
//   剧集类无季信号（国产剧）→ 0/0（站方语义：不区分季+全季）；
//   电影等范围外（§59.424 豁免类）→ 不填（站方仅剧集类要求）。
// 未配置 season 键（其它站）零动作。
func haidanSeasonEpisodeInject(form map[string]string, cfg *model.PublishFormConfig, meta *model.TorrentMetadata, tp *titleparser.TechProfile) {
	sf, hasSeason := cfg.FormFields[model.FieldDomainSeason]
	if !hasSeason || sf == "" {
		return
	}
	// 剧集类判定（站方提示四类=电视剧/综艺/动画/纪录片——§59.424 同范围）
	switch meta.Category {
	case "category.tv_series", "category.animation", "category.tv_shows", "category.documentary", "category.documentaries":
	default:
		return // 电影/音乐等——站方不要求
	}
	se := ""
	if tp != nil {
		se = tp.SeasonEpisode
	}
	season, episode := "0", "0" // 默认：不区分季+全季（国产剧全集形态）
	collages := false
	trimLead0 := func(x string) string {
		t := strings.TrimLeft(x, "0")
		if t == "" {
			return "0"
		}
		return t
	}
	if m := singleEpisodeRe.FindStringSubmatch(se); m != nil {
		// S04E01 单集形态（§59.424 全集豁免放行——整部一集）——剥前导零。
		// §59.514: 裸 Exx 兼容（(?:S\d{1,2})? 可选——S 前缀从 se 尾剥）
		sPart := strings.TrimSuffix(se, "E"+m[1])
		season, episode = trimLead0(strings.TrimPrefix(sPart, "S")), trimLead0(m[1])
	} else if haidanCrossSeasonRe.MatchString(se) {
		// S01-S02 / S01-02 跨季——多季=站方合集（§59.471 回归审核修正：
		// 首版手写 Contains 条件 S1-2 单位数漏判 + S01E02-03 集区间误判——
		// 正则对齐 §59.424 seasonPackFormRe 形态域）
		collages = true
	} else if m := regexp.MustCompile(`^S(\d{1,2})$`).FindStringSubmatch(se); m != nil {
		// S04 纯季——全季（episode=0）——剥前导零
		season = trimLead0(m[1])
	}
	form[sf] = season
	if ef, ok := cfg.FormFields[model.FieldDomainEpisode]; ok && ef != "" {
		form[ef] = episode
	}
	if collages {
		if cf, ok := cfg.FormFields[model.FieldDomainCollages]; ok && cf != "" {
			form[cf] = "1"
		}
	}
}

// hdareaPreChecker 刀2：站方预查重能力接口（好大 API 独有·
// publish 包内私有——executor 局部变量 adapter 遮蔽 adapter 包名）。
type hdareaPreChecker interface {
	CheckBeforeUpload(ctx context.Context, config *model.SiteConfig, torrentData []byte) (*adapter.PreCheckResult, error)
}

// repushConverge 补推成功后的原记录收敛（§59.159 设计意图·提方法可测）：
// uploaded 行 → status=pushed+seeded（上传未推种→已推种）；
// duplicate 行 → 仅置 seeded（v0.0.1455 Caddo Lake/Dead Man 双案：§59.509
// duplicate 成可补推主力后回写漏跟·status 保持「站上已有」事实不变·
// canRepush 判据 tid 在手且本地未做种——seeded 置位即按钮收敛）。
func (e *PublishExecutor) repushConverge(ctx context.Context, in ExecuteInput, now time.Time) {
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
	res3 := e.db.WithContext(ctx).Model(&model.PublishResultRecord{}).
		Where("torrent_id = ? AND target_site = ? AND status = ? AND seeded = ?",
			in.TorrentID, in.TargetSite, "duplicate", false).
		Updates(map[string]interface{}{
			"seeded":    true,
			"seeded_at": now,
		})
	if res3.Error != nil {
		e.logger.Warn("repush duplicate 回写失败", zap.Error(res3.Error))
	} else if res3.RowsAffected > 0 {
		e.logger.Info("repush duplicate 收敛", zap.String("tid", in.TorrentID), zap.Int64("rows", res3.RowsAffected))
	}
}
