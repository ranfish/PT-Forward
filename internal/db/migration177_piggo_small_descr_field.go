package db

import (
	"gorm.io/gorm"
)

// migration177 §59.541 首颗实战: small_descr 域映射补配（副标题断流根修）。
//
// 案：首颗真实发布 Obsession 422「请填写副标题」——素材副标题在场
// （meta.subtitle 非空）但 piggo form_fields 未配 small_descr 域 →
// executor setForm 守卫跳过（域不存在不写）→ :641 pubReq.Subtitle 读
// form[small_descr] 恒空 → adapter addField("small_descr","") → 站方
// 必填校验 422。与 migration176（description 缺域空键）同族第二例：
// **form_fields 缺域=文本域断链**——契约必填文本域必须配域。
//
// 修：补 "small_descr": "small_descr" 映射——meta.Subtitle 经 setForm
// 进 form["small_descr"]，pubReq.Subtitle 贯通，DryRun form 可预览。
// 空 subtitle 素材仍走站方 422 透出（piggo.md §摸站已知行为·不拦）。
func init() {
	RegisterMigration(177, "piggo_small_descr_field", func(gormDB *gorm.DB) error {
		return gormDB.Exec(`UPDATE sites
			SET publish_form_config = json_set(publish_form_config,
				'$.form_fields.small_descr', 'small_descr'),
			    updated_at = datetime('now')
			WHERE domain = 'piggo.me'`).Error
	})
}
