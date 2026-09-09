package db

import (
	"gorm.io/gorm"
)

// §59.183 · migration 39: download_hourly_limit 语义修订（v0.0.904）——
// 0 从"沿用全局默认 95"改为"不限"（自然语义），DB 默认值 0→95。
// migration 38 旧版（v0.0.903，DEFAULT 0）已跑过的库存量行为 0，
// 统一拉回 95；新装走修订后的 38（DEFAULT 95）本迁移无行可更，幂等空跑。
func setDownloadHourlyLimitDefault(gormDB *gorm.DB) error {
	type colCheck struct{ Count int }
	var c colCheck
	if err := gormDB.Raw("SELECT COUNT(*) as count FROM pragma_table_info('sites') WHERE name='download_hourly_limit'").Scan(&c).Error; err != nil {
		return err
	}
	if c.Count == 0 {
		return nil // 38 未跑（理论不可达——注册顺序保证），防御
	}
	return gormDB.Exec("UPDATE sites SET download_hourly_limit = 95 WHERE download_hourly_limit = 0").Error
}

func init() {
	RegisterMigration(39, "download_hourly_limit_default_95", setDownloadHourlyLimitDefault)
}
