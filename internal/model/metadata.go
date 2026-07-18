package model

import "time"

// §48.2 — TorrentMetadata: 源种元数据（抓取-存储-映射体系）
// §56.8 扩展：加 4 个 JSON 列（三源原始 + 合并结果）+ LastMergeMode
type TorrentMetadata struct {
	ID                uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	InfoHash          string    `json:"info_hash" gorm:"uniqueIndex:idx_hash_site;size:40"`
	SiteName          string    `json:"site_name" gorm:"uniqueIndex:idx_hash_site;size:100"`
	TorrentID         string    `json:"torrent_id" gorm:"size:50"`
	Title             string    `json:"title" gorm:"size:500"`
	Subtitle          string    `json:"subtitle" gorm:"size:500"`
	SourceCategory    string    `json:"source_category" gorm:"size:100"`
	StandardType      string    `json:"standard_type" gorm:"size:50"`
	Tags              string    `json:"tags" gorm:"type:text"`
	Flags             string    `json:"flags" gorm:"type:text"`
	SourceDescription string    `json:"source_description" gorm:"type:text"`
	Description       string    `json:"description" gorm:"type:text"`
	Screenshots       string    `json:"screenshots" gorm:"type:text"`
	Poster            string    `json:"poster" gorm:"size:500"`
	MediaInfo         string    `json:"mediainfo" gorm:"type:text"`
	SourceMediaInfo   string    `json:"source_mediainfo" gorm:"type:text"`
	MediaInfoSource   string    `json:"mediainfo_source" gorm:"size:20;default:''"`
	IMDbURL           string    `json:"imdb_url" gorm:"size:200"`
	DoubanURL         string    `json:"douban_url" gorm:"size:200"`
	TMDbURL           string    `json:"tmdb_url" gorm:"size:200"`
	FetchSource       string    `json:"fetch_source" gorm:"size:20;default:''"`
	Reviewed          bool      `json:"reviewed" gorm:"default:false"`
	FetchedAt         time.Time `json:"fetched_at"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`

	// §56.8 三源 JSON 列（详情页/PTGen/本地产物原始结果）
	DetailSourceJSON string `json:"detail_source_json" gorm:"column:detail_source_json;type:TEXT"`
	PTGenSourceJSON  string `json:"ptgen_source_json"  gorm:"column:ptgen_source_json;type:TEXT"`
	LocalSourceJSON  string `json:"local_source_json"  gorm:"column:local_source_json;type:TEXT"`

	// §56.8 合并结果 JSON（前端直接读"上次合并结果"，toggle 切换时合并一次写回）
	MergedJSON string `json:"merged_json" gorm:"column:merged_json;type:TEXT"`

	// §56.8 合并模式记录（UI toggle 显示）
	LastMergeMode string `json:"last_merge_mode" gorm:"column:last_merge_mode;size:20;default:'ptgen_first'"`
}

func (TorrentMetadata) TableName() string { return "torrent_metadata" }
