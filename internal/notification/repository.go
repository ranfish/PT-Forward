package notification

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

func (r *Repository) List(ctx context.Context) ([]model.NotificationChannel, error) {
	var channels []model.NotificationChannel
	err := r.db.WithContext(ctx).Order("name ASC").Find(&channels).Error
	return channels, err
}

func (r *Repository) GetByID(ctx context.Context, id uint) (*model.NotificationChannel, error) {
	var ch model.NotificationChannel
	err := r.db.WithContext(ctx).First(&ch, id).Error
	if err != nil {
		return nil, err
	}
	return &ch, nil
}

func (r *Repository) Create(ctx context.Context, ch *model.NotificationChannel) error {
	return dbimpl.ForceCreate(r.db.WithContext(ctx), ch)
}

func (r *Repository) Update(ctx context.Context, ch *model.NotificationChannel) error {
	return r.db.WithContext(ctx).Save(ch).Error
}

func (r *Repository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&model.NotificationChannel{}, id).Error
}

func (r *Repository) ExistsByName(ctx context.Context, name string, excludeID uint) (bool, error) {
	var count int64
	q := r.db.WithContext(ctx).Model(&model.NotificationChannel{}).Where("name = ?", name)
	if excludeID > 0 {
		q = q.Where("id != ?", excludeID)
	}
	err := q.Count(&count).Error
	return count > 0, err
}

func (r *Repository) ListHistory(ctx context.Context, channelID uint, limit int) ([]model.NotificationHistory, error) {
	var history []model.NotificationHistory
	q := r.db.WithContext(ctx).Order("created_at DESC")
	if channelID > 0 {
		q = q.Where("channel_id = ?", channelID)
	}
	if limit > 0 {
		q = q.Limit(limit)
	}
	err := q.Find(&history).Error
	return history, err
}

func (r *Repository) CleanupOldHistory(ctx context.Context, retainDays int) error {
	if retainDays <= 0 {
		retainDays = 30
	}
	cutoff := time.Now().AddDate(0, 0, -retainDays)
	return r.db.WithContext(ctx).
		Where("created_at < ?", cutoff).
		Delete(&model.NotificationHistory{}).Error
}
