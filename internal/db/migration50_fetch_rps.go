package db

import (
	"gorm.io/gorm"
)

// §59.340 C: 站点级获取 RPS——keepfrds 流控探究（§59.340 五阶段实验）定案：
// 安全线 2/min、惩罚带 4/min、封锁线 8/min（脉冲等效翻倍）。默认 0=全局
// 默认（0.5 RPS）；站点特调（如 keepfrds 0.0667=4/min）走本列。
// 幂等：model AutoMigrate 已建列则跳过。
func init() {
	RegisterMigration(50, "fetch_rps", func(gormDB *gorm.DB) error {
		var cnt int64
		gormDB.Raw("SELECT COUNT(*) FROM pragma_table_info('sites') WHERE name = 'fetch_rps'").Scan(&cnt)
		if cnt > 0 {
			return nil
		}
		return gormDB.Exec("ALTER TABLE sites ADD COLUMN fetch_rps REAL DEFAULT 0").Error
	})
}
