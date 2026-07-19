// Package publish MediaTag 推断器（§56.22 Q1）。
//
// 从 MediaInfo 文本 + 标题推断 MediaTags（38 个标准键的子集）。
// 参考 easy-upload MEDIA_TAG_RULES。
package publish

import "strings"

// MediaTagInferer 从 MediaInfo + 标题推断 MediaTags。
type MediaTagInferer struct{}

// NewMediaTagInferer 创建推断器。
func NewMediaTagInferer() *MediaTagInferer {
	return &MediaTagInferer{}
}

// Infer 从 MediaInfo 文本 + 标题推断 MediaTags。
// 返回推断出的标准键列表（如 hdr10/dolby_vision/chinese_subtitle）。
func (i *MediaTagInferer) Infer(mediaInfoText, title string) []string {
	mi := strings.ToLower(mediaInfoText)
	titleLower := strings.ToLower(title)
	var tags []string

	// === HDR/DV 系列 ===
	if strings.Contains(mi, "dolby vision") || strings.Contains(titleLower, "dovi") || strings.Contains(titleLower, "dv ") {
		tags = append(tags, "dolby_vision")
	}
	if strings.Contains(mi, "hdr10+") || strings.Contains(titleLower, "hdr10+") {
		tags = append(tags, "hdr10_plus")
	} else if strings.Contains(mi, "hdr10") || strings.Contains(titleLower, "hdr10") {
		tags = append(tags, "hdr10")
	}
	if strings.Contains(mi, "hlg") || strings.Contains(titleLower, "hlg") {
		tags = append(tags, "hlg")
	}

	// === 音频系列 ===
	if strings.Contains(mi, "dolby atmos") || strings.Contains(titleLower, "atmos") {
		tags = append(tags, "dolby_atmos")
	}
	if strings.Contains(mi, "dts:x") || strings.Contains(mi, "dts:x") || strings.Contains(titleLower, "dts:x") {
		tags = append(tags, "dts_x")
	}

	// === 中字/国语/粤语 ===
	if containsAny(titleLower, "中字", "chs", "chinese subtitle", "中文字幕", "简繁", "简体中字") {
		tags = append(tags, "chinese_subtitle")
	}
	if containsAny(titleLower, "国语", "普通话", "国配", "chinese audio", "mandarin") {
		tags = append(tags, "chinese_audio")
	}
	if containsAny(titleLower, "粤语", "粤", "cantonese") {
		tags = append(tags, "cantonese_audio")
	}

	// === 版本系列 ===
	if strings.Contains(titleLower, "diy") {
		tags = append(tags, "diy")
	}
	if strings.Contains(titleLower, "remux") {
		tags = append(tags, "remux")
	}
	if strings.Contains(titleLower, "10bit") || strings.Contains(mi, "10 bit") || strings.Contains(mi, "10-bit") {
		tags = append(tags, "10_bit")
	}

	return ApplyTagRules(tags)
}
