package model

import (
	"time"

	"gorm.io/gorm"
)

// §33.1.1 — TorrentEvent: RSS → 系统核心事件载体（Sprint 91）
type TorrentEvent struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	SourceID string `json:"source_id" gorm:"size:36;index"`
	SiteName string `json:"site_name" gorm:"size:50;not null;index"`

	TorrentID   string `json:"torrent_id" gorm:"size:50;not null"`
	Title       string `json:"title" gorm:"size:500"`
	DownloadURL string `json:"download_url" gorm:"size:512"`
	Size        int64  `json:"size"`

	InfoHash      string              `json:"info_hash" gorm:"size:40;index"`
	TorrentData   []byte              `json:"torrent_data" gorm:"type:blob"`
	FileTree      []FileInfo          `json:"file_tree" gorm:"type:json;serializer:json"`
	Fingerprint   *ContentFingerprint `json:"fingerprint" gorm:"foreignKey:FingerprintID"`
	FingerprintID *uint               `json:"fingerprint_id" gorm:"index"`

	Discount    DiscountLevel `json:"discount" gorm:"size:20;not null;default:'NONE'"`
	FreeEndAt   *time.Time    `json:"free_end_at"`
	HasHR       bool          `json:"has_hr" gorm:"default:false"`
	HRSeedTimeH int           `json:"hr_seed_time_h" gorm:"default:0"`

	MatchedRuleID   uint   `json:"matched_rule_id"`
	MatchedRuleName string `json:"matched_rule_name" gorm:"size:100"`

	Metadata map[string]any `json:"metadata" gorm:"type:json;serializer:json"`

	RequiresSideLoading bool           `json:"requires_side_loading" gorm:"default:false"`
	SideLoadStatus      SideLoadStatus `json:"side_load_status" gorm:"size:20;default:'not_required'"`
	SideLoadStartedAt   *time.Time     `json:"side_load_started_at"`
	SideLoadFinishedAt  *time.Time     `json:"side_load_finished_at"`
}

func (TorrentEvent) TableName() string { return "torrent_events" }

// FileInfo — 文件树信息节点
type FileInfo struct {
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Priority int    `json:"priority"`
}

// §33.1.39 — RSSTorrentEvent: RSS 引擎简化视图
type RSSTorrentEvent struct {
	SourceRSS     string         `json:"source_rss"`
	SiteName      string         `json:"site_name"`
	TorrentID     string         `json:"torrent_id"`
	Title         string         `json:"title"`
	DownloadURL   string         `json:"download_url"`
	Size          int64          `json:"size"`
	InfoHash      string         `json:"info_hash"`
	IsFree        bool           `json:"is_free"`
	DiscountLevel DiscountLevel  `json:"discount_level"`
	FreeEndAt     *time.Time     `json:"free_end_at"`
	HasHR         bool           `json:"has_hr"`
	HRSeedTimeH   int            `json:"hr_seed_time_h"`
	MatchedRule   *string        `json:"matched_rule"`
	TorrentData   []byte         `json:"-"`
	Metadata      map[string]any `json:"metadata"`
	Category      string         `json:"category"`
	Tags          []string       `json:"tags"`
	Uploader      string         `json:"uploader"`
}

// §33.1.3 — RSSSubscription: 订阅配置（Sprint 89, 7 处合并）
type RSSSubscription struct {
	ID        uint           `json:"id" gorm:"primaryKey;autoIncrement"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`
	Name      string         `json:"name" gorm:"size:100;not null"`
	Enabled   bool           `json:"enabled" gorm:"default:true"`

	URLs     []string `json:"urls" gorm:"type:json;size:2048;serializer:json"`
	SiteName string   `json:"site_name" gorm:"size:50;not null;index"`
	Cron     string   `json:"cron" gorm:"size:100;default:'*/5 * * * *'"`

	AcceptRuleIDs []uint          `json:"accept_rule_ids" gorm:"type:json;serializer:json"`
	RejectRuleIDs []uint          `json:"reject_rule_ids" gorm:"type:json;serializer:json"`
	Conditions    []RuleCondition `json:"conditions" gorm:"type:text;serializer:json"`

	ClientID  string `json:"client_id" gorm:"size:50;index"`
	SavePath  string `json:"save_path" gorm:"size:500"`
	Category  string `json:"category" gorm:"size:100"`
	AddPaused bool   `json:"add_paused" gorm:"default:false"`
	AutoTMM   bool   `json:"auto_tmm" gorm:"default:false"`

	UploadLimitKB   int64 `json:"upload_limit_kb"`
	DownloadLimitKB int64 `json:"download_limit_kb"`

	ScrapeFree bool `json:"scrape_free" gorm:"default:false"`
	ScrapeHR   bool `json:"scrape_hr" gorm:"default:false"`

	SkipSameSize    bool `json:"skip_same_size" gorm:"default:false"`
	AddCountPerHour int  `json:"add_count_per_hour" gorm:"default:0"`

	Tags []string `json:"tags" gorm:"type:json;size:512;serializer:json"`

	UseCustomRegex bool   `json:"use_custom_regex" gorm:"default:false"`
	RegexStr       string `json:"regex_str" gorm:"size:256"`
	ReplaceStr     string `json:"replace_str" gorm:"size:256"`

	AutoReseed      bool     `json:"auto_reseed" gorm:"default:false"`
	ReseedClientIDs []string `json:"reseed_client_ids" gorm:"type:json;serializer:json"`

	PublishEnabled bool     `json:"publish_enabled" gorm:"default:false"`
	PublishTargets []string `json:"publish_targets" gorm:"type:json;serializer:json"`

	PushNotify bool   `json:"push_notify" gorm:"default:false"`
	NotifyID   string `json:"notify_id" gorm:"size:50"`

	FreeWaitEnabled    bool `json:"free_wait_enabled" gorm:"default:false"`
	FreeWaitMaxWaitSec int  `json:"free_wait_max_wait_sec" gorm:"default:7200"`
	FreeWaitRecheckSec int  `json:"free_wait_recheck_sec" gorm:"default:600"`
	FreeWaitMinRemain  int  `json:"free_wait_min_remain" gorm:"default:30"`

	RecheckEnabled   bool `json:"recheck_enabled" gorm:"default:false"`
	RecheckIntervalH int  `json:"recheck_interval_h" gorm:"default:6"`
	RecheckMaxCount  int  `json:"recheck_max_count" gorm:"default:5"`
	RecheckMaxAgeH   int  `json:"recheck_max_age_h" gorm:"default:72"`

	FeasibilityEnabled    bool    `json:"feasibility_enabled" gorm:"default:false"`
	FeasibilitySpeedLimit float64 `json:"feasibility_speed_limit"`
	FeasibilitySizeLimit  float64 `json:"feasibility_size_limit"`
	FeasibilitySafety     float64 `json:"feasibility_safety" gorm:"default:0.8"`

	SkipSameSizeWindowMin int  `json:"skip_same_size_window_min" gorm:"default:30"`
	SkipSameSizeStrict    bool `json:"skip_same_size_strict" gorm:"default:false"`

	DiskBudgetEnabled bool    `json:"disk_budget_enabled" gorm:"default:false"`
	DiskBudgetMinGB   float64 `json:"disk_budget_min_gb" gorm:"default:10"`

	CandidateClients []string            `json:"candidate_clients" gorm:"type:json;serializer:json"`
	ClientSelection  ClientSelectionMode `json:"client_selection" gorm:"size:20;default:'fixed'"`

	ScoringConfig SeedingScoringConfig `json:"scoring_config" gorm:"embedded"`

	DiskGuardEnabled   bool       `json:"disk_guard_enabled" gorm:"default:true"`
	DiskGuardThreshold float64    `json:"disk_guard_threshold" gorm:"default:1073741824"`
	Paused             bool       `json:"paused" gorm:"default:false"`
	PauseReason        string     `json:"pause_reason" gorm:"size:50"`
	PausedAt           *time.Time `json:"paused_at"`

	LifecyclePauseSeeders    int `json:"lifecycle_pause_seeders" gorm:"default:0"`
	LifecycleDeleteSeeders   int `json:"lifecycle_delete_seeders" gorm:"default:0"`
	LifecycleDeleteSeedHours int `json:"lifecycle_delete_seed_hours" gorm:"default:0"`
}

func (RSSSubscription) TableName() string { return "rss_subscriptions" }

// §33.1.7 — SLData: S/L 数据
type SLData struct {
	Seeders   int        `json:"seeders"`
	Leechers  int        `json:"leechers"`
	FreeEndAt *time.Time `json:"free_end_at,omitempty"`
}

// §33.1.8 — PendingScoringEntry: 免费等待→评分管道
// v2: reserved — 待实现时激活
type PendingScoringEntry struct {
	SubscriptionID string        `json:"subscription_id"`
	TorrentID      string        `json:"torrent_id"`
	SiteName       string        `json:"site_name"`
	Discount       DiscountLevel `json:"discount"`
	BecameFreeAt   time.Time     `json:"became_free_at"`
	HasHR          bool          `json:"has_hr"`
	HRSeedTimeH    int           `json:"hr_seed_time_h"`
	FreeEndAt      *time.Time    `json:"free_end_at"`
	Size           int64         `json:"size"`
	Title          string        `json:"title"`
	DownloadURL    string        `json:"download_url"`
	InfoHash       string        `json:"info_hash"`
}

// §33.1.13 — ScoredCandidate: 刷流管道评分增强候选
// v2: reserved — 待实现时激活
type ScoredCandidate struct {
	SubscriptionID    string        `json:"subscription_id"`
	ClientID          string        `json:"client_id"`
	Event             *TorrentEvent `json:"event"`
	Seeders           int           `json:"seeders"`
	Leechers          int           `json:"leechers"`
	FreeEndAt         *time.Time    `json:"free_end_at"`
	Score             float64       `json:"score"`
	Rank              int           `json:"rank"`
	SeedersConfirmed  int           `json:"seeders_confirmed"`
	LeechersConfirmed int           `json:"leechers_confirmed"`
}

// §33.1.44 — RSSTorrentSeen: RSS 种子去重记录
type RSSTorrentSeen struct {
	ID             uint          `json:"id" gorm:"primaryKey;autoIncrement"`
	SiteName       string        `json:"site_name" gorm:"size:100;not null;uniqueIndex:idx_site_torrent,composite:site_name"`
	TorrentID      string        `json:"torrent_id" gorm:"size:100;not null;uniqueIndex:idx_site_torrent,composite:torrent_id"`
	SubscriptionID string        `json:"subscription_id" gorm:"size:100;not null"`
	InfoHash       string        `json:"info_hash" gorm:"size:40;index:idx_torrent_seen_info_hash"`
	IsFakeHash     bool          `json:"is_fake_hash" gorm:"default:false"`
	Title          string        `json:"title" gorm:"size:500"`
	Size           int64         `json:"size"`
	IsFree         bool          `json:"is_free" gorm:"default:false"`
	FreeEndAt      *time.Time    `json:"free_end_at"`
	FreeLevel      string        `json:"free_level" gorm:"size:20"`
	Discount       DiscountLevel `json:"discount" gorm:"size:20;default:'NONE'"`
	HasHR          bool          `json:"has_hr" gorm:"default:false"`
	HRSeedTimeH    int           `json:"hr_seed_time_h"`
	Status         string        `json:"status" gorm:"size:20;not null;default:'seen';index:idx_torrent_seen_status"`
	MatchedRule    string        `json:"matched_rule" gorm:"size:100"`
	SkipCount      int           `json:"skip_count" gorm:"not null;default:0"`
	LastCheckTime  *time.Time    `json:"last_check_time" gorm:"index:idx_torrent_seen_last_check"`
	PushTime       *time.Time    `json:"push_time"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
}

func (RSSTorrentSeen) TableName() string { return "rss_torrent_seen" }

type RSSFetchLog struct {
	ID             uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	SubscriptionID string    `json:"subscription_id" gorm:"size:100;not null;index:idx_fetch_log_sub"`
	Total          int       `json:"total" gorm:"default:0"`
	NewCount       int       `json:"new_count" gorm:"default:0"`
	Dispatched     int       `json:"dispatched" gorm:"default:0"`
	Status         string    `json:"status" gorm:"size:20;not null;default:'ok'"`
	ErrorMsg       string    `json:"error_msg" gorm:"size:500"`
	CreatedAt      time.Time `json:"created_at"`
}

func (RSSFetchLog) TableName() string { return "rss_fetch_logs" }

// §33.1.45 — RSSSubscriptionRule: 订阅-规则关联
type RSSSubscriptionRule struct {
	ID             uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	SubscriptionID string    `json:"subscription_id" gorm:"size:100;not null;uniqueIndex:idx_sub_rule,composite:subscription_id;index"`
	RuleID         uint      `json:"rule_id" gorm:"not null;uniqueIndex:idx_sub_rule,composite:rule_id"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (RSSSubscriptionRule) TableName() string { return "rss_subscription_rules" }
