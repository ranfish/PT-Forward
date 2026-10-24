package db

import (
	"gorm.io/gorm"
)

// §59.340 C 补: keepfrds 站点级 RPS 存量刷正（随版本自动下发）。
// migration50 已在 v0.0.1173 环境（243/29）执行过建列——补刷走本独立
// migration（已执行的不重跑）。幂等：仅刷 domain 含 keepfrds 且未手动
// 配置（fetch_rps=0）的行——用户已设值环境不覆盖。域名匹配（跨环境
// 站名可能不同，域名稳定）。0.0667 RPS=4 请求/分钟=流控探究惩罚带上限。
func init() {
	RegisterMigration(51, "fetch_rps_seed", func(gormDB *gorm.DB) error {
		return gormDB.Exec("UPDATE sites SET fetch_rps = 0.0667 WHERE domain LIKE '%keepfrds%' AND fetch_rps = 0").Error
	})
}
