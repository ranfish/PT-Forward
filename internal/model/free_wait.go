package model

import "time"

type FreeWaitEntry struct {
	ID             uint `gorm:"primaryKey;autoIncrement"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
	SiteName       string `gorm:"size:50;not null;uniqueIndex:idx_sitewait"`
	TorrentID      string `gorm:"size:50;not null;uniqueIndex:idx_sitewait"`
	InfoHash       string `gorm:"size:40"`
	Title          string `gorm:"size:500"`
	Size           int64
	ClientID       string `gorm:"size:50;not null"`
	SubscriptionID string `gorm:"size:50;not null"`
	HasHR          bool
	HRSeedTimeH    int
	CheckBefore    *time.Time
	CheckCount     int `gorm:"default:0"`
}
