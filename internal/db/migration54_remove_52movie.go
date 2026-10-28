package db

import "gorm.io/gorm"

// §59.362: 我爱电影（www.52movie.top）+ 忘年桥（www.ptlao.top）关站下线
// ——删除 sites 记录+组映射行（站点下线铁律：全系统不可见；
// supported_sites.json/sites.json/适配器文档已同步移除——复活时按新站
// 重新走接入流程）。历史发布记录（target_site 字符串关联）不受影响可查。
func init() {
	RegisterMigration(54, "remove_52movie_ptlao_sites", func(gormDB *gorm.DB) error {
		domains := []string{"www.52movie.top", "www.ptlao.top"}
		if err := gormDB.Exec("DELETE FROM sites WHERE domain IN ?", domains).Error; err != nil {
			return err
		}
		return gormDB.Exec("DELETE FROM release_group_mappings WHERE domain IN ?", domains).Error
	})
}
