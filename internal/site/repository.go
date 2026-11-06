package site

import (
	"context"
	"errors"
	"time"

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

func (r *Repository) List(ctx context.Context) ([]model.Site, error) {
	var sites []model.Site
	err := r.db.WithContext(ctx).
		Where("enabled = ? OR enabled = ?", true, false).
		Order("name ASC").
		Find(&sites).Error
	return sites, err
}

func (r *Repository) GetByID(ctx context.Context, id uint) (*model.Site, error) {
	var site model.Site
	err := r.db.WithContext(ctx).First(&site, id).Error
	if err != nil {
		return nil, err
	}
	return &site, nil
}

// GetByDomain §59.476: 域名/站名查站点。未找到 → (nil, nil)；查询错误
// （ctx 超时等）→ (nil, err) 透传——原版吞错伪装 not-found（12 环境
// "站点不存在: 不可羊" 实为 ctx deadline·两实证·误导诊断）。
func (r *Repository) GetByDomain(ctx context.Context, ident string) (*model.Site, error) {
	var site model.Site
	err := r.db.WithContext(ctx).Where("domain = ? OR name = ?", ident, ident).First(&site).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &site, nil
}

func (r *Repository) GetByName(ctx context.Context, name string) (*model.Site, error) {
	var site model.Site
	err := r.db.WithContext(ctx).Where("name = ?", name).First(&site).Error
	if err != nil {
		return nil, err
	}
	return &site, nil
}

func (r *Repository) Create(ctx context.Context, site *model.Site) error {
	return dbimpl.ForceCreate(r.db.WithContext(ctx), site)
}

func (r *Repository) Update(ctx context.Context, site *model.Site) error {
	return r.db.WithContext(ctx).Save(site).Error
}

func (r *Repository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&model.Site{}, id).Error
}

func (r *Repository) ExistsByDomain(ctx context.Context, domain string, excludeID uint) (bool, error) {
	var count int64
	q := r.db.WithContext(ctx).Model(&model.Site{}).Where("domain = ?", domain)
	if excludeID > 0 {
		q = q.Where("id != ?", excludeID)
	}
	err := q.Count(&count).Error
	return count > 0, err
}

func (r *Repository) ExistsByName(ctx context.Context, name string, excludeID uint) (bool, error) {
	var count int64
	q := r.db.WithContext(ctx).Model(&model.Site{}).Where("name = ?", name)
	if excludeID > 0 {
		q = q.Where("id != ?", excludeID)
	}
	err := q.Count(&count).Error
	return count > 0, err
}

func (r *Repository) UpdateStats(ctx context.Context, id uint, stats map[string]interface{}) error {
	return r.db.WithContext(ctx).Model(&model.Site{}).Where("id = ?", id).Updates(stats).Error
}

func (r *Repository) UpdateCredentials(ctx context.Context, id uint, creds map[string]interface{}) error {
	creds["updated_at"] = time.Now()
	return r.db.WithContext(ctx).Model(&model.Site{}).Where("id = ?", id).Updates(creds).Error
}

func (r *Repository) DB() *gorm.DB {
	return r.db
}
