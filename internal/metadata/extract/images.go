// Package extract 图片提取与分类（§56.12）。
package extract

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var bbcodeImgRe = regexp.MustCompile(`(?i)\[img\](.*?)\[/img\]`)

// dataSrcAttrs 懒加载 data-* 属性优先级（§56.12 决策 7）。
var dataSrcAttrs = []string{"data-src", "data-echo", "data-original", "src"}

// ExtractImages 从描述（HTML+BBCode 混合）提取所有图片 URL。
// 返回归一化、去重、黑白名单过滤后的 URL 数组（顺序保留）。
func ExtractImages(descrHTML, descrBBCode string) []string {
	var rawURLs []string

	// 1. goquery 提取 <img> 标签
	if doc, err := ParseHTML(descrHTML); err == nil {
		doc.Find("img").Each(func(_ int, s *goquery.Selection) {
			if isEmojiImage(s) {
				return
			}
			u := pickImageURL(s)
			if u != "" {
				rawURLs = append(rawURLs, u)
			}
		})
	}

	// 2. BBCode 文本扫描 [img]url[/img]（兜底）
	matches := bbcodeImgRe.FindAllStringSubmatch(descrBBCode, -1)
	for _, m := range matches {
		if len(m) > 1 && strings.TrimSpace(m[1]) != "" {
			rawURLs = append(rawURLs, strings.TrimSpace(m[1]))
		}
	}

	// 3. 归一化 + 去重（保留首次出现顺序）
	seen := make(map[string]struct{})
	normalized := make([]string, 0, len(rawURLs))
	for _, u := range rawURLs {
		n := NormalizeImageURL(u)
		if n == "" {
			continue
		}
		if _, exists := seen[n]; exists {
			continue
		}
		seen[n] = struct{}{}
		normalized = append(normalized, n)
	}

	// 4. 黑白名单过滤
	filtered := make([]string, 0, len(normalized))
	for _, u := range normalized {
		if IsWhitelistedImage(u) {
			filtered = append(filtered, u)
			continue
		}
		if IsUnwantedImage(u) {
			continue
		}
		filtered = append(filtered, u)
	}
	return filtered
}

// NormalizeImageURL 归一化 URL：去 query + 去 fragment + lower + trim slash。
func NormalizeImageURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return strings.ToLower(strings.TrimRight(raw, "/"))
	}
	u.RawQuery = ""
	u.Fragment = ""
	return strings.ToLower(strings.TrimRight(u.String(), "/"))
}

// pickImageURL 从 <img> selection 提取 URL（优先 data-src 系列）。
func pickImageURL(s *goquery.Selection) string {
	for _, attr := range dataSrcAttrs {
		if v, ok := s.Attr(attr); ok && v != "" {
			return v
		}
	}
	return ""
}

// isEmojiImage 判断是否为 emoji 图片（§56.12 决策 8）。
// <img title=":m:" alt=":m:"> → emoji，跳过（保留为字符 :m:）。
func isEmojiImage(s *goquery.Selection) bool {
	alt, _ := s.Attr("alt")
	title, _ := s.Attr("title")
	for _, v := range []string{alt, title} {
		v = strings.TrimSpace(v)
		if v != "" && strings.HasPrefix(v, ":") && strings.HasSuffix(v, ":") {
			return true
		}
	}
	return false
}
