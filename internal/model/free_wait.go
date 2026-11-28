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
	ClientUID      uint   `gorm:"not null;default:0"`
	SubscriptionID string `gorm:"size:50;not null"`
	HasHR          bool
	HRSeedTimeH    int
	CheckBefore    *time.Time
	CheckCount     int        `gorm:"default:0"`
	RecheckSec     int        `gorm:"default:600"`
	LastCheckAt    *time.Time `gorm:"index"`
	// §59.594（审计 20 刀1·V-01 根修）: 可行性检查阈值持久化——设计 P2
	// （docs/31:65897）加保护时漏接 P1 的「运行时字段↔DB 字段」配对纪律，
	// 重启后 RecoverOnStartup 无从回填→归零→「剩余过短跳过」静默失效。
	// default:30 与 model/rss.go:135 配置默认一致；存量行由 AutoMigrate
	// 补列填 30（合理近似——真实入队阈值当初未存不可追回·docs/31 §59.594）。
	MinRemainMin int `gorm:"default:30"`
}
