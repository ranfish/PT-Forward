package db

import "gorm.io/gorm"

// §59.363: 樱花（pt.ying.us.kg）+ 雨（raingfh.top）关站下线——删除 sites
// 记录+组映射行（站点下线铁律：全系统不可见；supported_sites.json/
// sites.json/适配器文档已同步移除）。历史发布记录不受影响可查。
func init() {
	RegisterMigration(55, "remove_ying_raing_sites", func(gormDB *gorm.DB) error {
		domains := []string{"pt.ying.us.kg", "raingfh.top"}
		if err := gormDB.Exec("DELETE FROM sites WHERE domain IN ?", domains).Error; err != nil {
			return err
		}
		return gormDB.Exec("DELETE FROM release_group_mappings WHERE domain IN ?", domains).Error
	})
}
