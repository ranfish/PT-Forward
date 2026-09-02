package db

import "gorm.io/gorm"

// §59.166 · migration 32: publish_result_records 加 pieces_hash 列
// （dedup 本地记忆——修道院 18:49 批实战：站方 API 密集查询下间歇空返回致 4 种
// 重传；本地记忆零依赖站方，一种多站/一站多种发布链公共生效）。
func addPiecesHashMemory(gormDB *gorm.DB) error {
	type colCheck struct{ Count int }
	var c colCheck
	if err := gormDB.Raw("SELECT COUNT(*) as count FROM pragma_table_info('publish_result_records') WHERE name='pieces_hash'").Scan(&c).Error; err != nil {
		return err
	}
	if c.Count > 0 {
		return nil // 幂等
	}
	return gormDB.Exec("ALTER TABLE publish_result_records ADD COLUMN pieces_hash VARCHAR(40) DEFAULT ''").Error
}

func init() {
	RegisterMigration(32, "add_pieces_hash_memory", addPiecesHashMemory)
}
