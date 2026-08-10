package model

import "time"

// TorrentSnapshot 下载器种子快照（定时同步，用于种子配置页路径选择和列表展示）。
// 主键 (hash, client_id)：同一种子在多个下载器各一行。
type TorrentSnapshot struct {
	ID         uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Hash       string    `json:"hash" gorm:"size:40;not null;uniqueIndex:idx_snapshot_hash_client,composite:hash"`
	ClientID   string    `json:"client_id" gorm:"size:50;not null;uniqueIndex:idx_snapshot_hash_client,composite:client_id;index"`
	Name       string    `json:"name" gorm:"size:500"`
	SavePath   string    `json:"save_path" gorm:"size:500;index"`
	Size       int64     `json:"size"`
	State      string    `json:"state" gorm:"size:50"`
	Progress   float64   `json:"progress"`
	Uploaded   int64     `json:"uploaded"`
	IsHidden   bool      `json:"is_hidden" gorm:"default:false;index"`
	LastSeen   time.Time `json:"last_seen"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (TorrentSnapshot) TableName() string { return "torrent_snapshots" }
