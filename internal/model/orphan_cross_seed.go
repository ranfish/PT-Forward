package model

import "time"

// OrphanCrossSeedRecord §59.359: 孤儿跨目录做种记录——下载器 infohash 仲裁
// 结果的持久化。恢复链 precheckCrossSeed 预查命中（同 hash 已在下载器其它
// 目录做种）时落表；扫描侧读表为孤儿 Entry 标 CrossSeeded 提示，用户下次
// 扫描即见"已在其它目录做种"不再困惑。是否删除冗余副本由用户决定——
// 系统零自动处置。
type OrphanCrossSeedRecord struct {
	ID          uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	ClientUID   uint      `gorm:"not null;uniqueIndex:idx_orphan_xseed_client_path" json:"client_uid"`
	OrphanPath  string    `gorm:"size:500;not null;uniqueIndex:idx_orphan_xseed_client_path" json:"orphan_path"`
	InfoHash    string    `gorm:"size:40;not null" json:"info_hash"`
	SeededPath  string    `gorm:"size:500;not null" json:"seeded_path"`
	TorrentName string    `gorm:"size:300" json:"torrent_name"`
	CreatedAt   time.Time `gorm:"not null" json:"created_at"`
}
