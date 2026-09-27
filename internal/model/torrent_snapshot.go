package model

import "time"

// TorrentSnapshot 下载器种子快照（定时同步，用于种子配置页路径选择和列表展示）。
// 主键 (hash, client_id)：同一种子在多个下载器各一行。
type TorrentSnapshot struct {
	ID         uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Hash       string    `json:"hash" gorm:"size:40;not null;uniqueIndex:idx_snapshot_hash_client,composite:hash"`
	ClientUID  uint     `json:"client_uid" gorm:"not null;default:0;uniqueIndex:idx_snapshot_hash_client,composite:client_uid;index:idx_snapshots_cluster,composite:client_path_name"`
	Name       string    `json:"name" gorm:"size:500;index:idx_snapshots_cluster,composite:client_path_name"`
	Comment    string    `json:"comment" gorm:"type:text"` // §59.61: 种子 comment——簇直达判据凭证（TR/qb syncer 同步）
	// §59.296: 下载来源站域名（tracker 首域 host）——簇副本归属判定
	// （馒头纯数字 comment 方言的站点上下文；comment 直达/选站 ①a 消费）。syncer 同步写入。
	TrackerDomain string    `json:"tracker_domain" gorm:"size:120;default:''"`
	SavePath   string    `json:"save_path" gorm:"size:500;index;index:idx_snapshots_cluster,composite:client_path_name"`
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
