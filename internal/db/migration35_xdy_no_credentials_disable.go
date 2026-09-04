package db

import "gorm.io/gorm"

// §59.167 · migration 35: 修道院无凭证存量矫正。
// 背景：migration 33 曾以 Enabled=true 建骨架（v0.0.855 时代库已跑——源码改
// false 对已跑库无效（migration 不重跑）——零凭证站滞留"我的站点"（PT31 实证：
// enabled=1+cookie 空+passkey 空）。矫正：无凭证的修道院回落未启用；用户已配
// 凭证的不动（该启用）。新装库不受影响（33 现版已 false）。
func disableXdyWithoutCredentials(gormDB *gorm.DB) error {
	return gormDB.Exec(`UPDATE sites SET enabled = 0, updated_at = CURRENT_TIMESTAMP
		WHERE domain = 'xdypt.vip' AND enabled = 1
		AND (cookie IS NULL OR cookie = '') AND (passkey IS NULL OR passkey = '')`).Error
}

func init() {
	RegisterMigration(35, "xdy_no_credentials_disable", disableXdyWithoutCredentials)
}
