package db

import "gorm.io/gorm"

// migration159 好大（hdarea.club）API 网关切换（2026-11-12 刀1）。
// 站方 2026.11.11 上线统一 API 网关（公告明示第三方工具切换 API Key 模式）。
// 用户定案：全量 API adapter（framework=hdarea·独立方言层）·API-only 零 web
// 双路（回退阀=framework 字段应急回 nexusphp·updateSiteRequest 可编辑）。
// 依据：docs/32/hdarea-api.md（2026-11-12 活体探针 11 项实测契约）。
func init() {
	RegisterMigration(159, "hdarea_api_adapter", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'hdarea.club'").Scan(&count)
		if count == 0 {
			return nil
		}
		// framework/auth_type/fetch_rps（read 60/min 上限→0.8 卡线收紧）
		if err := gormDB.Exec(`UPDATE sites SET framework = 'hdarea', auth_type = 'apikey',
			fetch_rps = 0.8, updated_at = datetime('now') WHERE domain = 'hdarea.club'`).Error; err != nil {
			return err
		}
		// publish_form_config.framework np→hdarea（跟随 mteam 惯例——配置
		// 生成形态与运行时分派一致性；配置中心支持非 np 值实证）
		if err := gormDB.Exec(`UPDATE sites SET publish_form_config = json_set(publish_form_config, '$.framework', 'hdarea')
			WHERE domain = 'hdarea.club' AND publish_form_config IS NOT NULL AND publish_form_config != ''`).Error; err != nil {
			return err
		}
		return nil
	})
}
