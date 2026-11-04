package db

import (
	"gorm.io/gorm"
)

// migration144 §59.461: 朱雀 form_config.enabled 刷正——migration143 漏写
// enabled 字段（§59.318 is_target 双义拆分：发布目标单真相=form_config.
// enabled，is_target=1 只是门槛之一）——DryRun"目标站未启用发布配置"
// 实战抓出。幂等：enabled 非真才写。
func init() {
	RegisterMigration(144, "zhuque_form_enabled", func(gormDB *gorm.DB) error {
		return gormDB.Exec(`UPDATE sites SET
			publish_form_config = json_set(publish_form_config, '$.enabled', json('true')),
			updated_at = datetime('now')
			WHERE domain = 'zhuque.in'
			AND json_extract(publish_form_config, '$.enabled') IS NOT json('true')`).Error
	})
}
