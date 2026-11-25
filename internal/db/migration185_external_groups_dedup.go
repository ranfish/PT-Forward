package db

import (
	"gorm.io/gorm"
)

// migration185 §59.577 附（1632·243 复验发现）: external_release_groups
// 去重+唯一索引补建。
//
// 案：v0.0.1630 的 243 crash loop 期间——每次重启 migration 184 重跑·
// INSERT OR IGNORE 因【无唯一索引】而无效（无约束冲突可忽略）→ 每轮
// 追加 19 行 → 40 轮 = 760 行（19 distinct × 40）。表由 migration 184
// 裸 SQL 创建（无 idx_ext_group_name——model tag 的 uniqueIndex 只有
// AutoMigrate 注册才会建·ExternalReleaseGroup 未进 AutoMigrate 清单）。
//
// 修：①去重（保留每组 MIN(id)——census 冷启动行优先）②建唯一索引
// idx_ext_group_name（此后 INSERT OR IGNORE 真幂等）。
// 注意：不注册进 AutoMigrate——AutoMigrate 在 migration 之前跑·脏数据
// 上建唯一索引会炸启动（AutoMigrate 失败=os.Exit=新 crash loop）。
// migration 拥有该表 schema 权威（同表创建本身即 migration 交付）。
func init() {
	RegisterMigration(185, "external_groups_dedup_unique_index", func(gormDB *gorm.DB) error {
		if err := gormDB.Exec(`DELETE FROM external_release_groups
			WHERE id NOT IN (SELECT MIN(id) FROM external_release_groups GROUP BY group_name)`).Error; err != nil {
			return err
		}
		return gormDB.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_ext_group_name
			ON external_release_groups(group_name)`).Error
	})
}
