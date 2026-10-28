package db

import "gorm.io/gorm"

// §59.365: 不可杜域名迁移 www.hddolby.com → www.uhddolby.com（站方改域，
// 非镜像；认证/凭证/官组均未变）——三表 UPDATE 保留行数据（sites 整行
// 含凭证不动仅改 domain 键）。tracker 同步统一新域：快照旧 tracker 域
// t.hddolby.com 刷为 www.uhddolby.com（29 实测 1845 行；下载器侧 tracker
// 更新后同步链自然对齐）。tracker.azusa.wiki 为梓澪站误录（sites.json
// 已删）——其 370 行快照不动，维持 unknown 归属（azusa 不接入定案）。
func init() {
	RegisterMigration(56, "hddolby_domain_migration", func(gormDB *gorm.DB) error {
		if err := gormDB.Exec("UPDATE sites SET domain = 'www.uhddolby.com' WHERE domain = 'www.hddolby.com'").Error; err != nil {
			return err
		}
		if err := gormDB.Exec("UPDATE release_group_mappings SET domain = 'www.uhddolby.com' WHERE domain = 'www.hddolby.com'").Error; err != nil {
			return err
		}
		return gormDB.Exec("UPDATE torrent_snapshots SET tracker_domain = 'www.uhddolby.com' WHERE tracker_domain = 't.hddolby.com'").Error
	})
}
