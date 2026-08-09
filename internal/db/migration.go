package db

import (
	"fmt"
	"sort"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
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
}
