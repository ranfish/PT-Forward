package db

import (
	"encoding/json"

	"gorm.io/gorm"
)

// §59.326 附三: 幸运站英语标签 stdkeys 刷正——tags 表英语(value=22)
// 无 standard_keys，english_audio 映射 miss 致英语标签从未提交（Alice/
// SendHelp/造雨人 TAGS_MISSING_ENGLISH_AUDIO 三案反转定性：非站方噪声，
// 是我方映射缺口；与 migration45 MiniBD 案同型）。
// 幂等：已有 standard_keys 跳过。
func init() {
	RegisterMigration(46, "luckpt_english_tag_stdkey", func(gormDB *gorm.DB) error {
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
			tags, ok := vm["tags"].([]any)
			if !ok {
				continue
			}
			changed := false
			for _, m := range tags {
				opt, ok := m.(map[string]any)
				if !ok || opt["label"] != "英语" {
					continue
				}
				if sk, exists := opt["standard_keys"].([]any); exists && len(sk) > 0 {
					continue
				}
				opt["standard_keys"] = []string{"tag.english_audio"}
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
