package db

import (
	"gorm.io/gorm"
)

// §59.317 P2: 馒头 site_field_mappings cat 存量清理——旧 23 行携带错误 ID 语义
// （v0.0.255 前误抄的表单快照：421=漫画/434=微卫星/449=AI 等，与馒头 API
// categoryList 权威树完全错位）。sites.json form.category 已按权威树重建
// （22 叶子），SeedFieldMappings 幂等键=label 不自动更新旧行——本迁移删除
// 馒头 cat 全部旧行，启动 seed 以新 label 重建。馒头发布未适配（publish_
// form_config 空），删除零消费风险。
func init() {
	RegisterMigration(42, "mteam_cat_field_mappings_rebuild", func(gormDB *gorm.DB) error {
		return gormDB.Exec("DELETE FROM site_field_mappings WHERE site_name = '馒头' AND field_type = 'cat'").Error
	})
}
