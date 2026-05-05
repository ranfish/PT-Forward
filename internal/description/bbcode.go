package description

import (
	"fmt"
	"regexp"
	"strings"
)

func BBCodeToMarkdown(input string) string {
	if input == "" {
		return ""
	}

	s := input

	s = regexp.MustCompile(`(?i)\[url=([^\]]+)\]([^\[]*?)\[/url\]`).ReplaceAllString(s, "[$2]($1)")
	s = regexp.MustCompile(`(?i)\[url\]([^\[]*?)\[/url\]`).ReplaceAllString(s, "<$1>")
	s = regexp.MustCompile(`(?i)\[img\]([^\[]*?)\[/img\]`).ReplaceAllString(s, "![]($1)")
	s = regexp.MustCompile(`(?i)\[b\](.*?)\[/b\]`).ReplaceAllString(s, "**$1**")
	s = regexp.MustCompile(`(?i)\[i\](.*?)\[/i\]`).ReplaceAllString(s, "*$1*")
	s = regexp.MustCompile(`(?i)\[u\](.*?)\[/u\]`).ReplaceAllString(s, "<u>$1</u>")
	s = regexp.MustCompile(`(?i)\[s\](.*?)\[/s\]`).ReplaceAllString(s, "~~$1~~")
	s = regexp.MustCompile(`(?i)\[code\](.*?)\[/code\]`).ReplaceAllString(s, "```\n$1\n```")
	s = regexp.MustCompile(`(?i)\[quote\](.*?)\[/quote\]`).ReplaceAllStringFunc(s, func(m string) string {
		inner := regexp.MustCompile(`(?is)\[quote\](.*?)\[/quote\]`).FindStringSubmatch(m)
		if len(inner) < 2 {
			return m
		}
		lines := regexp.MustCompile(`\n`).Split(inner[1], -1)
		result := ""
		for _, line := range lines {
			result += "> " + line + "\n"
		}
		return result
	})
	s = regexp.MustCompile(`(?i)\[color=[^\]]*\](.*?)\[/color\]`).ReplaceAllString(s, "$1")
	s = regexp.MustCompile(`(?i)\[size=[^\]]*\](.*?)\[/size\]`).ReplaceAllString(s, "$1")
	s = regexp.MustCompile(`(?i)\[font=[^\]]*\](.*?)\[/font\]`).ReplaceAllString(s, "$1")
	s = regexp.MustCompile(`(?i)\[align=[^\]]*\](.*?)\[/align\]`).ReplaceAllString(s, "$1")
	s = regexp.MustCompile(`(?i)\[center\](.*?)\[/center\]`).ReplaceAllString(s, "$1")
	s = regexp.MustCompile(`(?i)\[left\](.*?)\[/left\]`).ReplaceAllString(s, "$1")
	s = regexp.MustCompile(`(?i)\[right\](.*?)\[/right\]`).ReplaceAllString(s, "$1")
	s = regexp.MustCompile(`(?i)\[hr\]`).ReplaceAllString(s, "---")
	for level := 1; level <= 6; level++ {
		tag := fmt.Sprintf("h%d", level)
		re := regexp.MustCompile(`(?i)\[` + tag + `\](.*?)\[/` + tag + `\]`)
		prefix := strings.Repeat("#", level) + " "
		s = re.ReplaceAllString(s, prefix+"$1")
	}
	s = regexp.MustCompile(`(?i)\[list\](.*?)\[/list\]`).ReplaceAllStringFunc(s, func(m string) string {
		inner := regexp.MustCompile(`(?is)\[list\](.*?)\[/list\]`).FindStringSubmatch(m)
		if len(inner) < 2 {
			return m
		}
		items := regexp.MustCompile(`\[\*\]([^\[]*)`).FindAllStringSubmatch(inner[1], -1)
		result := ""
		for _, item := range items {
			result += "- " + item[1] + "\n"
		}
		return result
	})
	s = regexp.MustCompile(`(?i)\[table\](.*?)\[/table\]`).ReplaceAllString(s, "$1")
	s = regexp.MustCompile(`(?i)\[tr\](.*?)\[/tr\]`).ReplaceAllString(s, "$1|")
	s = regexp.MustCompile(`(?i)\[td\](.*?)\[/td\]`).ReplaceAllString(s, "$1|")
	s = regexp.MustCompile(`(?i)\[th\](.*?)\[/th\]`).ReplaceAllString(s, "$1|")
	s = regexp.MustCompile(`(?i)\[email\]([^\[]*?)\[/email\]`).ReplaceAllString(s, "[$1](mailto:$1)")
	s = regexp.MustCompile(`(?i)\[\*/?\]`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`(?i)\[/(?:color|size|font|align|center|left|right)\]`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`(?i)\[spoiler(?:=[^\]]*)?\](.*?)\[/spoiler\]`).ReplaceAllString(s, "<details>$1</details>")

	return s
}
