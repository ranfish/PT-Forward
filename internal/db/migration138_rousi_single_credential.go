package db

import (
	"gorm.io/gorm"
)

// migration138 §59.457 附五: 肉丝凭据单源化存量刷正（用户定案
// 2026-11-03：拆掉 passkey 认证只保留 API Key）。
//
// ① auth_type passkey→apikey：seed 链对已存在行不刷 auth_type
//   （site_data.go 只 UPDATE tracker_domains/title_format/pieces_hash
//   三项）——存量环境必须 migration 刷正；assets 已同步改（新装走
//   seed 建行即 apikey）
// ② passkey 列清空：passkey=tracker 专用（PeerGo 源码权威
//   "only to construct one authenticated announce URL"），.torrent
//   下载时站方自动注入下载者 passkey（实测 announce=rousi.pro/
//   tracker/{passkey}/announce），PT-Forward 全链只走 API 通道——
//   DB 存量 passkey 为冗余留档，拆除定案后清空防困惑。
//
// 幂等：无条件 UPDATE（值即终态）。
func init() {
	RegisterMigration(138, "rousi_single_credential", func(gormDB *gorm.DB) error {
		return gormDB.Exec(`UPDATE sites SET auth_type = 'apikey', passkey = '', updated_at = datetime('now') WHERE domain = 'rousi.pro'`).Error
	})
}
