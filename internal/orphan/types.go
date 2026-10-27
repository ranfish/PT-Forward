package orphan

import "time"

type Entry struct {
	Path       string    `json:"path"`
	Name       string    `json:"name"`
	Size       int64     `json:"size"`
	IsDir      bool      `json:"is_dir"`
	ClientUIDs []uint    `json:"client_uids"`
	SavePath   string    `json:"save_path"`
	DetectedAt time.Time `json:"detected_at"`
	// §59.359: 跨目录做种标记——记录表命中（下载器 infohash 仲裁持久化）。
	// SeededPaths 带做种条目路径供用户核对（防同名巧合误导）。
	CrossSeeded bool     `json:"cross_seeded"`
	SeededPaths []string `json:"seeded_paths,omitempty"`
}

type RecoverResult struct {
	Orphan         *Entry       `json:"orphan"`
	Found          bool         `json:"found"`
	Method         string       `json:"method"`
	SiteName       string       `json:"site_name"`
	Message        string       `json:"message"`
	SearchStats    *SearchStats `json:"search_stats,omitempty"`
	RecoveredCount int          `json:"recovered_count"` // 恢复的种子总数（含同站扩展）
	CrossSeeded    bool         `json:"cross_seeded"`    // §59.359: 已在下载器其它目录做种（infohash 仲裁短路）——冗余副本，删否用户定
}

type SearchStats struct {
	TotalSites  int           `json:"total_sites"`
	Searched    int           `json:"searched"`
	Skipped     int           `json:"skipped"`
	FailedSites []SiteFailure `json:"failed_sites,omitempty"`
}

type SiteFailure struct {
	Site   string `json:"site"`
	Reason string `json:"reason"`
}
