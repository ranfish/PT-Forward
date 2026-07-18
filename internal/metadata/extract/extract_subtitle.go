package extract

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var (
	subtitleDDRe    = regexp.MustCompile(`(?s)副标题</dt>\s*<dd>(.*?)</dd>`)
	subtitleSpanRe  = regexp.MustCompile(`(?s)<span[^>]*class=['"][^'"]*subtitle[^'"]*['"][^>]*>(.*?)</span>`)
)

// extractSubtitle 从详情页提取副标题。
// 三级回退：
//  1. goquery 找 dt 含"副标题"的 dd / .subtitle 类 span
//  2. 正则扫 HTML（兼容不规范 DOM）
//  3. 空字符串（不强行填充）
func (p *PublicExtractor) extractSubtitle(doc *goquery.Document, htmlStr string) string {
	// 主路径 1: dt/dd
	var subtitle string
	doc.Find("dt").Each(func(_ int, dt *goquery.Selection) {
		if subtitle != "" {
			return
		}
		if !strings.Contains(dt.Text(), "副标题") && !strings.Contains(dt.Text(), "副標題") {
			return
		}
		dd := dt.NextFiltered("dd")
		if dd.Length() > 0 {
			subtitle = strings.TrimSpace(dd.Text())
		}
	})
	if subtitle != "" {
		return subtitle
	}

	// 主路径 2: .subtitle 类 span
	doc.Find(`span[class*="subtitle"]`).Each(func(_ int, s *goquery.Selection) {
		if subtitle != "" {
			return
		}
		subtitle = strings.TrimSpace(s.Text())
	})
	if subtitle != "" {
		return subtitle
	}

	// Fallback: 正则
	if m := subtitleDDRe.FindStringSubmatch(htmlStr); len(m) > 1 {
		s := stripTags(m[1])
		s = strings.TrimSpace(s)
		if s != "" {
			return s
		}
	}
	if m := subtitleSpanRe.FindStringSubmatch(htmlStr); len(m) > 1 {
		s := stripTags(m[1])
		s = strings.TrimSpace(s)
		if s != "" {
			return s
		}
	}
	return ""
}

// stripTags 简单移除 HTML 标签（仅用于正则 fallback 输出清洗）。
func stripTags(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	return b.String()
}
