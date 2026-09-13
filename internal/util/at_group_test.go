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
