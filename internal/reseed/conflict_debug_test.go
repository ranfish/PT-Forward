package reseed

import (
	"fmt"
	"testing"
	"github.com/ranfish/pt-forward/internal/titleparser"
)

func TestDebugConflictPath(t *testing.T) {
	srcTitle := "Just.Mercy.2019.2160p.UHD.Blu-ray.DoVi.HDR10.HEVC.TrueHD.7.1.Atmos-DIY@UBits"
	src := titleparser.ParseTitleTech(srcTitle)
	fmt.Printf("源: Res=%q Codec=%q Source=%q HDR=%q\n", src.Resolution, src.VideoCodec, src.SourceType, src.HDR)

	candidates := []string{
		// 优堡搜索的 4 条结果（英文标题近似——用适配器返回的实际标题格式）
		"Just Mercy 2019 UHD BluRay 2160p REPACK DV HDR x265 10bit Atmos TrueHD 7.1-Ubits",
		"Just Mercy 2019 2160p UHD BluRay REMUX DoVi HDR10 HEVC Atmos TrueHD 7.1-Ubits",
		"[热门]【DIY 原盘 00884】正义的慈悲 / 以公之名 Just Mercy 2019 2160p UHD Blu-ray DoVi HDR10 HEVC TrueHD 7.1-DIY@UBits",
		"Just Mercy 2019 2160p UHD Blu-ray HEVC Atmos TrueHD7.1-DiY@HDHome",
	}

	for i, cand := range candidates {
		c := titleparser.ParseTitleTech(cand)
		fmt.Printf("\n候选%d: %s...\n", i+1, cand[:60])
		fmt.Printf("  Res=%q Codec=%q Source=%q HDR=%q\n", c.Resolution, c.VideoCodec, c.SourceType, c.HDR)

		// techProfileConflict
		conflict := techProfileConflict(src, cand)
		fmt.Printf("  techProfileConflict=%v\n", conflict)

		// techProfileVersionDefined (with bracket stripping)
		blocked := techProfileVersionDefined(src, cand)
		fmt.Printf("  versionBlocked=%v\n", blocked)

		// CompareSizeDisplay
		sizeOK := CompareSizeDisplay(72633544168, 72638634393)
		fmt.Printf("  sizeCompare(0.007%%)=%v\n", sizeOK)
	}
}
