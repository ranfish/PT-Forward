package reseed

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/titleparser"
)

// §59.190 挂账兑现回归——午夜凶铃案：4K修复版 Res 假冲突六站候选全灭，
// 修复后 Res 置空 → 组名+size 确认放行（size 仲裁 720p/1080p 版本）。
func TestMidnightRingEdition4KMatch(t *testing.T) {
	src := "午夜凶铃.4K修复版.1998.简繁中字￡CMCT梦幻"
	if p := titleparser.ParseTitleTech(src); p.Resolution != "" {
		t.Fatalf("source Resolution = %q, want empty (edition 4K)", p.Resolution)
	}
	const size = int64(8842575872)
	results := []*model.SeedingSearchResult{
		{TorrentID: "720", Title: "Ring REMASTERED 1998 GBR BluRay 720p x264 AC3-CMCT", Size: 4123456789},
		{TorrentID: "1080", Title: "Ring REMASTERED 1998 GBR BluRay 1080p x264 DTS-CMCT", Size: size},
	}
	m, stats := VerifyMatchWithTruncationCheckAndSource(results, "CMCT", size, src)
	if m == nil || m.TorrentID != "1080" {
		t.Fatalf("expected 1080 tid by size, got %+v (stats=%+v)", m, stats)
	}
	// §59.307 后 720p 的反驳归因从 SizeRefute 移至 VersionRefute（候选 GBR 地区码
	// 经 TechProfile 链恢复提取→techProfileVersionDefined 先于 size 门命中——归因
	// 更精确：GBR 版本定义差异而非尺寸）。语义不变：720p 被拒+1080 命中。
	if stats.SizeRefute+stats.VersionRefute < 1 {
		t.Errorf("720p 未被任何门拒: sizeRefute=%d versionRefute=%d", stats.SizeRefute, stats.VersionRefute)
	}
}
