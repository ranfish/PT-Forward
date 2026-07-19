package model

import "encoding/json"

// §56.22 决策 1 — SiteTagConfig: 站点 tag 配置。
// Mode: taglist / checkbox_id / checkbox_span / independent_field
// Tags: standard_key → selector/value（mode=taglist 时可为空）
type SiteTagConfig struct {
	Mode string            `json:"mode"` // taglist / checkbox_id / checkbox_span / independent_field
	Tags map[string]string `json:"tags"` // standard_key → selector/value
}

// Tag 模式常量。
const (
	TagModeTaglist           = "taglist"           // 标准 NP tagList[] 数组
	TagModeCheckboxID        = "checkbox_id"       // PTer 类 checkbox（tag_gf/tag_jz/...）
	TagModeCheckboxSpan      = "checkbox_span"     // BTSchool span[] 数组
	TagModeIndependentField  = "independent_field" // HDRoute 独立 input name
)

// ParseTagConfig 从 JSON 字符串解析 SiteTagConfig。
// 空字符串返回空配置（Mode=taglist 默认）。
func ParseTagConfig(s string) *SiteTagConfig {
	s = trimSpaces(s)
	if s == "" {
		return &SiteTagConfig{Mode: TagModeTaglist}
	}
	var cfg SiteTagConfig
	if err := json.Unmarshal([]byte(s), &cfg); err != nil {
		return &SiteTagConfig{Mode: TagModeTaglist}
	}
	if cfg.Mode == "" {
		cfg.Mode = TagModeTaglist
	}
	if cfg.Tags == nil {
		cfg.Tags = make(map[string]string)
	}
	return &cfg
}

// Serialize 序列化为 JSON 字符串（用于存入 sites.tag_config）。
func (c *SiteTagConfig) Serialize() string {
	data, err := json.Marshal(c)
	if err != nil {
		return ""
	}
	return string(data)
}

func trimSpaces(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\n') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t' || s[len(s)-1] == '\n') {
		s = s[:len(s)-1]
	}
	return s
}
