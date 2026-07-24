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
	if tf.Separator == "" {
		tf.Separator = " "
	}
	if tf.GroupConnector == "" {
		tf.GroupConnector = "-"
	}

	var parts []string
	for _, field := range tf.Order {
		val := getFieldValueFromTechProfile(p, field)
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

	for _, forbidden := range tf.Forbidden {
		result = strings.ReplaceAll(result, forbidden, "")
	}
	if tf.StripChinese {
		result = stripChineseChars(result)
	}
	result = cleanSeparators(result, tf.Separator)
	if tf.ChinesePrefix && p.ChinesePrefix != "" {
		result = "[" + p.ChinesePrefix + "] " + result
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
func getFieldValueFromTechProfile(p TechProfile, field string) string {
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
		return p.Resolution
	case "region_code":
		return p.RegionCode
	case "platform":
		return p.SourcePlatform
	case "source_type":
		return p.SourceType
	case "specification":
		return p.Specification
	case "medium":
		return composeMedium(p)
	case "hdr":
		return p.HDR
	case "bit_depth":
		return formatBitDepthForTitle(p)
	case "video_codec":
		return p.VideoCodec
	case "audio_codec":
		return p.AudioCodec
	case "audio_full":
		return composeAudio(p)
	case "audio_channels":
		return p.AudioChannels
	case "audio_technology":
		return p.AudioTechnology
	case "audio_tracks":
		return formatAudioTracksStr(p.AudioTracks)
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
func formatAudioTracksStr(n int) string {
	if n > 1 {
		return strconv.Itoa(n) + "Audios"
	}
	return ""
}
