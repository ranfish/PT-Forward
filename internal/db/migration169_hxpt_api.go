package db

import "gorm.io/gorm"

// migration169 §59.533 刀2: 好学（www.hxpt.org）获取链切 API adapter。
// 站方 2026-11 改版彻底离开 NexusPHP（Next.js + /api/v1 REST）——旧 NP
// 获取链（搜索/详情/下载）已全部失效。本 migration 只切获取侧：
//   - framework nexusphp→hxpt（HxptAdapter：API 面 cookie 同域直用）
//   - publish_form_config.framework 同步刷 hxpt
//   - publish_form_config.enabled→false（**临时**——旧值集全部错位
//     （年级/教材/年份 NP 槽语义已废）+发布链 adapter 刀3 未实施；
//     刀4 重造值集后 re-enable。防误发铁律：配置错位时宁关勿错）
func init() {
	RegisterMigration(169, "hxpt_api_adapter", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'www.hxpt.org'").Scan(&count)
		if count == 0 {
			return nil
		}
		if err := gormDB.Exec(`UPDATE sites SET framework = 'hxpt', updated_at = datetime('now')
			WHERE domain = 'www.hxpt.org'`).Error; err != nil {
			return err
		}
		if err := gormDB.Exec(`UPDATE sites SET publish_form_config = json_set(
				json_set(publish_form_config, '$.framework', 'hxpt'), '$.enabled', json('false'))
			WHERE domain = 'www.hxpt.org' AND publish_form_config IS NOT NULL AND publish_form_config != ''`).Error; err != nil {
			return err
		}
		return nil
	})
}
