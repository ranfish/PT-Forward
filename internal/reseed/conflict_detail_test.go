package reseed

import (
	"fmt"
	"testing"
	"github.com/ranfish/pt-forward/internal/titleparser"
)

func TestConflictDetailForTarget(t *testing.T) {
	srcTitle := "Just.Mercy.2019.2160p.UHD.Blu-ray.DoVi.HDR10.HEVC.TrueHD.7.1.Atmos-DIY@UBits"
	src := titleparser.ParseTitleTech(srcTitle)

	// 模拟 tid=109323 的完整标题（中文前缀+英文后缀）
	full := "[热门]【DIY 原盘 00884】正义的慈悲 / 以公义之名 Just Mercy 2019 2160p UHD Blu-ray DoVi HDR10 HEVC TrueHD 7.1-DIY@UBits"
	stripped := stripBracketsForVersionCheck(full)
	fmt.Printf("完整标题 (%d bytes): %s\n", len(full), full)
	fmt.Printf("剥后标题 (%d bytes): %s\n\n", len(stripped), stripped)

	cand := titleparser.ParseTitleTech(full)
	fmt.Printf("源:   Res=%q Codec=%q Source=%q HDR=%q AudioCodec=%q\n", src.Resolution, src.VideoCodec, src.SourceType, src.HDR, src.AudioCodec)
	fmt.Printf("候选: Res=%q Codec=%q Source=%q HDR=%q AudioCodec=%q\n\n", cand.Resolution, cand.VideoCodec, cand.SourceType, cand.HDR, cand.AudioCodec)

	// 逐个规则检查
	conflict := techProfileConflict(src, full)
	fmt.Printf("techProfileConflict(完整标题) = %v\n", conflict)

	// 手动检查各字段
	if src.Resolution != "" && cand.Resolution != "" && src.Resolution != cand.Resolution {
		fmt.Printf("  → Resolution conflict: %q vs %q\n", src.Resolution, cand.Resolution)
	}
	if src.VideoCodec != "" && cand.VideoCodec != "" && src.VideoCodec != cand.VideoCodec {
		fmt.Printf("  → VideoCodec conflict: %q vs %q\n", src.VideoCodec, cand.VideoCodec)
	}
	if src.SourceType != "" && cand.SourceType != "" && src.SourceType != cand.SourceType {
		fmt.Printf("  → SourceType conflict: %q vs %q\n", src.SourceType, cand.SourceType)
	}
	if src.HDR != "" && cand.HDR != "" && src.HDR != cand.HDR {
		fmt.Printf("  → HDR conflict: %q vs %q\n", src.HDR, cand.HDR)
	}
	if src.AudioCodec != "" && cand.AudioCodec != "" && src.AudioCodec != cand.AudioCodec {
		fmt.Printf("  → AudioCodec conflict: %q vs %q\n", src.AudioCodec, cand.AudioCodec)
	}
	
	// 也测试 techProfileConflict 用剥后标题
	conflictStripped := techProfileConflict(src, stripped)
	fmt.Printf("\ntechProfileConflict(剥后标题) = %v\n", conflictStripped)
}
