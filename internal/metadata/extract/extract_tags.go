package extract

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// extractTags 从详情页提取标签（NexusPHP <a class="tag">）。
func (p *PublicExtractor) extractTags(doc *goquery.Document) []string {
	tags := []string{}
	seen := make(map[string]struct{})

	// 主路径：<a class="tag*">
	doc.Find(`a[class*="tag"]`).Each(func(_ int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if text == "" {
			return
		}
		if _, ok := seen[text]; ok {
			return
		}
		seen[text] = struct{}{}
		tags = append(tags, text)
	})

	// 补充：<span class="tag*">
	doc.Find(`span[class*="tag"]`).Each(func(_ int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if text == "" {
			return
		}
		if _, ok := seen[text]; ok {
			return
		}
		seen[text] = struct{}{}
		tags = append(tags, text)
	})

	return tags
}
