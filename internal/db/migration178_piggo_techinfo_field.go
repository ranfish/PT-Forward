package db

import (
	"gorm.io/gorm"
)

// migration178 §59.541 附六: technical_info 域补配（MI 双通道消除·用户目视定案）。
//
// 案：首战五连发后用户浏览器目视——{{ptgen}} 渲染块形态无瑕疵（销账）；
// MI 双通道重复（descr 引用块内 MI 与站方 media_info 字段重复）。
// 用户定案：简介块仅保留 引用+PTGen——MI 只走站方 media_info 字段。
//
// 修：补 "technical_info": "media_info"——executor assembleDescription 的
// 「无 techinfo 域站 MI 以引用格式入简介」分支（§59.166 修道院规范）被
// 禁用（有域→不嵌 descr）；adapter 已走 req.MediaInfo→addField("media_info")
// 独立通道不受影响。piggo descr 最终形态：声明引用块 + {{ptgen}} + 截图。
func init() {
	RegisterMigration(178, "piggo_techinfo_field", func(gormDB *gorm.DB) error {
		return gormDB.Exec(`UPDATE sites
			SET publish_form_config = json_set(publish_form_config,
				'$.form_fields.technical_info', 'media_info'),
			    updated_at = datetime('now')
			WHERE domain = 'piggo.me'`).Error
	})
}
