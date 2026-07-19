// Package description 感谢引言生成（§56.20 决策 2）。
//
// 借鉴 easy-upload，全局配置 + 每站模板覆盖。
// 默认中文站: "转自[b]{source_site}[/b]，感谢原发布者！"
// 默认英文站: "Torrent from [b]{source_site}[/b]. All thanks to the original uploader!"
package description

import "strings"

// ThanksQuoteConfig 感谢引言配置。
type ThanksQuoteConfig struct {
	Enabled         bool   // 全局开关
	ChineseTemplate string // 中文站模板（空=用默认）
	EnglishTemplate string // 英文站模板（空=用默认）
	SiteTemplate    string // per-site 覆盖（空=用全局）
}

// 默认模板常量。
const (
	DefaultChineseThanksQuote = "转自[b]{source_site}[/b]，感谢原发布者！"
	DefaultEnglishThanksQuote = "Torrent from [b]{source_site}[/b].\nAll thanks to the original uploader!"

	// ThanksQuotePlaceholder 源站名占位符。
	ThanksQuotePlaceholder = "{source_site}"
)

// GenerateThanksQuote 生成感谢引言。
//   sourceSite: 源站名
//   isEnglishSite: 是否英文站（true=用英文模板）
//   config: 配置（nil=默认配置，Enabled=true）
// 返回空字符串表示禁用或模板为空。
func GenerateThanksQuote(sourceSite string, isEnglishSite bool, config *ThanksQuoteConfig) string {
	if config != nil && !config.Enabled {
		return ""
	}

	template := selectTemplate(isEnglishSite, config)
	if template == "" {
		return ""
	}

	return strings.ReplaceAll(template, ThanksQuotePlaceholder, sourceSite)
}

// selectTemplate 按优先级选择模板（SiteTemplate > 全局对应语言模板 > 默认模板）。
func selectTemplate(isEnglishSite bool, config *ThanksQuoteConfig) string {
	if config != nil && config.SiteTemplate != "" {
		return config.SiteTemplate
	}
	if isEnglishSite {
		if config != nil && config.EnglishTemplate != "" {
			return config.EnglishTemplate
		}
		return DefaultEnglishThanksQuote
	}
	if config != nil && config.ChineseTemplate != "" {
		return config.ChineseTemplate
	}
	return DefaultChineseThanksQuote
}
