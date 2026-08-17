// Package publish MediaTag 推断器（§56.22 Q1 + §59.26 增强 + §59.35 P4 查表化）。
//
// 推断规则数据源：dict/tag.json（Layer 1 tag 域，canonical/infer_pattern/infer_scope）。
// 本文件只保留输入编排 + ApplyTagRules 后处理（覆盖/互斥规则）。
package publish

import (
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
	})
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
