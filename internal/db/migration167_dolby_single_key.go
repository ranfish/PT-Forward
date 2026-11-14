package db

import (
	"encoding/json"

	"gorm.io/gorm"
)

// migration167 §59.529: 财神 Ip Man 双 11 案键层修正——键设计双向核对
// （§59.528 架构沉淀）落地：非互斥键集不归并同词条 + 组合语义词条不接键。
//
// ① 杜比双键族 7 站 → 单键 dolby_vision：
//    壹吧/咖啡/百川/财神/末日(agsvpt)/青蛙/聆音的「杜比/Dolby Vision/杜比视界」
//    词条原接 [dolby_vision, dolby_atmos]（§59.523 错误先例→§59.524 审计扩散）。
//    根因：两键跨维度不互斥（DoVi=视频·Atmos=音频·4K 原盘标配双产出）→
//    双双命中同词条→同值双投→站方表单重复值整栏作废（财神 22 封拒信 B/C
//    组 8 颗实证：标签全丢+Bot 拒）。站方语义「杜比」只指 DoVi（分层定律：
//    标签层杜比=视频增强·Atmos 走质量层 audiocodec）。
//    三月「Dolby Vision」已是单键（无 atm 键）·垃圾堆/五二同——不涉及。
// ② 字幕双语族 3 站 → 删键（label-only 化）：
//    垃圾堆「简英双语字幕」/三月「Ai双语字幕」/五二「双语字幕」原接
//    [chinese_subtitle, english_subtitle]（§59.525 组合键设计）。根因：
//    词条语义=两键 AND（中英字幕都在）而 applier 是 OR——单字幕种错勾+
//    双字幕种双投双重错误（用户定案删键：系统词汇表无「双语」组合词·
//    「没有→不投递」留给人工）。
//
// 配套（非本 migration）：executor TagArrayFields 同域同值去重（投递层
// 兜底）；dict/tag.json dolby_atmos infer_scope all→mi（音频技术只认 MI
// 铁证·两层一致性·§59.520 挂账闭环）。
//
// 幂等：键态已达成则跳过（setKeys 覆写目标态·delKeys 清空）。
func init() {
	RegisterMigration(167, "dolby_single_key_and_bilingual_del", func(gormDB *gorm.DB) error {
		// ① 覆写组：domain → (label → 目标键集)
		setPatches := map[string]map[string][]string{
			"1ptba.com":         {"Dolby Vision": {"tag.dolby_vision"}},
			"ptcafe.club":       {"杜比视界": {"tag.dolby_vision"}},
			"www.hitpt.com":     {"杜比": {"tag.dolby_vision"}},
			"cspt.top":          {"杜比": {"tag.dolby_vision"}},
			"www.agsvpt.com":    {"杜比": {"tag.dolby_vision"}},
			"www.qingwapt.com":  {"杜比视界": {"tag.dolby_vision"}},
			"pt.soulvoice.club": {"杜比": {"tag.dolby_vision"}},
		}
		// ② 删键组：domain → labels（standard_keys 置空）
		delPatches := map[string][]string{
			"pt.lajidui.top": {"简英双语字幕"},
			"duckboobee.org": {"Ai双语字幕"},
			"52pt.site":      {"双语字幕"},
		}
		for domain, labelKeys := range setPatches {
			if err := rewriteTagKeys(gormDB, domain, labelKeys, false); err != nil {
				return err
			}
		}
		for domain, labels := range delPatches {
			empty := map[string][]string{}
			for _, l := range labels {
				empty[l] = nil
			}
			if err := rewriteTagKeys(gormDB, domain, empty, true); err != nil {
				return err
			}
		}
		return nil
	})
}

// rewriteTagKeys 覆写/清空指定站 tags 值集词条的 standard_keys。
// allowEmpty=true 时目标键集为 nil 表示清空（label-only 化）。
func rewriteTagKeys(gormDB *gorm.DB, domain string, labelKeys map[string][]string, allowEmpty bool) error {
	var cfgStr string
	if err := gormDB.Raw(
		`SELECT publish_form_config FROM sites WHERE domain = ? AND publish_form_config IS NOT NULL`,
		domain).Scan(&cfgStr).Error; err != nil {
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
		if want == nil && !allowEmpty {
			continue
		}
		// 覆写为精确目标态（单键/清空）
		var merged []interface{}
		for _, k := range want {
			merged = append(merged, k)
		}
		if merged == nil {
			delete(m, "standard_keys")
		} else {
			m["standard_keys"] = merged
		}
		changed = true
	}
	if !changed {
		return nil
	}
	out, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return gormDB.Exec(
		`UPDATE sites SET publish_form_config = ?, updated_at = datetime('now') WHERE domain = ?`,
		string(out), domain).Error
}
