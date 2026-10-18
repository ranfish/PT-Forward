package model

import "time"

// FetchSelectEvent §59.331: 源站层选事件——error 级日志环境的层选可观测
// （freeze_events 同款先例；pt4 案实证 info/debug 被吞后只能 DB 反向验证）。
type FetchSelectEvent struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	InfoHash  string    `gorm:"size:40;index;not null" json:"info_hash"`
	Layer     string    `gorm:"size:40;not null" json:"layer"`      // ①组映射+comment_tid 等
	GroupName string    `gorm:"size:50" json:"group_name"`
	SiteName  string    `gorm:"size:50" json:"site_name"`
	TorrentID string    `gorm:"size:50" json:"torrent_id"`
	Evidence  string    `gorm:"size:50" json:"evidence"`
	CreatedAt time.Time `json:"created_at"`
}

func (FetchSelectEvent) TableName() string { return "fetch_select_events" }
