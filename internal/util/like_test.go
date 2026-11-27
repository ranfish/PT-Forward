package util

import (
	"testing"
)

// TestLikeEscapeOrder 13 刀3: 转义顺序铁律锚——\ → \\ 必须最先。
// 若顺序颠倒（先 % 后 \），后继引入的 \% 被二次转义为 \\%——
// LIKE 匹配字面反斜杠+百分号而非 %。
func TestLikeEscapeOrder(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{`a%b`, `a\%b`},
		{`a_b`, `a\_b`},
		{`a\b`, `a\\b`},
		{`100%done`, `100\%done`},
		{`Foo_Bar`, `Foo\_Bar`},
		// 顺序复合：\% 三字符——正确产出 \\%（\ → \\·% → \%）
		{`a\%b`, `a\\\%b`},
		{`a\_b`, `a\\\_b`},
		{"", ""},
		{`abc`, `abc`},
	}
	for _, c := range cases {
		if got := LikeEscape(c.in); got != c.want {
			t.Errorf("LikeEscape(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestLikeContainsPattern 13 刀3: 包含匹配模式工厂——%转义值%。
func TestLikeContainsPattern(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"foo", `%foo%`},
		{"50%", `%50\%%`},
		{"a_b", `%a\_b%`},
		{`\x`, `%\\x%`},
	}
	for _, c := range cases {
		if got := LikeContains(c.in); got != c.want {
			t.Errorf("LikeContains(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
