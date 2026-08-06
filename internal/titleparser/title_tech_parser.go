package titleparser

import (
	"regexp"
	"strconv"
	"strings"
)

// ParseTitleTech 从标题字符串解析出完整的 TechProfile（标题源，§56.34 步骤 2）。
//
// 在 ParseTitle（TitleComponents）基础上增强提取：
//   - EditionInfo（v1.05 其他信息 9 类优先级）
//   - SourceType + Specification（从 Medium 拆分）
//   - AudioChannels / AudioTechnology / AudioTracks（标题 fallback，MediaInfo 为准时被覆盖）
//
// ParseTitle 保持不变（兼容现有使用者）。本函数为 TechProfile 体系的增强解析器。
func ParseTitleTech(title string) TechProfile {
	tc := ParseTitle(title)
	p := TechProfileFromTitle(tc)

	p.EditionInfo = extractEditionInfo(title)
	p.SourceType, p.Specification = splitMedium(tc.Medium)
	p.AudioChannels = extractAudioChannelsFromTitle(title)
	p.AudioTechnology = extractAudioTechnologyFromTitle(title)
	p.AudioTracks = extractAudioTracksFromTitle(title)

	return p
}

// editionPattern 版本信息匹配规则。
type editionPattern struct {
	re   *regexp.Regexp
	name string
}

// editionPatterns v1.05 其他信息 9 类优先级（长 pattern 在前，避免短 pattern 子串重复匹配）。
// 分隔符用 [-_.\s]* 支持 PT 标题常见的点分隔/空格分隔/连字符。
var editionPatterns = []editionPattern{
	// 剪辑版本（优先级 1）
	{regexp.MustCompile(`(?i)\bDirector'?s?[-_.\s]*Cut\b`), "Director's Cut"},
	{regexp.MustCompile(`(?i)\bUltimate[-_.\s]*(?:Cut|Edition)\b`), "Ultimate Cut"},
	{regexp.MustCompile(`(?i)\bExtended[-_.\s]*Cut\b`), "Extended Cut"},
	{regexp.MustCompile(`(?i)\bTheatrical[-_.\s]*Cut\b`), "Theatrical Cut"},
	{regexp.MustCompile(`(?i)\bUncensored\b`), "Uncensored"},
	{regexp.MustCompile(`(?i)\bUnrated\b`), "Unrated"},
	{regexp.MustCompile(`(?i)\bUncut\b`), "Uncut"},
	{regexp.MustCompile(`(?i)\bExtended\b`), "Extended"},
	{regexp.MustCompile(`(?i)\bTheatrical\b`), "Theatrical"},
	// 2in1（优先级 2）
	{regexp.MustCompile(`(?i)\b[23][-_\s]?in[-_\s]?1\b`), "2in1"},
	// 版本（优先级 3）
	{regexp.MustCompile(`(?i)\b\d+(?:st|nd|rd|th)?[-_.\s]*Anniversary(?:[-_.\s]*Edition)?\b`), "Anniversary Edition"},
	{regexp.MustCompile(`(?i)\b4K[-_.\s]*Remaster(?:ed)?\b`), "4K Remaster"},
	{regexp.MustCompile(`(?i)\bRemastered\b`), "Remastered"},
	{regexp.MustCompile(`(?i)\bRemaster\b`), "Remaster"},
	{regexp.MustCompile(`(?i)\bLimited[-_.\s]*Edition\b`), "Limited Edition"},
	{regexp.MustCompile(`(?i)\bSpecial[-_.\s]*Edition\b`), "Special Edition"},
	{regexp.MustCompile(`(?i)\bCollector'?s?[-_.\s]*Edition\b`), "Collector's Edition"},
	{regexp.MustCompile(`(?i)\bCriterion[-_.\s]*Collection\b`), "Criterion Collection"},
	{regexp.MustCompile(`(?i)\bCriterion\b`), "Criterion"},
	// 特殊比例（优先级 4）
	{regexp.MustCompile(`(?i)\bIMAX[-_.\s]*Enhanced\b`), "IMAX Enhanced"},
	{regexp.MustCompile(`(?i)\bIMAX\b`), "IMAX"},
	{regexp.MustCompile(`(?i)\bOpen[-_.\s]*Matte\b`), "Open Matte"},
	// Hybrid（优先级 5）
	{regexp.MustCompile(`(?i)\bHybrid\b`), "Hybrid"},
	// MiniBD（优先级 9）
	{regexp.MustCompile(`(?i)\bMiniBD\b`), "MiniBD"},
	// CC = Criterion Collection 缩写，PT 标题中出现在年份后片源前（如 1972.CC.BluRay）
	{regexp.MustCompile(`(?i)(?:^|[.\s\-_])CC(?:[.\s\-_]|$)`), "Criterion Collection"},
}

// extractEditionInfo 从标题提取版本信息。
//
// 采用"消费式匹配"：长 pattern 先匹配并消费区域（替换为空格），
// 防止短 pattern 对同一文本的子串重复匹配（如 "Extended Cut" 被 "Extended" 重复提取）。
func extractEditionInfo(title string) string {
	var found []string
	seen := map[string]bool{}
	work := title
	for _, p := range editionPatterns {
		matches := p.re.FindAllStringIndex(work, -1)
		if len(matches) == 0 {
			continue
		}
		if !seen[p.name] {
			found = append(found, p.name)
			seen[p.name] = true
		}
		for _, loc := range matches {
			for i := loc[0]; i < loc[1]; i++ {
				work = work[:i] + " " + work[i+1:]
			}
		}
	}
	return strings.Join(found, " ")
}

// splitMedium 从 TitleComponents.Medium 组合值拆分片源类型 + 规格。
//
// v1.05 区分：
//   - 片源类型（source_type）：原盘带连字符（Blu-ray）vs 压制无连字符（BluRay）
//   - 规格（specification）：Remux/WEB-DL/WEBRip/HDTV/UHDTV
//
// 原盘类不填规格。WEB/HDTV 类不填片源类型（由 source_platform 标识来源）。
func splitMedium(medium string) (sourceType, specification string) {
	if medium == "" {
		return "", ""
	}
	upper := strings.ToUpper(medium)

	switch {
	case strings.Contains(upper, "WEB-DL") || strings.Contains(upper, "WEBDL"):
		specification = "WEB-DL"
	case strings.Contains(upper, "WEBRIP"):
		specification = "WEBRip"
	case strings.Contains(upper, "REMUX"):
		specification = "Remux"
	case strings.Contains(upper, "UHDTV"):
		specification = "UHDTV"
	case strings.Contains(upper, "HDTV"):
		specification = "HDTV"
	case strings.Contains(upper, "BDRIP"):
		specification = "BDRip"
	case strings.Contains(upper, "DVDRIP"):
		specification = "DVDRip"
	case strings.Contains(upper, "TVRIP"):
		specification = "TVRip"
	}

	switch {
	case strings.Contains(upper, "3D BLU"):
		sourceType = "3D Blu-ray"
	case strings.Contains(upper, "UHD BLU"):
		sourceType = "UHD Blu-ray"
	case strings.Contains(upper, "BLU-RAY") || strings.Contains(upper, "BLURAY") || strings.Contains(upper, "BLU DIY"):
		sourceType = "Blu-ray"
	case strings.Contains(upper, "DVD"):
		sourceType = "DVD"
	}

	return sourceType, specification
}

// reAudioChannelsTitle 匹配声道数，后向边界用 [-_.\s]|$ 避免 "2015.1080p" 中 "5.1" 误匹配。
// 不用前向 \b 因为 "DDP5.1" 中 P 和 5 之间不是 \b。
var reAudioChannelsTitle = regexp.MustCompile(`(?i)(1\.0|2\.0|5\.1|7\.1)(?:[-_.\s]|$)`)

// extractAudioChannelsFromTitle 从标题提取声道数（如 "5.1"/"7.1"）。
//
// 值域约束：仅匹配 PT 圈常见的 1.0/2.0/5.1/7.1，避免误匹配版本号等。
func extractAudioChannelsFromTitle(title string) string {
	m := reAudioChannelsTitle.FindStringSubmatch(title)
	if m != nil {
		return m[1]
	}
	return ""
}

// extractAudioTechnologyFromTitle 从标题提取音频技术（Atmos）。
func extractAudioTechnologyFromTitle(title string) string {
	if regexp.MustCompile(`(?i)\bAtmos\b`).MatchString(title) {
		return "Atmos"
	}
	return ""
}

var reAudioTracksTitle = regexp.MustCompile(`(?i)\b(\d+)\s*Audios?\b`)

// extractAudioTracksFromTitle 从标题提取音轨数（如 "2Audios"/"3Audios"）。
func extractAudioTracksFromTitle(title string) int {
	m := reAudioTracksTitle.FindStringSubmatch(title)
	if m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil {
			return n
		}
	}
	return 0
}
