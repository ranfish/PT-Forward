package rule

import (
	"context"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"gorm.io/gorm"
)

type Module struct {
	db *gorm.DB
}

func NewModule(db *gorm.DB) *Module {
	return &Module{db: db}
}

func (m *Module) ListRules(ctx context.Context) ([]model.DeleteRule, error) {
	var rules []model.DeleteRule
	err := m.db.WithContext(ctx).Order("priority DESC").Find(&rules).Error
	return rules, err
}

func (m *Module) ListEnabledRules(ctx context.Context) ([]model.DeleteRule, error) {
	var rules []model.DeleteRule
	err := m.db.WithContext(ctx).Where("enabled = ?", true).Order("priority DESC").Find(&rules).Error
	return rules, err
}

func (m *Module) ListRulesByIDs(ctx context.Context, ids []uint) ([]model.DeleteRule, error) {
	var rules []model.DeleteRule
	err := m.db.WithContext(ctx).Where("id IN ? AND enabled = ?", ids, true).Order("priority DESC").Find(&rules).Error
	return rules, err
}

func (m *Module) GetRule(ctx context.Context, id uint) (*model.DeleteRule, error) {
	var rule model.DeleteRule
	err := m.db.WithContext(ctx).First(&rule, id).Error
	return &rule, err
}

func (m *Module) CreateRule(ctx context.Context, rule *model.DeleteRule) error {
	return m.db.WithContext(ctx).Create(rule).Error
}

func (m *Module) UpdateRule(ctx context.Context, id uint, updates map[string]interface{}) error {
	updates["updated_at"] = time.Now()
	return m.db.WithContext(ctx).Model(&model.DeleteRule{}).Where("id = ?", id).Updates(updates).Error
}

func (m *Module) DeleteRule(ctx context.Context, id uint) error {
	return m.db.WithContext(ctx).Delete(&model.DeleteRule{}, id).Error
}

type Match struct {
	Context *Context
	Rule    *model.DeleteRule
}

func (m *Module) Evaluate(ctx context.Context, contexts []*Context, rules []*model.DeleteRule) []Match {
	var matches []Match
	for _, c := range contexts {
		for _, r := range rules {
			if MatchRule(c, r) {
				matches = append(matches, Match{Context: c, Rule: r})
				break
			}
		}
	}
	return matches
}
