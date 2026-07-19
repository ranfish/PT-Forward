// Package titleparser 标题校验规则加载 + DB 同步（§56.19 Q2.1/Q2.6）。
package titleparser

import (
	_ "embed"
	"encoding/json"

	"github.com/ranfish/pt-forward/internal/model"
	"gorm.io/gorm"
)

//go:embed data/title_rules.json
var titleRulesJSON []byte

// titleRuleData embed JSON 中的规则定义。
type titleRuleData struct {
	RuleType     string `json:"rule_type"`
	Field        string `json:"field"`
	Pattern      string `json:"pattern"`
	Replacement  string `json:"replacement"`
	AutoFix      bool   `json:"auto_fix"`
	ErrorMessage string `json:"error_message"`
}

// LoadDefaultTitleRules 解析 embed JSON 为规则列表（纯函数）。
func LoadDefaultTitleRules() ([]titleRuleData, error) {
	var data struct {
		Rules []titleRuleData `json:"rules"`
	}
	if err := json.Unmarshal(titleRulesJSON, &data); err != nil {
		return nil, err
	}
	return data.Rules, nil
}

// SyncTitleRulesToDB §56.19: 启动时从 embed JSON 同步标题规则到 DB。
// builtin 记录自动创建，user 记录跳过。
func SyncTitleRulesToDB(db *gorm.DB) error {
	rules, err := LoadDefaultTitleRules()
	if err != nil {
		return err
	}

	for _, r := range rules {
		var existing model.TitleRule
		result := db.Where("rule_type = ? AND field = ? AND pattern = ? AND site_code = ?",
			r.RuleType, r.Field, r.Pattern, "").First(&existing)

		if result.Error != nil {
			db.Create(&model.TitleRule{
				RuleType:     r.RuleType,
				Field:        r.Field,
				Pattern:      r.Pattern,
				Replacement:  r.Replacement,
				AutoFix:      r.AutoFix,
				ErrorMessage: r.ErrorMessage,
				SiteCode:     "", // 全局规则
				Source:       "builtin",
			})
		} else if existing.Source == "builtin" {
			db.Model(&existing).Updates(map[string]interface{}{
				"replacement":   r.Replacement,
				"auto_fix":      r.AutoFix,
				"error_message": r.ErrorMessage,
			})
		}
	}
	return nil
}
