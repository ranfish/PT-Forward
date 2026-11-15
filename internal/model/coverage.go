package model

import "time"

const (
	CoverageConfirmedHas    = "confirmed_has"
	CoverageProbablyHas     = "probably_has"
	CoverageConfirmedNot    = "confirmed_not"
	CoverageProbablyNot     = "probably_not"
	CoverageUnknown         = "unknown"

	CoverageSourceTracker    = "tracker"
	CoverageSourceDownload   = "download" // §59.196: 恢复链下载成功——原发页确定性证据（高于 tracker 存在性）
	CoverageSourceFetch      = "fetch"    // §59.535 刀4: 获取链采集成功——真 torrent 数据铁证（conf 1.0）
	CoverageSourcePiecesHash = "pieces_hash"
	CoverageSourceIYUU       = "iyuu"
	CoverageSourceNameSize   = "name_size"
	CoverageSourcePublish    = "publish"
)

type SiteCoverageCache struct {
	ID uint `gorm:"primaryKey;autoIncrement" json:"id"`
	// TargetInfoHash §59.535: 目标站的 info_hash（IYUU target 返回）——辅种注入
	// 阶段 Candidate.TargetInfoHash 依赖（下载目标站种子定位）。存量行 NULL 语义
	// =早期未记·消费方按 (site, tid) 下载 fallback。
	TargetInfoHash string    `gorm:"size:40" json:"target_info_hash"`
	InfoHash       string    `gorm:"size:40;not null;uniqueIndex:idx_coverage_hash_site" json:"info_hash"`
	SiteName       string    `gorm:"size:50;not null;uniqueIndex:idx_coverage_hash_site" json:"site_name"`
	Status         string    `gorm:"size:30;not null;index" json:"status"`
	Source         string    `gorm:"size:20;not null" json:"source"`
	Confidence     float64   `gorm:"not null;default:0" json:"confidence"`
	TorrentID      string    `gorm:"size:50" json:"torrent_id"`
	DetailURL      string    `gorm:"type:text" json:"detail_url"`
	QueriedAt      time.Time `gorm:"not null" json:"queried_at"`
	ExpiresAt      time.Time `gorm:"not null;index" json:"expires_at"`
}

// CoverageHit 覆盖查询结果（辅种引擎返回给覆盖查询的单条命中·§59.535 共享层
// 复用并扩 TargetInfoHash——目标站 info_hash·辅种注入 Candidate.TargetInfoHash 依赖）。
type CoverageHit struct {
	SiteName       string `json:"site_name"`
	TorrentID      string `json:"torrent_id"`
	Source         string `json:"source"` // "iyuu" / "pieces_hash"
	TargetInfoHash string `json:"target_info_hash,omitempty"`
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
