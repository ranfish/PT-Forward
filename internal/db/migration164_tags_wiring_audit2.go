package db

import (
	"encoding/json"

	"gorm.io/gorm"
)

// migration164 §59.525: tags 漏接线审计第二刀——用户指令升级审计主轴：
// 「推理是否有产出 → 站点是否需要 → 需要的话是否接线」三步流程全量检查
// （第一刀 §59.524 只扫 HDR/杜比关键词族·本刀=57 键全集语义匹配）。
//
// 新发现 34 条甄别（判定三问：语义匹配+审核消费+应自动投）：
// - 补 29 站词条：特效字幕（11 站）/Atmos（4）/4K·8K（5）/短剧（5）/
//   原盘·Remux·3D（5）/高码·高分（5）/大包·超分·补帧·外挂字幕（7）/
//   10Bit·连载·日语·韩语（Nova高清/OKPT）/双语字幕（3·准确键）
// - 不接①：分集 5 条（朱雀/末日/蟹黄堡/织梦/财神）——§59.424 系统级
//   禁发分集·词条对发布面永不触发·与禁转零键同理=正确设计
// - 不接②：天空复合 promo 7 条维持 §59.524 挂账
// - 键设计：「双语字幕」=字幕双语≠双语音轨（bilingual_audio 是音轨键）——
//   接 [chinese_subtitle, english_subtitle] 组合键（简中+英字同产命中）；
//   咖啡「高码高帧」复合→[high_bitrate, high_frame_rate]；优堡「高分国剧」
//   →high_rating（国剧无键·值域复合）；冬樱值集重复词条按 label 全命中
//
// 幂等：词条已含目标键跳过。全量 map 拉出改回（保全字段）。
func init() {
	RegisterMigration(164, "tags_wiring_audit_2", func(gormDB *gorm.DB) error {
		patches := map[string]map[string][]string{
			"13city.org":          {"4K": {"tag.resolution_4k"}},
			"hdvideo.top":         {"微短剧": {"tag.short_play"}},
			"pt.novahd.top":       {"10Bit": {"tag.10_bit"}, "连载": {"tag.ongoing"}},
			"www.okpt.net":        {"日语": {"tag.japanese_audio"}, "韩语": {"tag.korean_audio"}, "Atmos": {"tag.dolby_atmos"}},
			"duckboobee.org":      {"Ai双语字幕": {"tag.chinese_subtitle", "tag.english_subtitle"}},
			"52pt.site": {"原盘": {"tag.original_disc"}, "Remux": {"tag.remux"}, "繁中": {"tag.chinese_subtitle"},
				"简繁": {"tag.chinese_subtitle"}, "双语字幕": {"tag.chinese_subtitle", "tag.english_subtitle"}, "3D": {"tag.3d"}},
			"ubits.club":          {"高分国剧": {"tag.high_rating"}, "特效字幕": {"tag.special_effects_subs"}},
			"pt.0ff.cc":           {"短剧": {"tag.short_play"}},
			"wintersakura.net":    {"特效字幕": {"tag.special_effects_subs"}, "原生原盘": {"tag.original_disc"}},
			"ptlgs.org":           {"3D": {"tag.3d"}},
			"ptcafe.club":         {"高码高帧": {"tag.high_bitrate", "tag.high_frame_rate"}},
			"pt.lajidui.top":      {"简英双语字幕": {"tag.chinese_subtitle", "tag.english_subtitle"}},
			"1ptba.com":           {"特效": {"tag.special_effects_subs"}},
			"hdhome.org":          {"特效": {"tag.special_effects_subs"}},
			"pt.muxuege.org":      {"高码率": {"tag.high_bitrate"}, "4k": {"tag.resolution_4k"}, "特效字慕": {"tag.special_effects_subs"}, "大包": {"tag.big_pack"}},
			"xingwan.cc":          {"短剧": {"tag.short_play"}, "4K": {"tag.resolution_4k"}},
			"pt.xingyungept.org":  {"补帧": {"tag.frame_interp"}, "特效": {"tag.special_effects_subs"}},
			"hdfans.org":          {"特效": {"tag.special_effects_subs"}, "Atmos": {"tag.dolby_atmos"}, "4K": {"tag.resolution_4k"}, "8K": {"tag.resolution_8k"}},
			"lemonhd.net":         {"4K": {"tag.resolution_4k"}},
			"mua.xloli.cc":        {"Atmos": {"tag.dolby_atmos"}},
			"cangbao.ge":          {"高码": {"tag.high_bitrate"}, "特效": {"tag.special_effects_subs"}},
			"www.tangpt.top":      {"特效": {"tag.special_effects_subs"}},
			"www.qingwapt.com":    {"特效字幕": {"tag.special_effects_subs"}, "原生原盘": {"tag.original_disc"}, "Remux": {"tag.remux"}},
			"longpt.org":          {"高分": {"tag.high_rating"}},
			"cspt.top": {"外挂字幕": {"tag.external_subtitles"}, "Remux": {"tag.remux"}, "大包": {"tag.big_pack"},
				"超分": {"tag.upscale"}, "补帧": {"tag.frame_interp"}, "特效": {"tag.special_effects_subs"}},
			"www.agsvpt.com":      {"大包": {"tag.big_pack"}, "特效": {"tag.special_effects_subs"}, "超分": {"tag.upscale"}, "补帧": {"tag.frame_interp"}},
			"zmpt.cc":             {"短剧": {"tag.short_play"}},
			"crabpt.vip":          {"短剧": {"tag.short_play"}},
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
