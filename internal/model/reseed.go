package model

import "time"

// §33.1.33 — ContentFingerprint: 内容指纹统一（12 字段 DB 模型）
type ContentFingerprint struct {
	ID              uint             `json:"id" gorm:"primaryKey;autoIncrement"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
	InfoHash        string           `json:"info_hash" gorm:"size:40;uniqueIndex:idx_site_hash"`
	SiteName        string           `json:"site_name" gorm:"size:50;uniqueIndex:idx_site_hash"`
	TorrentID       string           `json:"torrent_id" gorm:"size:50"`
	PiecesHash      string           `json:"pieces_hash" gorm:"size:40;index"`
	PiecesRoot      string           `json:"pieces_root" gorm:"size:64"`
	TotalSize       int64            `json:"total_size"`
	FileCount       int              `json:"file_count"`
	LargestFileSize int64            `json:"largest_file_size"`
	FileTree        []byte           `json:"file_tree" gorm:"type:blob"`
	FileTreeParsed  map[string]int64 `json:"-" gorm:"-"`
	Title           string           `json:"title" gorm:"size:500"`
	FilesHash       string           `json:"files_hash" gorm:"size:128;index"`
}

func (ContentFingerprint) TableName() string { return "content_fingerprints" }

const (
	ReseedModeSeedFeature = "seed_feature"
	ReseedModeIYUUCloud   = "iyuu_cloud"
)

var ReseedModeDefaults = map[string]string{
	ReseedModeSeedFeature: "pieces_hash,file_tree,size_title,fingerprint",
	ReseedModeIYUUCloud:   "iyuu",
}

func ValidReseedMode(mode string) bool {
	_, ok := ReseedModeDefaults[mode]
	return ok
}

// §33.1.63 — ReseedTask: 辅种任务
type ReseedTask struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	Name    string `json:"name" gorm:"size:200;not null"`
	Enabled bool   `json:"enabled" gorm:"default:false"`

	ClientIDs            string `json:"client_ids" gorm:"type:text"`
	SourceSiteIDs        string `json:"source_site_ids" gorm:"type:text"`
	TargetSiteIDs        string `json:"target_site_ids" gorm:"type:text"`
	TargetSiteExcludes   string `json:"target_site_excludes" gorm:"type:text"`
	ReleaseGroupExcludes string `json:"release_group_excludes" gorm:"type:text"`
	CategoryExcludes     string `json:"category_excludes" gorm:"type:text"`
	TitleKeywordExcludes string `json:"title_keyword_excludes" gorm:"type:text"`

	MatchMethods        string  `json:"match_methods" gorm:"type:text"`
	ConfidenceThreshold float64 `json:"confidence_threshold" gorm:"default:0.7"`
	FallbackEnabled     bool    `json:"fallback_enabled" gorm:"default:true"`
	MaxFallbacks        int     `json:"max_fallbacks" gorm:"default:3"`

	EngineMode string `json:"engine_mode" gorm:"size:20;default:'seed_feature'"`

	SizeTolerancePercent float64 `json:"size_tolerance_percent" gorm:"default:1.0"`

	MaxInjectionsPerRun  int    `json:"max_injections_per_run" gorm:"default:100"`
	InjectionIntervalS   int    `json:"injection_interval_s" gorm:"default:15"`
	InjectionJitterS     int    `json:"injection_jitter_s" gorm:"default:5"`
	InjectionConcurrency int    `json:"injection_concurrency" gorm:"default:3"`
	ScanConcurrency      int    `json:"scan_concurrency" gorm:"default:5"`
	ReseedCategory       string `json:"reseed_category" gorm:"size:100;default:'cross-seed'"`

	Schedule string           `json:"schedule" gorm:"size:100;default:'0 */6 * * *'"`
	Status   ReseedTaskStatus `json:"status" gorm:"size:20;default:'idle'"`
	Version  int              `json:"version" gorm:"default:0"`

	MaxRetries     int `json:"max_retries" gorm:"default:3"`
	RetryIntervalH int `json:"retry_interval_h" gorm:"default:24"`

	LastRunAt *time.Time `json:"last_run_at"`
}

func (ReseedTask) TableName() string { return "reseed_tasks" }

// §33.1.64 — ReseedMatch: 辅种匹配结果
type ReseedMatch struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	ClientID        string `json:"client_id" gorm:"size:50;not null;uniqueIndex:idx_reseed_match_pair"`
	SourceSite      string `json:"source_site" gorm:"size:50;not null;uniqueIndex:idx_reseed_match_pair"`
	SourceTorrentID string `json:"source_torrent_id" gorm:"size:50;not null;uniqueIndex:idx_reseed_match_pair"`
	SourceInfoHash  string `json:"source_info_hash" gorm:"size:40;not null;index"`

	TargetSite      string `json:"target_site" gorm:"size:50;not null;uniqueIndex:idx_reseed_match_pair"`
	TargetTorrentID string `json:"target_torrent_id" gorm:"size:50;not null;uniqueIndex:idx_reseed_match_pair"`
	TargetInfoHash  string `json:"target_info_hash" gorm:"size:40"`

	MatchMethod    string  `json:"match_method" gorm:"size:20;not null"`
	Confidence     float64 `json:"confidence" gorm:"not null"`
	DecisionType   string  `json:"decision_type" gorm:"size:30"`
	DecisionDetail string  `json:"decision_detail" gorm:"size:512"`

	Status      ReseedMatchStatus `json:"status" gorm:"size:20;not null;default:'matched';index"`
	InjectedAt  *time.Time        `json:"injected_at"`
	FailReason  string            `json:"fail_reason" gorm:"size:512"`
	RetryCount  int               `json:"retry_count" gorm:"default:0"`
	NextRetryAt *time.Time        `json:"next_retry_at"`

	CachedTorrentData []byte `json:"-" gorm:"-"`
}

func (ReseedMatch) TableName() string { return "reseed_matches" }

// §33.1.65 — ReseedNegativeCache: 辅种负面清单
type ReseedNegativeCache struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	CreatedAt time.Time `json:"created_at"`

	SourceSite      string `json:"source_site" gorm:"size:50;not null;index"`
	SourceTorrentID string `json:"source_torrent_id" gorm:"size:50;not null"`
	SourceInfoHash  string `json:"source_info_hash" gorm:"size:40;not null;uniqueIndex:idx_neg_cache_source"`

	ExcludedTargets string    `json:"excluded_targets" gorm:"type:text"`
	LastMethod      string    `json:"last_method" gorm:"size:20"`
	LayerDepth      int       `json:"layer_depth" gorm:"default:3"`
	ExpiresAt       time.Time `json:"expires_at" gorm:"index"`
	HitCount        int       `json:"hit_count" gorm:"default:1"`
}

func (ReseedNegativeCache) TableName() string { return "reseed_negative_caches" }

// §33.1.18 — ReseedSource: 辅种过滤参数
// v2: reserved — 待实现时激活
type ReseedSource struct {
	InfoHash string `json:"info_hash" gorm:"size:40;index"`
	Title    string `json:"title" gorm:"size:500"`
	Category string `json:"category" gorm:"size:100"`
	SiteName string `json:"site_name" gorm:"size:50"`
	Size     int64  `json:"size"`
}

// §33.1.19 — IYUUCandidate: IYUU 匹配候选
// v2: reserved — 待实现时激活
type IYUUCandidate struct {
	Sid            int    `json:"sid"`
	Site           string `json:"site" gorm:"size:50"`
	TorrentID      int    `json:"torrent_id"`
	TorrentName    string `json:"torrent_name"`
	InfoHash       string `json:"info_hash" gorm:"size:40"`
	Size           int64  `json:"size"`
	SourceInfoHash string `json:"source_info_hash,omitempty" gorm:"size:40"`
}

// §33.1.20 — ReseedDecision: 辅种决策结果
// v2: reserved — 待实现时激活
type ReseedDecision struct {
	TargetSite  string       `json:"target_site" gorm:"size:50"`
	TorrentID   int          `json:"torrent_id"`
	InfoHash    string       `json:"info_hash" gorm:"size:40"`
	Decision    DecisionType `json:"decision"`
	TorrentData []byte       `json:"-" gorm:"type:blob"`
	Error       error        `json:"-" gorm:"-"`
	Reason      string       `json:"reason" gorm:"size:500"`
}

// §33.1.21 — ReseedExecutionResult: 辅种执行统计
type ReseedExecutionResult struct {
	TaskID          string    `json:"task_id" gorm:"size:50;primaryKey"`
	TotalSources    int       `json:"total_sources"`
	Matched         int       `json:"matched"`
	Injected        int       `json:"injected"`
	Failed          int       `json:"failed"`
	Skipped         int       `json:"skipped"`
	DuplicateExists int       `json:"duplicate_exists"`
	Blocked         int       `json:"blocked"`
	Duration        float64   `json:"duration_seconds"`
	CompletedAt     time.Time `json:"completed_at"`
}

// §33.1.61 — SearchCache: 辅种 L2 搜索结果缓存
type SearchCache struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`

	SiteName   string `json:"site_name" gorm:"size:50;not null;uniqueIndex:idx_search_cache_key"`
	CleanTitle string `json:"clean_title" gorm:"size:512;not null;uniqueIndex:idx_search_cache_key"`
	TotalSize  int64  `json:"total_size" gorm:"not null;uniqueIndex:idx_search_cache_key"`

	Results string `json:"results" gorm:"type:text"`
}

func (SearchCache) TableName() string { return "search_caches" }

// §33.1.66 — Candidate: 辅种候选匹配
type Candidate struct {
	TargetSite      string  `json:"target_site"`
	TargetTorrentID string  `json:"target_torrent_id"`
	TargetInfoHash  string  `json:"target_info_hash"`
	Confidence      float64 `json:"confidence"`
	MatchMethod     string  `json:"match_method"`
}
