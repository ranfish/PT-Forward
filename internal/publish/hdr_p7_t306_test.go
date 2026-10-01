package publish

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/titleparser"
)

// §59.306 DoVi P7 原盘 Remux hdr10 补线——dvhe.07 BL 同为 HDR10 兼容层
// （中南海保镖 GBR UHD Remux 案：幸运种审要求勾 HDR10 未勾）
func Test306_DoViP7_HDR10Tag(t *testing.T) {
	miP7 := "Video #1\nHDR format : Dolby Vision, Version 1.0, Profile 7.6, dvhe.07.06, BL+EL+RPU, no metadata compression, Blu-ray compatible / SMPTE ST 2086, Version HDR10, HDR10 compatible\n"
	inf := NewMediaTagInferer()
	tags := inf.InferFull(TagInput{MediaInfo: miP7})
	hasDV, has10 := false, false
	for _, x := range tags {
		if x == "dolby_vision" {
			hasDV = true
		}
		if x == "hdr10" {
			has10 = true
		}
	}
	if !hasDV || !has10 {
		t.Fatalf("P7 应双勾 dolby_vision+hdr10, got %v", tags)
	}
	// P8 回归（dvhe.08 既有行为不变）
	miP8 := "Video #1\nHDR format : Dolby Vision, Version 1.0, Profile 8.1, dvhe.08.06, BL+RPU, HDR10 compatible / SMPTE ST 2086, Version HDR10, HDR10 compatible\n"
	tags8 := inf.InferFull(TagInput{MediaInfo: miP8})
	found8 := false
	for _, x := range tags8 {
		if x == "hdr10" {
			found8 = true
		}
	}
	if !found8 {
		t.Fatalf("P8 hdr10 回归失败, got %v", tags8)
	}
	// P9 摘出组回归（dvav.09 仅 DV 不勾 hdr10——§59.154）
	miP9 := "Video #1\nHDR format : Dolby Vision, Version 1.0, Profile 9, dvav.09, BL+RPU\n"
	tags9 := inf.InferFull(TagInput{MediaInfo: miP9})
	for _, x := range tags9 {
		if x == "hdr10" {
			t.Fatalf("P9 不应勾 hdr10（SDR 兼容非 HDR10）, got %v", tags9)
		}
	}
	_ = titleparser.MISections{}
}
