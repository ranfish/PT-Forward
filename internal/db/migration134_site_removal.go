package db

import (
	"gorm.io/gorm"
)

// migration134 §59.452: 瞬间（www.momentpt.top）+昆仑（yhpp.cc）双站
// 关站下线（用户定案 2026-11-02）——按站点下线铁律（§59.362/363 先例）
// 存量清行：两站均无目标站适配（publish_form_config 0B 未开闸——零
// form_config 关联面）+无 docs/32 适配文档；29 实证两站 enabled=1 在
// 用中（is_target=0）。复活时按新站重新走接入流程（supported_sites
// 条目已移除——系统不再认识）。
// 关联资产四件同步：supported_sites.json 两条目/sites.json 两站条目
// （104→102——Moment 官组随瞬间条目消亡）/sites_source_keys 两文件删
// 除（§59.365 资产完整性先例）。
// 幂等：DELETE 语义天然幂等。
func init() {
	RegisterMigration(134, "momentpt_yhpp_site_removal", func(gormDB *gorm.DB) error {
		if err := gormDB.Exec(`DELETE FROM sites WHERE domain IN ('www.momentpt.top', 'yhpp.cc')`).Error; err != nil {
			return err
		}
		return gormDB.Exec(`DELETE FROM release_group_mappings WHERE site_name IN ('瞬间', '昆仑')`).Error
	})
}
