package db

import (
	"strings"

	"gorm.io/gorm"
)

// migration184 §59.577 刀1: 外站组支持地基三件。
//
// ① 建表 external_release_groups + 19 组冷启动（2026-11-25 842 种子普查
//
//	的未识别长尾：21 组 - PigGoWEB（mappings 容错行）- TNND@CMCT.iso
//	（.iso 顺修后 @子组归已知 CMCT）= 19 真外站组）。home_site 仅
//	高置信标注（PTP/1442→passthepopcorn.me）·其余空（信息性字段·
//	不参与运行时路由）。source='census'（普查来源）。
//
// ② PigGoWEB 笔误容错行收编 mappings（§59.577：发布者手写文件名笔误·
//
//	站方标题正确 PigoWEB 单 G——migration181 DSWeb 先例同款·
//	销 §59.540 缺口②）。
//
// ③ torrent_metadata.release_group 升列（三级链第Ⅰ级数据通道——
//
//	detail_source_json.release_group 标准键形态 team.xxx/team.other）
//	+ 存量 backfill（json_extract）。
//
// 注意：不写 fetch 写入侧（列由 backfill+新获取写入双通道——
// 写入侧在同刀 fetch 落库代码补·本 migration 只负责存量）。
func init() {
	RegisterMigration(184, "external_release_groups_foundation", func(gormDB *gorm.DB) error {
		if err := gormDB.Exec(`CREATE TABLE IF NOT EXISTS external_release_groups (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			group_name TEXT NOT NULL,
			home_site TEXT DEFAULT '',
			source TEXT DEFAULT 'manual',
			notes TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`).Error; err != nil {
			return err
		}
		// §59.577 刀1: 19 组冷启动（census 2026-11-25·243 快照普查）
		groups := []struct{ name, home, notes string }{
			{"PTP", "passthepopcorn.me", "PTP 官方组"},
			{"1442", "passthepopcorn.me", "PTP 内部组"},
			{"SMURF", "", "国际 encoder 组"},
			{"HiDt", "", "国际 encoder 组"},
			{"FLUX", "", "国际 encoder 组"},
			{"ADDICTION", "", "国际组"},
			{"TGP", "", "国际组"},
			{"SHOWTiME", "", "国际组"},
			{"B0MBARDiERS", "", "scene 组"},
			{"12GaugeShotgun", "", "国际组"},
			{"SwRd", "", "国际组"},
			{"AdBlue", "", "国际组"},
			{"XTC_iNT", "", "scene iNT 形态"},
			{"MAMA", "", "国际组"},
			{"TMT", "", "国际组"},
			{"SPHD", "", "国际组"},
			{"HYPNOKROETE", "", "国际组（德语区形态）"},
			{"PiG30N", "", "国际组"},
			{"PtBM", "", "国际组"},
		}
		for _, g := range groups {
			if err := gormDB.Exec(`INSERT OR IGNORE INTO external_release_groups
				(group_name, home_site, source, notes, created_at, updated_at)
				VALUES (?, ?, 'census', ?, datetime('now'), datetime('now'))`,
				g.name, g.home, g.notes).Error; err != nil {
				return err
			}
		}
		// ② PigGoWEB 笔误容错行（mappings·migration181 同款形态）
		if err := gormDB.Exec(`INSERT OR IGNORE INTO release_group_mappings
			(group_name, domain, site_name, is_official, is_builtin, created_at, updated_at)
			VALUES ('PigGoWEB', 'piggo.me', '二师兄', 1, 1, datetime('now'), datetime('now'))`).Error; err != nil {
			return err
		}
		// ③ release_group 升列 + backfill（json_extract 从存量 detail_source_json 抽出）
		if err := gormDB.Exec(`ALTER TABLE torrent_metadata ADD COLUMN release_group TEXT DEFAULT ''`).Error; err != nil {
			// 列已存在（重复跑）——sqlite 报 duplicate column·容忍继续
			if !isDuplicateColumnErr(gormDB, err) {
				return err
			}
		}
		return gormDB.Exec(`UPDATE torrent_metadata SET release_group = COALESCE(
			json_extract(detail_source_json, '$.release_group'), '')`).Error
	})
}

// isDuplicateColumnErr SQLite ALTER 重复列判定（幂等重跑容忍）。
func isDuplicateColumnErr(gormDB *gorm.DB, err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "duplicate column")
}
