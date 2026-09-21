package util

import "testing"

// §59.248 族三/四：时间尾段+组名锚形态免疫（12 用例——新造词/防误伤双覆盖）

func TestStrip_Family34(t *testing.T) {
	cases := []struct{ in, want string }{
		// 族三：裸时间尾
		{"Movie.2024 剩余时间：1天5时", "Movie.2024"},
		{"Movie.2024 [免费] 剩余时间：1天5时", "Movie.2024"},
		{"Movie.2024 优惠剩余:23:59:59", "Movie.2024"},
		// 族三防误伤（前缀词锚）
		{"Movie.剩余时光", "Movie.剩余时光"},
		// 族四：组名锚+新造词形态免疫
		{"Movie.2024.1080p-CMCT [站方新造词]", "Movie.2024.1080p-CMCT"},
		{"Movie.2024.1080p-CMCT [免费] 剩余时间：1天5时 (已审)", "Movie.2024.1080p-CMCT"},
		{"[木星上行].Jupiter.2015-CMCTV [新]（限时）", "[木星上行].Jupiter.2015-CMCTV"},
		// 族四防误伤：组名即结尾/组名后是内容
		{"Movie.2024.1080p-CMCT", "Movie.2024.1080p-CMCT"},
		{"Movie.2024-CMCT.官方版本", "Movie.2024-CMCT.官方版本"},
		{"Movie-CMCTV.2", "Movie-CMCTV.2"},
		// @ 组形态
		{"Movie.2024 10bit-Yumi@FRDS [2X 50%]", "Movie.2024 10bit-Yumi@FRDS"},
	}
	for _, c := range cases {
		if got := StripSiteOperationMarkers(c.in); got != c.want {
			t.Errorf("in=%q got=%q want=%q", c.in, got, c.want)
		}
	}
}
