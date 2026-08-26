package titleparser

import (
	"regexp"
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
		best := selectBestAudioStream(audioStreams)
		result.AudioCodec, result.AudioTechnology = audioFromMI(
			best.fields["format"], best.fields["commercial name"],
		)
		result.AudioChannels = channelsFromMI(
			best.fields["channel layout"], best.fields["channel(s)"],
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
// §59.27 P2：canonical 值域由 videoCodecRegistry 保证（MI 特有输入 MPEG VIDEO 由 pattern 覆盖）。
func codecFromMI(format, writingLibrary string) string {
	libLower := strings.ToLower(writingLibrary)

	if strings.Contains(libLower, "x264") {
		return "x264"
	}
	if strings.Contains(libLower, "x265") {
		return "x265"
	}

	// registry 顺序即优先级；MI Format 值（HEVC/AVC/AVS+/AVS2/AV1/VP9/VC-1/MPEG-2）
	// 与标题变体共用 pattern（MPEG VIDEO → MPEG-2 由追加映射覆盖）。
	if c := lookupToken(videoCodecRegistry, format); c != "" {
		return c
	}
	// MI 特有 Format 值兜底
	upper := strings.ToUpper(strings.TrimSpace(format))
	switch {
	case strings.Contains(upper, "MPEG VIDEO"):
		return "MPEG-2"
	}
	return ""
}

// audioFromMI 从 MediaInfo Audio Format + Commercial name 映射音频编码 + 音频技术。
//
// 返回 (codec, technology)。technology 非空时为 "Atmos"。
// DTS:X 不标注（v1.05 规范）。
// §59.27 P2：canonical 值域由 audioCodecRegistry 保证（MI 内部名 DTS XLL / MLP FBA /
// E-AC-3 JOC / MPEG AUDIO / USAC 已纳入 registry pattern，两侧单一来源）。
func audioFromMI(format, commercialName string) (codec, technology string) {
	upper := strings.ToUpper(strings.TrimSpace(format))
	commLower := strings.ToLower(commercialName)

	// Atmos 优先检测（E-AC-3 JOC = DDP Atmos，必须在普通 E-AC-3 之前）
	if strings.Contains(upper, "E-AC-3 JOC") || strings.Contains(upper, "EAC3 JOC") {
		codec = "DDP"
		technology = "Atmos"
	} else if c := lookupToken(audioCodecRegistry, format); c != "" {
		codec = c
	}

	// Atmos 补充检测（Commercial name）
	if technology == "" && codec != "" {
		if strings.Contains(commLower, "dolby atmos") {
			technology = "Atmos"
		}
	}
	// Auro3D 检测（§59.41，v1.05 对象信息值域另一半）——MI Commercial name
	// 形态 "Auro-3D"/"Auro-C"（Barco Auro 编码器产物，与 Dolby 系并列）
	if technology == "" && codec != "" {
		if strings.Contains(commLower, "auro") {
			technology = "Auro3D"
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
// commentaryTrackRe §59.77: 副标题评论音轨声明（243 实证形态）。
// 数量前缀中文数字词: 无前缀=1 / 双=2 / 三=3; "带字幕"等后缀天然兼容。
var commentaryTrackRe = regexp.MustCompile(`(?i)(三|双)?评论音轨|commentary[- ]?track`)

// AdjustCommentaryTracks §59.77: v1.05 "评论音轨不计入音轨数"——从副标题提取
// 评论轨数并扣减。防御: MI<=1 不扣（单轨不可能有评论轨，副标题误标防御）；
// 下限 0。
func AdjustCommentaryTracks(miTracks int, subtitle string) int {
	if miTracks <= 1 || subtitle == "" {
		return miTracks
	}
	m := commentaryTrackRe.FindStringSubmatch(subtitle)
	if m == nil {
		return miTracks
	}
	n := 1
	switch m[1] {
	case "双":
		n = 2
	case "三":
		n = 3
	}
	if miTracks-n < 0 {
		return 0
	}
	return miTracks - n
}

func countAudioTracks(audioStreams []miStream) int {
	total := len(audioStreams)
	if total <= 1 {
		return total
	}
	firstCodec, _ := audioFromMI(audioStreams[0].fields["format"], "")
	secondCodec, _ := audioFromMI(audioStreams[1].fields["format"], "")
	hiRes := firstCodec == "TrueHD" || firstCodec == "DTS-HD MA"
	compat := secondCodec == "DD" || secondCodec == "DDP"
	// §59.113: Title 佐证——兼容轨（同内容降级副本）技术特征是 Title 空；
	// Title 有内容标识（Mandarin (台配)/Cantonese 等语言内容）= 独立音轨不扣。
	// 幽灵公主实锤: DTS-HD MA + 3 条 Title 标识轨被误扣 1（4→3）。
	if hiRes && compat {
		if t := strings.TrimSpace(audioStreams[1].fields["title"]); t != "" {
			return total
		}
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

// audioSpecRank 音频编码规格排序（v1.05 :164 "多音轨仅标最高规格"）。
// 数值越高规格越高。Atmos 额外 +5。
var audioSpecRank = map[string]int{
	"TrueHD":    100,
	"DTS-HD MA": 90,
	"FLAC":      85,
	"DTS-HD HR": 80,
	"DTS-ES":    75,
	"DDP":       70,
	"DTS":       60,
	"DD":        50,
	"LPCM":      45,
	"AAC":       40,
	"AV3A":      35,
	"Opus":      30,
	"xHE-AAC":   25,
	"MP2":       20,
	"MP3":       15,
	"ALAC":      10,
}

// audioSpecRankOf 获取音轨的规格分数（含 Atmos 加分）。
func audioSpecRankOf(stream miStream) int {
	codec, tech := audioFromMI(stream.fields["format"], stream.fields["commercial name"])
	rank := audioSpecRank[codec]
	if tech == "Atmos" {
		rank += 5
	}
	return rank
}

// selectBestAudioStream 按最高规格选择音轨（v1.05 :164）。
//
// 多音轨种子中，标题只标最高规格音轨。例如：
//   - Audio #1 = DDP 5.1, Audio #2 = TrueHD Atmos 7.1 → 选 TrueHD Atmos（规格更高）
//   - Audio #1 = DTS-HD MA, Audio #2 = AC-3 → 选 DTS-HD MA（规格更高）
func selectBestAudioStream(streams []miStream) miStream {
	if len(streams) <= 1 {
		if len(streams) == 1 {
			return streams[0]
		}
		return miStream{}
	}
	bestIdx := 0
	bestRank := audioSpecRankOf(streams[0])
	for i := 1; i < len(streams); i++ {
		rank := audioSpecRankOf(streams[i])
		if rank > bestRank {
			bestIdx = i
			bestRank = rank
		}
	}
	return streams[bestIdx]
}
