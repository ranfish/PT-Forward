package titleparser

import (
	"errors"
	"regexp"
	"strings"
)

// CorrectWithMediaInfo 用 MediaInfo 文本纠正标题解析结果
// mediaInfoText 必须非空，否则返回错误
func CorrectWithMediaInfo(c *TitleComponents, mediaInfoText string) error {
	if mediaInfoText == "" {
		return errors.New("MediaInfo 为空，无法纠正")
	}
	if c == nil {
		return errors.New("components 为空")
	}

	mi := parseMediaInfo(mediaInfoText)

	// 视频编码纠正：MediaInfo Format 优先于标题
	if mi.videoFormat != "" {
		if corrected := codecFromMIFormat(mi.videoFormat, mi.writingLibrary); corrected != "" {
			c.VideoCodec = corrected
		}
	}

	// 分辨率纠正：MediaInfo Height 优先
	if mi.height > 0 {
		c.Resolution = resolutionFromHeight(mi.height)
	}

	// 色深纠正
	if mi.bitDepth > 0 {
		c.BitDepth = formatBitDepth(mi.bitDepth)
	}

	// HDR 纠正：合并所有 HDR format 行
	if len(mi.hdrFormats) > 0 {
		c.HDRFormat = normalizeHDRFromMI(strings.Join(mi.hdrFormats, "\n"))
	}

	// 音频编码纠正
	if mi.audioFormat != "" {
		if corrected := audioFromMIFormat(mi.audioFormat); corrected != "" {
			c.AudioCodec = corrected
		}
	}

	// codec-by-medium 二次纠正（用 MediaInfo 补充信息）
	NormalizeVideoCodecByMedium(c, mediaInfoText)

	return nil
}

type mediaInfoFields struct {
	videoFormat     string
	writingLibrary  string
	height          int
	bitDepth        int
	hdrFormats      []string // 多行 HDR format
	audioFormat     string
	audioChannels   int
}

func parseMediaInfo(text string) mediaInfoFields {
	mi := mediaInfoFields{}

	// 按行解析
	lines := strings.Split(text, "\n")
	section := ""
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 检测 section
		if !strings.Contains(line, ":") {
			upper := strings.ToUpper(line)
			if upper == "VIDEO" || strings.HasPrefix(upper, "VIDEO") {
				section = "video"
			} else if upper == "AUDIO" || strings.HasPrefix(upper, "AUDIO") {
				section = "audio"
			} else {
				section = ""
			}
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		switch section {
		case "video":
			switch strings.ToLower(key) {
			case "format":
				mi.videoFormat = val
			case "writing library":
				mi.writingLibrary = val
			case "height":
				mi.height = parseHeight(val)
			case "bit depth":
				mi.bitDepth = parseBitDepth(val)
			case "hdr format":
				mi.hdrFormats = append(mi.hdrFormats, val)
			case "colour description (general)":
				if len(mi.hdrFormats) == 0 && strings.Contains(strings.ToLower(val), "hdr") {
					mi.hdrFormats = append(mi.hdrFormats, val)
				}
			}
		case "audio":
			switch strings.ToLower(key) {
			case "format":
				mi.audioFormat = val
			case "channel(s)":
				mi.audioChannels = parseChannels(val)
			}
		}
	}

	return mi
}

func codecFromMIFormat(format, writingLibrary string) string {
	upper := strings.ToUpper(format)
	libLower := strings.ToLower(writingLibrary)

	// Writing library 优先判断
	if strings.Contains(libLower, "x264") {
		return "x264"
	}
	if strings.Contains(libLower, "x265") {
		return "x265"
	}

	switch {
	case strings.Contains(upper, "HEVC") || strings.Contains(upper, "H.265"):
		return "HEVC"
	case strings.Contains(upper, "AVC") || strings.Contains(upper, "H.264"):
		return "AVC"
	case strings.Contains(upper, "AV1"):
		return "AV1"
	case strings.Contains(upper, "MPEG-2") || strings.Contains(upper, "MPEG 2"):
		return "MPEG-2"
	case strings.Contains(upper, "VC-1"):
		return "VC-1"
	case strings.Contains(upper, "VP9"):
		return "VP9"
	}
	return ""
}

func resolutionFromHeight(height int) string {
	switch {
	case height >= 4320:
		return "2160p" // 8K → 标记为 2160p（实际 4320 行）
	case height >= 2160:
		return "2160p"
	case height >= 1080:
		return "1080p"
	case height >= 720:
		return "720p"
	case height >= 480:
		return "480p"
	default:
		return ""
	}
}

func formatBitDepth(bits int) string {
	if bits > 0 {
		return formatInt(bits) + "bit"
	}
	return ""
}

func normalizeHDRFromMI(hdr string) string {
	lower := strings.ToLower(hdr)
	hasDoVi := strings.Contains(lower, "dolby vision") || strings.Contains(lower, "dovi") || strings.Contains(lower, "dv ")
	hasHDR10Plus := strings.Contains(lower, "hdr10+") || strings.Contains(lower, "hdr10 plus")
	hasHDR10 := strings.Contains(lower, "hdr10")
	hasHDR := strings.Contains(lower, "hdr")
	hasHLG := strings.Contains(lower, "hlg")

	if hasDoVi && hasHDR10Plus {
		return "DoVi HDR10+"
	}
	if hasDoVi && hasHDR10 {
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
	if hasHLG {
		return "HLG"
	}
	if hasHDR {
		return "HDR"
	}
	return ""
}

func audioFromMIFormat(format string) string {
	upper := strings.ToUpper(format)
	switch {
	case strings.Contains(upper, "DTS-HD MASTER") || strings.Contains(upper, "DTS HD MASTER") || strings.Contains(upper, "DTS-HD MA") || strings.Contains(upper, "DTS HD MA") || strings.Contains(upper, "DTSHD-MA"):
		return "DTS-HD MA"
	case strings.Contains(upper, "DTS-HD HIGH") || strings.Contains(upper, "DTS HD HIGH RESOLUTION"):
		return "DTS-HD HR"
	case strings.Contains(upper, "DTS:X") || strings.Contains(upper, "DTS X"):
		return "DTS:X"
	case strings.Contains(upper, "DTS") && !strings.Contains(upper, "DTS-HD") && !strings.Contains(upper, "DTS HD"):
		return "DTS"
	case strings.Contains(upper, "TRUEHD"):
		return "TrueHD"
	case strings.Contains(upper, "E-AC-3") || strings.Contains(upper, "EAC3") || strings.Contains(upper, "DOLBY DIGITAL PLUS"):
		return "DDP"
	case strings.Contains(upper, "AC-3") || strings.Contains(upper, "AC3") || strings.Contains(upper, "DOLBY DIGITAL"):
		return "DD"
	case strings.Contains(upper, "FLAC"):
		return "FLAC"
	case strings.Contains(upper, "AAC"):
		return "AAC"
	case strings.Contains(upper, "ALAC"):
		return "ALAC"
	case strings.Contains(upper, "PCM") || strings.Contains(upper, "LPCM"):
		return "LPCM"
	case strings.Contains(upper, "OPUS"):
		return "Opus"
	case strings.Contains(upper, "DSD"):
		return "DSD"
	}
	return ""
}

func parseHeight(val string) int {
	val = strings.ReplaceAll(val, " ", "")
	val = regexp.MustCompile(`[^\d]`).ReplaceAllString(val, "")
	if val == "" {
		return 0
	}
	n := 0
	for _, ch := range val {
		if ch >= '0' && ch <= '9' {
			n = n*10 + int(ch-'0')
		}
	}
	return n
}

func parseBitDepth(val string) int {
	val = regexp.MustCompile(`[^\d]`).ReplaceAllString(val, "")
	if val == "" {
		return 0
	}
	n := 0
	for _, ch := range val {
		if ch >= '0' && ch <= '9' {
			n = n*10 + int(ch-'0')
		}
	}
	return n
}

func parseChannels(val string) int {
	val = strings.TrimSpace(val)
	m := regexp.MustCompile(`(\d+)\s*channel`).FindStringSubmatch(strings.ToLower(val))
	if m != nil {
		n := 0
		for _, ch := range m[1] {
			n = n*10 + int(ch-'0')
		}
		return n
	}
	return 0
}

func formatInt(n int) string {
	if n == 0 {
		return ""
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}
