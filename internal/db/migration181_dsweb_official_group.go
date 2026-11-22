package db

import (
	"gorm.io/gorm"
)

// migration181 §59.550: DSWeb 深度官组显式收编 + 前缀回退机制退役配套。
//
// 用户实态核查（2026-11-22）：DSWeb 为 dstudio.me 深度官组——站方为
// 全官组源站·组搜索箱未显式列出 DSWeb（采集漏采原因）·但以 DSWeb
// 搜索有大量种子（实存证据）。
//
// 配套：前缀回退（FuzzyGroupPrefixKey·CMCTf→CMCT）退役——用户定案
// 「系统不能推导官组·必须显式映射（开发者决定）」。数据面审计：退役
// 唯一受影响面=DSWeb（4 行·前缀命中 ds）——本 migration 收编后零丢失。
func init() {
	RegisterMigration(181, "dsweb_deep_official_group", func(gormDB *gorm.DB) error {
		return gormDB.Exec(`INSERT OR IGNORE INTO release_group_mappings
			(group_name, domain, site_name, is_official, is_builtin, created_at, updated_at)
			VALUES ('DSWeb', 'dstudio.me', '', 1, 1, datetime('now'), datetime('now'))`).Error
	})
}
