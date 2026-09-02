package db

import (
	"gorm.io/gorm"
)

// §59.166 · migration 31: sites 表加 publish_interval_seconds（一站多种批量发布
// 种间间隔——站点级配置，站点详情-发布配置卡片滚轮 1-60 秒，默认 1）。
// 用户定案：串行+间隔可配（NP 站连续上传反作弊风险——节奏拟人）。
func addPublishIntervalSeconds(gormDB *gorm.DB) error {
	type colCheck struct{ Count int }
	var c colCheck
	if err := gormDB.Raw("SELECT COUNT(*) as count FROM pragma_table_info('sites') WHERE name='publish_interval_seconds'").Scan(&c).Error; err != nil {
		return err
	}
	if c.Count > 0 {
		return nil // 幂等
	}
	return gormDB.Exec("ALTER TABLE sites ADD COLUMN publish_interval_seconds INTEGER NOT NULL DEFAULT 1").Error
}

func init() {
	RegisterMigration(31, "add_publish_interval_seconds", addPublishIntervalSeconds)
}
