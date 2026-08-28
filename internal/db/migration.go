package db

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/metadata/extract"
	"github.com/ranfish/pt-forward/internal/titleparser"
	"github.com/ranfish/pt-forward/internal/util"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Migration struct {
	Version int
	Name    string
	Up      func(*gorm.DB) error
}

var registeredMigrations []Migration

func RegisterMigration(version int, name string, up func(*gorm.DB) error) {
	registeredMigrations = append(registeredMigrations, Migration{Version: version, Name: name, Up: up})
}

func RunMigrations(gormDB *gorm.DB, log *zap.Logger) error {
	migrations := make([]Migration, len(registeredMigrations))
	copy(migrations, registeredMigrations)
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})
	for _, m := range migrations {
		var count int64
		if err := gormDB.Model(&model.SchemaMigration{}).Where("version = ?", m.Version).Count(&count).Error; err != nil {
			return fmt.Errorf("check migration %d: %w", m.Version, err)
		}
		if count > 0 {
			continue
		}
		log.Info("running data migration", zap.Int("version", m.Version), zap.String("name", m.Name))
		if err := m.Up(gormDB); err != nil {
			return fmt.Errorf("migration %d (%s) failed: %w", m.Version, m.Name, err)
		}
		if err := gormDB.Create(&model.SchemaMigration{Version: m.Version, AppliedAt: time.Now()}).Error; err != nil {
			return fmt.Errorf("record migration %d: %w", m.Version, err)
		}
	}
	return nil
}

func init() {
	RegisterMigration(1, "ema_alpha_default_0.1_to_0.3", func(gormDB *gorm.DB) error {
		return gormDB.Model(&model.SeedingClientConfig{}).Where("ema_alpha = ?", 0.1).Update("ema_alpha", 0.3).Error
	})
	RegisterMigration(2, "backfill_download_tasks_completed_at", func(gormDB *gorm.DB) error {
		return gormDB.Model(&model.DownloadTask{}).
			Where("status = ? AND completed_at IS NULL", model.DownloadStatusCompleted).
			Update("completed_at", gorm.Expr("updated_at")).Error
	})
	RegisterMigration(3, "backfill_reseed_matches_source_torrent_id", func(gormDB *gorm.DB) error {
		return gormDB.Exec(`UPDATE reseed_matches SET source_torrent_id = (
			SELECT torrent_id FROM seeding_torrent_records
			WHERE seeding_torrent_records.info_hash = reseed_matches.source_info_hash
			LIMIT 1
		) WHERE LENGTH(source_torrent_id) >= 40`).Error
	})
	RegisterMigration(4, "merge_download_client_configs_into_seeding", func(gormDB *gorm.DB) error {
		return gormDB.Exec(`INSERT INTO seeding_client_configs (
			client_id, created_at, updated_at, enabled, delete_rule_ids,
			auto_delete_cron, main_data_cron,
			disk_protect_enabled, min_disk_space_gb,
			space_alarm_enabled, space_alarm_gb, min_disk_space_percent,
			max_active_uploads, max_active_downloads, max_active_seeding,
			super_seeding_default, scope, role,
			reannounce_before, reannounce_retries, reannounce_interval_ms, reannounce_wait_ms
		)
		SELECT
			client_id, created_at, updated_at, enabled, delete_rule_ids,
			auto_delete_cron, main_data_cron,
			disk_protect_enabled, min_disk_space_gb,
			space_alarm_enabled, space_alarm_gb, min_disk_space_percent,
			max_active_uploads, max_active_downloads,
			CASE WHEN max_active_uploads > 0 THEN max_active_uploads
			     WHEN max_active_downloads > 0 THEN max_active_downloads
			     ELSE 100 END,
			super_seeding_default, scope, 'download',
			reannounce_before, reannounce_retries, reannounce_interval_ms, reannounce_wait_ms
		FROM download_client_configs
		WHERE client_id NOT IN (SELECT client_id FROM seeding_client_configs)`).Error
	})
	RegisterMigration(5, "rss_torrent_seen_unique_add_subscription_id", func(gormDB *gorm.DB) error {
		return gormDB.Transaction(func(tx *gorm.DB) error {
			tableDef := `CREATE TABLE rss_torrent_seen_new (
				id integer PRIMARY KEY AUTOINCREMENT,
				created_at datetime,
				updated_at datetime,
				site_name text NOT NULL,
				torrent_id text NOT NULL,
				subscription_id text NOT NULL,
				info_hash text,
				is_fake_hash numeric DEFAULT 0,
				title text,
				size integer,
				is_free numeric DEFAULT 0,
				free_end_at datetime,
				free_level text,
				discount text DEFAULT 'NONE',
				has_hr numeric DEFAULT 0,
				hr_seed_time_h integer DEFAULT 0,
				status text NOT NULL DEFAULT 'seen',
				source_category text,
				matched_rule text,
				skip_count integer NOT NULL DEFAULT 0,
				last_check_time datetime,
				push_time datetime
			)`
			if err := tx.Exec(tableDef).Error; err != nil {
				return err
			}
			if err := tx.Exec(`INSERT INTO rss_torrent_seen_new (
				id, created_at, updated_at, site_name, torrent_id, subscription_id,
				info_hash, is_fake_hash, title, size, is_free, free_end_at,
				free_level, discount, has_hr, hr_seed_time_h, status,
				source_category, matched_rule, skip_count, last_check_time, push_time
			) SELECT
				id, created_at, updated_at, site_name, torrent_id, subscription_id,
				info_hash, COALESCE(is_fake_hash, 0), title, size, COALESCE(is_free, 0), free_end_at,
				free_level, COALESCE(discount, 'NONE'), COALESCE(has_hr, 0), COALESCE(hr_seed_time_h, 0), COALESCE(status, 'seen'),
				source_category, matched_rule, COALESCE(skip_count, 0), last_check_time, push_time
			FROM rss_torrent_seen`).Error; err != nil {
				return err
			}
			if err := tx.Exec(`DROP TABLE rss_torrent_seen`).Error; err != nil {
				return err
			}
			if err := tx.Exec(`ALTER TABLE rss_torrent_seen_new RENAME TO rss_torrent_seen`).Error; err != nil {
				return err
			}
			if err := tx.Exec(`CREATE UNIQUE INDEX idx_site_torrent_sub ON rss_torrent_seen(site_name, torrent_id, subscription_id)`).Error; err != nil {
				return err
			}
			if err := tx.Exec(`CREATE INDEX idx_torrent_seen_info_hash ON rss_torrent_seen(info_hash)`).Error; err != nil {
				return err
			}
			if err := tx.Exec(`CREATE INDEX idx_torrent_seen_status ON rss_torrent_seen(status)`).Error; err != nil {
				return err
			}
			if err := tx.Exec(`CREATE INDEX idx_rss_torrent_seen_subscription_id ON rss_torrent_seen(subscription_id)`).Error; err != nil {
				return err
			}
			return nil
		})
	})
	RegisterMigration(6, "drop_legacy_download_client_configs", func(gormDB *gorm.DB) error {
		return gormDB.Migrator().DropTable("download_client_configs")
	})
	RegisterMigration(7, "close_is_source_for_sites_without_group_mappings", func(gormDB *gorm.DB) error {
		return gormDB.Exec(`UPDATE sites SET is_source = 0 WHERE is_source = 1 AND name NOT IN (
			SELECT DISTINCT site_name FROM release_group_mappings WHERE site_name != ''
		)`).Error
	})
	RegisterMigration(8, "rename_main_data_cron_column", func(gormDB *gorm.DB) error {
		// §59.20: MainDataCron JSON tag 是 maindata_cron，GORM 蛇形推导是 main_data_cron
		// 加 column:maindata_cron tag 后需要处理列名变更
		var oldExists, newExists string
		gormDB.Raw(`SELECT name FROM pragma_table_info('seeding_client_configs') WHERE name = 'main_data_cron' LIMIT 1`).Scan(&oldExists)
		gormDB.Raw(`SELECT name FROM pragma_table_info('seeding_client_configs') WHERE name = 'maindata_cron' LIMIT 1`).Scan(&newExists)
		if oldExists == "main_data_cron" && newExists == "maindata_cron" {
			// 两列都存在（AutoMigrate 已创建新列）→ 复制数据后删旧列
			if err := gormDB.Exec(`UPDATE seeding_client_configs SET maindata_cron = main_data_cron WHERE maindata_cron = '' OR maindata_cron IS NULL`).Error; err != nil {
				return err
			}
			return gormDB.Exec(`ALTER TABLE seeding_client_configs DROP COLUMN main_data_cron`).Error
		} else if oldExists == "main_data_cron" && newExists == "" {
			// 仅有旧列 → 直接重命名
			return gormDB.Exec(`ALTER TABLE seeding_client_configs RENAME COLUMN main_data_cron TO maindata_cron`).Error
		}
		// 只有新列或都不存在 → 无需操作
		return nil
	})
	RegisterMigration(9, "enable_pieces_hash_api_for_yemapt", func(gormDB *gorm.DB) error {
		return gormDB.Exec(`UPDATE sites SET supports_pieces_hash_api = 1 WHERE framework = 'yemapt' AND enabled = 1`).Error
	})
	RegisterMigration(10, "sync_yemapt_auth_type", func(gormDB *gorm.DB) error {
		return gormDB.Exec(`UPDATE sites SET auth_type = 'apikey' WHERE framework = 'yemapt'`).Error
	})
	RegisterMigration(11, "enable_yemapt_pieces_hash_via_gorm", func(gormDB *gorm.DB) error {
		// migration #9 raw SQL 列名不匹配（已执行无效果）
		return gormDB.Model(&model.Site{}).Where("framework = ?", "yemapt").Update("supports_pieces_hash_api", true).Error
	})
	RegisterMigration(12, "enable_yemapt_pieces_hash_struct", func(gormDB *gorm.DB) error {
		// #11 用字符串列名仍不匹配，用 struct 让 GORM 自己解析
		return gormDB.Model(&model.Site{}).Where("framework = ?", "yemapt").
			Updates(model.Site{SupportsPiecesHashAPI: true}).Error
	})
	// §59.34: 存量 source_type/specification 重算。
	// 旧 splitMedium 把压制写法（BluRay/UHD BluRay 无连字符）一律归为原盘写法
	// （带连字符），Encode 语义丢失。按 v1.05 忠实原文写法重算。
	RegisterMigration(13, "recompute_torrent_metadata_source_type", func(gormDB *gorm.DB) error {
		const batchSize = 500
		var lastID uint
		for {
			var rows []model.TorrentMetadata
			if err := gormDB.Where("id > ? AND title != ''", lastID).
				Order("id ASC").Limit(batchSize).Find(&rows).Error; err != nil {
				return err
			}
			if len(rows) == 0 {
				return nil
			}
			for _, row := range rows {
				lastID = row.ID
				p := titleparser.ParseTitleTech(row.Title)
				if err := gormDB.Model(&model.TorrentMetadata{}).Where("id = ?", row.ID).
					Updates(map[string]interface{}{
						"source_type":   p.SourceType,
						"specification": p.Specification,
					}).Error; err != nil {
					return err
				}
			}
		}
	})
	// §59.46: 清 doubaninfo 坏路径时期写入的空 BBCode 缓存——
	// queryDoubanInfo 修复前 raw_bbcode 恒空落缓存（140 行），30 天 TTL 内
	// 命中即短路远端重取，修复不生效。删行让下次 Query 穿透（poster_url 为
	// 独立列但整行删除后海报穿透一次 doubaninfo ~1s，代价可忽略）。
	RegisterMigration(18, "purge_empty_bbcode_ptgen_cache", func(gormDB *gorm.DB) error {
		return gormDB.Where("json_data LIKE '%\"raw_bbcode\":\"\"%'").
			Delete(&model.PTGenCache{}).Error
	})
	// §59.61 管道层: torrent_snapshots 加 comment 列——簇直达判据凭证。
	// 新库 AutoMigrate 已带列；存量库（29/243 等）ALTER 补列 + 复合簇索引。
	RegisterMigration(19, "snapshot_add_comment_column", func(gormDB *gorm.DB) error {
		var hasCol int64
		gormDB.Raw("SELECT COUNT(*) FROM pragma_table_info('torrent_snapshots') WHERE name='comment'").Scan(&hasCol)
		if hasCol == 0 {
			if err := gormDB.Exec("ALTER TABLE torrent_snapshots ADD COLUMN comment text").Error; err != nil {
				return err
			}
		}
		var hasIdx int64
		gormDB.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='idx_snapshots_cluster'").Scan(&hasIdx)
		if hasIdx == 0 {
			if err := gormDB.Exec("CREATE INDEX idx_snapshots_cluster ON torrent_snapshots(client_id, save_path, name)").Error; err != nil {
				return err
			}
		}
		return nil
	})
	// §59.63: 簇截图链接缓存表（观察期复用——清簇重取免重截重传）
	// §59.123: 站点别名域名补齐随版本同步（§59.60 O3 曾只在 29 手工 UPDATE——
	// 10 站 tracker_domains/alternative_domains 补实测别名; 243 实测 9/10 缺）
	// §59.136: 运营标记存量清洗——副标题尾部 "[50%]"/"[中性种子(NL)]" 等（采集层
	// §59.99 词表漏百分比形态 + 混排 [禁转] 挡尾锚, 243 实测 32 行）。
	// 用 metadata.StripSiteOperationMarkers 公共方法（与采集层同一实现, [禁转] 保留）。
	RegisterMigration(25, "subtitle_op_markers_cleanup", func(gormDB *gorm.DB) error {
		type subRow struct {
			InfoHash string
			Subtitle string
		}
		var rows []subRow
		if err := gormDB.Table("torrent_metadata").
			Select("info_hash, subtitle").
			Where("subtitle LIKE ?", "%[%]").Find(&rows).Error; err != nil {
			return err
		}
		for _, r := range rows {
			cleaned := util.StripSiteOperationMarkers(r.Subtitle)
			if cleaned == r.Subtitle {
				continue
			}
			if err := gormDB.Table("torrent_metadata").
				Where("info_hash = ?", r.InfoHash).
				Update("subtitle", cleaned).Error; err != nil {
				return err
			}
		}
		return nil
	})
	RegisterMigration(24, "site_alias_domains_sync", func(gormDB *gorm.DB) error {
		type aliasRow struct {
			Site string
			Add  string // 追加到 alternative_domains（若主域未含）
		}
		aliases := []aliasRow{
			{"不可说", "hdcmct.org"}, {"猫", "pterclub.com"}, {"青蛙", "qingwapt.org"},
			{"憨憨", "hhanclub.top"}, {"13城", "13city.online"}, {"柠檬不甜", "lemonhd.club"},
			{"HDVideo", "hdvideo.one"}, {"莫妮卡", "mua.xloli.cc"}, {"拾刻", "ptskit.org"},
			{"萝莉", "azusa.wiki"},
		}
		for _, a := range aliases {
			var site model.Site
			if err := gormDB.Where("name = ?", a.Site).First(&site).Error; err != nil {
				continue // 站未配置——跳过（不杜撰站点）
			}
			// 已含（主域或别名任一）则跳过——幂等
			if strings.Contains(site.TrackerDomains, a.Add) || strings.Contains(site.AlternativeDomains, a.Add) {
				continue
			}
			extra := site.AlternativeDomains
			if extra != "" && !strings.HasSuffix(extra, ",") {
				extra += ","
			}
			extra += a.Add
			if err := gormDB.Model(&model.Site{}).Where("id = ?", site.ID).
				Update("alternative_domains", extra).Error; err != nil {
				return err
			}
		}
		return nil
	})
	RegisterMigration(23, "seeding_cleanup_thresholds", func(gormDB *gorm.DB) error {
		// §59.122: 评分清理阈值配置化（原硬编码 0.3/48h）
		var hasCol int64
		gormDB.Raw("SELECT COUNT(*) FROM pragma_table_info('seeding_client_configs') WHERE name='cleanup_min_score'").Scan(&hasCol)
		if hasCol == 0 {
			if err := gormDB.Exec("ALTER TABLE seeding_client_configs ADD COLUMN cleanup_min_score real DEFAULT 0").Error; err != nil {
				return err
			}
			if err := gormDB.Exec("ALTER TABLE seeding_client_configs ADD COLUMN cleanup_min_age_hours real DEFAULT 0").Error; err != nil {
				return err
			}
		}
		return nil
	})
	RegisterMigration(22, "metadata_add_audio_tracks", func(gormDB *gorm.DB) error {
		var hasCol int64
		gormDB.Raw("SELECT COUNT(*) FROM pragma_table_info('torrent_metadata') WHERE name='audio_tracks'").Scan(&hasCol)
		if hasCol == 0 {
			return gormDB.Exec("ALTER TABLE torrent_metadata ADD COLUMN audio_tracks integer DEFAULT 0").Error
		}
		return nil
	})
	RegisterMigration(20, "cluster_screenshot_cache", func(gormDB *gorm.DB) error {
		return gormDB.Migrator().CreateTable(&model.ClusterScreenshotCache{})
	})
	// §59.44: 存量尾斜杠路径归一——TR 上报的历史脏数据（"PT6/SSD/" vs "PT6/SSD"）
	// 在三元组资源键下劈裂资源，统一 Clean 形态（与 syncer 前置修复配套）。
	RegisterMigration(17, "normalize_snapshot_save_path", func(gormDB *gorm.DB) error {
		var rows []model.TorrentSnapshot
		if err := gormDB.Where("save_path LIKE ?", "%/").Find(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			clean := filepath.Clean(row.SavePath)
			if clean == row.SavePath {
				continue
			}
			// 同 (hash,client) UPSERT 键无冲突；仅路径串更新
			if err := gormDB.Model(&model.TorrentSnapshot{}).Where("id = ?", row.ID).
				Update("save_path", clean).Error; err != nil {
				return err
			}
		}
		return nil
	})
	// §59.40: 存量 tags 重刷——extractFlags 并入站方标签后，detail_source.tags
	// 有值的行重算 flags（标签禁转形态补检）。29 现存 tags 全空实际零行受影响，
	// 保险性补齐（其他环境若有标签禁转站采集数据则生效）。
	RegisterMigration(16, "rescan_flags_with_tags", func(gormDB *gorm.DB) error {
		var rows []model.TorrentMetadata
		if err := gormDB.
			Where("detail_source_json LIKE '%\"tags\": [%'").
			Find(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			title, subtitle, tags, descr := "", "", "", ""
			if row.DetailSourceJSON != "" {
				var ds struct {
					Title    string   `json:"title"`
					Subtitle string   `json:"subtitle"`
					Tags     []string `json:"tags"`
					Intro    struct {
						Body string `json:"body"`
					} `json:"intro"`
				}
				if json.Unmarshal([]byte(row.DetailSourceJSON), &ds) == nil {
					title, subtitle, descr = ds.Title, ds.Subtitle, ds.Intro.Body
					tags = strings.Join(ds.Tags, " ")
				}
			}
			newFlags := extract.ExtractFlagsFromText(title + " " + subtitle + " " + tags + " " + descr)
			nf, _ := json.Marshal(newFlags)
			if string(nf) != row.Flags {
				if err := gormDB.Model(&model.TorrentMetadata{}).Where("id = ?", row.ID).
					Update("flags", string(nf)).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
	// §59.39: 存量 flags 重刷——extractFlags 关键词层新增 quote 剥离后，
	// 上游声明（引用块内禁转词）误标的行解除。4/4 实证全部为同类误标。
	RegisterMigration(15, "rescan_torrent_metadata_flags_quote", func(gormDB *gorm.DB) error {
		var rows []model.TorrentMetadata
		if err := gormDB.
			Where("flags LIKE ? OR flags LIKE ? OR flags LIKE ? OR flags LIKE ? OR flags LIKE ? OR flags LIKE ?",
				"%禁转%", "%禁止转载%", "%谢绝%", "%严禁%", "%独占%", "%限转%").
			Find(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			title, descr := "", ""
			if row.DetailSourceJSON != "" {
				var ds struct {
					Title string `json:"title"`
					Intro struct {
						Body string `json:"body"`
					} `json:"intro"`
				}
				if json.Unmarshal([]byte(row.DetailSourceJSON), &ds) == nil {
					title, descr = ds.Title, ds.Intro.Body
				}
			}
			newFlags := extract.ExtractFlagsFromText(title + " " + descr)
			nf, _ := json.Marshal(newFlags)
			if string(nf) != row.Flags {
				if err := gormDB.Model(&model.TorrentMetadata{}).Where("id = ?", row.ID).
					Update("flags", string(nf)).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
	// §59.34 审计修复: #13 只按 title 重算，丢了 BuildTechProfile 的 DOM>title
	// 优先级（fetch 写入时 DetailSourceJSON.medium 覆盖 title 值）。本迁移按
	// fetch 同款管线重算：有 DOM medium 的行以 DOM 为准。
	RegisterMigration(14, "recompute_source_type_with_dom_priority", func(gormDB *gorm.DB) error {
		const batchSize = 500
		var lastID uint
		for {
			var rows []model.TorrentMetadata
			if err := gormDB.Where("id > ? AND title != ''", lastID).
				Order("id ASC").Limit(batchSize).Find(&rows).Error; err != nil {
				return err
			}
			if len(rows) == 0 {
				return nil
			}
			for _, row := range rows {
				lastID = row.ID
				var domMedium string
				if row.DetailSourceJSON != "" {
					var ds struct {
						Medium string `json:"medium"`
					}
					if json.Unmarshal([]byte(row.DetailSourceJSON), &ds) == nil {
						// standard key（medium.webdl/UNK*）→ 规范显示名；
						// 未映射 key → 空（保留 title 派生值）
						domMedium = titleparser.ReverseLookup(ds.Medium)
					}
				}
				// MI 不影响 source_type/specification（只填技术参数），传空
				p := titleparser.BuildTechProfile(row.Title, "", domMedium, "", "", "")
				if err := gormDB.Model(&model.TorrentMetadata{}).Where("id = ?", row.ID).
					Updates(map[string]interface{}{
						"source_type":   p.SourceType,
						"specification": p.Specification,
					}).Error; err != nil {
					return err
				}
			}
		}
	})
}
