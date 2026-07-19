package titleparser

import (
	"regexp"
	"strings"
)

// TitleFormat 站点标题格式模板
// §56.19 决策 4: 加 MaxTitleLength + ReplacePatterns
type TitleFormat struct {
	Separator      string           `json:"separator"`       // 分隔符: "." 或 " "
	Order          []string         `json:"order"`           // 字段顺序
	GroupConnector string           `json:"group_connector"` // 制作组连接符: "-" (默认)
	StripChinese   bool             `json:"strip_chinese"`   // 是否去除中文
	Forbidden      []string         `json:"forbidden"`       // 禁止内容
	ChinesePrefix  bool             `json:"chinese_prefix"`  // 是否加 [中文名] 前缀
	Hook           string           `json:"_hook,omitempty"` // hook 名（复杂规则兜底)
	MaxTitleLength int              `json:"max_title_length,omitempty"` // §56.19: 标题截断（如 M-Team 255）
	ReplacePatterns []ReplacePattern `json:"replace_patterns,omitempty"` // §56.19: 字段值替换（如 1080p→FHD）
}

// ReplacePattern §56.19 决策 4: 字段值替换规则。
type ReplacePattern struct {
	From string `json:"from"` // 正则模式
	To   string `json:"to"`   // 替换文本
}

// Reassemble 将标题组件按目标站模板重组为标题字符串
func Reassemble(c TitleComponents, tf TitleFormat) string {
	// 如果有 hook 名，优先用 hook
	if tf.Hook != "" {
		if hook := GetSiteHook(tf.Hook); hook != nil {
			return hook(c, tf)
		}
	}
	return reassembleWithTemplate(c, tf)
}

// reassembleWithTemplate 模板方式重组
func reassembleWithTemplate(c TitleComponents, tf TitleFormat) string {
	if tf.Separator == "" {
		tf.Separator = " "
	}
	if tf.GroupConnector == "" {
		tf.GroupConnector = "-"
	}

	var parts []string

	for _, field := range tf.Order {
		val := getFieldValue(c, field)
		if val == "" {
			continue
		}
		if field == "group" {
			// 制作组直接附加到上一个 part（不加 separator）
			if len(parts) > 0 {
				parts[len(parts)-1] = parts[len(parts)-1] + tf.GroupConnector + val
			}
		} else {
			// 点分隔时，字段内的空格替换为点
			if tf.Separator == "." {
				val = strings.ReplaceAll(val, " ", ".")
			}
			parts = append(parts, val)
		}
	}

	result := strings.Join(parts, tf.Separator)

	// 清洗禁止内容
	for _, forbidden := range tf.Forbidden {
		result = strings.ReplaceAll(result, forbidden, "")
	}

	// 去除中文（如果要求）
	if tf.StripChinese {
		result = stripChineseChars(result)
	}

	// 清理多余分隔符
	result = cleanSeparators(result, tf.Separator)

	// 加中文名前缀
	if tf.ChinesePrefix && c.ChinesePrefix != "" {
		result = "[" + c.ChinesePrefix + "] " + result
	}

	// §56.19 决策 4: 字段值替换（如 1080p→FHD）
	for _, rp := range tf.ReplacePatterns {
		if rp.From == "" {
			continue
		}
		if re, err := regexp.Compile(rp.From); err == nil {
			result = re.ReplaceAllString(result, rp.To)
		}
	}

	// §56.19 决策 4: 标题截断（按 rune，避免中文截半）
	if tf.MaxTitleLength > 0 {
		if runes := []rune(result); len(runes) > tf.MaxTitleLength {
			result = string(runes[:tf.MaxTitleLength])
		}
	}

	return strings.TrimSpace(result)
}

func getFieldValue(c TitleComponents, field string) string {
	switch field {
	case "title":
		return c.MainTitle
	case "year":
		return c.Year
	case "season":
		return c.SeasonEpisode
	case "resolution":
		return c.Resolution
	case "medium":
		return c.Medium
	case "video_codec":
		return c.VideoCodec
	case "audio_codec":
		return c.AudioCodec
	case "hdr":
		return c.HDRFormat
	case "platform":
		return c.SourcePlatform
	case "bit_depth":
		return c.BitDepth
	case "frame_rate":
		return c.FrameRate
	case "group":
		return c.ReleaseGroup
	case "release_version":
		return c.ReleaseVersion
	default:
		return ""
	}
}

func stripChineseChars(s string) string {
	var result []rune
	for _, r := range s {
		if r >= 0x4e00 && r <= 0x9fff {
			continue
		}
		if r >= 0x3400 && r <= 0x4dbf {
			continue
		}
		if r >= 0xff00 && r <= 0xffef {
			continue // 全角符号
		}
		result = append(result, r)
	}
	return string(result)
}

func cleanSeparators(s, sep string) string {
	// 去除连续分隔符
	for strings.Contains(s, sep+sep) {
		s = strings.ReplaceAll(s, sep+sep, sep)
	}
	// 去除首尾分隔符
	s = strings.Trim(s, sep+" -.")
	// 分隔符前后的空格
	s = strings.ReplaceAll(s, " "+sep, sep)
	s = strings.ReplaceAll(s, sep+" ", sep)
	return strings.TrimSpace(s)
}

// DefaultTitleFormat 返回一个通用的标题模板（空格分隔）
func DefaultTitleFormat() TitleFormat {
	return TitleFormat{
		Separator: " ",
		Order: []string{"title", "year", "season", "resolution", "medium", "audio_codec", "video_codec", "group"},
		GroupConnector: "-",
		StripChinese: true,
		Forbidden: []string{"BDMV", "BDISO", "BDBOX", "DVDISO", ".mkv", ".mp4", ".iso"},
	}
}
