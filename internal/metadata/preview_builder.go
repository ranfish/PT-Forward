// Package metadata 字段预览构建器（§56.24 决策 2/Q3）。
//
// 从 MergedMetadata + 用户覆盖构建 PreviewResponse，
// 用于 9d reverse mapping UI（显示目标站将接收的最终字段）。
package metadata

import (
	"encoding/json"

	"github.com/ranfish/pt-forward/internal/model"
)

// PreviewBuilder 字段预览构建器。
type PreviewBuilder struct{}

// NewPreviewBuilder 创建构建器。
func NewPreviewBuilder() *PreviewBuilder {
	return &PreviewBuilder{}
}

// BuildPreviewFromMeta 从 TorrentMetadata 构建预览。
// 优先用 merged_json（已有合并结果），否则从三源 JSON 实时合并。
func (b *PreviewBuilder) BuildPreviewFromMeta(
	meta *model.TorrentMetadata,
	userOverrides map[string]string,
	targetSite, mode string,
) *model.PreviewResponse {
	if meta == nil {
		return &model.PreviewResponse{TargetSite: targetSite, Mode: mode}
	}

	// 优先用 merged_json
	if meta.MergedJSON != "" {
		var merged MergedMetadata
		if err := json.Unmarshal([]byte(meta.MergedJSON), &merged); err == nil {
			return b.Build(&merged, userOverrides, targetSite, mode)
		}
	}

	// fallback: 从三源 JSON 实时合并
	detail, _ := UnmarshalDetailSource(meta.DetailSourceJSON)
	ptgen, _ := UnmarshalPTGenSource(meta.PTGenSourceJSON)
	local, _ := UnmarshalLocalSource(meta.LocalSourceJSON)
	merged := Merge(detail, ptgen, local, MergeMode(mode))
	return b.Build(&merged, userOverrides, targetSite, mode)
}

// Build 从 MergedMetadata 构建预览响应。
//   merged: 三源合并后的元数据
//   userOverrides: 用户编辑覆盖（字段名 → 用户值）
//   targetSite: 目标站名（用于标注）
//   mode: 合并模式（ptgen_first/detail_first）
func (b *PreviewBuilder) Build(
	merged *MergedMetadata,
	userOverrides map[string]string,
	targetSite, mode string,
) *model.PreviewResponse {
	if merged == nil {
		return &model.PreviewResponse{
			TargetSite: targetSite,
			Mode:       mode,
		}
	}

	var fields []model.PreviewField

	// === 元数据类 ===
	fields = append(fields, b.buildField("metadata", "title", "标题", merged.Title, userOverrides, merged.SourceOf, true))
	fields = append(fields, b.buildField("metadata", "subtitle", "副标题", merged.Subtitle, userOverrides, merged.SourceOf, false))
	fields = append(fields, b.buildField("metadata", "type", "分类", merged.Type, userOverrides, merged.SourceOf, true))
	fields = append(fields, b.buildField("metadata", "medium", "媒介", merged.Medium, userOverrides, merged.SourceOf, false))
	fields = append(fields, b.buildField("metadata", "video_codec", "视频编码", merged.VideoCodec, userOverrides, merged.SourceOf, true))
	fields = append(fields, b.buildField("metadata", "audio_codec", "音频编码", merged.AudioCodec, userOverrides, merged.SourceOf, false))
	fields = append(fields, b.buildField("metadata", "resolution", "分辨率", merged.Resolution, userOverrides, merged.SourceOf, true))
	fields = append(fields, b.buildField("metadata", "release_group", "制作组", merged.ReleaseGroup, userOverrides, merged.SourceOf, false))

	// === 简介类 ===
	introPreview := truncateForPreview(merged.Intro.Body, 200)
	fields = append(fields, b.buildField("intro", "body", "简介", introPreview, userOverrides, merged.SourceOf, false))
	fields = append(fields, b.buildField("intro", "mediainfo", "MediaInfo", truncateForPreview(merged.MediaInfo, 100), userOverrides, merged.SourceOf, false))
	fields = append(fields, b.buildField("intro", "bdinfo", "BDInfo", truncateForPreview(merged.BDInfo, 100), userOverrides, merged.SourceOf, false))

	// === 媒体类 ===
	fields = append(fields, b.buildField("media", "poster", "海报", merged.Intro.Poster, userOverrides, merged.SourceOf, false))
	screenshotCount := len(merged.Intro.ScreenshotURLs())
	screenshotValue := ""
	if screenshotCount > 0 {
		screenshotValue = pluralize(screenshotCount, "张截图")
	}
	fields = append(fields, b.buildField("media", "screenshots", "截图", screenshotValue, userOverrides, merged.SourceOf, false))

	// === 链接类 ===
	fields = append(fields, b.buildField("links", "imdb_url", "IMDb", merged.IMDbURL, userOverrides, merged.SourceOf, false))
	fields = append(fields, b.buildField("links", "douban_url", "豆瓣", merged.DoubanURL, userOverrides, merged.SourceOf, false))
	fields = append(fields, b.buildField("links", "tmdb_url", "TMDb", merged.TMDbURL, userOverrides, merged.SourceOf, false))

	// === 标签类 ===
	tagValue := ""
	if len(merged.Tags) > 0 {
		tagValue = joinStrings(merged.Tags, ", ")
	}
	fields = append(fields, b.buildField("tags", "tags", "标签", tagValue, userOverrides, merged.SourceOf, false))

	// === 完整度检查 ===
	completeness := b.calcCompleteness(fields)

	return &model.PreviewResponse{
		TargetSite:   targetSite,
		Mode:         mode,
		Fields:       fields,
		Completeness: completeness,
	}
}

// buildField 构建单个字段。
func (b *PreviewBuilder) buildField(
	category, standardKey, label, sourceValue string,
	userOverrides map[string]string,
	sourceOf map[string]string,
	required bool,
) model.PreviewField {
	targetValue := sourceValue
	isUserEdited := false
	source := sourceOf[standardKey]

	// 应用用户覆盖
	if userOverrides != nil {
		if userVal, ok := userOverrides[standardKey]; ok && userVal != "" {
			targetValue = userVal
			isUserEdited = true
			source = "user"
		}
	}

	return model.PreviewField{
		Category:     category,
		StandardKey:  standardKey,
		Label:        label,
		TargetValue:  targetValue,
		SourceValue:  sourceValue,
		Source:       source,
		IsUserEdited: isUserEdited,
		Required:     required,
		Missing:      required && targetValue == "",
	}
}

// calcCompleteness 计算完整度。
func (b *PreviewBuilder) calcCompleteness(fields []model.PreviewField) model.PreviewCompleteness {
	total := len(fields)
	filled := 0
	missing := 0
	for _, f := range fields {
		if f.TargetValue != "" {
			filled++
		}
		if f.Missing {
			missing++
		}
	}
	percent := 0
	if total > 0 {
		percent = filled * 100 / total
	}
	return model.PreviewCompleteness{
		Total:   total,
		Filled:  filled,
		Missing: missing,
		Percent: percent,
	}
}

// truncateForPreview 截断长文本用于预览。
func truncateForPreview(s string, maxRunes int) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "..."
}

// joinStrings 简单的字符串拼接。
func joinStrings(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += sep + parts[i]
	}
	return result
}

// pluralize 中文数量描述。
func pluralize(n int, unit string) string {
	return string(rune('0'+n/10)) + string(rune('0'+n%10)) + unit
}
