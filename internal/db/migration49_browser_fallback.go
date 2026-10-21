package db

import (
	"gorm.io/gorm"
)

// §59.336: 浏览器 fallback 开关列——详情页 HTTP 失败时用内置
// Chromium 兜底（keepfrds 案：CF 对详情页检测严于首页/RSS，
// azuretls Chrome 不足，真实浏览器 + Cookie 实测一次通过）。
// 通用能力非 keepfrds 专属——任何站遇到同型问题可开。
// 幂等：model AutoMigrate 已建列则跳过（新装测试先跑 AutoMigrate）。
func init() {
	RegisterMigration(49, "browser_fallback", func(gormDB *gorm.DB) error {
		var cnt int64
		gormDB.Raw("SELECT COUNT(*) FROM pragma_table_info('sites') WHERE name = 'browser_fallback'").Scan(&cnt)
		if cnt > 0 {
			return nil
		}
		return gormDB.Exec("ALTER TABLE sites ADD COLUMN browser_fallback BOOLEAN DEFAULT 0").Error
	})
}
