package reseed

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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

// stripCJKParenNote §59.217: CJK 后挂括号注记剥离（半角/全角括号，
// 前邻 CJK 且内容含 CJK 才剥）。"雅尼雅典卫城音乐会(25周年纪念版)" →
// "雅尼雅典卫城音乐会"；"[杀人回忆].Memories"（方括号）/"Movie.(2019)"
// （内容无 CJK）不受影响。
func stripCJKParenNote(s string) string {
	runes := []rune(s)
	var b strings.Builder
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r == '(' || r == '（' {
			closer := ')'
			if r == '（' {
				closer = '）'
			}
			// 前邻 CJK 才考虑剥离
			if b.Len() > 0 {
				prev := []rune(b.String())
				p := prev[len(prev)-1]
				if p >= 0x4E00 && p <= 0x9FFF {
					// 找闭括号，内容含 CJK 则整组剥除
					for j := i + 1; j < len(runes); j++ {
						if runes[j] == closer {
							content := string(runes[i+1 : j])
							for _, c := range content {
								if c >= 0x4E00 && c <= 0x9FFF {
									i = j // 跳过整组
									goto next
								}
							}
							break
						}
					}
				}
			}
		}
		b.WriteRune(r)
	next:
	}
	return b.String()
}

// ColonSubSegment §59.215: 全角冒号副题段提取——CJK 标题「主题：副题」
// 形态的副题段关键词（柯南剧场版16案：站方副标题只含副题"第11位前锋"，
// 主关键词整段 AND 全灭）。返回空=无冒号或副题段为空。
func ColonSubSegment(kw string) string {
	at := strings.Index(kw, "：")
	if at < 0 || at == len(kw)-len("：") {
		return ""
	}
	return strings.TrimSpace(kw[at+len("："):])
}

// keywordHasTitleAnchor §59.196: 关键词是否含标题锚（CJK 词或 ≥4 字母的
// token）。年份/分辨率 token 的字母数 <4 不构成锚；媒介词已在短词剥离链剥除。
func keywordHasTitleAnchor(kw string) bool {
	if hasCJKWord(kw) {
		return true
	}
	for _, tok := range strings.Fields(strings.ToLower(kw)) {
		letters := 0
		for _, r := range tok {
			if unicode.IsLetter(r) {
				letters++
			}
		}
		if letters >= 4 {
			return true
		}
	}
	return false
}

// StripYearToken 剥离关键词中的年份 token（19xx/20xx 四位词）。
// §59.198 导出：恢复链 Phase-1 年份剥离降级轮复用（站方年份笔误家族）。
func StripYearToken(kw string) string {
	out := kw
	for _, f := range strings.Fields(kw) {
		if len(f) == 4 && (strings.HasPrefix(f, "19") || strings.HasPrefix(f, "20")) {
			out = strings.ReplaceAll(out, " "+f, "")
			out = strings.ReplaceAll(out, f+" ", "")
		}
	}
	return strings.TrimSpace(strings.Join(strings.Fields(out), " "))
}

// stripResolutionTokens 剥离关键词中的分辨率词。
func stripResolutionTokens(kw string) string {
	out := kw
	for _, res := range []string{"2160p", "1080p", "1080i", "720p", "480p", "4320p"} {
		out = strings.ReplaceAll(out, " "+res, "")
		out = strings.ReplaceAll(out, res+" ", "")
	}
	return strings.TrimSpace(strings.Join(strings.Fields(out), " "))
}

// stripMediumTokensLoose 剥离关键词中的媒介词。
func stripMediumTokensLoose(kw string) string {
	out := kw
	for _, m := range []string{"Blu-ray", "BluRay", "WEB-DL", "WEBRip", "UHD", "HDTV"} {
		out = strings.ReplaceAll(out, " "+m, "")
		out = strings.ReplaceAll(out, m+" ", "")
	}
	return strings.TrimSpace(strings.Join(strings.Fields(out), " "))
}

// leadingCJKSegment §59.184 B 补线：提取标题的前导 CJK 段（B1 中文前缀）。
// 跳过 [【】] 括号组；允许段内分隔符（点/空格/·/冒号）延续；遇 ASCII 即止；
// 尾部分隔符剥净。"冰冻星球.BBC.Frozen..." → "冰冻星球"；"忍者神龟：变种时代.BluRay..." → "忍者神龟：变种时代"。
func leadingCJKSegment(title string) string {
	title = stripEpisodeNumberPrefix(title) // §59.184 附四: 序号前缀不遮蔽 B1 判定
	var b []rune
	inBracket := false
	started := false
	stop := func() string {
		return strings.TrimRight(strings.TrimSpace(string(b)), ". ·：:，!")
	}
	for _, r := range title {
		switch {
		case r == '[' || r == '【':
			if started {
				return stop()
			}
			inBracket = true
		case inBracket && (r == ']' || r == '】'):
			inBracket = false
		case inBracket:
			// 跳过括号内容
		case (r >= 0x4E00 && r <= 0x9FFF) || (r >= 0x3400 && r <= 0x4DBF):
			b = append(b, r)
			started = true
		case started && (r == '.' || r == ' ' || r == '·' || r == '：' || r == ':' || r == '，' || r == '!'):
			b = append(b, r)
		default:
			return stop()
		}
	}
	return stop()
}

// NormalizeMixedKeyword §59.187 ③: 混合词归一化——词内 CJK|ASCII 边界拆分，
// 剔除 ≤2 字符 ASCII 词（噪音源：IF 在英文标题 ubiquitous，用户实测多页噪音）。
// "神秘友友IF 2024 1080P" → "神秘友友 2024 1080P"（用户实测一页命中的精准词）。
// 粘连词在站方同粘连形态时本就命中（回到黑暗Back 案），故作降级轮变体而非替换。
// 无混合词或归一化后无变化返回 ""（调用方跳过该轮）。
func NormalizeMixedKeyword(keyword string) string {
	words := strings.Fields(keyword)
	changed := false
	var out []string
	hasCJKAnchor := false
	for _, w := range words {
		hasCJK, hasASCII := false, false
		for _, r := range w {
			if (r >= 0x4E00 && r <= 0x9FFF) || (r >= 0x3400 && r <= 0x4DBF) {
				hasCJK = true
			} else if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
				hasASCII = true
			}
		}
		switch {
		case hasCJK && hasASCII:
			// 词内边界拆分：CJK 段与 ASCII 段
			var cjkPart, asciiPart []rune
			for _, r := range w {
				if (r >= 0x4E00 && r <= 0x9FFF) || (r >= 0x3400 && r <= 0x4DBF) {
					cjkPart = append(cjkPart, r)
				} else {
					asciiPart = append(asciiPart, r)
				}
			}
			if len(cjkPart) > 0 {
				out = append(out, string(cjkPart))
				hasCJKAnchor = true
			}
			if len(asciiPart) > 2 { // ≤2 字符 ASCII 剔除（IF/3D 噪音）
				out = append(out, string(asciiPart))
			}
			changed = true
		case !hasCJK && len([]rune(w)) <= 2 && !hasCJKAnchor:
			// 中文锚出现前的纯短英文词——剔除（防裸英文噪音起始，如 "IF 2024"）
			changed = true
		default:
			out = append(out, w)
			if hasCJK {
				hasCJKAnchor = true
			}
		}
	}
	if !changed {
		return ""
	}
	normalized := strings.Join(out, " ")
	if normalized == keyword || !hasCJKAnchor {
		return ""
	}
	return normalized
}

// editionSuffixes 中文版式词后缀（长在前优先修剪）。§59.194。
// §59.218: +十周年/周年纪念版/纪念版/蓝光版/蓝光碟（角斗士十周年加长版/
// 死亡诗社蓝光版案）；后缀修剪改循环剥净（角斗士案两连后缀实证）。
var editionSuffixes = []string{"十周年纪念版", "周年纪念版", "终极剪辑版", "导演剪辑版", "数字修复版", "未删减版", "未删节版", "修复版", "重制版", "加长版", "完整版", "剪辑版", "十周年", "纪念版", "蓝光版", "蓝光碟", "修复", "重制"}

var reEditionWordOnly = regexp.MustCompile(`^(?:终极剪辑版|导演剪辑版|数字修复版|未删减版|未删节版|修复版|重制版|加长版|完整版|剪辑版|修复|重制|蓝光版|蓝光碟)$`)

// normalizeSpecVariants §59.220: 规格词拼写变体归一（token 前缀替换，
// 有界词表）。minbd→MiniBD——收藏转发名少 i vs 官站 MiniBD 词形
// （Inception/True Grit 案：AND 全灭，包子/大青虫同源转发名恰命中）。
func normalizeSpecVariants(kw string) string {
	words := strings.Fields(kw)
	out := make([]string, 0, len(words))
	for _, w := range words {
		lw := strings.ToLower(w)
		if strings.HasPrefix(lw, "minbd") {
			w = "MiniBD" + w[len("minbd"):]
		}
		out = append(out, w)
	}
	return strings.Join(out, " ")
}

// stripAnnotNoiseTokens §59.218: 按词剥离 CJK 注记噪声（音轨/碟媒）。
func stripAnnotNoiseTokens(kw string) string {
	words := strings.Fields(kw)
	out := make([]string, 0, len(words))
	for _, w := range words {
		if reAnnotNoiseToken.MatchString(w) {
			continue
		}
		out = append(out, w)
	}
	return strings.Join(out, " ")
}

// reAnnotNoiseToken §59.218: 主路径 CJK 注记噪声词——音轨注记（国英双语/
// 国粤双语/国粤日三语 等）与碟媒注记（蓝光版）。站方副标题不保证携带，
// AND 阻断词（勇敢的心案：kw 残"国英双语"致 0 行；探针"勇敢的心 1995"
// 18 行含 tid=4260 同版本）。
var reAnnotNoiseToken = regexp.MustCompile(`^(?:国英|国粤|国日|国粤日|粤英|英日|中日)?[双三]语$|^蓝光[版碟]?$`)

// PurifyChineseKeyword §59.194: 中文关键词净化降级变体。
// 站方副标题桥要求精确子串，而中文关键词常带装饰元素（六案实证：
// 午夜凶铃 tid=270313/宝贝计划 tid=5824/安娜·卡列尼娜 tid=46343 等）：
// ① 版式词 token 剥离——站方副标题纯片名（REMASTERED 在英文标题，
//
//	中文 4K修复版 词 AND 必失配）
//
// ② CJK 词尾版式后缀修剪——"宝贝计划加长版"（词内粘连）→"宝贝计划"
// ③ 中点消解——"安娜·卡列尼娜"→"安娜卡列尼娜"（副标题无点形态）
// 无 CJK / 无变化 / 剥空 均返回 ""（调用方跳过该轮）。
func PurifyChineseKeyword(keyword string) string {
	if !hasCJKWord(keyword) {
		return ""
	}
	var out []string
	changed := false
	for _, w := range strings.Fields(keyword) {
		if !hasCJKWord(w) {
			out = append(out, w)
			continue
		}
		if reEditionWordOnly.MatchString(w) { // 整词版式词 → 剥
			changed = true
			continue
		}
		nw := strings.NewReplacer("·", "", "・", "", "•", "").Replace(w)
		trimmed := false
		for { // §59.218: 循环剥净——角斗士十周年加长版两连后缀实证
			peeled := false
			for _, suf := range editionSuffixes {
				if strings.HasSuffix(nw, suf) {
					nw = strings.TrimSuffix(nw, suf)
					changed = true
					trimmed = true
					peeled = true
					break
				}
			}
			if !peeled {
				break
			}
		}
		if nw != w {
			changed = true
		}
		// 版式词剥净后剩纯规格（4K修复版→4K）、单字残片或空 → 整词丢弃
		if nw == "" || (hasCJKWord(w) && (!hasCJKWord(nw) || (trimmed && cjkRunes(nw) < 2))) {
			continue
		}
		out = append(out, nw)
	}
	if !changed {
		return ""
	}
	res := strings.Join(out, " ")
	if res == keyword || res == "" || !hasCJKWord(res) {
		return ""
	}
	return res
}

// cjkRunes: 字符串中 CJK 字数
func cjkRunes(s string) int {
	n := 0
	for _, r := range s {
		if (r >= 0x4E00 && r <= 0x9FFF) || (r >= 0x3400 && r <= 0x4DBF) {
			n++
		}
	}
	return n
}

// HasCJKWord §59.184: hasCJKWord 导出形态——孤儿恢复 B 补线跨包消费。
func HasCJKWord(s string) bool { return hasCJKWord(s) }

// LeadingCJKSegment §59.184: leadingCJKSegment 导出形态——孤儿恢复 B 补线跨包消费。
func LeadingCJKSegment(title string) string { return leadingCJKSegment(title) }

// stripCJKWords 剥离关键词中的 CJK 词，保留 ASCII 部分。
func stripCJKWords(s string) string {
	fields := strings.Fields(s)
	var out []string
	for _, f := range fields {
		if !hasCJKWord(f) {
			out = append(out, f)
		}
	}
	return strings.Join(out, " ")
}

// stripASCIIWords 剥离关键词中的 ASCII 词，保留 CJK 部分。
func stripASCIIWords(s string) string {
	fields := strings.Fields(s)
	var out []string
	for _, f := range fields {
		if hasCJKWord(f) {
			out = append(out, f)
		}
	}
	return strings.Join(out, " ")
}

var videoExtensions = []string{".mkv", ".mp4", ".avi", ".ts", ".m2ts", ".wmv", ".flv", ".mov"}

func findMainVideoFile(fileTree map[string]int64) string {
	var bestPath string
	var bestSize int64
	for path, size := range fileTree {
		lower := strings.ToLower(path)
		for _, ext := range videoExtensions {
			if strings.HasSuffix(lower, ext) && size > bestSize {
				bestPath = path
				bestSize = size
				break
			}
		}
	}
	return bestPath
}

func extractFromFileTree(fileTree map[string]int64) (keyword, groupName string) {
	videoFile := findMainVideoFile(fileTree)
	if videoFile == "" {
		return "", ""
	}
	if idx := strings.LastIndex(videoFile, "/"); idx >= 0 {
		videoFile = videoFile[idx+1:]
	}
	keyword = ExtractSearchKeyword(videoFile)
	groupName = ExtractGroupName(videoFile)
	return
}

func KeywordStartsWithYear(keyword string) bool {
	if len(keyword) < 4 {
		return false
	}
	year, err := strconv.Atoi(keyword[:4])
	if err != nil {
		return false
	}
	return year >= 1920 && year <= 2030
}

// KeywordHasNoTitle 判断关键词是否缺少有效标题内容（应跳过 L2，走文件级恢复）。
// 三种情况：
//   - 空关键词
//   - 以年份开头且后续只有规格词（纯中文标题残留，如 "2016 1080p BluRay"）
//   - 第一个词是 1-3 位纯数字（续集编号，如 招魂2 → "2 2016 1080p"）
//
// 以年份开头但后续含 CJK 字符或非规格英文词时不视为"无标题"
// （片名本身以年份开头，如 "2001太空漫游"、"2001 A Space Odyssey"）。
// isResolutionWord: 分辨率/规格词判定（含 2K/8K/3D 等词典外延）。
func isResolutionWord(w string) bool {
	lw := strings.ToLower(w)
	for _, r := range resolutionKeywords {
		if lw == r {
			return true
		}
	}
	switch lw {
	case "2k", "8k", "3d", "sdr", "hdr", "uhd", "hd":
		return true
	}
	return false
}

// reTitlelessSpecToken 无标题价值的 token：纯数字 / 分辨率规格（4K/2K/8K/3D/1080p…）/
// 中文版式词及其混合形态（"4K修复版"/"2K修复版"/"修复版"）。§59.190 ②:
// 浪人.4K修复版 / 爱情万岁.2K修复版 案——英文主体仅剩年份+规格时关键词无片名。
var reTitlelessSpecToken = regexp.MustCompile(`(?i)^(?:\d{1,4}k)?(?:(?:终极|導演|导演)?剪辑版?|終極剪輯版?|加长版?|加長版?|修复版?|修復版?|重制版?|重製版?|完整版?|未删[减节]版?|数字修复|數字修復)?$`)

// keywordAllTitleless 关键词全部由规格/版式/数字 token 构成 = 无片名
// （触发 chineseTitleFallback 补中文片名）。
// isRomanNumeralToken §59.204 F3: 纯罗马数字 token（I/II/III…X）——合集部数
// 编号非标题（色即是空I.II合集：英文主体仅剩 "II合集"，II 是编号）。
func isRomanNumeralToken(w string) bool {
	switch strings.ToUpper(w) {
	case "I", "II", "III", "IV", "V", "VI", "VII", "VIII", "IX", "X":
		return true
	}
	return false
}

// isCollectionMarkerToken §59.204 F3: 合集结构词（合集/全集）——标记非标题。
// 粘着形态（"II合集"＝罗马编号+合集一体，色即是空案）同判：剥去合集/全集
// 后缀，余部为空或罗马数字/数字＝纯结构标记。
func isCollectionMarkerToken(w string) bool {
	switch w {
	case "合集", "全集":
		return true
	}
	if strings.HasSuffix(w, "合集") || strings.HasSuffix(w, "全集") {
		head := w[:len(w)-len("合集")]
		if head == "" {
			return true
		}
		if _, err := strconv.Atoi(head); err == nil {
			return true
		}
		if isRomanNumeralToken(head) {
			return true
		}
	}
	return false
}

func keywordAllTitleless(keyword string) bool {
	// §59.201: 数字 token 紧邻年份（19xx/20xx）= 数字片名（65.2023/300.2006
	// 家族）——视为有标题（"65" 即站方标题词，AND 可命中）。
	words := strings.Fields(keyword)
	for i, w := range words {
		// §59.204 F3: 罗马数字编号/合集结构词不构成标题
		if isRomanNumeralToken(w) || isCollectionMarkerToken(w) {
			continue
		}
		if !reTitlelessSpecToken.MatchString(w) && !isResolutionWord(w) {
			if _, err := strconv.Atoi(w); err != nil {
				return false // 存在非规格词 → 有标题
			}
		}
		if i+1 < len(words) && len(w) >= 2 && len(w) <= 3 {
			if _, err := strconv.Atoi(w); err == nil && reYearWord.MatchString(words[i+1]) {
				return false // 2-3 位数字+年份相邻 = 数字片名（1 位=序号污染，§59.190 用例保持）
			}
		}
	}
	return true
}

// reYearWord §59.201: 独立年份词（19xx/20xx）。
var reYearWord = regexp.MustCompile(`^(?:19|20)\d{2}$`)

func KeywordHasNoTitle(keyword string) bool {
	if keyword == "" {
		return true
	}
	// §59.190 ②: 全规格词形态（"4K修复版 1998 1080p"）= 无片名
	if keywordAllTitleless(keyword) {
		return true
	}
	if KeywordStartsWithYear(keyword) {
		return !hasTitleAfterYear(keyword[4:])
	}
	fields := strings.Fields(keyword)
	if len(fields) > 0 && len(fields[0]) <= 3 {
		if _, err := strconv.Atoi(fields[0]); err == nil {
			// §59.201: 2-3 位数字+年份相邻 = 数字片名（65 2023/300 2006），
			// 非序号污染；1 位数字保持序号语义（"2 2016"用例）
			if len(fields[0]) >= 2 && len(fields) > 1 && reYearWord.MatchString(fields[1]) {
				return false
			}
			// §59.216: 其余 token 含标题成分时不判无标题——行首短数字是
			// 序号/单位形态（"0 0兆赫 2019"案：0.0MHz 片名点变空格，"0兆赫"
			// 混合 token 是标题，守卫曾越权秒拒 0 站搜索）
			if len(fields) > 1 && !keywordAllTitleless(strings.Join(fields[1:], " ")) {
				return false
			}
			return true
		}
	}
	return false
}

// hasTitleAfterYear 检查年份前缀之后是否有有效标题内容：
// CJK 字符或 4+ 字母且非规格术语的英文词。
func hasTitleAfterYear(rest string) bool {
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return false
	}
	for _, r := range rest {
		if (r >= 0x4E00 && r <= 0x9FFF) || (r >= 0x3400 && r <= 0x4DBF) {
			return true
		}
	}
	for _, f := range strings.Fields(rest) {
		lower := strings.ToLower(strings.Trim(f, "0123456789"))
		if len(lower) >= 4 && !yearPrefixSpecTerms[lower] {
			return true
		}
	}
	return false
}

var yearPrefixSpecTerms = map[string]bool{
	"bluray": true, "remux": true, "remastered": true,
	"web-dl": true, "webdl": true, "webrip": true,
	"hdtv": true, "dvdrip": true, "hd-dvd": true,
	"atmos": true, "truehd": true,
}

var audioExtensions = []string{".flac", ".wav", ".ape", ".tta", ".wv", ".mp3", ".m4a", ".ogg", ".opus", ".aac", ".dsf", ".dff", ".wma", ".aiff", ".m4b"}

// DetectMusicFromDir 读取目录内容，判断是否为音乐资源（有音频文件且无视频文件）。
// 检查直接子文件和一级子目录（CD1/CD2 等分碟结构）。
// SACD ISO 镜像（.iso 无标准音频扩展名）通过目录名含 "SACD" 识别。
func DetectMusicFromDir(dirPath string) bool {
	// SACD（Super Audio CD）是纯音频格式，目录名含 SACD 直接判定为音乐
	if strings.Contains(strings.ToUpper(filepath.Base(dirPath)), "SACD") {
		return true
	}

	hasAudio := detectAudioInDir(dirPath)
	if !hasAudio {
		// 检查一级子目录（CD1/CD2 分碟结构）
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			return false
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			if detectAudioInDir(filepath.Join(dirPath, e.Name())) {
				hasAudio = true
				break
			}
		}
	}
	return hasAudio
}

// detectAudioInDir 检查目录内是否有音频文件（无视频文件）。
func detectAudioInDir(dirPath string) bool {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return false
	}
	hasAudio := false
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		for _, v := range videoExtensions {
			if ext == v {
				return false
			}
		}
		for _, a := range audioExtensions {
			if ext == a {
				hasAudio = true
			}
		}
	}
	return hasAudio
}

func ExtractMusicKeyword(title string) string {
	if title == "" {
		return ""
	}

	// Scene naming: Artist-Album-24BIT-FLAC-2023-GRP
	if result, ok := musicSceneNaming(title); ok {
		return musicNormalize(result)
	}

	// Freeform naming: Artist - Album Year [Params]
	if keyword, ok := musicFreeformNaming(title); ok {
		keyword = musicStripFormatNoise(keyword)
		return musicNormalize(keyword)
	}

	// Fallback: no ' - ' separator, process brackets and noise
	s := musicStripCurlyBraces(title)
	s = musicProcessSquareBrackets(s)
	s = musicStripDatePrefix(s)
	s = musicStripYearParens(s)
	s = musicStripTrailingFormat(s)
	s = musicStripFormatNoise(s)
	return musicNormalize(s)
}

var musicFreeformDashRe = regexp.MustCompile(`\s+[-–—]\s+`)
var musicAlbumYearRe = regexp.MustCompile(`\s+[\(\[\{]?(19|20)\d{2}[\)\]\}]?`)

// musicFreeformNaming 处理 "Artist - Album Year [Params]" 格式。
// 第一个 ' - '/' – ' 分割 Artist 和 Album；Album 中的年份标记名称/参数分界线。
// 年份保留（区分同名专辑），年份后内容全部丢弃（FLAC/目录号/规格等参数）。
func musicFreeformNaming(title string) (string, bool) {
	prefixYear, s := musicExtractDatePrefix(title)

	dashLoc := musicFreeformDashRe.FindStringIndex(s)
	if dashLoc == nil {
		return "", false
	}
	artist := strings.TrimSpace(s[:dashLoc[0]])
	rest := strings.TrimSpace(s[dashLoc[1]:])
	if artist == "" || rest == "" {
		return "", false
	}

	yearLoc := musicAlbumYearRe.FindStringIndex(rest)

	var album string
	year := prefixYear
	if yearLoc != nil {
		album = musicStripAllBrackets(strings.TrimSpace(rest[:yearLoc[0]]))
		if year == "" {
			year = strings.Trim(rest[yearLoc[0]:yearLoc[1]], " ()[]{}")
		}
	} else {
		album = musicStripAllBrackets(rest)
	}

	if album == "" {
		album = rest
	}

	keyword := album
	if year != "" {
		keyword += " " + year
	}
	return keyword, true
}

// musicExtractDatePrefix 提取日期前缀中的年份。
// "2013 - Artist..." → ("2013", "Artist...")
// "[2021.09.09] Artist..." → ("2021", "Artist...")
func musicExtractDatePrefix(s string) (year, rest string) {
	if m := regexp.MustCompile(`^(\d{4})\s*[-–—]\s+`).FindStringSubmatch(s); m != nil {
		return m[1], s[len(m[0]):]
	}
	if m := regexp.MustCompile(`^\[(\d{4})(?:\.\d{2}(?:\.\d{2})?)?\]\s*`).FindStringSubmatch(s); m != nil {
		return m[1], s[len(m[0]):]
	}
	if m := regexp.MustCompile(`^(\d{4})\.\d{2}\.\d{2}\s*[-–—]?\s*`).FindStringSubmatch(s); m != nil {
		return m[1], s[len(m[0]):]
	}
	return "", s
}

// musicStripAllBrackets 剥离所有括号及其内容。
func musicStripAllBrackets(s string) string {
	for _, pair := range [][2]string{{"(", ")"}, {"[", "]"}, {"{", "}"}} {
		for {
			i := strings.Index(s, pair[0])
			if i < 0 {
				break
			}
			j := strings.Index(s[i:], pair[1]) //nolint:gosec // i 来自 Index ≥0 严格小于 len(s)
			if j < 0 {
				break
			}
			s = s[:i] + " " + s[i+j+len(pair[1]):] //nolint:gosec // i,j 来自 Index ≥0 有界
		}
	}
	return strings.TrimSpace(s)
}

func musicStripCurlyBraces(s string) string {
	for {
		start := strings.Index(s, "{")
		if start < 0 {
			break
		}
		end := strings.Index(s[start:], "}")
		if end < 0 {
			break
		}
		s = s[:start] + " " + s[start+end+1:]
	}
	return s
}

var musicBracketNoisePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(FLAC|APE|WAV|MP3|DSD|DSF|CUE|SACD|ISO)`),
	regexp.MustCompile(`(?i)(bit|kHz|KHz)`),
	regexp.MustCompile(`^\d+-\d+$`),
	regexp.MustCompile(`(?i)^(EAC|XLD|OpenCD)$`),
	regexp.MustCompile(`(?i)(Genie|KKBOX|Bugs|Tidal|MQA|Spotify)`),
	regexp.MustCompile(`^\d{4}(\.\d{2}(\.\d{2})?)?$|^\d{6}$|^\d{8}$`),
	regexp.MustCompile(`(?i)(版|初回|限定|復刻|Remaster|Edition)`),
	regexp.MustCompile(`(?i)^(Album|Single|EP|Live|Mini)`),
	regexp.MustCompile(`C\d{2}|例大祭|Comiket|ボーマス|M3-`),
	regexp.MustCompile(`^[A-Z]{2,5}[-_]?\d{2,6}`),
	regexp.MustCompile(`(?i)^(US|EU|JP|KR|HK|TW|CN|DE|SE|FI|FR|NL|IT|AU|UK)$`),
}

func musicIsBracketNoise(content string) bool {
	if len([]rune(content)) > 20 {
		return false
	}
	for _, p := range musicBracketNoisePatterns {
		if p.MatchString(content) {
			return true
		}
	}
	return false
}

func musicProcessSquareBrackets(s string) string {
	for _, pair := range [][2]string{{"[", "]"}, {"【", "】"}} {
		open, closeCh := pair[0], pair[1]
		for {
			start := strings.Index(s, open)
			if start < 0 {
				break
			}
			end := strings.Index(s[start:], closeCh)
			if end < 0 {
				break
			}
			content := s[start+len(open) : start+end]
			rest := s[start+end+len(closeCh):]
			if musicIsBracketNoise(content) {
				s = s[:start] + " " + rest
			} else {
				s = s[:start] + " " + content + " " + rest
			}
		}
	}
	return s
}

var musicSceneAnchors = map[string]bool{
	"CD": true, "CDR": true, "CDEP": true, "CDM": true,
	"WEB": true, "WEBFLAC": true, "2CD": true, "3CD": true,
	"DVD": true, "VINYL": true, "12INCH_VINYL": true, "7_INCH_VINYL": true,
	"16BIT": true, "24BIT": true,
	"FLAC": true, "FLACME": true,
	"REMASTERED": true, "REPACK": true, "GOLD": true,
	"BOOTLEG": true, "PROMO": true, "SPLIT": true,
	"SINGLE": true, "EP": true,
}

var musicCountryCodes = map[string]bool{
	"DE": true, "SE": true, "FI": true, "CN": true,
	"US": true, "JP": true, "KR": true, "UK": true,
	"FR": true, "NL": true, "IT": true, "AU": true,
}

var musicYearRe = regexp.MustCompile(`^(19|20)\d{2}$`)
var musicDeluxeRe = regexp.MustCompile(`(?i)^(Deluxe|Limited)`)

func musicIsMetadataAnchor(seg string) bool {
	upper := strings.ToUpper(seg)
	if musicSceneAnchors[upper] || musicCountryCodes[upper] {
		return true
	}
	if strings.HasPrefix(seg, "(") {
		return true
	}
	if musicYearRe.MatchString(seg) || musicDeluxeRe.MatchString(seg) {
		return true
	}
	return false
}

func musicSceneNaming(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if strings.Contains(s, " ") || hasCJKChar(s) || !strings.Contains(s, "-") {
		return s, false
	}
	segs := strings.Split(s, "-")
	var filtered []string
	for _, seg := range segs {
		if seg != "" && seg != "_" {
			filtered = append(filtered, seg)
		}
	}
	anchorIdx := -1
	for i, seg := range filtered {
		if musicIsMetadataAnchor(seg) {
			anchorIdx = i
			break
		}
	}
	if anchorIdx <= 0 {
		return s, false
	}
	keep := filtered[:anchorIdx]
	if len(keep) < 1 {
		return s, false
	}
	return strings.Join(keep, "-"), true
}

var musicDatePrefixPatterns = []struct {
	re   *regexp.Regexp
	repl string
}{
	{regexp.MustCompile(`^\d{4}\s*[-–—]\s+`), ""},
	{regexp.MustCompile(`^\d{4}\.\d{2}\.\d{2}\s*[-–—]?\s*`), ""},
	{regexp.MustCompile(`^\d{4}\.\d{2}\s+`), ""},
	{regexp.MustCompile(`^\d{4}\.\s+`), ""},
	{regexp.MustCompile(`^\d{4}-([A-Z])`), "$1"},
}

func musicStripDatePrefix(s string) string {
	for _, p := range musicDatePrefixPatterns {
		s = p.re.ReplaceAllString(s, p.repl)
	}
	return s
}

var musicFYearRe = regexp.MustCompile(`(19|20)\d{2}`)
var musicFFormatRe = regexp.MustCompile(`(?i)^(FLAC|WAV|APE|MP3|DSD|M4A|OGG)$`)
var musicFTypeRe = regexp.MustCompile(`(?i)^(album|EP|Single)$`)

func musicShouldStripParen(content string) bool {
	return musicFYearRe.MatchString(content) ||
		musicFFormatRe.MatchString(content) ||
		musicFTypeRe.MatchString(content)
}

func musicStripYearParens(s string) string {
	for _, pair := range [][2]string{{"(", ")"}, {"（", "）"}} {
		open, closeCh := pair[0], pair[1]
		var buf strings.Builder
		i := 0
		for i < len(s) {
			idx := strings.Index(s[i:], open)
			if idx < 0 {
				buf.WriteString(s[i:])
				break
			}
			closeIdx := strings.Index(s[i+idx+len(open):], closeCh)
			if closeIdx < 0 {
				buf.WriteString(s[i:])
				break
			}
			content := strings.TrimSpace(s[i+idx+len(open) : i+idx+len(open)+closeIdx])
			parenEnd := i + idx + len(open) + closeIdx + len(closeCh)
			buf.WriteString(s[i : i+idx])
			if musicShouldStripParen(content) {
				buf.WriteByte(' ')
			} else {
				buf.WriteString(s[i+idx : parenEnd])
			}
			i = parenEnd
		}
		s = buf.String()
	}
	return s
}

var musicTrailingFormatPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\s+(?:-\s+)?(?:FLAC|Flac|flac)(?:\s*分轨)?$`),
	regexp.MustCompile(`\s+分轨$`),
}

func musicStripTrailingFormat(s string) string {
	for {
		changed := false
		for _, p := range musicTrailingFormatPatterns {
			newS := p.ReplaceAllString(s, "")
			if newS != s {
				s = newS
				changed = true
			}
		}
		s = strings.TrimRight(s, " -")
		if !changed {
			break
		}
	}
	return s
}

var musicFormatNoisePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(FLAC|APE|WAV|MP3|AAC|M4A|OGG|DSD|DSF|CUE)\b`),
	regexp.MustCompile(`(?i)\b\d+bit\b`),
	regexp.MustCompile(`\b\d{1,2}-\d{2,3}\b`), // 24-96, 16-44 (bitdepth-samplerate)
}

func musicStripFormatNoise(s string) string {
	for _, p := range musicFormatNoisePatterns {
		s = p.ReplaceAllString(s, " ")
	}
	return s
}

var musicInvisibleRunes = map[rune]bool{
	'\u200E': true, '\u200B': true, '\u200C': true, '\u200D': true, '\uFEFF': true,
}

var musicSymbolsToSpace = map[rune]bool{
	'(': true, ')': true, '[': true, ']': true, '{': true, '}': true,
	'（': true, '）': true, '【': true, '】': true,
	'《': true, '》': true, '「': true, '」': true,
	'『': true, '』': true, '〈': true, '〉': true,
	':': true, '：': true, ';': true, '；': true,
	'-': true, '–': true, '—': true, '‐': true, '－': true,
	'~': true, '～': true, '〜': true,
	'.': true, '．': true, '。': true,
	',': true, '，': true, '、': true,
	'!': true, '！': true, '?': true, '？': true,
	'+': true, '#': true, '@': true, '$': true, '%': true,
	'/': true, '／': true, '=': true, '＝': true,
	'^': true, '`': true, '\\': true, '<': true, '>': true,
	'|': true, '｜': true, '&': true, '＆': true,
	'…': true, '_': true, '・': true, '·': true, '•': true,
	'*': true, '＊': true,
	'\'': true, '\u2018': true, '\u2019': true,
	'"': true, '\u201C': true, '\u201D': true,
	'†': true, '′': true, '″': true, '→': true,
	'∶': true, '‧': true, '⁄': true,
}

var musicMultiSpaceRe = regexp.MustCompile(`\s+`)

func musicNormalize(s string) string {
	var buf strings.Builder
	for _, r := range s {
		if musicInvisibleRunes[r] {
			continue
		}
		if r >= '\uFF01' && r <= '\uFF5E' {
			r -= '\uFEE0'
		} else if r == '\u3000' {
			r = ' '
		}
		if musicSymbolsToSpace[r] {
			buf.WriteRune(' ')
		} else {
			buf.WriteRune(r)
		}
	}
	result := musicMultiSpaceRe.ReplaceAllString(buf.String(), " ")
	// CJK 和数字之间插入空格，帮助 NexusPHP 分词匹配
	result = musicCJKDigitRe.ReplaceAllString(result, "$1 $2")
	result = musicDigitCJKRe.ReplaceAllString(result, "$1 $2")
	return strings.TrimSpace(result)
}

var musicCJKDigitRe = regexp.MustCompile(`([\p{Han}\p{Hiragana}\p{Katakana}])(\d)`)
var musicDigitCJKRe = regexp.MustCompile(`(\d)([\p{Han}\p{Hiragana}\p{Katakana}])`)

func hasCJKChar(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) {
			return true
		}
	}
	return false
}

// extractMeaningfulTitleWords 从标题中提取有意义的 ASCII 词（≥4 字符），
var shortStopwords = map[string]bool{
	"the": true, "and": true, "for": true, "but": true, "not": true,
	"you": true, "all": true, "any": true, "was": true, "had": true,
	"has": true, "her": true, "his": true, "its": true, "our": true,
	"who": true, "did": true, "can": true, "how": true, "too": true,
	"own": true, "off": true, "out": true, "get": true, "let": true,
	"via": true, "per": true, "yet": true, "nor": true, "put": true,
	"set": true, "try": true, "end": true, "are": true, "been": true,
	"into": true, "from": true, "with": true,
}

// 排除年份、分辨率、介质/地区词、编解码术语、组名等通用词。
func extractMeaningfulTitleWords(title, groupName string) []string {
	if title == "" {
		return nil
	}
	// truncation retry 时 groupName 为空，需从标题自动提取，否则组名会成为 meaningful word
	if groupName == "" {
		groupName = ExtractGroupName(title)
	}
	lower := strings.ToLower(title)
	groupLower := strings.ToLower(groupName)
	words := strings.FieldsFunc(lower, func(r rune) bool {
		return r == ' ' || r == '.' || r == '-' || r == '_' || r == '/' ||
			r == '(' || r == ')' || r == '[' || r == ']' || r == '￡' || r == ':'
	})
	var meaningful []string
	for _, w := range words {
		if len(w) < 3 {
			continue
		}
		if len(w) == 3 && shortStopwords[w] {
			continue
		}
		hasCJK := false
		for _, r := range w {
			if (r >= 0x4E00 && r <= 0x9FFF) || (r >= 0x3400 && r <= 0x4DBF) {
				hasCJK = true
				break
			}
		}
		if hasCJK {
			continue
		}
		if _, err := strconv.Atoi(w); err == nil {
			continue
		}
		skip := false
		for _, res := range resolutionKeywords {
			if w == res || strings.Contains(w, res) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		for _, term := range mediumAndRegionTerms {
			if w == strings.ToLower(term) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		switch w {
		case "x264", "x265", "h264", "h265", "hevc", "avc", "vc1",
			"10bit", "8bit", "mnhd", "minbd", "ddp5", "ddp",
			"atmos", "truehd", "ac3", "aac", "flac", "lpcm",
			"hdr", "dovi", "sdr", "repack", "remux", "extended",
			"uncut", "imax", "hybrid", "internal", "readnfo",
			"proper", "diy", "hsbs", "sbs", "2audio", "audio",
			"movie", "season", "complete", "series", "part",
			"collection", "trilogy", "duology", "multi", "oar":
			continue
		}
		if w == groupLower {
			continue
		}
		meaningful = append(meaningful, w)
	}
	return meaningful
}

func extractCJKSubstrings(s string) []string {
	var result []string
	var current []rune
	for _, r := range s {
		if (r >= 0x4E00 && r <= 0x9FFF) || (r >= 0x3400 && r <= 0x4DBF) {
			current = append(current, r)
		} else {
			if len(current) >= 2 {
				result = append(result, string(current))
			}
			current = nil
		}
	}
	if len(current) >= 2 {
		result = append(result, string(current))
	}
	return result
}

// romanToValue 将罗马数字（I~IX）转为整数，无效返回 0。
func romanToValue(s string) int {
	switch strings.ToUpper(s) {
	case "I":
		return 1
	case "II":
		return 2
	case "III":
		return 3
	case "IV":
		return 4
	case "V":
		return 5
	case "VI":
		return 6
	case "VII":
		return 7
	case "VIII":
		return 8
	case "IX":
		return 9
	default:
		return 0
	}
}

// extractSequelNumber 从标题中提取续集号（1-9），返回 0 表示未检测到。
//
// 检测策略：
//  1. CJK 字符后紧跟的 ASCII 段（无分隔符）：白发魔女传I → 1, 招魂2 → 2
//     CJK 后缀中 I~IX 和 1~9 均有效（无歧义）。
//  2. 英文标题中独立的罗马数字词（II~IX，排除歧义的 I）：
//     The Bride with White Hair II → 2, Rocky IV → 4
//     仅扫描年份（1900~2099）之前出现的词，避免误匹配技术元数据。
var reSeasonMarker = regexp.MustCompile(`(?i)\bS(\d{1,2})(?:E\d{1,3})?\b`)

// limitKeywordTerms §59.206: 站方搜索词数上限感知——springsunday 实测 5 词
// 以上 AND 0 结果（Aliens 案 6/7/9 词全灭 vs ≤5 词全中）。超 5 词时剥规格词
// （编码/音频/媒介/分辨率——验证层自有 TechProfile 比对，搜索层规格词仅是
// 收窄，剥除只放宽召回）。≤5 词零变化（既有行为免疫）。
func limitKeywordTerms(kw string) string {
	words := strings.Fields(kw)
	if len(words) <= 5 {
		return kw
	}
	out := make([]string, 0, len(words))
	for _, w := range words {
		if isKeywordSpecWord(w) {
			continue
		}
		out = append(out, w)
	}
	return strings.Join(out, " ")
}

// isKeywordSpecWord §59.206: 关键词规格词判定（编码/音频/媒介/分辨率）。
func isKeywordSpecWord(w string) bool {
	lw := strings.ToLower(w)
	switch lw {
	case "x264", "x265", "h264", "h265", "h266", "avc", "hevc", "av1", "vc1", "xvid", "divx",
		"dts", "ac3", "aac", "flac", "truehd", "ddp", "dd", "lpcm", "atmos", "ddp5",
		"minibd", "remux", "web-dl", "webdl", "webrip", "hdtv", "blu-ray", "bluray", "bdrip", "hdrip",
		"10bit", "8bit", "sdr", "hdr", "hdr10", "dovi", "dv", "hdr10+":
		return true
	}
	// 数字后缀音频词（DD5/DDP5.1/DTS-HD 等）
	if reAudioNumToken.MatchString(lw) {
		return true
	}
	// 粘连规格+分辨率 token（MiniBD1080P）
	for _, p := range []string{"minibd", "remux", "webdl", "web", "hdtv", "bluray", "blu-ray", "bdrip", "hdrip", "webrip"} {
		if strings.HasPrefix(lw, p) && isResolutionWord(strings.TrimPrefix(lw, p)) {
			return true
		}
	}
	return isResolutionWord(lw)
}

// reAudioNumToken §59.206: 数字后缀音频词（dd5/ddp5.1/dts-hd 等）。
var reAudioNumToken = regexp.MustCompile(`^(?:dd|ddp|ac|aac|dts|truehd|lpcm|atmos)[-\d.]*$`)

// reAlphaDigitGlue §59.206: 字母段(≥3)+纯数字尾粘连。仅拆数字后无字母的
// 形态（Aliens2/Aliens3）；分辨率后缀粘连（MiniBD1080P——数字后跟 P，
// 无 \b 边界）不拆——拆出独立 "1080P" 词实测反致 0 结果（用户实测+
// 探针双证）。x264/H265/S01 短字母段免疫。
var reAlphaDigitGlue = regexp.MustCompile(`([A-Za-z]{3,})(\d{1,4})\b`)

// reSequelPair §59.205: 相邻罗马续集号对（I.II / II.III）——多部合集形态。
// 仅罗马对（数字对假阳性高：音频声道 "2.0 2Audios"/年份区间同形，无实证案例）。
var reSequelPair = regexp.MustCompile(`(?i)(?:^|[^0-9a-z])[ivx]{1,4}[._ -]+[ivx]{1,4}(?:$|[^0-9a-z])`)

func extractSequelNumber(title string) int {
	if title == "" {
		return 0
	}

	// §59.205: 多部合集（I.II 合集）——续集号非单一判别值，返回 0 不参与
	// 反驳。色即是空I.II合集案：源 CJK 路取 I=1 × 候选英文路取 II=2，
	// 双正序列误杀（家园/南洋正确候选 sequel_refute 出局）。
	if reSequelPair.MatchString(title) {
		return 0
	}

	// 策略 0（§59.184）：季标记 S01/S02E03——季号是强身份信号，不同季=不同内容
	if m := reSeasonMarker.FindStringSubmatch(title); len(m) > 1 {
		if n, err := strconv.Atoi(m[1]); err == nil && n >= 1 && n <= 99 {
			return n
		}
	}

	// 策略 1：CJK 后紧跟的 ASCII 段
	runes := []rune(title)
	for i := 1; i < len(runes); i++ {
		prev := runes[i-1]
		if (prev < 0x4E00 || prev > 0x9FFF) && (prev < 0x3400 || prev > 0x4DBF) {
			continue
		}
		r := runes[i]
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			continue
		}
		var seg []rune
		for j := i; j < len(runes); j++ {
			rj := runes[j]
			if (rj >= 'a' && rj <= 'z') || (rj >= 'A' && rj <= 'Z') || (rj >= '0' && rj <= '9') {
				seg = append(seg, rj)
			} else {
				break
			}
		}
		s := string(seg)
		if n, err := strconv.Atoi(s); err == nil && n >= 1 && n <= 9 {
			return n
		}
		if n := romanToValue(s); n > 0 {
			return n
		}
		var alphaPrefix []rune
		for _, c := range seg {
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
				alphaPrefix = append(alphaPrefix, c)
			} else {
				break
			}
		}
		if len(alphaPrefix) >= 1 && len(alphaPrefix) <= 4 {
			if n := romanToValue(string(alphaPrefix)); n > 0 {
				return n
			}
		}
	}

	// 策略 2：英文标题中独立的罗马数字词（II~IX），扫描到年份停止
	lower := strings.ToLower(title)
	words := strings.FieldsFunc(lower, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	})
	for _, w := range words {
		if n, err := strconv.Atoi(w); err == nil && n >= 1900 && n <= 2099 {
			break
		}
		if n := romanToValue(w); n >= 2 && n <= 9 {
			return n
		}
	}

	return 0
}

// titleKeywordRelevant 检查源标题与候选标题是否有内容关联。
// 返回 true 表示关联或无法判定（跳过检查），false 表示确定无关联。
// TitleRelevant §59.61 D3: 直达后标题轻校验（公开包装——metadata 直达链复用，
// 判定语义与 L2 搜索闸门一致：本地名关键词 vs 详情标题相关性）。
func TitleRelevant(sourceTitle, candidateTitle string) bool {
	meaningfulWords := extractMeaningfulTitleWords(sourceTitle, ExtractGroupName(sourceTitle))
	sourceCJK := extractCJKSubstrings(sourceTitle)
	return titleKeywordRelevant(meaningfulWords, sourceCJK, sourceTitle, candidateTitle)
}

func titleKeywordRelevant(meaningfulWords []string, sourceCJK []string, sourceTitle, candidateTitle string) bool {
	// 续集号比较（硬过滤）：双方都有续集号且不同 → 拒绝
	sourceSequel := extractSequelNumber(sourceTitle)
	candidateSequel := extractSequelNumber(candidateTitle)
	if sourceSequel > 0 && candidateSequel > 0 && sourceSequel != candidateSequel {
		return false
	}

	if candidateTitle == "" {
		return true
	}
	if len(meaningfulWords) == 0 && len(sourceCJK) == 0 {
		return true
	}
	candLower := strings.ToLower(candidateTitle)
	if len(meaningfulWords) > 0 {
		for _, w := range meaningfulWords {
			if strings.Contains(candLower, w) {
				return true
			}
		}
		return false
	}
	candCJK := extractCJKSubstrings(candidateTitle)
	if len(sourceCJK) > 0 && len(candCJK) > 0 {
		for _, s := range sourceCJK {
			if strings.Contains(candidateTitle, s) {
				return true
			}
		}
		return false
	}
	return true
}

// ExtractMusicSearchKeyword §59.474: 音乐种搜索关键词统一管道 v2。
// 4 步：① 标题主体提取（剥头【】/[..] 连发+剥空回退括号内文+《》内文优先）
// ② 分隔符空格化（- – _ . ．）③ token 清洗（边缘括号剥·年份剥·≤5 词截断）
// 返回 (keyword, isStripYearFallback)。失败返回 ("", false)——调用方
// fallback 影视提取。三站 7417 颗全量验证+60 例皇后真实搜索 100% 召回。
func ExtractMusicSearchKeyword(name string, stripYear bool) string {
	s := strings.TrimSpace(name)
	// ① 剥头【】/[..]（连发）
	changed := true
	for changed {
		changed = false
		if strings.HasPrefix(s, "【") {
			if i := strings.Index(s, "】"); i >= 0 {
				s = s[i+len("】"):]
				changed = true
			}
		}
		if strings.HasPrefix(s, "[") {
			if i := strings.Index(s, "]"); i >= 0 {
				s = s[i+1:]
				changed = true
			}
		}
		s = strings.TrimSpace(s)
	}
	// 剥头后全空 → 回退首个括号内文
	if s == "" {
		m := regexp.MustCompile(`^[\[【]([^\]】]+)[\]】]`).FindStringSubmatch(strings.TrimSpace(name))
		if m != nil {
			s = m[1]
		}
	}
	// 《》内文优先
	if m := regexp.MustCompile(`《([^》]+)》`).FindStringSubmatch(s); m != nil {
		s = m[1]
	}
	// ② 分隔符空格化（§59.477 A类: & 入分隔符集——皇后带 & 必零结果·
	// "Iggy & the"→"Iggy the"·"Nos.20&21"→"Nos 20 21"·对齐 §59.192 影视语义）
	s = regexp.MustCompile(`[-–_\.．&]+`).ReplaceAllString(s, " ")
	// ③ token 清洗
	var out []string
	for _, t := range strings.Fields(s) {
		t = strings.Trim(t, "()[]{}（）【】,，")
		if t == "" {
			continue
		}
		if stripYear && regexp.MustCompile(`^(19|20)\d{2}$`).MatchString(t) {
			continue
		}
		// §59.477 B类: Scene 形态清洗——目录号（≥2 大写字母+≥2 数字·
		// KSR043/LAB070）与规格词（16BIT/\d+BIT·CDEP·EP·WEB·Single·
		// 音频格式）非标题主体（主体=Artist+Track）。Scene 命名序
		// [Artist]-[Track]-[(目录号)]-[规格]-[年份]-[组]：**首见即截断**
		// （后续 token 全是元数据·保留下游 AND 零结果风险——皇后站方
		// 改 canonical 名即零命中）；canonical 名无此形态零影响
		// （用户七案实测：Sydro/Sambassadeur 案）。
		if musicCatalogNoRe.MatchString(t) || musicSpecTokenRe.MatchString(t) {
			break
		}
		// §59.480: 艺人缩写剥除（strip-continue·非 Scene 截断）——
		// "VA"=Various Artists 缩写·皇后 canonical 全称形态 AND 必零
		// （Ultimate Box Set 案用户实测：带 VA 零结果·剥除后命中）。
		// 剥除不截断（VA 常在首位——截断会摧毁关键词主体）。
		if musicAbbrevTokenRe.MatchString(t) {
			continue
		}
		// §59.478: 字母数字粘连拆分（§59.206 视频语义继承·音乐漏继承第
		// 三族）——"60SACD"→["60","SACD"]：皇后标题 "60 SACD"（空格形态）
		// 对粘连 token 的 AND Contains 必 miss（RCA 案实证）。**仅纯 ASCII
		// 字母数字 token**（含 CJK/全角字符的整体 token 不拆——日文标题
		// 「アマガミSS」案：拆分组提取会摧毁 CJK 主体只留 ASCII 碎片）。
		// 拆后子词续走目录号/规格词清洗。
		if musicPureAlnumRe.MatchString(t) {
			if subs := musicGluedSplitRe.FindAllString(t, -1); len(subs) > 1 {
				for _, st := range subs {
					if st == "" || musicCatalogNoRe.MatchString(st) || musicSpecTokenRe.MatchString(st) {
						continue
					}
					out = append(out, st)
				}
				continue
			}
		}
		out = append(out, t)
	}
	return strings.Join(out[:min(5, len(out))], " ")
}

// musicPureAlnumRe §59.478: 纯 ASCII 字母数字 token 判定（粘连拆分的前置门）。
var musicPureAlnumRe = regexp.MustCompile(`^[A-Za-z0-9]+$`)

// musicAbbrevTokenRe §59.480: 艺人缩写词（strip-continue 语义）——
// VA（Various Artists·大小写不敏感精确词）。与 spec 词的 break 语义
// 相反：VA 常居词首·截断会摧毁主体。
var musicAbbrevTokenRe = regexp.MustCompile(`(?i)^va$`)

// musicGluedSplitRe §59.478: 字母/数字边界切分（RE2 无 lookaround·
// FindAllString 分组提取两形态段）。
var musicGluedSplitRe = regexp.MustCompile(`[A-Za-z]+|\d+`)

// musicCatalogNoRe §59.477: Scene 目录号——全大写字母 2-6 + 数字 2-6
// （KSR043/LAB070/PHCP2035 形态族）。标题主体不含此形态。
var musicCatalogNoRe = regexp.MustCompile(`^[A-Z]{2,6}\d{2,6}$`)

// musicSpecTokenRe §59.477/§59.478/§59.487: 规格/载体词——\d+BIT（16BIT/24BIT）·
// CDEP/EP/WEB/Single·音频格式（FLAC/WAV/APE/ALAC/MP3/M4A——皇后标题
// 恒带 FLAC 非判别词·剥除=关键词更宽=召回更稳·验证层过滤）。
// §59.487 用户实测定案：SACD/ISO 移出——站内标题不恒带（少数合集带）
// = 判别词·保留有筛价值（"SACD&LP 合集"案：皇后搜 "SACD LP" 直接命中
// 站方标题形态；首 token spec 触发 break 摧毁关键词的案外根治）。
// RCA 案回归推演："60SACD"→[60,SACD] 拆分后两词保留——皇后站方标题
// "60 SACD"（空格形态）AND Contains 命中·不回归（更精准）。
var musicSpecTokenRe = regexp.MustCompile(`(?i)^\d+bit$|^(?:cdep|ep|web|single|flac|wav|ape|alac|mp3|m4a)$`)
