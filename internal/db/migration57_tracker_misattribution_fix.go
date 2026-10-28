package db

import "gorm.io/gorm"

// §59.365 附四：误录期错标存量刷正——2026-07-22 至 2026-10-28 tracker
// 误录窗口（附二修正映射）期间，正主站 tracker 的种子被错判给误录挂载站。
// 映射修正（v0.0.1204）不回溯存量——seeding_records 错标行按 tracker
// 权威归属刷正（29 实证 2 组 167 行：hdupt→好大 144/greatposterwall→
// 轨道炮 23）。hhanclub 判憨憨一直正确（重复挂载无实害）不涉及。
func init() {
	RegisterMigration(57, "tracker_misattribution_fix", func(gormDB *gorm.DB) error {
		if err := gormDB.Exec(`UPDATE seeding_torrent_records SET site_name = '好多油'
			WHERE site_name = '好大'
			AND info_hash IN (SELECT hash FROM torrent_snapshots WHERE tracker_domain = 'pt.hdupt.com')`).Error; err != nil {
			return err
		}
		return gormDB.Exec(`UPDATE seeding_torrent_records SET site_name = '海豹'
			WHERE site_name = '轨道炮'
			AND info_hash IN (SELECT hash FROM torrent_snapshots WHERE tracker_domain = 'tracker.greatposterwall.com')`).Error
	})
}
