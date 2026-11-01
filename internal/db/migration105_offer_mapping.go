package db

import (
	"gorm.io/gorm"
)

// migration105 §59.421: 候选发布（offer）字段映射分发——优堡+家园。
// 用户讨论定案：两站 offers/asoffer checkbox 语义相同（勾=自愿进候选区
// 站方管理员审核/不勾=直接发布），默认不勾保持现状；站点详情页暴露
// "候选发布"开关（对齐匿名发布 uplver 模式）——开启则提交时自动勾选。
// 本 migration 补两站 form_fields["offer"] 映射（checkbox 字段名不同：
// 优堡 asoffer/家园 offers——executor 经 FieldDomainOffer 域取映射注入
// form[字段名]="yes"）。
// 优堡候选积分制（积分<10 自动进候选区）与家园 Crazy User 等级门槛为
// 站方后端强制路由层——与本开关（自愿候选）独立互补。
// 幂等：json_set 重入同值。
func init() {
	RegisterMigration(105, "offer_field_mapping", func(gormDB *gorm.DB) error {
		return gormDB.Exec(`
			UPDATE sites SET publish_form_config = json_set(publish_form_config, '$.form_fields.offer', 'asoffer'), updated_at = datetime('now') WHERE domain = 'ubits.club';
			UPDATE sites SET publish_form_config = json_set(publish_form_config, '$.form_fields.offer', 'offers'), updated_at = datetime('now') WHERE domain = 'hdhome.org';
		`).Error
	})
}
