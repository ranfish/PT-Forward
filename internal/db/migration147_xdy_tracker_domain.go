package db

import (
	"gorm.io/gorm"
)

// migration147 §59.468: 修道院 tracker_domains 补配（全站唯一缺配站）。
//
// 修道院是 migration33 建的骨架站（不在 site_data.go 常规 seed 列表）
// ——tracker_domains 从未配过。补值=["xdypt.vip"]（同域 announce，
// docs/32/xdy.md §站点信息权威：https://xdypt.vip/announce.php）。
//
// 背景：tracker 判归属基建（sites.tracker_domains·104 站 103 配——
// TrackerMatcher.MatchAll 消费）对修道院种失效（匹配 miss）。
//
// 幂等：条件写（空才 UPDATE）。
func init() {
	RegisterMigration(147, "xdy_tracker_domain", func(gormDB *gorm.DB) error {
		return gormDB.Exec(`UPDATE sites SET
			tracker_domains = '["xdypt.vip"]',
			updated_at = datetime('now')
			WHERE domain = 'xdypt.vip'
			AND (tracker_domains IS NULL OR tracker_domains = '')`).Error
	})
}
