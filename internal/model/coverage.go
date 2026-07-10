package model

import "time"

const (
	CoverageConfirmedHas    = "confirmed_has"
	CoverageProbablyHas     = "probably_has"
	CoverageConfirmedNot    = "confirmed_not"
	CoverageProbablyNot     = "probably_not"
	CoverageUnknown         = "unknown"

	CoverageSourceTracker    = "tracker"
	CoverageSourcePiecesHash = "pieces_hash"
	CoverageSourceIYUU       = "iyuu"
	CoverageSourceNameSize   = "name_size"
	CoverageSourcePublish    = "publish"
)

type SiteCoverageCache struct {
	ID         uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	InfoHash   string    `gorm:"size:40;not null;uniqueIndex:idx_coverage_hash_site" json:"info_hash"`
	SiteName   string    `gorm:"size:50;not null;uniqueIndex:idx_coverage_hash_site" json:"site_name"`
	Status     string    `gorm:"size:30;not null;index" json:"status"`
	Source     string    `gorm:"size:20;not null" json:"source"`
	Confidence float64   `gorm:"not null;default:0" json:"confidence"`
	TorrentID  string    `gorm:"size:50" json:"torrent_id"`
	DetailURL  string    `gorm:"type:text" json:"detail_url"`
	QueriedAt  time.Time `gorm:"not null" json:"queried_at"`
	ExpiresAt  time.Time `gorm:"not null;index" json:"expires_at"`
}

func (SiteCoverageCache) TableName() string {
	return "site_coverage_cache"
}

type CoverageQueryState struct {
	InfoHash  string    `gorm:"size:40;primaryKey" json:"info_hash"`
	QueriedAt time.Time `gorm:"not null" json:"queried_at"`
	ExpiresAt time.Time `gorm:"not null;index" json:"expires_at"`
}

func (CoverageQueryState) TableName() string {
	return "coverage_query_state"
}
