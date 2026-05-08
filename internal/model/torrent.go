package model

import "time"

// §33.1.22 — TorrentDetail: 种子详情
type TorrentDetail struct {
	Title        string     `json:"title"`
	Subtitle     string     `json:"subtitle"`
	Description  string     `json:"description"`
	Category     string     `json:"category"`
	Source       string     `json:"source"`
	Resolution   string     `json:"resolution"`
	Codec        string     `json:"codec"`
	ReleaseGroup string     `json:"release_group"`
	Tags         []string   `json:"tags"`
	AudioCodec   string     `json:"audio_codec"`
	Processing   string     `json:"processing"`
	Region       string     `json:"region"`
	NFO          string     `json:"nfo"`
	IMDbID       string     `json:"imdb_id"`
	DoubanID     string     `json:"douban_id"`
	PosterURL    string     `json:"poster_url"`
	Screenshots  []string   `json:"screenshots"`
	MediaInfo    string     `json:"media_info"`
	Size         int64      `json:"size"`
	FileTree     []FileInfo `json:"file_tree"`
	InfoHash     string     `json:"info_hash"`
	UploadTime   time.Time  `json:"upload_time"`
}

// §33.1.23 — DiscountResult: 免费检测结果
type DiscountResult struct {
	Level      DiscountLevel `json:"level"`
	FreeEndAt  *time.Time    `json:"free_end_at"`
	Multiplier float64       `json:"multiplier"`
}

// §33.1.24 — HRResult: HR 检测结果
type HRResult struct {
	HasHR     bool       `json:"has_hr"`
	SeedTimeH int        `json:"hr_seed_time_h"`
	MinRatio  float64    `json:"min_ratio"`
	Deadline  *time.Time `json:"deadline"`
}

// §33.1.36 — RawTorrent: 源站原始种子数据
// v2: reserved — 待实现时激活
type RawTorrent struct {
	SiteName      string    `json:"site_name"`
	TorrentID     string    `json:"torrent_id"`
	InfoHash      string    `json:"info_hash"`
	Title         string    `json:"title"`
	Subtitle      string    `json:"subtitle"`
	RawCategory   string    `json:"raw_category"`
	RawMedium     string    `json:"raw_medium"`
	RawResolution string    `json:"raw_resolution"`
	RawVideoCodec string    `json:"raw_video_codec"`
	RawAudioCodec string    `json:"raw_audio_codec"`
	RawSource     string    `json:"raw_source"`
	RawProcessing string    `json:"raw_processing"`
	RawTeam       string    `json:"raw_team"`
	RawTags       []string  `json:"raw_tags"`
	Description   string    `json:"description"`
	MediaInfo     string    `json:"media_info"`
	Screenshots   []string  `json:"screenshots"`
	Size          int64     `json:"size"`
	DownloadURL   string    `json:"download_url"`
	UploadTime    time.Time `json:"upload_time"`
	RepostControl string    `json:"repost_control"`
	IsOfficial    bool      `json:"is_official"`
	FetchTime     time.Time `json:"fetch_time"`
	LocalSavePath string    `json:"local_save_path"`
	LocalFilePath string    `json:"local_file_path"`
	FileCount     int       `json:"file_count"`
	UploadUser    string    `json:"upload_user"`
}

// §33.1.38 — SearchOptions: 搜索选项
type SearchOptions struct {
	Category   string `json:"category"`
	FreeOnly   bool   `json:"free_only"`
	SortBy     string `json:"sort_by"`
	MaxResults int    `json:"max_results"`
}

// §33.1.74 — PTGenResult: PTGen 查询结果
type PTGenResult struct {
	ChineseTitle string       `json:"chinese_title"`
	ForeignTitle string       `json:"foreign_title"`
	Year         string       `json:"year"`
	Region       []string     `json:"region"`
	Genre        []string     `json:"genre"`
	Language     []string     `json:"language"`
	Episodes     string       `json:"episodes"`
	Duration     string       `json:"duration"`
	Director     []string     `json:"director"`
	Cast         []PersonInfo `json:"cast"`
	Writer       []string     `json:"writer"`
	Introduction string       `json:"introduction"`
	PosterURL    string       `json:"poster_url"`
	DoubanRating string       `json:"douban_rating"`
	DoubanVotes  string       `json:"douban_votes"`
	DoubanURL    string       `json:"douban_url"`
	IMDBRating   string       `json:"imdb_rating"`
	IMDBVotes    string       `json:"imdb_votes"`
	IMDBID       string       `json:"imdb_id"`
	IMDBURL      string       `json:"imdb_url"`
	TMDbURL      string       `json:"tmdb_url"`
	Awards       []string     `json:"awards"`
	AKA          []string     `json:"aka"`
	RawBBCode    string       `json:"raw_bbcode"`
	Source       string       `json:"source"`
	Cached       bool         `json:"cached"`
}

type PersonInfo struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	Character string `json:"character"`
}

// §33.1.75 — DescriptionData: 描述渲染输入
type DescriptionData struct {
	Statement     string   `json:"statement"`
	PosterURL     string   `json:"poster_url"`
	PTGenBody     string   `json:"ptgen_body"`
	MediaInfoText string   `json:"mediainfo_text"`
	BDInfoText    string   `json:"bdinfo_text"`
	Screenshots   []string `json:"screenshots"`
	SourceSite    string   `json:"source_site"`
}

// §33.1.28 — PublishStepResult: 21 步中间产物容器
// v2: reserved — 待实现时激活
type PublishStepResult struct {
	TargetSite     string
	TorrentID      string
	InfoHash       string
	DetailURL      string
	DownloadURL    string
	Description    string
	RewrittenTitle string
	VideoFilePath  string
	ScreenshotURLs []string
	MediaInfoText  string
	PTGenData      *PTGenResult
	TorrentBytes   []byte
	Injected       bool
}

// §33.1.70 — AppError: 统一应用错误
type AppError struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Detail    string `json:"detail"`
	Retryable bool   `json:"retryable"`
	Cause     error  `json:"-"`
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}
func (e *AppError) Unwrap() error { return e.Cause }

// §33.1.86 — RateLimitConfig: 搜索速率控制配置
// v2: reserved — 待实现时激活
type RateLimitConfig struct {
	MaxConcurrency int           `yaml:"max_concurrency"`
	MinInterval    time.Duration `yaml:"min_interval"`
}

type UploadForm struct {
	Fields []UploadFormField `json:"fields"`
}

type UploadFormField struct {
	Name        string   `json:"name"`
	Label       string   `json:"label"`
	Type        string   `json:"type"`
	Required    bool     `json:"required"`
	Options     []string `json:"options,omitempty"`
	Placeholder string   `json:"placeholder,omitempty"`
	Value       string   `json:"value,omitempty"`
}

type EditForm struct {
	TorrentID   string            `json:"torrent_id"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Fields      map[string]string `json:"fields"`
}

type PublishDedupResult struct {
	TorrentID string `json:"torrent_id"`
	Title     string `json:"title"`
	Size      int64  `json:"size"`
	InfoHash  string `json:"info_hash"`
}

type ListOptions struct {
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
	SortBy   string `json:"sort_by"`
	SortDir  string `json:"sort_dir"`
	Search   string `json:"search"`
}
