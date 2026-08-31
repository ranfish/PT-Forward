package db

import (
	"gorm.io/gorm"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.155 回归审核补丁 · migration 28: 幸运 publish_form_config 补 tag_config_legacy。
//
// migration 27 遗漏：TagConfig{Mode} 未落库（幸运=checkbox_span——切片 2 TagApplier
// 切读 form_config 的 mode 依据；FormFields.tags 字段名形态可推断但显式优于隐式）。
// 27 已在 29/243 执行（schema_migrations 防重跑），故 28 增量补。
func luckptTagConfigLegacy(gormDB *gorm.DB) error {
	var site model.Site
	if err := gormDB.Where("name = ?", "幸运").First(&site).Error; err != nil {
		return nil // 非幸运环境跳过（与 27 同语义）
	}
	cfg := model.ParseFormConfig(site.PublishFormConfig)
	if cfg == nil {
		return nil // 配置为空（27 未跑过）——跳过
	}
	if cfg.TagConfig != nil {
		return nil // 已有（幂等）
	}
	cfg.TagConfig = &model.SiteTagConfig{
		Mode: model.TagModeCheckboxSpan,
		Tags: map[string]string{}, // 值映射已由 ValueMappings 承载——此处仅 mode 语义
	}
	return gormDB.Model(&model.Site{}).Where("id = ?", site.ID).
		Update("publish_form_config", cfg.Serialize()).Error
}

func init() {
	RegisterMigration(28, "luckpt_tag_config_legacy", luckptTagConfigLegacy)
}
