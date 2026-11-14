package db

import (
	"encoding/json"

	"gorm.io/gorm"
)

// migration168 §59.530: 字幕双语键接线（用户语义框架定案后正确对接）。
//
// 键设计见 dict/tag.json：bilingual_subtitle（泛·MI & 双码铁证+副标
// 声明词形双通道）/ bilingual_subtitle_chs_eng（简英精确·仅 MI）。
//
// 对接：
// - 垃圾堆「简英双语字幕」← 精确键 chs_eng（站方语义严格简中+英对照·
//   繁英对照 CHT&ENG 不勾——用户定案）
// - 五二「双语字幕」← 泛键（泛称·简英或繁英对照都算——用户定案）
// - 三月「Ai双语字幕」← 不接（双语可证·「AI 翻译」不可证——用户定案
//   半真的勾不勾·维持 §59.529 删键态）
//
// 双向核对第三形态自查：垃圾堆/五二各接一键·无同词条双键·无包含
// 关系键组同接 ✓。
//
// 幂等：键态已达成跳过。
func init() {
	RegisterMigration(168, "bilingual_subtitle_wiring", func(gormDB *gorm.DB) error {
		patches := map[string]map[string][]string{
			"pt.lajidui.top": {"简英双语字幕": {"tag.bilingual_subtitle_chs_eng"}},
			"52pt.site":      {"双语字幕": {"tag.bilingual_subtitle"}},
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
