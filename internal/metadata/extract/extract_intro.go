// Package extract 简介分段（§56.10 核心算法）。
//
// 借鉴 PTNexus review_extract.go:250-391：
//   - 首图位置 + quote 前后二分
//   - Statement 识别（官组声明 + 短文本）
//   - ARDTU 工具签名剥离
//   - Body 清理（移除已归入 Statement/ARDTU 的内容）
package extract

import (
	"regexp"
	"sort"
	"strings"
)

// quoteBlockRe 提取 BBCode [quote] 块（含位置信息）。
var quoteBlockRe = regexp.MustCompile(`(?is)\[quote(?:=[^\]]*)?\](.*?)\[/quote\]`)

// quoteBlock BBCode 中的 quote 块（含位置信息）。
type quoteBlock struct {
	Start int    // 在 BBCode 中的起始位置（含 [quote] 标签）
	End   int    // 结束位置（含 [/quote] 标签）
	Full  string // 完整 [quote]...[/quote] 文本
	Inner string // quote 内部文本
}

// splitIntroSections 简介分段主入口。
// 输入：描述容器 HTML（提取图片用） + BBCode（提取 quote 用）
// 输出：IntroData{Statement, Poster, Body, Screenshots, RemovedARDTUDeclarations}
func (p *PublicExtractor) splitIntroSections(descrHTML, descrBBCode string) IntroData {
	intro := IntroData{}

	// 1. 抽取所有图片 URL + 归一化 + 去重 + 黑白名单过滤（§56.12）
	validImages := ExtractImages(descrHTML, descrBBCode)

	// 2. 识别海报（4 层优先级：站点特殊 > URL 关键词 > 首图）
	posterURL, screenshots := ClassifyPosterAndScreenshots(validImages, p.siteCode)
	intro.Poster = posterURL
	intro.SetScreenshotURLs(screenshots)

	// 3. 提取 quote 块
	quotes := extractQuoteBlocks(descrBBCode)

	// 4. 确定拆分点：海报位置 > MediaInfo 位置 > 全部归类
	posterIdx := -1
	if posterURL != "" {
		posterIdx = strings.Index(descrBBCode, posterURL)
	}
	if posterIdx < 0 {
		posterIdx = findMediaInfoPosition(descrBBCode)
	}
	beforePoster, _ := splitQuotesByPosition(quotes, posterIdx)

	// 5. 分类首图前的 quote 块
	statements, ardtuFulls, stmtFulls := classifyBeforePosterQuotes(beforePoster)
	intro.Statement = strings.Join(statements, "\n\n")
	intro.RemovedARDTUDeclarations = ardtuFulls

	// 6. 构建 Body：整个 BBCode 去掉 Statement/ARDTU 对应的 quote 整块
	body := descrBBCode
	for _, full := range ardtuFulls {
		body = strings.ReplaceAll(body, full, "")
	}
	for _, full := range stmtFulls {
		body = strings.ReplaceAll(body, full, "")
	}
	body = compactBlankLines(body)
	intro.Body = strings.TrimSpace(body)

	return intro
}

// extractQuoteBlocks 从 BBCode 中提取所有 [quote] 块（含位置信息）。
// §59.26: 栈式解析器，正确处理嵌套 [quote][quote][/quote][/quote]。
// 返回所有匹配对（含内层），按起始位置排序。外层 block 的 inner 包含内层 block 原样。
func extractQuoteBlocks(bbcode string) []quoteBlock {
	openRe := regexp.MustCompile(`(?i)\[quote(?:=[^\]]*)?\]`)
	closeTagLower := "[/quote]"

	type token struct {
		pos    int // tag 起始位置
		end    int // tag 结束位置（exclusive）
		isOpen bool
	}

	var tokens []token
	for _, m := range openRe.FindAllStringIndex(bbcode, -1) {
		tokens = append(tokens, token{pos: m[0], end: m[1], isOpen: true})
	}
	lower := strings.ToLower(bbcode)
	searchFrom := 0
	for {
		idx := strings.Index(lower[searchFrom:], closeTagLower)
		if idx < 0 {
			break
		}
		absIdx := searchFrom + idx
		closeEnd := absIdx + len(closeTagLower)
		tokens = append(tokens, token{pos: absIdx, end: closeEnd, isOpen: false})
		searchFrom = closeEnd
	}

	sort.Slice(tokens, func(i, j int) bool { return tokens[i].pos < tokens[j].pos })

	var stack []token
	var blocks []quoteBlock
	for _, tok := range tokens {
		if tok.isOpen {
			stack = append(stack, tok)
		} else {
			if len(stack) == 0 {
				continue
			}
			openTag := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			blocks = append(blocks, quoteBlock{
				Start: openTag.pos,
				End:   tok.end,
				Full:  bbcode[openTag.pos:tok.end],
				Inner: bbcode[openTag.end:tok.pos],
			})
		}
	}
	return blocks
}

// findMediaInfoPosition 在 BBCode 中查找 MediaInfo 段的起始位置。
// §59.26: 无海报时用 MediaInfo 位置作声明/正文的拆分点。
var mediaInfoMarkerRe = regexp.MustCompile(`(?im)(?:^|\n)\s*(?:\[url[^\]]*\]\s*)?MediaInfo\s*[:：]|(?:^|\n)\s*\[quote\]\s*General\s*\n`)

func findMediaInfoPosition(bbcode string) int {
	loc := mediaInfoMarkerRe.FindStringIndex(bbcode)
	if loc != nil {
		return loc[0]
	}
	return -1
}

// splitQuotesByPosition 按 posterIdx 二分 quote 块。
// posterIdx < 0（无首图、无 MediaInfo）→ 全部归 before（跑声明分类，不丢内容）。
func splitQuotesByPosition(quotes []quoteBlock, posterIdx int) (before, after []quoteBlock) {
	if posterIdx < 0 {
		return quotes, nil
	}
	for _, q := range quotes {
		if q.End <= posterIdx {
			before = append(before, q)
		} else {
			after = append(after, q)
		}
	}
	return before, after
}

// classifyBeforePosterQuotes 分类首图前的 quote 块。
// 顺序很重要：By ARDTU 前缀优先（能保留正文），其次工具签名/技术参数（剥离），最后官组声明（保留）。
func classifyBeforePosterQuotes(quotes []quoteBlock) (statements, ardtuFulls, stmtFulls []string) {
	for _, q := range quotes {
		text := strings.TrimSpace(q.Inner)
		if text == "" {
			continue
		}
		// 1. By ARDTU@... 前缀 → 剥离前缀归 Statement，整块从 Body 移除
		if IsByARDTUWithGroupInfo(text) {
			stripped := StripByARDTUPrefix(text)
			if stripped != "" {
				statements = append(statements, stripped)
			}
			ardtuFulls = append(ardtuFulls, q.Full)
			continue
		}
		// 2. ARDTU 工具签名 / 技术参数块 → 整块移除
		if IsToolSignatureQuote(text) || IsTechParamsQuote(text) {
			ardtuFulls = append(ardtuFulls, q.Full)
			continue
		}
		// 3. 官组声明 → 归 Statement（完整 BBCode），整块从 Body 移除
		if IsAcknowledgmentQuote(text) {
			statements = append(statements, q.Full)
			stmtFulls = append(stmtFulls, q.Full)
			continue
		}
		// 4. 其他 → 保留在 Body 中（不丢内容）
	}
	return statements, ardtuFulls, stmtFulls
}

// compactBlankLines 压缩 3+ 换行为 2 个。
func compactBlankLines(s string) string {
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	return s
}
