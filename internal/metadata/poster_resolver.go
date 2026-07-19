// Package metadata 海报 fallback 解析器（§56.18 决策 3）。
//
// 2 级自动 fallback（按 toggle 模式）：
//   ptgen_first（默认）: PTGen.PosterURL > detail_source.poster
//   detail_first:        detail_source.poster > PTGen.PosterURL
//
// 用户 URL 覆盖走 UserOverrides（pipeline.go 已有机制），不在此处理。
// rehost（图床转存）由 pipeline.rehostPoster 处理，PosterResolver 只做选择。
package metadata

import (
	"context"

	"github.com/ranfish/pt-forward/internal/model"
)

// PosterSource 海报来源（URL + 来源标注）。
type PosterSource struct {
	URL    string `json:"url"`
	Source string `json:"source"` // "ptgen" / "detail" / ""（空表示无可用海报）
}

// PosterResolver 海报 fallback 解析器。
type PosterResolver struct{}

// NewPosterResolver 创建 PosterResolver。
func NewPosterResolver() *PosterResolver {
	return &PosterResolver{}
}

// Resolve 按 toggle 模式解析海报 URL。
//   ptgen_first（默认）: PTGen > detail
//   detail_first:        detail > PTGen
// 失败返回空 PosterSource（用户可在 UI 手动填写 URL）。
func (r *PosterResolver) Resolve(
	_ context.Context,
	ptgen *model.PTGenResult,
	detailPoster string,
	mode MergeMode,
) *PosterSource {
	ptgenURL := ""
	if ptgen != nil {
		ptgenURL = ptgen.PosterURL
	}

	// 按 toggle 模式确定优先级
	var first, second string
	var firstSrc, secondSrc string
	if mode == MergeModeDetailFirst {
		first, firstSrc = detailPoster, SourceDetail
		second, secondSrc = ptgenURL, SourcePTGen
	} else {
		// 默认 ptgen_first
		first, firstSrc = ptgenURL, SourcePTGen
		second, secondSrc = detailPoster, SourceDetail
	}

	if first != "" {
		return &PosterSource{URL: first, Source: firstSrc}
	}
	if second != "" {
		return &PosterSource{URL: second, Source: secondSrc}
	}
	return &PosterSource{URL: "", Source: ""}
}
