package adapter

import (
	"html"
	"regexp"
	"strconv"
	"strings"
)

var reSizeValue = regexp.MustCompile(`([\d.]+)\s*(TB|GB|MB|KB|TiB|GiB|MiB|KiB|B)`)

func parseSizeString(s string) int64 {
	s = strings.TrimSpace(s)
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, "\u00a0", " ")
	if s == "" {
		return 0
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return n
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return int64(f)
	}
	m := reSizeValue.FindStringSubmatch(s)
	if len(m) < 3 {
		return 0
	}
	return sizeToBytes(m[1], m[2])
}

func sizeToBytes(val string, unit string) int64 {
	var multiplier float64
	switch strings.ToUpper(unit) {
	case "TB", "TIB":
		multiplier = 1 << 40
	case "GB", "GIB":
		multiplier = 1 << 30
	case "MB", "MIB":
		multiplier = 1 << 20
	case "KB", "KIB":
		multiplier = 1 << 10
	case "B":
		multiplier = 1
	default:
		return 0
	}
	var f float64
	for _, c := range val {
		if c >= '0' && c <= '9' {
			f = f*10 + float64(c-'0')
		} else if c == '.' {
			fraction := 0.1
			for _, fc := range val[strings.Index(val, ".")+1:] {
				if fc >= '0' && fc <= '9' {
					f += float64(fc-'0') * fraction
					fraction *= 0.1
				}
			}
			break
		}
	}
	return int64(f * multiplier)
}

var reStripTags = regexp.MustCompile(`<[^>]+>`)

func stripHTMLTags(s string) string {
	return reStripTags.ReplaceAllString(s, "")
}

func cleanText(s string) string {
	s = stripHTMLTags(s)
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, "\u00a0", " ")
	return strings.TrimSpace(s)
}
