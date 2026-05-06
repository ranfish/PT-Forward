package model

import "time"

// §33.1.46 — PTGenCache: PTGen 查询缓存
type PTGenCache struct {
	ID           uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	QueryKey     string    `json:"query_key" gorm:"size:500;not null;uniqueIndex"`
	DoubanURL    string    `json:"douban_url" gorm:"size:500"`
	IMDbURL      string    `json:"imdb_url" gorm:"size:500"`
	ChineseTitle string    `json:"chinese_title" gorm:"size:200"`
	PosterURL    string    `json:"poster_url" gorm:"size:500"`
	BBCode       string    `json:"bbcode" gorm:"type:text"`
	JSONData     string    `json:"json_data" gorm:"type:text"`
	Source       string    `json:"source" gorm:"size:50"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (PTGenCache) TableName() string { return "ptgen_cache" }

// CookieCloudConfig — CookieCloud 连接配置
type CookieCloudConfig struct {
	ID           uint       `json:"id" gorm:"primaryKey;autoIncrement"`
	ServerURL    string     `json:"server_url" gorm:"size:512;not null"`
	UUID         string     `json:"uuid" gorm:"size:128;not null"`
	Password     string     `json:"password" gorm:"size:128;not null"`
	CryptoType   string     `json:"crypto_type" gorm:"size:30;default:'legacy'"`
	SyncEnabled  bool       `json:"sync_enabled" gorm:"default:false"`
	SyncInterval int        `json:"sync_interval" gorm:"default:60"`
	LastSyncAt   *time.Time `json:"last_sync_at"`
}

func (CookieCloudConfig) TableName() string { return "cookie_cloud_configs" }

// CookieCloudSyncHistory — CookieCloud 同步历史
type CookieCloudSyncHistory struct {
	ID           uint                   `json:"id" gorm:"primaryKey;autoIncrement"`
	Status       string                 `json:"status" gorm:"size:20"`
	UpdateTime   time.Time              `json:"update_time"`
	SyncedSites  int                    `json:"synced_sites"`
	SkippedSites int                    `json:"skipped_sites"`
	ErrorMessage string                 `json:"error_message,omitempty" gorm:"type:text"`
	Errors       []CookieCloudSyncError `json:"errors,omitempty" gorm:"type:json;serializer:json"`
	SyncDuration time.Duration          `json:"sync_duration"`
	CreatedAt    time.Time              `json:"created_at"`
}

func (CookieCloudSyncHistory) TableName() string { return "cookie_cloud_sync_histories" }

// CookieCloudSyncError — CookieCloud 同步错误
type CookieCloudSyncError struct {
	Phase      string    `json:"phase"`
	Message    string    `json:"message"`
	Retryable  bool      `json:"retryable"`
	Timestamp  time.Time `json:"timestamp"`
	RetryCount int       `json:"retry_count"`
}
