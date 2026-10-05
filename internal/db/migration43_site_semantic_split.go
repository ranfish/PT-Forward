package db

import (
	"gorm.io/gorm"
)

// §59.318 D2: 站点语义拆分存量迁移——is_target 历史上兼职"辅种探测池"与
// "发布目标"两义（92 站种子预标 is_target=true 致统计虚高/发布语义错位）。
// ①新列 is_reseed_target 继承旧 is_target（探测行为零变化）
// ②is_target 收敛为 publish_form_config.enabled（发布目标真值——恰=已适配
// 且配置启用的站）；COALESCE 防 json_extract NULL 落脏
// ③DROP 死列 target_types（UI 可配三选项但引擎零消费——唯一消费方
// HasTargetType 为死代码；90 站值全为兜底默认，删除零损失）
// 列存在性双守卫：AutoMigrate 先行已加新列/新库无旧列时幂等跳过。
func init() {
	RegisterMigration(43, "site_semantic_split", func(gormDB *gorm.DB) error {
		var hasReseed int64
		gormDB.Raw("SELECT COUNT(*) FROM pragma_table_info('sites') WHERE name='is_reseed_target'").Scan(&hasReseed)
		if hasReseed == 0 {
			if err := gormDB.Exec("ALTER TABLE sites ADD COLUMN is_reseed_target numeric NOT NULL DEFAULT 0").Error; err != nil {
				return err
			}
		}
		if err := gormDB.Exec("UPDATE sites SET is_reseed_target = is_target").Error; err != nil {
			return err
		}
		// json_valid 三防护：空串/NULL/坏 JSON 均 malformed 报错（单测实证空串
		// 即炸）——非合法 JSON 一律收敛 0。
		if err := gormDB.Exec(`UPDATE sites SET is_target = CASE
			WHEN publish_form_config IS NOT NULL AND publish_form_config != '' AND json_valid(publish_form_config)
			THEN COALESCE(json_extract(publish_form_config, '$.enabled'), 0)
			ELSE 0 END`).Error; err != nil {
			return err
		}
		var hasTT int64
		gormDB.Raw("SELECT COUNT(*) FROM pragma_table_info('sites') WHERE name='target_types'").Scan(&hasTT)
		if hasTT > 0 {
			if err := gormDB.Exec("ALTER TABLE sites DROP COLUMN target_types").Error; err != nil {
				return err
			}
		}
		return nil
	})
}
