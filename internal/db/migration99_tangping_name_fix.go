package db

import (
	"gorm.io/gorm"
)

// migration99 §59.415: 站名笔误刷正——"趟平"→"躺平"（www.tangpt.top）。
// 收录时站名笔误（tangpt=躺平拼音——站名应为"躺平"）。
// 修正三件套（§59.365 域名迁移先例变体——站名键非域名键）：
//   1. 资产源头：supported_sites.json name_cn + sites.json name（seed 链
//      UPDATE sites 行 name——OTA 重启自动刷正；release_group_mappings
//      seed 按 site_name 键 upsert 新键行，旧键行残留由本 migration 刷正）
//   2. DB 站名键表存量 UPDATE（8 表）：sites.name + release_group_
//      mappings/site_field_mappings/seeding_torrent_records/site_
//      coverage_cache/site_traffic_daily/site_traffic_flows/torrent_
//      traffic 的 site_name
//   3. docs 三处勘误（00-站点域名/32-README/36-认证采集手册）
// 零引用表（已审计）：publish_exclusions/rss_subscriptions/site_config_
// overrides/scoring_logs/fetch_select_events/iyuu_site_mappings/
// client_publish_targets/publish_group_members；代码零硬编码。
// 幂等：WHERE 旧名再跑扫 0 行 no-op。
func init() {
	RegisterMigration(99, "tangping_site_name_fix", func(gormDB *gorm.DB) error {
		return gormDB.Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec(`UPDATE sites SET name = '躺平', updated_at = datetime('now') WHERE name = '趟平'`).Error; err != nil {
				return err
			}
			for _, table := range []string{
				"release_group_mappings",
				"site_field_mappings",
				"seeding_torrent_records",
				"site_coverage_cache",
				"site_traffic_daily",
				"site_traffic_flows",
				"torrent_traffic",
			} {
				if err := tx.Exec(`UPDATE ` + table + ` SET site_name = '躺平' WHERE site_name = '趟平'`).Error; err != nil {
					return err
				}
			}
			return nil
		})
	})
}
