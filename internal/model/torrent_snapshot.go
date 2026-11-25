package model

import "time"

// TorrentSnapshot 下载器种子快照（定时同步，用于种子配置页路径选择和列表展示）。
// 主键 (hash, client_id)：同一种子在多个下载器各一行。
type TorrentSnapshot struct {
	ID        uint   `json:"id" gorm:"primaryKey;autoIncrement"`
	Hash      string `json:"hash" gorm:"size:40;not null;uniqueIndex:idx_snapshot_hash_client,composite:hash"`
	ClientUID uint   `json:"client_uid" gorm:"not null;default:0;uniqueIndex:idx_snapshot_hash_client,composite:client_uid;index:idx_snapshots_cluster,composite:client_path_name"`
	Name      string `json:"name" gorm:"size:500;index:idx_snapshots_cluster,composite:client_path_name"`
	Comment   string `json:"comment" gorm:"type:text"` // §59.61: 种子 comment——簇直达判据凭证（TR/qb syncer 同步）
	// §59.296: 下载来源站域名（tracker 首域 host）——簇副本归属判定
	// （馒头纯数字 comment 方言的站点上下文；comment 直达/选站 ①a 消费）。syncer 同步写入。
	TrackerDomain string  `json:"tracker_domain" gorm:"size:120;default:''"`
	SavePath      string  `json:"save_path" gorm:"size:500;index;index:idx_snapshots_cluster,composite:client_path_name"`
	Size          int64   `json:"size"`
	State         string  `json:"state" gorm:"size:50"`
	Progress      float64 `json:"progress"`
	Uploaded      int64   `json:"uploaded"`
	IsHidden      bool    `json:"is_hidden" gorm:"default:false;index"`
	// §59.578 刀1（v0.0.1634）: 获取失败烙印（口径 A·用户定案）——未获取颗
	// 首获终态失败（瞬态 isTransientFetchErr 之外）落此标记：列表 unsupported②
	// 判定源 + 弹窗「获取未配置」排除条件（任一 hash 烙印→整 name 排除）。
	// BDScanFailedAt 先例模式（时间戳+原因）。清除=获取成功（簇级）∨
	// handleDeleteSeed（簇级·删 metadata 同步清印）。刀1 只加列不写不读
	// （②生效在刀2）；快照是未获取颗唯一宿主（无 metadata 行）。
	FetchFailedAt *time.Time `json:"fetch_failed_at" gorm:"index"`
	FetchError    string     `json:"fetch_error" gorm:"size:500;default:''"`
	LastSeen      time.Time  `json:"last_seen"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func (TorrentSnapshot) TableName() string { return "torrent_snapshots" }
