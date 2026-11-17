package description

import (
	"fmt"
	"regexp"
	"strings"
)

var bbcodePatterns = struct {
	URLWithText   *regexp.Regexp
	URLPlain      *regexp.Regexp
	Img           *regexp.Regexp
	Bold          *regexp.Regexp
	Italic        *regexp.Regexp
	Underline     *regexp.Regexp
	Strike        *regexp.Regexp
	Code          *regexp.Regexp
	Color         *regexp.Regexp
	Size          *regexp.Regexp
	Font          *regexp.Regexp
	Align         *regexp.Regexp
	Center        *regexp.Regexp
	Left          *regexp.Regexp
	Right         *regexp.Regexp
	HR            *regexp.Regexp
	Table         *regexp.Regexp
	TR            *regexp.Regexp
	TD            *regexp.Regexp
	TH            *regexp.Regexp
	Email         *regexp.Regexp
	AsteriskClose *regexp.Regexp
	StyleClose    *regexp.Regexp
	Spoiler       *regexp.Regexp
	Quote         *regexp.Regexp
	QuoteInner    *regexp.Regexp
	Newline       *regexp.Regexp
	List          *regexp.Regexp
	ListInner     *regexp.Regexp
	ListItem      *regexp.Regexp
	Heading       [6]*regexp.Regexp
}{
	URLWithText:   regexp.MustCompile(`(?i)\[url=([^\]]+)\]([^\[]*?)\[/url\]`),
	URLPlain:      regexp.MustCompile(`(?i)\[url\]([^\[]*?)\[/url\]`),
	Img:           regexp.MustCompile(`(?i)\[img\]([^\[]*?)\[/img\]`),
	Bold:          regexp.MustCompile(`(?is)\[b\](.*?)\[/b\]`),
	Italic:        regexp.MustCompile(`(?is)\[i\](.*?)\[/i\]`),
	Underline:     regexp.MustCompile(`(?is)\[u\](.*?)\[/u\]`),
	Strike:        regexp.MustCompile(`(?is)\[s\](.*?)\[/s\]`),
	Code:          regexp.MustCompile(`(?is)\[code\](.*?)\[/code\]`),
	Color:         regexp.MustCompile(`(?is)\[color=[^\]]*\](.*?)\[/color\]`),
	Size:          regexp.MustCompile(`(?is)\[size=[^\]]*\](.*?)\[/size\]`),
	Font:          regexp.MustCompile(`(?is)\[font=[^\]]*\](.*?)\[/font\]`),
	Align:         regexp.MustCompile(`(?is)\[align=[^\]]*\](.*?)\[/align\]`),
	Center:        regexp.MustCompile(`(?is)\[center\](.*?)\[/center\]`),
	Left:          regexp.MustCompile(`(?is)\[left\](.*?)\[/left\]`),
	Right:         regexp.MustCompile(`(?is)\[right\](.*?)\[/right\]`),
	HR:            regexp.MustCompile(`(?i)\[hr\]`),
	Table:         regexp.MustCompile(`(?is)\[table\](.*?)\[/table\]`),
	TR:            regexp.MustCompile(`(?is)\[tr\](.*?)\[/tr\]`),
	TD:            regexp.MustCompile(`(?is)\[td\](.*?)\[/td\]`),
	TH:            regexp.MustCompile(`(?is)\[th\](.*?)\[/th\]`),
	Email:         regexp.MustCompile(`(?i)\[email\]([^\[]*?)\[/email\]`),
	AsteriskClose: regexp.MustCompile(`(?i)\[\*/?\]`),
	StyleClose:    regexp.MustCompile(`(?i)\[/(?:color|size|font|align|center|left|right)\]`),
	Spoiler:       regexp.MustCompile(`(?is)\[spoiler(?:=[^\]]*)?\](.*?)\[/spoiler\]`),
	Quote:         regexp.MustCompile(`(?i)\[quote\](.*?)\[/quote\]`),
	QuoteInner:    regexp.MustCompile(`(?is)\[quote\](.*?)\[/quote\]`),
	Newline:       regexp.MustCompile(`\n`),
	List:          regexp.MustCompile(`(?is)\[list\](.*?)\[/list\]`),
	ListInner:     regexp.MustCompile(`(?is)\[list\](.*?)\[/list\]`),
	ListItem:      regexp.MustCompile(`\[\*\]([^\[]*)`),
}

func init() {
	for level := 1; level <= 6; level++ {
		tag := fmt.Sprintf("h%d", level)
		bbcodePatterns.Heading[level-1] = regexp.MustCompile(`(?i)\[` + tag + `\](.*?)\[/` + tag + `\]`)
	}
}

// quoteInnermostRe §59.541 附三: 最内层 quote 匹配（RE2 无前瞻——内容形态
// =不含 '[' 后随 q/Q，即不含嵌套 [quote] 开标签）；配合外层循环实现由内
// 向外逐层转换（statement 嵌套引用实战形态）。
var quoteInnermostRe = regexp.MustCompile(`(?is)\[quote\]((?:[^[]|\[[^qQ])*)\[/quote\]`)

// residualQuoteTokenRe §59.541 附三: 落单/畸形 quote token 清扫兜底——
// MD 站任何残留 BBCode=422（Obsession 首发实战）。
var residualQuoteTokenRe = regexp.MustCompile(`(?i)\[/?quote(?:=[^\]]*)?\]`)

func BBCodeToMarkdown(input string) string {
	if input == "" {
		return ""
	}

	s := input
	p := &bbcodePatterns

	s = p.URLWithText.ReplaceAllString(s, "[$2]($1)")
	s = p.URLPlain.ReplaceAllString(s, "<$1>")
	s = p.Img.ReplaceAllString(s, "![]($1)")
	s = p.Bold.ReplaceAllString(s, "**$1**")
	s = p.Italic.ReplaceAllString(s, "*$1*")
	s = p.Underline.ReplaceAllString(s, "<u>$1</u>")
	s = p.Strike.ReplaceAllString(s, "~~$1~~")
	s = p.Code.ReplaceAllString(s, "```\n$1\n```")
	// §59.541 附三: 嵌套 quote 内层优先循环——单遍非贪婪在嵌套形态下必残留
	// 内层 [quote]+落单 [/quote]（Obsession 首发实战 422 实证）；且原 Quote
	// 模式无 (?s)，statement 多行引用块整个不转换是第二漏点。由内向外逐层
	// 转换，无进展即停（16 层上限防御）。
	for pass := 0; pass < 16; pass++ {
		next := quoteInnermostRe.ReplaceAllStringFunc(s, func(m string) string {
			g := quoteInnermostRe.FindStringSubmatch(m)
			if len(g) < 2 {
				return m
			}
			lines := p.Newline.Split(g[1], -1)
			result := ""
			for _, line := range lines {
				result += "> " + line + "\n"
			}
			return result
		})
		if next == s {
			break
		}
		s = next
	}
	// 残骸清扫: 落单/畸形 quote token 兜底剥除。
	s = residualQuoteTokenRe.ReplaceAllString(s, "")
	s = p.Color.ReplaceAllString(s, "$1")
	s = p.Size.ReplaceAllString(s, "$1")
	s = p.Font.ReplaceAllString(s, "$1")
	s = p.Align.ReplaceAllString(s, "$1")
	s = p.Center.ReplaceAllString(s, "$1")
	s = p.Left.ReplaceAllString(s, "$1")
	s = p.Right.ReplaceAllString(s, "$1")
	s = p.HR.ReplaceAllString(s, "---")
	for level := 1; level <= 6; level++ {
		prefix := strings.Repeat("#", level) + " "
		s = p.Heading[level-1].ReplaceAllString(s, prefix+"$1")
	}
	s = p.List.ReplaceAllStringFunc(s, func(m string) string {
		inner := p.ListInner.FindStringSubmatch(m)
		if len(inner) < 2 {
			return m
		}
		items := p.ListItem.FindAllStringSubmatch(inner[1], -1)
		result := ""
		for _, item := range items {
			result += "- " + item[1] + "\n"
		}
		return result
	})
	s = p.Table.ReplaceAllString(s, "$1")
	s = p.TR.ReplaceAllString(s, "$1|")
	s = p.TD.ReplaceAllString(s, "$1|")
	s = p.TH.ReplaceAllString(s, "$1|")
	s = p.Email.ReplaceAllString(s, "[$1](mailto:$1)")
	s = p.AsteriskClose.ReplaceAllString(s, "")
	s = p.StyleClose.ReplaceAllString(s, "")
	s = p.Spoiler.ReplaceAllString(s, "<details>$1</details>")

	return s
}
