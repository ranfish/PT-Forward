package db

import "gorm.io/gorm"

// migration151 §59.476: 音乐官组映射种子行（用户定案 2026-11-06）。
//
// 音乐官组词表来源改为 release_group_mappings 推导（单一真相源——
// S2+ 判定/①层组映射直达同资产）：LLM/TSxD/KHQ→皇后官组·
// Musopia→乌托邦官组（OpenCD→皇后已在）。is_official=true（如影视
// 官组语义）。MusopiaBAK 经用户实查无此组·不种。
//
// 幂等：group_name 全局唯一索引（idx_group_name）→ INSERT OR IGNORE；
// OpenCD 行补验 is_official。
func init() {
	RegisterMigration(151, "music_official_groups", func(gormDB *gorm.DB) error {
		if err := gormDB.Exec(`INSERT OR IGNORE INTO release_group_mappings
			(group_name, domain, site_name, is_official, is_builtin, created_at, updated_at)
			VALUES
			('LLM',     'open.cd',          '皇后',   1, 1, datetime('now'), datetime('now')),
			('TSxD',    'open.cd',          '皇后',   1, 1, datetime('now'), datetime('now')),
			('KHQ',     'open.cd',          '皇后',   1, 1, datetime('now'), datetime('now')),
			('Musopia', 'www.musopia.vip',  '乌托邦', 1, 1, datetime('now'), datetime('now'))`).Error; err != nil {
			return err
		}
		return gormDB.Exec(`UPDATE release_group_mappings SET is_official = 1
			WHERE group_name = 'OpenCD' AND site_name = '皇后'`).Error
	})
}
