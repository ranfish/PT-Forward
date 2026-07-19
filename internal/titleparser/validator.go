// Package titleparser 标题校验器（§56.19 Q2 修正决策）。
//
// 8 类校验维度 + 自动修复（差异化价值，竞品都不做）：
//   required / regex / case / order / length / forbidden / whitespace / character
//
// TitleValidator 是纯函数（不查 DB），规则通过参数传入。
// 调用方（handler/pipeline）从 DB 加载 model.TitleRule 后转换为 titleparser.TitleRule。
package titleparser

import (
	"fmt"
	"regexp"
	"strings"
)

// TitleRule 标题校验规则（独立于 model.TitleRule，保持 titleparser 包独立性）。
type TitleRule struct {
	SiteCode     string // 站点代码（空=全局）
	RuleType     string // 8 种维度（RuleType* 常量）
	Field        string // 字段名（title/year/resolution/hdr/codec/...）
	Pattern      string // 正则模式或匹配文本
	Replacement  string // 自动修复替换文本
	AutoFix      bool   // 是否自动修复
	ErrorMessage string // 不合规提示文案
}

// 8 种校验维度常量（与 model.TitleRule 常量保持同步）。
const (
	RuleTypeRequired   = "required"   // 必填字段
	RuleTypeRegex      = "regex"      // 格式正则
	RuleTypeCase       = "case"       // 大小写
	RuleTypeOrder      = "order"      // 字段顺序
	RuleTypeLength     = "length"     // 长度限制
	RuleTypeForbidden  = "forbidden"  // 禁止内容
	RuleTypeWhitespace = "whitespace" // 空格数量
	RuleTypeCharacter  = "character"  // 特殊字符
)

// ValidationResult 校验结果。
type ValidationResult struct {
	Title       string            // 校验/修复后的标题
	SourceTitle string            // 原始标题
	Errors      []ValidationError // 仍存在的错误（无法自动修复）
	AutoFixed   []AutoFixRecord   // 已自动修复的记录
}

// ValidationError 校验错误。
type ValidationError struct {
	RuleType string // 规则类型
	Field    string // 字段
	Message  string // 错误文案
}

// AutoFixRecord 自动修复记录。
type AutoFixRecord struct {
	RuleType string // 规则类型
	Field    string // 字段
	From     string // 修复前
	To       string // 修复后
}

// TitleValidator 标题校验器（纯函数，无 DB 依赖）。
type TitleValidator struct{}

// NewTitleValidator 创建校验器。
func NewTitleValidator() *TitleValidator {
	return &TitleValidator{}
}

// Validate 校验标题并尝试自动修复。
// rules 按 site_code 已过滤的规则列表。
func (v *TitleValidator) Validate(title string, rules []TitleRule) *ValidationResult {
	result := &ValidationResult{
		SourceTitle: title,
		Title:       title,
	}

	for _, rule := range rules {
		v.applyRule(result, rule)
	}

	return result
}

// applyRule 应用单条规则。
func (v *TitleValidator) applyRule(result *ValidationResult, rule TitleRule) {
	switch rule.RuleType {
	case RuleTypeRequired:
		v.applyRequired(result, rule)
	case RuleTypeRegex:
		v.applyRegex(result, rule)
	case RuleTypeCase:
		v.applyCase(result, rule)
	case RuleTypeLength:
		v.applyLength(result, rule)
	case RuleTypeForbidden:
		v.applyForbidden(result, rule)
	case RuleTypeWhitespace:
		v.applyWhitespace(result, rule)
	case RuleTypeCharacter:
		v.applyCharacter(result, rule)
	case RuleTypeOrder:
		// order 规则复杂（需解析标题组件），暂仅校验不修复
		v.applyOrder(result, rule)
	}
}

// applyRequired 必填字段检查。
// Pattern 为字段名（year/resolution/...），检查标题中是否包含该字段值。
// 简化实现：检查 Pattern 关键词是否出现在标题中。
func (v *TitleValidator) applyRequired(result *ValidationResult, rule TitleRule) {
	if rule.Pattern == "" {
		return
	}
	// 简化：Pattern 是必须出现的关键词（如 "20xx" 年份格式）
	re, err := regexp.Compile(rule.Pattern)
	if err != nil {
		return
	}
	if !re.MatchString(result.Title) {
		msg := rule.ErrorMessage
		if msg == "" {
			msg = fmt.Sprintf("缺少必填内容: %s", rule.Field)
		}
		result.Errors = append(result.Errors, ValidationError{
			RuleType: rule.RuleType,
			Field:    rule.Field,
			Message:  msg,
		})
	}
}

// applyRegex 格式正则校验 + 自动修复。
// Pattern 为正则，Replacement 为替换文本。
func (v *TitleValidator) applyRegex(result *ValidationResult, rule TitleRule) {
	if rule.Pattern == "" {
		return
	}
	re, err := regexp.Compile(rule.Pattern)
	if err != nil {
		return
	}
	if re.MatchString(result.Title) {
		// 匹配到不合规内容
		if rule.AutoFix && rule.Replacement != "" {
			fixed := re.ReplaceAllString(result.Title, rule.Replacement)
			if fixed != result.Title {
				result.AutoFixed = append(result.AutoFixed, AutoFixRecord{
					RuleType: rule.RuleType,
					Field:    rule.Field,
					From:     result.Title,
					To:       fixed,
				})
				result.Title = fixed
			}
		} else {
			msg := rule.ErrorMessage
			if msg == "" {
				msg = fmt.Sprintf("格式不合规: %s", rule.Field)
			}
			result.Errors = append(result.Errors, ValidationError{
				RuleType: rule.RuleType,
				Field:    rule.Field,
				Message:  msg,
			})
		}
	}
}

// applyCase 大小写校验 + 自动修复。
// Pattern 为需要大写的关键词（如 "HDR10"），标题中含其小写/混合形式时自动转大写。
func (v *TitleValidator) applyCase(result *ValidationResult, rule TitleRule) {
	if rule.Pattern == "" {
		return
	}
	lower := strings.ToLower(rule.Pattern)
	upper := strings.ToUpper(rule.Pattern)
	// 标题（小写化）含 Pattern 的小写形式，且标题不含全大写形式
	titleLower := strings.ToLower(result.Title)
	if strings.Contains(titleLower, lower) && !strings.Contains(result.Title, upper) {
		if rule.AutoFix {
			fixed := replaceCaseInsensitive(result.Title, lower, upper)
			if fixed != result.Title {
				result.AutoFixed = append(result.AutoFixed, AutoFixRecord{
					RuleType: rule.RuleType,
					Field:    rule.Field,
					From:     result.Title,
					To:       fixed,
				})
				result.Title = fixed
			}
		} else {
			result.Errors = append(result.Errors, ValidationError{
				RuleType: rule.RuleType,
				Field:    rule.Field,
				Message:  fmt.Sprintf("%s 必须大写", rule.Pattern),
			})
		}
	}
}

// applyLength 长度限制 + 自动截断。
// Pattern 为空，Field 为 "title"，Replacement 为数字字符串（如 "255"）。
func (v *TitleValidator) applyLength(result *ValidationResult, rule TitleRule) {
	maxLen := 0
	if _, err := fmt.Sscanf(rule.Replacement, "%d", &maxLen); err != nil || maxLen <= 0 {
		return
	}
	runes := []rune(result.Title)
	if len(runes) > maxLen {
		if rule.AutoFix {
			fixed := string(runes[:maxLen])
			result.AutoFixed = append(result.AutoFixed, AutoFixRecord{
				RuleType: rule.RuleType,
				Field:    rule.Field,
				From:     result.Title,
				To:       fixed,
			})
			result.Title = fixed
		} else {
			result.Errors = append(result.Errors, ValidationError{
				RuleType: rule.RuleType,
				Field:    rule.Field,
				Message:  fmt.Sprintf("标题长度超限（%d > %d）", len(runes), maxLen),
			})
		}
	}
}

// applyForbidden 禁止内容检查（不可自动修复）。
// Pattern 为禁止关键词（逗号分隔多个）。
func (v *TitleValidator) applyForbidden(result *ValidationResult, rule TitleRule) {
	if rule.Pattern == "" {
		return
	}
	for _, kw := range strings.Split(rule.Pattern, ",") {
		kw = strings.TrimSpace(kw)
		if kw == "" {
			continue
		}
		if strings.Contains(strings.ToLower(result.Title), strings.ToLower(kw)) {
			msg := rule.ErrorMessage
			if msg == "" {
				msg = fmt.Sprintf("标题含禁止内容: %s", kw)
			}
			result.Errors = append(result.Errors, ValidationError{
				RuleType: rule.RuleType,
				Field:    rule.Field,
				Message:  msg,
			})
		}
	}
}

// applyWhitespace 空格数量校验 + 自动补齐。
// Pattern 为正则（如 "\s{5,}"），Replacement 为替换文本（如 "     "）。
func (v *TitleValidator) applyWhitespace(result *ValidationResult, rule TitleRule) {
	if rule.Pattern == "" {
		return
	}
	re, err := regexp.Compile(rule.Pattern)
	if err != nil {
		return
	}
	if rule.Replacement != "" && re.MatchString(result.Title) {
		if rule.AutoFix {
			fixed := re.ReplaceAllString(result.Title, rule.Replacement)
			if fixed != result.Title {
				result.AutoFixed = append(result.AutoFixed, AutoFixRecord{
					RuleType: rule.RuleType,
					Field:    rule.Field,
					From:     result.Title,
					To:       fixed,
				})
				result.Title = fixed
			}
		}
	}
}

// applyCharacter 特殊字符校验 + 自动移除/替换。
// Pattern 为正则（如 "[()（）]"），Replacement 为替换文本（空=移除）。
func (v *TitleValidator) applyCharacter(result *ValidationResult, rule TitleRule) {
	if rule.Pattern == "" {
		return
	}
	re, err := regexp.Compile(rule.Pattern)
	if err != nil {
		return
	}
	if re.MatchString(result.Title) {
		if rule.AutoFix {
			fixed := re.ReplaceAllString(result.Title, rule.Replacement)
			if fixed != result.Title {
				result.AutoFixed = append(result.AutoFixed, AutoFixRecord{
					RuleType: rule.RuleType,
					Field:    rule.Field,
					From:     result.Title,
					To:       fixed,
				})
				result.Title = fixed
			}
		} else {
			result.Errors = append(result.Errors, ValidationError{
				RuleType: rule.RuleType,
				Field:    rule.Field,
				Message:  fmt.Sprintf("标题含禁止字符: %s", rule.Pattern),
			})
		}
	}
}

// applyOrder 字段顺序校验（不自动修复，仅提示）。
// Pattern 为 "before:after" 格式（如 "resolution:codec"）。
func (v *TitleValidator) applyOrder(result *ValidationResult, rule TitleRule) {
	if rule.Pattern == "" {
		return
	}
	parts := strings.SplitN(rule.Pattern, ":", 2)
	if len(parts) != 2 {
		return
	}
	before, after := parts[0], parts[1]
	beforeIdx := strings.Index(result.Title, before)
	afterIdx := strings.Index(result.Title, after)
	if beforeIdx >= 0 && afterIdx >= 0 && beforeIdx > afterIdx {
		result.Errors = append(result.Errors, ValidationError{
			RuleType: rule.RuleType,
			Field:    rule.Field,
			Message:  fmt.Sprintf("%s 应在 %s 之前", before, after),
		})
	}
}

// replaceCaseInsensitive 大小写不敏感替换。
func replaceCaseInsensitive(s, old, new string) string {
	re := regexp.MustCompile("(?i)" + regexp.QuoteMeta(old))
	return re.ReplaceAllString(s, new)
}
