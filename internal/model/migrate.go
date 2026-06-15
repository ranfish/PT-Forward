package model

import (
	"github.com/ranfish/pt-forward/internal/setting"
	"gorm.io/gorm"
)

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
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
		&ReseedTask{},
		&ReseedMatch{},
		&ReseedNegativeCache{},
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
		&SchemaMigration{},
		&setting.Setting{},
	)
}
