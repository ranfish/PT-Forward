package util

import "testing"

// §59.249: 用户 22 词表（方括号+圆括号×11 形态）全量+防误伤
func TestUserWordsFullCoverage(t *testing.T) {
	words := []string{
		"[30%]", "[50%]", "[75%]", "[2x]", "[2x50%]", "[free]", "[2xfree]", "[免费]", "[2x免费]", "[普通]",
		"(30%)", "(50%)", "(75%)", "(2x)", "(2x50%)", "(free)", "(2xfree)", "(免费)", "(2x免费)", "(普通)",
	}
	for _, w := range words {
		title := "Movie.2023.1080p.BluRay.x264-CMCT " + w
		if got := StripSiteOperationMarkers(title); got != "Movie.2023.1080p.BluRay.x264-CMCT" {
			t.Errorf("漏剥 %s → %q", w, got)
		}
	}
	// 防误伤回归
	safe := []struct{ in, want string }{
		{"火车梦.2025-CMCT 铁路梦影(港)", "火车梦.2025-CMCT 铁路梦影(港)"},       // (港) 内容
		{"Movie.2023.1080p-CMCT  (已审)", "Movie.2023.1080p-CMCT"},              // 状态
		{"M.1080p-FRDS [50%] [禁转]", "M.1080p-FRDS [禁转]"},                    // 禁转保留
		{"Movie.2023.1080p.BluRay.x264-CMCT", "Movie.2023.1080p.BluRay.x264-CMCT"}, // 无标注
	}
	for _, c := range safe {
		if got := StripSiteOperationMarkers(c.in); got != c.want {
			t.Errorf("误剥 %q → %q, want %q", c.in, got, c.want)
		}
	}
}
