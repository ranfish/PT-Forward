package download

import (
	"context"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, task *model.DownloadTask) error {
	return r.db.WithContext(ctx).Create(task).Error
}

func (r *Repository) GetByID(ctx context.Context, id uint) (*model.DownloadTask, error) {
	var task model.DownloadTask
	err := r.db.WithContext(ctx).First(&task, id).Error
	return &task, err
}

func (r *Repository) List(ctx context.Context, page, size int, clientID, status string) ([]model.DownloadTask, int64, error) {
	var tasks []model.DownloadTask
	var total int64

	q := r.db.WithContext(ctx).Model(&model.DownloadTask{}).Where("status != ?", model.DownloadStatusDeleted)
	if clientID != "" {
		q = q.Where("client_id = ?", clientID)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * size
	if err := q.Order("updated_at DESC").Offset(offset).Limit(size).Find(&tasks).Error; err != nil {
		return nil, 0, err
	}
	return tasks, total, nil
}

func (r *Repository) ListActive(ctx context.Context) ([]model.DownloadTask, error) {
	var tasks []model.DownloadTask
	err := r.db.WithContext(ctx).
		Where("status IN ?", []string{model.DownloadStatusPending, model.DownloadStatusDownloading, model.DownloadStatusPaused}).
		Find(&tasks).Error
	return tasks, err
}

func (r *Repository) FindByClientAndHash(ctx context.Context, clientID, infoHash string) (*model.DownloadTask, error) {
	var task model.DownloadTask
	err := r.db.WithContext(ctx).
		Where("client_id = ? AND info_hash = ? AND status != ?", clientID, infoHash, model.DownloadStatusDeleted).
		First(&task).Error
	return &task, err
}

func (r *Repository) FindExistingHashes(ctx context.Context, clientID string) (map[string]bool, error) {
	var hashes []string
	err := r.db.WithContext(ctx).Model(&model.DownloadTask{}).
		Where("client_id = ? AND status != ?", clientID, model.DownloadStatusDeleted).
		Pluck("info_hash", &hashes).Error
	if err != nil {
		return nil, err
	}
	result := make(map[string]bool, len(hashes))
	for _, h := range hashes {
		result[h] = true
	}
	return result, nil
}

func (r *Repository) UpdateStatus(ctx context.Context, id uint, status string, progress float64, errMsg string) error {
	return r.db.WithContext(ctx).Model(&model.DownloadTask{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":         status,
			"progress":       progress,
			"error_message":  errMsg,
			"updated_at":     time.Now(),
		}).Error
}

func (r *Repository) UpdateProgress(ctx context.Context, id uint, updates map[string]interface{}) error {
	updates["updated_at"] = time.Now()
	return r.db.WithContext(ctx).Model(&model.DownloadTask{}).
		Where("id = ?", id).
		Updates(updates).Error
}

func (r *Repository) UpdateTransfer(ctx context.Context, id uint, transferStatus, transferClientID, transferHash string) error {
	updates := map[string]interface{}{
		"transfer_status":    transferStatus,
		"transfer_client_id": transferClientID,
		"updated_at":         time.Now(),
	}
	if transferHash != "" {
		updates["transfer_hash"] = transferHash
	}
	if transferStatus == model.TransferStatusTransferred {
		now := time.Now()
		updates["transferred_at"] = &now
		updates["status"] = model.DownloadStatusCompleted
	}
	return r.db.WithContext(ctx).Model(&model.DownloadTask{}).
		Where("id = ?", id).
		Updates(updates).Error
}

func (r *Repository) MarkDeleted(ctx context.Context, id uint, action string) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&model.DownloadTask{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":        model.DownloadStatusDeleted,
			"deleted_at":    &now,
			"delete_action": action,
			"updated_at":    now,
		}).Error
}

func (r *Repository) UpdateClientAndHash(ctx context.Context, id uint, clientID, infoHash string) error {
	return r.db.WithContext(ctx).Model(&model.DownloadTask{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"client_id":  clientID,
			"info_hash":  infoHash,
			"updated_at": time.Now(),
		}).Error
}
