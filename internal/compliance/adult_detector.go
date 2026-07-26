// Package compliance — 成人内容精确识别（qui 项目移植）。
// 参考: examples/qui/pkg/releases/content_type.go
// 4 种正则检测模式: XXX 关键词 / JAV 番号 / 日期格式 / 括号日期
// 2 种误报排除: xXx 电影系列 / RIAJ 音乐编码
// 1 种覆写: JAV 番号剥离后剩余标题含视频标记 → 非成人
package compliance

import (
	"regexp"
	"strings"
)

var (
	reAdultXXX    = regexp.MustCompile(`(?i)\bxxx\b`)
	reJAV         = regexp.MustCompile(`(?i)\b(?:[A-Z0-9]{3,4})-\d{3,4}\b`)
	reAdultDate   = regexp.MustCompile(`\b\d{6,8}[_-]\d{3}\b`)
	reBracketDate = regexp.MustCompile(`\[[12]\d{3}\.\d{2}\.\d{2}\]`)
	reRIAJ        = regexp.MustCompile(`(?i)\b[A-Z]{4}-?\d{3,5}\b`)
	reSeasonEp    = regexp.MustCompile(`(?i)s\d{1,2}e\d{1,3}`)
)

// riajMediaTypes RIAJ 媒体类型映射（4 字母编码的第 3 个字符决定媒体类型）。
var riajMediaTypes = map[byte]string{
	'A': "dvd-audio",
	'B': "dvd-video",
	'C': "cd",
	'D': "cd-single",
	'F': "cd-video",
	'G': "sacd",
	'H': "hd-dvd",
	'I': "video-cd",
	'J': "vinyl-lp",
	'K': "vinyl-ep",
	'L': "ld-30cm",
	'M': "ld-20cm",
	'N': "cd-g",
	'P': "ps-game",
	'R': "cd-rom",
	'S': "cassette-single",
	'T': "cassette-album",
	'U': "umd-video",
	'V': "vhs",
	'W': "dvd-music",
	'X': "bluray",
	'Y': "md",
	'Z': "multi-format",
}

// detectRIAJMediaType 检查标题中是否包含 RIAJ 音乐编码（4 字母 + 3-5 位数字）。
// 返回非空 = 合法音乐编码（非 JAV）。
func detectRIAJMediaType(title string) string {
	match := reRIAJ.FindString(title)
	if match == "" {
		return ""
	}
	code := strings.ReplaceAll(match, "-", "")
	code = strings.ToUpper(code)
	if len(code) < 4 {
		return ""
	}
	mediaChar := code[2]
	if mediaType, exists := riajMediaTypes[mediaChar]; exists {
		return mediaType
	}
	return ""
}

// isBenignXXX 排除主流 xXx 电影系列（2002/2005/2017）。
func isBenignXXX(titleLower, subtitleLower string) bool {
	if !strings.HasPrefix(titleLower, "xxx") && !strings.HasPrefix(subtitleLower, "xxx") {
		return false
	}
	benignKeywords := []string{
		"xander cage",
		"state of the union",
		"return of xander cage",
	}
	for _, kw := range benignKeywords {
		if strings.Contains(titleLower, kw) || strings.Contains(subtitleLower, kw) {
			return true
		}
	}
	// xXx 电影年份
	reXXXYear := regexp.MustCompile(`xxx.*\b(2002|2005|2017)\b`)
	return reXXXYear.MatchString(titleLower) || reXXXYear.MatchString(subtitleLower)
}

// videoReleaseTokens 合法视频/音频发布中常见的 token。
// 用于 JAV 番号剥离后判断剩余标题是否为合法发布。
var videoReleaseTokens = []string{
	"2160p", "1080p", "720p", "576p", "480p", "1440p",
	"remux", "web-dl", "webdl", "webrip", "hdtv",
	"bluray", "blu-ray", "bdrip", "bdremux", "uhd",
	"h264", "h265", "x264", "x265", "hevc", "av1", "xvid",
	"dts", "atmos", "truehd", "ac3", "eac3", "flac", "aac",
	"dolby", "hdr", "sdr", "dv",
}

// musicReleaseTokens 合法音乐发布中常见的 token。
var musicReleaseTokens = []string{
	"flac", "alac", "ape", "wav", "mp3", "dsd",
	"album", "single", "ep", "ost",
	"vinyl", "sacd", "cd", "cassette",
	"24bit", "16bit", "96khz", "192khz", "44.1khz",
	"lossless", "hq",
}

func hasVideoReleaseMarkers(title string) bool {
	lower := strings.ToLower(title)
	for _, token := range videoReleaseTokens {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return reSeasonEp.MatchString(title)
}

func hasMusicReleaseMarkers(title string) bool {
	lower := strings.ToLower(title)
	for _, token := range musicReleaseTokens {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return false
}

// DetectAdult 检查标题/副标题是否包含成人内容模式。
// 返回 (matched bool, reason string)。
// 4 种模式: XXX 关键词 / JAV 番号 / 日期格式 / 括号日期。
func DetectAdult(title, subtitle string) (bool, string) {
	if title == "" && subtitle == "" {
		return false, ""
	}

	titleLower := strings.ToLower(title)
	subtitleLower := strings.ToLower(subtitle)

	// 1. XXX 关键词（单词边界）— 排除 xXx 电影系列
	if reAdultXXX.MatchString(title) || reAdultXXX.MatchString(subtitle) {
		if !isBenignXXX(titleLower, subtitleLower) {
			return true, "XXX"
		}
	}

	// 2. JAV 番号 (ABC-123) — 剥离后检查剩余标题是否为合法视频/音乐发布
	if match := reJAV.FindString(title); match != "" {
		remaining := strings.TrimSpace(reJAV.ReplaceAllString(title, ""))
		if remaining == "" {
			return true, "JAV:" + match
		}
		if !hasVideoReleaseMarkers(remaining) && !hasMusicReleaseMarkers(remaining) {
			return true, "JAV:" + match
		}
	}

	// 3. 成人日期格式 (010124_001)
	if reAdultDate.MatchString(titleLower) || reAdultDate.MatchString(subtitleLower) {
		return true, "adult_date"
	}

	// 4. 括号日期格式 ([2024.01.01])
	if reBracketDate.MatchString(titleLower) || reBracketDate.MatchString(subtitleLower) {
		return true, "bracket_date"
	}

	return false, ""
}
