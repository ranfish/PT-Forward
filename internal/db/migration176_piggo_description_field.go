package db

import (
	"gorm.io/gorm"
)

// migration176 §59.541 b1 附: description 域映射补配（DryRun 空键根修）。
//
// 案：猪猪 form_fields 未配 description 域 → executor :558
// `form[cfg.FormFields[description]] = assembleDescription(...)` 裸写无守卫
// （setForm :338 有守卫·裸写没有）→ 组装简介进 form[""]（空键）——:642
// pubReq.Description 读 form[""] 恰好贯通（隐形依赖·功能不断）但 DryRun
// form 出现脏空键。
//
// 修：补 "description": "descr" 映射——组装简介进 form["descr"]、DryRun
// 干净、pubReq.Description 贯通不断。无双写：PiggoAdapter 显式字段列表
// （req.Description 顶层消费）不遍历 FormFields。
func init() {
	RegisterMigration(176, "piggo_description_field", func(gormDB *gorm.DB) error {
		return gormDB.Exec(`UPDATE sites
			SET publish_form_config = json_set(publish_form_config,
				'$.form_fields.description', 'descr'),
			    updated_at = datetime('now')
			WHERE domain = 'piggo.me'`).Error
	})
}
