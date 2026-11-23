package publish

// §59.558 PR-5: Execute 编排拆分产物——三阶段函数（PushOnly 补推通道/
// 表单构建/DryRun 检查点）。失败语义经返回值传递：failStatus 非空时
// 主函数按 useFailRec 选 fail（不落库）或 failRec（落库）闭包。

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/metadata/extract"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/pusher"
	"github.com/ranfish/pt-forward/internal/titleparser"
	"go.uber.org/zap"
)

// executePushOnly §59.159 PushOnly 补推通道（原 Execute :236-280 整段抽取）：
// 跳过上传/dedup——推指定 TorrentID。失败经 (failStatus, failMsg) 返回
// （主函数 failRec 落库）。
func (e *PublishExecutor) executePushOnly(ctx context.Context, in ExecuteInput, meta *model.TorrentMetadata, rv *ResourceView) (*ExecuteResult, string, string) {
	if in.TorrentID == "" {
		return nil, "failed", "PushOnly 需 TorrentID"
	}
	if e.pipe.pusher == nil {
		return nil, "failed", "pusher 未注入"
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

	// §59.159 回归审核：补推成功回写原记录（同 tid 最新行 Status/Seeded）
	if res.Status == "pushed" || res.Status == "pushed_existing" {
		e.repushConverge(ctx, in, time.Now())
	}
	return res, "", ""
}

// executeFormBuild 表单构建产物（原 Execute ④⑤⑥ 段抽取）。
type executeFormBuild struct {
	form           map[string]string
	tp             titleparser.TechProfile
	publishTitle   string
	arbNotes       []string
	jobs           []domainJob
	tags           []string
	tagArrayFields []model.TagKV
	applied        []string
	gpwExtra       map[string]string
	gpwArtists     []model.PublishArtist
	gpwSubs        []string
}

// buildExecuteForm §59.558 PR-5: 表单段（原 Execute :329-601 抽取）——
// 域值组装/tp 终态/标题锚定/jobs 映射/站点特化注入/tags/描述组装。
// 失败返回 (nil, status, msg, useFailRec)——主函数按 useFailRec 选闭包。
func (e *PublishExecutor) buildExecuteForm(ctx context.Context, in ExecuteInput, meta *model.TorrentMetadata, rv *ResourceView, site *model.Site, cfg *model.PublishFormConfig, torrentData []byte) (*executeFormBuild, string, string, bool) {
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
	// §59.541 附十一: 截图独立框（劳改所——URL 一行一张；配置此域的站
	// assembleDescription 不再内嵌截图·descr 保持无图形态；未配域自然跳过）
	setForm(model.FieldDomainScreenshot, strings.Join(parseScreenshotsCol(meta.Screenshots), "\n"))
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
		return nil, "ineligible", reason, true
	}
	// §59.425: 碟粉站点级双检查（五年规则+禁欧美——用户定案）。挂载
	// PushOnly 分支后（恒定检查同 §59.424 P1 补推豁免）；DryRun 同拦可见。
	// **站点门控（回归修复：首版漏门控——碟粉规则拦了所有站，时光
	// DryRun 被"碟粉站点规则"拦实战抓出）**：仅 targetSite==碟粉 触发。
	if in.TargetSite == "碟粉" {
		if blocked, reason := discfanReleaseBlocked(meta, &tp); blocked {
			return nil, "ineligible", reason, true
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
			return nil, "failed", "标题重组失败：MI 缺失或视频不可读——检查下载器文件可访问性后重新获取数据", false
		}
		return nil, "failed", "标题重组失败：标题不可解析且 PTGen 无英文名——重新获取数据后再发布", false
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
			return nil, "failed", fmt.Sprintf(
				"类型映射缺失：站点表单要求 type（%s），元数据 category=%q 无对应选项——检查站点发布表单配置 type 域映射", tf, meta.Category), true
		}
	}
	// §59.471 附: region/style 必填域同款 fail-fast（回归审核补——配置异常
	// （兜底项被手改删除）时静默不发→站方"请选择地区"错误循环重蹈；正常
	// 路径有兜底（28/6）不会触发）。
	for _, domain := range []string{model.FieldDomainRegion, model.FieldDomainStyle} {
		if f, ok := cfg.FormFields[domain]; ok && f != "" {
			if _, filled := form[f]; !filled {
				return nil, "failed", fmt.Sprintf(
					"%s 映射缺失：站点表单要求必填域 %s（%s）但映射不出（含兜底）——检查站点发布表单配置 %s 域 value_mappings（含兜底项）", domain, domain, f, domain), true
			}
		}
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
			return nil, "ineligible", reason, true
		}
		// 2d-② 截图三要件门 §59.462 附九（格式 PNG/图床白名单/≥3 张——
		// 站规 2.2.1 硬性要求，质量门=拦截+行动指引文案；DryRun 同拦）
		if blocked, reason := gpwScreenshotGate(parseScreenshotsCol(meta.Screenshots)); blocked {
			return nil, "ineligible", reason, true
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
	// §59.541: 二师兄特化注入（section_id 动画组合判据覆写+category_ids
	// 体裁数组+source/processing 域特化组装——form_config 单值映射覆盖
	// 不了的组装逻辑）
	if in.TargetSite == "二师兄" {
		piggoPublishInject(form, cfg, meta, tp)
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
	// §59.531: 段提取为 AssembleTagArrayForSite——发布链与修复工具（tools/fix-site-tags）
	// 同源消费（单源防双链分叉）；行为零变化（§59.309/§59.529 全链随迁）。
	tags, tagArrayFields, applied := e.AssembleTagArrayForSite(meta, cfg, in.TargetSite, tp.HDR, in.TagOverrides)
	// §59.454: 城市站标签依赖注入（chnize 消费 meta.Tags 的
	// chinese_subtitle——不经 assembleTags 过滤：城市站无 tags 域配置
	// → allowed 空 → 全部标签被滤（首版传 tags 变量恒空实测抓出）；
	// 挂 DryRun 检查点前——§59.453 挂载教训）。
	if in.TargetSite == "城市" {
		hdcityTagDependentFill(form, meta.Tags)
	}

	// ⑥ 描述组装——纯本地资产消费（§59.159 白名单：替代 renderDescription
	// [queryPTGen 在线/rehostPoster 图床转存双违规]；零网络依赖）
	if strings.TrimSpace(meta.Title) == "" || strings.TrimSpace(meta.Description) == "" {
		return nil, "ineligible", "资产不完备：name/简介为空（先完成获取-审核）", true
	}
	form[cfg.FormFields[model.FieldDomainDescription]] = assembleDescription(meta, cfg)
	// §59.554 刀2: TNode 站方元数据自渲染（朱雀形态）——descr 域值重组为
	// note 三件套（纯文本声明+IMDb/豆瓣链接行·全角冒号格式），PT-Gen 简介
	// 不投（站方以 TMDb 渲染简介/封面）。判据与工厂路由同源（site.Framework
	// ——factory.Create 同字段·特化面≡适配器面永不漂移）。DescrQuoteOnly
	// （§59.541 劳改所·豆瓣自渲染）同族不同种：朱雀更彻底——链接行替代全部。
	if site.Framework == "tnode" {
		form[cfg.FormFields[model.FieldDomainDescription]] = tnodeNoteOf(meta)
	}
	// §59.541 附四: piggo {{ptgen}} 模板——已填豆瓣/IMDb 时站方要求简介含
	// {{ptgen}} 占位符（站方自营 PTGen 服务端渲染）；自制 PTGen 正文换占位符
	if cfg.Framework == "piggo" {
		form[cfg.FormFields[model.FieldDomainDescription]] = piggoDescrPtgen(form[cfg.FormFields[model.FieldDomainDescription]], meta)
		// §59.541 附八: screenshots[] 独立字段——piggo 专有截图 URL 输入行
		// （站方页面展示走独立字段·descr 内嵌可能不渲染）
		if shots := parseScreenshotsCol(meta.Screenshots); len(shots) > 0 {
			form["__piggo_screenshots"] = strings.Join(shots, "\n")
		}
	}

	// DryRun 检查点：组装+渲染+预检之后、上传之前（§59.156——完整表单形态验证，
	// 预检未通过不阻断 dry_run：预检修正循环数据源）
	return &executeFormBuild{form: form, tp: tp, publishTitle: publishTitle, arbNotes: arbNotes,
		jobs: jobs, tags: tags, tagArrayFields: tagArrayFields, applied: applied,
		gpwExtra: gpwExtra, gpwArtists: gpwArtists, gpwSubs: gpwSubs}, "", "", false
}

// dryRunCheckpoint §59.156/§59.515 刀2 E4（原 Execute :602-637 抽取）：
// DryRun 检查点——组装+渲染+预检之后、上传之前。
func (e *PublishExecutor) dryRunCheckpoint(ctx context.Context, in ExecuteInput, meta *model.TorrentMetadata, torrentData []byte,
	cfg *model.PublishFormConfig, form map[string]string, preAudit *PreAuditResult, localFindings []LocalAuditFinding,
	applied []string, arbNotes []string) *ExecuteResult {
	res := &ExecuteResult{Status: "dry_run_ok", Form: form, Tags: applied, PreAudit: preAudit, LocalAudit: localFindings,
		FormLabels: buildFormLabels(cfg, form),
		TagAssets:  parseTagsAssets(meta.Tags),
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
