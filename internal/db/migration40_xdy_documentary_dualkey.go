package db

import (
	"encoding/json"

	"github.com/ranfish/pt-forward/internal/model"
	"gorm.io/gorm"
)

// §59.289: 修道院 type 域纪录片映射单复数双键——dict L3 语义集 canonical=
// category.documentary（单数）而 migration 33 手写基线为复数 documentaries，
// 种子 category 列（单数）精确匹配恒 miss → type 字段不发 → 站方
// "Invalid integer format or integer overflow"（Untold Shooting Guards 三连
// 未知响应案）。幂等：已含单数键则跳过。
func init() {
	RegisterMigration(40, "xdy_documentary_dualkey", func(gormDB *gorm.DB) error {
		var site model.Site
		if err := gormDB.Where("name = ?", "修道院").First(&site).Error; err != nil {
			return nil // 站未配置——跳过
		}
		if site.PublishFormConfig == "" {
			return nil
		}
		var cfg model.PublishFormConfig
		if err := json.Unmarshal([]byte(site.PublishFormConfig), &cfg); err != nil {
			return nil // 非 JSON——不动
		}
		typeMappings, ok := cfg.ValueMappings[model.FieldDomainType]
		if !ok {
			return nil
		}
		changed := false
		for i, m := range typeMappings {
			hasSingular, hasPlural := false, false
			for _, k := range m.StandardKeys {
				if k == "category.documentary" {
					hasSingular = true
				}
				if k == "category.documentaries" {
					hasPlural = true
				}
			}
			// 仅纪录片条目（有复数键或标签含"纪录"）——补齐双键
			if hasPlural && !hasSingular {
				m.StandardKeys = append([]string{"category.documentary"}, m.StandardKeys...)
				typeMappings[i] = m
				changed = true
			}
		}
		if !changed {
			return nil
		}
		cfg.ValueMappings[model.FieldDomainType] = typeMappings
		out, err := json.Marshal(&cfg)
		if err != nil {
			return err
		}
		return gormDB.Model(&model.Site{}).
			Where("id = ?", site.ID).
			Update("publish_form_config", string(out)).Error
	})
}
