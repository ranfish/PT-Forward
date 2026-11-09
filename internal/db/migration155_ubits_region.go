package db

import (
	"encoding/json"

	"gorm.io/gorm"
)

// migration155 §59.494: 优堡 source_sel[4]（地区·质量区必填组之一）补配。
//
// 背景：优堡上传表单 JS 动态渲染（静态 HTML 零字段）——Playwright 渲染
// 摸底（§59.494）实测 6 个 [质量] select：medium/codec/audiocodec/
// standard/source/team——系统缺 source_sel[4] → 站方拒"一个或多个
// 质量区域未选择"（243 The Continent 案）。
//
// 语义反转：优堡 source_sel = 地区（Region）非媒介来源——复用 §59.471
// 通用 region 机制（executor regionMappingOf 注入·PTGen 产区词
// canonical 化）。值集=Playwright 实测 11 项：
//   1 中国大陆 / 2 港 / 3 台 / 4 欧美（欧美系多键归并）/ 5 日 / 6 韩 /
//   7 泰 / 8 印 / 9 俄 / 11 其它
// 幂等：region 域已存在跳过。站点不存在/未配置跳过。
func init() {
	RegisterMigration(155, "ubits_region", func(gormDB *gorm.DB) error {
	var raw string
	if err := gormDB.Raw(`SELECT publish_form_config FROM sites WHERE name = '优堡'`).Scan(&raw).Error; err != nil || raw == "" {
		return err
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return err
	}
	if vm, ok := cfg["value_mappings"].(map[string]interface{}); ok {
		if _, exists := vm["region"]; exists {
			return nil
		}
	}
	if ff, ok := cfg["form_fields"].(map[string]interface{}); ok {
		ff["region"] = "source_sel[4]"
	}
	if vm, ok := cfg["value_mappings"].(map[string]interface{}); ok {
		vm["region"] = []map[string]interface{}{
			{"label": "中国大陆", "value": "1", "standard_keys": []string{"region.cn"}},
			{"label": "中国香港", "value": "2", "standard_keys": []string{"region.hk"}},
			{"label": "中国台湾", "value": "3", "standard_keys": []string{"region.tw"}},
			// 欧美大桶：美/英/加/澳/法/德/意/西等欧美系多键归并（站方
			// 值集仅一个"欧美"——PTGen 小国产区词全部收敛此桶）
			{"label": "欧美", "value": "4", "standard_keys": []string{
				"region.us", "region.gb", "region.ca", "region.au",
				"region.fr", "region.de", "region.it", "region.es",
				"region.ru_eu", "region.nz", "region.se", "region.nl",
			}},
			{"label": "日本", "value": "5", "standard_keys": []string{"region.jp"}},
			{"label": "韩国", "value": "6", "standard_keys": []string{"region.kr"}},
			{"label": "泰国", "value": "7", "standard_keys": []string{"region.th"}},
			{"label": "印度", "value": "8", "standard_keys": []string{"region.in"}},
			{"label": "俄罗斯", "value": "9", "standard_keys": []string{"region.ru"}},
			{"label": "其它", "value": "11", "standard_keys": []string{}},
		}
	}
	blob, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return gormDB.Exec(`UPDATE sites SET publish_form_config = ? WHERE name = '优堡'`, string(blob)).Error
})
}
