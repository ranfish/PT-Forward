// Package compliance 规则加载 + DB 同步（§56.21 决策 1 + Q3）。
package compliance

import (
	_ "embed"
	"encoding/json"

	"github.com/ranfish/pt-forward/internal/model"
	"gorm.io/gorm"
)

//go:embed data/default_rules.json
var defaultRulesJSON []byte

// complianceRuleData embed JSON 中的规则定义。
type complianceRuleData struct {
	RuleType string `json:"rule_type"`
	Pattern  string `json:"pattern"`
	Category string `json:"category"`
	SiteCode string `json:"site_code"`
	Scope    string `json:"scope"`
}

// LoadDefaultRules 解析 embed JSON 为规则列表（纯函数，不依赖 DB）。
func LoadDefaultRules() ([]complianceRuleData, error) {
	var data struct {
		Rules []complianceRuleData `json:"rules"`
	}
	if err := json.Unmarshal(defaultRulesJSON, &data); err != nil {
		return nil, err
	}
	return data.Rules, nil
}

// SyncComplianceRulesToDB §56.21: 启动时从 embed JSON 同步合规规则到 DB。
// builtin 记录自动更新，user 记录跳过（保留用户自定义）。
func SyncComplianceRulesToDB(db *gorm.DB) error {
	rules, err := LoadDefaultRules()
	if err != nil {
		return err
	}

	for _, r := range rules {
		scope := r.Scope
		if scope == "" {
			scope = model.ScopeShare
		}
		var existing model.ComplianceRule
		result := db.Where("rule_type = ? AND pattern = ? AND site_code = ?",
			r.RuleType, r.Pattern, r.SiteCode).First(&existing)

		if result.Error != nil {
			// 不存在 → 创建
			if err := db.Create(&model.ComplianceRule{
				RuleType: r.RuleType,
				Pattern:  r.Pattern,
				Category: r.Category,
				SiteCode: r.SiteCode,
				Scope:    scope,
				Source:   "builtin",
			}).Error; err != nil {
				continue // 单条失败不阻断
			}
		} else if existing.Source == "builtin" {
			// builtin → 更新 scope
			db.Model(&existing).Updates(map[string]interface{}{
				"scope": scope,
			})
		}
		// user 记录跳过
	}
	return nil
}
