package db

import (
	"gorm.io/gorm"
)

// migration180 §59.541 附十二: 劳改所 descr_quote_only 开关。
//
// 用户人工修改 114977 详情页实态：站方有独立「豆瓣信息」渲染区（根据
// pt_gen 链接自动解析简介）——我们 descr 里的 PTGen 正文（1717 字）
// 是重复冗余，人工修改后仅留引用块（250 字）。
// 生产形态：descr = 引用块（Remux 来源声明+致谢）·无 PTGen 正文。
//
// 幂等：json_set 覆盖为 true（重复执行同值无害）。
func init() {
	RegisterMigration(180, "ptlgs_descr_quote_only", func(gormDB *gorm.DB) error {
		return gormDB.Exec(`UPDATE sites SET publish_form_config = json_set(
			publish_form_config,
			'$.descr_quote_only', json('true')
		), updated_at = datetime('now') WHERE domain = 'ptlgs.org'`).Error
	})
}
