package model

import "time"

// §33.1.14 — PublishCandidate: 待发布候选（决策 #214）
type PublishCandidate struct {
	ID              uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	SubscriptionID  string    `json:"subscription_id" gorm:"size:36;index"`
	SourceSite      string    `json:"source_site" gorm:"size:100"`
	SourceTorrentID string    `json:"source_torrent_id" gorm:"size:50"`
	InfoHash        string    `json:"info_hash" gorm:"index;size:40"`
	TorrentName     string    `json:"torrent_name" gorm:"size:500"`
	Size            int64     `json:"size"`
	ClientID        string    `json:"client_id" gorm:"size:50"`
	SourceClientID  string    `json:"source_client_id" gorm:"size:50"`

	LocalSavePath string `json:"local_save_path" gorm:"size:500"`
	LocalFilePath string `json:"local_file_path" gorm:"size:500"`

	TargetSites string `json:"target_sites" gorm:"type:text"`

	Discount  DiscountLevel `json:"discount" gorm:"size:20;default:'NONE'"`
	FreeEndAt *time.Time    `json:"free_end_at"`

	DownloadCompleted bool                   `json:"download_completed" gorm:"default:false"`
	CompletedAt       *time.Time             `json:"completed_at"`
	PublishStatus     PublishCandidateStatus `json:"publish_status" gorm:"size:20;default:'pending'"`
	PublishResult     string                 `json:"publish_result" gorm:"type:text"`
	SkipReason        string                 `json:"skip_reason" gorm:"size:200"`
	UserOverrides     string                 `json:"user_overrides" gorm:"type:text"`
	Role              PublishCandidateRole   `json:"role" gorm:"size:20;default:'download'"`
}

func (PublishCandidate) TableName() string { return "publish_candidates" }

// §33.1.52 — PublishGroup: 发布组
type PublishGroup struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	CandidateID     uint   `json:"candidate_id" gorm:"index"`
	SourceHash      string `json:"source_hash" gorm:"index;size:40"`
	SourceSite      string `json:"source_site" gorm:"size:100"`
	SourceTorrentID string `json:"source_torrent_id" gorm:"size:50"`
	SubscriptionID  string `json:"subscription_id" gorm:"index;size:36"`

	Status        PublishGroupStatus `json:"status" gorm:"size:20;default:'active'"`
	LastError     string             `json:"last_error" gorm:"size:500"`
	SeedStartTime *time.Time         `json:"seed_start_time"`
}

func (PublishGroup) TableName() string { return "publish_groups" }

// §33.1.53 — PublishGroupMember: 发布组成员
type PublishGroupMember struct {
	ID             uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	PublishGroupID uint      `json:"publish_group_id" gorm:"uniqueIndex:idx_group_site;not null"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`

	InfoHash  string `json:"info_hash" gorm:"index;size:40"`
	SiteName  string `json:"site_name" gorm:"uniqueIndex:idx_group_site;size:100"`
	TorrentID string `json:"torrent_id" gorm:"size:50"`
	Role      string `json:"role" gorm:"size:20"`
	ClientID  string `json:"client_id" gorm:"size:50"`
	Size      int64  `json:"size" gorm:"default:0"`
	SavePath  string `json:"save_path" gorm:"size:500"`

	Status   MemberStatus `json:"status" gorm:"size:20;default:'new'"`
	StatusAt *time.Time   `json:"status_at"`
	Paused   bool         `json:"paused" gorm:"default:false"`

	Seeders  int `json:"seeders" gorm:"default:0"`
	Leechers int `json:"leechers" gorm:"default:0"`

	HRProtected    bool       `json:"hr_protected" gorm:"default:false"`
	HRMinSeedHours int        `json:"hr_min_seed_hours" gorm:"default:0"`
	HRMinRatio     float64    `json:"hr_min_ratio" gorm:"default:0"`
	HRSeedStart    *time.Time `json:"hr_seed_start"`
	HRSite         string     `json:"hr_site" gorm:"size:100"`
	HRReleased     bool       `json:"hr_released" gorm:"default:false"`

	LastError  string     `json:"last_error" gorm:"size:500"`
	ErrorAt    *time.Time `json:"error_at"`
	IsBanned   bool       `json:"is_banned" gorm:"default:false"`
	BannedAt   *time.Time `json:"banned_at"`
	IsReported bool       `json:"is_reported" gorm:"default:false"`
	ReportedAt *time.Time `json:"reported_at"`

	LastCompletedStep int `json:"last_completed_step" gorm:"default:0"`
}

func (PublishGroupMember) TableName() string { return "publish_group_members" }

// §33.1.54 — PublishGroupStatusHistory: 发布组状态变更历史
type PublishGroupStatusHistory struct {
	ID             uint         `json:"id" gorm:"primaryKey;autoIncrement"`
	PublishGroupID uint         `json:"publish_group_id" gorm:"index;not null"`
	MemberHash     string       `json:"member_hash" gorm:"index;size:40"`
	OldStatus      MemberStatus `json:"old_status" gorm:"size:20"`
	NewStatus      MemberStatus `json:"new_status" gorm:"size:20"`
	Reason         string       `json:"reason" gorm:"size:500"`
	CreatedAt      time.Time    `json:"created_at"`
}

func (PublishGroupStatusHistory) TableName() string { return "publish_group_status_histories" }

// §33.1.10b — PublishResultRecord: 发布结果记录
type PublishResultRecord struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	CandidateID uint   `json:"candidate_id" gorm:"index"`
	SourceSite  string `json:"source_site" gorm:"size:100"`
	TargetSite  string `json:"target_site" gorm:"size:100"`
	TorrentID   string `json:"torrent_id" gorm:"size:50"`

	IsOfficial        bool   `json:"is_official"`
	HasForbid         bool   `json:"has_forbid"`
	HasExclusive      bool   `json:"has_exclusive"`
	HRDetected        bool   `json:"hr_detected"`
	SizeOutOfRange    bool   `json:"size_out_of_range"`
	CrossSiteExcluded bool   `json:"cross_site_excluded"`
	TeamDetected      string `json:"team_detected" gorm:"size:50"`

	Status       PublishResultStatus `json:"status" gorm:"size:20"`
	SkipReason   string              `json:"skip_reason" gorm:"size:200"`
	PublishURL   string              `json:"publish_url" gorm:"size:500"`
	ErrorMessage string              `json:"error_message" gorm:"size:500"`
	CompletedAt  *time.Time          `json:"completed_at"`
}

func (PublishResultRecord) TableName() string { return "publish_result_records" }

// §33.1.82 — PublishTask: 发布任务
type PublishTask struct {
	ID           uint              `json:"id" gorm:"primaryKey;autoIncrement"`
	Type         PublishTaskType   `json:"type" gorm:"size:20;default:'manual'"`
	SourceSiteID uint              `json:"source_site_id" gorm:"not null;index"`
	TargetSites  []string          `json:"target_sites" gorm:"type:json;serializer:json"`
	ManualCheck  bool              `json:"manual_check" gorm:"default:true"`
	CheckedAt    *time.Time        `json:"checked_at" gorm:"default:null"`
	Status       PublishTaskStatus `json:"status" gorm:"size:20;default:'pending'"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

func (PublishTask) TableName() string { return "publish_tasks" }

// §33.1.9 — PublishRequest: 发种请求
type PublishRequest struct {
	TorrentData     []byte            `json:"-"`
	FormFields      map[string]string `json:"form_fields"`
	TagFields       map[string]string `json:"tag_fields"`
	Title           string            `json:"title"`
	Subtitle        string            `json:"subtitle"`
	Description     string            `json:"description"`
	MediaInfo       string            `json:"media_info"`
	BDInfo          string            `json:"bd_info"`
	Screenshots     []string          `json:"screenshots"`
	IMDbLink        string            `json:"imdb_link"`
	DoubanLink      string            `json:"douban_link"`
	Anonymous       bool              `json:"anonymous"`
	ExtraFields     map[string]string `json:"extra_fields"`
	SourceSite      string            `json:"source_site"`
	SourceInfoHash  string            `json:"source_info_hash"`
	SourceTorrentID string            `json:"source_torrent_id"`
	ClientID        string            `json:"client_id"`
	GroupID         uint              `json:"group_id"`
	TargetSite      string            `json:"target_site"`
}

// §33.1.10 — PublishResponse: 站点上传响应
type PublishResponse struct {
	Success      bool   `json:"success"`
	TorrentID    string `json:"torrent_id"`
	DetailURL    string `json:"detail_url"`
	IsExisting   bool   `json:"is_existing"`
	ExistingID   string `json:"existing_id"`
	ErrorMessage string `json:"error_message"`
	TargetSite   string `json:"target_site"`
	InfoHash     string `json:"info_hash"`
}

// §33.1.77 — PublishDedupResult: 发布去重搜索结果
type PublishDedupResult struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Size  int64  `json:"size"`
	URL   string `json:"url"`
}

// §33.1.73 — EditForm: 发布编辑表单
type EditForm struct {
	ExistingDesc string `json:"existing_desc"`
	Category     string `json:"category"`
	Title        string `json:"title"`
}

// §33.1.26 — UploadForm: 发布表单
type UploadForm struct {
	Categories   []FormOption `json:"categories"`
	Sources      []FormOption `json:"sources"`
	Resolutions  []FormOption `json:"resolutions"`
	Codecs       []FormOption `json:"codecs"`
	Teams        []FormOption `json:"teams"`
	CustomFields []FormField  `json:"custom_fields"`
}

type FormOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type FormField struct {
	Name     string       `json:"name"`
	Label    string       `json:"label"`
	Type     string       `json:"type"`
	Required bool         `json:"required"`
	Options  []FormOption `json:"options,omitempty"`
}

// §33.1.27 — GlobalPushConfig: 全局推送配置
type GlobalPushConfig struct {
	DefaultCategory      string   `json:"default_category"`
	DefaultTags          []string `json:"default_tags"`
	DefaultPaused        bool     `json:"default_paused"`
	DefaultUploadLimit   int64    `json:"default_upload_limit"`
	DefaultDownloadLimit int64    `json:"default_download_limit"`
	MinFreeGB            float64  `json:"min_free_gb"`
	MaxConcurrentPushes  int      `json:"max_concurrent_pushes"`
}
