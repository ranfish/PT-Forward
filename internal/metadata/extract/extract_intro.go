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

	// 4. 用首图位置二分 quote 块
	posterIdx := -1
	if posterURL != "" {
		posterIdx = strings.Index(descrBBCode, posterURL)
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
func extractQuoteBlocks(bbcode string) []quoteBlock {
	indices := quoteBlockRe.FindAllStringSubmatchIndex(bbcode, -1)
	if len(indices) == 0 {
		return nil
	}
	blocks := make([]quoteBlock, 0, len(indices))
	for _, idx := range indices {
		fullStart, fullEnd := idx[0], idx[1]
		innerStart, innerEnd := idx[2], idx[3]
		blocks = append(blocks, quoteBlock{
			Start: fullStart,
			End:   fullEnd,
			Full:  bbcode[fullStart:fullEnd],
			Inner: bbcode[innerStart:innerEnd],
		})
	}
	return blocks
}

// splitQuotesByPosition 按 posterIdx 二分 quote 块。
// posterIdx < 0（无首图）→ 全部归 after。
func splitQuotesByPosition(quotes []quoteBlock, posterIdx int) (before, after []quoteBlock) {
	if posterIdx < 0 {
		return nil, quotes
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
		// 3. 官组声明 → 归 Statement，整块从 Body 移除
		if IsAcknowledgmentQuote(text) {
			statements = append(statements, text)
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
