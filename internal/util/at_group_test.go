package util

import "testing"

// §59.211 补: £ 署名完整保留（@ 不截断）——OfficialGroupKey 消费。
func TestExtractGroupNamePoundSignature(t *testing.T) {
	cases := []struct{ in, want string }{
		{"克兰弗德S01-S02.Cranford.2007-2009.1080p.Blu-ray.x265.AC3￡cXcY@FRDS", "FRDS"},
		{"根(国英)S01.Roots.1977.1080p.Blu-ray.x265.DTS￡cXcY@FRDS", "FRDS"},
		{"侠女.1970.1080p.国语.简繁中字￡CMCT陆判", "CMCT"},
		{"Movie.2020.BluRay.x264-JK@UBits", "UBits"},
		{"[永安镇故事集].Ripples.of.Life.2021.CHN.BluRay.1080p.x264.DTS-CMCT", "CMCT"},
	}
	for _, c := range cases {
		if got := ExtractGroupName(c.in); got != c.want {
			t.Errorf("ExtractGroupName(%q) = %q, want %q", c.in[:30], got, c.want)
		}
	}
	if ExtractGroupName(cases[0].in) != "FRDS" {
		t.Error("£ 署名应直接产出官方组 FRDS（方案 B 主路径）")
	}
}

// TestSpaceTailAtGroup §59.360 续: 空格尾段 @ 复合回退——合集种子站方
// 标题空格连接组署名形态（pt31 Top138 英雄本色案：DB title
// "…2Audios GREENOTEA@FRDS" 无 dash 无 £，词表层被 @ 跳过+dash/£ 锚
// 全 miss → 空组名 Tab1 "—"）。@ 是无歧义组署名锚，尾段含 @ 即组段。
func TestSpaceTailAtGroup(t *testing.T) {
	cases := []struct {
		name, title, wantFull, wantOfficial string
	}{
		// 本案：空格尾段 @ 复合——full 整段/官方后段
		{"本案空格@复合", "A Better Tomorrow 1986 REMASTERED Bluray 1080p x265 AAC(5.1) 2Audios GREENOTEA@FRDS", "GREENOTEA@FRDS", "FRDS"},
		// DIY@ 通用前缀剥离（dash 分支两版一致语义）
		{"空格尾段DIY@", "Some Movie 2020 1080p DIY@CMCT", "CMCT", "CMCT"},
		// 不误伤：普通标题尾词无 @
		{"尾词无@", "Some Movie 2020 1080p BluRay x265", "", ""},
		// 不触发：@ 在非尾段
		{"@非尾段", "foo@bar Some Movie 2020", "", ""},
	}
	for _, c := range cases {
		if got := ExtractGroupNameFull(c.title); got != c.wantFull {
			t.Errorf("%s: Full(%q) = %q, want %q", c.name, c.title, got, c.wantFull)
		}
		if got := ExtractGroupName(c.title); got != c.wantOfficial {
			t.Errorf("%s: Official(%q) = %q, want %q", c.name, c.title, got, c.wantOfficial)
		}
	}
}
