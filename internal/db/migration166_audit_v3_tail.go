package db

import (
	"encoding/json"

	"gorm.io/gorm"
)

// migration166 §59.527: 三步法回归审计 v3 补尾——方法升级：语义规则从
// dict/tag.json 自动提取（variants+infer_pattern 全自动·消除手写 RULES
// 漏词形根源——13城「多语」即 bilingual_audio variants 含「多语」而前两轮
// 手写规则只有「中英双语/国英双语」）。
//
// v3 结果：8 条残留→2 补 6 挂账：
// - 补①13城「多语(10)」→ tag.bilingual_audio（真实漏网·自动规则抓到）
// - 补②天空「Atmos(21)」→ tag.dolby_atmos（复合 promo 族中的纯单语义
//   例外——词条语义=纯 Atmos·推理键 dolby_atmos 语义精确对应·无错配）
// - 挂账 6 条维持：次世代国语/全景声国语/临境声国语（复合词错配——普通
//   国语音轨命中「次世代国语」站方判次世代音轨会拒）/SL-HDR1（专有格式
//   HDR10 单层变体·接 hdr10 有 DoVi-SDR 错配面）/DoVi+HDR/DIY纯净版（复合）
//
// 幂等：词条已含目标键跳过。全量 map 拉出改回（保全字段）。
func init() {
	RegisterMigration(166, "audit_v3_tail", func(gormDB *gorm.DB) error {
		patches := map[string]map[string][]string{
			"13city.org":   {"多语": {"tag.bilingual_audio"}},
			"hdsky.me":     {"Atmos": {"tag.dolby_atmos"}},
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
