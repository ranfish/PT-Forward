package titleparser

import (
	"regexp"
	"strings"
)

var (
	reSeasonEpisode = regexp.MustCompile(`(?i)\bS\d{1,2}(?:E\d{1,3}(?:[-~]E?\d{1,3})?)?\b`)
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
	reBitDepth      = regexp.MustCompile(`(?i)\b(8|10|12|16|24)\s*BIT\b`)
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

	// 季集
	c.SeasonEpisode, title = extractSeasonEpisodeAndRemove(title)
	// 年份
	c.Year, title = extractYearAndRemove(title)
	// 发布版本
	c.ReleaseVersion = extractReleaseVersion(title)
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
	// 移除音轨数 token（2Audios/3Audios 等）
	title = reAudioTracksCleanup.ReplaceAllString(title, " ")
	title = strings.TrimSpace(title)
	// 制作组
	c.ReleaseGroup = extractGroup(title)
	title = removeGroupSuffix(title, c.ReleaseGroup)

	// 剩余部分 = 主标题 + 无法识别
	c.MainTitle, c.Unrecognized = extractMainAndUnrecognized(title)

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

func extractSeasonEpisodeAndRemove(title string) (value, remaining string) {
	match := reSeasonEpisode.FindString(title)
	if match == "" {
		return "", title
	}
	remaining = strings.TrimSpace(reSeasonEpisode.ReplaceAllString(title, " "))
	remaining = strings.TrimSpace(regexp.MustCompile(`\s+`).ReplaceAllString(remaining, " "))
	return match, remaining
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

func extractResolution(title string) string {
	return strings.TrimSpace(reResolutionTok.FindString(title))
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
		if regexp.MustCompile(`(?i)\bDIY\b`).MatchString(title) {
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
		return "DoVi HDR10"
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
	return strings.TrimSpace(reBitDepth.FindString(title))
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
	return ""
}

func extractGroup(title string) string {
	title = strings.TrimSpace(title)
	idx := strings.LastIndex(title, "-")
	if idx > 0 && idx < len(title)-1 {
		raw := strings.TrimSpace(title[idx+1:])
		raw = stripFileExtension(raw)
		upper := strings.ToUpper(raw)
		if upper == "NOGROUP" || upper == "N/A" || upper == "NONE" || upper == "UNKNOWN" {
			// 继续尝试 ￡ 分隔符
		} else if len(raw) >= 2 && len(raw) <= 30 {
			return raw
		}
	}

	// ￡ 分隔符（SSD 特有格式 ￡CMCT发布者，取连续英文 = 组名）
	if pIdx := strings.LastIndex(title, "￡"); pIdx >= 0 {
		rest := title[pIdx+len("￡"):]
		var b strings.Builder
		for _, r := range rest {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
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

func isLikelyTitleWord(s string) bool {
	if len(s) <= 1 {
		return false
	}
	if regexp.MustCompile(`^\d+$`).MatchString(s) {
		return false
	}
	if regexp.MustCompile(`^\d{3,4}[pi]$`).MatchString(s) {
		return false
	}
	upper := strings.ToUpper(s)
	switch upper {
	case "UHD", "HDR", "SDR", "HLG", "DV", "DOVI", "ATMOS", "HYBRID", "REMASTER", "REMASTERED",
		"MUHD", "MHD", "MSD":
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

func removeGroupSuffix(title, group string) string {
	if group == "" {
		return title
	}
	// 移除 -group 后缀
	re := regexp.MustCompile(`(?i)[-.\s]+` + regexp.QuoteMeta(group) + `\s*$`)
	return strings.TrimSpace(re.ReplaceAllString(title, ""))
}
