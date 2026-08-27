package rss

import (
	"context"
	"time"

	dbimpl "github.com/ranfish/pt-forward/internal/db"
	"github.com/ranfish/pt-forward/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "site_name"}, {Name: "torrent_id"}, {Name: "subscription_id"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"info_hash", "is_fake_hash", "title", "size",
			"is_free", "free_end_at", "free_level", "discount",
			"has_hr", "hr_seed_time_h", "status", "source_category",
			"updated_at",
		}),
	}).Create(seen).Error
}

func (r *Repository) IsSeen(ctx context.Context, siteName, torrentID, subscriptionID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.RSSTorrentSeen{}).
		Where("site_name = ? AND torrent_id = ? AND subscription_id = ? AND status IN ?",
			siteName, torrentID, subscriptionID,
			[]string{"pushed", "blocked"}).
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

// MarkStatus §59.120: 订阅隔离——按 (site, tid, subscription_id) 精确更新。
// §59.17 每订阅独立语义的漏改残留（§59.33 审计 #9）：原 WHERE 缺 subscription_id，
// 订阅 A 打 seen 会污染同站同 tid 的订阅 B 的 seen 行 → B 永远静默丢种。
func (r *Repository) MarkStatus(ctx context.Context, subscriptionID, siteName, torrentID, status string) {
	r.db.WithContext(ctx).
		Model(&model.RSSTorrentSeen{}).
		Where("site_name = ? AND torrent_id = ? AND subscription_id = ?", siteName, torrentID, subscriptionID).
		Update("status", status)
}

// ClearSeen §59.33: 清除订阅的 seen 记录（指定 tid 或全部），返回清除数。
// is_fake_hash 侧载记录不清（避免重新侧载风暴）。清后下轮 RSS 视为新种子
// 重走完整过滤链（免费态实时重判，不用 seen 旧值）。
func (r *Repository) ClearSeen(ctx context.Context, subscriptionID string, torrentIDs []string) (int64, error) {
	q := r.db.WithContext(ctx).
		Where("subscription_id = ? AND is_fake_hash = ?", subscriptionID, false)
	if len(torrentIDs) > 0 {
		q = q.Where("torrent_id IN ?", torrentIDs)
	}
	result := q.Delete(&model.RSSTorrentSeen{})
	return result.RowsAffected, result.Error
}

// ListSeenBySubscription §59.33: 订阅的 seen 记录列表（前端历史/清除用）。
func (r *Repository) ListSeenBySubscription(ctx context.Context, subscriptionID string, limit int) ([]model.RSSTorrentSeen, error) {
	if limit <= 0 {
		limit = 200
	}
	var seen []model.RSSTorrentSeen
	err := r.db.WithContext(ctx).
		Where("subscription_id = ?", subscriptionID).
		Order("updated_at DESC").
		Limit(limit).
		Find(&seen).Error
	return seen, err
}
