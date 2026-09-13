package reseed

import (
	"testing"
)

// §59.206 字母+数字粘连拆分——Aliens2 案（词级匹配失效+截断失明双修）。
func TestAlphaDigitGlueSplit(t *testing.T) {
	cases := []struct{ in, want string }{
		// 主案：拆分+截断收敛为 5 词（探针 8 行含目标 tid=4343）
		{"Aliens2.1986.Specia. Edition.BluRay.x264.DTS.MiniBD1080P-CMCT", "Aliens 2 1986 Specia Edition"},
		{"Aliens3.1992.Special.Edition.BluRay.x264.DTS.MiniBD1080P-CMCT", "Aliens 3 1992 Special Edition"},
		// 技术词免疫（字母段<3）
		{"Movie.x264.H265.AC3.DD5.2020.1080p-CMCT", "Movie 2020"},
		// 常规回归
		{"Dearest.Anita.2019.Bluray.1080p.REMUX.AVC.TrueHD.7.1.2Audios-CMCT", "Dearest Anita 2019 1080p"},
		{"侠女.1970.1080p.国语.简繁中字￡CMCT陆判", "侠女 1970 1080p"},
	}
	for _, c := range cases {
		if got := ExtractSearchKeyword(c.in); got != c.want {
			t.Errorf("ExtractSearchKeyword(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
