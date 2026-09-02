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
	HasHR     bool          `json:"has_hr" gorm:"default:false"`

	DownloadCompleted bool                   `json:"download_completed" gorm:"default:false"`
	CompletedAt       *time.Time             `json:"completed_at"`
	PublishStatus     PublishCandidateStatus `json:"publish_status" gorm:"size:20;default:'pending'"`
	PublishResult     string                 `json:"publish_result" gorm:"type:text"`
	SkipReason        string                 `json:"skip_reason" gorm:"size:200"`
	UserOverrides     string                 `json:"user_overrides" gorm:"type:text"`
	Role              PublishCandidateRole   `json:"role" gorm:"size:20;default:'download'"`
	RetryCount        int                    `json:"retry_count" gorm:"default:0"`
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
	RetryCount        int `json:"retry_count" gorm:"default:0"`
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
	TargetSite  string `json:"target_site" gorm:"size:100;index"`
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
	// §59.166 dedup 本地记忆（站方 pieces-hash API 瞬时空返回实战打穿——18:49 批
	// 4 种重传；发布成功/拦截均落，dedup 先查本地零依赖站方）
	PiecesHash string `json:"pieces_hash" gorm:"size:40;index"`
	// §59.159: 源种 hash（记录溯源）
	SourceInfoHash string            `json:"source_info_hash" gorm:"size:40;index"`
	// §59.159: 加种目标路径（发布时落库——补推纯记录回放，不依赖快照行存活）
	SavePath       string            `json:"save_path" gorm:"size:500"`
	PublishURL   string              `json:"publish_url" gorm:"size:500"`
	ErrorMessage string              `json:"error_message" gorm:"size:500"`
	CompletedAt  *time.Time          `json:"completed_at"`

	Trigger      string `json:"trigger" gorm:"size:20"`       // manual/batch/reseed/wizard
	BatchGroupID string `json:"batch_group_id" gorm:"size:36;index"` // 批量发布 UUID
	Title        string `json:"title" gorm:"size:500"`
	Subtitle     string `json:"subtitle" gorm:"size:500"`
	DownloaderID string `json:"downloader_id" gorm:"size:50"`
	CostMS       int64  `json:"cost_ms" gorm:"default:0"`
	Logs         string `json:"logs" gorm:"type:text"`

	// §56.30: 加种回下载器
	Seeded    bool       `json:"seeded" gorm:"default:false"`      // 是否已加种
	SeededAt  *time.Time `json:"seeded_at"`                         // 加种时间
	SeedError string     `json:"seed_error" gorm:"size:500"`        // 加种失败原因
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
	// §59.156: checkbox 数组标签（tags[4][] 同名字段多次重复——multipart 数组语义；
	// map 无法承载重复键，NUL 拼接 adapter 不可拆）
	TagArrayFields []TagKV           `json:"tag_array_fields,omitempty"`
	TagConfig       string            `json:"tag_config,omitempty"` // §56.22: 站点 tag 配置 JSON（SiteTagConfig）
	Title           string            `json:"title"`
	Subtitle        string            `json:"subtitle"`
	Description     string            `json:"description"`
	MediaInfo       string            `json:"media_info"`
	BDInfo          string            `json:"bd_info"`
	Screenshots     []string          `json:"screenshots"`
	ScreenshotInDesc bool             `json:"screenshot_in_desc"`
	IMDbLink        string            `json:"imdb_link"`
	DoubanLink      string            `json:"douban_link"`
	Anonymous       bool              `json:"anonymous"`
	ExtraFields     map[string]string `json:"extra_fields"`
	SourceSite      string            `json:"source_site"`
	SourceInfoHash  string            `json:"source_info_hash"`
	SourceTorrentID string            `json:"source_torrent_id"`
	ClientID        string            `json:"client_id"`
	GroupID         uint              `json:"group_group_id"`
	TargetSite      string            `json:"target_site"`
}

// TagKV 有序表单键值对（同名字段重复语义）。
type TagKV struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// §33.1.10 — PublishResponse: 站点上传响应
type PublishResponse struct {
	Success      bool   `json:"success"`
	TorrentID    string `json:"torrent_id"`
	DetailURL    string `json:"detail_url"`
	IsExisting   bool   `json:"is_existing"`
	ExistingID   string `json:"existing_id"`
	// §59.159 IDSource: TorrentID 来源——"redirect"=成功页 302 最终 URL（权威，
	// 加种免检）/"body"=响应文本提取（推荐位风险，name 校验兜底）。用户定案：
	// 主防线是下载源正确（成功页下载必然正确），校验仅 body 来源兜底。
	IDSource     string `json:"id_source,omitempty"`
	ErrorMessage string `json:"error_message"`
	TargetSite   string `json:"target_site"`
	InfoHash     string `json:"info_hash"`
}

// §46 — SitePublishLimit: 站点发布限制配置
type SitePublishLimit struct {
	ID          uint   `json:"id" gorm:"primaryKey;autoIncrement"`
	SiteName    string `json:"site_name" gorm:"uniqueIndex;size:100"`
	Enabled     bool   `json:"enabled" gorm:"default:false"`
	MaxCount    int    `json:"max_count" gorm:"default:20"`
	WindowHours int    `json:"window_hours" gorm:"default:24"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (SitePublishLimit) TableName() string { return "site_publish_limits" }
