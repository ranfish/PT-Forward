package titleparser

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	// §59.98: 范围/复合形态——S01-S03(跨季)/S01E03-E05/S01+S02 组合
	reSeasonEpisode = regexp.MustCompile(`(?i)\bS\d{1,2}(?:E\d{1,3}(?:[-~]E?\d{1,3})?)?(?:[-+]S\d{1,2}(?:E\d{1,3})?)*\b`)
	reYearToken     = regexp.MustCompile(`[\s.(]((?:19|20)\d{2})([\s.)]|$)`)
	reResolutionTok = regexp.MustCompile(`(?i)\b(4320p|8k|2160p|4k|1440p|1080p|1080i|720p|480p)\b`)
	reBDRipToken    = regexp.MustCompile(`(?i)\bBD[-\s]?RIP\b`)
	reTVRipToken    = regexp.MustCompile(`(?i)\bTV[-\s]?RIP\b`)
	reDVDRipToken   = regexp.MustCompile(`(?i)\bDVD[-\s]?RIP\b`)
	reDVDDiscToken  = regexp.MustCompile(`(?i)\bDVD(?:5|9)\b`)
	reHDRTitleToken = regexp.MustCompile(`(?i)Dolby Vision|DoVi|HDR10\+|HDRVivid|HDR10|HLG|HDR|SDR|EDR|DV|Vivid`)
	reAudioDTSHDMA  = regexp.MustCompile(`(?i)\bDTS[-._\s]?HD[-._\s]*MA\b`)
	reAudioCodecDD  = regexp.MustCompile(`(?i)\bDD\b`)
	reChinesePrefix = regexp.MustCompile(`^\s*\[([^\]]+)\]\s*`)
// §59.226 附十四: 视频位深合法域仅 8|10|12（16/24 是音频位深——FLAC 24BIT
// 音乐标题曾污染视频位深字段）；捕获数字归一 "Nbit"（"10 BIT"→"10bit"）。
reBitDepth      = regexp.MustCompile(`(?i)\b(8|10|12)[._\-\s]?BIT\b`)
	reFrameRate     = regexp.MustCompile(`(?i)\b(\d{2,3}(?:\.\d+)?)\s*FPS\b`)
	reVideoCodecToken = regexp.MustCompile(`(?i)\b(AV1|VP[89]|AVS2|X265|H\.?265|HEVC|X264|H\.?264|AVC|VC-?1|MPEG-?2)\b`)
	reAudioCodecToken = regexp.MustCompile(`(?i)\b(TrueHD|True[-.\s]*HD|DTS[-.\s]*HD[-.\s]*(?:MA|HR)|DTS:X|DTS|E[-]?AC[-]?3|DDPA|DDP|DD\+|AC[-]?3|DD|FLAC|ALAC|AAC|APE|WAV|OPUS|MP3|LPCM|PCM)\d*(?:\.\d+)?`)
	reAudioTracksCleanup = regexp.MustCompile(`(?i)\b\d+\s*Audios?\b`)
	reSiteTagSuffix = regexp.MustCompile(`(?:\s*(?:\[[^\]]*\]|\([^)]*\)))+\s*$`)
	reHDRToken = regexp.MustCompile(`(?i)\b(Dolby[-\s]?Vision|DoVi|DV|HDR[-\s]?Vivid|HDRVivid|HDR10[-\s]?Plus|HDR10\+|HDR10|PQ10|HLG|HDR|SDR)\b`)
	reChinesePrefixNoBracket = regexp.MustCompile(`^([\p{Han}][\p{Han}0-9：，、:（）\(\)]*)\.`)
)

// ParseTitle 解析标题，返回结构化组件
func ParseTitle(title string) TitleComponents {
	c := TitleComponents{}
	if title = strings.TrimSpace(title); title == "" {
		return c
	}

	// 剥离文件扩展名（部分站点的种子标题用文件名格式）
	title = stripFileExtension(title)

	// 归一化 non-breaking space（源站标题可能含 U+00A0，用 Unicode 级替换避免破坏中文）
	title = strings.ReplaceAll(title, "\u00a0", " ")

	// 移除末尾站点标签（[热门] [2X免费] [50%] (已审) 等标签后缀）
	title = reSiteTagSuffix.ReplaceAllString(title, "")
	title = strings.TrimSpace(title)

	// 剥离 [中文名] 前缀
	c.ChinesePrefix, title = extractChinesePrefix(title)

	// §59.35: WEB 上下文在 token 消费开始前判定（platform 提取点位于 medium/audio
	// 移除之后，彼时 WEB-DL/HDTV token 已被剥除，上下文会丢失）
	webCtx := hasWebContext(title)
	mainLocked := "" // §59.97: 年份锚定的主标题（空=未锁定，走逐词 fallback）

	// §59.98: 统一边界锚——「季集或年份」最先出现者为主标题右边界（v1.05 顺序
	// 剧名→季集→年份; 无年份剧集由季集锚获结构边界, Saki 案例不再依赖逐词 fallback）
	preBoundary := title
	c.SeasonEpisode, c.Year, title, mainLocked = extractBoundaryAnchor(title)
	// 发布版本
	// §59.203: 版本词在年份前形态（To.Live.REPACK.1994...）——边界锚把年份前
	// 内容锁入主标题并从工作标题剥除，REPACK 对 extractReleaseVersion 不可见
	//（活着 tid=295484 错配注入案：规则 B 版本反驳未触发）。改用锚前全文提取。
	c.ReleaseVersion = extractReleaseVersion(preBoundary)
	title = removeToken(title, c.ReleaseVersion)
	// 剧集状态
	c.SeriesStatus = extractSeriesStatus(title)
	title = removeToken(title, c.SeriesStatus)
	// 分辨率
	c.Resolution = extractResolution(title)
	title = removeToken(title, c.Resolution)
	// 媒介
	c.Medium = extractMedium(title)
	title = removeMediumTokens(title, c.Medium)
	// HDR
	c.HDRFormat = extractHDRFormat(title)
	title = removeHDRTokens(title)
	// 视频编码
	c.VideoCodec = extractVideoCodec(title)
	title = removeVideoCodecTokens(title)
	// 音频编码
	c.AudioCodec = extractAudio(title)
	title = removeAudioCodecTokens(title)
	// 色深
	c.BitDepth = extractBitDepth(title)
	title = removeToken(title, c.BitDepth)
	// 帧率
	c.FrameRate = extractFrameRate(title)
	title = removeToken(title, c.FrameRate)
	// 片源平台
	// §59.35: webCtx 来自解析起点（见函数开头），消费后标题已无 WEB token
	c.SourcePlatform = extractSourcePlatformWithContext(title, webCtx)
	title = removeToken(title, c.SourcePlatform)
	// 地区码 §59.97（v1.05 #6——年份后技术区扫; platform 词表优先消歧 HKG）
	c.RegionCode, title = extractRegionCodeAndRemove(title)
	// 移除音轨数 token（2Audios/3Audios 等）
	title = reAudioTracksCleanup.ReplaceAllString(title, " ")
	title = strings.TrimSpace(title)
	// 制作组
	c.ReleaseGroup = extractGroup(title)
	title = removeGroupSuffix(title, c.ReleaseGroup)

	// 剩余部分 = 无法识别（§59.97 定案: 年份右侧技术区残余不回填主标题——
	// 边界左侧全是片名、右侧全是技术词，无词性猜测；HKG/ITA 走 RegionCode extractor）
	if mainLocked != "" {
		c.MainTitle = mainLocked
		_, c.Unrecognized = extractMainAndUnrecognized(title)
	} else {
		c.MainTitle, c.Unrecognized = extractMainAndUnrecognized(title)
	}

	return c
}

func extractChinesePrefix(title string) (prefix, remaining string) {
	// 1. [中文] 中括号格式
	m := reChinesePrefix.FindStringSubmatch(title)
	if m != nil {
		return m[1], strings.TrimSpace(title[len(m[0]):])
	}
	// 2. 无中括号格式：中文片名.英文片名.技术信息（CSWEB/CMCT 格式）
	// 正则匹配开头的连续中文段（含数字/中文标点），以 . 分隔
	m = reChinesePrefixNoBracket.FindStringSubmatch(title)
	if m != nil {
		candidate := m[1]
		// 至少 2 个中文字符才提取（避免误匹配单个中文标点）
		hanCount := 0
		for _, r := range candidate {
			if r >= 0x4e00 && r <= 0x9fff {
				hanCount++
			}
		}
		if hanCount >= 2 {
			return candidate, strings.TrimSpace(title[len(m[0]):])
		}
	}
	return "", title
}

// reCNEpisodeRange §59.226 附三: 中文集数范围——"第01-24集"/"第1~12集"。
// 结构词"第…集"自锚定（电影标题不出现此形态——无歧义，不开 PTGen 门）。
var reCNEpisodeRange = regexp.MustCompile(`第\s*(\d{1,4})\s*[-~－至]\s*(\d{1,4})\s*集`)

// reCNEpisodeCount §59.226 附三: 中文全集数——"全24集"/"全五集"（中文数字
// §59.204 F1 同族词表：零一二三四五六七八九十百千）。
var reCNEpisodeCount = regexp.MustCompile(`全\s*([0-9]{1,4}|[零一二三四五六七八九十百千])\s*集`)

// cnNumToInt §59.226 附三: 中文数字→阿拉伯（一~十/百内简单组合）。
func cnNumToInt(cn string) int {
	if n := len(cn); n == 1 {
		singles := map[rune]int{'零': 0, '一': 1, '二': 2, '三': 3, '四': 4,
			'五': 5, '六': 6, '七': 7, '八': 8, '九': 9}
		if v, ok := singles[rune(cn[0])]; ok {
			return v
		}
		if cn == "十" {
			return 10
		}
	} else if n >= 2 {
		// 十位组合：十五/二十/二十五
		tenIdx := strings.Index(cn, "十")
		if tenIdx >= 0 {
			head, tail := 1, 0
			singles := map[rune]int{'一': 1, '二': 2, '三': 3, '四': 4,
				'五': 5, '六': 6, '七': 7, '八': 8, '九': 9}
			if tenIdx > 0 {
				if v, ok := singles[rune(cn[0])]; ok {
					head = v
				}
			}
			if tenIdx+len("十") < len(cn) {
				if v, ok := singles[rune(cn[tenIdx+len("十")])]; ok {
					tail = v
				}
			}
			return head*10 + tail
		}
	}
	return 0
}

// extractCNEpisodeAndRemove 中文集数提取归一（E01-E24 形态）。
// §59.226 附三: "第01-24集"→"E01-E24"；"全24集"→"E01-E24"（起点补 01）。
// 无匹配返回空。调用点在 reSeasonEpisode 未命中时兜底（S/E 形态优先）。
func extractCNEpisodeAndRemove(title string) (value, remaining string) {
	if m := reCNEpisodeRange.FindStringSubmatch(title); m != nil {
		start, end := padE(m[1]), padE(m[2])
		remaining := strings.TrimSpace(reCNEpisodeRange.ReplaceAllString(title, " "))
		return "E" + start + "-E" + end, remaining
	}
	if m := reCNEpisodeCount.FindStringSubmatch(title); m != nil {
		num := m[1]
		if !regexp.MustCompile(`^[0-9]+$`).MatchString(num) {
			num = fmt.Sprint(cnNumToInt(num))
			if num == "0" {
				num = "1"
			}
		}
		remaining := strings.TrimSpace(reCNEpisodeCount.ReplaceAllString(title, " "))
		return "E01-E" + padE(num), remaining
	}
	return "", title
}

// padE 集数补零到 2 位（"1"→"01"；"124"→"124"）。
func padE(num string) string {
	for len(num) < 2 {
		num = "0" + num
	}
	return num
}

func extractSeasonEpisodeAndRemove(title string) (value, remaining string) {
	match := reSeasonEpisode.FindString(title)
	if match == "" {
		// §59.226 附三: S/E 形态未命中 → 中文集数兜底（第N集/全N集——
		// 结构词自锚定）
		return extractCNEpisodeAndRemove(title)
	}
	remaining = strings.TrimSpace(reSeasonEpisode.ReplaceAllString(title, " "))
	remaining = strings.TrimSpace(regexp.MustCompile(`\s+`).ReplaceAllString(remaining, " "))
	return match, remaining
}

// extractBoundaryAnchor §59.98: 统一边界锚。
// 锚 = 第一个「季集 token」或「后随技术词的年份 token」（位置最先者）;
// 双年份取最后技术跟随者（§59.97《2046》）。
// 返回 (seasonEpisode, year, remaining, mainLocked)。
func extractBoundaryAnchor(title string) (se, year, remaining, mainLocked string) {
	// 季集 token 位置
	seLoc := reSeasonEpisode.FindStringIndex(title)
	// §59.226 附三: 中文集数形态（"第01-24集"/"全五集"）同季集锚——
	// 结构词自锚定，锚在年份前也不锁进主标题（"大江大河.第01-24集.2024"）
	if seLoc == nil {
		if cnLoc := reCNEpisodeRange.FindStringIndex(title); cnLoc != nil {
			seLoc = cnLoc
		} else if cnLoc2 := reCNEpisodeCount.FindStringIndex(title); cnLoc2 != nil {
			seLoc = cnLoc2
		}
	}
	// 年份锚（技术跟随）
	var yearStart, yearEnd int
	matches := reYearToken.FindAllStringSubmatchIndex(title, -1)
	anchorVal := ""
	for _, m := range matches {
		if techFollowRe.MatchString(title[m[3]:]) {
			yearStart, yearEnd, anchorVal = m[2], m[3], title[m[2]:m[3]]
		}
	}
	takeMain := func(cut int) string {
		m := strings.TrimSpace(title[:cut])
		m = strings.NewReplacer(".", " ", "_", " ").Replace(m)
		m = strings.TrimSpace(regexp.MustCompile(`\s+`).ReplaceAllString(m, " "))
		return strings.Trim(m, "- ")
	}
	// 季集锚无条件优先（§59.98: 季集 token 是比年份更强的边界——年份可倒序
	// 2024.S01E03 / 可在片名 2046, 季集永远紧邻剧名右侧）。年份可在季集前后——
	// 有技术跟随的年份即真年份, 两侧剥除防污染
	if seLoc != nil {
		se = title[seLoc[0]:seLoc[1]]
		// §59.226 附三: 中文集数锚归一（"第01-24集"→"E01-E24"）
		if m := reCNEpisodeRange.FindStringSubmatch(se); m != nil {
			se = "E" + padE(m[1]) + "-E" + padE(m[2])
		} else if m2 := reCNEpisodeCount.FindStringSubmatch(se); m2 != nil {
			num := m2[1]
			if !regexp.MustCompile(`^[0-9]+$`).MatchString(num) {
				num = fmt.Sprint(cnNumToInt(num)) // 中文数字转阿拉伯
				if num == "0" {
					num = "1"
				}
			}
			se = "E01-E" + padE(num)
		}
		mainLocked = takeMain(seLoc[0])
		remaining = strings.TrimSpace(title[seLoc[1]:])
		if anchorVal != "" {
			year = anchorVal
			// 右侧: 从 remaining 剥
			remaining = strings.Replace(remaining, anchorVal, "", 1)
			remaining = strings.TrimSpace(regexp.MustCompile(`[\s.]+`).ReplaceAllString(remaining, " "))
			// 左侧: mainLocked 里的年份词剔除（"Test Show 2024"→"Test Show"）
			mmL := strings.NewReplacer(".", " ").Replace(mainLocked)
			if strings.Contains(mmL, anchorVal) {
				reY := regexp.MustCompile(`(?i)\b` + anchorVal + `\b`)
				mainLocked = strings.TrimSpace(reY.ReplaceAllString(mmL, ""))
			}
		}
		if remaining == "" {
			remaining = title
		}
		return se, year, remaining, mainLocked
	}
	// 年份锚（§59.97 原逻辑）
	if anchorVal != "" {
		mainLocked = takeMain(yearStart)
		remaining = strings.TrimSpace(title[yearEnd:])
		if remaining == "" {
			remaining = title
		}
		return "", anchorVal, remaining, mainLocked
	}
	// 无锚: 季集无/年份无技术跟随——季集仍可作弱锚（Saki 无年份场景已在上分支）
	// 完全无锚走 fallback
	return "", "", title, ""
}

// techFollowRe §59.97: 年份 token 后随技术词（判定真年份锚——片名含年份数字
// 但后无技术词的场景如 "Blade Runner 2049 Alone" 不截断）。
var techFollowRe = regexp.MustCompile(`(?i)(1080|2160|720|480|4320|bluray|blu-?ray|uhd|web|hdvd|remux|x26[45]|h\.?26[45]|hevc|avc|xvid|dvd)`)

// extractYearAnchorMain §59.97: 年份锚定主标题——返回 (year, remaining, mainLocked)。
// 规则（用户定案）:
//   - 全部独立年份 token；无 → ("", title, "")
//   - 取最后一个"后随技术 token"的年份为真年份（2046.2004 → 2004）
//   - 主标题 = 首个年份 token 之前的部分（保住以数字为片名的《2046》）
//   - remaining = 真年份之后的标题（技术区，后续 extractor 消费）
func extractYearAnchorMain(title string) (year, remaining, mainLocked string) {
	matches := reYearToken.FindAllStringSubmatchIndex(title, -1)
	if len(matches) == 0 {
		return "", title, ""
	}
	type yTok struct{ val string; start, end int; hasTechAfter bool }
	var toks []yTok
	for _, m := range matches {
		val := title[m[2]:m[3]]
		after := title[m[3]:]
		hasTech := techFollowRe.MatchString(after)
		toks = append(toks, yTok{val, m[2], m[3], hasTech})
	}
	// 真年份 = 最后一个后随技术词的年份 token
	anchor := -1
	for i := len(toks) - 1; i >= 0; i-- {
		if toks[i].hasTechAfter {
			anchor = i
			break
		}
	}
	if anchor < 0 {
		return "", title, "" // 无锚（片名含年份无技术词）→ 不截断
	}
	year = toks[anchor].val
	// 主标题 = 首个年份 token 前（片名以数字开头时首个即片名一部分，保留）
	// §59.97 定案: 主标题 = 锚年份左侧全部（含首个年份 token 2046——片名数字;
	// 边界左侧不猜词性, 全是片名成分）
	mainLocked = strings.TrimSpace(title[:toks[anchor].start])
	mainLocked = strings.NewReplacer(".", " ", "_", " ").Replace(mainLocked)
	mainLocked = strings.TrimSpace(regexp.MustCompile(`\s+`).ReplaceAllString(mainLocked, " "))
	mainLocked = strings.Trim(mainLocked, "- ")
	// remaining = 锚年份后（保留后续技术区；锚与首个 token 之间的年份词也归主标题已处理——
	// 双年份场景中间内容(即无)忽略）
	remaining = strings.TrimSpace(title[toks[anchor].end:])
	if remaining == "" {
		remaining = title
	}
	return year, remaining, mainLocked
}

func extractYearAndRemove(title string) (value, remaining string) {
	match := reYearToken.FindStringSubmatch(title)
	if match == nil {
		return "", title
	}
	reRemove := regexp.MustCompile(`[\s.(]` + regexp.QuoteMeta(match[1]) + `([\s.)]|$)`)
	remaining = strings.TrimSpace(reRemove.ReplaceAllString(title, " "))
	remaining = strings.TrimSpace(regexp.MustCompile(`\s+`).ReplaceAllString(remaining, " "))
	return match[1], remaining
}

// regionCodeSet §59.97: v1.05 地区码词表（qingwapt W 章 + ISO 常见）。
var regionCodeSet = map[string]bool{
	"ITA": true, "USA": true, "JPN": true, "HKG": true, "TWN": true, "KOR": true,
	"GBR": true, "UK": true, "FRA": true, "GER": true, "DEU": true, "CAN": true,
	"AUS": true, "ESP": true, "NLD": true, "SWE": true, "NOR": true, "DEN": true,
	"FIN": true, "RUS": true, "CHN": true, "IND": true, "THA": true, "BRA": true,
	"MEX": true, "ARG": true, "NZL": true,
}

// extractRegionCodeAndRemove 年份后技术区的地区码提取（点/空格分隔均处理——
// Fields 按空格, 点分隔需先归一）。返回 (code, remaining)。
func extractRegionCodeAndRemove(title string) (string, string) {
	normalized := strings.NewReplacer(".", " ", "_", " ").Replace(title)
	for _, f := range strings.Fields(normalized) {
		up := strings.ToUpper(f)
		if len(up) == 3 && regionCodeSet[up] {
			re := regexp.MustCompile(`(?i)(^|[\s.-])` + up + `([\s.-]|$)`)
			remaining := re.ReplaceAllString(title, "$1")
			remaining = strings.Trim(strings.TrimSpace(regexp.MustCompile(`\s+`).ReplaceAllString(remaining, " ")), " .-")
			return up, remaining
		}
	}
	return "", title
}

// reEdition4K §59.190 挂账兑现/§59.197 附: "4K修复版"/"4K.REMASTER" 版式短语
// ——4K 是修复源描述（4K 扫描母版修复），非编码分辨率。此语境下无像素形态
// token 时 Resolution 置空（编码分辨率未标），交 size 仲裁版本（午夜凶铃案：
// Res="4K"×候选 1080p 假冲突六站候选全灭）。裸 "4K"（无版式语境）维持原值。
var reEdition4K = regexp.MustCompile(`(?i)(?:4k|8k)[-_\.\s]*(?:修复版?|修復版?|重制版?|重製版?|数字修复|數字修復|remaster(?:ed)?)`)

func extractResolution(title string) string {
	// §59.196: 显式像素形态优先——"4K.REMASTER...1080p" 形态中裸 4K 是版式/片源
	// 描述（4K 修复版源），真实编码分辨率是其后的 1080p；首匹配取 4K 曾致
	// Resolution 假冲突（侠女 tid=288084 主轮误拒）。仅含 4K/8K 时维持原值。
	all := reResolutionTok.FindAllString(title, -1)
	for _, tok := range all {
		if isPixelFormResolution(tok) {
			return strings.TrimSpace(tok)
		}
	}
	if len(all) > 0 {
		pick := strings.TrimSpace(all[0])
		lt := strings.ToLower(pick)
		if (lt == "4k" || lt == "8k") && reEdition4K.MatchString(title) {
			return ""
		}
		return pick
	}
	return ""
}

// isPixelFormResolution §59.196: 是否显式像素形态（4320p/2160p/1440p/1080p/
// 1080i/720p/480p）——区别于 4K/8K 营销词形。
func isPixelFormResolution(tok string) bool {
	t := strings.ToLower(strings.TrimSpace(tok))
	if len(t) < 3 {
		return false
	}
	suffix := t[len(t)-1:]
	if suffix != "p" && suffix != "i" {
		return false
	}
	for _, r := range t[:len(t)-1] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func extractMedium(title string) string {
	upper := strings.ToUpper(title)
	parts := make([]string, 0, 6)

	if regexp.MustCompile(`(?i)\bUHDTV\b`).MatchString(title) {
		parts = append(parts, "UHDTV")
	}
	if regexp.MustCompile(`(?i)\bHDTV\b`).MatchString(title) {
		parts = append(parts, "HDTV")
	}
	blurayToken := preferredBlurayToken(title)
	// 独立 UHD 修饰（如 "2160p UHD"）：UHD BluRay 组合已含 UHD 语义，不再重复追加
	if strings.Contains(upper, "UHD") && !strings.HasPrefix(blurayToken, "UHD") {
		parts = append(parts, "UHD")
	}

	if blurayToken != "" {
		if hasMediaDIY(title) {
			parts = append(parts, blurayToken+" DIY")
		} else {
			parts = append(parts, blurayToken)
		}
	}

	if strings.Contains(upper, "REMUX") {
		parts = append(parts, "Remux")
	}
	if reBDRipToken.MatchString(title) {
		parts = append(parts, "BDRip")
	}
	// §59.84: HDDVDRip（v1.05 压制类明列，extractMedium 曾无 case 双空）
	if regexp.MustCompile(`(?i)\bHDDVDRIP\b`).MatchString(title) {
		parts = append(parts, "HDDVDRip")
	}
	if raw := strings.TrimSpace(reTVRipToken.FindString(title)); raw != "" {
		parts = append(parts, "TVRip")
	}
	if raw := strings.TrimSpace(reDVDRipToken.FindString(title)); raw != "" {
		parts = append(parts, "DVDRip")
	}
	if raw := strings.TrimSpace(reDVDDiscToken.FindString(title)); raw != "" {
		parts = append(parts, strings.ToUpper(raw))
	} else if regexp.MustCompile(`(?i)\bDVD\b`).MatchString(title) && !strings.Contains(upper, "DVDRIP") {
		// §59.84: 裸 DVD（原盘类）——reDVDDiscToken 只认 DVD5/9, "DVD.Full" 曾双空
		parts = append(parts, "DVD")
	}
	if strings.Contains(upper, "WEB-DL") || strings.Contains(upper, "WEBDL") {
		parts = append(parts, "WEB-DL")
	} else if strings.Contains(upper, "WEBRIP") {
		parts = append(parts, "WEBRip")
	}
	return strings.Join(parts, " ")
}

// preferredBlurayToken 蓝光片源 token（v1.05 原盘/压制以连字符区分写法）。
//
// 原盘：Blu-ray / UHD Blu-ray / 3D Blu-ray（带连字符或点分隔的 "RAY"）
// 压制：BluRay / UHD BluRay / 3D BluRay（无连字符）
func preferredBlurayToken(title string) string {
	upper := strings.ToUpper(title)
	// 带分隔的 RAY → 原盘写法；其余（BLURAY/UHDBluRay）→ 压制写法
	suffix := "BluRay"
	if strings.Contains(upper, "BLU-RAY") || strings.Contains(upper, "BLU.RAY") {
		suffix = "Blu-ray"
	}
	switch {
	case strings.Contains(upper, "UHD BLU") || strings.Contains(upper, "UHDBLU"):
		return "UHD " + suffix
	// §59.84: 3D 分隔三形态（空格/点/无分隔）——点分隔 "3D.Blu-ray" 曾落入普通 BLU
	case strings.Contains(upper, "3D BLU") || strings.Contains(upper, "3D.BLU") || strings.Contains(upper, "3DBLU"):
		return "3D " + suffix
	case strings.Contains(upper, "BLU"):
		return suffix
	}
	return ""
}

// extractVideoCodec 从标题提取视频编码（§59.27: token registry 派生）。
func extractVideoCodec(title string) string {
	return lookupToken(videoCodecRegistry, title)
}

func extractHDRFormat(title string) string {
	matches := findHDRTokens(title)
	if len(matches) == 0 {
		return ""
	}
	hasDoVi, hasHDR10Plus, hasHDR10, hasHDR, hasHLG, hasVivid, hasSDR := false, false, false, false, false, false, false
	for _, item := range matches {
		switch strings.ToUpper(strings.TrimSpace(item)) {
		case "DOLBY VISION", "DOVI", "DV":
			hasDoVi = true
		case "HDR10+":
			hasHDR10Plus = true
		case "HDR10":
			hasHDR10 = true
		case "HLG":
			hasHLG = true
		case "HDRVIVID":
			hasVivid = true
		case "HDR":
			hasHDR = true
		case "SDR":
			hasSDR = true
		}
	}
	if hasDoVi && hasHDR10Plus {
		return "DoVi HDR10+"
	}
	if hasDoVi && hasHDR10 {
		// §59.226 附十三: "DoVi HDR10" 是词表外冗余形态（DoVi 组合 HDR10 即
		// "DoVi HDR"——"10" 冗余；对齐 MI 侧 hdrFromMI/词表/§59.151）
		return "DoVi HDR"
	}
	if hasDoVi && hasHDR {
		return "DoVi HDR"
	}
	if hasDoVi {
		return "DoVi"
	}
	if hasHDR10Plus {
		return "HDR10+"
	}
	if hasHDR10 {
		return "HDR10"
	}
	if hasVivid {
		return "HDR Vivid"
	}
	if hasHDR {
		return "HDR"
	}
	if hasHLG {
		return "HLG"
	}
	if hasSDR {
		return "SDR"
	}
	return ""
}

func findHDRTokens(title string) []string {
	return reHDRTitleToken.FindAllString(title, -1)
}

// extractAudio 从标题提取音频编码（§59.27: token registry 派生）。
func extractAudio(title string) string {
	return lookupToken(audioCodecRegistry, title)
}

func extractBitDepth(title string) string {
	m := reBitDepth.FindStringSubmatch(title)
	if m == nil {
		return ""
	}
	return m[1] + "bit"
}

func extractFrameRate(title string) string {
	return strings.TrimSpace(reFrameRate.FindString(title))
}

// reWebContext WEB 上下文判定（§59.35 决策 2）：标题含 WEB/HDTV/UHDTV token
// 时 platform 域 requires=web 词条（2 字符缩写）才启用。
var reWebContext = regexp.MustCompile(`(?i)\bWEB[._\- ]?(?:DL|RIP)?\b|\bU?HDTV\b`)

// hasWebContext 标题是否证实为 WEB 类资源。
func hasWebContext(title string) bool {
	return reWebContext.MatchString(title)
}

// extractSourcePlatform 从标题提取流媒体厂商缩写（§59.35：platform 字典域）。
//
// 词条顺序即优先级（canonical 长度降序，加载器排序）；
// 2 字符缩写挂 requires=web——仅 WEB 上下文启用，误命中方向从
// "剥词污染主标题" 变为 "不提取"（空值无损）。
func extractSourcePlatform(title string) string {
	return extractSourcePlatformWithContext(title, hasWebContext(title))
}

// extractSourcePlatformWithContext 显式传入 webContext（ParseTitle 用原始标题判定，
// 避免前置 token 剥除导致上下文丢失）。
func extractSourcePlatformWithContext(title string, webContext bool) string {
	return lookupTokenWebContext(platformRegistry, title, webContext)
}

func extractSeriesStatus(title string) string {
	upper := strings.ToUpper(title)
	if strings.Contains(upper, "COMPLETE") {
		return "Complete"
	}
	return ""
}

func extractReleaseVersion(title string) string {
	upper := strings.ToUpper(title)
	// §59.32: REMUX 不在此表——v1.05 中 Remux 是规格（specification），
	// 由 extractMedium→splitMedium 产出；此处抢词会提前剥掉 token 导致规格丢失
	for _, kw := range []string{"PROPER", "REPACK", "RERIP", "DIRFIX", "INTERNAL"} {
		if strings.Contains(upper, kw) {
			return kw
		}
	}
	// §59.96: REPACK2/REPACK3 变体——Contains 命中 REPACK 但返回须带数字
	//（removeToken  边界对 REPACK2 需完整 token 才能剥除）
	if m := regexp.MustCompile(`REPACK(\d+)`).FindStringSubmatch(upper); m != nil {
		return m[0]
	}
	return ""
}

// reAtGroupTail §59.226 附八 补: 尾部 @ 组名段（"DIY@CMCT"/"@CMCT"/"SHB931@UBWEB"）。
var reAtGroupTail = regexp.MustCompile(`(?i)(?:^|[\s.])([A-Za-z0-9_]*@[A-Za-z0-9_]+)$`)

// reDIYWord §59.226 附八: 媒介位 DIY 词（Go regexp 无负向前瞻——
// 后继字符检查实现：DIY 后紧跟 @ = 组名署名形态 "-DIY@CMCT" 非媒介词）。
var reDIYWord = regexp.MustCompile(`(?i)\bDIY\b`)

// hasMediaDIY 标题是否存在媒介位 DIY（至少一个 DIY 后不跟 @）。
func hasMediaDIY(title string) bool {
	for _, loc := range reDIYWord.FindAllStringIndex(title, -1) {
		end := loc[1]
		if end >= len(title) || title[end] != '@' {
			return true
		}
	}
	return false
}

func extractGroup(title string) string {
	title = strings.TrimSpace(title)
	// §59.226 附八 补: 无 dash 的 @ 组名段——前置剥除链（音频/分辨率 token
	// 剥除后的清理）可能清掉 dash 残留，输入裸 "DIY@CMCT"/"@CMCT" 形态
	// （Batman 插桩实证）。@ 是无歧义组署名锚。
	if strings.Contains(title, "@") {
		if m := reAtGroupTail.FindStringSubmatch(title); m != nil {
			seg := m[1]
			upper := strings.ToUpper(seg)
			if strings.HasPrefix(upper, "DIY@") && len(seg) > len("DIY@")+1 {
				return seg[len("DIY@"):] // DIY@ 伪复合剥离
			}
			if strings.HasPrefix(seg, "@") && len(seg) > 1 {
				return seg[1:] // 裸 @ 残留（DIY 被前置链剥除）
			}
			return seg // 真复合整段（SHB931@UBits）
		}
	}
	// §59.211 补: ￡ 标记优先于 dash——dash 在 S01-S02/Blu-ray 多连字符名下
	// 产垃圾段（"克兰弗德S01-S02...Blu-ray...￡cXcY@FRDS" 曾提取
	// "2009. . . . ￡cXcY@FRDS"）；￡ 是无歧义组署名锚（SSD 族惯例）。
	if pIdx := strings.LastIndex(title, "￡"); pIdx >= 0 {
		rest := title[pIdx+len("￡"):]
		var b strings.Builder
		for _, r := range rest {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '@' || (r >= '0' && r <= '9') {
				b.WriteRune(r)
			} else {
				break
			}
		}
		if b.Len() >= 2 {
			return b.String()
		}
	}
	idx := strings.LastIndex(title, "-")
	// §59.97: idx>=0——前导连字符残留("-FRDS", token 剥除后)也是组段;
	// 点分隔 ".-FRDS" 一直靠前导点占位侥幸通过, 空格分隔剥后 "-FRDS" 被拒(实锤)
	if idx >= 0 && idx < len(title)-1 {
		raw := strings.TrimSpace(title[idx+1:])
		raw = stripFileExtension(raw)
		upper := strings.ToUpper(raw)
		if upper == "NOGROUP" || upper == "N/A" || upper == "NONE" || upper == "UNKNOWN" {
			// 继续尝试 ￡ 分隔符
		} else if len(raw) >= 2 && len(raw) <= 30 {
			// §59.226 附八: DIY@ 伪复合剥离——DIY 是通用标记非组员名
			// （区别于 SHB931@UBits 真复合整段保留），剥离后仅返回纯组名
			if strings.HasPrefix(upper, "DIY@") && len(raw) > len("DIY@")+1 {
				return raw[len("DIY@"):]
			}
			return raw
		}
	}

	// ￡ 分隔符（SSD 特有格式 ￡CMCT发布者，取连续英文+@子组标记 = 完整组名）
	// §59.179: 保留 @ 子组标记——"￡cXcY@FRDS" 取完整 "cXcY@FRDS" 而非截断到 "cXcY"
	// （修道院 tid=2111 实证：Forrest Gump DTS￡cXcY@FRDS 重组后丢 @FRDS）
	if pIdx := strings.LastIndex(title, "￡"); pIdx >= 0 {
		rest := title[pIdx+len("￡"):]
		var b strings.Builder
		for _, r := range rest {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '@' || (r >= '0' && r <= '9') {
				b.WriteRune(r)
			} else {
				break
			}
		}
		if b.Len() >= 2 {
			return b.String()
		}
	}

	return ""
}

// stripFileExtension 剥离常见视频/容器文件扩展名。
// 部分站点（如憨憨）的种子标题用文件名，导致 .mp4/.mkv 后缀混入 release_group。
func stripFileExtension(s string) string {
	exts := []string{".mp4", ".mkv", ".avi", ".iso", ".m2ts", ".ts", ".mov", ".flv", ".wmv", ".m4v", ".mpg", ".mpeg", ".vob", ".wtv"}
	lower := strings.ToLower(s)
	for _, ext := range exts {
		if strings.HasSuffix(lower, ext) {
			return s[:len(s)-len(ext)]
		}
	}
	return s
}

func extractMainAndUnrecognized(remaining string) (mainTitle, unrecognized string) {
	remaining = strings.TrimSpace(remaining)
	remaining = strings.ReplaceAll(remaining, ".", " ")
	remaining = strings.Trim(remaining, ".- ")
	remaining = regexp.MustCompile(`\s+`).ReplaceAllString(remaining, " ")
	if remaining == "" {
		return "", ""
	}
	parts := strings.Fields(remaining)
	var main, unknown []string
	for _, p := range parts {
		p = strings.Trim(p, ".-")
		if p == "" {
			continue
		}
		if isLikelyTitleWord(p) {
			main = append(main, p)
		} else {
			unknown = append(unknown, p)
		}
	}
	return strings.Join(main, " "), strings.Join(unknown, " ")
}

// techWordRe §59.96: 技术词形态——黑名单词 + 数字后缀变体(REPACK2/PROPER2)
// + Hi10P 类比特标记 + *FiX 修复标记。
var techWordRe = regexp.MustCompile(`(?i)^(UHD|HDR|SDR|HLG|DV|DOVI|ATMOS|HYBRID|REMASTER(?:ED)?|MUHD|MHD|MSD|MNHD|REPACK\d*|PROPER\d*|RERIP|INTERNAL|HI\d+P|\w*FiX)$`)

func isLikelyTitleWord(s string) bool {
	if len(s) <= 1 {
		return false
	}
	// §59.97: 纯数字词不剔（年份已由锚定层剥离——能到 fallback 的数字
	// 都是片名成分: 2049/1917/2001/Se7en 型）
	if regexp.MustCompile(`^\d{3,4}[pi]$`).MatchString(s) {
		return false
	}
	if techWordRe.MatchString(s) {
		return false
	}
	return true
}

func removeToken(title, token string) string {
	if token == "" {
		return title
	}
	// §59.35: platform token 按词条 pattern 移除——
	// 1) 大小写敏感缩写（iT/iP/FUNi）(?i) 会把 "IT" 等普通词剥掉污染主标题
	// 2) 点分隔标题中 \b 对 "Movie.1080p.NF.WEB-DL" 的 NF 边界判定不可靠
	for _, t := range platformRegistry {
		if t.Canonical == token {
			return strings.TrimSpace(t.re().ReplaceAllString(title, " "))
		}
	}
	re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(token) + `\b`)
	return strings.TrimSpace(re.ReplaceAllString(title, " "))
}

// removeVideoCodecTokens 移除标题中所有视频编码变体 token。
// §59.27: 从 videoCodecRegistry 自动合成，与 extractVideoCodec 单一来源。
func removeVideoCodecTokens(title string) string {
	return strings.TrimSpace(removeAllTokenPatterns(videoCodecRegistry, title, false))
}

// removeAudioCodecTokens 移除标题中所有音频编码变体 token。
// §59.27: 从 audioCodecRegistry 自动合成，与 extractAudio 单一来源。
// 正则末尾的 [._\d]*(?:\.[\d]+)? 语义由 registry pattern 内生（如 DDP5.1/AAC2.0）。
func removeAudioCodecTokens(title string) string {
	return strings.TrimSpace(removeAllTokenPatterns(audioCodecRegistry, title, false))
}

// removeHDRTokens 移除标题中所有 HDR/DV 变体 token。
//
// 同 removeVideoCodecTokens 原理：extractHDRFormat 返回标准化名称（如 "HDR Vivid"），
// 但标题原文可能是 "HDRVivid"，removeToken 匹配失败。
func removeHDRTokens(title string) string {
	return strings.TrimSpace(reHDRToken.ReplaceAllString(title, " "))
}

func removeMediumTokens(title, medium string) string {
	if medium == "" {
		return title
	}
	for _, part := range strings.Fields(medium) {
		title = removeToken(title, part)
	}
	// 额外清理常见媒介变体
	for _, kw := range []string{"BluRay", "Blu-ray", "BLURAY", "WEB-DL", "WEBDL", "WEBRip", "WEB", "Remux", "REMUX", "Encode", "HDTV", "UHDTV"} {
		re := regexp.MustCompile(`(?i)\b` + kw + `\b`)
		title = strings.TrimSpace(re.ReplaceAllString(title, " "))
	}
	return title
}

// groupSegmentRe §59.96: 尾部组段形态——"-MNHD-FRDS"/"-VCB-Studio"（1-4 个短大写/小写词链）。
var groupSegmentRe = regexp.MustCompile(`(?i)[-\s]+([A-Za-z][A-Za-z0-9@]{1,30}(?:[-][A-Za-z][A-Za-z0-9@]{1,30})*)\s*$`)

func removeGroupSuffix(title, group string) string {
	if group == "" {
		return title
	}
	// §59.96: 组段整体剥除——尾部连字符词链 "-X(-Y)*" 以 group 结尾时整段移除
	//（MNHD-FRDS: group=FRDS, MNHD 压制线前缀同段; VCB-Studio: group=Studio, VCB 前缀）。
	// 先试整段(更准), 不中再精确后缀。
	m := groupSegmentRe.FindStringSubmatch(title)
	if m != nil {
		seg := m[1]
		segLower := strings.ToLower(seg)
		groupLower := strings.ToLower(group)
		if segLower == groupLower {
			return strings.TrimSpace(title[:len(title)-len(m[0])])
		}
		if strings.HasSuffix(segLower, "@"+groupLower) {
			// §59.226 附八: DIY@ 伪复合剥离后的整段剥除（group=CMCT 时
			// 尾部 "-DIY@CMCT" 段整段移除——防主标题残留污染）
			prefix := segLower[:len(segLower)-len(groupLower)-1]
			if len(prefix) <= 10 && !strings.ContainsAny(prefix, " ") {
				return strings.TrimSpace(title[:len(title)-len(m[0])])
			}
		}
		if strings.HasSuffix(segLower, "-"+groupLower) {
			// 段以 -group 结尾: 前缀词链是组名组成部分(VCB-Studio)或压制线标识(MNHD-FRDS)
			// ——整段剥除。前缀须为短词(≤10)防误吞真标题词(如 "Part-2" 不在尾部场景)
			prefix := segLower[:len(segLower)-len(groupLower)-1]
			if len(prefix) <= 10 && !strings.ContainsAny(prefix, " ") {
				return strings.TrimSpace(title[:len(title)-len(m[0])])
			}
		}
	}
	// 精确后缀 fallback
	re := regexp.MustCompile(`(?i)[-.\s]+` + regexp.QuoteMeta(group) + `\s*$`)
	return strings.TrimSpace(re.ReplaceAllString(title, ""))
}
