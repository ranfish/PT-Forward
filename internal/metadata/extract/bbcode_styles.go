package extract

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// applyStylesFromAttr 从 style 属性解析 CSS 并应用到 accumulator。
func applyStylesFromAttr(acc *bbcodeAccumulator, s *goquery.Selection) {
	style, _ := s.Attr("style")
	if style == "" {
		return
	}
	applyStylesFromCSS(acc, style)
}

// applyStylesFromCSS 解析 CSS 内联样式并应用（§56.9 决策 3）。
// 支持 color/font-size/font-weight/font-style/text-decoration/text-align。
func applyStylesFromCSS(acc *bbcodeAccumulator, css string) {
	props := parseInlineStyles(css)

	if v, ok := props["color"]; ok {
		hex := normalizeColorToHex(v)
		if hex != "" {
			acc.add("[color="+hex+"]", "[/color]")
		}
	}
	if v, ok := props["font-size"]; ok {
		size := mapPixelSizeToBBCodeSize(v)
		if size != "" {
			acc.add("[size="+size+"]", "[/size]")
		}
	}
	if v, ok := props["font-weight"]; ok {
		if isBoldFontWeight(v) {
			acc.add("[b]", "[/b]")
		}
	}
	if v, ok := props["font-style"]; ok {
		if v == "italic" || v == "oblique" {
			acc.add("[i]", "[/i]")
		}
	}
	if v, ok := props["text-decoration"]; ok {
		applyTextDecoration(acc, v)
	}
	if v, ok := props["text-align"]; ok {
		switch v {
		case "left":
			acc.add("[left]", "[/left]")
		case "right":
			acc.add("[right]", "[/right]")
		case "center":
			acc.add("[center]", "[/center]")
		}
	}
}

// parseInlineStyles 解析 "color: red; font-size: 14px;" 形式的内联样式。
func parseInlineStyles(css string) map[string]string {
	props := make(map[string]string)
	for _, decl := range strings.Split(css, ";") {
		decl = strings.TrimSpace(decl)
		if decl == "" {
			continue
		}
		colonIdx := strings.Index(decl, ":")
		if colonIdx < 0 {
			continue
		}
		key := strings.TrimSpace(strings.ToLower(decl[:colonIdx]))
		val := strings.TrimSpace(decl[colonIdx+1:])
		if key != "" && val != "" {
			props[key] = val
		}
	}
	return props
}

var (
	rgbRe          = regexp.MustCompile(`(?i)^rgb\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*\)$`)
	hexShortRe     = regexp.MustCompile(`(?i)^#([0-9a-f])([0-9a-f])([0-9a-f])$`)
	hexLongRe      = regexp.MustCompile(`(?i)^#([0-9a-f]{2})([0-9a-f]{2})([0-9a-f]{2})$`)
	fontSizePxRe   = regexp.MustCompile(`(?i)^(\d+(?:\.\d+)?)px$`)
	fontSizePtRe   = regexp.MustCompile(`(?i)^(\d+(?:\.\d+)?)pt$`)
)

// normalizeColorToHex 将 CSS 颜色值归一化为 #rrggbb 格式（§56.9 决策 3）。
// 支持: rgb()/rgba()/#rgb/#rrggbb/颜色名（red/blue/green 等常见 16 个）。
func normalizeColorToHex(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = strings.ToLower(s)

	if m := rgbRe.FindStringSubmatch(s); m != nil {
		r, _ := strconv.Atoi(m[1])
		g, _ := strconv.Atoi(m[2])
		b, _ := strconv.Atoi(m[3])
		return fmt.Sprintf("#%02x%02x%02x", clampByte(r), clampByte(g), clampByte(b))
	}
	if m := hexLongRe.FindStringSubmatch(s); m != nil {
		return "#" + m[1] + m[2] + m[3]
	}
	if m := hexShortRe.FindStringSubmatch(s); m != nil {
		return "#" + m[1] + m[1] + m[2] + m[2] + m[3] + m[3]
	}
	if hex, ok := namedColors[s]; ok {
		return hex
	}
	return ""
}

// mapPixelSizeToBBCodeSize px/pt → BBCode size 1-7。
// PTNexus 映射: 12px→1, 14px→2, 16px→3, 18px→4, 24px→5, 32px→6, 48px→7。
func mapPixelSizeToBBCodeSize(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var px float64
	if m := fontSizePxRe.FindStringSubmatch(s); m != nil {
		px, _ = strconv.ParseFloat(m[1], 64)
	} else if m := fontSizePtRe.FindStringSubmatch(s); m != nil {
		pt, _ := strconv.ParseFloat(m[1], 64)
		px = pt * 96.0 / 72.0
	} else {
		return ""
	}
	switch {
	case px < 13:
		return "1"
	case px < 15:
		return "2"
	case px < 17:
		return "3"
	case px < 24:
		return "4"
	case px < 32:
		return "5"
	case px < 48:
		return "6"
	default:
		return "7"
	}
}

// isBoldFontWeight font-weight: bold 或数值 ≥ 600。
func isBoldFontWeight(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "bold" {
		return true
	}
	if n, err := strconv.Atoi(v); err == nil && n >= 600 {
		return true
	}
	return false
}

// applyTextDecoration text-decoration 支持多值（underline line-through）。
func applyTextDecoration(acc *bbcodeAccumulator, v string) {
	for _, tok := range strings.Fields(v) {
		switch strings.ToLower(tok) {
		case "underline":
			acc.add("[u]", "[/u]")
		case "line-through":
			acc.add("[s]", "[/s]")
		}
	}
}

func clampByte(n int) int {
	if n < 0 {
		return 0
	}
	if n > 255 {
		return 255
	}
	return n
}

// namedColors CSS 基本颜色名（最常用 16 个）。
var namedColors = map[string]string{
	"black":   "#000000",
	"white":   "#ffffff",
	"red":     "#ff0000",
	"lime":    "#00ff00",
	"blue":    "#0000ff",
	"yellow":  "#ffff00",
	"cyan":    "#00ffff",
	"magenta": "#ff00ff",
	"silver":  "#c0c0c0",
	"gray":    "#808080",
	"maroon":  "#800000",
	"olive":   "#808000",
	"green":   "#008000",
	"purple":  "#800080",
	"teal":    "#008080",
	"navy":    "#000080",
	"orange":  "#ffa500",
	"pink":    "#ffc0cb",
}
