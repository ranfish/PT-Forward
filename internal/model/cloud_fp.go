package model

type CloudFPConfig struct {
	ID                uint   `gorm:"primarykey" json:"id"`
	BaseURL           string `gorm:"size:128;not null" json:"base_url"`
	APIToken          string `gorm:"size:128;not null" encrypted:"true" json:"api_token"`
	Enabled           bool   `gorm:"default:false" json:"enabled"`
	RequestTimeoutSec int    `gorm:"default:10" json:"request_timeout_sec"`
}

func (CloudFPConfig) TableName() string {
	return "cloud_fp_configs"
}

type CloudFPMatch struct {
	SiteName  string `json:"site_name"`
	TorrentID string `json:"torrent_id"`
	TotalSize int64  `json:"total_size"`
}

type CloudFPDeleteReport struct {
	SiteName  string `json:"site_name"`
	TorrentID string `json:"torrent_id"`
}

type CloudFPContribute struct {
	PiecesHash string `json:"pieces_hash"`
	SiteName   string `json:"site_name"`
	TorrentID  string `json:"torrent_id"`
	TotalSize  int64  `json:"total_size"`
}
