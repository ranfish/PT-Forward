package db

import (
	"gorm.io/gorm"
)

// migration99 §59.415: 站名笔误刷正——"趟平"→"躺平"（www.tangpt.top）。
// 收录时站名笔误（tangpt=躺平拼音——站名应为"躺平"）。
// 修正三件套（§59.365 域名迁移先例变体——站名键非域名键）：
//   1. 资产源头：supported_sites.json name_cn + sites.json name
//   2. DB 站名键表存量 UPDATE（8 表）
//   3. docs 三处勘误（00-站点域名/32-README/36-认证采集手册）
//
// v2 修复（v1 撞 UNIQUE crash loop 教训——§59.415 附记）：启动序列
// seed 先于 migration——site_field_mappings 已被 seed 按"躺平"新键
// 写入 114 行（与"趟平"旧行内容全等），v1 纯 UPDATE 撞 (site_name,
// field_type, source_value) 唯一约束失败退出（事务回滚 DB 无害）。
// UNIQUE 约束审计（同族防线）：
//   - 含 site_name 三表 site_field_mappings/coverage_cache/traffic_
//     daily：UPDATE OR IGNORE + DELETE 残留组合——无冲突环境全改/
//     双份环境（seed 新行已在）旧行 IGNORE 后清冗余（信息由新行
//     承载零丢失）/新环境 0 行操作，三态收敛
//   - 不涉 site_name 四表（group_mappings 全局单列 group_name/
//     seeding client+hash/flows+torrent_traffic 无 UNIQUE）：纯
//     UPDATE 安全
// 零引用表（已审计）：publish_exclusions/rss_subscriptions/site_
// config_overrides/scoring_logs/fetch_select_events/iyuu_site_
// mappings/client_publish_targets/publish_group_members；代码零硬编码。
// 幂等：WHERE 旧名再跑扫 0 行 no-op。
func init() {
	RegisterMigration(99, "tangping_site_name_fix", func(gormDB *gorm.DB) error {
		return gormDB.Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec(`UPDATE sites SET name = '躺平', updated_at = datetime('now') WHERE name = '趟平'`).Error; err != nil {
				return err
			}
			// 约束不涉 site_name 的表：纯 UPDATE
			for _, table := range []string{
				"release_group_mappings",
				"seeding_torrent_records",
				"site_traffic_flows",
				"torrent_traffic",
			} {
				if err := tx.Exec(`UPDATE ` + table + ` SET site_name = '躺平' WHERE site_name = '趟平'`).Error; err != nil {
					return err
				}
			}
			// UNIQUE 含 site_name 的表：IGNORE 改名 + 清冗余旧行
			// （撞约束旧行=已存在等价新行，删除零丢失）
			for _, table := range []string{
				"site_field_mappings",
				"site_coverage_cache",
				"site_traffic_daily",
			} {
				if err := tx.Exec(`UPDATE OR IGNORE ` + table + ` SET site_name = '躺平' WHERE site_name = '趟平'`).Error; err != nil {
					return err
				}
				if err := tx.Exec(`DELETE FROM ` + table + ` WHERE site_name = '趟平'`).Error; err != nil {
					return err
				}
			}
			return nil
		})
	})
}
