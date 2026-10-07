package titleparser

import (
	"strings"
)

// ExtractBDInfo 从 BDInfo 完整报告提取技术特征（§59.319 D5）。
//
// 输入是 go-bdinfo Run 产出的完整报告（含尾部 QUICK SUMMARY 块）。
// 解析锚定 QUICK SUMMARY 段：主播放列表的规范拼接行（"Video: X / Y / Z"），
// 字段以 " / " 分隔；"* " 行前缀为隐藏流（DoVi 增强层等）。
// 测试锚 = 243 PT0 下载器真盘实测报告（testdata/bdinfo_jr_1080p.txt 1080p AVC
// SDR 老盘 + bdinfo_alice_uhd.txt UHD HEVC DoVi 双层盘）。
//
// 已知局限（挂账观察）：
//   - Atmos/DTS:X 标注：BDInfo 报告音轨行是否携带待样本实证（TrueHD Atmos 盘）
//   - 老盘（JR 1080p）Video 行无 bit depth/HDR 字段——SDR 8bit 时代无标注，
//     BitDepth/HDR 留空由调用方兜底（标题推断链）
func ExtractBDInfo(text string) MediaInfoTech {
	var result MediaInfoTech
	summary := extractQuickSummaryBlock(text)
	if summary == "" {
		return result
	}
	result.Encoded = false // BD 结构=原盘铁证（无重编码概念）

	var mainVideoFields []string
	var hasDoVi bool
	var mainAudioFields []string
	audioTracks := 0
	var atmos bool
	for _, line := range strings.Split(summary, "\n") {
		trimmed := strings.TrimSpace(line)
		hidden := strings.HasPrefix(trimmed, "* ")
		if hidden {
			trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "*"))
		}
		switch {
		case strings.HasPrefix(trimmed, "Video:"):
			fields := splitBDFields(strings.TrimPrefix(trimmed, "Video:"))
			if hidden {
				if containsHDRToken(fields, "Dolby Vision") {
					hasDoVi = true
				}
				continue
			}
			if mainVideoFields == nil {
				mainVideoFields = fields
			}
			if containsHDRToken(fields, "Dolby Vision") {
				hasDoVi = true // 单层 DV 盘：DV 直接在主行
			}
		case strings.HasPrefix(trimmed, "Audio:"):
			fields := splitBDFields(strings.TrimPrefix(trimmed, "Audio:"))
			if hidden {
				continue
			}
			audioTracks++
			if mainAudioFields == nil {
				mainAudioFields = fields
			}
			if strings.Contains(strings.Join(fields, " / "), "Atmos") {
				atmos = true
			}
		}
	}

	if mainVideoFields != nil {
		for _, f := range mainVideoFields {
			f = strings.TrimSpace(f)
			if result.VideoCodec == "" {
				result.VideoCodec = videoCodecFromBD(f)
			}
			if result.Resolution == "" && isBDResolutionToken(f) {
				result.Resolution = f
			}
			if result.FrameRate == "" && strings.HasSuffix(f, " fps") {
				result.FrameRate = strings.TrimSpace(strings.TrimSuffix(f, "fps"))
			}
			if result.BitDepth == "" {
				result.BitDepth = bitDepthFromBD(f)
			}
		}
		result.HDR = hdrFromBD(mainVideoFields, hasDoVi)
	}
	if mainAudioFields != nil {
		// 字段序：语言 / codec / 声道 / 采样率 / 码率 / 位深（DTS Core 括注）
		for _, f := range mainAudioFields {
			f = strings.TrimSpace(f)
			if result.AudioCodec == "" {
				result.AudioCodec = audioCodecFromBD(f)
			}
			if result.AudioChannels == "" && isChannelToken(f) {
				result.AudioChannels = f
			}
		}
		result.AudioTracks = audioTracks
		if atmos {
			result.AudioTechnology = "Atmos"
		}
	}
	return result
}

// FormatBDInfoReport §59.319 附四：BDInfo 报告展示形态规范化（幂等）。
// 三规则：a.截 "DISC INFO:" 起（头部 Disc Label/BDINFO HOME 论坛致谢+
// BEGIN FORUMS PASTE 行一并清除）；b.剥 [code]/[/code] 行；c.删
// "<--- END FORUMS PASTE --->" 行。保留 DISC INFO/PLAYLIST REPORT/VIDEO/
// AUDIO/SUBTITLES/FILES/STREAM DIAGNOSTICS/QUICK SUMMARY 八块正文。
// 应用三口：BDInfoScanner.Scan 出口（本地生成）/fetch 归位（源站提取）/
// 存量刷正——发布下发与解析器（QUICK SUMMARY 锚仍在）零影响。
func FormatBDInfoReport(report string) string {
	idx := strings.Index(report, "DISC INFO:")
	if idx < 0 {
		return report // 非报告形态原样（幂等安全）
	}
	out := report[idx:]
	lines := strings.Split(out, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if t == "[code]" || t == "[/code]" {
			continue
		}
		if strings.Contains(t, "END FORUMS PASTE") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.TrimRight(strings.Join(kept, "\n"), "\n")
}

// extractQuickSummaryBlock 截取首个 QUICK SUMMARY 块（完整报告的尾部子集，
// §59.319 D3）。找不到锚=非完整报告形态，返回空。
// §59.319 P3 修复：全 playlist 报告含多块 QUICK SUMMARY（243 Alice 6 轨
// 被跨块累计成 18 实锤）——遇下一个块标记截断，只取首块。
func extractQuickSummaryBlock(text string) string {
	idx := strings.Index(text, "QUICK SUMMARY:")
	if idx < 0 {
		return ""
	}
	block := text[idx+len("QUICK SUMMARY:"):]
	if next := strings.Index(block, "QUICK SUMMARY:"); next >= 0 {
		block = block[:next]
	}
	return block
}

// splitBDFields 按 " / " 分割（quick summary 规范拼接）。
// 注意必须带两侧空格：音轨名内嵌斜杠标注（"Dolby TrueHD/Atmos Audio"——
// Mercy 实测）不能被切断。
func splitBDFields(s string) []string {
	parts := strings.Split(s, " / ")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// bdVideoCodecMap BDInfo 官方流名 → canonical（对齐 audio/video codec registry
// 值域；原盘用规范名 AVC/HEVC 而非 x264/x265——§59.235 P3 BDAV 门同语义）。
var bdVideoCodecMap = []struct{ name, canonical string }{
	{"MPEG-4 AVC Video", "AVC"},
	{"MPEG-H HEVC Video", "HEVC"},
	{"MPEG-2 Video", "MPEG-2"},
	{"SMPTE VC-1 Video", "VC-1"},
}

func videoCodecFromBD(field string) string {
	for _, m := range bdVideoCodecMap {
		if strings.Contains(field, m.name) {
			return m.canonical
		}
	}
	return ""
}

// bdAudioCodecMap BDInfo 官方音轨名 → canonical（audioCodecRegistry 值域：
// AAC ALAC APE AV3A DD DDP DSD DTS DTS-ES DTS-HD HR DTS-HD MA DTS:X FLAC
// LPCM MP2 MP3 Opus TrueHD WAV xHE-AAC）。最长名优先防 DTS/DTS-HD 前缀误吞。
var bdAudioCodecMap = []struct{ name, canonical string }{
	{"DTS-HD Master Audio", "DTS-HD MA"},
	{"DTS-HD High Resolution Audio", "DTS-HD HR"},
	{"DTS-ES Audio", "DTS-ES"},
	{"DTS-Express Audio", "DTS-HD HR"}, // DTS Express=低码率 HD HR 系（罕见，归族）
	{"DTS Audio", "DTS"},
	{"Dolby Digital Plus Audio", "DDP"},
	{"Dolby Digital Audio", "DD"},
	{"Dolby TrueHD Audio", "TrueHD"},
	{"LPCM Audio", "LPCM"},
	{"MPEG Audio", "MP2"},
}

func audioCodecFromBD(field string) string {
	// go-bdinfo 在音轨名内嵌标注：TrueHD Atmos 盘实测 "Dolby TrueHD/Atmos
	// Audio"（Mercy 样本）——剥离 "/Atmos" 后再匹配
	field = strings.ReplaceAll(field, "/Atmos", "")
	for _, m := range bdAudioCodecMap {
		if strings.Contains(field, m.name) {
			return m.canonical
		}
	}
	return ""
}

func isBDResolutionToken(f string) bool {
	if len(f) < 4 {
		return false
	}
	head, tail := f[:len(f)-1], f[len(f)-1:]
	if tail != "p" && tail != "i" {
		return false
	}
	for _, c := range head {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func isChannelToken(f string) bool {
	if len(f) != 3 {
		return false
	}
	return (f[0] >= '0' && f[0] <= '9') && f[1] == '.' && (f[2] >= '0' && f[2] <= '9')
}

// bitDepthFromBD "10 bits"/"8 bits" → "10bit"/"8bit"（MediaInfoTech canonical）。
func bitDepthFromBD(f string) string {
	lower := strings.ToLower(strings.TrimSpace(f))
	if strings.HasSuffix(lower, " bits") {
		if n := strings.TrimSpace(strings.TrimSuffix(lower, " bits")); n != "" {
			return n + "bit"
		}
	}
	return ""
}

func containsHDRToken(fields []string, token string) bool {
	for _, f := range fields {
		if strings.Contains(f, token) {
			return true
		}
	}
	return false
}

// hdrFromBD 主行 HDR + 全行 DoVi 组合 → canonical（对齐 hdrFromMI 值域：
// DoVi HDR10+/DoVi HDR/DoVi/HDR10+/HDR Vivid/HDR10/HLG/PQ10）。
// UHD 双层盘形态（Alice 实证）：主行 2160p HDR10 + "* "隐藏行 1080p Dolby
// Vision → "DoVi HDR"。
func hdrFromBD(mainFields []string, hasDoVi bool) string {
	lower := func(s string) string { return strings.ToLower(s) }
	hasHDR10Plus := containsHDRTokenLower(mainFields, "hdr10+", lower)
	hasHDR10 := containsHDRTokenLower(mainFields, "hdr10", lower)
	hasVivid := containsHDRTokenLower(mainFields, "hdr vivid", lower)
	hasHLG := containsHDRTokenLower(mainFields, "hlg", lower)
	hasPQ10 := containsHDRTokenLower(mainFields, "pq10", lower)

	switch {
	case hasDoVi && hasHDR10Plus:
		return "DoVi HDR10+"
	case hasDoVi && hasHDR10:
		return "DoVi HDR"
	case hasDoVi:
		return "DoVi"
	case hasHDR10Plus:
		return "HDR10+"
	case hasVivid:
		return "HDR Vivid"
	case hasHDR10:
		return "HDR10"
	case hasHLG:
		return "HLG"
	case hasPQ10:
		return "PQ10"
	}
	return ""
}

func containsHDRTokenLower(fields []string, token string, lower func(string) string) bool {
	for _, f := range fields {
		if strings.Contains(lower(f), token) {
			return true
		}
	}
	return false
}
