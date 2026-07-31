package orphan

import "time"

type Entry struct {
	Path       string    `json:"path"`
	Name       string    `json:"name"`
	Size       int64     `json:"size"`
	IsDir      bool      `json:"is_dir"`
	ClientIDs  []string  `json:"client_ids"`
	SavePath   string    `json:"save_path"`
	DetectedAt time.Time `json:"detected_at"`
}

type RecoverResult struct {
	Orphan      *Entry       `json:"orphan"`
	Found       bool         `json:"found"`
	Method      string       `json:"method"`
	SiteName    string       `json:"site_name"`
	Message     string       `json:"message"`
	SearchStats *SearchStats `json:"search_stats,omitempty"`
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
