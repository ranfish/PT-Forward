// Package description BBCode → HTML 转换（§56.20 决策 5）。
//
// 基于现有 BBCodeToMarkdown 模式（复用 bbcodePatterns 正则集），替换为 HTML 标签。
// 用途：描述预览 UI 显示（Gazelle/Unit3D 等支持 HTML 的站点）。
package description

import (
	"fmt"
	"strings"
)

// BBCodeToHTML 将 BBCode 转换为 HTML。
// 复用 bbcodePatterns 正则集（与 BBCodeToMarkdown 一致），替换为对应 HTML 标签。
func BBCodeToHTML(input string) string {
	if input == "" {
		return ""
	}

	s := input
	p := &bbcodePatterns

	// 链接和图片
	s = p.URLWithText.ReplaceAllString(s, `<a href="$1">$2</a>`)
	s = p.URLPlain.ReplaceAllString(s, `<a href="$1">$1</a>`)
	s = p.Img.ReplaceAllString(s, `<img src="$1"/>`)

	// 文本样式
	s = p.Bold.ReplaceAllString(s, "<strong>$1</strong>")
	s = p.Italic.ReplaceAllString(s, "<em>$1</em>")
	s = p.Underline.ReplaceAllString(s, "<u>$1</u>")
	s = p.Strike.ReplaceAllString(s, "<del>$1</del>")

	// 代码块
	s = p.Code.ReplaceAllString(s, "<pre><code>$1</code></pre>")

	// 引用
	s = p.Quote.ReplaceAllStringFunc(s, func(m string) string {
		inner := p.QuoteInner.FindStringSubmatch(m)
		if len(inner) < 2 {
			return m
		}
		return "<blockquote>" + inner[1] + "</blockquote>"
	})

	// 样式标签（剥离 color/size/font，保留内容）
	s = p.Color.ReplaceAllString(s, "$1")
	s = p.Size.ReplaceAllString(s, "$1")
	s = p.Font.ReplaceAllString(s, "$1")
	s = p.Align.ReplaceAllString(s, "$1")

	// 对齐
	s = p.Center.ReplaceAllString(s, `<div style="text-align:center">$1</div>`)
	s = p.Left.ReplaceAllString(s, `$1`)
	s = p.Right.ReplaceAllString(s, `$1`)

	// 水平线
	s = p.HR.ReplaceAllString(s, "<hr/>")

	// 标题
	for level := 1; level <= 6; level++ {
		tag := fmt.Sprintf("h%d", level)
		s = p.Heading[level-1].ReplaceAllString(s, "<"+tag+">$1</"+tag+">")
	}

	// 列表
	s = p.List.ReplaceAllStringFunc(s, func(m string) string {
		inner := p.ListInner.FindStringSubmatch(m)
		if len(inner) < 2 {
			return m
		}
		items := p.ListItem.FindAllStringSubmatch(inner[1], -1)
		result := "<ul>"
		for _, item := range items {
			result += "<li>" + item[1] + "</li>"
		}
		return result + "</ul>"
	})

	// 表格
	s = p.Table.ReplaceAllString(s, "<table>$1</table>")
	s = p.TR.ReplaceAllString(s, "<tr>$1</tr>")
	s = p.TD.ReplaceAllString(s, "<td>$1</td>")
	s = p.TH.ReplaceAllString(s, "<th>$1</th>")

	// 邮箱
	s = p.Email.ReplaceAllString(s, `<a href="mailto:$1">$1</a>`)

	// 闭合标签清理
	s = p.AsteriskClose.ReplaceAllString(s, "")
	s = p.StyleClose.ReplaceAllString(s, "")

	// 折叠
	s = p.Spoiler.ReplaceAllString(s, "<details>$1</details>")

	// 换行 → <br>
	s = strings.ReplaceAll(s, "\n", "<br/>\n")

	return s
}
