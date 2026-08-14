// Package publish MediaTag 推断器（§56.22 Q1 + §59.26 增强）。
//
// 从 MediaInfo 文本 + 标题 + 副标题 + 简介 + NFO 推断 MediaTags（38 个标准键的子集）。
// 参考 easy-upload MEDIA_TAG_RULES + auto_feed get_label()（朋友站无源站标签时的关键词推断）。
package publish

import (
	"regexp"
	"strings"
)

// MediaTagInferer 从多源文本推断 MediaTags。
type MediaTagInferer struct{}

// NewMediaTagInferer 创建推断器。
func NewMediaTagInferer() *MediaTagInferer {
	return &MediaTagInferer{}
}

// TagInput 推断输入（§59.26：对齐 auto_feed label_str = 副标题+标题+简介+MI+NFO）。
type TagInput struct {
	MediaInfo  string // MediaInfo 全文
	Title      string // 主标题
	Subtitle   string // 副标题
	Description string // 简介（BBCode）
	NFO        string // NFO/BDInfo
}

// mediaTagPatterns 关键词规则（对齐 auto_feed get_label）。
var (
	reChineseSub = regexp.MustCompile(`(?i)([简繁].{0,12}字幕|[简繁中].{0,3}字|简中|DIY.{1,5}字|内封.{0,3}[繁中字]|Presentation Graphics.*Chinese|(Text[^\n]*Chinese)|(subtitles?[^\n]*chs)|(subtitles?[^\n]*mandarin)|(subtitle[^\n]*chinese))`)
	reEnglishSub = regexp.MustCompile(`(?i)(英.{0,12}字幕|英.{0,3}字|内封.{0,3}英.{0,3}字|(Text[^\n]*English)|(subtitles?[^\n]*eng)|(subtitle[^\n]*english)|(Graphics[^\n]*English))`)
	reChineseAud = regexp.MustCompile(`(?i)([^多]国.{0,3}语|国.{0,3}配|台.{0,3}语|台.{0,3}配|(Audio[^\n]*Chinese)|(Audio[^\n]*mandarin))`)
	reCantonese  = regexp.MustCompile(`(?i)(粤.{0,3}语|粤.{0,3}配|(Audio[^\n]*cantonese))`)
	rePTGenMandarin = regexp.MustCompile(`◎语.*?言　汉语普通话`)
	rePTGenCantonese = regexp.MustCompile(`◎语.*?言　粤语`)
)

// Infer 从多源文本推断 MediaTags。
// 返回推断出的标准键列表（如 hdr10/dolby_vision/chinese_subtitle）。
func (i *MediaTagInferer) Infer(mediaInfoText, title string) []string {
	return i.InferFull(TagInput{MediaInfo: mediaInfoText, Title: title})
}

// InferFull 从完整输入推断 MediaTags（§59.26）。
// 数据源对齐 auto_feed：副标题+主标题+简介+MI+NFO 拼接后统一关键词匹配。
func (i *MediaTagInferer) InferFull(in TagInput) []string {
	all := in.Subtitle + in.Title + in.Description + in.MediaInfo + in.NFO
	allLower := strings.ToLower(all)
	miLower := strings.ToLower(in.MediaInfo)
	titleLower := strings.ToLower(in.Title + " " + in.Subtitle)

	var tags []string

	// === HDR/DV 系列 ===
	if strings.Contains(allLower, "dolby vision") || strings.Contains(allLower, "dovi") || strings.Contains(titleLower, "dv ") {
		tags = append(tags, "dolby_vision")
	}
	if strings.Contains(allLower, "hdr10+") {
		tags = append(tags, "hdr10_plus")
	} else if strings.Contains(allLower, "hdr10") {
		tags = append(tags, "hdr10")
	}
	if strings.Contains(allLower, "hlg") {
		tags = append(tags, "hlg")
	}

	// === 音频系列 ===
	if strings.Contains(allLower, "dolby atmos") || strings.Contains(allLower, "atmos") {
		tags = append(tags, "dolby_atmos")
	}
	if strings.Contains(allLower, "dts:x") {
		tags = append(tags, "dts_x")
	}

	// === 字幕系列（auto_feed: 中字/英字，数据源含 MI Text 段）===
	if reChineseSub.MatchString(all) {
		tags = append(tags, "chinese_subtitle")
	}
	if reEnglishSub.MatchString(all) {
		tags = append(tags, "english_subtitle")
	}

	// === 音轨系列（auto_feed: 国语/粤语，排除"国家"误匹配）===
	if m := reChineseAud.FindString(all); m != "" && !strings.Contains(m, "国家") {
		tags = append(tags, "chinese_audio")
	}
	if rePTGenMandarin.MatchString(in.Description) {
		tags = append(tags, "chinese_audio")
	}
	if reCantonese.MatchString(all) || rePTGenCantonese.MatchString(in.Description) {
		tags = append(tags, "cantonese_audio")
	}

	// === 版本系列 ===
	if strings.Contains(titleLower, "diy") {
		tags = append(tags, "diy")
	}
	if strings.Contains(titleLower, "remux") {
		tags = append(tags, "remux")
	}
	if strings.Contains(allLower, "10bit") || strings.Contains(miLower, "10 bit") || strings.Contains(miLower, "10-bit") {
		tags = append(tags, "10_bit")
	}

	// === 完结（auto_feed: S01 无 E / 全N集 / 剧集+原盘）===
	if regexp.MustCompile(`(?i)[\. ]S\d+\b`).MatchString(in.Title) && !regexp.MustCompile(`(?i)S\d+\.?E\d+`).MatchString(in.Title) {
		tags = append(tags, "complete")
	} else if regexp.MustCompile(`全\d+集`).MatchString(in.Subtitle) {
		tags = append(tags, "complete")
	}



	return ApplyTagRules(dedupTags(tags))
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
