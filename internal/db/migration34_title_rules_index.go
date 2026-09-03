package db

import "gorm.io/gorm"

// §59.167 · migration 34: title_rules 唯一索引重建（四列含 pattern）。
// 原三列键（site_code,rule_type,field）下同 field 多 pattern 规则必撞 UNIQUE
// （resolution 的 8K/4K 多条 regex——PT31 新装 AutoMigrate 建表实证：规则同步
// 部分失败。老库（243）同步查重掩盖未暴露，加新规则同样会撞）。
func rebuildTitleRulesIndex(gormDB *gorm.DB) error {
	// 幂等：索引已是新形态（AutoMigrate 新库已建四列索引）则跳过
	var cnt int64
	gormDB.Raw(`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='idx_rule_unique' AND sql LIKE '%pattern%'`).Scan(&cnt)
	if cnt > 0 {
		return nil
	}
	if err := gormDB.Exec("DROP INDEX IF EXISTS idx_rule_unique").Error; err != nil {
		return err
	}
	return gormDB.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_rule_unique ON title_rules(site_code, rule_type, field, pattern)").Error
}

func init() {
	RegisterMigration(34, "rebuild_title_rules_index", rebuildTitleRulesIndex)
}
