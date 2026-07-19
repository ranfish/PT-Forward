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

	"github.com/PuerkitoBio/goquery"
)

// quoteBlockRe 提取 BBCode [quote] 块（含位置信息）。
var quoteBlockRe = regexp.MustCompile(`(?is)\[quote(?:=[^\]]*)?\](.*?)\[/quote\]`)

// posterURLKeywords 海报 URL 关键词（命中则优先当海报，而非首图）。
var posterURLKeywords = []string{"doubanio", "tmdb", "poster", "pavicon"}

// introUnwantedImagePics 简化版图片黑名单（TODO 2b.6 升级到 embed JSON + DB）。
var introUnwantedImagePics = []string{"ico", "logo", "banner", "icon", "emoji", "smiley", "rank_", "badge"}

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

	// 1. 抽取所有图片 URL + 黑名单过滤
	allImages := extractImageURLsFromHTML(descrHTML)
	validImages := filterUnwantedImages(allImages)

	// 2. 识别海报（URL 关键词优先 > 首图）
	posterURL, screenshots := splitPosterAndScreenshots(validImages)
	intro.Poster = posterURL
	intro.SetScreenshotURLs(screenshots)

	// 3. 提取 quote 块
	quotes := extractQuoteBlocks(descrBBCode)

	// 4. 用首图位置二分 quote 块（afterPoster 不单独使用，Body 从整个 descrBBCode 构建）
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
		// idx: [fullStart, fullEnd, innerStart, innerEnd]
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
// 返回：
//   - statements: 候选 Statement 文本（已剥离 ARDTU 前缀）
//   - ardtuFulls: ARDTU 工具签名 quote 的完整文本（用于记录 + 从 Body 移除）
//   - stmtFulls: Statement quote 的完整文本（用于从 Body 移除）
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

// extractImageURLsFromHTML 从 HTML 中提取所有 <img src> 和 data-src（去重）。
func extractImageURLsFromHTML(htmlStr string) []string {
	doc, err := ParseHTML(htmlStr)
	if err != nil {
		return nil
	}
	var urls []string
	seen := make(map[string]struct{})
	doc.Find("img").Each(func(_ int, s *goquery.Selection) {
		src, ok := s.Attr("src")
		if !ok || src == "" {
			src, ok = s.Attr("data-src")
			if !ok || src == "" {
				return
			}
		}
		if _, exists := seen[src]; exists {
			return
		}
		seen[src] = struct{}{}
		urls = append(urls, src)
	})
	return urls
}

// filterUnwantedImages 过滤黑名单图片（icon/logo/banner 等）。
func filterUnwantedImages(urls []string) []string {
	if len(urls) == 0 {
		return nil
	}
	result := make([]string, 0, len(urls))
	for _, u := range urls {
		if isUnwantedImageURL(u) {
			continue
		}
		result = append(result, u)
	}
	return result
}

// isUnwantedImageURL 判断 URL 是否命中图片黑名单。
func isUnwantedImageURL(url string) bool {
	lower := strings.ToLower(url)
	for _, pattern := range introUnwantedImagePics {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

// splitPosterAndScreenshots 海报识别：URL 关键词优先 > 首图。
func splitPosterAndScreenshots(urls []string) (poster string, screenshots []string) {
	if len(urls) == 0 {
		return "", nil
	}
	// URL 关键词优先
	for i, u := range urls {
		lower := strings.ToLower(u)
		for _, kw := range posterURLKeywords {
			if strings.Contains(lower, kw) {
				poster = u
				screenshots = append(screenshots, urls[:i]...)
				screenshots = append(screenshots, urls[i+1:]...)
				return poster, screenshots
			}
		}
	}
	// 首图当海报
	return urls[0], urls[1:]
}

// compactBlankLines 压缩 3+ 换行为 2 个。
func compactBlankLines(s string) string {
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	return s
}
