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
	Flags        []string   `json:"flags"`
	AudioCodec   string     `json:"audio_codec"`
	Processing   string     `json:"processing"`
	Region       string     `json:"region"`
	NFO          string     `json:"nfo"`
	IMDbID       string     `json:"imdb_id"`
	DoubanID     string     `json:"douban_id"`
	IMDbURL      string     `json:"imdb_url"`
	DoubanURL    string     `json:"douban_url"`
	TMDbURL      string     `json:"tmdb_url"`
	PosterURL    string     `json:"poster_url"`
	Screenshots  []string   `json:"screenshots"`
	MediaInfo    string     `json:"media_info"`
	BDInfo       string     `json:"bd_info"`
	Size         int64      `json:"size"`
	FileTree     []FileInfo `json:"file_tree"`
	InfoHash     string     `json:"info_hash"`
	UploadTime   time.Time  `json:"upload_time"`

	// RawHTML 原始详情页 HTML（§56.13 接线：暴露给 Engine + PublicExtractor）。
	// 仅 NexusPHP/Unit3D 等 HTML 框架的 adapter 填充，MTeam API 模式留空。
	RawHTML string `json:"raw_html,omitempty"`

	// §56.13 方案 B：Engine 提取的元信息（adapter 内部调 Engine 后填）
	// SourceRegion 产地（如"欧美"），与 Source（语义=媒介）区分
	SourceRegion string `json:"source_region,omitempty"`
	// EngineExtractorName Engine 用的 extractor 名（如 "pterclub_special"），用于诊断
	EngineExtractorName string `json:"engine_extractor_name,omitempty"`
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
// §56.16 决策 9: 加 Playdate 字段（◎上映日期）
type PTGenResult struct {
	ChineseTitle string       `json:"chinese_title"`
	ForeignTitle string       `json:"foreign_title"`
	Year         string       `json:"year"`
	Region       []string     `json:"region"`
	Genre        []string     `json:"genre"`
	Language     []string     `json:"language"`
	Episodes     string       `json:"episodes"`
	Duration     string       `json:"duration"`
	Playdate     string       `json:"playdate"` // §56.16 决策 9: ◎上映日期
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
// §56.16 决策 5: 加 PTGen 结构化字段（替代 PTGenBody 单字段，PTGenBody 保留向后兼容）
type DescriptionData struct {
	Statement     string        `json:"statement"`
	PosterURL     string        `json:"poster_url"`
	PTGenBody     string        `json:"ptgen_body"`        // deprecated: 用 PTGen 替代，保留兼容
	PTGen         *PTGenResult  `json:"ptgen,omitempty"`   // §56.16: 完整结构化 PTGen
	MediaInfoText string        `json:"mediainfo_text"`
	BDInfoText    string        `json:"bdinfo_text"`
	Screenshots   []string      `json:"screenshots"`
	SourceSite    string        `json:"source_site"`
	Title         string        `json:"title"`             // §59.20: 种子标题（用于提取制作组名生成致谢）
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

// §56.23 扩展: 加 Category + ExistingDesc 字段
type EditForm struct {
	TorrentID    string            `json:"torrent_id"`
	Title        string            `json:"title"`
	Description  string            `json:"description"`
	Fields       map[string]string `json:"fields"`
	Category     string            `json:"category,omitempty"`      // §56.23: 现有分类
	ExistingDesc string            `json:"existing_desc,omitempty"` // §56.23: 现有描述（与 Description 同义，冗余但清晰）
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
