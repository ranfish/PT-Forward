package orphan

import "time"

type Entry struct {
	Path       string    `json:"path"`
	Name       string    `json:"name"`
	Size       int64     `json:"size"`
	IsDir      bool      `json:"is_dir"`
	ClientID   string    `json:"client_id"`
	SavePath   string    `json:"save_path"`
	DetectedAt time.Time `json:"detected_at"`
}

type RecoverResult struct {
	Orphan   *Entry  `json:"orphan"`
	Found    bool    `json:"found"`
	Method   string  `json:"method"`
	SiteName string  `json:"site_name"`
	Message  string  `json:"message"`
}
