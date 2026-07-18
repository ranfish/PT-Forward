package extract

import (
	"testing"
)

func TestNormalizeColorToHex(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"red", "#ff0000"},
		{"blue", "#0000ff"},
		{"#fff", "#ffffff"},
		{"#abc", "#aabbcc"},
		{"#aabbcc", "#aabbcc"},
		{"rgb(255,0,0)", "#ff0000"},
		{"rgb( 0 , 128 , 255 )", "#0080ff"},
		{"RGB(100%,100%,100%)", ""},   // 百分比暂不支持
		{"unknown_color", ""},
		{"", ""},
	}
	for _, c := range cases {
		got := normalizeColorToHex(c.in)
		if got != c.want {
			t.Errorf("normalizeColorToHex(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestMapPixelSizeToBBCodeSize(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"12px", "1"},
		{"14px", "2"},
		{"16px", "3"},
		{"18px", "4"},
		{"24px", "5"},
		{"32px", "6"},
		{"48px", "7"},
		{"100px", "7"},
		{"10px", "1"},
		{"12pt", "2"},  // 12pt * 96/72 = 16px → 3？让我验算: 12*96/72 = 16，size=3
		{"invalid", ""},
		{"", ""},
	}
	for _, c := range cases {
		got := mapPixelSizeToBBCodeSize(c.in)
		// 12pt = 16px → "3"，调整测试期望
		if c.in == "12pt" {
			if got != "3" {
				t.Errorf("12pt → %q, want 3", got)
			}
			continue
		}
		if got != c.want {
			t.Errorf("mapPixelSizeToBBCodeSize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestIsBoldFontWeight(t *testing.T) {
	cases := map[string]bool{
		"bold": true,
		"BOLD": true,
		"600":  true,
		"700":  true,
		"500":  false,
		"normal": false,
		"abc":  false,
	}
	for in, want := range cases {
		if got := isBoldFontWeight(in); got != want {
			t.Errorf("isBoldFontWeight(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestParseInlineStyles(t *testing.T) {
	css := "color: red; font-size: 14px; font-weight: bold"
	props := parseInlineStyles(css)
	if props["color"] != "red" {
		t.Errorf("color mismatch: %q", props["color"])
	}
	if props["font-size"] != "14px" {
		t.Errorf("font-size mismatch: %q", props["font-size"])
	}
	if props["font-weight"] != "bold" {
		t.Errorf("font-weight mismatch: %q", props["font-weight"])
	}

	// 空/畸形
	props = parseInlineStyles("")
	if len(props) != 0 {
		t.Errorf("empty css → empty props, got %v", props)
	}
	props = parseInlineStyles("noseparator")
	if len(props) != 0 {
		t.Errorf("malformed css → empty props, got %v", props)
	}
}
