package db

import (
	"gorm.io/gorm"
)

// §59.183 · migration 38: sites 表加 download_hourly_limit（站点级 .torrent 下载限流，
// 次/小时；0=不限，默认 95）。优堡等实测无此严限制的站点可在站点管理-详情-网络调高；
// 孤儿恢复批量下载曾被全局 95/hour 误拦（PT30 实证）。
func addDownloadHourlyLimit(gormDB *gorm.DB) error {
	type colCheck struct{ Count int }
	var c colCheck
	if err := gormDB.Raw("SELECT COUNT(*) as count FROM pragma_table_info('sites') WHERE name='download_hourly_limit'").Scan(&c).Error; err != nil {
		return err
	}
	if c.Count > 0 {
		return nil // 幂等
	}
	return gormDB.Exec("ALTER TABLE sites ADD COLUMN download_hourly_limit INTEGER NOT NULL DEFAULT 95").Error
}

func init() {
	RegisterMigration(38, "add_download_hourly_limit", addDownloadHourlyLimit)
}
