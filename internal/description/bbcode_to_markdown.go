package description

import (
	"regexp"
	"strings"
)

// 私有增强正则（bbcodePatterns 公共集缺多行 s 标志与 img 带参形态——
// 现有 BBCodeToMarkdown 同受此限但 renderer 场景未暴露；此处不改公共集）。
var (
	mdNoImgImageArg = regexp.MustCompile(`(?is)\[img[^\]]*\]\s*.*?\[/img\]`)
	mdNoImgCode     = regexp.MustCompile(`(?is)\[code\]\s*(.*?)\s*\[/code\]`)
)

// §59.458-⑤: BBCode→Markdown 禁图变体（肉丝 PeerGo 架构专用）。
//
// 与 BBCodeToMarkdown（renderer 消费——img 转 ![]() 图形态）语义相反：
// 站方 description 禁图（markdownImagePattern 命中 ![](/<img 即 4001
// 拒收）——本变体 [img] 整段剥除（截图走 images base64 字段）。
// 复用 bbcodePatterns 正则集（单源），转换序列按肉丝契约定制：
//   [img]            → 整段剥除（最先——防 URL 残留重组）
//   [quote]          → > 行前缀（同款逻辑）
//   [code]           → ``` 围栏（同款）
//   [b]/[i]          → **/*（同款）
//   [url=U]L         → [L](U)（同款）
//   [url]U           → [U](U)（差异：不用 <$1> 尖括号形态——统一链接形态）
//   [u]/[color]/[size]/[font]/[align] 族 → 剥壳保内容（差异：不产 HTML 标签）
//   [spoiler]        → 剥壳保内容（差异：不产 <details> HTML）
//   [list]/[*]       → - 列表（同款）
func BBCodeToMarkdownNoImage(input string) string {
	if input == "" {
		return ""
	}
	p := &bbcodePatterns

	// ① 图片剥除（最先——站方 4001 硬校验防御；无参+带参双形态）
	s := p.Img.ReplaceAllString(input, "")
	s = mdNoImgImageArg.ReplaceAllString(s, "")

	// ② 引用块 → > 行前缀（复用现有同构逻辑）
	s = p.QuoteInner.ReplaceAllStringFunc(s, func(m string) string {
		inner := p.QuoteInner.FindStringSubmatch(m)
		if len(inner) < 2 {
			return m
		}
		lines := p.Newline.Split(inner[1], -1)
		var b strings.Builder
		for _, line := range lines {
			b.WriteString("> ")
			b.WriteString(strings.TrimSpace(line))
			b.WriteString("\n")
		}
		return b.String()
	})

	// ③ 代码围栏（私有多行版——公共 Code 无 s 标志多行不命中）
	s = mdNoImgCode.ReplaceAllString(s, "```\n$1\n```")

	// ④ 链接（带参同款；裸链统一 []() 形态）
	s = p.URLWithText.ReplaceAllString(s, "[$2]($1)")
	s = p.URLPlain.ReplaceAllString(s, "[$1]($1)")

	// ⑤ 行内样式（同款）
	s = p.Bold.ReplaceAllString(s, "**$1**")
	s = p.Italic.ReplaceAllString(s, "*$1*")
	s = p.Strike.ReplaceAllString(s, "~~$1~~")

	// ⑥ 剥壳族（不产 HTML 标签——差异点）
	s = p.Underline.ReplaceAllString(s, "$1")
	s = p.Color.ReplaceAllString(s, "$1")
	s = p.Size.ReplaceAllString(s, "$1")
	s = p.Font.ReplaceAllString(s, "$1")
	s = p.Align.ReplaceAllString(s, "$1")
	s = p.Center.ReplaceAllString(s, "$1")
	s = p.Left.ReplaceAllString(s, "$1")
	s = p.Right.ReplaceAllString(s, "$1")
	s = p.Spoiler.ReplaceAllString(s, "$1")

	// ⑦ 列表（同款）
	s = p.List.ReplaceAllStringFunc(s, func(m string) string {
		inner := p.ListInner.FindStringSubmatch(m)
		if len(inner) < 2 {
			return m
		}
		items := p.ListItem.FindAllStringSubmatch(inner[1], -1)
		var b strings.Builder
		for _, item := range items {
			b.WriteString("- ")
			b.WriteString(item[1])
			b.WriteString("\n")
		}
		return b.String()
	})
	s = p.AsteriskClose.ReplaceAllString(s, "")
	s = p.StyleClose.ReplaceAllString(s, "")

	// 压缩三连空行
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(s)
}
