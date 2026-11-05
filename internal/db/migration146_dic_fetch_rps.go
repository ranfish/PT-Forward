package db

import (
	"gorm.io/gorm"
)

// migration146 §59.466 附: 海豚（DIC Music）获取侧限流配置。
//
// 海豚=Gazelle 纯音乐站（获取链第二数据源·音乐字段抽取路）：
//   - ajax.php cookie 通道（api.php 端点不存在——2026-11-05 实证返回
//     HTML 页；设置页无 API Key 入口·用户观察互证）
//   - fetch_rps=0.4（24/min）：与海豹（Gazelle 同框架）对齐——黄金规则
//     10s/5req=30/min 严格默认桶卡线，收紧留 20% 余量（§59.462 先例）
//
// 仅配获取侧限流：is_target 不动（发布适配未开始——api.php 不可用=
// 海豚发布未来走 HTML 表单通道，与海豹 API 上传通道不同路）。
//
// 幂等：条件写（fetch_rps 未配才 UPDATE）。
func init() {
	RegisterMigration(146, "dic_fetch_rps", func(gormDB *gorm.DB) error {
		return gormDB.Exec(`UPDATE sites SET
			fetch_rps = 0.4,
			updated_at = datetime('now')
			WHERE domain = 'dicmusic.com'
			AND (fetch_rps IS NULL OR fetch_rps = 0)`).Error
	})
}
