// Package extract 工具签名识别 helper（§56.10 借鉴 PTNexus review_extract.go:624-715）。
//
// 初始版本：覆盖中文 PT 圈常见工具签名模式。
// TODO(2b.9): 用黄金样本（88 个真实站点 HTML）验证并扩展清单。
package extract

import (
	"regexp"
	"strings"
)

// toolSignatureBlacklist 工具签名黑名单（命中任一 → 视为工具签名 quote 块）。
// 仅含工具名/编码器签名，不含声明文本（声明文本由 IsAcknowledgmentQuote 处理，保留为 Statement）。
// 后续 2b.9 用真实样本验证扩展（对齐 PTNexus 生产清单）。
var toolSignatureBlacklist = []string{
	// 工具名签名
	"CSAUTO",
	"ARDTU", // 不以 By 开头的 ARDTU 引用（By ARDTU@... 由 IsByARDTUWithGroupInfo 优先处理）
	"NEXUS 发布器",
	"NEXUS发布器",
	"PThome",
	"btschool",
	// 编码器签名
	"Encoder by",
	"Encoded by",
	"Ripped by",
	"压制by",
}

// techParamAndGroups 技术参数 AND 匹配组。
// 一组内所有关键字同时命中 → 视为技术参数块（MediaInfo 伪装成 quote）。
// 初始版本 ~10 组，后续 2b.9 扩展到 14 组。
var techParamAndGroups = [][]string{
	{"Format", "Bit rate", "Writing library"},
	{"Unique ID", "Complete name", "File size"},
	{"Format profile", "Codec ID"},
	{"Writing application", "Encoding settings"},
	{"Scan type", "Color space", "Bit depth"},
	{"Channel(s)", "Sampling rate"},
	{"Frame rate", "Resolution"},
	{"Stream size", "Language"},
	{"Duration", "Bit rate mode"},
	{"Frame count", "Color primaries"},
}

// byARDTUPrefixRe 匹配 "By ARDTU@GroupName" 前缀。
// 示例: "By ARDTU@FRDS: 正文内容" → 剥离前缀 → "正文内容"。
var byARDTUPrefixRe = regexp.MustCompile(`(?i)^\s*by\s+ARDTU(?:\s*@?\s*[\w-]+)?\s*[:：]?\s*`)

// IsToolSignatureQuote 判断 quote 块是否为工具签名（命中黑名单任一关键字）。
func IsToolSignatureQuote(text string) bool {
	for _, kw := range toolSignatureBlacklist {
		if strings.Contains(text, kw) {
			return true
		}
	}
	return false
}

// IsTechParamsQuote 判断 quote 块是否为技术参数块（某 AND 组全部命中）。
func IsTechParamsQuote(text string) bool {
	for _, group := range techParamAndGroups {
		allMatch := true
		for _, kw := range group {
			if !strings.Contains(text, kw) {
				allMatch = false
				break
			}
		}
		if allMatch {
			return true
		}
	}
	return false
}

// IsByARDTUWithGroupInfo 判断 quote 块是否以 "By ARDTU@..." 开头。
func IsByARDTUWithGroupInfo(text string) bool {
	trimmed := strings.TrimSpace(text)
	loc := byARDTUPrefixRe.FindStringIndex(trimmed)
	return loc != nil && loc[0] == 0
}

// StripByARDTUPrefix 剥离 "By ARDTU@GroupName" 前缀，保留剩余正文。
// 若剥离后正文为空，返回空字符串（调用方据此判断是否丢弃整个 quote）。
func StripByARDTUPrefix(text string) string {
	trimmed := strings.TrimSpace(text)
	stripped := byARDTUPrefixRe.ReplaceAllString(trimmed, "")
	return strings.TrimSpace(stripped)
}

// IsAcknowledgmentQuote 判断 quote 块是否为官组声明（始终保留为 Statement）。
// 命中"官组/感谢/原制作者/字幕组"或文本较短（< 200 rune）视为声明。
func IsAcknowledgmentQuote(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	// 短文本通常为声明
	if len([]rune(trimmed)) < 200 {
		return true
	}
	// 命中声明关键词
	ackKeywords := []string{"官组", "感谢", "原制作者", "字幕组", "声明", "免责", "仅供"}
	for _, kw := range ackKeywords {
		if strings.Contains(trimmed, kw) {
			return true
		}
	}
	return false
}
