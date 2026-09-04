package db

import "gorm.io/gorm"

// §59.167 · migration 36: 传道院下线终态——删除 sites 记录（用户定案语义：
// 下线/取消站点支持 = 全系统不可见；此前 enabled=0 仅"未启用"仍占列表位）。
// 历史发布记录（target_site 字符串关联）不受影响可查。supported_sites 的
// blocked 拦新增保留（双保险：删存量+拦再增）。
func removeCdySite(gormDB *gorm.DB) error {
	return gormDB.Exec("DELETE FROM sites WHERE domain = 'pt.cdy.skin'").Error
}

func init() {
	RegisterMigration(36, "remove_cdy_site", removeCdySite)
}
