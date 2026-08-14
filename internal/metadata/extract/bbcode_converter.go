// Package extract BBCode 转换器（§56.9）。
//
// 双栈 accumulator + 递归 goquery 遍历，30 个标签，性能 < 50ms/100KB HTML。
package extract

import (
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

const (
	maxNestingDepth = 50
	maxHTMLBytes    = 1 << 20 // 1MB
)

// bbcodeAccumulator 双栈标签累加器（§56.9 决策 1）。
// open 推入 openingTags 尾部，close 推入 closingTags 头部。
// wrap 时自动正确嵌套（openingTags 正序 + content + closingTags 已逆序）。
type bbcodeAccumulator struct {
	openingTags []string
	closingTags []string
}

func (a *bbcodeAccumulator) add(open, close string) {
	if open != "" {
		a.openingTags = append(a.openingTags, open)
	}
	if close != "" {
		a.closingTags = append([]string{close}, a.closingTags...)
	}
}

func (a *bbcodeAccumulator) wrap(inner string) string {
	if len(a.openingTags) == 0 && len(a.closingTags) == 0 {
		return inner
	}
	var b strings.Builder
	b.Grow(len(a.openingTags)*8 + len(inner) + len(a.closingTags)*8)
	for _, t := range a.openingTags {
		b.WriteString(t)
	}
	b.WriteString(inner)
	for _, t := range a.closingTags {
		b.WriteString(t)
	}
	return b.String()
}

// HTMLToBBCodeConverter 带 siteCode 上下文的转换器（2b.7 用于站点特殊处理）。
type HTMLToBBCodeConverter struct {
	siteCode     string
	siteNickname string
}

// NewHTMLToBBCodeConverter 创建转换器。
func NewHTMLToBBCodeConverter(siteCode, siteNickname string) *HTMLToBBCodeConverter {
	return &HTMLToBBCodeConverter{siteCode: siteCode, siteNickname: siteNickname}
}

// HTMLToBBCode 将 HTML 片段转换为 BBCode（无站点特殊处理）。
func HTMLToBBCode(htmlStr string) (string, error) {
	return NewHTMLToBBCodeConverter("", "").Convert(htmlStr)
}

// HTMLToBBCodeMust 测试用，失败 panic。
func HTMLToBBCodeMust(htmlStr string) string {
	out, err := HTMLToBBCode(htmlStr)
	if err != nil {
		panic(err)
	}
	return out
}

// Convert 执行 HTML → BBCode 转换。
func (c *HTMLToBBCodeConverter) Convert(htmlStr string) (string, error) {
	htmlStr = strings.TrimSpace(htmlStr)
	if htmlStr == "" {
		return "", nil
	}
	if len(htmlStr) > maxHTMLBytes {
		return "", fmt.Errorf("extract: html too large (%d bytes)", len(htmlStr))
	}
	doc, err := ParseHTML(htmlStr)
	if err != nil {
		return "", fmt.Errorf("parse html: %w", err)
	}

	// 优先取 body 内容（避免 html/head），找不到就直接用根
	body := doc.Find("body")
	var root *goquery.Selection
	if body.Length() > 0 {
		root = body
	} else {
		root = doc.Selection
	}

	var b strings.Builder
	root.Contents().Each(func(_ int, s *goquery.Selection) {
		b.WriteString(c.convertNode(s, 0))
	})
	return strings.TrimSpace(c.cleanup(b.String())), nil
}

// convertNode 递归转换单个节点为 BBCode。
func (c *HTMLToBBCodeConverter) convertNode(s *goquery.Selection, depth int) string {
	if depth > maxNestingDepth || len(s.Nodes) == 0 {
		return ""
	}
	node := s.Nodes[0]

	// 文本节点直接返回数据（剥离前导 \r\n，避免与 <br/> 产生的 \n 叠加成空行）
	if node.Type == html.TextNode {
		return strings.TrimLeft(node.Data, "\r\n")
	}
	if node.Type != html.ElementNode {
		return ""
	}

	tagName := strings.ToLower(node.Data)

	// <script>/<style> 丢弃整节点（含子节点）
	if tagName == "script" || tagName == "style" {
		return ""
	}

	acc := &bbcodeAccumulator{}
	handler := lookupTagHandler(tagName)
	suffix := handler(acc, s, c)

	// 递归处理子节点
	var inner strings.Builder
	s.Contents().Each(func(_ int, child *goquery.Selection) {
		inner.WriteString(c.convertNode(child, depth+1))
	})

	return acc.wrap(inner.String()) + suffix
}

// cleanup 压缩多余空行（3+ 换行 → 2 个）。
func (c *HTMLToBBCodeConverter) cleanup(s string) string {
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	return s
}
