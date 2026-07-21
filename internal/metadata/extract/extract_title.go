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

	// titlePageSuffixRe <title> 标签后缀清理（v0.0.254 从 keepfrds 提取器通用化）。
	// 模式： ":: 种子详情..." / " - Powered by XXX" 等 NexusPHP 站点通用后缀。
	// 注意：只作用于从 <title> 提取的标题（不影响 h1/h2 提取）。
	titlePageSuffixRe = regexp.MustCompile(`\s*::\s*种子详情.*|\s*::\s*Torrent.*|\s*-\s*Powered.*`)
)

// extractTitle 从详情页提取标题。
// 多 selector 回退（PT-Plugin-Plus 风格）：
//   1. <h1 id="top"> 标题
//   2. <h2> / <h3> 标题
//   3. <title> 标签（带后缀清理）
//   4. Fallback: 输入参数 fallbackTitle
func (p *PublicExtractor) extractTitle(doc *goquery.Document, fallbackTitle string) string {
	selectors := []string{
		`#top`,
		`#torrent-name`, // SSD（不可说）特有
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
		// v0.0.254: <title> 标签特殊后缀清理（keepfrds/hhanclub 等）
		if sel == "title" {
			title = titlePageSuffixRe.ReplaceAllString(title, "")
		}
		title = strings.TrimSpace(title)
		if title != "" {
			return title
		}
	}
	return fallbackTitle
}
