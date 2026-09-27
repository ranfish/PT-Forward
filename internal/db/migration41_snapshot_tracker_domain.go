package db

import (
	"gorm.io/gorm"
)

// §59.296: torrent_snapshots 加 tracker_domain 列——簇副本归属判定
// （馒头纯数字 comment 方言的站点上下文）。syncer 下轮同步自然回填。
func init() {
	RegisterMigration(41, "snapshot_tracker_domain", func(gormDB *gorm.DB) error {
		var hasCol int64
		gormDB.Raw("SELECT COUNT(*) FROM pragma_table_info('torrent_snapshots') WHERE name='tracker_domain'").Scan(&hasCol)
		if hasCol == 0 {
			return gormDB.Exec("ALTER TABLE torrent_snapshots ADD COLUMN tracker_domain text NOT NULL DEFAULT ''").Error
		}
		return nil
	})
}
