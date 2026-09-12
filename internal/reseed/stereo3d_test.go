package reseed

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/titleparser"
)

// §59.197 3D 封装反驳——300勇士案：站内 HSBS/HOU 两版同体积同组，
// 验证链此前对封装词全盲（结果顺序定生死）。

func TestExtractStereo3D(t *testing.T) {
	cases := []struct{ in, want string }{
		{"300.Rise.Of.An.Empire.3D.2014.BluRay.1080p.HSBS.x264.DTS-HD.MA7.1-CMCT", "HSBS"},
		{"Movie.3D.2014.BluRay.1080p.H-SBS.x264-GRP", "HSBS"},
		{"Movie.3D.2014.BluRay.1080p.Half-SBS.x264-GRP", "HSBS"},
		{"Movie.3D.2014.BluRay.1080p.HOU.x264-GRP", "HOU"},
		{"Movie.3D.2014.BluRay.1080p.H-OU.x264-GRP", "HOU"},
		{"Movie.3D.2014.BluRay.1080p.Half.OU.x264-GRP", "HOU"},
		{"Movie.3D.2014.BluRay.1080p.SBS.x264-GRP", "SBS"},
		{"Movie.3D.2014.BluRay.1080p.FSBS.x264-GRP", ""}, // FSBS 不匹配（F 前缀无界）
		{"Movie.3D.2014.BluRay.1080p.x264-GRP", ""},      // 无封装词
		{"Movie.3D.2014.BluRay.1080p.x264-GRP", ""},      // "3D" 泛词不算
		{"Miss.You.Already.2014.BluRay.1080p.x264-GRP", ""}, // You 尾词不误判 OU
	}
	for _, c := range cases {
		if got := titleparser.ParseTitleTech(c.in).Stereo3D; got != c.want {
			t.Errorf("Stereo3D(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestStereo3DRefute(t *testing.T) {
	src := "[300勇士：帝国崛起].300.Rise.Of.An.Empire.3D.2014.BluRay.1080p.HSBS.x264.DTS-HD.MA7.1-CMCT"
	const size = int64(14987654321)
	// 搜索结果 HOU 在前（站方按时间倒序、HOU 后发布）——修复前会错配
	results := []*model.SeedingSearchResult{
		{TorrentID: "hou-tid", Title: "300.Rise.Of.An.Empire.3D.2014.BluRay.1080p.HOU.x264.DTS-HD.MA7.1-CMCT", Size: size},
		{TorrentID: "hsbs-tid", Title: "300.Rise.Of.An.Empire.3D.2014.BluRay.1080p.HSBS.x264.DTS-HD.MA7.1-CMCT", Size: size},
	}
	m, stats := VerifyMatchWithTruncationCheckAndSource(results, "CMCT", size, src)
	if m == nil || m.TorrentID != "hsbs-tid" {
		t.Fatalf("expected hsbs-tid, got %+v", m)
	}
	if stats.TechRefute != 1 {
		t.Errorf("TechRefute = %d, want 1 (HOU refuted)", stats.TechRefute)
	}
}

func TestStereo3D_SameAndOmitted(t *testing.T) {
	src := "Movie.3D.2014.BluRay.1080p.H-SBS.x264-CMCT"
	const size = int64(9999999999)
	// 同族书写变体（H-SBS vs HSBS）→ 归一后相等 → 放行
	m, _ := VerifyMatchWithTruncationCheckAndSource(
		[]*model.SeedingSearchResult{{TorrentID: "t1", Title: "Movie.3D.2014.BluRay.1080p.HSBS.x264-CMCT", Size: size}},
		"CMCT", size, src)
	if m == nil || m.TorrentID != "t1" {
		t.Fatalf("same-family variant should pass, got %+v", m)
	}
	// 候选省略封装词（单侧空值）→ 放行（站点省略形态不反驳）
	m2, _ := VerifyMatchWithTruncationCheckAndSource(
		[]*model.SeedingSearchResult{{TorrentID: "t2", Title: "Movie.3D.2014.BluRay.1080p.x264-CMCT", Size: size}},
		"CMCT", size, src)
	if m2 == nil || m2.TorrentID != "t2" {
		t.Fatalf("omitted stereo should pass, got %+v", m2)
	}
}
