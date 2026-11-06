package db

import "gorm.io/gorm"

// migration152 §59.477: 皇后 fetch_rps 0.0→0.4——批量音乐搜索无节流
// （~20+ 搜索/7min）触发站方按 IP 防滥用封锁（12 环境实证：成功/失败
// 交错·22:53 恢复），与海豚对齐（migration146 同款）2.5s/请求。
// 幂等：仅 0.0 时写（用户已自定义值不覆盖）。
func init() {
	RegisterMigration(152, "queen_fetch_rps", func(gormDB *gorm.DB) error {
		return gormDB.Exec(`UPDATE sites SET fetch_rps = 0.4
			WHERE name = '皇后' AND fetch_rps = 0.0`).Error
	})
}
