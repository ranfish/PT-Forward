package extract

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var (
	// 标题工具 ID 后缀（PT-Forward 现有清洗逻辑）。
	titleToolIDSuffixRe = regexp.MustCompile(`\s*\|(?:atu|dtu|pter|FRDS)$`)
	whitespaceCollapse  = regexp.MustCompile(`\s+`)
)

// extractTitle 从详情页提取标题。
// 多 selector 回退（PT-Plugin-Plus 风格）：
//   1. <h1 id="top"> 标题
//   2. <h2> / <h3> 标题
//   3. <title> 标签
//   4. Fallback: 输入参数 fallbackTitle
func (p *PublicExtractor) extractTitle(doc *goquery.Document, fallbackTitle string) string {
	selectors := []string{
		`#top`,
		`h1#top`,
		`h1.title`,
		`h1`,
		`h2`,
		`h3`,
		`title`,
	}
	for _, sel := range selectors {
		s := doc.Find(sel).First()
		if s.Length() == 0 {
			continue
		}
		title := strings.TrimSpace(s.Text())
		title = whitespaceCollapse.ReplaceAllString(title, " ")
		title = titleToolIDSuffixRe.ReplaceAllString(title, "")
		title = strings.TrimSpace(title)
		if title != "" {
			return title
		}
	}
	return fallbackTitle
}
