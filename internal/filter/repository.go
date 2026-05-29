package filter

import (
	"context"

	dbimpl "github.com/ranfish/pt-forward/internal/db"
	"github.com/ranfish/pt-forward/internal/model"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) List(ctx context.Context) ([]model.FilterRule, error) {
	var rules []model.FilterRule
	err := r.db.WithContext(ctx).
		Order("priority ASC, id ASC").
		Find(&rules).Error
	return rules, err
}

func (r *Repository) GetByID(ctx context.Context, id uint) (*model.FilterRule, error) {
	var rule model.FilterRule
	err := r.db.WithContext(ctx).First(&rule, id).Error
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

func (r *Repository) Create(ctx context.Context, rule *model.FilterRule) error {
	return dbimpl.ForceCreate(r.db.WithContext(ctx), rule)
}

func (r *Repository) Update(ctx context.Context, rule *model.FilterRule) error {
	return r.db.WithContext(ctx).Save(rule).Error
}

func (r *Repository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&model.FilterRule{}, id).Error
}

func (r *Repository) ExistsByName(ctx context.Context, name string, excludeID uint) (bool, error) {
	var count int64
	q := r.db.WithContext(ctx).Model(&model.FilterRule{}).
		Where("name = ?", name)
	if excludeID > 0 {
		q = q.Where("id != ?", excludeID)
	}
	err := q.Count(&count).Error
	return count > 0, err
}

func (r *Repository) ListEnabled(ctx context.Context, ruleType string) ([]model.FilterRule, error) {
	var rules []model.FilterRule
	q := r.db.WithContext(ctx).
		Where("enabled = ?", true)
	if ruleType != "" {
		q = q.Where("rule_type = ?", ruleType)
	}
	err := q.Order("priority ASC, id ASC").Find(&rules).Error
	return rules, err
}
