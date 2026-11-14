package db

import (
	"encoding/json"

	"gorm.io/gorm"
)

// migration165 §59.526: 特效字幕专项审计补漏——按精确归纳（特效字幕不做技术
// 推断·三通道声明保真：源站标签透传+副标声明词形+声明区声明词形）做三步
// 全量审计：产出有源（dict 词条）→73 站特效族词条 28 个→键位比对。
//
// 结果：26 已接·2 漏（聆音「特效」/蟹黄堡「特效字幕」）——均为 §59.525
// 审计输出已列出但 migration164 patches 漏收（实施遗漏·本刀补齐）。
//
// 幂等：词条已含目标键跳过。全量 map 拉出改回（保全字段）。
func init() {
	RegisterMigration(165, "effects_subs_wiring_fix", func(gormDB *gorm.DB) error {
		patches := map[string]map[string][]string{
			"pt.soulvoice.club": {"特效": {"tag.special_effects_subs"}},
			"crabpt.vip":        {"特效字幕": {"tag.special_effects_subs"}},
		}
		for domain, labelKeys := range patches {
			var cfgStr string
			if err := gormDB.Raw(
				`SELECT publish_form_config FROM sites WHERE domain = ? AND publish_form_config IS NOT NULL`,
				domain).Scan(&cfgStr).Error; err != nil {
				return err
			}
			if cfgStr == "" {
				continue
			}
			var cfg map[string]interface{}
			if err := json.Unmarshal([]byte(cfgStr), &cfg); err != nil {
				continue
			}
			vm, ok := cfg["value_mappings"].(map[string]interface{})
			if !ok {
				continue
			}
			list, ok := vm["tags"].([]interface{})
			if !ok {
				continue
			}
			changed := false
			for _, item := range list {
				m, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				label, _ := m["label"].(string)
				want, ok := labelKeys[label]
				if !ok {
					continue
				}
				keys, _ := m["standard_keys"].([]interface{})
				merged := make([]interface{}, 0, len(keys)+len(want))
				existing := map[string]bool{}
				for _, k := range keys {
					if s, ok := k.(string); ok {
						existing[s] = true
						merged = append(merged, s)
					}
				}
				for _, k := range want {
					if !existing[k] {
						merged = append(merged, k)
						changed = true
					}
				}
				if len(merged) > 0 {
					m["standard_keys"] = merged
				}
			}
			if !changed {
				continue
			}
			out, err := json.Marshal(cfg)
			if err != nil {
				return err
			}
			if err := gormDB.Exec(
				`UPDATE sites SET publish_form_config = ?, updated_at = datetime('now') WHERE domain = ?`,
				string(out), domain).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
