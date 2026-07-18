package extract

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

// tagHandlerFunc 处理单个标签，往 acc 添加 open/close 标签，返回 suffix。
type tagHandlerFunc func(acc *bbcodeAccumulator, s *goquery.Selection, c *HTMLToBBCodeConverter) (suffix string)

// tagHandlers 30 个标签 dispatch（§56.9 决策 2）。
var tagHandlers = map[string]tagHandlerFunc{
	"br":      handleBR,
	"p":       handleBlock,
	"div":     handleBlock,
	"section": handleBlock,
	"article": handleBlock,
	"fieldset": handleQuote,
	"blockquote": handleQuote,
	"pre":      handleQuote,
	"code":     handleQuote,
	"legend":   handleDefault, // UI 标签丢弃（仅渲染子）
	"li":       handleLI,
	"ul":       handleUL,
	"ol":       handleOL,
	"img":      handleImg,
	"a":        handleAnchor,
	"strong":   handleBold,
	"b":        handleBold,
	"em":       handleItalic,
	"i":        handleItalic,
	"u":        handleUnderline,
	"s":        handleStrike,
	"del":      handleStrike,
	"strike":   handleStrike,
	"span":     handleStyled,
	"font":     handleFont,
	"center":   handleCenter,
	"h1":       makeHeadingHandler("7"),
	"h2":       makeHeadingHandler("6"),
	"h3":       makeHeadingHandler("5"),
	"h4":       makeHeadingHandler("4"),
	"h5":       makeHeadingHandler("3"),
	"h6":       makeHeadingHandler("3"),
	"hr":       handleHR,
	"table":    handleTable,
	"th":       handleTableCell,
	"td":       handleTableCell,
}

// lookupTagHandler 查找标签 handler，未命中返回 handleDefault（渲染子节点，不丢内容）。
func lookupTagHandler(tagName string) tagHandlerFunc {
	if h, ok := tagHandlers[tagName]; ok {
		return h
	}
	return handleDefault
}

// handleDefault 默认 handler：渲染子节点，不包裹标签。
func handleDefault(acc *bbcodeAccumulator, _ *goquery.Selection, _ *HTMLToBBCodeConverter) string {
	return ""
}

// handleBR `<br>` → "\n"。
func handleBR(_ *bbcodeAccumulator, _ *goquery.Selection, _ *HTMLToBBCodeConverter) string {
	return "\n"
}

// handleBlock `<p>/<div>/<section>/<article>` → 内容 + "\n\n"。
// 同时处理 style 属性（text-align 等）。
func handleBlock(acc *bbcodeAccumulator, s *goquery.Selection, _ *HTMLToBBCodeConverter) string {
	applyStylesFromAttr(acc, s)
	return "\n\n"
}

// handleQuote `<fieldset>/<blockquote>/<pre>/<code>` → [quote]内容[/quote]\n\n。
func handleQuote(acc *bbcodeAccumulator, _ *goquery.Selection, _ *HTMLToBBCodeConverter) string {
	acc.add("[quote]", "[/quote]")
	return "\n\n"
}

// handleLI `<li>` 根据父节点（ul/ol）选 prefix。
func handleLI(acc *bbcodeAccumulator, s *goquery.Selection, _ *HTMLToBBCodeConverter) string {
	parentName := parentTagName(s)
	if parentName == "ol" {
		acc.add("[*]", "")
	} else {
		acc.add("- ", "")
	}
	return "\n"
}

// handleUL `<ul>` 内容不包裹。
func handleUL(_ *bbcodeAccumulator, _ *goquery.Selection, _ *HTMLToBBCodeConverter) string {
	return "\n"
}

// handleOL `<ol>` → [list=1]内容[/list]。
func handleOL(acc *bbcodeAccumulator, _ *goquery.Selection, _ *HTMLToBBCodeConverter) string {
	acc.add("[list=1]", "[/list]")
	return "\n"
}

// handleImg `<img>` → [img]src[/img]（src 为空则不输出）。
func handleImg(acc *bbcodeAccumulator, s *goquery.Selection, _ *HTMLToBBCodeConverter) string {
	src, _ := s.Attr("src")
	if src == "" {
		src, _ = s.Attr("data-src")
	}
	if src == "" {
		return ""
	}
	acc.add("[img]"+src+"[/img]", "")
	return ""
}

// handleAnchor `<a href>` → [url=href]text[/url]（href 为空仅输出 text）。
func handleAnchor(acc *bbcodeAccumulator, s *goquery.Selection, _ *HTMLToBBCodeConverter) string {
	href, _ := s.Attr("href")
	if href == "" {
		return ""
	}
	acc.add("[url="+href+"]", "[/url]")
	return ""
}

func handleBold(acc *bbcodeAccumulator, _ *goquery.Selection, _ *HTMLToBBCodeConverter) string {
	acc.add("[b]", "[/b]")
	return ""
}

func handleItalic(acc *bbcodeAccumulator, _ *goquery.Selection, _ *HTMLToBBCodeConverter) string {
	acc.add("[i]", "[/i]")
	return ""
}

func handleUnderline(acc *bbcodeAccumulator, _ *goquery.Selection, _ *HTMLToBBCodeConverter) string {
	acc.add("[u]", "[/u]")
	return ""
}

func handleStrike(acc *bbcodeAccumulator, _ *goquery.Selection, _ *HTMLToBBCodeConverter) string {
	acc.add("[s]", "[/s]")
	return ""
}

// handleStyled `<span>` 解析 style 属性。
func handleStyled(acc *bbcodeAccumulator, s *goquery.Selection, _ *HTMLToBBCodeConverter) string {
	applyStylesFromAttr(acc, s)
	return ""
}

// handleFont `<font>` 解析 color/size/face 属性（face 不映射）。
func handleFont(acc *bbcodeAccumulator, s *goquery.Selection, _ *HTMLToBBCodeConverter) string {
	if color, ok := s.Attr("color"); ok && color != "" {
		hex := normalizeColorToHex(color)
		acc.add("[color="+hex+"]", "[/color]")
	}
	if size, ok := s.Attr("size"); ok && size != "" {
		acc.add("[size="+size+"]", "[/size]")
	}
	return ""
}

func handleCenter(acc *bbcodeAccumulator, _ *goquery.Selection, _ *HTMLToBBCodeConverter) string {
	acc.add("[center]", "[/center]")
	return ""
}

// makeHeadingHandler 创建 h1~h6 handler（size 分别 7/6/5/4/3/3）。
func makeHeadingHandler(size string) tagHandlerFunc {
	return func(acc *bbcodeAccumulator, _ *goquery.Selection, _ *HTMLToBBCodeConverter) string {
		acc.add("[b][size=\""+size+"\"]", "[/size][/b]")
		return "\n"
	}
}

func handleHR(acc *bbcodeAccumulator, _ *goquery.Selection, _ *HTMLToBBCodeConverter) string {
	acc.add("[hr]", "")
	return "\n"
}

// handleTable `<table>` 2b.7 站点特殊（默认仅输出内容 + 换行）。
func handleTable(_ *bbcodeAccumulator, _ *goquery.Selection, _ *HTMLToBBCodeConverter) string {
	return "\n"
}

// handleTableCell `<th>/<td>` 2b.7 站点特殊（默认 quote 包裹）。
func handleTableCell(acc *bbcodeAccumulator, _ *goquery.Selection, _ *HTMLToBBCodeConverter) string {
	acc.add("[quote]", "[/quote]")
	return " "
}

// parentTagName 返回父节点的 tag 名（小写），无父节点或非元素返回 ""。
func parentTagName(s *goquery.Selection) string {
	parent := s.Parent()
	if len(parent.Nodes) == 0 {
		return ""
	}
	if parent.Nodes[0].Type != html.ElementNode {
		return ""
	}
	return strings.ToLower(parent.Nodes[0].Data)
}
