package model

import "time"

const (
	DownloadSourceRSS    = "rss"
	DownloadSourceManual = "manual"

	DownloadStatusPending    = "pending"
	DownloadStatusDownloading = "downloading"
	DownloadStatusPaused     = "paused"
	DownloadStatusCompleted  = "completed"
	DownloadStatusError      = "error"
	DownloadStatusDeleted    = "deleted"

	TransferStatusNone           = ""
	TransferStatusPending        = "transfer_pending"
	TransferStatusTransferring   = "transferring"
	TransferStatusTransferred    = "transferred"
	TransferStatusFailed         = "transfer_failed"
	TransferStatusPartial        = "transfer_partial"

	DeleteActionWithCompanions = "with_companions"
	DeleteActionSiteOnly       = "site_only"
)

type DownloadTask struct {
	ID         uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`

	Source         string `json:"source" gorm:"size:20;not null;default:'rss'"`
	SubscriptionID *uint  `json:"subscription_id" gorm:"index"`

	ClientID string `json:"client_id" gorm:"size:50;index;not null"`

	InfoHash    string `json:"info_hash" gorm:"size:40;index"`
	TorrentName string `json:"torrent_name" gorm:"type:text"`
	SavePath    string `json:"save_path" gorm:"type:text"`
	TotalSize   int64  `json:"total_size"`
	SiteName    string `json:"site_name" gorm:"size:100;index"`

	Status       string  `json:"status" gorm:"size:20;not null;default:'pending';index"`
	Progress     float64 `json:"progress"`
	ErrorMessage string  `json:"error_message" gorm:"type:text"`

	TransferStatus   string     `json:"transfer_status" gorm:"size:20"`
	TransferClientID string     `json:"transfer_client_id" gorm:"size:50"`
	TransferHash     string     `json:"transfer_hash" gorm:"size:40"`
	TransferredAt    *time.Time `json:"transferred_at"`

	DeletedAt    *time.Time `json:"deleted_at"`
	DeleteAction string     `json:"delete_action" gorm:"size:20"`

	Category string `json:"category" gorm:"size:100"`
}

func (DownloadTask) TableName() string { return "download_tasks" }
