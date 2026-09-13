package reseed

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/titleparser"
)

// §59.207 品牌血统反驳——克拉之膝案：站内 CC/FLAC 与 DTS 两版同 size 同组，
// 品牌是唯一判别器（结果顺序曾错选 CC 版 recheck 0%）。
func TestBrandEditionRefute(t *testing.T) {
	src := "克拉之膝.1970.1080p.法语中字￡CMCT风潇潇"
	if ed := titleparser.ParseTitleTech("Claire's.Knee.1970.CC.BluRay.1080p.x264.FLAC-CMCT").EditionInfo; !isBrandEdition(ed) {
		t.Fatalf("CC EditionInfo = %q, want brand", ed)
	}
	const size = int64(10737418240)
	const local = int64(10737604242)
	// CC 版在前（站方结果顺序）——品牌反驳出局，DTS 版胜出
	results := []*model.SeedingSearchResult{
		{TorrentID: "309436", Title: "Claire's.Knee.1970.CC.BluRay.1080p.x264.FLAC-CMCT", Size: size},
		{TorrentID: "73609", Title: "Claires.Knee.1970.BluRay.1080p.x264.DTS-CMCT", Size: size},
	}
	m, stats := VerifyMatchWithTruncationCheckAndSource(results, "CMCT", local, src)
	if m == nil || m.TorrentID != "73609" {
		t.Fatalf("expected DTS tid 73609, got %+v (stats=%+v)", m, stats)
	}
	// 回归①：源带 CC × 候选带 CC（奇爱博士形态）→ 不反驳
	m2, _ := VerifyMatchWithTruncationCheckAndSource(
		[]*model.SeedingSearchResult{{TorrentID: "t1", Title: "Dr.Strangelove.1964.CC.BluRay.720p.x264-CMCT", Size: size}},
		"CMCT", size, "奇爱博士.1964.CC.720p.中英字幕￡CMCT风潇潇")
	if m2 == nil {
		t.Fatal("CC×CC should pass")
	}
	// 回归②：营销版式词仍不反驳（§59.191 b 保持——侠女 REMASTER 形态）
	m3, _ := VerifyMatchWithTruncationCheckAndSource(
		[]*model.SeedingSearchResult{{TorrentID: "t2", Title: "A.Touch.of.Zen.1970.4K.REMASTER.HKG.BluRay.1080p.x264.FLAC-CMCT", Size: size}},
		"CMCT", size, "侠女.1970.1080p.国语.简繁中字￡CMCT陆判")
	if m3 == nil {
		t.Fatal("marketing edition (REMASTER) must stay un-refuted")
	}
}

// §59.214 品牌反驳相对化——密阳案回归：唯一候选即 CC 版（中文名不写 CC
// 是命名习惯），无兄弟时品牌词不构成判别器，放行。
func TestBrandEditionRelativeMiyang(t *testing.T) {
	src := "密阳.2007.简体中字￡CMCT暮雨潇潇"
	const local = int64(4079791986) // 真实本地（与站方差 42KB——不走门6 短路）
	rows := []*model.SeedingSearchResult{
		{TorrentID: "424501", Title: "Secret.Sunshine.2007.JPN.4K.REMASTERED.BluRay.1080p.x264.DTS-CMCT", Size: 13314398617},
		{TorrentID: "422927", Title: "Milyang.2007.REMASTERED.1080p.JPN.Blu-ray.AVC.DTS-HD.MA.5.1-Anonymous", Size: 44710609551},
		{TorrentID: "411889", Title: "Secret.Sunshine.2007.1080p.BluRay.DD+5.1.x265.10bit-PTer", Size: 15214921646},
		{TorrentID: "70698", Title: "Secret.Sunshine.2007.CC.BluRay.1080p.AVC.DTS-HD.MA.5.1-blucook@CHDBits", Size: 45193793372},
		{TorrentID: "2670", Title: "Secret.Sunshine.2007.CC.BluRay.720p.x264.AC3-CMCT", Size: 4080218931},
	}
	m, _ := VerifyMatchWithTruncationCheckAndSource(rows, "CMCT", local, src)
	if m == nil || m.TorrentID != "2670" {
		t.Fatalf("密阳唯一正确候选（CC 版）应放行, got %+v", m)
	}
}
