package db

import (
	"gorm.io/gorm"
)

// §59.333: TLS/H2 指纹伪装列——keepfrds 试点（Cloudflare 指纹库识别
// Go 三层特征致 empty_response 267 次/天）。默认空（全站零变化）。
// 幂等：model AutoMigrate 已建列则跳过（新装测试先跑 AutoMigrate）。
func init() {
	RegisterMigration(48, "tls_pretend", func(gormDB *gorm.DB) error {
		var cnt int64
		gormDB.Raw("SELECT COUNT(*) FROM pragma_table_info('sites') WHERE name = 'tls_pretend'").Scan(&cnt)
		if cnt > 0 {
			return nil
		}
		return gormDB.Exec("ALTER TABLE sites ADD COLUMN tls_pretend TEXT DEFAULT ''").Error
	})
}
