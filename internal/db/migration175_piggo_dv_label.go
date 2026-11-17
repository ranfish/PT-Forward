package db

import (
	"gorm.io/gorm"
)

// migration175 §59.540 附六·三轮回归: processing 域 DV 行 label 对齐站方原文
// 「DV（杜比视界)」（catalog 2026-11-16 快照——站方括号不对称原样保留·站点
// 数据原样写入铁律）。migration174 首版误写缩写「DV」——value(9) 正确不影响
// 上传，但值集对账工具（formdiff/-metadata）每轮报假阳性，真漂移来了没人信
// （审计面噪声也是缺陷·§59.1476 财神 diff=0 先例）。
//
// 幂等：json_set 原子替换该行 label，重跑无害。
func init() {
	RegisterMigration(175, "piggo_dv_label_align", func(gormDB *gorm.DB) error {
		return gormDB.Exec(`UPDATE sites
			SET publish_form_config = json_set(publish_form_config,
				'$.value_mappings.processing[3].label', 'DV（杜比视界)'),
			    updated_at = datetime('now')
			WHERE domain = 'piggo.me'`).Error
	})
}
