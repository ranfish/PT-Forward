package reseed

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ranfish/pt-forward/internal/util"
)

func NormalizeTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}

	// Step 1: Unicode NFKD normalization equivalent — strip diacritics
	var norm strings.Builder
	for _, r := range title {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		norm.WriteRune(r)
	}
	title = norm.String()

	// Step 2: Lowercase
	title = strings.ToLower(title)

	// Step 3: Remove content within brackets and parentheses
	var clean strings.Builder
	depth := 0
	for _, r := range title {
		if r == '[' || r == '(' || r == '【' || r == '（' {
			depth++
			continue
		}
		if r == ']' || r == ')' || r == '】' || r == '）' {
			if depth > 0 {
				depth--
			}
			continue
		}
		if depth == 0 {
			clean.WriteRune(r)
		}
	}
	title = clean.String()

	// Step 4: Collapse whitespace
	title = strings.Join(strings.Fields(title), " ")

	// Step 5: Trim
	title = strings.TrimSpace(title)

	// Step 6: Truncate at quality keywords
	stopWords := []string{"2160p", "1080p", "720p", "480p", "x264", "x265", "h264", "h265", "hevc", "web-dl", "bluray", "bdrip", "hdrip", "webrip", "remux"}
	lower := title
	for _, w := range stopWords {
		if idx := strings.Index(lower, w); idx > 3 {
			title = strings.TrimSpace(title[:idx])
			break
		}
	}

	if len(title) > 80 {
		title = title[:80]
	}

	return title
}

var resolutionKeywords = []string{"2160p", "1080p", "1080i", "720p", "576p", "576i", "480p", "480i", "1440p", "4320p", "4k"}

var mediaTypeKeywords = []string{
	"Blu-ray", "BluRay", "Bluray", "Blu ray", "Blue-ray",
	"WEB-DL", "WebDL", "WEBRip",
	"HDTV", "HD-DVD", "HDDVD",
	"UHD", "DVDRip",
}

// reChannelWord §59.234 ②: 频道词（制作频道=分发方语义——同 BBC 之于
// BBC 剧）。CCTVx（x=数字或文字，含"电影频道"中文形态）。刘老庄案：
// "[刘老庄八十二壮士].CCTV6.82.Warriors..." 的 CCTV6 站剧名区——被字母
// 数字拆词拆成 CCTV+6 后独立 "6" 污染关键词（AND 必失配）。
var reChannelWord = regexp.MustCompile(`(?i)\bCCTV\s?[0-9]{0,2}\b|\bCCTV\s?(?:电影|电视剧|音乐|戏曲|少儿|新闻|财经|体育|综合|国防|世界地理|纪录|中文国际|英语新闻)频道?\b`)

func ExtractSearchKeyword(title string) string {
	if title == "" {
		return ""
	}
	// §59.184 附四: 剥合集数字序号前缀——剥后即标准 B1 形态（中文前缀+英文主体），
	// 英文链/KeywordHasNoTitle/中文线（leadingCJKSegment 同步剥）三点一线恢复。
	title = stripEpisodeNumberPrefix(title)

	// §59.234 ②: 频道词剥离（CCTV 族——剧名区频道词非片名组成部分，
	// 站方标题搜核心片名即可召回；独立 "6" 尾巴是 AND 污染源）
	title = strings.TrimSpace(reChannelWord.ReplaceAllString(title, " "))

	// §59.217: CJK 后挂括号注记剥离——"雅尼雅典卫城音乐会(25周年纪念版).1993"
	// （雅尼案：注记被当中文前缀整段剥离，keyword 残"25周年纪念版) 1993 720p"
	// 带残括号且丢片名）。仅剥【前邻 CJK + 内容含 CJK】的半角/全角括号组；
	// 方括号标题形态（[杀人回忆].Memories…）与年份括号不受影响。
	title = stripCJKParenNote(title)

	rest := stripChinesePrefix(title)
	if rest == "" {
		// stripChinesePrefix 失败（如 三国.全95集.2010...，分隔符后是中文）
		// 在原始标题上截取年份兜底，产出"剧名 年份"
		rest = truncateToYear(title)
		if rest == "" {
			return ""
		}
	}

	// 剧集检测：stripChinesePrefix 后以 S01/S01E01 等开头 = 中文剧名被跳过了
	// 保留中文剧名用于搜索（NexusPHP 支持中文搜索）
	trimmed := strings.TrimLeft(rest, ". ")
	if seasonPattern.MatchString(trimmed) {
		rest = title // 回退原始标题，保留中文剧名
	}

	// §59.206: 字母+数字粘连拆分（Aliens2→Aliens 2 / MiniBD1080P→MiniBD 1080P）。
	// 双收益：①粘连 token 词级匹配失效（站方词级引擎不吃词内子串——Aliens2
	// 形态实测 0 行 vs Aliens 2 形态 8 行含目标 tid=4343；单字符数字被站方
	// 忽略）②拆出的分辨率词恢复 truncateToResolution 截断点（粘连形态致
	// 截断失明、7 词全量残留撞站方词数上限）。字母段 ≥3 防技术词误拆
	// （x264/H265/AC3/S01E01 等形态免疫）。
	rest = reAlphaDigitGlue.ReplaceAllString(rest, "$1 $2")

	raw := truncateToResolution(rest)
	if raw == "" {
		raw = truncateToYear(rest)
	}
	raw = strings.TrimLeft(raw, ".")
	if raw == "" {
		return ""
	}
	raw = strings.ReplaceAll(raw, ".", " ")
	raw = strings.ReplaceAll(raw, "_", " ") // §59.192 B: 下划线分隔名（BOHEMIAN_RHAPSODY）站方点分隔 AND 失配
	raw = stripBrandColonPrefix(raw)
	// 去掉介质/来源词和地区码
	for _, term := range mediumAndRegionTerms {
		raw = strings.ReplaceAll(raw, term, " ")
	}
	// 去掉中文集数描述（全16集、16集等）
	raw = chineseEpisodeCountRe.ReplaceAllString(raw, " ")
	raw = reYearRangeToken.ReplaceAllString(raw, " ") // §59.204 F2: 年份区间
	// §59.218: CJK 注记噪声词（音轨/碟媒）token 剥离
	raw = stripAnnotNoiseTokens(raw)
	// §59.220: 规格词拼写变体归一——minbd→minibd（收藏转发名少 i 形态，
	// Inception 案：官站 MiniBD1080P 词形 AND 全灭走次优站包子）
	raw = normalizeSpecVariants(raw)
	raw = strings.Join(strings.Fields(raw), " ")
	raw = stripApostrophes(raw)

	// §59.180: 删除 strippedCJK 回退——剥离成功=英文标题可用，不需要"补回"中文。
	// chineseTitleFallback 只在 KeywordHasNoTitle（英文确实无有效标题）时触发。
	// 原 strippedCJK 条件会用含中文前缀的结果覆盖正确的英文搜索词
	// （孤儿恢复三失败实证：不设限通缉Running.On.Empty... → 搜索词被污染）。
	if KeywordHasNoTitle(raw) {
		if fb := chineseTitleFallback(title); fb != "" && fb != raw {
			return limitKeywordTerms(stripVersionTokens(appendMediaAfterResolution(stripApostrophes(fb), title)))
		}
	}
	// §59.188: 终段剥版本词（两个返回点统一）
	return limitKeywordTerms(stripVersionTokens(appendMediaAfterResolution(raw, title)))
}

// reEpisodeNumPrefix 合集序号前缀（可重复，如 "12." / "12.2." / "E09."）。
// §59.200: E+数字形态（电影合集单文件命名——007 合集 E09=金枪人案）：
// 站方独立资源标题无集号词 → AND 0 结果。纯数字（迪士尼收集案）与 E 形态
// 统一剥除；4 位年份开头（"2019."）不受影响（1-3 位限制）。
var reEpisodeNumPrefix = regexp.MustCompile(`^((?:\d{1,3}|(?i:E\d{1,3}))[.])+`)

// stripEpisodeNumberPrefix §59.184 附四: 剥合集数字序号前缀。
// 关键词被序号污染（"12 仙履奇缘 Cinderella…"）会触发 KeywordHasNoTitle
// 误杀（首词数字 → 判"无标题"→ 整个 L2 跳过——迪士尼动画收集 8 孤儿实证）。
// 限 1-3 位数字+点分隔（"2019.片名" 年份开头 4 位形态不受影响）。
func stripEpisodeNumberPrefix(title string) string {
	loc := reEpisodeNumPrefix.FindStringIndex(title)
	if loc == nil {
		return title
	}
	// §59.201: 2-3 位序号后紧跟年份 = 数字本身是片名（65.2023/300.2006——
	// 数字命名电影家族；1 位序号年份形态结局同跳过，不区分）。
	if reYearTokenFollows.MatchString(title[loc[1]:]) {
		return title
	}
	return title[:loc[0]] + title[loc[1]:]
}

// reYearTokenFollows §59.201: 行首年份 token（数字片名判别的配套锚）。
var reYearTokenFollows = regexp.MustCompile(`^(?:19|20)\d{2}[.]`)

// reVersionToken 版本词（与 extractReleaseVersion 同表：PROPER/REPACKn/RERIP/DIRFIX/INTERNAL）。
var reVersionToken = regexp.MustCompile(`(?i)^(?:PROPER|REPACK\d*|RERIP|DIRFIX|INTERNAL)$`)

// stripVersionTokens §59.188: 搜索关键词剥版本词。
// 版本词非内容身份——站方标题带不带不可控（加勒比 tid=105267 案实证：
// 发布名含 PROPER 站方标题无 → NexusPHP AND 0 结果）；版本不对称由验证层
// 规则 B 管辖（§59.76/G2），搜索层剥除纯增益召回（PROPER/非PROPER 版都搜得）。
func stripVersionTokens(keyword string) string {
	words := strings.Fields(keyword)
	out := make([]string, 0, len(words))
	for _, w := range words {
		if reVersionToken.MatchString(w) {
			continue
		}
		// §59.192 A: "and" 连接词——站方标题惯用 "&"（Queen.Rock.Montreal.&.Live.Aid
		// tid=410220 案），词形不对称 AND 必空；剥离后余词仍全含于标题，无损召回
		if strings.EqualFold(w, "and") {
			continue
		}
		out = append(out, w)
	}
	return strings.Join(out, " ")
}

// stripApostrophes 剥离直撇号和弯撇号，防止 NexusPHP SQL LIKE 注入式失败。
func stripApostrophes(s string) string {
	s = strings.ReplaceAll(s, "'", " ")
	s = strings.ReplaceAll(s, "\u2019", " ")
	return strings.Join(strings.Fields(s), " ")
}

// appendMediaAfterResolution 从原始标题中提取紧跟分辨率后的媒体类型（如 Blu-ray、WEB-DL），
// 追加到关键词中提高搜索精度。关键词过宽导致搜索结果太多、正确种子不在第一页。
// "IF 2024 1080p" + "IF.2024.1080p.Blu-ray.REPACK..." → "IF 2024 1080p Blu-ray"
func appendMediaAfterResolution(keyword, originalTitle string) string {
	lowerKW := strings.ToLower(keyword)
	lowerOrig := strings.ToLower(originalTitle)

	for _, res := range resolutionKeywords {
		kwResIdx := strings.Index(lowerKW, res)
		if kwResIdx < 0 {
			continue
		}
		origResIdx := strings.Index(lowerOrig, res)
		if origResIdx < 0 {
			continue
		}
		afterResStart := origResIdx + len(res)
		if afterResStart >= len(originalTitle) {
			break
		}
		afterRes := originalTitle[afterResStart:]
		trimmed := strings.TrimLeft(afterRes, ". _-")
		for _, media := range mediaTypeKeywords {
			if len(trimmed) < len(media) {
				continue
			}
			if !strings.EqualFold(trimmed[:len(media)], media) {
				continue
			}
			afterMediaIdx := len(media)
			if afterMediaIdx < len(trimmed) {
				ch := trimmed[afterMediaIdx]
				if ch != '.' && ch != ' ' && ch != '_' && ch != '-' {
					continue
				}
			}
			if strings.Contains(lowerKW, strings.ToLower(media)) {
				break
			}
			return keyword + " " + media
		}
		break
	}
	return keyword
}

// chineseTitleFallback 在英文关键词提取失败（hasNoTitle=true）时，
// 用原始标题截到分辨率或年份，保留中文标题用于 NexusPHP 中文搜索。
// "乾隆王朝.2002.40集全..." → "乾隆王朝 2002"
// "男人.2022.1080p..." → "男人 2022 1080p"
func chineseTitleFallback(title string) string {
	fb := truncateToResolution(title)
	if fb == "" {
		fb = truncateToYear(title)
	}
	if fb == "" {
		return ""
	}
	fb = strings.ReplaceAll(fb, ".", " ")
	fb = stripBrandColonPrefix(fb)
	for _, term := range mediumAndRegionTerms {
		fb = strings.ReplaceAll(fb, term, " ")
	}
	fb = chineseEpisodeCountRe.ReplaceAllString(fb, " ")
	fb = reYearRangeToken.ReplaceAllString(fb, " ") // §59.204 F2: 年份区间
	fb = strings.Join(strings.Fields(fb), " ")
	return fb
}

// stripBrandColonPrefix 剥离"品牌+中文描述："前缀。
// 当冒号（：或:）前的部分同时含 ASCII 字母和 CJK 字符时（如 "HBO史诗巨著"），
// 判定为频道/品牌描述前缀，返回冒号后的内容。
// 纯 CJK 标题（如 "忍者神龟：变种时代"）不剥离。
func stripBrandColonPrefix(s string) string {
	for _, colon := range []string{"：", ":"} {
		idx := strings.Index(s, colon)
		if idx <= 0 {
			continue
		}
		prefix := s[:idx]
		hasASCII, hasCJK := false, false
		for _, r := range prefix {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				hasASCII = true
			}
			if (r >= 0x4E00 && r <= 0x9FFF) || (r >= 0x3400 && r <= 0x4DBF) {
				hasCJK = true
			}
		}
		if hasASCII && hasCJK {
			return s[idx+len(colon):]
		}
	}
	return s
}

var seasonPattern = regexp.MustCompile(`(?i)^S\d{1,2}(?:E\d{1,3})?`)

// §59.204 F1: 集数标记支持中文数字（全五集/全四十集）——原仅阿拉伯数字，
// BBC王朝案"全五集"残留致站方 AND 0 结果（用户实证 "BBC 王朝 2018 720p" 命中）。
var chineseEpisodeCountRe = regexp.MustCompile(`全?[0-9零一二三四五六七八九十百千]+集全?`)

// reYearRangeToken §59.204 F2: 年份区间 token（合集多部年份跨度 2002-2007）——
// 站方标题/副标题通常不含区间写法，AND 阻断词（色即是空案实证：
// "色即是空 合集"=3 行 vs "色即是空 合集 2002-2007"=0 行）。
var reYearRangeToken = regexp.MustCompile(`\b(?:19|20)\d{2}[-.](?:19|20)\d{2}\b`)

var mediumAndRegionTerms = []string{
	// 介质/来源词（变体问题：Blu-ray/Bluray、Web-DL/WebDL）
	"Blu-ray", "Bluray", "Blu ray", "BluRay", "Blue-ray",
	"Web-DL", "WebDL", "Web DL", "WEBRip",
	"HDTV", "UHD", "HD-DVD", "HDDVD",
	"DVDR", "DVDRip", "Remux", "REMUX",
	// 地区码（§56.34 field #6，仅原盘类，非标题内容）
	"GBR", "USA", "JPN", "HKG", "TWN", "KOR", "EUR",
	"CAN", "AUS", "FRA", "GER", "CZE", "NOR", "ITA",
}

func stripChinesePrefix(title string) string {
	i := 0
	inBracket := false
	for i < len(title) {
		r, size := utf8.DecodeRuneInString(title[i:])
		if r == '[' || r == '【' {
			inBracket = true
			i += size
			continue
		}
		if inBracket && (r == ']' || r == '】') {
			inBracket = false
			i += size
			if i < len(title) {
				r2, _ := utf8.DecodeRuneInString(title[i:])
				if r2 == '.' || r2 == ' ' || r2 == ']' || r2 == '】' {
					i++
				}
			}
			continue
		}
		if inBracket {
			i += size
			continue
		}
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			// ASCII 紧跟 CJK（无分隔符）：可能是中文标题的续集号/编号（II、5、3D、VIII），
			// 也可能是英文标题开头（Back）。≤3 字符或全大写/纯数字 ≤4 字符视为续集号跳过。
			if i > 0 {
				prevRune, _ := utf8.DecodeLastRuneInString(title[:i])
				if (prevRune >= 0x4E00 && prevRune <= 0x9FFF) || (prevRune >= 0x3400 && prevRune <= 0x4DBF) {
					segLen := 0
					allUpperOrDigit := true
					j := i
					for j < len(title) {
						r2, size2 := utf8.DecodeRuneInString(title[j:])
						if (r2 >= 'a' && r2 <= 'z') || (r2 >= 'A' && r2 <= 'Z') || (r2 >= '0' && r2 <= '9') {
							if r2 >= 'a' && r2 <= 'z' {
								allUpperOrDigit = false
							}
							segLen++
							j += size2
						} else {
							break
						}
					}
					if segLen <= 3 || (allUpperOrDigit && segLen == 4) {
						i = j
						continue
					}
				}
			}
			return title[i:]
		}
		if r == '.' || r == ' ' || r == '+' || r == '-' || r == '_' {
			prev := i
			i += size
			found := false
			for i < len(title) {
				r2, size2 := utf8.DecodeRuneInString(title[i:])
				if (r2 >= 'a' && r2 <= 'z') || (r2 >= 'A' && r2 <= 'Z') || (r2 >= '0' && r2 <= '9') {
					found = true
					break
				}
				if r2 == '.' || r2 == ' ' || r2 == '+' || r2 == '-' || r2 == '_' {
					i += size2
					continue
				}
				break
			}
			if !found {
				return ""
			}
			return title[prev:]
		}
		i += size
	}
	return ""
}

func truncateToResolution(s string) string {
	lower := strings.ToLower(s)
	bestIdx := -1
	bestEnd := 0
	for _, kw := range resolutionKeywords {
		searchFrom := 0
		for searchFrom < len(lower) {
			relIdx := strings.Index(lower[searchFrom:], kw)
			if relIdx < 0 {
				break
			}
			idx := searchFrom + relIdx
			endIdx := idx + len(kw)
			// 短关键词（≤2 字符如 "4k"）要求匹配后有词边界（分隔符或 EOF），
			// 防止 "4K修复版" 中的 "4k" 被误认为分辨率标记。
			if len(kw) <= 2 && endIdx < len(lower) {
				next := lower[endIdx]
				if next != '.' && next != ' ' && next != '-' && next != '_' {
					searchFrom = endIdx
					continue
				}
			}
			if bestIdx < 0 || idx < bestIdx {
				bestIdx = idx
				bestEnd = endIdx
			}
			break
		}
	}
	if bestIdx < 0 {
		return ""
	}
	end := bestEnd
	if has3D(lower) {
		afterRes := s[bestEnd:]
		afterLower := lower[bestEnd:]
		for _, spec := range []string{"hsbs", "hou"} {
			idx := strings.Index(afterLower, spec)
			if idx >= 0 {
				pos := idx + len(spec)
				if idx == 0 || afterRes[idx-1] == '.' || afterRes[idx-1] == ' ' || afterRes[idx-1] == '-' {
					end = bestEnd + pos
					break
				}
			}
		}
	}
	return s[:end]
}

var re3D = regexp.MustCompile(`(?i)\b3d\b`)

func has3D(lowerS string) bool {
	return re3D.MatchString(lowerS)
}

var yearTruncateRe = regexp.MustCompile(`(?:19|20)\d{2}(?:-(?:19|20)\d{2})?`)

// truncateToYear 截取到第一个年份（含），用于无分辨率的种子名（如 DVDRip）。
// 跳过位置 0 的年份——标题本身以年份开头（如 "2012世界末日.2009..."）时，
// 位置 0 的年份是片名组成部分，不是发布年份分隔符。
func truncateToYear(s string) string {
	searchStart := 0
	for {
		loc := yearTruncateRe.FindStringIndex(s[searchStart:])
		if loc == nil {
			return ""
		}
		absStart := searchStart + loc[0]
		absEnd := searchStart + loc[1]
		if absStart == 0 {
			searchStart = absEnd
			continue
		}
		return s[:absEnd]
	}
}

func ExtractGroupName(title string) string {
	return util.ExtractGroupName(title)
}
