package db

import (
	"encoding/json"

	"github.com/ranfish/pt-forward/internal/model"
	"gorm.io/gorm"
)

// migration63 §59.376: type 域纪录片映射单复数双键——全站泛化刷正。
// §59.289 修道院案（migration40）同款重犯：龙之家（migration60）/壹吧
// （migration61）form_config 的 type 槽纪录片 standard_keys 只配复数
// category.documentaries（extended 采集归一形态），而发布消费 category
// 列 canonical=单数 category.documentary（dict/type.json 权威）——
// lookupByStdKey 精确匹配恒 miss → "类型映射缺失"（243 龙之家
// Afternoons of Solitude 2024 纪录片案，v0.0.1221 批次 1 条 failed）。
// 修复=migration40 同款双键（单数前插），泛化扫全站（修道院已双键
// 幂等跳过；未来存量站如有同款一并治）。幂等：已含单数键则跳过。
func init() {
	RegisterMigration(63, "type_documentary_dualkey_all", func(gormDB *gorm.DB) error {
		var sites []model.Site
		if err := gormDB.Where("publish_form_config != ''").Find(&sites).Error; err != nil {
			return nil
		}
		for _, site := range sites {
			var cfg model.PublishFormConfig
			if err := json.Unmarshal([]byte(site.PublishFormConfig), &cfg); err != nil {
				continue // 非 JSON——不动
			}
			typeMappings, ok := cfg.ValueMappings[model.FieldDomainType]
			if !ok {
				continue
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
				// 仅纪录片条目（有复数键）——补齐双键
				if hasPlural && !hasSingular {
					m.StandardKeys = append([]string{"category.documentary"}, m.StandardKeys...)
					typeMappings[i] = m
					changed = true
				}
			}
			if !changed {
				continue
			}
			cfg.ValueMappings[model.FieldDomainType] = typeMappings
			out, err := json.Marshal(&cfg)
			if err != nil {
				continue
			}
			if err := gormDB.Model(&model.Site{}).
				Where("id = ?", site.ID).
				Update("publish_form_config", string(out)).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
