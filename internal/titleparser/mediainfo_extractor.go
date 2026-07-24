package titleparser

import (
	"strconv"
	"strings"
)

// MediaInfoTech 从 MediaInfo 纯文本提取的技术特征（§56.34 MediaInfo 提取器）。
//
// 基于 9802 个样本（9726 红豆饭 + 76 tr辅种）归纳的字段映射表，
// 替代 corrector.go 中 parseMediaInfo 的 4 字段纠正。
// 覆盖 9 种 Video Format + 27 种 Audio Format + 7 种 HDR 格式。
type MediaInfoTech struct {
	Resolution      string // 2160p/1080p/720p/480p/1440p/4320p
	VideoCodec      string // x264/x265/HEVC/AVC/AV1/VP9/MPEG-2/VC-1/AVS+/AVS2
	AudioCodec      string // DDP/DD/DTS/DTS-HD MA/DTS-HD HR/DTS-ES/TrueHD/FLAC/AAC/LPCM/Opus/AV3A/xHE-AAC/ALAC
	AudioChannels   string // 2.0/5.1/7.1
	AudioTechnology string // Atmos
	AudioTracks     int    // 有效音轨数（排除兼容音轨）
	HDR             string // HDR10/HDR10+/DoVi/DoVi HDR/DoVi HDR10+/HDR Vivid/HLG/PQ10
	BitDepth        string // 8bit/10bit
}

// miStream MediaInfo 纯文本中的一个段（Video / Audio #N）。
type miStream struct {
	name   string // "video" / "audio"
	fields map[string]string
}

// ExtractMediaInfo 从 MediaInfo 纯文本提取技术特征。
//
// 解析策略：
//   - 按段标题（Video / Audio #N）分割
//   - 每段按 "字段名 : 值" 解析（SplitN ":", 2）
//   - 按 9802 样本映射表转换
//
// 空输入返回零值 MediaInfoTech（不 error，调用方用 nil 判断）。
func ExtractMediaInfo(text string) MediaInfoTech {
	var result MediaInfoTech
	if strings.TrimSpace(text) == "" {
		return result
	}

	streams := parseMIStreams(text)

	// Video 段（取第一个）
	for _, s := range streams {
		if s.name == "video" {
			result.Resolution = resolutionFromWidth(s.fields["width"])
			result.VideoCodec = codecFromMI(s.fields["format"], s.fields["writing library"])
			result.BitDepth = bitDepthFromMI(s.fields["bit depth"])
			result.HDR = hdrFromMI(s.fields["hdr format"])
			break
		}
	}

	// Audio 段（可能有多个）
	var audioStreams []miStream
	for _, s := range streams {
		if s.name == "audio" {
			audioStreams = append(audioStreams, s)
		}
	}
	if len(audioStreams) > 0 {
		first := audioStreams[0]
		result.AudioCodec, result.AudioTechnology = audioFromMI(
			first.fields["format"], first.fields["commercial name"],
		)
		result.AudioChannels = channelsFromMI(
			first.fields["channel layout"], first.fields["channel(s)"],
		)
		result.AudioTracks = countAudioTracks(audioStreams)
	}

	return result
}

// parseMIStreams 将 MediaInfo 纯文本解析为段列表。
//
// 段标题是不含 ":" 的独立行（Video / Audio #1 / Audio #2 / General / Text / Menu）。
// 只保留 video 和 audio 段（其他段忽略）。
func parseMIStreams(text string) []miStream {
	var streams []miStream
	curIdx := -1
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.Contains(line, ":") {
			upper := strings.ToUpper(line)
			switch {
			case upper == "VIDEO":
				streams = append(streams, miStream{name: "video", fields: map[string]string{}})
				curIdx = len(streams) - 1
			case strings.HasPrefix(upper, "AUDIO"):
				streams = append(streams, miStream{name: "audio", fields: map[string]string{}})
				curIdx = len(streams) - 1
			default:
				curIdx = -1
			}
			continue
		}
		if curIdx < 0 {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		val := strings.TrimSpace(parts[1])
		if _, exists := streams[curIdx].fields[key]; !exists {
			streams[curIdx].fields[key] = val
		}
	}
	return streams
}

// resolutionFromWidth 按 Width 推断分辨率（Height 63% 非标准，不可靠）。
func resolutionFromWidth(widthStr string) string {
	w := parseMIInt(widthStr)
	if w <= 0 {
		return ""
	}
	switch {
	case w >= 7600:
		return "4320p"
	case w >= 2800:
		return "2160p"
	case w >= 1800:
		return "1080p"
	case w >= 1400:
		return "1440p"
	case w >= 1180:
		return "720p"
	case w >= 600:
		return "480p"
	default:
		return ""
	}
}

// codecFromMI 从 MediaInfo Format + Writing Library 映射视频编码。
//
// Writing Library 优先：x264/x265 是压制编码器（覆盖 Format）。
// 其他 Writing Library（V265/mgtv/ptw/VOLO/mp4ff/o264/DOVI）不改变编码 → 用 Format。
func codecFromMI(format, writingLibrary string) string {
	upper := strings.ToUpper(strings.TrimSpace(format))
	libLower := strings.ToLower(writingLibrary)

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
	case strings.Contains(upper, "AVS+"):
		return "AVS+"
	case strings.Contains(upper, "AVS2"):
		return "AVS2"
	case strings.Contains(upper, "AV1"):
		return "AV1"
	case strings.Contains(upper, "VP9"):
		return "VP9"
	case strings.Contains(upper, "VC-1"):
		return "VC-1"
	case strings.Contains(upper, "MPEG-2") || strings.Contains(upper, "MPEG VIDEO"):
		return "MPEG-2"
	}
	return ""
}

// audioFromMI 从 MediaInfo Audio Format + Commercial name 映射音频编码 + 音频技术。
//
// 返回 (codec, technology)。technology 非空时为 "Atmos"。
// DTS:X 不标注（v1.05 规范）。
func audioFromMI(format, commercialName string) (codec, technology string) {
	upper := strings.ToUpper(strings.TrimSpace(format))
	commLower := strings.ToLower(commercialName)

	switch {
	// TrueHD 系列（MLP FBA = Meridian Lossless Packing FBA = TrueHD 内部格式）
	case strings.Contains(upper, "MLP FBA"):
		codec = "TrueHD"
	case strings.Contains(upper, "TRUEHD"):
		codec = "TrueHD"

	// DTS 系列（从特例到一般，顺序敏感）
	case strings.Contains(upper, "DTS-UHD"):
		codec = "DTS"
	case strings.Contains(upper, "DTS ES"):
		codec = "DTS-ES"
	case strings.Contains(upper, "DTS XLL X"):
		codec = "DTS"
	case strings.Contains(upper, "DTS XLL"):
		codec = "DTS-HD MA"
	case strings.Contains(upper, "DTS LBR"):
		codec = "DTS-HD HR"
	case strings.HasPrefix(upper, "DTS"):
		codec = "DTS"

	// E-AC-3 系列（JOC = Atmos）
	case strings.Contains(upper, "E-AC-3 JOC") || strings.Contains(upper, "EAC3 JOC"):
		codec = "DDP"
		technology = "Atmos"
	case strings.Contains(upper, "E-AC-3") || strings.Contains(upper, "EAC3"):
		codec = "DDP"

	// AC-3（必须在 E-AC-3 之后，因为 "E-AC-3" 也 Contains "AC-3"）
	case strings.Contains(upper, "AC-3") || strings.Contains(upper, "AC3"):
		codec = "DD"

	// 其他编码
	case strings.Contains(upper, "FLAC"):
		codec = "FLAC"
	case strings.Contains(upper, "PCM"):
		codec = "LPCM"
	case strings.HasPrefix(upper, "AAC"):
		codec = "AAC"
	case strings.Contains(upper, "MPEG AUDIO"):
		codec = "MP2"
	case upper == "AV3A":
		codec = "AV3A"
	case strings.Contains(upper, "OPUS"):
		codec = "Opus"
	case strings.Contains(upper, "USAC"):
		codec = "xHE-AAC"
	case strings.Contains(upper, "ALAC"):
		codec = "ALAC"
	}

	// Atmos 补充检测（Commercial name）
	if technology == "" && codec != "" {
		if strings.Contains(commLower, "dolby atmos") {
			technology = "Atmos"
		}
	}

	return codec, technology
}

// channelsFromMI 从 Channel layout / Channel(s) 推断声道数。
//
// Channel layout 优先（精确到每个声道 token）：
//   - token 数 = 全频声道 + LFE
//   - 有 LFE → N.1，无 LFE → N.0
//
// 无 Channel layout 时 fallback 到 Channel(s) 数字。
func channelsFromMI(channelLayout, channelsStr string) string {
	layout := strings.TrimSpace(channelLayout)
	if layout != "" {
		tokens := strings.Fields(layout)
		hasLFE := strings.Contains(strings.ToUpper(layout), "LFE")
		fullRange := len(tokens)
		if hasLFE {
			fullRange--
		}
		if fullRange > 0 {
			lfe := 0
			if hasLFE {
				lfe = 1
			}
			return strconv.Itoa(fullRange) + "." + strconv.Itoa(lfe)
		}
	}
	n := parseMIInt(channelsStr)
	switch n {
	case 1:
		return "1.0"
	case 2:
		return "2.0"
	case 6:
		return "5.1"
	case 8:
		return "7.1"
	default:
		if n > 0 {
			return strconv.Itoa(n) + ".0"
		}
		return ""
	}
}

// hdrFromMI 解析 MediaInfo 的 HDR format 字段（可能含多个值用 " / " 分隔）。
//
// 组合判断（顺序敏感）：
//   - DoVi + HDR10+  → DoVi HDR10+
//   - DoVi + HDR10   → DoVi HDR
//   - DoVi alone     → DoVi
//   - HDR10+         → HDR10+
//   - HDR Vivid      → HDR Vivid
//   - HDR10          → HDR10
//   - HLG / PQ10     → HLG / PQ10
func hdrFromMI(hdrFormat string) string {
	if strings.TrimSpace(hdrFormat) == "" {
		return ""
	}
	lower := strings.ToLower(hdrFormat)
	hasDoVi := strings.Contains(lower, "dolby vision") || strings.Contains(lower, "dovi") ||
		strings.Contains(lower, "dvhe") || strings.Contains(lower, "dvha") || strings.Contains(lower, "dvav")
	hasHDR10Plus := strings.Contains(lower, "hdr10+") || strings.Contains(lower, "smpte st 2094")
	hasHDR10 := strings.Contains(lower, "hdr10") || strings.Contains(lower, "smpte st 2086")
	hasVivid := strings.Contains(lower, "hdr vivid") || strings.Contains(lower, "hdrvivid")
	hasHLG := strings.Contains(lower, "hlg")
	hasPQ10 := strings.Contains(lower, "pq10")

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
	if hasVivid {
		return "HDR Vivid"
	}
	if hasHDR10 {
		return "HDR10"
	}
	if hasHLG {
		return "HLG"
	}
	if hasPQ10 {
		return "PQ10"
	}
	return ""
}

// bitDepthFromMI 从 Bit depth 字段提取色深。
func bitDepthFromMI(bitDepthStr string) string {
	n := parseMIInt(bitDepthStr)
	if n <= 0 {
		return ""
	}
	return strconv.Itoa(n) + "bit"
}

// countAudioTracks 计算有效音轨数（排除兼容音轨）。
//
// 排除规则：Audio #1 是高清音轨（TrueHD / DTS-HD MA）+ Audio #2 是 AC-3/DDP → 后者为兼容音轨，不计入。
func countAudioTracks(audioStreams []miStream) int {
	total := len(audioStreams)
	if total <= 1 {
		return total
	}
	firstCodec, _ := audioFromMI(audioStreams[0].fields["format"], "")
	secondCodec, _ := audioFromMI(audioStreams[1].fields["format"], "")
	hiRes := firstCodec == "TrueHD" || firstCodec == "DTS-HD MA"
	compat := secondCodec == "DD" || secondCodec == "DDP"
	if hiRes && compat {
		return total - 1
	}
	return total
}

// parseMIInt 从 MediaInfo 字段值中提取数字。
//
// "3 840 pixels" → 3840；"10 bits" → 10；"6 channels" → 6。
// 字段值中只有一个数字串，去掉空格后提取连续数字。
func parseMIInt(val string) int {
	val = strings.ReplaceAll(val, " ", "")
	val = strings.ReplaceAll(val, "\u00a0", "")
	n := 0
	for _, ch := range val {
		if ch >= '0' && ch <= '9' {
			n = n*10 + int(ch-'0')
		}
	}
	return n
}
