package db

import (
	"encoding/json"

	"gorm.io/gorm"
)

// migration162 §59.523: 财神（cspt.top）tags「杜比(11)」补 standard_keys——
// Flight 2012 DoVi HDR10+ 案被站方拒「MediaInfo 检测到 Dolby Vision,请选择
// 杜比标签」。与 §59.520 织梦杜比零键同族：适配期已知「DV 种站方要求勾选」
// （cspt.md :1340）却漏接线。
//
// 语义（cspt.md :812 权威）：杜比 = 支持杜比视界【或】杜比全景声——
// 双键并入 [tag.dolby_vision, tag.dolby_atmos]。
//
// 顺路审计（值集 25 项全对账）：HDR(7) 已配四键 [hdr10,hdr10_plus,
// hdr_vivid,hlg] ✓；国语/粤语/DIY/禁转/完结已配 ✓；喜剧(8) 零键=正确
// （csptGenreTagValue label 直配注入通道 §59.400）；首发/分集零键=铁律
// （首发禁勾/§59.424 禁发分集）；其余零键=无对应键或不该自动投。
//
// 幂等：已含目标键则跳过。全量 map 拉出改回（保 form_config 全字段）。
func init() {
	RegisterMigration(162, "cspt_dolby_keys", func(gormDB *gorm.DB) error {
		var cfgStr string
		if err := gormDB.Raw(
			`SELECT publish_form_config FROM sites WHERE domain = 'cspt.top' AND publish_form_config IS NOT NULL`,
		).Scan(&cfgStr).Error; err != nil {
			return err
		}
		if cfgStr == "" {
			return nil
		}
		var cfg map[string]interface{}
		if err := json.Unmarshal([]byte(cfgStr), &cfg); err != nil {
			return nil
		}
		vm, ok := cfg["value_mappings"].(map[string]interface{})
		if !ok {
			return nil
		}
		list, ok := vm["tags"].([]interface{})
		if !ok {
			return nil
		}
		want := []string{"tag.dolby_vision", "tag.dolby_atmos"}
		for _, item := range list {
			m, ok := item.(map[string]interface{})
			if !ok || m["label"] != "杜比" {
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
			changed := false
			for _, k := range want {
				if !existing[k] {
					merged = append(merged, k)
					changed = true
				}
			}
			if !changed {
				return nil
			}
			m["standard_keys"] = merged
			out, err := json.Marshal(cfg)
			if err != nil {
				return err
			}
			return gormDB.Exec(
				`UPDATE sites SET publish_form_config = ?, updated_at = datetime('now') WHERE domain = 'cspt.top'`,
				string(out)).Error
		}
		return nil
	})
}
