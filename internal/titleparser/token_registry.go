// Package titleparser Token Registry（§59.27 P1）。
//
// v1.05 标题规范的技术 token 单一注册表：每个 token 定义
//   - Canonical：TechProfile 内部值（解析产物、比较值、MI 映射目标）
//   - TitleForm：重组标题的点分隔形式（发布产物）
//   - Pattern：匹配标题中所有变体（点/空格/连字符分隔）
//
// 消费方（全部从 registry 派生，禁止再手写变体）：
//   - extractVideoCodec/extractAudio：首个命中（registry 顺序即优先级）
//   - removeVideoCodecTokens/removeAudioCodecTokens：从 Pattern 合成移除正则
//   - mediainfo_extractor codecFromMI/audioFromMI：MI 值 → Canonical
//   - ReassembleFromTechProfile：Canonical → TitleForm
//
// 核心不变量（round-trip，token_registry_test.go 持续护航）：
//   parse(reassemble(profile)) == profile
package titleparser

import "regexp"

// TokenDef 单个技术 token 定义。
type TokenDef struct {
	Canonical string // TechProfile 内部值（如 "DTS-HD MA"）
	TitleForm string // 重组标题形式（如 "DTS-HD.MA"），空 = 与 Canonical 相同
	Pattern   string // 变体匹配正则（不含前后缀锚定，编译见下方缓存）
}

// titleForm 返回重组形式（空则用 Canonical）。
func (t TokenDef) titleForm() string {
	if t.TitleForm != "" {
		return t.TitleForm
	}
	return t.Canonical
}

// compileToken 编译 token pattern（长 token 在前由 registry 顺序保证，
// 此处只做词边界包裹）。flagI 大小写不敏感。
var tokenReCache = map[string]*regexp.Regexp{}

func (t TokenDef) re() *regexp.Regexp {
	if re, ok := tokenReCache[t.Pattern]; ok {
		return re
	}
	re := regexp.MustCompile(`(?i)` + t.Pattern)
	tokenReCache[t.Pattern] = re
	return re
}

// === 视频编码（v1.05 字段 12，顺序即提取优先级：AV1 > VP9 > AVS2 > x265 > HEVC > x264 > AVC > VC-1 > MPEG-2）===

var videoCodecRegistry = []TokenDef{
	{Canonical: "AV1", Pattern: `\bAV1\b`},
	{Canonical: "VP9", Pattern: `\bVP[89]\b`},
	{Canonical: "AVS2", Pattern: `\bAVS2\b`},
	{Canonical: "AVS+", Pattern: `\bAVS\+`},
	// §59.27 P3：值域演进（对齐 standard_keys）
	{Canonical: "VVC", Pattern: `\bVVC\b|\bH[._\- ]?266\b`},
	{Canonical: "Xvid", Pattern: `\bXvid\b|\bXviD\b`},
	{Canonical: "x265", Pattern: `\bX265\b`},
	// H.265 / H265 / HEVC
	{Canonical: "HEVC", Pattern: `\bH[._\- ]?265\b|\bHEVC\b`},
	{Canonical: "x264", Pattern: `\bX264\b`},
	{Canonical: "AVC", Pattern: `\bH[._\- ]?264\b|\bAVC\b`},
	{Canonical: "VC-1", Pattern: `\bVC[._\- ]?1\b`},
	{Canonical: "MPEG-2", Pattern: `\bMPEG[._\- ]?2\b`},
}

// === 音频编码（v1.05 字段 13，顺序即提取优先级：特例在前）===

var audioCodecRegistry = []TokenDef{
	{Canonical: "TrueHD", Pattern: `\bTrue[._\- ]?HD\b|\bMLP FBA\b`},
	{Canonical: "DTS:X", Pattern: `\bDTS[._\- :]?[Xx]\b`},
	// DTS XLL X = DTS:X 的 MI 内部名，按 v1.05 规范 DTS:X 不标注 → 归 DTS。
	// 必须在 DTS-HD MA（DTS XLL）之前判定。
	{Canonical: "DTS", Pattern: `\bDTS[._\- ]?XLL[._\- ]?X\b`},
	// DTS-HD MA / DTS HD MA / DTS-HD.MA / DTSHDMA / DTS XLL（MI 内部名）
	{Canonical: "DTS-HD MA", TitleForm: "DTS-HD.MA", Pattern: `\bDTS[._\- ]?HD[._\- ]*MA\b|\bDTS[._\- ]?XLL\b`},
	{Canonical: "DTS-HD HR", TitleForm: "DTS-HD.HR", Pattern: `\bDTS[._\- ]?HD[._\- ]*HR\b|\bDTS[._\- ]?LBR\b`},
	{Canonical: "DTS-ES", TitleForm: "DTS-ES", Pattern: `\bDTS[._\- ]?ES\b`},
	// DTS-UHD 是 DTS:X 的 UHD 变体，归 DTS
	{Canonical: "DTS", Pattern: `\bDTS\b|\bDTS[._\- ]?UHD\b`},
	// E-AC-3 / EAC3 / DDP / DD+ / DDPlus（含声道后缀 DDP5.1）
	{Canonical: "DDP", Pattern: `\bE[._\- ]?AC[._\- ]?3\b|\bDDP5?[._\d]*|\bDD\+|\bDDPlus\b`},
	{Canonical: "DD", Pattern: `\bDD\b|\bAC[._\- ]?3\b`},
	{Canonical: "FLAC", Pattern: `\bFLAC\b`},
	// xHE-AAC（USAC）必须在 AAC 之前（AAC 是其子串）
	{Canonical: "xHE-AAC", Pattern: `\bx[._\- ]?HE[._\- ]?AAC\b|\bUSAC\b`},
	{Canonical: "AAC", Pattern: `\bAAC(?:[._\d]+)?\b`},
	{Canonical: "ALAC", Pattern: `\bALAC\b`},
	{Canonical: "APE", Pattern: `\bAPE\b`},
	{Canonical: "WAV", Pattern: `\bWAV\b`},
	{Canonical: "Opus", TitleForm: "Opus", Pattern: `\bOpus\b|\bOPUS\b`},
	{Canonical: "MP3", Pattern: `\bMP3\b`},
	{Canonical: "LPCM", Pattern: `\bLPCM\b|\bPCM\b`},
	// §59.27 P2：音乐/新编码演进（对齐 MI 侧值域）
	{Canonical: "MP2", Pattern: `\bMP2\b|\bMPEG[._\- ]?Audio\b`},
	{Canonical: "AV3A", Pattern: `\bAV3A\b`},
	// §59.27 P3：值域演进（对齐 standard_keys）
	{Canonical: "DSD", Pattern: `\bDSD\b`},
}

// === HDR（v1.05 字段 10，组合值由 extractHDRFormat 合成，此处为原子 token）===

var hdrTokenRegistry = []TokenDef{
	{Canonical: "Dolby Vision", TitleForm: "DoVi", Pattern: `\bDolby[._\- ]?Vision\b|\bDoVi\b|\bDV\b`},
	{Canonical: "HDR10+", Pattern: `\bHDR10\+?\b`},
	{Canonical: "HDR10", Pattern: `\bHDR10\b`},
	{Canonical: "HDR Vivid", TitleForm: "HDRVivid", Pattern: `\bHDR[._\- ]?Vivid\b|\bHDRVivid\b`},
	{Canonical: "HLG", Pattern: `\bHLG\b`},
	{Canonical: "HDR", Pattern: `\bHDR\b`},
	{Canonical: "SDR", Pattern: `\bSDR\b`},
	{Canonical: "PQ10", Pattern: `\bPQ10\b`},
}

// lookupToken 从 registry 找首个命中 token（顺序即优先级）。
func lookupToken(registry []TokenDef, s string) string {
	for _, t := range registry {
		if t.re().MatchString(s) {
			return t.Canonical
		}
	}
	return ""
}

// titleFormOfToken canonical → 重组形式（§59.27 P3 重组显式出口）。
// registry 未收录的值原样返回（EditionInfo 等自由文本不经过 registry）。
func titleFormOfToken(registry []TokenDef, canonical string) string {
	if canonical == "" {
		return ""
	}
	for _, t := range registry {
		if t.Canonical == canonical {
			return t.titleForm()
		}
	}
	return canonical
}

// AudioTitleForm / VideoTitleForm / HDRTitleForm 对外出口（重组器使用）。
func AudioTitleForm(canonical string) string { return titleFormOfToken(audioCodecRegistry, canonical) }
func VideoTitleForm(canonical string) string { return titleFormOfToken(videoCodecRegistry, canonical) }

// removeAllTokenPatterns 从 registry 合成移除正则（替代手写 remove*Tokens）。
// 返回移除所有变体后的字符串。
func removeAllTokenPatterns(registry []TokenDef, s string) string {
	for _, t := range registry {
		s = t.re().ReplaceAllString(s, " ")
	}
	return s
}
