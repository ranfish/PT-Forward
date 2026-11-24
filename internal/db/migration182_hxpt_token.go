package db

import (
	"gorm.io/gorm"
)

// migration182 §59.570: 好学 auth_type cookie→apikey（Token 全切配套）。
// 前端 needsApiKey 判定 authType==='apikey' 才显示 API Key 输入框。
func init() {
	RegisterMigration(182, "hxpt_auth_type_apikey", func(gormDB *gorm.DB) error {
		return gormDB.Exec("UPDATE sites SET auth_type = 'apikey' WHERE domain = 'www.hxpt.org' AND auth_type = 'cookie'").Error
	})
}
