package reseed

import (
	"strings"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/titleparser"
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

// §59.190 ①: 中文版式词归一——爱情万岁 tid=328261 三连误杀复现与修复断言。
func TestChineseRemasterEdition(t *testing.T) {
	const size = int64(11263515487)
	src := "爱情万岁.1994.USA.2K修复版.1080p.国语.简繁中字￡CMCT蒙太奇"
	cand := &model.SeedingSearchResult{TorrentID: "328261",
		Title: "Vive.L'Amour.1994.USA.2K.REMASTERED.BluRay.1080p.x264.FLAC-CMCT",
		Size:  11263551733} // 差 36KB < 1MB 地板
	m, stats := VerifyMatchWithTruncationCheckAndSource([]*model.SeedingSearchResult{cand}, "CMCT", size, src)
	if m == nil {
		t.Fatalf("2K修复版≡REMASTERED 应放行: stats=%+v", stats)
	}
	if stats.VersionRefute != 0 {
		t.Errorf("版本反驳不应触发: %+v", stats)
	}
	// EditionInfo 归一断言
	p := titleparser.ParseTitleTech(src)
	if p.EditionInfo != "Remastered" {
		t.Errorf("中文修复版 EditionInfo=%q want Remastered", p.EditionInfo)
	}
}

// §59.190 ②: 全规格词关键词判无标题 → 中文兜底触发。
func TestKeywordAllTitleless(t *testing.T) {
	for _, kw := range []string{"4K修复版 1998 1080p", "1994 2K修复版 1080p", "1080p", "2K"} {
		if !KeywordHasNoTitle(kw) {
			t.Errorf("%q 应判无标题", kw)
		}
	}
	for _, kw := range []string{"侠女 1970 1080p", "Cinderella 1950 1080p", "2001太空漫游 1080p"} {
		if KeywordHasNoTitle(kw) {
			t.Errorf("%q 应判有标题", kw)
		}
	}
	// 端到端：浪人关键词应含片名（经中文兜底）
	kw := ExtractSearchKeyword("浪人.4K修复版.1998.1080p.国英双语.中英字幕￡CMCT旧梦")
	if !strings.Contains(kw, "浪人") {
		t.Errorf("浪人关键词丢片名: %q", kw)
	}
}

// §59.191 b: EditionInfo 移出版本反驳（反向案三连免疫）；ReleaseVersion 保留。
func TestVersionRefuteEditionOnlyOut(t *testing.T) {
	const size = int64(19928648253) // 守望者正主实尺
	// 反向案形态：源无版式词 × 候选 Ultimate Cut（守望者 tid 实证）
	src := "守望者.2009.GBR.终极剪辑版.1080p.中英字幕￡CMCT蒙太奇"
	cand := &model.SeedingSearchResult{TorrentID: "w1",
		Title: "Watchmen.2009.GBR.The.Ultimate.Cut.BluRay.1080p.x264.DTS-CMCT", Size: size}
	m, stats := VerifyMatchWithTruncationCheckAndSource([]*model.SeedingSearchResult{cand}, "CMCT", size, src)
	if m == nil {
		t.Fatalf("Ultimate Cut 版式词应放行: %+v", stats)
	}

	// 宾虚反向案：源无版式词 × 候选 Collector's Edition
	src2 := "宾虚.国英双语.1959.中英字幕￡CMCT暮雨潇潇"
	cand2 := &model.SeedingSearchResult{TorrentID: "w2",
		Title: "Ben-Hur.1959.Ultimate.Collector's.Edition.BluRay.1080p.x264.DTS.4Audios-CMCT", Size: size}
	m2, _ := VerifyMatchWithTruncationCheckAndSource([]*model.SeedingSearchResult{cand2}, "CMCT", size, src2)
	if m2 == nil {
		t.Fatalf("Collector's Edition 版式词应放行")
	}

	// 卡萨布兰卡反向案：70th Anniversary
	src3 := "卡萨布兰卡.1942.1080p.国英双语.中英字幕￡CMCT风潇潇"
	cand3 := &model.SeedingSearchResult{TorrentID: "w3",
		Title: "Casablanca.1942.70th.Anniversary.BluRay.1080p.x264.DTS.6Audios-CMCT", Size: size}
	m3, stats3 := VerifyMatchWithTruncationCheckAndSource([]*model.SeedingSearchResult{cand3}, "CMCT", size, src3)
	if m3 == nil {
		t.Fatalf("Anniversary 版式词应放行: %+v", stats3)
	}

	// REPACK 重发布标记仍反驳（b 方案保留项）
	src4 := "Movie.2019.1080p.BluRay.x264-GRP"
	cand4 := &model.SeedingSearchResult{TorrentID: "w4",
		Title: "Movie 2019 1080p BluRay REPACK x264-GRP", Size: size + 2*1024*1024} // 窗内非精确——避免门6短路
	m4, stats4 := VerifyMatchWithTruncationCheckAndSource([]*model.SeedingSearchResult{cand4}, "GRP", size, src4)
	if m4 != nil || stats4.VersionRefute != 1 {
		t.Fatalf("REPACK 应仍反驳: m=%v stats=%+v", m4, stats4)
	}
}

// §59.191: titleless 词表扩展——守望者/月光光 关键词丢片名。
func TestKeywordTitlelessExtended(t *testing.T) {
	for _, kw := range []string{"2009 终极剪辑版 1080p", "2021 加长版 720p"} {
		if !KeywordHasNoTitle(kw) {
			t.Errorf("%q 应判无标题", kw)
		}
	}
	// 端到端：守望者/月光光 关键词带片名
	for name, want := range map[string]string{
		"守望者.2009.GBR.终极剪辑版.1080p.中英字幕￡CMCT蒙太奇": "守望者",
		"月光光心慌慌：杀戮.2021.加长版.720p.中英字幕￡CMCT梦回":  "月光光",
	} {
		if kw := ExtractSearchKeyword(name); !strings.Contains(kw, want) {
			t.Errorf("%s 关键词丢片名: %q", want, kw)
		}
	}
}
