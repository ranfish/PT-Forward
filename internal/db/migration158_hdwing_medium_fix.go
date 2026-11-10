package db

import "gorm.io/gorm"

// migration158 §59.511 附三: 瓷器 medium 值集 Blu-ray 槽补 uhd_bluray 双键
// ——DryRun 矩阵 ④ 案（Alice in Wonderland UHD BluRay 2160p 真原盘）：
// SourceType 归一 uhd_bluray → 值集只有 medium.bluray 单键 miss → medium
// 槽缺失。对照 Remux 槽双键先例（remux+uhd_remux）。157 已跑过的存量
// 环境由本 migration 重写 form_config（幂等 UPDATE 同 157 形态）。
func init() {
	RegisterMigration(158, "hdwing_medium_uhd_key", func(gormDB *gorm.DB) error {
		return gormDB.Exec(`UPDATE sites SET publish_form_config = json_set(publish_form_config,
			'$.value_mappings.medium[0].standard_keys', json('["medium.bluray","medium.uhd_bluray"]')
		), updated_at = datetime('now') WHERE domain = 'hdwing.org'
			AND json_extract(publish_form_config, '$.value_mappings.medium[0].label') = 'Blu-ray'`).Error
	})
}
