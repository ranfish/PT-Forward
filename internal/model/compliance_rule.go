package model

import "time"

// §56.21 决策 1 — ComplianceRule: 合规规则（统一三套硬编码）。
// rule_type: adult / forbidden_keyword / forbidden_group / site_blacklist_category
// scope: all / publish / reseed / share / download（§56.31 统一术语）
type ComplianceRule struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	RuleType  string    `json:"rule_type" gorm:"size:50;uniqueIndex:idx_compliance_unique"` // adult / forbidden_keyword / forbidden_group / site_blacklist_category
	Pattern   string    `json:"pattern" gorm:"size:200;uniqueIndex:idx_compliance_unique"`  // 关键词/小组名/分类 ID
	Category  string    `json:"category" gorm:"size:50"`                                    // 维度标签（word/type/standard/tag）
	SiteCode  string    `json:"site_code" gorm:"size:50;uniqueIndex:idx_compliance_unique;default:''"` // 站点代码（空=全局）
	Scope     string    `json:"scope" gorm:"size:20;default:'share'"`                       // all/publish/reseed/share/download
	Source    string    `json:"source" gorm:"size:20;default:'builtin'"`                    // builtin / user
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (ComplianceRule) TableName() string { return "compliance_rules" }

// 规则类型常量。
const (
	RuleTypeAdult                  = "adult"
	RuleTypeForbiddenKeyword       = "forbidden_keyword"
	RuleTypeForbiddenGroup         = "forbidden_group"
	RuleTypeSiteBlacklistCategory  = "site_blacklist_category"
)

// ComplianceScope §56.31 统一术语。
const (
	ScopeAll      = "all"      // 所有场景
	ScopePublish  = "publish"  // 仅发布
	ScopeReseed   = "reseed"   // 仅辅种
	ScopeShare    = "share"    // 发布+辅种（分享）
	ScopeDownload = "download" // 仅下载
)
