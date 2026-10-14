package db

import (
	"encoding/json"

	"gorm.io/gorm"
)

// §59.325: MinBD 精确键刷正——migration27 建幸运站时 medium.json 无
// medium.minibd canonical（"MiniBD 待标注"），后期 dict 已补（MiniBD →
// medium.minibd）。给幸运站 publish_form_config 的 medium 域 MiniBD
// 选项补 standard_keys（True.Grit MinBD1080p 案站方 TYPE_MISMATCH——
// 发布链 hasMinBDToken → medium.minibd → 站点选项命中）。
// 幂等：已有 standard_keys 跳过；非 MiniBD 选项不动；map 形态全字段保留。
func init() {
	RegisterMigration(45, "luckpt_minibd_stdkey", func(gormDB *gorm.DB) error {
		var rows []struct {
			Name              string
			PublishFormConfig string
		}
		if err := gormDB.Table("sites").
			Where("name = ? AND publish_form_config != ''", "幸运").
			Find(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			var cfg map[string]any
			if err := json.Unmarshal([]byte(row.PublishFormConfig), &cfg); err != nil {
				continue
			}
			vm, ok := cfg["value_mappings"].(map[string]any)
			if !ok {
				continue
			}
			mediums, ok := vm["medium"].([]any)
			if !ok {
				continue
			}
			changed := false
			for _, m := range mediums {
				opt, ok := m.(map[string]any)
				if !ok || opt["label"] != "MiniBD" {
					continue
				}
				if sk, exists := opt["standard_keys"].([]any); exists && len(sk) > 0 {
					continue
				}
				opt["standard_keys"] = []string{"medium.minibd"}
				changed = true
			}
			if !changed {
				continue
			}
			out, err := json.Marshal(&cfg)
			if err != nil {
				continue
			}
			if err := gormDB.Exec("UPDATE sites SET publish_form_config = ? WHERE name = ?",
				string(out), row.Name).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
