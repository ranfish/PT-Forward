package publish

import (
	"context"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type PublishLimitGuard struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewPublishLimitGuard(db *gorm.DB, logger *zap.Logger) *PublishLimitGuard {
	return &PublishLimitGuard{db: db, logger: logger}
}

func (g *PublishLimitGuard) Check(ctx context.Context, targetSite string) (bool, string) {
	var limit model.SitePublishLimit
	if err := g.db.WithContext(ctx).
		Where("site_name = ? AND enabled = ?", targetSite, true).
		First(&limit).Error; err != nil {
		return true, ""
	}

	if limit.MaxCount <= 0 || limit.WindowHours <= 0 {
		return true, ""
	}

	cutoff := time.Now().Add(time.Duration(-limit.WindowHours) * time.Hour)
	var count int64
	g.db.WithContext(ctx).Model(&model.PublishResultRecord{}).
		Where("target_site = ? AND created_at >= ? AND status IN ?",
			targetSite, cutoff,
			[]string{"completed", "exists", "edited"}).
		Count(&count)

	if count >= int64(limit.MaxCount) {
		g.logger.Info("publish limit exceeded",
			zap.String("target_site", targetSite),
			zap.Int64("recent_count", count),
			zap.Int("max_count", limit.MaxCount),
			zap.Int("window_hours", limit.WindowHours))
		return false, "publish_limit_exceeded"
	}

	return true, ""
}

func (g *PublishLimitGuard) GetLimit(ctx context.Context, siteName string) (*model.SitePublishLimit, bool) {
	var limit model.SitePublishLimit
	if err := g.db.WithContext(ctx).
		Where("site_name = ?", siteName).
		First(&limit).Error; err != nil {
		return nil, false
	}
	return &limit, true
}

func (g *PublishLimitGuard) ListLimits(ctx context.Context) ([]model.SitePublishLimit, error) {
	var limits []model.SitePublishLimit
	if err := g.db.WithContext(ctx).Find(&limits).Error; err != nil {
		return nil, err
	}
	return limits, nil
}

func (g *PublishLimitGuard) UpsertLimit(ctx context.Context, limit *model.SitePublishLimit) error {
	return g.db.WithContext(ctx).Save(limit).Error
}

func (g *PublishLimitGuard) DeleteLimit(ctx context.Context, siteName string) error {
	return g.db.WithContext(ctx).
		Where("site_name = ?", siteName).
		Delete(&model.SitePublishLimit{}).Error
}

func (g *PublishLimitGuard) GetCurrentCount(ctx context.Context, targetSite string) (int64, int, int) {
	limit, ok := g.GetLimit(ctx, targetSite)
	if !ok || !limit.Enabled {
		return 0, 0, 0
	}
	cutoff := time.Now().Add(time.Duration(-limit.WindowHours) * time.Hour)
	var count int64
	g.db.WithContext(ctx).Model(&model.PublishResultRecord{}).
		Where("target_site = ? AND created_at >= ? AND status IN ?",
			targetSite, cutoff,
			[]string{"completed", "exists", "edited"}).
		Count(&count)
	return count, limit.MaxCount, limit.WindowHours
}
