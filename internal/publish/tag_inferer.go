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
func (i *MediaTagInferer) Infer(mediaInfoText, title string) []string {
	return i.InferFull(TagInput{MediaInfo: mediaInfoText, Title: title})
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
	//（General["overall bit rate"] / Videos[0]["width"/"frame rate"]）。
	miSec := titleparser.ParseMISections(in.MediaInfo)
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
	// §59.151 附7: 英语标签条件产出（lucky TAGS_MISSING_ENGLISH_AUDIO 实证——
	// 纯英语种无国语/粤语/中字时必须产 lucky_english_audio）。
	// Buried.Alive.1990 探针：勾英语→100；不勾→TAGS_MISSING_ENGLISH_AUDIO 扣分
	if containsStr(tags, "english_audio") && !containsStr(tags, "chinese_audio") &&
		!containsStr(tags, "cantonese_audio") && !containsStr(tags, "chinese_subtitle") {
		tags = append(tags, "lucky_english_audio")
	}
	hasHB, hasHF := inferNumericSpecTagsSections(miSec)
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
	//（ubits: 连载=分集资源; 词条 regex 表达不了依赖，代码层）
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
//（层内无跨行歧义; Title 行语义标识如"台配/央视国配"由文案层 pattern 承载）。
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

// inferNumericSpecTags §59.69: 从 MI 文本解析码率/宽度/帧率做阈值判定。
// MI 数字含千分位空格（"3 840"），单位 Mb/s 或 kb/s。
func inferNumericSpecTags(mi string) (highBitrate, highFrameRate bool) {
	rate := parseMIOverallBitrateMbps(mi)
	width := parseMIWidthPixels(mi)
	fps := parseMIFrameRate(mi)

	if rate > 0 && width > 0 {
		if width >= 3840 { // ≥4K（2160p/4320p，DCI 4096 同覆盖）
			highBitrate = rate >= 15
		} else { // <4K（1080p 及以下）
			highBitrate = rate >= 9
		}
	}
	if fps >= 60 { // 59.940 NTSC 不算（用户定案：字面 ≥60）
		highFrameRate = true
	}
	return highBitrate, highFrameRate
}

var (
	miOverallBitrateRe = regexp.MustCompile(`(?i)Overall bit rate\s*:\s*([\d\s.,]+)\s*(Mb/s|kb/s|Gb/s)`)
	miWidthRe          = regexp.MustCompile(`(?im)^\s*Width\s*:\s*([\d\s]+)\s*pixels`)
	miFpsRe            = regexp.MustCompile(`(?im)^\s*Frame rate\s*:\s*([\d.]+)\s*FPS`)
)

// parseMIOverallBitrateMbps 解析 General 段 Overall bit rate，归一 Mb/s。
func parseMIOverallBitrateMbps(mi string) float64 {
	m := miOverallBitrateRe.FindStringSubmatch(mi)
	if m == nil {
		return 0
	}
	num, _ := strconv.ParseFloat(strings.ReplaceAll(strings.ReplaceAll(m[1], " ", ""), ",", ""), 64)
	if num == 0 {
		return 0
	}
	switch strings.ToLower(m[2]) {
	case "gb/s":
		return num * 1000
	case "kb/s":
		return num / 1000
	default:
		return num
	}
}

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

// parseMIFrameRate 解析 Video 段 Frame rate（取最大值——多段 MI 场景保守）。
func parseMIFrameRate(mi string) float64 {
	var max float64
	for _, m := range miFpsRe.FindAllStringSubmatch(mi, -1) {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil && v > max {
			max = v
		}
	}
	return max
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
		"vivid_hdr": true, "hlg": true, "pq10": true,
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
	} else if hasDoVi && strings.Contains(hdr, "dvhe.08") {
		// Profile 8 = DV+HDR10 双层（BL+RPU——MI 尾段 HDR10 兼容层）。
		// §59.154：P9 摘出双勾组——Dolby Profiles v1.5 官方表 P9=8-bit AVC+SDR 兼容
		// （标志 dvav.09；不勾 hdr10，仅 DV），与 P8 语义不同。
		out = append(out, "hdr10")
	} else if hasHDR10 && !hasDoVi {
		out = append(out, "hdr10")
	}
	if hasVivid {
		out = append(out, "vivid_hdr")
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
		cnSub := strings.Contains(lang, "chinese") || regexAny(title, []string{"chs", "cht", "简", "繁", "中文"})
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
