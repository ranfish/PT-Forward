package seeding

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type RuleEvaluator struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewRuleEvaluator(db *gorm.DB, logger *zap.Logger) *RuleEvaluator {
	return &RuleEvaluator{db: db, logger: logger}
}

type RuleMatch struct {
	Rule     model.DeleteRule
	Records  []model.SeedingTorrentRecord
	Priority int
}

func (re *RuleEvaluator) EvaluateRules(ctx context.Context, clientID string) ([]RuleMatch, error) {
	var rules []model.DeleteRule
	if err := re.db.WithContext(ctx).
		Where("enabled = ?", true).
		Order("priority DESC").
		Find(&rules).Error; err != nil {
		return nil, fmt.Errorf("query delete rules: %w", err)
	}

	if len(rules) == 0 {
		return nil, nil
	}

	var records []model.SeedingTorrentRecord
	if err := re.db.WithContext(ctx).
		Where("client_id = ? AND status = ?", clientID, model.SeedingStatusSeeding).
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("query seeding records: %w", err)
	}

	if len(records) == 0 {
		return nil, nil
	}

	var matches []RuleMatch
	for _, rule := range rules {
		matched := re.matchRule(rule, records)
		if len(matched) > 0 {
			matches = append(matches, RuleMatch{
				Rule:     rule,
				Records:  matched,
				Priority: rule.Priority,
			})
		}
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Priority > matches[j].Priority
	})

	return matches, nil
}

func (re *RuleEvaluator) matchRule(rule model.DeleteRule, records []model.SeedingTorrentRecord) []model.SeedingTorrentRecord {
	if rule.Conditions == "" && rule.Expr == "" {
		return nil
	}

	conditions := parseConditions(rule.Conditions)

	var matched []model.SeedingTorrentRecord
	for _, rec := range records {
		if re.matchConditions(rec, conditions) {
			matched = append(matched, rec)
		}
	}

	if rule.DeleteNum > 0 && len(matched) > rule.DeleteNum {
		matched = matched[:rule.DeleteNum]
	}

	return matched
}

type ruleCondition struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

func parseConditions(conditionsJSON string) []ruleCondition {
	if conditionsJSON == "" {
		return nil
	}
	var conditions []ruleCondition
	if err := json.Unmarshal([]byte(conditionsJSON), &conditions); err != nil {
		return nil
	}
	return conditions
}

func (re *RuleEvaluator) matchConditions(rec model.SeedingTorrentRecord, conditions []ruleCondition) bool {
	for _, cond := range conditions {
		if !re.matchCondition(rec, cond) {
			return false
		}
	}
	return len(conditions) > 0
}

func (re *RuleEvaluator) matchCondition(rec model.SeedingTorrentRecord, cond ruleCondition) bool {
	var fieldValue string
	switch cond.Field {
	case "site_name":
		fieldValue = rec.SiteName
	case "status":
		fieldValue = string(rec.Status)
	case "is_free":
		fieldValue = fmt.Sprintf("%t", rec.IsFree)
	case "has_hr":
		fieldValue = fmt.Sprintf("%t", rec.HasHR)
	case "discount":
		fieldValue = string(rec.Discount)
	default:
		return true
	}

	switch cond.Operator {
	case "equals", "==":
		return fieldValue == cond.Value
	case "not_equals", "!=":
		return fieldValue != cond.Value
	case "contains":
		return contains(fieldValue, cond.Value)
	case "not_contains":
		return !contains(fieldValue, cond.Value)
	default:
		return true
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
