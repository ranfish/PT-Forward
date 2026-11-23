package reseed

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

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
