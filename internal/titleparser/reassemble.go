package titleparser

import (
	"strings"
)

// TitleFormat 站点标题格式模板
// §56.19 决策 4: 加 MaxTitleLength + ReplacePatterns
// §56.35 阶段 1: 加 func_overrides 字段
type TitleFormat struct {
	Separator      string           `json:"separator"`                   // 分隔符: "." 或 " "
	Order          []string         `json:"order"`                       // 字段顺序
	GroupConnector string           `json:"group_connector"`             // 制作组连接符: "-" (默认) / "." (SpringSunday)
	StripChinese   bool             `json:"strip_chinese"`               // 是否去除中文
	Forbidden      []string         `json:"forbidden"`                   // 禁止内容
	ChinesePrefix  bool             `json:"chinese_prefix"`              // 是否加 [中文名] 前缀
	MaxTitleLength int              `json:"max_title_length,omitempty"`  // 标题截断（如 M-Team 255）
	ReplacePatterns []ReplacePattern `json:"replace_patterns,omitempty"` // 字段值替换（如 1080p→FHD）

	// §56.35 func_overrides（站点级格式化覆盖）
	ResolutionCase   string `json:"resolution_case,omitempty"`    // "lower"(默认) / "upper"(DuckBooBee 大写 P)
	HDR10ToHDR       bool   `json:"hdr10_to_hdr,omitempty"`       // HDR10→HDR 泛指（青蛙）
	AudioCountSuffix string `json:"audio_count_suffix,omitempty"` // "s"(默认 4Audios) / "none"(HDArea 4Audio)

	// §56.35 阶段 2：范式 + 后处理
	Paradigm           string `json:"paradigm,omitempty"`            // "dot"/"space"/"chinese_prefixed"（为空时用显式 separator）
	BracketsForbidden  bool   `json:"brackets_forbidden,omitempty"`  // 禁止 []（PTLGS）
	ParensForbidden    bool   `json:"parens_forbidden,omitempty"`    // 禁止 ()（PTerClub）
	RemoveSourcePrefix bool   `json:"remove_source_prefix,omitempty"` // 移除 [站名] 前缀
}

// ReplacePattern §56.19 决策 4: 字段值替换规则。
type ReplacePattern struct {
	From string `json:"from"` // 正则模式
	To   string `json:"to"`   // 替换文本
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
