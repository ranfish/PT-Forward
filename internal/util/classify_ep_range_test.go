package util

import "testing"

// §59.187 ②: EP 范围签名——S01.EP01-E06 判 season_pack（原误 unknown）。
func TestClassifyEPRange(t *testing.T) {
	cases := []struct {
		title    string
		wantForm string
	}{
		{"Obi-Wan.Kenobi.2022.S01.EP01-E06.BluRay.1080p.x265.10bit.DTS-HDMA.5.1-UBits", "season_pack"},
		{"Show.2020.S01.E01-E12.1080p.WEB-DL.x264-GRP", "season_pack"},
		{"Show.2020.S02.EP03-E06.1080p.WEB-DL.x264-GRP", "partial_pack"}, // 起始非 1
		{"Show.2020.S01.EP01-E06.COMPLETE.1080p-GRP", "season_pack"},   // COMPLETE 原路径不回归
		{"Show.2020.S01.1080p.WEB-DL.x264-GRP", "unknown"},              // 无范围仍 unknown
	}
	for _, c := range cases {
		got := ClassifyTorrent(c.title, nil)
		if got.Form != c.wantForm {
			t.Errorf("ClassifyTorrent(%q).Form=%q want %q", c.title, got.Form, c.wantForm)
		}
	}
}
