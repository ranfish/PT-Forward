package db

import "gorm.io/gorm"

// §59.369: 大青虫（cyanbug.net）源站角色刷正——18 个"制作组"全为其它站
// 官组（HDS/CHD/CMCT/WiKi/LeagueWEB/HHWEB/SharkWEB/ADWeb 等），自身无官组
// = 转载站（领域定义：源站必须有官组，无官组=转载站）。存量 is_source=1
// 为历史误标（docs/32/cyanbug.md 本就是发布表单分析=按目标站适配的站）。
// 源头 sites.json 已同步改 false（防 seed 链 OTA 重启刷回）。
func init() {
	RegisterMigration(59, "cyanbug_source_fix", func(gormDB *gorm.DB) error {
		return gormDB.Exec(`UPDATE sites SET is_source = 0 WHERE domain = 'cyanbug.net'`).Error
	})
}
