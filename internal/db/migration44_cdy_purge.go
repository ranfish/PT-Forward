package db

import (
	"gorm.io/gorm"
)

// §59.318 N2: 传道院（pt.cdy.skin）下线铁律补执行——migration33 仅禁用未删行
// （243/29 均有残留）。传道院与修道院（xdypt.vip）是两个不同站点（历史混淆
// 源），按"站点下线=删除记录+移除支持"铁律 DELETE；种子已同步移除不再重建。
func init() {
	RegisterMigration(44, "cdy_purge", func(gormDB *gorm.DB) error {
		return gormDB.Exec("DELETE FROM sites WHERE domain = 'pt.cdy.skin'").Error
	})
}
