package model

import "time"

type DownloadClientConfig struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	ClientID      string `json:"client_id" gorm:"uniqueIndex;size:50;not null"`
	Enabled       bool   `json:"enabled" gorm:"default:true"`
	DeleteRuleIDs string `json:"delete_rule_ids" gorm:"type:text"`

	AutoDeleteCron string `json:"auto_delete_cron" gorm:"size:100;default:'*/30 * * * *'"`
	MainDataCron   string `json:"main_data_cron" gorm:"size:100;default:'*/20 * * * *'"`

	DiskProtectEnabled  bool    `json:"disk_protect_enabled" gorm:"default:true"`
	MinDiskSpaceGB      float64 `json:"min_disk_space_gb" gorm:"default:50"`
	SpaceAlarmEnabled   bool    `json:"space_alarm_enabled" gorm:"default:false"`
	SpaceAlarmGB        float64 `json:"space_alarm_gb" gorm:"default:10"`
	MinDiskSpacePercent float64 `json:"min_disk_space_percent" gorm:"default:0"`

	MaxActiveUploads   int `json:"max_active_uploads" gorm:"default:0"`
	MaxActiveDownloads int `json:"max_active_downloads" gorm:"default:0"`

	SuperSeedingDefault bool `json:"super_seeding_default" gorm:"default:false"`

	Scope string `json:"scope" gorm:"size:16;not null;default:'managed'"`

	ReannounceBefore     bool `json:"reannounce_before" gorm:"default:true"`
	ReannounceRetries    int  `json:"reannounce_retries" gorm:"default:2"`
	ReannounceIntervalMs int  `json:"reannounce_interval_ms" gorm:"default:3000"`
	ReannounceWaitMs     int  `json:"reannounce_wait_ms" gorm:"default:2000"`
}

func (DownloadClientConfig) TableName() string { return "download_client_configs" }
