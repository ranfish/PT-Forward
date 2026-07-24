package titleparser

import (
	"regexp"
	"strconv"
	"strings"
)

// ReassembleFromTechProfile 从 TechProfile 按目标站模板重组标题（§56.34 步骤 3）。
//
// 与 Reassemble(TitleComponents) 的区别：
//   - 支持全部 v1.05 字段（edition_info/hdr/audio_channels/audio_technology 等）
//   - "medium" 字段从 SourceType + Specification 复合拼接
//   - "audio_full" 字段从 AudioCodec + AudioTechnology + AudioChannels + AudioTracks 复合拼接
//   - "bit_depth" 字段按 v1.05 规则（仅 AVC 系 10bit 输出 Hi10P）
//
// 不支持 site hook（hook 绑定 TitleComponents，如需 hook 继续用 Reassemble）。
func ReassembleFromTechProfile(p TechProfile, tf TitleFormat) string {
	if tf.SkipReassemble {
		return ""
	}
	// 音乐类检测：Artist 非空时用音乐范式重组（§56.35 阶段 3）
	if p.Artist != "" {
		return reassembleMusic(p, tf)
	}
	// 按分类切换 paradigm（§56.35 阶段 3：ZMPT 电影 dot / 电视剧 space）
	if tf.Category != "" && len(tf.Paradigms) > 0 {
		if paradigm, ok := tf.Paradigms[tf.Category]; ok {
			tf.Paradigm = paradigm
		}
	}
	applyParadigm(&tf)
	if tf.Separator == "" {
		tf.Separator = " "
	}
	if tf.GroupConnector == "" {
		tf.GroupConnector = "-"
	}

	var parts []string
	for _, field := range tf.Order {
		val := getFieldValueFromTechProfile(p, field, tf)
		if val == "" {
			continue
		}
		if field == "group" {
			if len(parts) > 0 {
				parts[len(parts)-1] = parts[len(parts)-1] + tf.GroupConnector + val
			}
		} else {
			if tf.Separator == "." {
				val = strings.ReplaceAll(val, " ", ".")
			}
			parts = append(parts, val)
		}
	}

	result := strings.Join(parts, tf.Separator)

	if tf.RemoveSourcePrefix && !tf.ChinesePrefix {
		result = removeSourcePrefix(result)
	}
	for _, forbidden := range tf.Forbidden {
		result = strings.ReplaceAll(result, forbidden, "")
	}
	if tf.StripChinese {
		result = normalizeHalfWidth(result)
		result = stripChineseChars(result)
	}
	result = cleanSeparators(result, tf.Separator)
	if tf.ChinesePrefix && p.ChinesePrefix != "" {
		result = "[" + p.ChinesePrefix + "] " + result
	}
	if tf.BracketsForbidden {
		result = strings.ReplaceAll(result, "[", "")
		result = strings.ReplaceAll(result, "]", "")
	}
	if tf.ParensForbidden {
		result = strings.ReplaceAll(result, "(", "")
		result = strings.ReplaceAll(result, ")", "")
	}
	for _, rp := range tf.ReplacePatterns {
		if rp.From == "" {
			continue
		}
		if re, err := regexp.Compile(rp.From); err == nil {
			result = re.ReplaceAllString(result, rp.To)
		}
	}
	if tf.MaxTitleLength > 0 {
		if runes := []rune(result); len(runes) > tf.MaxTitleLength {
			result = string(runes[:tf.MaxTitleLength])
		}
	}

	return strings.TrimSpace(result)
}

// V105TitleFormat 返回 v1.05 完整顺序的 TitleFormat（17 字段 + group）。
func V105TitleFormat() TitleFormat {
	return TitleFormat{
		Separator: " ",
		Order: []string{
			"title", "season", "year", "edition_info",
			"resolution", "region_code", "platform",
			"source_type", "specification",
			"hdr", "bit_depth",
			"video_codec",
			"audio_codec", "audio_channels", "audio_technology", "audio_tracks",
			"group",
		},
		GroupConnector: "-",
		StripChinese:   true,
	}
}

// getFieldValueFromTechProfile 按 title_format 字段名从 TechProfile 取值。
// 应用 func_overrides 格式化（codecStyle/normalizeResolution/normalizeAudio 等）。
func getFieldValueFromTechProfile(p TechProfile, field string, tf TitleFormat) string {
	switch field {
	case "title":
		return p.MainTitle
	case "year":
		return p.Year
	case "season":
		return p.SeasonEpisode
	case "edition_info":
		return p.EditionInfo
	case "resolution":
		return normalizeResolution(p.Resolution, tf.ResolutionCase)
	case "region_code":
		return p.RegionCode
	case "platform":
		return p.SourcePlatform
	case "source_type":
		return p.SourceType
	case "specification":
		return p.Specification
	case "medium":
		return normalizeMedium(composeMedium(p))
	case "hdr":
		return normalizeHDR(p.HDR, tf.HDR10ToHDR)
	case "bit_depth":
		return formatBitDepthForTitle(p)
	case "video_codec":
		return codecStyle(p.VideoCodec, composeMedium(p))
	case "audio_codec":
		return normalizeAudio(p.AudioCodec)
	case "audio_full":
		return composeAudio(p)
	case "audio_channels":
		if shouldOmitChannels(p.AudioCodec, p.AudioChannels) {
			return ""
		}
		return p.AudioChannels
	case "audio_technology":
		return p.AudioTechnology
	case "audio_tracks":
		return audioCountWord(p.AudioTracks, tf.AudioCountSuffix)
	case "group":
		return p.ReleaseGroup
	case "release_version":
		return p.ReleaseVersion
	default:
		return ""
	}
}

// composeMedium 从 SourceType + Specification 拼接媒介组合值。
// 当 SourceType + Specification 均为空时 fallback 到过渡字段 Medium（兼容旧数据）。
func composeMedium(p TechProfile) string {
	var parts []string
	if p.SourceType != "" {
		parts = append(parts, p.SourceType)
	}
	if p.Specification != "" {
		parts = append(parts, p.Specification)
	}
	composed := strings.Join(parts, " ")
	if composed != "" {
		return composed
	}
	return p.Medium
}

// composeAudio 按 v1.05 格式拼接完整音频串（编码 Atmos 声道 音轨数）。
//
// 省略规则：
//   - AAC/MP2/MP3 且声道=2.0 → 省略声道
//   - Atmos 在声道前（v1.05 实例：TrueHD Atmos 7.1）
//   - 音轨数 > 1 时输出（如 2Audios）
func composeAudio(p TechProfile) string {
	if p.AudioCodec == "" {
		return ""
	}
	var parts []string
	parts = append(parts, p.AudioCodec)

	if p.AudioTechnology == "Atmos" {
		parts = append(parts, "Atmos")
	}

	if p.AudioChannels != "" {
		omit := shouldOmitChannels(p.AudioCodec, p.AudioChannels)
		if !omit {
			parts = append(parts, p.AudioChannels)
		}
	}

	if p.AudioTracks > 1 {
		parts = append(parts, strconv.Itoa(p.AudioTracks)+"Audios")
	}

	return strings.Join(parts, " ")
}

// shouldOmitChannels AAC/MP2/MP3 且 2.0 时省略声道（v1.05 :188）。
func shouldOmitChannels(codec, channels string) bool {
	upper := strings.ToUpper(codec)
	if (upper == "AAC" || upper == "MP2" || upper == "MP3") && channels == "2.0" {
		return true
	}
	return false
}

// formatBitDepthForTitle v1.05 :146-148：仅 AVC 系 10bit 输出 Hi10P（编码前）。
func formatBitDepthForTitle(p TechProfile) string {
	if p.BitDepth != "10bit" {
		return ""
	}
	upper := strings.ToUpper(p.VideoCodec)
	if strings.Contains(upper, "AVC") || strings.Contains(upper, "X264") ||
		strings.Contains(upper, "H.264") || strings.Contains(upper, "H264") {
		return "Hi10P"
	}
	return ""
}

// formatAudioTracksStr 音轨数 → 标题字符串（0/1 省略，≥2 输出 XAudios）。
// §56.35: 被 audioCountWord 替代（支持 audio_count_suffix 站点覆盖），保留向后兼容。
func formatAudioTracksStr(n int) string {
	return audioCountWord(n, "")
}

// codecStyle 按媒介上下文转换视频编码写法（§56.35 阶段 1，v1.05 :150-160）。
//
// 原盘/Remux → AVC / HEVC
// WEB-DL     → H.264 / H.265
// HDTV       → H264 / H265（无点）
// 压制       → x264 / x265
//
// Writing Library 明确有 x264/x265 时（压制编码器），WEB/HDTV 也可写 x264/x265。
func codecStyle(codec, medium string) string {
	upper := strings.ToUpper(codec)
	family := ""
	switch {
	case strings.Contains(upper, "HEVC") || strings.Contains(upper, "H.265") ||
		strings.Contains(upper, "H265") || strings.Contains(upper, "X265"):
		family = "h265"
	case strings.Contains(upper, "AVC") || strings.Contains(upper, "H.264") ||
		strings.Contains(upper, "H264") || strings.Contains(upper, "X264"):
		family = "h264"
	default:
		return codec // AV1/VP9/MPEG-2 等非 H.264/H.265 族，原样返回
	}

	// x264/x265 是压制编码器，直接返回（不转换）
	if strings.Contains(upper, "X264") || strings.Contains(upper, "X265") {
		if family == "h264" {
			return "x264"
		}
		return "x265"
	}

	switch classifyMediumForCodec(medium) {
	case "disc":
		if family == "h264" {
			return "AVC"
		}
		return "HEVC"
	case "web":
		if family == "h264" {
			return "H.264"
		}
		return "H.265"
	case "hdtv":
		if family == "h264" {
			return "H264"
		}
		return "H265"
	default:
		return codec // 无法判断媒介，原样返回
	}
}

// classifyMediumForCodec 从媒介字符串推断编码写法分类。
func classifyMediumForCodec(medium string) string {
	upper := strings.ToUpper(medium)
	if strings.Contains(upper, "WEB-DL") || strings.Contains(upper, "WEBDL") {
		return "web"
	}
	if strings.Contains(upper, "WEBRIP") || strings.Contains(upper, "BDRIP") || strings.Contains(upper, "ENCODE") {
		return "encode"
	}
	if strings.Contains(upper, "UHDTV") || strings.Contains(upper, "HDTV") {
		return "hdtv"
	}
	if strings.Contains(upper, "REMUX") || strings.Contains(upper, "BLU-RAY") || strings.Contains(upper, "BLURAY") {
		return "disc"
	}
	return ""
}

// normalizeResolution 分辨率大小写（§56.35：DuckBooBee 要求大写 P）。
// 默认转为小写（v1.05 规范：p 不写 P）。upper 模式只转 p→P，不影响 i。
func normalizeResolution(res, caseMode string) string {
	res = strings.ToLower(res)
	if caseMode == "upper" && strings.HasSuffix(res, "p") {
		return res[:len(res)-1] + "P"
	}
	return res
}

// normalizeAudio 音频编码归一化（§56.35：AC3→DD, E-AC3→DDP）。
func normalizeAudio(codec string) string {
	upper := strings.ToUpper(codec)
	switch {
	case strings.Contains(upper, "E-AC-3") || strings.Contains(upper, "EAC3") ||
		strings.Contains(upper, "DDP") || strings.Contains(upper, "DD+"):
		return "DDP"
	case strings.Contains(upper, "AC-3") || strings.Contains(upper, "AC3"):
		return "DD"
	default:
		return codec
	}
}

// normalizeHDR HDR 格式归一化（§56.35：青蛙 HDR10→HDR 泛指）。
func normalizeHDR(hdr string, hdr10ToHDR bool) string {
	if hdr10ToHDR && hdr == "HDR10" {
		return "HDR"
	}
	return hdr
}

// normalizeMedium 媒介归一化（预留扩展点）。
func normalizeMedium(medium string) string {
	return medium
}

// audioCountWord 音轨数 → 标题字符串（§56.35：支持 audio_count_suffix 站点覆盖）。
func audioCountWord(n int, suffix string) string {
	if n <= 1 {
		return ""
	}
	if suffix == "none" {
		return strconv.Itoa(n) + "Audio"
	}
	return strconv.Itoa(n) + "Audios"
}

// applyParadigm 根据范式自动推断 separator（§56.35 阶段 2）。
//
// 仅当 Separator 未显式配置时生效。StripChinese/ChinesePrefix 仍由站点显式配置。
//
//	dot              → "."
//	space            → " "
//	chinese_prefixed → " "
func applyParadigm(tf *TitleFormat) {
	if tf.Paradigm == "" || tf.Separator != "" {
		return
	}
	switch tf.Paradigm {
	case "dot":
		tf.Separator = "."
	case "space", "chinese_prefixed":
		tf.Separator = " "
	}
}

// reSourceTagPrefix 匹配开头的 [xxx] 源站标签前缀（1-15 字符）。
var reSourceTagPrefix = regexp.MustCompile(`^\[[^\]]{1,15}\]\s*`)

// removeSourcePrefix 移除标题开头的 [站名] 源站标签前缀（§56.35 阶段 2）。
func removeSourcePrefix(s string) string {
	return reSourceTagPrefix.ReplaceAllString(s, "")
}

// normalizeHalfWidth 将全角字符归一化为半角 ASCII（§56.35 阶段 3：片名标点保护）。
//
// §56.26 规则 #50：全角标点 → 半角（：→: ，→, ！→! 等）。
// 在 stripChineseChars 之前执行，确保全角标点被转换为半角（而非删除）。
//
//	0xFF01-0xFF5E（全角字母数字标点）→ 减 0xFEE0 → 0x21-0x7E（半角）
//	0x3000（全角空格）→ 0x20（半角空格）
func normalizeHalfWidth(s string) string {
	var result []rune
	for _, r := range s {
		switch {
		case r >= 0xFF01 && r <= 0xFF5E:
			result = append(result, r-0xFEE0)
		case r == 0x3000:
			result = append(result, ' ')
		default:
			result = append(result, r)
		}
	}
	return string(result)
}
