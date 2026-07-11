package titleparser

import (
	"regexp"
	"strings"
)

var (
	reSeasonEpisode = regexp.MustCompile(`(?i)\bS\d{1,2}(?:E\d{1,3}(?:[-~]E?\d{1,3})?)?\b`)
	reYearToken     = regexp.MustCompile(`[\s.(]((?:19|20)\d{2})([\s.)]|$)`)
	reResolutionTok = regexp.MustCompile(`(?i)\b(4320p|8k|2160p|4k|1080p|1080i|720p|480p)\b`)
	reBDRipToken    = regexp.MustCompile(`(?i)\bBD[-\s]?RIP\b`)
	reTVRipToken    = regexp.MustCompile(`(?i)\bTV[-\s]?RIP\b`)
	reDVDRipToken   = regexp.MustCompile(`(?i)\bDVD[-\s]?RIP\b`)
	reDVDDiscToken  = regexp.MustCompile(`(?i)\bDVD(?:5|9)\b`)
	reHDRTitleToken = regexp.MustCompile(`(?i)Dolby Vision|DoVi|HDR10\+|HDRVivid|HDR10|HLG|HDR|SDR|EDR|DV|Vivid`)
	reAudioDTSHDMA  = regexp.MustCompile(`(?i)\bDTS[-\s]?HD\s*MA\b`)
	reAudioCodecDD  = regexp.MustCompile(`(?i)\bDD\b`)
	reChinesePrefix = regexp.MustCompile(`^\s*\[([^\]]+)\]\s*`)
	reBitDepth      = regexp.MustCompile(`(?i)\b(8|10|12|16|24)\s*BIT\b`)
	reFrameRate     = regexp.MustCompile(`(?i)\b(\d{2,3}(?:\.\d+)?)\s*FPS\b`)
)

const sourcePlatformAlternatives = `MA|Apple\s?TV\+|ViuTV|MyTVSuper|MyTVS|DNSP|iT|NowE|MyVideo|TWN|LiTV|TVBAnywhere|DMM|iPad|TX|iQIYI|MUBI|TVB|YOUKU|NowPlay|AMZN|Amazon|Netflix|NF|DSNP|MAX|HMAX|HULU|ATVP|iTunes|friDay|USA|EUR|JPN|CEE|FRA|LINETV|PCOK|Hami|GBR|NowPlayer|CR|Crunchyroll|SEEZN|GER|CAN|CHN|Viu|WeTV|meWATCH|CATCHPLAY|AMC\+|TVING|Baha|KKTV|IQ|HKG|ITA|ESP|Disney\+|Disney`

var reSourcePlatformBoundary = regexp.MustCompile("(?i)(?:^|[^\\p{L}\\p{N}_])(" + sourcePlatformAlternatives + ")(?:$|[^\\p{L}\\p{N}_])")

// ParseTitle 解析标题，返回结构化组件
func ParseTitle(title string) TitleComponents {
	c := TitleComponents{}
	if title = strings.TrimSpace(title); title == "" {
		return c
	}

	// 剥离 [中文名] 前缀
	c.ChinesePrefix, title = extractChinesePrefix(title)

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
	title = removeToken(title, c.HDRFormat)
	// 视频编码
	c.VideoCodec = extractVideoCodec(title)
	title = removeToken(title, c.VideoCodec)
	// 视频格式
	c.VideoFormat = extractVideoFormat(title)
	// 音频编码
	c.AudioCodec = extractAudio(title)
	title = removeToken(title, c.AudioCodec)
	// 色深
	c.BitDepth = extractBitDepth(title)
	title = removeToken(title, c.BitDepth)
	// 帧率
	c.FrameRate = extractFrameRate(title)
	title = removeToken(title, c.FrameRate)
	// 片源平台
	c.SourcePlatform = extractSourcePlatform(title)
	title = removeToken(title, c.SourcePlatform)
	// 制作组
	c.ReleaseGroup = extractGroup(title)
	title = removeGroupSuffix(title, c.ReleaseGroup)

	// 剩余部分 = 主标题 + 无法识别
	c.MainTitle, c.Unrecognized = extractMainAndUnrecognized(title)

	return c
}

func extractChinesePrefix(title string) (prefix, remaining string) {
	m := reChinesePrefix.FindStringSubmatch(title)
	if m != nil {
		return m[1], strings.TrimSpace(title[len(m[0]):])
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
	if strings.Contains(upper, "UHD") {
		parts = append(parts, "UHD")
	}

	if blurayToken := preferredBlurayToken(title); blurayToken != "" {
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
	if raw := strings.TrimSpace(reTVRipToken.FindString(title)); raw != "" {
		parts = append(parts, "TVRip")
	}
	if raw := strings.TrimSpace(reDVDRipToken.FindString(title)); raw != "" {
		parts = append(parts, "DVDRip")
	}
	if raw := strings.TrimSpace(reDVDDiscToken.FindString(title)); raw != "" {
		parts = append(parts, strings.ToUpper(raw))
	}
	if strings.Contains(upper, "WEB-DL") || strings.Contains(upper, "WEBDL") {
		parts = append(parts, "WEB-DL")
	} else if strings.Contains(upper, "WEBRIP") {
		parts = append(parts, "WEBRip")
	}
	return strings.Join(parts, " ")
}

func preferredBlurayToken(title string) string {
	upper := strings.ToUpper(title)
	if strings.Contains(upper, "UHD BLU") || strings.Contains(upper, "UHDBLU") {
		return "UHD Blu-ray"
	}
	if strings.Contains(upper, "3D BLU") || strings.Contains(upper, "3DBLU") {
		return "3D Blu-ray"
	}
	if strings.Contains(upper, "BLU") {
		return "Blu-ray"
	}
	return ""
}

func extractVideoCodec(title string) string {
	upper := strings.ToUpper(title)
	switch {
	case strings.Contains(upper, "AV1"):
		return "AV1"
	case strings.Contains(upper, "VP9"), strings.Contains(upper, "VP8"):
		return "VP9"
	case strings.Contains(upper, "AVS2"):
		return "AVS2"
	case strings.Contains(upper, "X265"):
		return "x265"
	case strings.Contains(upper, "H.265"), strings.Contains(upper, "H265"), strings.Contains(upper, "HEVC"):
		return "HEVC"
	case strings.Contains(upper, "X264"):
		return "x264"
	case strings.Contains(upper, "H.264"), strings.Contains(upper, "H264"), strings.Contains(upper, "AVC"):
		return "AVC"
	case strings.Contains(upper, "VC-1"), strings.Contains(upper, "VC1"):
		return "VC-1"
	case strings.Contains(upper, "MPEG-2"):
		return "MPEG-2"
	default:
		return ""
	}
}

func extractVideoFormat(title string) string {
	upper := strings.ToUpper(title)
	if strings.Contains(upper, ".MKV") || strings.Contains(upper, " MKV") {
		return "MKV"
	}
	if strings.Contains(upper, ".ISO") || strings.Contains(upper, " ISO") {
		return "ISO"
	}
	if strings.Contains(upper, ".M2TS") || strings.Contains(upper, " M2TS") {
		return "M2TS"
	}
	if strings.Contains(upper, ".MP4") || strings.Contains(upper, " MP4") {
		return "MP4"
	}
	return ""
}

func extractHDRFormat(title string) string {
	matches := findHDRTokens(title)
	if len(matches) == 0 {
		return ""
	}
	hasDoVi, hasHDR10Plus, hasHDR, hasHLG, hasVivid, hasSDR := false, false, false, false, false, false
	for _, item := range matches {
		switch strings.ToUpper(strings.TrimSpace(item)) {
		case "DOLBY VISION", "DOVI", "DV":
			hasDoVi = true
		case "HDR10+":
			hasHDR10Plus = true
		case "HDR10":
			hasHDR = true
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
	if hasDoVi && hasHDR {
		return "DoVi HDR"
	}
	if hasDoVi {
		return "DoVi"
	}
	if hasHDR10Plus {
		return "HDR10+"
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

func extractAudio(title string) string {
	upper := strings.ToUpper(title)
	switch {
	case strings.Contains(upper, "TRUEHD"):
		return "TrueHD"
	case strings.Contains(upper, "DTS:X"), strings.Contains(upper, "DTS X"):
		return "DTS:X"
	case strings.Contains(upper, "DTS-HD MA"), strings.Contains(upper, "DTS HD MA"), reAudioDTSHDMA.MatchString(title):
		return "DTS-HD MA"
	case strings.Contains(upper, "DTS-HD HR"), strings.Contains(upper, "DTS HD HR"):
		return "DTS-HD HR"
	case strings.Contains(upper, "DTS"):
		return "DTS"
	case strings.Contains(upper, "E-AC-3"), strings.Contains(upper, "DDP"), strings.Contains(upper, "DD+"):
		return "DDP"
	case reAudioCodecDD.MatchString(title), strings.Contains(upper, "AC-3"), strings.Contains(upper, "AC3"):
		return "DD"
	case strings.Contains(upper, "FLAC"):
		return "FLAC"
	case strings.Contains(upper, "AAC"):
		return "AAC"
	case strings.Contains(upper, "ALAC"):
		return "ALAC"
	case strings.Contains(upper, "APE"):
		return "APE"
	case strings.Contains(upper, "WAV"):
		return "WAV"
	case strings.Contains(upper, "OPUS"):
		return "Opus"
	case strings.Contains(upper, "MP3"):
		return "MP3"
	case strings.Contains(upper, "LPCM"):
		return "LPCM"
	default:
		return ""
	}
}

func extractBitDepth(title string) string {
	return strings.TrimSpace(reBitDepth.FindString(title))
}

func extractFrameRate(title string) string {
	return strings.TrimSpace(reFrameRate.FindString(title))
}

func extractSourcePlatform(title string) string {
	match := reSourcePlatformBoundary.FindStringSubmatch(title)
	if match == nil {
		return ""
	}
	return match[1]
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
	for _, kw := range []string{"PROPER", "REPACK", "RERIP", "DIRFIX", "INTERNAL", "REMUX"} {
		if strings.Contains(upper, kw) {
			return kw
		}
	}
	return ""
}

func extractGroup(title string) string {
	title = strings.TrimSpace(title)
	idx := strings.LastIndex(title, "-")
	if idx <= 0 || idx >= len(title)-1 {
		return ""
	}
	raw := strings.TrimSpace(title[idx+1:])
	upper := strings.ToUpper(raw)
	if upper == "NOGROUP" || upper == "N/A" || upper == "NONE" || upper == "UNKNOWN" {
		return ""
	}
	if len(raw) < 2 || len(raw) > 30 {
		return ""
	}
	return raw
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
	case "UHD", "HDR", "SDR", "HLG", "DV", "DOVI", "ATMOS", "HYBRID", "REMASTER", "REMASTERED":
		return false
	}
	return true
}

func removeToken(title, token string) string {
	if token == "" {
		return title
	}
	re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(token) + `\b`)
	return strings.TrimSpace(re.ReplaceAllString(title, " "))
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

// NormalizeVideoCodecByMedium 根据媒介类型纠正视频编码命名
// 原盘→AVC/HEVC, WEB-DL→H.264/H.265, 压制→x264/x265
func NormalizeVideoCodecByMedium(c *TitleComponents, mediaInfo string) {
	if c.Medium == "" || c.VideoCodec == "" {
		return
	}
	sourceType := classifySourceFromMedium(c.Medium)
	if sourceType == "" {
		return
	}
	family := detectCodecFamily(c.VideoCodec, mediaInfo)
	if family == "" {
		return
	}
	targetMap := map[string]map[string]string{
		"disc":  {"h264": "AVC", "h265": "HEVC"},
		"webdl": {"h264": "H.264", "h265": "H.265"},
		"rip":   {"h264": "x264", "h265": "x265"},
	}
	if target, ok := targetMap[sourceType]; ok {
		if codec, ok := target[family]; ok {
			c.VideoCodec = codec
		}
	}
}

func classifySourceFromMedium(medium string) string {
	if medium == "" {
		return ""
	}
	if regexp.MustCompile(`(?i)\bWEB[-\s]?DL\b`).MatchString(medium) {
		return "webdl"
	}
	if regexp.MustCompile(`(?i)\b(?:UHDTV|HDTV)\b`).MatchString(medium) {
		return "webdl"
	}
	if regexp.MustCompile(`(?i)\b(?:TV|DVD|BD)[-?\s]?RIP\b`).MatchString(medium) {
		return "rip"
	}
	if regexp.MustCompile(`(?i)\b(?:Blu.?ray|BD|UHD|Remux|DVD[59])\b`).MatchString(medium) {
		return "disc"
	}
	if regexp.MustCompile(`(?i)\bWEBRip\b`).MatchString(medium) {
		return "rip"
	}
	return ""
}

func detectCodecFamily(candidates ...string) string {
	combined := strings.Join(candidates, " ")
	if combined == "" {
		return ""
	}
	hasH265 := regexp.MustCompile(`(?i)\b(HEVC|x265|H\s*\.?\s*265)\b`).MatchString(combined)
	hasH264 := regexp.MustCompile(`(?i)\b(AVC|x264|H\s*\.?\s*264)\b`).MatchString(combined)
	if hasH265 && !hasH264 {
		return "h265"
	}
	if hasH264 && !hasH265 {
		return "h264"
	}
	return ""
}
