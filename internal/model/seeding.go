package model

import "time"

// §33.1.5 — SeedingTorrentRecord: 刷流种子记录
type SeedingTorrentRecord struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	ClientID  string `json:"client_id" gorm:"size:50;not null;uniqueIndex:idx_client_hash"`
	InfoHash  string `json:"info_hash" gorm:"size:40;not null;uniqueIndex:idx_client_hash"`
	SiteName  string `json:"site_name" gorm:"size:50;not null;index"`
	TorrentID string `json:"torrent_id" gorm:"size:50;not null"`

	HasHR       bool          `json:"has_hr" gorm:"default:false"`
	HRSeedTimeH int           `json:"hr_seed_time_h" gorm:"default:0"`
	IsFree      bool          `json:"is_free" gorm:"default:false"`
	FreeEndAt   *time.Time    `json:"free_end_at"`
	FreeLevel   string        `json:"free_level" gorm:"size:20"`
	Discount    DiscountLevel `json:"discount" gorm:"size:20;default:'NONE'"`

	Status        SeedingTorrentStatus `json:"status" gorm:"size:20;not null;default:'seeding';index"`
	LastActionBy  string               `json:"last_action_by" gorm:"size:100"`
	Source        string               `json:"source" gorm:"size:20;default:'rss'"`
	SubscriptionID string     `json:"subscription_id" gorm:"size:50;index"`
	FlushedAt      *time.Time `json:"flushed_at" gorm:"index"`

	FirstMatchedAt    *time.Time `json:"first_matched_at" gorm:"index"`
	FinalUploaded     int64      `json:"final_uploaded" gorm:"default:0"`
	FinalDownloaded   int64      `json:"final_downloaded" gorm:"default:0"`
	TorrentSize       int64      `json:"torrent_size" gorm:"default:0"`

	Unregistered     bool       `json:"unregistered" gorm:"default:false"`
	UnregisteredAt   *time.Time `json:"unregistered_at"`
	UnregisteredMsg  string     `json:"unregistered_msg" gorm:"size:200"`
}

func (SeedingTorrentRecord) TableName() string { return "seeding_torrent_records" }

// §33.1.6 — SeedingClientConfig: 刷流下载器配置（34 字段）
type SeedingClientConfig struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	ClientID  string    `json:"client_id" gorm:"uniqueIndex;size:50;not null"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Enabled   bool      `json:"enabled" gorm:"default:true"`

	DeleteRuleIDs string `json:"delete_rule_ids" gorm:"type:text"`
	RejectRuleIDs string `json:"reject_rule_ids" gorm:"type:text"`

	AutoDeleteCron string `json:"auto_delete_cron" gorm:"size:100;default:'*/30 * * * *'"`
	MainDataCron   string `json:"maindata_cron" gorm:"size:100;default:'*/10 * * * * *'"`
	FitTimeCheckMs int    `json:"fit_time_check_ms" gorm:"default:2000"`

	DiskProtectEnabled  bool    `json:"disk_protect_enabled" gorm:"default:true"`
	MinDiskSpaceGB      float64 `json:"min_disk_space_gb" gorm:"default:50"`
	EmergencyBuffer     float64 `json:"emergency_buffer" gorm:"default:0.2"`
	SpaceAlarmEnabled   bool    `json:"space_alarm_enabled" gorm:"default:false"`
	SpaceAlarmGB        float64 `json:"space_alarm_gb" gorm:"default:10"`
	MinDiskSpacePercent float64 `json:"min_disk_space_percent" gorm:"default:0"`

	MaxActiveUploads    int  `json:"max_active_uploads" gorm:"default:0"`
	MaxActiveDownloads  int  `json:"max_active_downloads" gorm:"default:0"`
	MaxActiveSeeding    int  `json:"max_active_seeding" gorm:"default:100"`
	SuperSeedingDefault bool `json:"super_seeding_default" gorm:"default:false"`

	Scope string `json:"scope" gorm:"size:16;not null;default:'managed'"`

	PreFilterEnabled     bool `json:"pre_filter_enabled" gorm:"default:true"`
	EnhancementBatchSize int  `json:"enhancement_batch_size" gorm:"default:20"`
	EnhancementCacheTTL  int  `json:"enhancement_cache_ttl" gorm:"default:600"`

	ActiveTimeWindows string `json:"active_time_windows" gorm:"size:256"`

	EmaAlpha float64 `json:"ema_alpha" gorm:"default:0.1"`

	CleanupScoreWeights string `json:"cleanup_score_weights" gorm:"type:text"`

	ArchiveGranularity string `json:"archive_granularity" gorm:"size:20;default:'daily'"`

	ReannounceBefore     bool `json:"reannounce_before" gorm:"default:true"`
	ReannounceRetries    int  `json:"reannounce_retries" gorm:"default:2"`
	ReannounceIntervalMs int  `json:"reannounce_interval_ms" gorm:"default:3000"`
	ReannounceWaitMs     int  `json:"reannounce_wait_ms" gorm:"default:2000"`

	MinSeedHoursBeforeDelete float64 `json:"min_seed_hours_before_delete" gorm:"default:1"`
}

func (SeedingClientConfig) TableName() string { return "seeding_client_configs" }

// §33.1.84 — SeedingClientState: 刷流下载器持久化状态
type SeedingClientState struct {
	ID               uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	ClientID         string    `gorm:"uniqueIndex;not null" json:"client_id"`
	UpdatedAt        time.Time `json:"updated_at"`
	AvgUploadSpeed   float64   `json:"avg_upload_speed"`
	AvgDownloadSpeed float64   `json:"avg_download_speed"`
	Initialized      bool      `json:"initialized"`
	WasLowSpace      bool      `json:"was_low_space"`
	AllTimeUpload    int64     `gorm:"default:0" json:"alltime_upload"`
	AllTimeDownload  int64     `gorm:"default:0" json:"alltime_download"`
	DayStartUpload   int64     `gorm:"default:0" json:"day_start_upload"`
	DayStartDownload int64     `gorm:"default:0" json:"day_start_download"`
	DayStartDate     string    `gorm:"default:''" json:"day_start_date"`
}

func (SeedingClientState) TableName() string { return "seeding_client_states" }

// §33.1.76 — SeedingCandidate: 刷流候选
// v2: reserved — 待实现时激活
type SeedingCandidate struct {
	SubscriptionID string        `json:"subscription_id"`
	ClientID       string        `json:"client_id"`
	Event          *TorrentEvent `json:"event"`
	CollectedAt    time.Time     `json:"collected_at"`
}

// §33.1.78 — SeedingTorrentView: 刷流种子 API 聚合视图
// v2: reserved — 待实现时激活
type SeedingTorrentView struct {
	ID            uint      `json:"id"`
	InfoHash      string    `json:"info_hash"`
	TorrentName   string    `json:"torrent_name"`
	SiteName      string    `json:"site_name"`
	Size          int64     `json:"size"`
	Seeders       int       `json:"seeders"`
	Leechers      int       `json:"leechers"`
	UploadBytes   int64     `json:"upload_bytes"`
	Ratio         float64   `json:"ratio"`
	SeedTimeHours float64   `json:"seed_time_hours"`
	AgeHours      float64   `json:"age_hours"`
	Status        string    `json:"status"`
	Score         *float64  `json:"score,omitempty"`
	LastCheckedAt time.Time `json:"last_checked_at"`
	AddedAt       time.Time `json:"added_at"`
}

// §14 — DeleteRule: 删种/保护规则
type DeleteRule struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	Alias    string `json:"alias" gorm:"size:100;not null"`
	Priority int    `json:"priority" gorm:"default:0"`
	Enabled  bool   `json:"enabled" gorm:"default:true"`

	Type       string `json:"type" gorm:"size:20;not null;default:'normal'"`
	Logic      string `json:"logic" gorm:"size:10;not null;default:'and'"`
	Conditions string `json:"conditions" gorm:"type:text"`
	Expr       string `json:"expr" gorm:"type:text"`

	FitTime int `json:"fit_time" gorm:"default:0"`

	Action            string `json:"action" gorm:"size:20;not null;default:'delete'"`
	DeleteNum         int    `json:"delete_num" gorm:"default:1"`
	RemoveData        bool   `json:"remove_data" gorm:"default:true"`
	OnlyDeleteTorrent bool   `json:"only_delete_torrent" gorm:"default:false"`
	LimitSpeedBytes   int64  `json:"limit_speed_bytes" gorm:"default:0"`

	ReannounceBefore     bool `json:"reannounce_before" gorm:"default:true"`
	ReannounceWaitMs     int  `json:"reannounce_wait_ms" gorm:"default:2000"`
	ReannounceRetries    int  `json:"reannounce_retries" gorm:"default:2"`
	ReannounceIntervalMs int  `json:"reannounce_interval_ms" gorm:"default:3000"`

	CascadeDelete   bool `json:"cascade_delete" gorm:"default:false"`
	CascadeMaxDepth int  `json:"cascade_max_depth" gorm:"default:1"`
}

func (DeleteRule) TableName() string { return "delete_rules" }

// §14 — TorrentTraffic: 种子级流量快照
type TorrentTraffic struct {
	ID            uint      `gorm:"primaryKey;autoIncrement"`
	ClientID      string    `gorm:"index:idx_traffic_time"`
	InfoHash      string    `gorm:"index:idx_traffic_time"`
	SiteName      string    `gorm:"index"`
	Uploaded      int64     `gorm:""`
	Downloaded    int64     `gorm:""`
	UploadSpeed   int64     `gorm:""`
	DownloadSpeed int64     `gorm:""`
	Ratio         float64   `gorm:""`
	RecordedAt    time.Time `gorm:"index:idx_traffic_time"`
}

func (TorrentTraffic) TableName() string { return "torrent_traffic" }

// §14 — DownloaderSpeedSnapshot: 下载器级速度快照
type DownloaderSpeedSnapshot struct {
	ID             uint      `gorm:"primaryKey;autoIncrement"`
	ClientID       string    `gorm:"index"`
	UploadSpeed    int64     `gorm:""`
	DownloadSpeed  int64     `gorm:""`
	FreeSpaceBytes int64     `gorm:""`
	ActiveTorrents int       `gorm:""`
	RecordedAt     time.Time `gorm:"index"`
}

func (DownloaderSpeedSnapshot) TableName() string { return "downloader_speed_snapshots" }

// §14 — SiteTrafficDaily: 站点级日流量
type SiteTrafficDaily struct {
	ID            uint      `gorm:"primaryKey;autoIncrement"`
	SiteName      string    `gorm:"uniqueIndex:idx_site_date"`
	Date          time.Time `gorm:"uniqueIndex:idx_site_date"`
	UploadDelta   int64     `gorm:""`
	DownloadDelta int64     `gorm:""`
	TorrentCount  int       `gorm:""`
	SeedingCount  int       `gorm:""`
}

func (SiteTrafficDaily) TableName() string { return "site_traffic_daily" }

// §14 — FreezeEventRecord: 冻结事件持久化
type FreezeEventRecord struct {
	ID       uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Domain   string    `json:"domain" gorm:"index;size:100"`
	Reason   string    `json:"reason" gorm:"size:64"`
	Duration int64     `json:"duration_ms"`
	URL      string    `json:"url" gorm:"size:512"`
	At       time.Time `json:"at" gorm:"index"`
}

func (FreezeEventRecord) TableName() string { return "freeze_events" }

// §18.6 — SeedingScoringConfig: 刷流评分配置（embedded in RSSSubscription）
type SeedingScoringConfig struct {
	Enabled bool `json:"enabled" gorm:"column:scoring_enabled;default:false"`

	HalfLifeHours   float64 `json:"half_life_hours" gorm:"default:2"`
	SiteWeightsJSON string  `json:"site_weights_json" gorm:"type:text;default:'{}'"`
	Include2xUp     bool    `json:"include_2xup" gorm:"default:false"`

	MaxCandidates    int     `json:"max_candidates" gorm:"default:50"`
	MaxActiveSeeding int     `json:"max_active_seeding" gorm:"default:100"`
	TopNConfirm      int     `json:"top_n_confirm" gorm:"default:10"`
	BatchLimit       int     `json:"batch_limit" gorm:"default:10"`
	MinScore         float64 `json:"min_score" gorm:"default:1.0"`

	PushIntervalMs int `json:"push_interval_ms" gorm:"default:0"`
}

// §33.1.67 — SeedingSearchResult: 刷流搜索结果
type SeedingSearchResult struct {
	TorrentID   string        `json:"torrent_id"`
	Title       string        `json:"title"`
	Size        int64         `json:"size"`
	Seeders     int           `json:"seeders"`
	Leechers    int           `json:"leechers"`
	Discount    DiscountLevel `json:"discount"`
	PublishAt   time.Time     `json:"publish_at"`
	DetailURL   string        `json:"detail_url"`
	DownloadURL string        `json:"download_url"`
}

// §33.1.85 — CleanupScoreWeights: 清理评分权重
type CleanupScoreWeights struct {
	SeedHours   float64 `json:"seed_hours"`
	UploadSpeed float64 `json:"upload_speed"`
	Ratio       float64 `json:"ratio"`
	DiskUsage   float64 `json:"disk_usage"`
}

type ScoringLog struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	CycleID   string    `json:"cycle_id" gorm:"size:30;index;not null"`
	ClientID  string    `json:"client_id" gorm:"size:50;index;not null"`
	InfoHash  string    `json:"info_hash" gorm:"size:40;index;not null"`
	SiteName  string    `json:"site_name" gorm:"size:100"`
	TorrentID string    `json:"torrent_id" gorm:"size:50"`
	Score     float64   `json:"score"`
	Demand    float64   `json:"demand"`
	UploadVal float64   `json:"upload_val"`
	Recency   float64   `json:"recency"`
	Seeders   int       `json:"seeders"`
	Leechers  int       `json:"leechers"`
	AgeHours  float64   `json:"age_hours"`
	Discount  string    `json:"discount" gorm:"size:20"`
	IsFree    bool      `json:"is_free"`
	HasHR     bool      `json:"has_hr"`
	UploadSpeed int64   `json:"upload_speed"`
	CreatedAt time.Time `json:"created_at" gorm:"index"`
}

func (ScoringLog) TableName() string { return "scoring_logs" }
