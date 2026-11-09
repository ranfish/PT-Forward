package db

import "gorm.io/gorm"

// §59.501: sites 表加 upload_auth 列——好多油临时上传 Basic Auth 凭证
// （"user:pass" 整体·model encrypted:"true" GORM 通道加密存储）。
// 站点放开普通上传后用户清空字段即终（消费端空凭证零行为·无需版本清理）。
// 列存在防重（migration 19 同款 pragma 探测——AutoMigrate 先建列场景）。
func init() {
	RegisterMigration(156, "add_sites_upload_auth", func(gormDB *gorm.DB) error {
		var hasCol int64
		gormDB.Raw("SELECT COUNT(*) FROM pragma_table_info('sites') WHERE name='upload_auth'").Scan(&hasCol)
		if hasCol > 0 {
			return nil
		}
		return gormDB.Exec("ALTER TABLE sites ADD COLUMN upload_auth text").Error
	})
}
