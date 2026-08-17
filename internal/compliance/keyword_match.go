package compliance

import (
	"regexp"
	"strings"
	"sync"
)

// §56.2x 修正：关键词层防误伤匹配（对齐 qui 正则层 XXX 词边界先例）。
//
// 设计依据（docs/31 §56.2x qui 成人检测移植节）：
//   - qui 层 XXX 已用 \bxxx\b 词边界 + isBenignXXX 排除
//   - 关键词层（compliance_rules）设计为子串匹配，未考虑 ASCII 关键词
//     子串误伤（"Porn" ⊂ "The Pornographers"，v0.0.642 实证）
//
// 修复策略：
//   - 纯 ASCII 关键词 → 词边界正则（(?:^|[^\w])KW(?:$|[^\w])，大小写不敏感，
//     缓存编译）。[^\w] 而非 \s：点分隔/连字符分隔（Movie.Porn.2024）保持命中。
//     \w 含数字： Porn123 / 123Porn 不命中（连写规避面不变，qui 正则层兜底）
//   - 含非 ASCII（中文/全角）→ 保持子串（中文无词边界概念，走排除词路线）
//   - 中文误报排除（设计已定未实现，本次补齐）：成人+教育/高考/大学/学院 放行

var (
	kwMu    sync.RWMutex
	kwRe    = map[string]*regexp.Regexp{}
)

// chineseFalsePositivePatterns §56.2x 设计稿：中文关键词误报排除对。
var chineseFalsePositivePatterns = []struct {
	keyword   string
	exclusion string
}{
	{"成人", "教育"},
	{"成人", "高考"},
	{"成人", "大学"},
	{"成人", "学院"},
}

// isASCIIKeyword 判定关键词是否纯 ASCII（英文/数字/符号）。
func isASCIIKeyword(s string) bool {
	for _, r := range s {
		if r > 127 {
			return false
		}
	}
	return s != ""
}

// keywordBoundaryRe ASCII 关键词词边界正则（缓存）。
func keywordBoundaryRe(kw string) *regexp.Regexp {
	kwMu.RLock()
	if re, ok := kwRe[kw]; ok {
		kwMu.RUnlock()
		return re
	}
	kwMu.RUnlock()
	re := regexp.MustCompile(`(?i)(?:^|[^\w])` + regexp.QuoteMeta(kw) + `(?:$|[^\w])`)
	kwMu.Lock()
	kwRe[kw] = re
	kwMu.Unlock()
	return re
}

// matchKeyword 单个关键词对文本的防误伤匹配。
// text原文 + textLower 由调用方预计算（避免循环内重复 ToLower）。
func matchKeyword(kw, text, textLower string) bool {
	if kw == "" || text == "" {
		return false
	}
	if isASCIIKeyword(kw) {
		return keywordBoundaryRe(kw).MatchString(text)
	}
	// 中文关键词：子串（原文或大小写不敏感——关键词可能含英文混排如 "9KG站"）
	return strings.Contains(text, kw) || strings.Contains(textLower, strings.ToLower(kw))
}

// isChineseFalsePositive 中文关键词误报排除：命中文本含排除词则放行。
// 对齐设计稿 chineseFalsePositivePatterns（成人教育/成人高考/成人大学/成人学院）。
func isChineseFalsePositive(text, matchedKeyword string) bool {
	for _, fp := range chineseFalsePositivePatterns {
		if matchedKeyword == fp.keyword && strings.Contains(text, fp.exclusion) {
			return true
		}
	}
	return false
}
