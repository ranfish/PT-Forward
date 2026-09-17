package util

import "testing"

// §59.248: 标题净化族二——圆括号站方状态剥除+内容性括号保留
func TestStripSiteStateMarkers(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Ilo.Ilo.2013.TWN.BluRay.1080p.x264.DDP.5.1-CMCT  (已审)", "Ilo.Ilo.2013.TWN.BluRay.1080p.x264.DDP.5.1-CMCT"},
		{"Movie.2023.1080p-CMCT (已复核)", "Movie.2023.1080p-CMCT"},
		{"Movie.2023.1080p-CMCT (待审)", "Movie.2023.1080p-CMCT"},
		// 内容性括号保留（译名地区标注）
		{"火车梦.2025.1080p-CMCT 铁路梦影(港)", "火车梦.2025.1080p-CMCT 铁路梦影(港)"},
		// 跨族混排（状态+运营）
		{"Movie.2023.1080p-CMCT (已审) [50%]", "Movie.2023.1080p-CMCT"},
		// 既有族一回归
		{"Movie.2023.1080p-Yumi@FRDS\u00a0\u00a0 [2X 50%]", "Movie.2023.1080p-Yumi@FRDS"},
	}
	for _, c := range cases {
		if got := StripSiteOperationMarkers(c.in); got != c.want {
			t.Errorf("Strip(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
