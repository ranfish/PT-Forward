package model

import (
	"github.com/ranfish/pt-forward/internal/setting"
	"gorm.io/gorm"
)

func AutoMigrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&User{},
		&Site{},
		&ClientConfig{},
		&ClientPathMapping{},
		&ClientPublishTarget{},
		&RSSSubscription{},
		&RSSSubscriptionRule{},
		&RSSTorrentSeen{},
		&RSSFetchLog{},
		&FilterRule{},
		&DeleteRule{},
		&TorrentEvent{},
		&PublishCandidate{},
		&PublishGroup{},
		&PublishGroupMember{},
		&PublishGroupStatusHistory{},
		&PublishResultRecord{},
		&PublishTask{},
		&PublishExclusion{},
		&SiteFieldMapping{},
		&SiteConfigOverride{},
		&ContentFingerprint{},
		&SearchCache{},
		&PTGenCache{},
		&NotificationChannel{},
		&NotificationHistory{},
		&SeedingTorrentRecord{},
		&SeedingClientConfig{},
		&SeedingClientState{},
		&TorrentTraffic{},
		&DownloaderSpeedSnapshot{},
		&SiteTrafficDaily{},
		&TrafficStatsHourly{},
		&ReseedTask{},
		&ReseedMatch{},
		&ReseedNegativeCache{},
		&ReseedIYUULog{},
		&ReseedFeatureLog{},
		&CookieCloudSyncHistory{},
		&CookieCloudConfig{},
		&FreezeEventRecord{},
		&ScoringLog{},
		&IYUUConfig{},
		&IYUUSiteMapping{},
		&CloudFPConfig{},
		&OperationAuditLog{},
		&SchedulerTaskOverride{},
		&FreeWaitEntry{},
		&DownloadTask{},
		&DownloadClientConfig{},
		&SchemaMigration{},
		&SiteCoverageCache{},
		&CoverageQueryState{},
		&ReleaseGroupMapping{},
		&OrphanScanConfig{},
		&SitePublishLimit{},
		&TorrentMetadata{},
		&StandardKey{},
		&PublishSetting{},
		&TitleRule{},
		&ComplianceRule{},
		&setting.Setting{},
	); err != nil {
		return err
	}
	migrateAutoTransferColumns(db)
	return nil
}

// migrateAutoTransferColumns §55.11: 旧列 auto_reseed/reseed_client_ids 数据迁移到 auto_transfer/transfer_client_ids。
// gorm AutoMigrate 加新列但不迁数据，此处补迁移（旧列保留不删，无副作用）。
func migrateAutoTransferColumns(db *gorm.DB) {
	var hasOldCol int64
	db.Raw("SELECT COUNT(*) FROM pragma_table_info('rss_subscriptions') WHERE name='auto_reseed'").Scan(&hasOldCol)
	if hasOldCol == 0 {
		return
	}
	db.Exec("UPDATE rss_subscriptions SET auto_transfer = auto_reseed WHERE auto_reseed = 1 AND auto_transfer = 0")
	db.Exec("UPDATE rss_subscriptions SET transfer_client_ids = reseed_client_ids WHERE reseed_client_ids IS NOT NULL AND reseed_client_ids != '' AND (transfer_client_ids IS NULL OR transfer_client_ids = '' OR transfer_client_ids = 'null')")
}
