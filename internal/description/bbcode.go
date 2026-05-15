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
	Bold:          regexp.MustCompile(`(?i)\[b\](.*?)\[/b\]`),
	Italic:        regexp.MustCompile(`(?i)\[i\](.*?)\[/i\]`),
	Underline:     regexp.MustCompile(`(?i)\[u\](.*?)\[/u\]`),
	Strike:        regexp.MustCompile(`(?i)\[s\](.*?)\[/s\]`),
	Code:          regexp.MustCompile(`(?i)\[code\](.*?)\[/code\]`),
	Color:         regexp.MustCompile(`(?i)\[color=[^\]]*\](.*?)\[/color\]`),
	Size:          regexp.MustCompile(`(?i)\[size=[^\]]*\](.*?)\[/size\]`),
	Font:          regexp.MustCompile(`(?i)\[font=[^\]]*\](.*?)\[/font\]`),
	Align:         regexp.MustCompile(`(?i)\[align=[^\]]*\](.*?)\[/align\]`),
	Center:        regexp.MustCompile(`(?i)\[center\](.*?)\[/center\]`),
	Left:          regexp.MustCompile(`(?i)\[left\](.*?)\[/left\]`),
	Right:         regexp.MustCompile(`(?i)\[right\](.*?)\[/right\]`),
	HR:            regexp.MustCompile(`(?i)\[hr\]`),
	Table:         regexp.MustCompile(`(?i)\[table\](.*?)\[/table\]`),
	TR:            regexp.MustCompile(`(?i)\[tr\](.*?)\[/tr\]`),
	TD:            regexp.MustCompile(`(?i)\[td\](.*?)\[/td\]`),
	TH:            regexp.MustCompile(`(?i)\[th\](.*?)\[/th\]`),
	Email:         regexp.MustCompile(`(?i)\[email\]([^\[]*?)\[/email\]`),
	AsteriskClose: regexp.MustCompile(`(?i)\[\*/?\]`),
	StyleClose:    regexp.MustCompile(`(?i)\[/(?:color|size|font|align|center|left|right)\]`),
	Spoiler:       regexp.MustCompile(`(?i)\[spoiler(?:=[^\]]*)?\](.*?)\[/spoiler\]`),
	Quote:         regexp.MustCompile(`(?i)\[quote\](.*?)\[/quote\]`),
	QuoteInner:    regexp.MustCompile(`(?is)\[quote\](.*?)\[/quote\]`),
	Newline:       regexp.MustCompile(`\n`),
	List:          regexp.MustCompile(`(?i)\[list\](.*?)\[/list\]`),
	ListInner:     regexp.MustCompile(`(?is)\[list\](.*?)\[/list\]`),
	ListItem:      regexp.MustCompile(`\[\*\]([^\[]*)`),
}

func init() {
	for level := 1; level <= 6; level++ {
		tag := fmt.Sprintf("h%d", level)
		bbcodePatterns.Heading[level-1] = regexp.MustCompile(`(?i)\[` + tag + `\](.*?)\[/` + tag + `\]`)
	}
}

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
	s = p.Quote.ReplaceAllStringFunc(s, func(m string) string {
		inner := p.QuoteInner.FindStringSubmatch(m)
		if len(inner) < 2 {
			return m
		}
		lines := p.Newline.Split(inner[1], -1)
		result := ""
		for _, line := range lines {
			result += "> " + line + "\n"
		}
		return result
	})
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
