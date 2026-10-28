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
		// §59.366: #top 元素语义校验——标准 NP 站 #top 是 h1 自身（锚点），
		// 深度站（dstudio.me）把 id="top" 放在外层 header（容器）——容器
		// .Text() 会拼接标题+促销 span+副题（"Title [50%]中文副题/" 污染）。
		// 非标题标签则跳过继续回退链（深度站落到 h1 干净值）。
		if sel == "#top" || sel == "h1#top" {
			if !isHeadingNode(s) {
				continue
			}
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

// isHeadingNode §59.366: selection 的节点是否标题标签（h1/h2/h3）。
func isHeadingNode(s *goquery.Selection) bool {
	if s.Length() == 0 {
		return false
	}
	node := s.Get(0)
	if node.Type != 1 { // element
		return false
	}
	name := strings.ToLower(node.Data)
	return name == "h1" || name == "h2" || name == "h3"
}
