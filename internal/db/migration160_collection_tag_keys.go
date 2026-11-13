package db

import (
	"encoding/json"

	"gorm.io/gorm"
)

// migration160 §59.518: 青蛙「系列合集」+ 蟹黄堡「合集大包」补
// standard_keys=["tag.collection"]——两站多作品合集语义标签此前零键
// （推断层 collection 命中也投不出·applier 查 miss 静默丢）。
//
// 用户定案语义（2026-11-13）：
//   青蛙「系列合集」  = 系列电影合集（超人系列/007系列）
//   蟹黄堡「合集大包」= 多部电影合集大包（站内 tid 103201/24241/52856 实证）
//   ——均为多作品合集语义，与推断层 tag.collection（幸运语义=多部不同
//   作品的集合·副标「合集/集全/期全」词形命中）精确对齐。
//
// 蟹黄堡「未完结」（单集）不补：ongoing=连载中 语义不匹配，且单集场景
// 被 §59.424 系统级禁发挡死——补了是死配置。
//
// 幂等：条目已含 tag.collection 键则跳过。数组条目改写走全量 map
// （保留 form_config 全部字段——窄 struct 写回会丢 form_fields/enabled
// 等·§59.318 稀疏覆盖同族防线）。seed 零同步（data/sites.json 无
// form_config——publish 配置是 migration 域）。
func init() {
	RegisterMigration(160, "collection_tag_keys", func(gormDB *gorm.DB) error {
		targets := map[string]string{
			"www.qingwapt.com": "系列合集",
			"crabpt.vip":       "合集大包",
		}
		for domain, label := range targets {
			var cfgStr string
			if err := gormDB.Raw(
				`SELECT publish_form_config FROM sites WHERE domain = ? AND publish_form_config IS NOT NULL`,
				domain).Scan(&cfgStr).Error; err != nil {
				return err
			}
			if cfgStr == "" {
				continue // 站点不存在或无配置——幂等跳过
			}
			cfgRaw := []byte(cfgStr)
			// 全量 map 解析——保住 form_config 所有字段
			var cfg map[string]interface{}
			if err := json.Unmarshal(cfgRaw, &cfg); err != nil {
				continue // 配置形态异常——不动
			}
			vm, ok := cfg["value_mappings"].(map[string]interface{})
			if !ok {
				continue
			}
			tagsRaw, ok := vm["tags"].([]interface{})
			if !ok {
				continue
			}
			changed := false
			for _, item := range tagsRaw {
				m, ok := item.(map[string]interface{})
				if !ok || m["label"] != label {
					continue
				}
				keys, _ := m["standard_keys"].([]interface{})
				hasKey := false
				for _, k := range keys {
					if k == "tag.collection" {
						hasKey = true
						break
					}
				}
				if !hasKey {
					m["standard_keys"] = append(keys, "tag.collection")
					changed = true
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
