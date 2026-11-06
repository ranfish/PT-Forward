package db

import (
	"encoding/json"

	"gorm.io/gorm"
)

// migration149 §59.471: HDVideo 地区/风格必填两域补配（适配遗漏修复）。
//
// 案情：用户 2026-11-05 发布 HDVideo 报"请选择地区"——站方 upload 表单
// region_sel[4]（地区·必填单选）与 style_sel[4][]（风格·必填多选）两字段
// 在 62 站适配时漏配（[4] 后缀=依附 type 401 电影分支的条件字段——当时
// 表单 diff 未触发分支盲区·docs/32/hdvideo.md "无地区选择"为漏记非站方
// 改版——用户上传发布页 HTML diff 证实站方未变）。
//
// 值集（29 cookie 实测站方表单 2026-11-06）：
//   - region 17 有效项：中国大陆 1/中国港台 6（hk+tw 双键承接·朱雀先例）
//     /美英日韩泰印法德意西加澳俄各 ISO 键；北欧 27、其他地区 28 label-only
//     （兜底目标=28·§59.471 用户定案 c）
//   - style 19 项：喜剧/动作/科幻/惊悚/剧情/爱情/恐怖/犯罪/悬疑/冒险/音乐
//     各 genre 键+短片 genre.short/真人秀 genre.reality/脱口秀 genre.talkshow；
//     竞技/访谈/旅行/美食 label-only（兜底目标=6 剧情·§59.471 用户定案 a）
//
// 消费链（executor §59.471）：regionMappingOf/styleMappingOf——PTGen 产区/
// 类型词 canonical 化（extract.LookupStandardKey）后 standard_keys 匹配；
// miss 站方值兜底（碟粉 410 同族先例）。
//
// 幂等：region 域已存在（len>0）跳过。
func init() {
	RegisterMigration(149, "hdvideo_region_style", func(gormDB *gorm.DB) error {
		var raw string
		if err := gormDB.Raw(`SELECT publish_form_config FROM sites WHERE name = 'HDVideo'`).Scan(&raw).Error; err != nil {
			return err
		}
		if raw == "" {
			return nil // 站点不存在/未配置——跳过
		}
		var cfg map[string]interface{}
		if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
			return err
		}
		// 幂等门
		if vm, ok := cfg["value_mappings"].(map[string]interface{}); ok {
			if _, exists := vm["region"]; exists {
				return nil
			}
		}
		// form_fields 补两键
		if ff, ok := cfg["form_fields"].(map[string]interface{}); ok {
			ff["region"] = "region_sel[4]"
			ff["style"] = "style_sel[4][]"
		}
		// value_mappings 补两域
		if vm, ok := cfg["value_mappings"].(map[string]interface{}); ok {
			vm["region"] = []map[string]interface{}{
				{"label": "中国大陆", "value": "1", "standard_keys": []string{"region.cn"}},
				{"label": "中国港台", "value": "6", "standard_keys": []string{"region.hk", "region.tw"}},
				{"label": "美国", "value": "2", "standard_keys": []string{"region.us"}},
				{"label": "英国", "value": "4", "standard_keys": []string{"region.gb"}},
				{"label": "日本", "value": "7", "standard_keys": []string{"region.jp"}},
				{"label": "韩国", "value": "3", "standard_keys": []string{"region.kr"}},
				{"label": "泰国", "value": "5", "standard_keys": []string{"region.th"}},
				{"label": "印度", "value": "11", "standard_keys": []string{"region.in"}},
				{"label": "法国", "value": "8", "standard_keys": []string{"region.fr"}},
				{"label": "德国", "value": "9", "standard_keys": []string{"region.de"}},
				{"label": "意大利", "value": "10", "standard_keys": []string{"region.it"}},
				{"label": "西班牙", "value": "23", "standard_keys": []string{"region.es"}},
				{"label": "加拿大", "value": "24", "standard_keys": []string{"region.ca"}},
				{"label": "澳大利亚", "value": "25", "standard_keys": []string{"region.au"}},
				{"label": "俄罗斯", "value": "26", "standard_keys": []string{"region.ru"}},
				{"label": "北欧", "value": "27"},
				{"label": "其他地区", "value": "28"},
			}
			vm["style"] = []map[string]interface{}{
				{"label": "短片", "value": "1", "standard_keys": []string{"genre.short"}},
				{"label": "喜剧", "value": "2", "standard_keys": []string{"genre.comedy"}},
				{"label": "动作", "value": "3", "standard_keys": []string{"genre.action"}},
				{"label": "科幻", "value": "4", "standard_keys": []string{"genre.scifi"}},
				{"label": "惊悚", "value": "5", "standard_keys": []string{"genre.thriller"}},
				{"label": "剧情", "value": "6", "standard_keys": []string{"genre.drama"}},
				{"label": "爱情", "value": "7", "standard_keys": []string{"genre.romance"}},
				{"label": "恐怖", "value": "8", "standard_keys": []string{"genre.horror"}},
				{"label": "犯罪", "value": "9", "standard_keys": []string{"genre.crime"}},
				{"label": "悬疑", "value": "10", "standard_keys": []string{"genre.suspense"}},
				{"label": "冒险", "value": "11", "standard_keys": []string{"genre.adventure"}},
				{"label": "音乐", "value": "23", "standard_keys": []string{"genre.music"}},
				{"label": "真人秀", "value": "24", "standard_keys": []string{"genre.reality"}},
				{"label": "脱口秀", "value": "25", "standard_keys": []string{"genre.talkshow"}},
				{"label": "竞技", "value": "26"},
				{"label": "访谈", "value": "27"},
				{"label": "旅行", "value": "28"},
				{"label": "美食", "value": "29"},
			}
		}
		out, err := json.Marshal(cfg)
		if err != nil {
			return err
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, updated_at = datetime('now')
			WHERE name = 'HDVideo'`, string(out)).Error
	})
}
