package rss

import (
	"context"
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

func (r *Repository) List(ctx context.Context) ([]model.RSSSubscription, error) {
	var subs []model.RSSSubscription
	err := r.db.WithContext(ctx).
		Order("name ASC").
		Find(&subs).Error
	return subs, err
}

func (r *Repository) ListActive(ctx context.Context) ([]model.RSSSubscription, error) {
	var subs []model.RSSSubscription
	err := r.db.WithContext(ctx).
		Where("enabled = ? AND paused = ?", true, false).
		Find(&subs).Error
	return subs, err
}

func (r *Repository) GetByID(ctx context.Context, id uint) (*model.RSSSubscription, error) {
	var sub model.RSSSubscription
	err := r.db.WithContext(ctx).First(&sub, id).Error
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

func (r *Repository) Create(ctx context.Context, sub *model.RSSSubscription) error {
	return dbimpl.ForceCreate(r.db.WithContext(ctx), sub)
}

func (r *Repository) Update(ctx context.Context, sub *model.RSSSubscription) error {
	return r.db.WithContext(ctx).Save(sub).Error
}

func (r *Repository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&model.RSSSubscription{}, id).Error
}

func (r *Repository) ExistsByName(ctx context.Context, name string, excludeID uint) (bool, error) {
	var count int64
	q := r.db.WithContext(ctx).Model(&model.RSSSubscription{}).
		Where("name = ?", name)
	if excludeID > 0 {
		q = q.Where("id != ?", excludeID)
	}
	err := q.Count(&count).Error
	return count > 0, err
}

func (r *Repository) MarkSeen(ctx context.Context, seen *model.RSSTorrentSeen) error {
	return r.db.WithContext(ctx).Save(seen).Error
}

func (r *Repository) IsSeen(ctx context.Context, siteName, torrentID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.RSSTorrentSeen{}).
		Where("site_name = ? AND torrent_id = ?", siteName, torrentID).
		Count(&count).Error
	return count > 0, err
}

func (r *Repository) ListSeenBySite(ctx context.Context, siteName string, since time.Time) ([]model.RSSTorrentSeen, error) {
	var seen []model.RSSTorrentSeen
	q := r.db.WithContext(ctx).Where("site_name = ?", siteName)
	if !since.IsZero() {
		q = q.Where("created_at > ?", since)
	}
	err := q.Find(&seen).Error
	return seen, err
}

func (r *Repository) CleanupOldSeen(ctx context.Context, retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		retentionDays = 30
	}
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	result := r.db.WithContext(ctx).
		Where("status IN ? AND updated_at < ?",
			[]string{"pushed", "expired", "skipped_rule"},
			cutoff,
		).Delete(&model.RSSTorrentSeen{})
	return result.RowsAffected, result.Error
}

func (r *Repository) MarkStatus(ctx context.Context, siteName, torrentID, status string) {
	r.db.WithContext(ctx).
		Model(&model.RSSTorrentSeen{}).
		Where("site_name = ? AND torrent_id = ?", siteName, torrentID).
		Update("status", status)
}
