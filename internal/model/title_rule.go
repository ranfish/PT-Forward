package model

import "time"

// §56.19 Q2.1 — TitleRule: 标题校验规则（8 类维度）。
// rule_type: required / regex / case / order / length / forbidden / whitespace / character
// auto_fix: true=可自动修复, false=仅提示阻止
type TitleRule struct {
	ID           uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	SiteCode     string    `json:"site_code" gorm:"size:50;uniqueIndex:idx_rule_unique"` // 站点代码（空=全局规则）
	RuleType     string    `json:"rule_type" gorm:"size:20;uniqueIndex:idx_rule_unique"` // 8 种维度
	Field        string    `json:"field" gorm:"size:50;uniqueIndex:idx_rule_unique"`     // title/year/resolution/hdr/codec/...
	Pattern      string    `json:"pattern" gorm:"size:500"`                              // 正则模式
	Replacement  string    `json:"replacement" gorm:"size:200"`                           // 自动修复替换文本
	AutoFix      bool      `json:"auto_fix" gorm:"default:true"`                          // 是否自动修复
	ErrorMessage string    `json:"error_message" gorm:"size:500"`                         // 不合规提示文案
	Source       string    `json:"source" gorm:"size:20;default:'builtin'"`               // builtin / user
	CreatedAt    time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (TitleRule) TableName() string { return "title_rules" }

// 8 种校验维度常量。
const (
	RuleTypeRequired   = "required"   // 必填字段
	RuleTypeRegex      = "regex"      // 格式正则（如 ^\d{3,4}p$）
	RuleTypeCase       = "case"       // 大小写（如 HDR 必须大写）
	RuleTypeOrder      = "order"      // 字段顺序
	RuleTypeLength     = "length"     // 长度限制
	RuleTypeForbidden  = "forbidden"  // 禁止内容
	RuleTypeWhitespace = "whitespace" // 空格数量
	RuleTypeCharacter  = "character"  // 特殊字符
)
