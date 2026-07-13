package model

import "time"

// §48.2 — TorrentMetadata: 源种元数据（抓取-存储-映射体系）
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
	FetchedAt         time.Time `json:"fetched_at"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func (TorrentMetadata) TableName() string { return "torrent_metadata" }
