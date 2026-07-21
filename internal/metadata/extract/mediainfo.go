// Package extract MediaInfo/BDInfo 提取（§56.11）。
//
// 4 层候选源（<pre>/[quote]/<div class="codemain">/<fieldset>）+ 合法性检测 +
// 前缀去污染 + 空行压缩。借鉴 PTNexus review_extract.go:1740-1942。
package extract

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// 合法性检测关键字（§56.11 直接移植 PTNexus）。
var (
	mediaInfoStrongKeywords = []string{
		"GENERAL", "VIDEO", "AUDIO", "COMPLETE NAME", "DURATION", "FILE SIZE",
	}
	mediaInfoWeakKeywords = []string{
		"MEDIAINFO", "FORMAT", "BIT RATE", "LANGUAGE", "UNIQUE ID", "OVERALL BIT RATE",
	}
	bdInfoKeywords = []string{
		"DISC INFO", "PLAYLIST REPORT", "QUICK SUMMARY", "FILES:", "CHAPTERS:", "DISC SIZE",
	}

	// bdInfoSubtitlesCutRe BDInfo SUBTITLES → FILES 段截断（v0.0.254 从 audiences 提取器通用化）。
	// Audiences 等站的 BDInfo 中 SUBTITLES 到 FILES 之间是字幕文件列表（非 BDInfo 正文），需移除。
	// BDInfo 标准格式：SUBTITLES: 段后跟 FILES: 段，中间字幕列表对技术信息无意义。
	bdInfoSubtitlesCutRe = regexp.MustCompile(`(?is)(SUBTITLES:.*?)(FILES:)`)
)

// 前缀污染锚点正则。
var (
	mediaInfoGeneralAnchor    = regexp.MustCompile("(?is)\\bGeneral\\s*(?:\\r?\\n|\\s{2,})\\s*Unique ID\\b")
	mediaInfoSectionStartLine = regexp.MustCompile(`(?im)^\s*(General|Video|Audio|Text|Menu|Chapters)\s*$`)
	bdInfoDiscAnchor           = regexp.MustCompile("(?is)\\bDISC INFO\\b")
	bdInfoSectionStartLine    = regexp.MustCompile(`(?im)^\s*(DISC INFO|PLAYLIST REPORT|QUICK SUMMARY|FILES:|CHAPTERS:|DISC SIZE)\s*$`)
	mediaInfoSectionHeader    = regexp.MustCompile(`(?im)^\s*(General|Video|Audio|Text|Menu|Chapters)(\s*#\d+)?\s*$`)
	quoteContentRe            = regexp.MustCompile(`(?is)\[quote(?:=[^\]]*)?\](.*?)\[/quote\]`)
)

// ExtractMediaInfo 从详情页提取 MediaInfo / BDInfo 文本。
// descrHTML: 详情页描述容器的 HTML
// descrBBCode: 同一段描述的 BBCode
// siteCode: 站点代码（2b.7 用于路由站点特殊提取）
// 返回：mediainfo, bdinfo（空字符串表示未提取到）
func ExtractMediaInfo(descrHTML, descrBBCode, siteCode string) (mediainfo, bdinfo string) {
	candidates := extractMediaInfoCandidates(descrHTML, descrBBCode)

	var validMI, validBD []string
	for _, c := range candidates {
		text := strings.TrimSpace(c)
		if text == "" {
			continue
		}
		if isLikelyMediaInfoText(text) {
			validMI = append(validMI, text)
		}
		if isLikelyBDInfoText(text) {
			validBD = append(validBD, text)
		}
	}

	if len(validMI) > 0 {
		mediainfo = pickLongest(validMI)
		mediainfo = trimMediaInfoLeadingNoise(mediainfo)
		mediainfo = compactBlankLinesForMediaReports(mediainfo)
	}
	if len(validBD) > 0 {
		bdinfo = pickLongest(validBD)
		bdinfo = trimBDInfoLeadingNoise(bdinfo)
		bdinfo = compactBlankLinesForMediaReports(bdinfo)
		// v0.0.254: 通用 BDInfo SUBTITLES → FILES 段截断（从 audiences 提取器迁移）
		// 移除 SUBTITLES 到 FILES 之间的字幕文件列表（非 BDInfo 技术正文）
		bdinfo = bdInfoSubtitlesCutRe.ReplaceAllString(bdinfo, "$2")
	}
	return mediainfo, bdinfo
}

// extractMediaInfo 委托给独立函数 ExtractMediaInfo（PublicExtractor 内部封装）。
func (p *PublicExtractor) extractMediaInfo(descrHTML, descrBBCode string) (mediainfo, bdinfo string) {
	return ExtractMediaInfo(descrHTML, descrBBCode, p.siteCode)
}

// extractMediaInfoCandidates 从 4 层候选源提取文本块。
// 优先级：<pre> > [quote] > <div class="codemain"> > <fieldset>。
func extractMediaInfoCandidates(descrHTML, descrBBCode string) []string {
	var candidates []string

	// 1. <pre> 块（goquery 主路径）
	if doc, err := ParseHTML(descrHTML); err == nil {
		doc.Find("pre").Each(func(_ int, s *goquery.Selection) {
			if text := strings.TrimSpace(s.Text()); text != "" {
				candidates = append(candidates, text)
			}
		})
		// 3. <div class="codemain">（NexusPHP 标准容器）
		doc.Find("div.codemain").Each(func(_ int, s *goquery.Selection) {
			if text := strings.TrimSpace(s.Text()); text != "" {
				candidates = append(candidates, text)
			}
		})
		// 4. <fieldset>
		doc.Find("fieldset").Each(func(_ int, s *goquery.Selection) {
			if text := strings.TrimSpace(s.Text()); text != "" {
				candidates = append(candidates, text)
			}
		})
	}

	// 2. [quote] 块（BBCode 文本扫描）
	matches := quoteContentRe.FindAllStringSubmatch(descrBBCode, -1)
	for _, m := range matches {
		if len(m) > 1 {
			candidates = append(candidates, m[1])
		}
	}

	return candidates
}

// isLikelyMediaInfoText 判断文本是否为合法 MediaInfo。
// 规则：强关键字命中 ≥ 2 / 弱关键字命中 ≥ 2 且含 VIDEO 或 AUDIO / 特殊组合。
func isLikelyMediaInfoText(text string) bool {
	upper := strings.ToUpper(text)

	strongCount := 0
	for _, kw := range mediaInfoStrongKeywords {
		if strings.Contains(upper, kw) {
			strongCount++
		}
	}
	if strongCount >= 2 {
		return true
	}

	weakCount := 0
	for _, kw := range mediaInfoWeakKeywords {
		if strings.Contains(upper, kw) {
			weakCount++
		}
	}
	if weakCount >= 2 && (strings.Contains(upper, "VIDEO") || strings.Contains(upper, "AUDIO")) {
		return true
	}

	// 特殊组合
	if strings.Contains(upper, "COMPLETE NAME") && strings.Contains(upper, "FORMAT") {
		return true
	}
	if strings.Contains(upper, ".RELEASE.INFO") && strings.Contains(upper, "ENCODER") {
		return true
	}

	return false
}

// isLikelyBDInfoText 判断文本是否为合法 BDInfo（单关键字命中即可）。
func isLikelyBDInfoText(text string) bool {
	upper := strings.ToUpper(text)
	for _, kw := range bdInfoKeywords {
		if strings.Contains(upper, kw) {
			return true
		}
	}
	return false
}

// trimMediaInfoLeadingNoise 从 MediaInfo 文本中去除前缀污染。
// 尝试从 General anchor 或分节标题开始截取。
func trimMediaInfoLeadingNoise(text string) string {
	if loc := mediaInfoGeneralAnchor.FindStringIndex(text); loc != nil {
		return text[loc[0]:]
	}
	if loc := mediaInfoSectionStartLine.FindStringIndex(text); loc != nil {
		return text[loc[0]:]
	}
	return text
}

// trimBDInfoLeadingNoise 从 BDInfo 文本中去除前缀污染。
func trimBDInfoLeadingNoise(text string) string {
	if loc := bdInfoDiscAnchor.FindStringIndex(text); loc != nil {
		return text[loc[0]:]
	}
	if loc := bdInfoSectionStartLine.FindStringIndex(text); loc != nil {
		return text[loc[0]:]
	}
	return text
}

// compactBlankLinesForMediaReports 压缩 MediaInfo/BDInfo 文本中的多余空行。
// 规则：先压缩连续空行为 1 个，再移除"前后都不是分节标题"的空行。
func compactBlankLinesForMediaReports(text string) string {
	// 1. 压缩连续空行为 1 个
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}
	// 2. 移除非标题间的空行
	lines := strings.Split(text, "\n")
	result := make([]string, 0, len(lines))
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			prevIsHeader := len(result) > 0 && mediaInfoSectionHeader.MatchString(strings.TrimSpace(result[len(result)-1]))
			nextIsHeader := i+1 < len(lines) && mediaInfoSectionHeader.MatchString(strings.TrimSpace(lines[i+1]))
			if !prevIsHeader && !nextIsHeader {
				continue
			}
		}
		result = append(result, line)
	}
	return strings.Join(result, "\n")
}

// pickLongest 从候选中选最长的文本（MediaInfo/BDInfo 通常越长越完整）。
func pickLongest(candidates []string) string {
	longest := ""
	for _, c := range candidates {
		if len(c) > len(longest) {
			longest = c
		}
	}
	return longest
}
