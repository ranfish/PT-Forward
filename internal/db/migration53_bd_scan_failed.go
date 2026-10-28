package db

import (
	"gorm.io/gorm"
)

// §59.361 A: 原盘 BDInfo 扫描失败可观测——metadata 加失败记录两列。
// onDone 失败写（簇级）/成功清/重扫入队清；列表 API 透出 disc_scan_failed
// 三态（扫描中/失败/未扫）——失败显示"扫描失败"而非永转蓝圈。
func init() {
	RegisterMigration(53, "bd_scan_failed_columns", func(gormDB *gorm.DB) error {
		if err := gormDB.Exec("ALTER TABLE torrent_metadata ADD COLUMN bd_scan_failed_at datetime").Error; err != nil {
			return err
		}
		return gormDB.Exec("ALTER TABLE torrent_metadata ADD COLUMN bd_scan_error text").Error
	})
}
