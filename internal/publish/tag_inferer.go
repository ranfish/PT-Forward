// Package publish MediaTag 推断器（§56.22 Q1 + §59.26 增强 + §59.35 P4 查表化）。
//
// 推断规则数据源：dict/tag.json（Layer 1 tag 域，canonical/infer_pattern/infer_scope）。
// 本文件只保留输入编排 + ApplyTagRules 后处理（覆盖/互斥规则）。
package publish

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/ranfish/pt-forward/internal/titleparser"
)

// MediaTagInferer 从多源文本推断 MediaTags。
type MediaTagInferer struct{}

// NewMediaTagInferer 创建推断器。
func NewMediaTagInferer() *MediaTagInferer {
	return &MediaTagInferer{}
}

// TagInput 推断输入（§59.26：对齐 auto_feed label_str = 副标题+标题+简介+MI+NFO）。
type TagInput struct {
	MediaInfo   string // MediaInfo 全文
	Title       string // 主标题
	Subtitle    string // 副标题
	Description string // 简介（BBCode）
	NFO         string // NFO/BDInfo
	Size        int64  // §59.72 B2: 种子体积（字节）——big_pack >1TB 判据（AGSV 审核硬规则）
	Statement   string // §59.73: 引用/声明区（源站 quote 块）——特效字幕等发布者声明
	// §59.151: PTGen 产地（labels 串，如 "中国香港"）——港片原声=粤语复合判据
	Region string
}

// Infer 从多源文本推断 MediaTags。
// 返回推断出的标准键列表（如 hdr10/dolby_vision/chinese_subtitle）。
func (i *MediaTagInferer) Infer(mediaInfoText, title, subtitle string) []string {
	// §59.166 修3：副标题通道接通（天空之城"国粤英日四语"站方审核实证——原 Infer
	// 二参形态丢 Subtitle，粤语 ②a 判据的副标题条件永远为空）
	return i.InferFull(TagInput{MediaInfo: mediaInfoText, Title: title, Subtitle: subtitle})
}

// InferFull 从完整输入推断 MediaTags（§59.35 P4：规则查 dict/tag.json）。
// 数据源对齐 auto_feed：副标题+主标题+简介+MI+NFO。
func (i *MediaTagInferer) InferFull(in TagInput) []string {
	tags := titleparser.TagInferMatches(titleparser.TagInferInput{
		Subtitle:    in.Subtitle,
		Title:       in.Title,
		Description: in.Description,
		MediaInfo:   in.MediaInfo,
		NFO:         in.NFO,
		Statement:   in.Statement,
	})
	// §59.117: MI 层查询（MISections 统一信息源）——语言信号从 Audios 层
	// Language/Title 字段直证（跨行 regex 补丁废除）; 数值判据层内化
	// （General["overall bit rate"] / Videos[0]["width"/"frame rate"]）。
	miSec := titleparser.ParseMISections(in.MediaInfo)
	// §59.326: MI 音轨完备门——Audios 非空且至少一轨有语言证据=MI 是
	// 完备音轨清单，desc_raw 通道（TagInferMatches 扫豆瓣简介"◎语 言"=
	// 作品元数据非本种音轨）产出的音频语言标签全部剔除（智齿案：LiTV
	// 版单英语轨，豆瓣"粤语"元数据打了 cantonese_audio 致站方
	// WRONGLY_CANTONESE）；MI 直证随后补上真实轨语言。
	// 语义：冲突信 MI（用户定案：MI > 副标题 > 豆瓣）；一致时 MI 直证
	// 等价补回无损（天空之城副标题声明同 MI 轨）；MI 语言证据缺失
	// （站方 MI 不标 Language）不启用门——副标题声明保留。
	miHasLangEvidence := false
	for _, a := range miSec.Audios {
		if strings.TrimSpace(a["language"]) != "" || strings.TrimSpace(a["title"]) != "" {
			miHasLangEvidence = true
			break
		}
	}
	if len(miSec.Audios) > 0 && miHasLangEvidence {
		audioLangTags := map[string]bool{
			"chinese_audio": true, "cantonese_audio": true, "english_audio": true,
			"japanese_audio": true, "korean_audio": true, "russian_audio": true,
			"french_audio": true, "german_audio": true, "spanish_audio": true,
			"thai_audio": true, "hindi_audio": true, "italian_audio": true,
		}
		filtered := tags[:0]
		for _, t := range tags {
			if !audioLangTags[t] {
				filtered = append(filtered, t)
			}
		}
		tags = filtered
	}
	for _, lt := range inferLanguageFromMIAudios(miSec) {
		if !containsStr(tags, lt) {
			tags = append(tags, lt)
		}
	}
	// §59.151: 粤语复合判据（用户经验 + 反例修正）：
	// ① PTGen 产地香港 × MI 原声轨（Language=Chinese——港片粤语原声；一个好人
	//    反例：English|原声=英语原声不判）
	// ② 副标题"粤"字声明（天空之城"国粤英日四语"）——仅在 MI 无 Mandarin/
	//    国语 Title 反证时生效（迫降航班反例：粤配轨 MI 标记 Mandarin，幸运按
	//    MI Language 判无粤语→勾了=WRONGLY）
	if !containsStr(tags, "cantonese_audio") {
		if hasHKOriginalTrack(miSec, in.Region) ||
			(strings.Contains(in.Subtitle, "粤") && !hasCantoneseDubbedTrapTrack(miSec)) {
			tags = append(tags, "cantonese_audio")
		}
	}
	// §59.151: HDR 族 MI 层结构化判据（dict 文本 regex 已删——MI Video 层
	// HDR format 含 Profile 语义是唯一真相：dvhe.08/09=DV+HDR10 双层 /
	// dvhe.05=仅DV / ST2086=HDR10 / ST2094=HDR10+ / Vivid / HLG / PQ10）
	tags = inferHDRTagsFromMI(miSec, tags)
	// §59.151: 字幕语言 MI Text 段补充（Language 行直证——副标题声明之外的
	// 铁证通道；挽救计划案例：特效字幕声明 ≠ 中字 tag，MI Text 中文轨才是）
	for st := range inferSubtitleFromMITexts(miSec) {
		if !containsStr(tags, st) {
			tags = append(tags, st)
		}
	}
	// §59.319 附二十六: 副标题关键词 + PTGen 产地语言联合推断
	// 设计定案（用户）：零字幕轨=MI 无法提供语言证据 → PTGen 兜底；
	// 有字幕轨时 MI Text 是权威不需要兜底。产地=影片国别（非发行平台）。
	tags = inferRegionLanguageTags(miSec, in, tags)
	// §59.319 附三十一: BDInfo 报告语言/HDR 通道——原盘 MI 恒空（D8 停产），
	// 语言（Audio/Subtitle 行）和 HDR（Video 行标记）信息全在 BDInfo 报告
	// 但引擎无解析通道（Mercy 案：美国电影含中字轨+DoVi HDR 全漏）。
	// 与附十七 inferNumericSpecTagsBDInfo（NFO 数值通道）同构。
	tags = inferBDInfoLanguageHDR(in.NFO, miSec, tags)
	// §59.151 附7（§59.304 修订）: 原此处追加内部标记 lucky_english_audio——被追加
	// 的前提即 english_audio 已在 tags（纯冗余），且落库+Tab1 展示污染（无 dict 词条
	// 显示代码、与"英语"重复显示——Life.Unexpected 案用户报）。站规判定单点收敛到
	// executor.luckptEnglishTagValue（预检/DryRun/上传三链同源），本处不再产标记。
	hasHB, hasHF := inferNumericSpecTagsSections(miSec)
	// §59.319 附十七: BDInfo 数值判据（NFO 通道）——原盘行 MediaInfo 空且
	// BDInfo 文本非 MI 格式，MI sections 无数值（高码/高帧在清除重获后
	// 扫描完成前的 fetch 期推断恒缺）；NFO=BDInfo 报告时从 QUICK SUMMARY
	// 的 Total Bitrate + Video 行补数值，阈值与 §59.69 五判据同源
	if !hasHB || !hasHF {
		if hb, hf := inferNumericSpecTagsBDInfo(in.NFO); hb || hf {
			if hb {
				hasHB = true
			}
			if hf {
				hasHF = true
			}
		}
	}
	// §59.70: 高分——豆瓣评分 ≥8.0（Description 源——PTGen 简介行，
	// "◎豆瓣评分　8.2/10"；无评分/暂无评分不命中）
	hasHR := parseDoubanRatingScore(in.Description) >= 8.0
	has := func(k string) bool {
		for _, t := range tags {
			if t == k {
				return true
			}
		}
		return false
	}
	if hasHB && !has("high_bitrate") {
		tags = append(tags, "high_bitrate")
	}
	if hasHF && !has("high_frame_rate") {
		tags = append(tags, "high_frame_rate")
	}
	if hasHR && !has("high_rating") {
		tags = append(tags, "high_rating")
	}
	// §59.72 B2: 组合判据——连载 = 分集 && 非完结 && 非合集
	// （ubits: 连载=分集资源; 词条 regex 表达不了依赖，代码层）
	if has("episode_split") && !has("complete") && !has("collection") && !has("ongoing") {
		tags = append(tags, "ongoing")
	}
	// §59.72 B2: 大包 = size > 1TB（AGSV 审核脚本硬规则，字面 >1T）
	if in.Size > 1024*1024*1024*1024 && !has("big_pack") {
		tags = append(tags, "big_pack")
	}
	// §59.85: 4K/8K 数值判据——MI Width（与 high_bitrate 同源解析）
	if w := parseMIWidthPixels(in.MediaInfo); w > 0 {
		if w >= 7680 && !has("resolution_8k") {
			tags = append(tags, "resolution_8k")
		} else if w >= 3840 && w < 7680 && !has("resolution_4k") {
			tags = append(tags, "resolution_4k")
		}
	}
	return ApplyTagRules(dedupTags(tags))
}

// containsStr 小工具。
func containsStr(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// inferLanguageFromMIAudios §59.117: MI Audios 层语言判据——Language 字段直证
// （层内无跨行歧义; Title 行语义标识如"台配/央视国配"由文案层 pattern 承载）。
func inferLanguageFromMIAudios(s titleparser.MISections) []string {
	var out []string
	seen := map[string]bool{}
	add := func(k string) {
		if !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	for _, a := range s.Audios {
		lang := strings.ToLower(a["language"])
		title := strings.ToLower(a["title"])
		// §59.151: Commentary 轨排除（§59.114 同源判据——评论轨不承载内容语言）
		if strings.Contains(title, "commentary") || strings.Contains(title, "评论") ||
			strings.Contains(title, "解说") {
			continue
		}
		// Language 字段直证 + Title 行语义标识（"Mandarin (台配)"/"Cantonese"——
		// §59.113 案例库: 幽灵公主 Title: Cantonese 的直证通道）
		// §59.151: 中文 Title 补齐（醉拳2 Title=['粤语','国语','英语'] 实证——
		// 中英文 Title 双通道；'原声'/Title 空 → 泛中文（Lang=Chinese 兜底））
		switch {
		case strings.Contains(lang, "cantonese") || strings.Contains(title, "cantonese") ||
			strings.Contains(title, "粤语") || strings.Contains(title, "广东话"):
			add("cantonese_audio")
		case strings.Contains(lang, "chinese") || strings.Contains(lang, "mandarin") ||
			strings.Contains(title, "mandarin") || strings.Contains(title, "国语") ||
			strings.Contains(title, "国配") || strings.Contains(title, "普通话"):
			add("chinese_audio")
		}
		if strings.Contains(lang, "english") {
			add("english_audio")
		}
		if strings.Contains(lang, "japanese") {
			add("japanese_audio")
		}
		if strings.Contains(lang, "korean") {
			add("korean_audio")
		}
		if strings.Contains(lang, "english") || strings.Contains(title, "english") {
			add("english_audio")
		}
	}
	return out
}

// inferNumericSpecTagsSections §59.117: 数值判据层内化。
func inferNumericSpecTagsSections(s titleparser.MISections) (highBitrate, highFrameRate bool) {
	if len(s.Videos) == 0 {
		return false, false
	}
	rate := rateValueMbps(s.General["overall bit rate"])
	width := parseMIIntStr(s.Videos[0]["width"])
	fps := fpsValue(s.Videos[0]["frame rate"])
	if rate > 0 && width > 0 {
		if width >= 3840 {
			highBitrate = rate >= 15
		} else {
			highBitrate = rate >= 9
		}
	}
	if fps >= 60 {
		highFrameRate = true
	}
	return highBitrate, highFrameRate
}

// rateValueMbps §59.117: 字段值（"15.2 Mb/s"/"9 500 kb/s"）→ Mb/s 归一。
func rateValueMbps(v string) float64 {
	v = strings.TrimSpace(v)
	lower := strings.ToLower(v)
	numEnd := 0
	var numStr string
	for i, c := range lower {
		if (c >= '0' && c <= '9') || c == '.' || c == ' ' {
			numEnd = i + 1
		} else {
			break
		}
	}
	numStr = strings.ReplaceAll(v[:numEnd], " ", "")
	var n float64
	fmtSscanf(numStr, "%f", &n)
	switch {
	case strings.Contains(lower, "gb/s"):
		return n * 1000
	case strings.Contains(lower, "kb/s"):
		return n / 1000
	default:
		return n
	}
}

// fpsValue §59.117: 字段值（"60.000 FPS"/"23.976 FPS"）→ float。
func fpsValue(v string) float64 {
	numEnd := 0
	for i, c := range v {
		if (c >= '0' && c <= '9') || c == '.' {
			numEnd = i + 1
		} else {
			break
		}
	}
	var n float64
	fmtSscanf(v[:numEnd], "%f", &n)
	return n
}

func fmtSscanf(s string, format string, args ...interface{}) {
	_, _ = fmt.Sscanf(s, format, args...)
}

func parseMIIntStr(s string) int {
	var n int
	digit := false
	for _, c := range strings.ReplaceAll(s, " ", "") {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
			digit = true
		} else if digit {
			break
		}
	}
	return n
}

var (
	miWidthRe          = regexp.MustCompile(`(?im)^\s*Width\s*:\s*([\d\s]+)\s*pixels`)
)

// parseMIWidthPixels 解析 Video 段 Width（像素，含千分位空格）。
func parseMIWidthPixels(mi string) int {
	m := miWidthRe.FindStringSubmatch(mi)
	if m == nil {
		return 0
	}
	n, _ := strconv.Atoi(strings.ReplaceAll(m[1], " ", ""))
	return n
}

// doubanRatingLineRe §59.70: PTGen 豌豆评分行——全角/半角空格、"N.N"或"N.N/10" 形态。
// IMDb 行因前缀不同（◎IMDb评分）天然不匹配。
var doubanRatingLineRe = regexp.MustCompile(`◎豆瓣评分[\s\x{3000}]+([0-9]+(?:\.[0-9]+)?)(?:\s*/\s*10)?`)

// parseDoubanRatingScore 从简介文本解析豆瓣评分（无行/暂无评分返回 0）。
func parseDoubanRatingScore(desc string) float64 {
	m := doubanRatingLineRe.FindStringSubmatch(desc)
	if m == nil {
		return 0
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0
	}
	return v
}

// dedupTags 保序去重。
func dedupTags(tags []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range tags {
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}


// inferHDRTagsFromMI §59.151: HDR 族标签 MI 层结构化产出（唯一真相源）。
// 先剔除文本 regex 可能残留的 HDR 族（防双源冲突），再按 MI Video 层
// HDR format 判据单点产出。MI 缺 Video 层/HDR 行时不产任何 HDR 族。
func inferHDRTagsFromMI(s titleparser.MISections, tags []string) []string {
	hdrFamily := map[string]bool{
		"dolby_vision": true, "hdr10": true, "hdr10_plus": true,
		"hdr_vivid": true, "vivid_hdr": true, "hlg": true, "pq10": true,
	}
	out := tags[:0]
	for _, t := range tags {
		if !hdrFamily[t] {
			out = append(out, t)
		}
	}
	if len(s.Videos) == 0 {
		return out
	}
	hdr := strings.ToLower(s.Videos[0]["hdr format"])
	if hdr == "" {
		return out
	}
	hasDoVi := strings.Contains(hdr, "dolby vision") || strings.Contains(hdr, "dovi") ||
		strings.Contains(hdr, "dvhe") || strings.Contains(hdr, "dvha") || strings.Contains(hdr, "dvav")
	hasHDR10Plus := strings.Contains(hdr, "hdr10+") || strings.Contains(hdr, "smpte st 2094")
	hasHDR10 := strings.Contains(hdr, "hdr10") || strings.Contains(hdr, "smpte st 2086")
	hasVivid := strings.Contains(hdr, "hdr vivid") || strings.Contains(hdr, "hdrvivid")
	hasHLG := strings.Contains(hdr, "hlg")
	hasPQ10 := strings.Contains(hdr, "pq10")
	if hasDoVi {
		out = append(out, "dolby_vision")
	}
	if hasHDR10Plus {
		out = append(out, "hdr10_plus")
	} else if hasDoVi && (strings.Contains(hdr, "dvhe.08") || strings.Contains(hdr, "dvhe.07")) {
		// Profile 7/8 = DV+HDR10 双层（BL 兼容层 HDR10——MI 尾段
		// "SMPTE ST 2086, Version HDR10, HDR10 compatible" 明示）。
		// §59.154：P9 摘出双勾组——Dolby Profiles v1.5 官方表 P9=8-bit AVC+SDR 兼容
		// （标志 dvav.09；不勾 hdr10，仅 DV），与 P7/P8 语义不同。
		// §59.306：P7 补线——UHD 原盘 Remux（dvhe.07.06 BL+EL+RPU）BL 同为 HDR10
		// 兼容，曾仅 P8 特例致中南海保镖 P7 Remux 未勾 HDR10（幸运种审拒绝）。
		out = append(out, "hdr10")
	} else if hasHDR10 && !hasDoVi {
		out = append(out, "hdr10")
	}
	if hasVivid {
		// §59.267: 产出 dict canonical 键 hdr_vivid（vivid_hdr 旧形态无 dict 条目，
		// Tab1 ③标签显示原文代码——243 莫离 HDR.Vivid 案）
		out = append(out, "hdr_vivid")
	}
	if hasHLG {
		out = append(out, "hlg")
	}
	if hasPQ10 {
		out = append(out, "pq10")
	}
	return out
}


// inferSubtitleFromMITexts §59.151: MI Text 段字幕语言（Language 行直证）。
// Commentary 排除同 §59.114 判据（评论字幕不承载内容语言）。
func inferSubtitleFromMITexts(s titleparser.MISections) map[string]bool {
	out := map[string]bool{}
	for _, t := range s.Texts {
		lang := strings.ToLower(t["language"])
		title := strings.ToLower(t["title"])
		if strings.Contains(title, "commentary") || strings.Contains(title, "评论") {
			continue
		}
		// §59.151: Language+Title 双通道（Title-only 形态防御——CHS/CHT/简/繁
		// 无 Language 行的压制组标记；48/48 假纯英语料的 Text 实证）
		// §59.326: 词形补齐——iTunes 系 MI 的字幕轨 Language 用
		// "Mandarin (Hans)/(Hant)"/"Cantonese (Hant)" 形态（不含 chinese），
		// Title 用 "Chinese (Simplified)" 全词（词表原只认 chs/cht 缩写）——
		// 鸡皮疙瘩 37 字幕轨中字全漏案。
		cnSub := containsAny(lang, "chinese", "mandarin", "cantonese") ||
			regexAny(title, []string{"chs", "cht", "简", "繁", "中文", "chinese", "mandarin", "cantonese"})
		enSub := strings.Contains(lang, "english") || strings.Contains(title, "eng")
		if cnSub {
			out["chinese_subtitle"] = true
		}
		if enSub {
			out["english_subtitle"] = true
		}
	}
	return out
}


// hasHKOriginalTrack §59.151: 产地香港 × MI 原声轨（Language=Chinese）→ 粤语。
// 一个好人反例：English|原声=英语原声（成龙港片的英语轨标记）不判粤语。
func hasHKOriginalTrack(s titleparser.MISections, region string) bool {
	if !strings.Contains(region, "香港") {
		return false
	}
	for _, a := range s.Audios {
		if strings.TrimSpace(a["title"]) == "原声" &&
			strings.Contains(strings.ToLower(a["language"]), "chinese") {
			return true
		}
	}
	return false
}

// hasMandarinTitledTrack §59.151: MI Audio Title 含 Mandarin/国语标记（副标题
// 粤字声明的反证——迫降航班粤配轨 MI 标记 Mandarin，幸运按 Language 判）。
func hasMandarinTitledTrack(s titleparser.MISections) bool {
	for _, a := range s.Audios {
		t := strings.ToLower(a["title"])
		if strings.Contains(t, "mandarin") || strings.Contains(t, "国语") || strings.Contains(t, "国配") {
			return true
		}
	}
	return false
}

// hasCantoneseDubbedTrapTrack §59.166 反证收窄（英雄本色2 站方审核实证）：
// 原 Mandarin Title 即反证误伤多语并存（"Title: Mandarin"=国语轨与粤语轨并存
// 是正常形态）；真陷阱仅"声明粤实为国配"的轨道——Title 同时含 Mandarin 系
// **与粤字**（迫降形态 "Mandarin (粤配)"）。
func hasCantoneseDubbedTrapTrack(s titleparser.MISections) bool {
	for _, a := range s.Audios {
		t := strings.ToLower(a["title"])
		mandarin := strings.Contains(t, "mandarin") || strings.Contains(t, "国语") || strings.Contains(t, "国配")
		if mandarin && strings.Contains(t, "粤") {
			return true
		}
	}
	return false
}


// regexAny 子串命中任一（小写化后匹配——CHS/CHT 形态）。
func regexAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// inferNumericSpecTagsBDInfo §59.319 附十七：BDInfo 报告（NFO 通道）数值
// 判据——QUICK SUMMARY 的 "Total Bitrate: 42.27 Mbps" + Video 行
// "2160p / 23.976 fps"。阈值与 inferNumericSpecTagsSections 同源
// （≥4K 且 ≥15Mb/s；<4K 且 ≥9Mb/s；fps≥60）。
func inferNumericSpecTagsBDInfo(nfo string) (highBitrate, highFrameRate bool) {
	if !strings.Contains(nfo, "QUICK SUMMARY:") {
		return false, false
	}
	// 码率：Total Bitrate: 42.27 Mbps
	rate := 0.0
	if m := regexp.MustCompile(`Total Bitrate:\s*([\d.]+)\s*Mbps`).FindStringSubmatch(nfo); m != nil {
		fmt.Sscanf(m[1], "%f", &rate)
	}
	// 分辨率+帧率：Video 行 "MPEG-H HEVC Video / 81085 kbps / 2160p / 23.976 fps / ..."
	is4K := false
	fps := 0.0
	for _, line := range strings.Split(nfo, "\n") {
		trimmed := strings.TrimPrefix(strings.TrimSpace(line), "* ")
		if !strings.HasPrefix(trimmed, "Video: ") {
			continue
		}
		if m := regexp.MustCompile(` (\d{3,4}[pi]) / `).FindStringSubmatch(trimmed); m != nil {
			is4K = m[1] == "2160p" || m[1] == "4320p"
		}
		if m := regexp.MustCompile(` ([\d.]+) fps`).FindStringSubmatch(trimmed); m != nil {
			fmt.Sscanf(m[1], "%f", &fps)
		}
		break // 主 Video 行（首条非隐藏）
	}
	if rate > 0 {
		if is4K {
			highBitrate = rate >= 15
		} else {
			highBitrate = rate >= 9
		}
	}
	if fps >= 60 {
		highFrameRate = true
	}
	return highBitrate, highFrameRate
}

// inferRegionLanguageTags §59.319 附二十六: 副标题关键词 + PTGen 产地
// 语言联合推断——用户定案规则：
//
// 副标题路径（任何时候，不依赖零字幕轨）：
//   副标题含 普通话/国语 → chinese_audio
//   副标题含 粤语       → cantonese_audio
//
// PTGen 联合回退（仅零字幕轨——MI 无语言证据时兜底）：
//   零字幕轨 + 产地(中国/港/台) + 语言(汉语/普通话/国语) → chinese_audio
//   零字幕轨 + 产地(香港) + 语言(粤语)                 → cantonese_audio
//   零字幕轨 + 产地(中国/港/台)                        → chinese_subtitle（硬字幕推定）
//
// 产地=影片国别（非发行平台）——产地中国=中国影片，零字幕轨=Bilibili
// 等流媒体平台烧字幕，硬字幕推定安全（零误报）。
func inferRegionLanguageTags(miSec titleparser.MISections, in TagInput, tags []string) []string {
	has := func(k string) bool { return containsStr(tags, k) }

	// §59.326: MI 音轨完备门（用户三级定案：MI > 副标题 > 豆瓣元数据）——
	// MI 有音轨语言证据时 A/B 音频声明让位：冲突信 MI（智齿案：LiTV 单
	// 英语轨 × 副标题"国语"/豆瓣"粤语"全让位）；一致时 MI 直证已打标签，
	// !has 门自然跳过无损（天空之城副标题声明同 MI 轨）。
	miHasLangEvidence := false
	for _, a := range miSec.Audios {
		if strings.TrimSpace(a["language"]) != "" || strings.TrimSpace(a["title"]) != "" {
			miHasLangEvidence = true
			break
		}
	}

	// A. 副标题关键词（MI 无音轨语言证据时——源站声明是直接证据）
	subLower := strings.ToLower(in.Subtitle)
	if !miHasLangEvidence {
		if !has("chinese_audio") && containsAny(subLower, "普通话", "国语") {
			tags = append(tags, "chinese_audio")
		}
		if !has("cantonese_audio") && containsAny(subLower, "粤语", "广东话") {
			tags = append(tags, "cantonese_audio")
		}
	}

	// B. PTGen 联合回退（仅零字幕轨——MI 无语言证据时兜底）
	if len(miSec.Texts) > 0 {
		return tags // 有字幕轨——MI Text 已提供权威证据，PTGen 不兜底
	}

	// §59.319 附二十六补: Region 兜底——fetch 时 TagInput 未传 Region
	// （调用点未填），从 Description "◎产　地　XX" 解析
	regionLower := strings.ToLower(in.Region)
	if regionLower == "" && in.Description != "" {
		descLower := strings.ToLower(in.Description)
		// ◎产　地　中国大陆 / ◎产地 中国香港 等变体
		if m := regexp.MustCompile(`◎产[　地]*\s*([^\n]+)`).FindStringSubmatch(in.Description); m != nil {
			regionLower = strings.ToLower(strings.TrimSpace(m[1]))
		} else if strings.Contains(descLower, "产地") {
			// 宽松兜底
			regionLower = descLower
		}
	}
	isChineseRegion := containsAny(regionLower, "中国大陆", "中国香港", "中国台湾", "大陆", "香港", "台湾", "中国")
	if !isChineseRegion {
		return tags
	}

	descLower := strings.ToLower(in.Description)
	// 中文音频：产地中文区 + 语言(汉语/普通话/国语)——MI 完备门让位（§59.326）
	if !miHasLangEvidence && !has("chinese_audio") && containsAny(descLower, "汉语普通话", "汉语", "普通话", "国语") {
		tags = append(tags, "chinese_audio")
	}
	// 粤语音频：产地香港 + 语言(粤语)——MI 完备门让位（§59.326）
	if !miHasLangEvidence && !has("cantonese_audio") && containsAny(regionLower, "香港") && strings.Contains(descLower, "粤语") {
		tags = append(tags, "cantonese_audio")
	}
	// 硬字幕推定：产地中文区 + 零字幕轨（中国影片无软字幕=几乎确定硬字幕）
	if !has("chinese_subtitle") {
		tags = append(tags, "chinese_subtitle")
	}
	return tags
}


// inferBDInfoLanguageHDR §59.319 附三十一: BDInfo 报告语言/HDR 推断通道。
// 原盘 MI 恒空（D8 停产）→ miSec 无 Text/Audio/Video 段 → 语言/HDR 推断
// 全灭。BDInfo 报告（QUICK SUMMARY 格式）包含：
//   Video 行: "MPEG-H HEVC Video / 81085 kbps / 2160p / ... / HDR10 / BT.2020"
//   * Video 隐藏行: "... / Dolby Vision / BT.2020"（DoVi 双层盘增强层）
//   Audio 行: "Chinese / Dolby TrueHD/Atmos Audio / 7.1 / ..."
//   Subtitle 行: "Chinese / 29.516 kbps"
// 解析这些行产出语言/HDR 标签——MI 已有证据时优先（不重复）。
func inferBDInfoLanguageHDR(nfo string, miSec titleparser.MISections, tags []string) []string {
	if !strings.Contains(nfo, "QUICK SUMMARY:") {
		return tags
	}
	has := func(k string) bool { return containsStr(tags, k) }

	miHasAudio := len(miSec.Audios) > 0
	miHasText := len(miSec.Texts) > 0
	miHasVideo := len(miSec.Videos) > 0

	// Audio 行语言
	if !miHasAudio {
		if bdAudioLangs := extractBDInfoAudioLangs(nfo); len(bdAudioLangs) > 0 {
			for lang := range bdAudioLangs {
				switch {
				case lang == "chinese":
					if !has("chinese_audio") {
						tags = append(tags, "chinese_audio")
						has = func(k string) bool { return containsStr(tags, k) }
					}
				case lang == "cantonese":
					if !has("cantonese_audio") {
						tags = append(tags, "cantonese_audio")
						has = func(k string) bool { return containsStr(tags, k) }
					}
				case lang == "english":
					if !has("english_audio") {
						tags = append(tags, "english_audio")
						has = func(k string) bool { return containsStr(tags, k) }
					}
				case lang == "japanese":
					if !has("japanese_audio") {
						tags = append(tags, "japanese_audio")
						has = func(k string) bool { return containsStr(tags, k) }
					}
				case lang == "korean":
					if !has("korean_audio") {
						tags = append(tags, "korean_audio")
						has = func(k string) bool { return containsStr(tags, k) }
					}
				}
			}
		}
	}

	// Subtitle 行语言
	if !miHasText {
		if bdSubLangs := extractBDInfoSubtitleLangs(nfo); len(bdSubLangs) > 0 {
			for lang := range bdSubLangs {
				switch {
				case lang == "chinese":
					if !has("chinese_subtitle") {
						tags = append(tags, "chinese_subtitle")
					}
				case lang == "english":
					if !has("english_subtitle") {
						tags = append(tags, "english_subtitle")
					}
				}
			}
		}
	}

	// Video 行 HDR 标记
	if !miHasVideo {
		if hdr := extractBDInfoHDR(nfo); hdr != "" {
			switch hdr {
			case "DoVi HDR10+":
				if !has("dolby_vision") {
					tags = append(tags, "dolby_vision")
				}
				if !has("hdr10_plus") {
					tags = append(tags, "hdr10_plus")
				}
			case "DoVi HDR":
				if !has("dolby_vision") {
					tags = append(tags, "dolby_vision")
				}
				if !has("hdr10") {
					tags = append(tags, "hdr10")
				}
			case "DoVi":
				if !has("dolby_vision") {
					tags = append(tags, "dolby_vision")
				}
			case "HDR10+":
				if !has("hdr10_plus") {
					tags = append(tags, "hdr10_plus")
				}
			case "HDR10":
				if !has("hdr10") {
					tags = append(tags, "hdr10")
				}
			case "HDR Vivid":
				if !has("hdr_vivid") {
					tags = append(tags, "hdr_vivid")
				}
			}
		}
	}
	return tags
}

// extractBDInfoAudioLangs 从 BDInfo 报告 Audio 行提取语言集合。
// 行格式: "Audio: Chinese / Dolby TrueHD/Atmos Audio / 7.1 / 48 kHz / ..."
// 语言在首个 " / " 分隔段。
func extractBDInfoAudioLangs(nfo string) map[string]string {
	langs := map[string]string{}
	for _, line := range strings.Split(nfo, "\n") {
		trimmed := strings.TrimPrefix(strings.TrimSpace(line), "* ")
		if !strings.HasPrefix(trimmed, "Audio: ") {
			continue
		}
		rest := strings.TrimPrefix(trimmed, "Audio: ")
		langRaw := strings.TrimSpace(strings.SplitN(rest, "/", 2)[0])
		lang := mapBDInfoLang(langRaw)
		if lang != "" {
			langs[lang] = langRaw
		}
	}
	return langs
}

// extractBDInfoSubtitleLangs 从 BDInfo 报告 Subtitle 行提取语言集合。
func extractBDInfoSubtitleLangs(nfo string) map[string]string {
	langs := map[string]string{}
	for _, line := range strings.Split(nfo, "\n") {
		trimmed := strings.TrimPrefix(strings.TrimSpace(line), "* ")
		if !strings.HasPrefix(trimmed, "Subtitle: ") {
			continue
		}
		rest := strings.TrimPrefix(trimmed, "Subtitle: ")
		langRaw := strings.TrimSpace(strings.SplitN(rest, "/", 2)[0])
		lang := mapBDInfoLang(langRaw)
		if lang != "" {
			langs[lang] = langRaw
		}
	}
	return langs
}

// mapBDInfoLang BDInfo 语言名 → canonical。
func mapBDInfoLang(raw string) string {
	lower := strings.ToLower(strings.TrimSpace(raw))
	switch {
	case lower == "chinese" || strings.HasPrefix(lower, "chinese"):
		return "chinese"
	case lower == "mandarin":
		return "chinese"
	case lower == "cantonese":
		return "cantonese"
	case lower == "english":
		return "english"
	case lower == "japanese":
		return "japanese"
	case lower == "korean":
		return "korean"
	}
	return ""
}

// extractBDInfoHDR 从 BDInfo Video 行提取 HDR 类型。
// 判据与 ExtractBDInfo 的 hdrFromBD 同构：
//   主行 HDR10 + 隐藏行 Dolby Vision → "DoVi HDR"
//   主行 HDR10+ → "HDR10+"（+隐藏 DV → "DoVi HDR10+"）
//   主行 Dolby Vision（无 HDR10）→ "DoVi"
//   主行 HDR10 → "HDR10"
//   主行 HDR Vivid → "HDR Vivid"
func extractBDInfoHDR(nfo string) string {
	mainHDR := ""
	hasDoVi := false
	for _, line := range strings.Split(nfo, "\n") {
		trimmed := strings.TrimSpace(line)
		hidden := strings.HasPrefix(trimmed, "* ")
		if hidden {
			trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "*"))
		}
		if !strings.HasPrefix(trimmed, "Video: ") {
			continue
		}
		if hidden {
			if strings.Contains(trimmed, "Dolby Vision") {
				hasDoVi = true
			}
			continue // 隐藏行只提取 DV 标记
		}
		// 主 Video 行
		if strings.Contains(trimmed, "HDR10+") {
			mainHDR = "HDR10+"
		} else if strings.Contains(trimmed, "HDR10") {
			mainHDR = "HDR10"
		} else if strings.Contains(trimmed, "Dolby Vision") {
			mainHDR = "DoVi" // 单层 DV
		} else if strings.Contains(trimmed, "HDR Vivid") {
			mainHDR = "HDR Vivid"
		}
		// 主 Video 行标记后继续——隐藏行在主行之后，不能 break
		// （DoVi 双层盘：主行 HDR10 + 隐藏行 Dolby Vision）
	}
	if hasDoVi {
		switch mainHDR {
		case "HDR10+":
			return "DoVi HDR10+"
		case "HDR10":
			return "DoVi HDR"
		default:
			return "DoVi"
		}
	}
	return mainHDR
}
