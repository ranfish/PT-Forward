package db

import (
	"encoding/json"

	"gorm.io/gorm"
)

// migration163 §59.524: tags 漏接线族全面审计批量补键——织梦（§59.520）/
// 财神（§59.523）两案同型触发：目标站 tags 值集「label 语义对应推理键但
// standard_keys 零键」的词条全量扫描（29 库 73 目标站），漏配候选 65 条
// 分层判别后补 16 站 33 词条。
//
// 分层判别（不补的三类）：
// ① 禁转族零键=正确设计（22 条）——我们发布的非禁转资源（R7 防线在发布前
//    拦截禁转源）·目标站禁转标签对我们永不勾
// ② 蟹黄堡未完结——§59.520 定案（单集语义≠连载+§59.424 场景不存在）
// ③ 天空 option_sel 复合 promo 标签（次世代国语/全景声国语等=技术+语言
//    复合语义）+慕雪阁完结日漫/国漫（分类复合）——不适用单键映射·挂账
//
// 键设计（先例对齐）：
// - 单「杜比/杜比视界/Dolby Vision」→ [dolby_vision, dolby_atmos]
//   （财神语义 DoVi∨Atmos 双键·§59.523）
// - 独立「杜比全景声/Dolby Atmos」词条 → dolby_atmos 单键（分立站：
//   慕雪阁/蟹黄堡/野马——站方 Vision/Atmos 分列·分键对齐）
// - 泛「HDR」→ hdr10（织梦先例 §59.520）；HDR10/HDR 10/HDR10bit → hdr10；
//   HDR10+ → hdr10_plus；HLG → hlg；英字 → english_subtitle
// - 冬樱「HDR+杜比视界」复合词条 → dolby_vision（DV 种 base=HDR10）
// - 推理源已验：dolby_vision（标题词形 DV/DoVi/杜比）+dolby_atmos
//   （infer_pattern atmos）都有产出通道（dict/tag.json）
//
// 幂等：词条已含目标键跳过。全量 map 拉出改回（保全字段）。
func init() {
	RegisterMigration(163, "tags_wiring_audit_batch", func(gormDB *gorm.DB) error {
		// domain → (label → 追加键)
		patches := map[string]map[string][]string{
			"52pt.site":            {"杜比视界": {"tag.dolby_vision"}, "HDR": {"tag.hdr10"}},
			"xdypt.vip":            {"HDR": {"tag.hdr10"}},
			"wintersakura.net":     {"HDR10": {"tag.hdr10"}, "HDR10+": {"tag.hdr10_plus"}, "HDR+杜比视界": {"tag.dolby_vision"}},
			"ptcafe.club":          {"杜比视界": {"tag.dolby_vision", "tag.dolby_atmos"}, "HDR": {"tag.hdr10"}},
			"1ptba.com":            {"Dolby Vision": {"tag.dolby_vision", "tag.dolby_atmos"}, "HDR10": {"tag.hdr10"}, "HDR10+": {"tag.hdr10_plus"}},
			"cyanbug.net":          {"HDR 10": {"tag.hdr10"}, "Dolby Vision": {"tag.dolby_vision"}},
			"kufei.org":            {"HDR": {"tag.hdr10"}},
			"pt.muxuege.org":       {"杜比全景声": {"tag.dolby_atmos"}, "杜比视频": {"tag.dolby_vision"}},
			"xingwan.cc":           {"HDR10bit": {"tag.hdr10"}},
			"www.agsvpt.com":       {"英字": {"tag.english_subtitle"}, "HDR": {"tag.hdr10"}, "杜比": {"tag.dolby_vision", "tag.dolby_atmos"}},
			"www.hitpt.com":        {"HDR": {"tag.hdr10"}, "杜比": {"tag.dolby_vision", "tag.dolby_atmos"}},
			"pt.soulvoice.club":    {"HDR": {"tag.hdr10"}, "HLG": {"tag.hlg"}, "杜比": {"tag.dolby_vision", "tag.dolby_atmos"}},
			"crabpt.vip":           {"HDR": {"tag.hdr10"}, "Dolby Vision": {"tag.dolby_vision"}, "Dolby Atmos": {"tag.dolby_atmos"}},
			"carpt.net":            {"HDR": {"tag.hdr10"}},
			"www.qingwapt.com":     {"杜比视界": {"tag.dolby_vision", "tag.dolby_atmos"}, "HDR10+": {"tag.hdr10_plus"}, "HDR": {"tag.hdr10"}},
			"www.yemapt.org":       {"杜比全景声(Atmos)": {"tag.dolby_atmos"}},
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
