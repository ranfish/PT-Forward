package db

import "gorm.io/gorm"

// migration153 §59.485 刀2: 乌托邦音乐发布开闸+系统级默认匿名。
//
// ① 乌托邦 publish_form_config（音乐目标站第一站——值集词表在
// internal/publish/utopia_music.go 直配·配置层只配键名域+开关）；
// is_target 同步开（目标站门槛 §59.318）。
// ② 全站 form_config.Anonymous 刷 true（§59.485 用户定案：音乐/影视
// 系统级统一默认匿名发布——§59.159 机制已在（executor 单点消费
// cfg.Anonymous→uplver=yes）·仅缺统一默认；保留站点级开关能力·
// 某站想署名单独关）。幂等：json 里 anonymous 已 true 的行不重复写。
func init() {
	RegisterMigration(153, "utopia_music_publish_anonymous", func(gormDB *gorm.DB) error {
		// ① 乌托邦发布配置（不存在则插入·已存在不覆盖用户调整）
		utopiaCfg := `{"enabled":true,"framework":"np","anonymous":true,"form_fields":{` +
			`"type":"type","medium":"medium_sel[4]","audiocodec":"audiocodec_sel[4]",` +
			`"team":"team_sel[4]","small_descr":"small_descr"}}`
		if err := gormDB.Exec(`UPDATE sites SET
			publish_form_config = ?,
			is_target = 1
			WHERE name = '乌托邦' AND (publish_form_config IS NULL OR publish_form_config = '')`,
			utopiaCfg).Error; err != nil {
			return err
		}
		// ② 全站默认匿名（仅 anonymous 非 true 的行刷——保留 enabled 等既有配置）
		return gormDB.Exec(`UPDATE sites SET publish_form_config = json_set(
				CASE WHEN publish_form_config IS NULL OR publish_form_config = ''
					THEN '{}' ELSE publish_form_config END,
				'$.anonymous', json('true'))
			WHERE is_target = 1
			AND publish_form_config IS NOT NULL
			AND publish_form_config != ''
			AND json_extract(publish_form_config, '$.anonymous') IS NOT json('true')`).Error
	})
}
