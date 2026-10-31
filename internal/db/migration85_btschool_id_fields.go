package db

import (
	"encoding/json"

	"gorm.io/gorm"
)

// migration85 §59.399: 学校站（pt.btschool.club）form_fields 补
// IMDb/豆瓣编号形态双键（imdb_id→imdbid / douban_id→doubanid）。
//
// §59.398 挂账兑现（用户定案方案 A）：学校站 imdbid/doubanid 仅收
// 编号（tt4154756 / 24773958——站方特别要求），发布链从 meta 完整
// URL 提取尾部编号（executor §59.399 extractIMDbID/extractDoubanID
// 同版本实施）；与 imdb_url/dburl 完整链接域并存——域语义=站方要
// 什么形态配什么域（南洋 dburl 完整链接 vs 学校 douban_id 编号）。
//
// 实现：读 publish_form_config → JSON 解析 → 加双键（已存在跳过）
// → 写回。幂等：键已在零写入。
func init() {
	RegisterMigration(85, "btschool_imdb_douban_id_fields", func(gormDB *gorm.DB) error {
		var raw string
		if err := gormDB.Raw("SELECT publish_form_config FROM sites WHERE domain = 'pt.btschool.club'").Scan(&raw).Error; err != nil {
			return err
		}
		if raw == "" {
			return nil
		}
		var cfg map[string]any
		if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
			return nil // 非 JSON 形态不动（防御——理论不可达）
		}
		ff, _ := cfg["form_fields"].(map[string]any)
		if ff == nil {
			return nil
		}
		if v, ok := ff["imdb_id"].(string); ok && v == "imdbid" {
			if v2, ok2 := ff["douban_id"].(string); ok2 && v2 == "doubanid" {
				return nil // 已刷正（幂等）
			}
		}
		ff["imdb_id"] = "imdbid"
		ff["douban_id"] = "doubanid"
		cfg["form_fields"] = ff
		out, err := json.Marshal(cfg)
		if err != nil {
			return err
		}
		return gormDB.Exec("UPDATE sites SET publish_form_config = ?, updated_at = datetime('now') WHERE domain = 'pt.btschool.club'", string(out)).Error
	})
}
