package db

import (
	"encoding/json"

	"gorm.io/gorm"
)

// migration150 §59.472: 海胆季/集/合集三键补配（From S04 案）。
//
// 案情：S04 纯季包（§59.424 合集形态放行）发布海胆失败——站方要求
// 剧集类（电视剧/综艺/动画/纪录片）必填季集信息（season 第几季·
// episode 第几集·collages 合集 checkbox·站方语义：季 0=不区分季·
// 集 0=全季·多季勾合集——29 cookie 实测表单）。
//
// 消费链（executor §59.472 haidanSeasonEpisodeInject）：tp.SeasonEpisode
// 归一解析注入（S04→4/0·S01-S02→0/0+勾合集·S04E01→4/1·无信号剧集→
// 0/0·电影类不填）。幂等：season 键已存在跳过。
func init() {
	RegisterMigration(150, "haidan_season_episode", func(gormDB *gorm.DB) error {
		var raw string
		if err := gormDB.Raw(`SELECT publish_form_config FROM sites WHERE name = '海胆'`).Scan(&raw).Error; err != nil {
			return err
		}
		if raw == "" {
			return nil
		}
		var cfg map[string]interface{}
		if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
			return err
		}
		ff, ok := cfg["form_fields"].(map[string]interface{})
		if !ok {
			return nil
		}
		if _, exists := ff["season"]; exists {
			return nil // 幂等
		}
		ff["season"] = "season"
		ff["episode"] = "episode"
		ff["collages"] = "collages"
		out, err := json.Marshal(cfg)
		if err != nil {
			return err
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, updated_at = datetime('now')
			WHERE name = '海胆'`, string(out)).Error
	})
}
