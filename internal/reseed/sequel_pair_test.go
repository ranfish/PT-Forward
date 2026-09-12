package reseed

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.205 多部合集续集号对——I.II 合集形态不参与续集反驳。
func TestSequelPairCollection(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"色即是空I.II合集.2002-2007.韩语中字￡CMCT如烟", 0},
		{"Sex.Is.Zero.I.II.2002-2007.720p.HDTV.x264.AC3-CMCT", 0},
		{"Movie.II.2000.1080p-CMCT", 2},   // 单续集号保持
		{"Movie.2.2001.720p-CMCT", 0},     // 独立数字续集仅 CJK 路识别（既有语义保持）
		{"生死格斗.2006.中英字幕￡CMCT", 0}, // 无对（原即 0）
		{"指环王1.2.3合集", 1},              // 数字合集不检测（策略1 取 CJK 后 1）
		{"Movie.2002-2007.Collection", 0}, // 年份区间非对（多数字）
	}
	for _, c := range cases {
		if got := extractSequelNumber(c.in); got != c.want {
			t.Errorf("extractSequelNumber(%q) = %d, want %d", c.in, got, c.want)
		}
	}

	// 端到端：色即是空 源 × 家园候选（同 size）→ 放行
	src := "色即是空I.II合集.2002-2007.韩语中字￡CMCT如烟"
	m, stats := VerifyMatchWithTruncationCheckAndSource(
		[]*model.SeedingSearchResult{{
			TorrentID: "t1",
			Title:     "Sex Is Zero I II 2002-2007 720p HDTV x264 AC3-CMCT",
			Size:      3468186091,
		}}, "CMCT", 3465178912, src)
	if m == nil {
		t.Fatalf("collection pack must pass (was sequel_refuted), stats=%+v", stats)
	}
}
