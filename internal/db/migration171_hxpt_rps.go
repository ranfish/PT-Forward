package db

import "gorm.io/gorm"

// migration171 §59.533 附三: 好学 fetch_rps 收紧 0.4（黄金规则·海豹/可乐
// 先例）。实证：好学有 IP 短窗速率防御——29 连打 13+ 发后请求 hang（连
// headers 不回）·约 1 分钟恢复；243 全站 stats 同步+发布叠加曾触发 403。
func init() {
	RegisterMigration(171, "hxpt_fetch_rps", func(gormDB *gorm.DB) error {
		return gormDB.Exec(`UPDATE sites SET fetch_rps = 0.4, updated_at = datetime('now')
			WHERE domain = 'www.hxpt.org'`).Error
	})
}
