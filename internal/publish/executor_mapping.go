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
	"strings"

	"github.com/ranfish/pt-forward/internal/metadata"

	"github.com/ranfish/pt-forward/internal/metadata/extract"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/titleparser"
	"github.com/ranfish/pt-forward/internal/util"
)

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

// applyMediumSourceFidelity §59.549: 媒介源保真改写（源分立站精确分流）。
// 输入通用翻译产物 stdKey·命中源键条件且值集有该槽时改写·否则原样返回。
// 保守面：仅改写 medium.encode（BD 源压制→BDRip 的适配期定案保持——
// bdrip/dvdrip spec 不进血统通道〔spec 显式声明优先〕·非 encode 键零触碰）。
// ⚠ 仅 medium 域投值链消费（mediumMappingOf/mediumMapping 两处）——
// codec 联动（:1272）与原盘编码分维（blurayCodecOption）的
// mediumStdKeyOf(tp,"") 直调是「形态判定」语义（WEB 压制的 codec 归并
// 仍需 encode 判定）·禁止把源保真统一接进去（会破坏 x265→H.265 联动）。
func (e *PublishExecutor) applyMediumSourceFidelity(cfg *model.PublishFormConfig, tp titleparser.TechProfile, stdKey string) string {
	if stdKey != "medium.encode" || cfg == nil {
		return stdKey
	}
	srcKey := ""
	switch strings.ToLower(tp.Specification) {
	case "webrip":
		srcKey = "medium.webrip"
	case "tvrip":
		srcKey = "medium.tvrip"
	case "encode", "":
		// 血统隐式：encode 判定+WEB 血统（SourceType=WEB——标题无 webrip 词
		// 但提取层判 WEB 源·AMZN x265 式）→ WEB 源压制语义
		if strings.EqualFold(tp.SourceType, "WEB") {
			srcKey = "medium.webrip"
		}
	}
	if srcKey == "" {
		return stdKey
	}
	if e.lookupByStdKey(cfg, model.FieldDomainMedium, srcKey) == nil {
		return stdKey // 值集无该槽（通用 Encode 大类站）——回退通用翻译
	}
	return srcKey
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
	// §59.549 媒介源保真（与 mediumMappingOf 同源——修一处 grep 另一处纪律）：
	// 源分立站精确分流·值集无该槽回退通用翻译。
	stdKey = e.applyMediumSourceFidelity(cfg, titleparser.TechProfile{
		SourceType: meta.SourceType, Specification: meta.Specification,
		VideoCodec: meta.VideoCodec, RawTitle: meta.Title,
	}, stdKey)
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
		if util.ContainsAny(r, regionMainlandWords...) {
			return regionMainland
		}
		if util.ContainsAny(r, regionHKTWWords...) {
			return regionHKTW
		}
		if util.ContainsAny(r, regionJapanKoreaWr...) {
			return regionJapanKorea
		}
		if util.ContainsAny(r, regionAsianOtherWr...) {
			return regionAsianOther
		}
	}
	return regionNone
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
