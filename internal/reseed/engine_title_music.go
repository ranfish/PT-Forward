package reseed

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

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
