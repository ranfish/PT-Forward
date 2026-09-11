package reseed

import (
	"strings"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.187 ①: 组名同族豁免——UBbits/UBits 同映射变体不反驳。
func TestGroupVariantFamily(t *testing.T) {
	const size = int64(11927356482)
	// 回到未来II 实证形态：孤儿署名 -UBbits，站方标题 -UBits
	src := "回到未来II.Back.To.The.Future.Part.II.1989.BluRay.1080P.x264.DTS-HD.MA.5.1-UBbits"
	cand := &model.SeedingSearchResult{TorrentID: "109991",
		Title: "Back To The Future Part II 1989 BluRay 1080P x264 DTS-HD MA 5.1-UBits",
		Size:  11929271664} // 差 1.83MB < 显示窗

	// 无 resolver：退化为纯字符串比较 → 反驳（回到未来II 当日实况）
	m, stats := VerifyMatchWithStatsAndSource([]*model.SeedingSearchResult{cand}, "UBbits", size, src)
	if m != nil || stats.GroupRefute != 1 {
		t.Fatalf("无 resolver 应反驳: m=%v stats=%+v", m, stats)
	}

	// 注入同族 resolver：豁免反驳 → 标题词+size 确认通过
	GroupFamilyResolver = func(g string) string {
		if g == "UBits" || g == "UBbits" || g == "UBWEB" {
			return "ubits.club"
		}
		return ""
	}
	defer func() { GroupFamilyResolver = nil }()
	m2, stats2 := VerifyMatchWithStatsAndSource([]*model.SeedingSearchResult{cand}, "UBbits", size, src)
	if m2 == nil || m2.TorrentID != "109991" {
		t.Fatalf("同族豁免应放行 tid=109991: m=%v stats=%+v", m2, stats2)
	}

	// 非同族（映射不同站）：仍反驳
	candHD := &model.SeedingSearchResult{TorrentID: "x", Title: "Movie 2019 1080p x264-UBits", Size: size}
	GroupFamilyResolver = func(g string) string {
		if g == "UBbits" {
			return "ubits.club"
		}
		return "other.site" // UBits 映射到别的站——不同族
	}
	m3, stats3 := VerifyMatchWithStatsAndSource([]*model.SeedingSearchResult{candHD}, "UBbits", size, "Movie.2019.1080p.x264-UBbits")
	if m3 != nil || stats3.GroupRefute != 1 {
		t.Fatalf("异族应反驳: m=%v stats=%+v", m3, stats3)
	}
}

// §59.187 ③: 混合词归一化。
func TestNormalizeMixedKeyword(t *testing.T) {
	cases := []struct{ in, want string }{
		{"神秘友友IF 2024 1080P", "神秘友友 2024 1080P"}, // 粘连拆分+IF 剔除（用户实测词）
		{"IF 2024 1080P", ""},           // 无中文锚 → 跳过（裸英文噪音防护）
		{"Cinderella 1950 1080p", ""},     // 无混合词 → 跳过
		{"冰冻星球BBC 2011 1080p", "冰冻星球 BBC 2011 1080p"}, // BBC 3 字符保留
	}
	for _, c := range cases {
		if got := NormalizeMixedKeyword(c.in); got != c.want {
			t.Errorf("NormalizeMixedKeyword(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

// §59.188: 搜索关键词剥版本词——加勒比 tid=105267 案（发布名含 PROPER 站方标题无 → AND 0 结果）。
func TestStripVersionTokens(t *testing.T) {
	// 加勒比实证形态：PROPER 在年份与分辨率之间
	kw := ExtractSearchKeyword("Pirates.of.the.Caribbean.Dead.Man's.Chest.2006.PROPER.2160p.UHD.Blu-ray.HDR10.HEVC.Atmos.TrueHD.7.1-DIY@UBits")
	for _, must := range []string{"Pirates", "Chest", "2006", "2160p", "UHD"} {
		if !strings.Contains(kw, must) {
			t.Errorf("keyword %q 缺 %q", kw, must)
		}
	}
	if strings.Contains(kw, "PROPER") {
		t.Errorf("keyword %q 应剥除 PROPER", kw)
	}

	// REPACK 在分辨率后（IF 案形态）——truncateToResolution 已天然排除，双保险断言
	kw2 := ExtractSearchKeyword("IF.2024.1080p.Blu-ray.REPACK.AVC.TrueHD.Atmos.7.1-DIY@UBits")
	if strings.Contains(kw2, "REPACK") {
		t.Errorf("keyword %q 应无 REPACK", kw2)
	}

	// 变体 REPACK2 / 单元函数
	if got := stripVersionTokens("Movie 2019 REPACK2 1080p INTERNAL PROPER"); got != "Movie 2019 1080p" {
		t.Errorf("stripVersionTokens=%q", got)
	}
	// 无版本词不变
	if got := stripVersionTokens("Cinderella 1950 1080p"); got != "Cinderella 1950 1080p" {
		t.Errorf("无版本词误剥: %q", got)
	}
}
