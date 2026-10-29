package db

import "gorm.io/gorm"

// §59.368: RSS 订阅停摆可观测——rss_fetch_logs 加 detect_count/
// detect_failures 两列（历史行 DEFAULT 0=健康轮语义，不产生假红；
// 隐性停摆历史不回溯，从部署起积累——设计定案 docs/31 §59.368）。
// 幂等：新装链 AutoMigrate 可能已建列（model 已声明）——查列存在再
// ALTER（§59.361 同款，TestNewInstallFullMigrations 实证）。
func init() {
	RegisterMigration(58, "rss_detect_stats", func(gormDB *gorm.DB) error {
		hasCol := func(col string) bool {
			var cnt int64
			gormDB.Raw("SELECT COUNT(*) FROM pragma_table_info('rss_fetch_logs') WHERE name = ?", col).Scan(&cnt)
			return cnt > 0
		}
		if !hasCol("detect_count") {
			if err := gormDB.Exec(`ALTER TABLE rss_fetch_logs ADD COLUMN detect_count integer DEFAULT 0`).Error; err != nil {
				return err
			}
		}
		if !hasCol("detect_failures") {
			if err := gormDB.Exec(`ALTER TABLE rss_fetch_logs ADD COLUMN detect_failures integer DEFAULT 0`).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
