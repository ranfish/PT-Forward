package model

import "time"

type OrphanScanConfig struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	ClientID  string    `json:"client_id" gorm:"size:50;not null;uniqueIndex:idx_orphan_scan_client_path"`
	ScanPath  string    `json:"scan_path" gorm:"size:500;not null;uniqueIndex:idx_orphan_scan_client_path"`
	Enabled   bool      `json:"enabled" gorm:"default:true"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (OrphanScanConfig) TableName() string { return "orphan_scan_configs" }
