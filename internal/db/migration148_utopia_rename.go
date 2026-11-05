package db

import (
	"gorm.io/gorm"
)

// migration148 §59.469: 站名系统性修改——「音乐乌托邦」→「乌托邦」（用户指令）。
//
// 29 库关联表零引用（torrent_metadata/seeding/coverage 等无该名行）——
// 仅 sites.name 单点 + seed 双 JSON 同步（data/sites.json ·
// supported_sites.json name_cn）。
//
// 幂等：条件写（旧名才 UPDATE）。
func init() {
	RegisterMigration(148, "utopia_rename", func(gormDB *gorm.DB) error {
		return gormDB.Exec(`UPDATE sites SET
			name = '乌托邦',
			updated_at = datetime('now')
			WHERE domain = 'www.musopia.vip' AND name = '音乐乌托邦'`).Error
	})
}
