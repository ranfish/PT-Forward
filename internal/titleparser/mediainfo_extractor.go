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
	Encoded         bool   // §59.151: Writing library 存在=重编码（IsEncode MI 驱动唯一判据）
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
			result.Resolution = resolutionFromHeightOrWidth(s.fields["height"], s.fields["width"])
			result.VideoCodec = codecFromMI(s.fields["format"], s.fields["writing library"])
			result.BitDepth = bitDepthFromMI(s.fields["bit depth"])
			result.HDR = hdrFromMI(s.fields["hdr format"])
			// §59.151: Writing library 存在 = 重编码铁证（x265 等编码器写入痕迹）——
			// IsEncode MI 驱动的唯一判据（165/166 语料实证；spec/标题循环依赖废除）
			result.Encoded = strings.TrimSpace(s.fields["writing library"]) != ""
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
// MISections §59.117: MI 五层结构化（General/Video/Audio/Text/Menu）——
// 统一信息源：所有 MI 信息采集从各自层进行，消灭全文 regex（跨行补丁式
// 判据的根除——§59.109/113 语言判据、§59.116 评论字幕误扣的治本层）。
type MISections struct {
	General map[string]string
	Videos  []map[string]string
	Audios  []map[string]string
	Texts   []map[string]string
	Menus   []map[string]string
}

// ParseMISections 解析 MI 纯文本为五层结构。非标文本返回空结构（不 error——
// 消费方 fallback 全文 regex）。字段 key 小写、首值优先（与 parseMIStreams 一致）。
func ParseMISections(text string) MISections {
	var s MISections
	var cur *map[string]string
	var curList *[]map[string]string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.Contains(line, ":") {
			upper := strings.ToUpper(line)
			switch {
			case strings.HasPrefix(upper, "GENERAL"):
				s.General = map[string]string{}
				cur, curList = &s.General, nil
			case strings.HasPrefix(upper, "VIDEO"):
				s.Videos = append(s.Videos, map[string]string{})
				cur, curList = nil, &s.Videos
			case strings.HasPrefix(upper, "AUDIO"):
				s.Audios = append(s.Audios, map[string]string{})
				cur, curList = nil, &s.Audios
			case strings.HasPrefix(upper, "TEXT"):
				s.Texts = append(s.Texts, map[string]string{})
				cur, curList = nil, &s.Texts
			case strings.HasPrefix(upper, "MENU"):
				s.Menus = append(s.Menus, map[string]string{})
				cur, curList = nil, &s.Menus
			default:
				cur, curList = nil, nil
			}
			continue
		}
		var target map[string]string
		if cur != nil {
			target = *cur
		} else if curList != nil && len(*curList) > 0 {
			target = (*curList)[len(*curList)-1]
		}
		if target == nil {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		val := strings.TrimSpace(parts[1])
		if _, exists := target[key]; !exists {
			target[key] = val
		}
		// 回写（slice 元素是值——map 头本身共享, 无需回写; cur 指针同理）
	}
	return s
}

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

// resolutionFromHeightOrWidth §59.166 宽幅修正（地道战/天堂里的烦恼站方审核实证）：
// 高度有效（>=400）优先按高度——1.37:1 等特殊画幅铺满 1080p 容器（高 1080 宽
// 1432-1480）应判 1080p 而非 1440p；高度缺失/异常回退宽度（Height 63% 历史顾虑
// 针对的是异常高度值——档位下限 400 过滤脏值）。
func resolutionFromHeightOrWidth(heightStr, widthStr string) string {
	// 形态学（测试实证 x264_DTSXLL 案）：宽幅裁高（h800×w1920=1080p 容器 2.35:1）
	// 应按宽度；特殊高画幅（h1080×w1432）应按高度。规则=高度≥1000（满高）用
	// 高度档，否则回退宽度档（高度档只保留 4320p/2160p/1080p——720p 及以下
	// 高度语义弱，统一宽度）。
	if h := parseMIInt(heightStr); h >= 1000 {
		switch {
		case h >= 4200:
			return "4320p"
		case h >= 2000:
			return "2160p"
		default:
			return "1080p"
		}
	}
	return resolutionFromWidth(widthStr)
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

// commentaryTrackRe §59.77: 评论音轨声明（副标题中文形态）。
// 数量前缀中文数字词: 无前缀=1 / 双=2 / 三=3; "带字幕"等后缀天然兼容。
var commentaryTrackRe = regexp.MustCompile(`(?i)(三|双)?评论音轨`)

// commentaryTitleInAudio §59.114/§59.116: MI **Audio 段内** Title 行 Commentary 计数。
// 按段切片（复用 parseMIStreams 语义）而非单 regex——Text(字幕轨)的 Commentary
// 配套字幕不是评论音轨（天堂里的烦恼 2−2=0 误扣实锤）；RE2 无前瞻，切片是正解。
func commentaryTitleInAudio(mediaInfo string) int {
	if mediaInfo == "" {
		return 0
	}
	n := 0
	for _, s := range parseMIStreams(mediaInfo) {
		if s.name != "audio" {
			continue
		}
		if commentaryTitleLineRe.MatchString(s.fields["title"]) {
			n++
		}
	}
	return n
}

var commentaryTitleLineRe = regexp.MustCompile(`(?i)\bCommentary\b`)

// AdjustCommentaryTracks §59.77/§59.114: v1.05 "评论音轨不计入音轨数"——扣减信号两源:
// ① 副标题"评论音轨"声明（中文，数量前缀）② MI Title 行 Commentary（每行算 1）。
// 防御: MI<=1 不扣（单轨不可能有评论轨，误标防御）；下限 0；两源取大值不叠加。
func AdjustCommentaryTracks(miTracks int, subtitle string, mediaInfo string) int {
	if miTracks <= 1 {
		return miTracks
	}
	nSub := 0
	if subtitle != "" {
		if m := commentaryTrackRe.FindStringSubmatch(subtitle); m != nil {
			nSub = 1
			switch m[1] {
			case "双":
				nSub = 2
			case "三":
				nSub = 3
			}
		}
	}
	nMI := commentaryTitleInAudio(mediaInfo)
	n := nSub
	if nMI > n {
		n = nMI
	}
	if n == 0 {
		return miTracks
	}
	if miTracks-n < 0 {
		return 0
	}
	return miTracks - n
}

func countAudioTracks(audioStreams []miStream) int {
	// §59.114: v1.05 权威语义——音轨数 = Audio 段数（评论轨扣减在
	// AdjustCommentaryTracks 声明制层）。兼容轨排除（TrueHD 内嵌 DD 副本扣 1）
	// 是违反 v1.05 的多余启发式——兼容副本也是正片音轨应计入（极限审判实锤:
	// 3 正片轨被误扣为 2；用户以 v1.05:199 原文纠偏）。
	return len(audioStreams)
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
