package model

import "time"

// §33.1.41 — ClientConfig: 下载器配置（Sprint 78 DB-2）
type ClientConfig struct {
	ID             uint       `json:"id" gorm:"primaryKey;autoIncrement"`
	Name           string     `json:"name" gorm:"size:100;not null;uniqueIndex"`
	Type           string     `json:"type" gorm:"size:50;not null"`
	URL            string     `json:"url" gorm:"size:500;not null"`
	Username       string     `json:"username" gorm:"size:100"`
	Password       string     `json:"password" gorm:"size:100"`
	Config         string     `json:"config" gorm:"type:text"`
	Enabled        bool       `json:"enabled" gorm:"default:true"`
	IsDefault      bool       `json:"is_default" gorm:"default:false"`
	Role           string     `json:"role" gorm:"size:20;default:'seeding'"`
	ReseedTargetID string     `json:"reseed_target_id,omitempty" gorm:"size:50"`
	LastPingAt     *time.Time `json:"last_ping_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	DeletedAt      time.Time  `json:"deleted_at" gorm:"index"`
}

func (ClientConfig) TableName() string { return "clients" }

// §33.1.42 — ClientPathMapping: 客户端路径映射
type ClientPathMapping struct {
	ID             uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	SourceClientID uint      `json:"source_client_id" gorm:"not null;index:idx_path_mapping,composite:source_client_id"`
	ReseedClientID uint      `json:"reseed_client_id" gorm:"not null;index:idx_path_mapping,composite:reseed_client_id"`
	SourcePath     string    `json:"source_path" gorm:"size:512;not null;index:idx_path_mapping,composite:source_path"`
	ReseedPath     string    `json:"reseed_path" gorm:"size:512;not null"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (ClientPathMapping) TableName() string { return "client_path_mappings" }

// §33.1.43 — ClientPublishTarget: 客户端发布目标配置
type ClientPublishTarget struct {
	ID              uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	ClientID        uint      `json:"client_id" gorm:"not null;uniqueIndex:idx_client_site"`
	SiteName        string    `json:"site_name" gorm:"size:100;not null;uniqueIndex:idx_client_site"`
	CategoryMapping string    `json:"category_mapping" gorm:"type:text"`
	SourceMapping   string    `json:"source_mapping" gorm:"type:text"`
	CodecMapping    string    `json:"codec_mapping" gorm:"type:text"`
	AutoPublish     bool      `json:"auto_publish" gorm:"default:false"`
	NotifyOnPublish bool      `json:"notify_on_publish" gorm:"default:true"`
	Enabled         bool      `json:"enabled" gorm:"default:true"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (ClientPublishTarget) TableName() string { return "client_publish_targets" }

// §33.1.34 — TorrentInfo: 下载器种子信息
type TorrentInfo struct {
	Hash          string    `json:"hash"`
	Name          string    `json:"name"`
	IsFinished    bool      `json:"is_finished"`
	IsPaused      bool      `json:"is_paused"`
	Removed       bool      `json:"removed"`
	State         string    `json:"state"`
	Error         string    `json:"error"`
	NumComplete   int       `json:"num_complete"`
	NumIncomplete int       `json:"num_incomplete"`
	Ratio         float64   `json:"ratio"`
	SavePath      string    `json:"save_path"`
	Tags          []string  `json:"tags"`
	TotalSize     int64     `json:"total_size"`
	Category      string    `json:"category"`
	Progress      float64   `json:"progress"`
	Uploaded      int64     `json:"uploaded"`
	UploadSpeed   int64     `json:"upload_speed"`
	DownloadSpeed int64     `json:"download_speed"`
	SeedTime      int64     `json:"seed_time"`
	AddedAt       time.Time `json:"added_at"`
}

// §33.1.35 — AddTorrentOptions: 种子添加选项
type AddTorrentOptions struct {
	SavePath         string   `json:"save_path"`
	Category         string   `json:"category"`
	Tags             []string `json:"tags"`
	Paused           bool     `json:"paused"`
	UploadLimit      int64    `json:"upload_limit"`
	DownloadLimit    int64    `json:"download_limit"`
	SkipChecking     bool     `json:"skip_checking"`
	AutoTMM          bool     `json:"auto_tmm"`
	RatioLimit       float64  `json:"ratio_limit"`
	SeedingTimeLimit int      `json:"seeding_time_limit"`
}

// §33.1.12 — AddResult: 下载器添加种子返回值
type AddResult struct {
	InfoHash    string `json:"info_hash"`
	Name        string `json:"name"`
	IsDuplicate bool   `json:"is_duplicate"`
}

// §33.1.25 — Maindata: 下载器全局状态
type Maindata struct {
	Torrents    map[string]TorrentInfo `json:"torrents"`
	ServerState ServerState            `json:"server_state"`
	CategoryMap map[string]string      `json:"categories"`
	Tags        []string               `json:"tags"`
	FreeSpace   int64                  `json:"free_space"`
}

type ServerState struct {
	UploadSpeed int64 `json:"upload_speed"`
}

// §33.1.68 — SharedPathMapping: 共享路径映射
type SharedPathMapping struct {
	SourcePath string `json:"source_path"`
	ReseedPath string `json:"reseed_path"`
}
