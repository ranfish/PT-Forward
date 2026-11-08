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

	// titleTokenSplitRe §59.487: 裸标题标签与 <title> 清理值的词交集切分
	// （CJK 逐字+ASCII 词——交集判定用）。
	titleTokenSplitRe = regexp.MustCompile(`[^\p{L}\p{N}]+`)

	// titleQuotedContentRe §59.487: <title> 引号内真标题提取（NP 形态
	// "站名 :: 种子详情 \"真标题\""——后缀清理只剥尾剩站名·引号优先）。
	titleQuotedContentRe = regexp.MustCompile(`"([^"]{4,200})"` )
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
	// §59.487: <title> 标签清理值预提取——裸 h1/h2/h3 命中后的横幅防御
	// 校验基准。修道院案：全站固定横幅 h1（"渡劫，炼炁，修真而登仙"=站点
	// slogan·未登录也在）——裸 h1 回退命中横幅·真标题在 <title>
	// （"站名 :: 种子详情 \"A Penny...\"" 清理后）。
	titleTagValue := cleanTitleTagText(doc)

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
		// §59.487: 引号内优先——NP 形态 "站名 :: 种子详情 \"真标题\""
		// 后缀清理只剥尾段剩站名（修道院案 got=站名）；引号内容才是
		// 真标题·优先提取·无引号形态维持后缀清理。
		if sel == "title" {
			if q := titleQuotedContentRe.FindStringSubmatch(title); len(q) > 1 {
				title = q[1]
			} else {
				title = titlePageSuffixRe.ReplaceAllString(title, "")
			}
		}
		title = strings.TrimSpace(title)
		if title == "" {
			continue
		}
		// §59.487: 裸 h1/h2/h3 横幅防御——与 <title> 清理值零公共词的裸
		// 标题标签判定为横幅/装饰元素（站点 slogan·页头横幅），跳过继续
		// 回退（终达 <title> 特异提取=正确值）。特异选择器（#top/
		// #torrent-name/h1#top/h1.title）语义明确不校验。防误伤：正常站
		// h1=种子名与 <title> 高度重叠 → 校验过；h1 中文/<title> 英文的
		// 站 → 跳 h1 落 <title>（英文官方标题·仍是该种正确标识）。
		if sel == "h1" || sel == "h2" || sel == "h3" {
			if titleTagValue != "" && !titleSharesToken(title, titleTagValue) {
				continue
			}
		}
		return title
	}
	return fallbackTitle
}

// cleanTitleTagText §59.487: <title> 标签清理值（引号内容优先·后缀剥离）。
func cleanTitleTagText(doc *goquery.Document) string {
	s := doc.Find("title").First()
	if s.Length() == 0 {
		return ""
	}
	t := whitespaceCollapse.ReplaceAllString(strings.TrimSpace(s.Text()), " ")
	if q := titleQuotedContentRe.FindStringSubmatch(t); len(q) > 1 {
		return strings.TrimSpace(q[1])
	}
	t = titlePageSuffixRe.ReplaceAllString(t, "")
	return strings.TrimSpace(t)
}

// titleSharesToken §59.487: 两标题是否共享至少一个词（词级交集）。
// token 化：非字母数字切分+小写；CJK 段以连续汉字串为一个 token。
func titleSharesToken(a, b string) bool {
	if a == "" || b == "" {
		return true // 空值不构成判别依据——保持原值（不跳过）
	}
	tb := titleTokenSet(b)
	for _, tok := range titleTokens(a) {
		if tb[tok] {
			return true
		}
	}
	return false
}

func titleTokens(s string) []string {
	parts := titleTokenSplitRe.Split(strings.ToLower(s), -1)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func titleTokenSet(s string) map[string]bool {
	set := make(map[string]bool)
	for _, tok := range titleTokens(s) {
		set[tok] = true
	}
	return set
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
