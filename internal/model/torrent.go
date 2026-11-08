package model

import "time"

// §33.1.22 — TorrentDetail: 种子详情
// MusicMeta §59.466: 音乐种子结构化元数据（获取链 L3 音乐字段抽取承载——
// 皇后新版结构化字段行/海豚 Gazelle musicInfo 等音乐源产出；影视种恒 nil）。
type MusicMeta struct {
	Artist        string   `json:"artist,omitempty"`         // 艺术家名
	Album         string   `json:"album,omitempty"`          // 专辑名称
	Genre         string   `json:"genre,omitempty"`          // 类别/流派
	Tags          []string `json:"tags,omitempty"`           // 标签列表（地区+风格双维）
	Format        string   `json:"format,omitempty"`         // 格式（FLAC/WAV/DFF/…）
	AudioMode     string   `json:"audio_mode,omitempty"`     // 整轨/分轨
	Media         string   `json:"media,omitempty"`          // 媒介（Web/CD/SACD/…）
	Year          string   `json:"year,omitempty"`           // 发行年份
	ShareRule     string   `json:"share_rule,omitempty"`     // 转载规则（欢迎转载…）
	Origin        string   `json:"origin,omitempty"`         // 来自（转载/原创）
	OtherVersions string   `json:"other_versions,omitempty"` // 其它版本（Dupe 站内信号）
	TrackList     string   `json:"track_list,omitempty"`     // 曲目列表（独立通道）
	LogText       string   `json:"log_text,omitempty"`       // log 文本（独立通道）
	// §59.466 附: 海豚 Gazelle 路扩展——API 直读信号（皇后源后续解析 log
	// 文本得分行同填——两源归一消费：发布预检 R2/Dupe 仲裁）。
	LogScore    int    `json:"log_score,omitempty"`    // Log 得分（0-100；CD 抓轨质量核心）
	HasLog      bool   `json:"has_log,omitempty"`      // log 在场
	HasCue      bool   `json:"has_cue,omitempty"`      // cue 在场
	ReleaseType string `json:"release_type,omitempty"` // 发行类型（Album/EP/Single/Compilation…Gazelle 枚举）
	// §59.485: 海豚原创族标记（action=torrentgroup 补采——detail API 盲区）。
	// R7 判定锚：VH∨scene → 拦（原创作品/Scene 发布）；双 false → 转载可转
	// （海豚规则：禁转标记仅原创资源可用——转载种无权禁转）。
	IsVanityHouse bool `json:"is_vanity_house,omitempty"` // 原创作品（自己/授权代表发布）
	IsScene        bool `json:"is_scene,omitempty"`        // Scene 发布
}

type TorrentDetail struct {
	Title string `json:"title"`
	// §59.485: Gazelle 组 ID 透出（海豚 torrentgroup 补采锚——海豹发布
	// 查组挂组同字段语义；NP 站零值不受影响）
	GroupID string `json:"group_id,omitempty"`
	// §59.162 keepfrds 禁转两态采集：列表页行原始文本（含 [禁转]/[ 限时禁转 ] 标记）
	// 与详情页发布时间原文（"由 xx 发布于20时46分前"——相对时间，天档以上=超 24h 窗）
	RawListRow  string     `json:"raw_list_row,omitempty"`
	AddedAtText string     `json:"added_at_text,omitempty"`
	// §59.162: 限时禁转让渡截止（判定收口 fetcher——buildMetadata 传递落库）
	NoTransferUntil *time.Time `json:"no_transfer_until,omitempty"`
	// §59.369 附二: 详情页发布时间（绝对）——NP 新版 details-inline-time 的
	// title 属性（"发布于<span title='2026-10-29 19:35:30'>4时6分钟前</span>"，
	// 深度站实证）；限转 72h 窗等站规判定消费。
	PublishedAt *time.Time `json:"published_at,omitempty"`
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
	Statement    string     `json:"statement"` // §59.26: 源站官组声明（从 Intro.Statement 映射）
	MediaInfo    string     `json:"media_info"`
	BDInfo       string     `json:"bd_info"`
	Size         int64      `json:"size"`
	FileTree     []FileInfo `json:"file_tree"`
	InfoHash     string     `json:"info_hash"`
	// §59.466: 音乐结构化元数据（音乐种非 nil——获取链 L3 产出）
	Music        *MusicMeta `json:"music,omitempty"`
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
	SearchArea string `json:"search_area,omitempty"` // §59.30: NexusPHP search_area（0=标题 1=简介），空=默认 0
	// §59.486: 翻页（NP page 参数 0 起——大合集场景同体积真身在默认序
	// 100+ 位·第一页 50 截断之外·音乐第三轮翻页聚合覆盖）
	Page int `json:"page,omitempty"`
}

// §33.1.74 — PTGenResult: PTGen 查询结果
// §56.16 决策 9: 加 Playdate 字段（◎上映日期）
type PTGenResult struct {
	ChineseTitle string       `json:"chinese_title"`
	// ForeignTitle §59.237 弃用：端点（doubaninfo）foreign_title 实测恒=chinese_title
	// （148 行历史实证+fnos 三样本）——语义陷阱字段（名承诺 foreign 实为中文别名）。
	// 保留仅对接端点字段名，禁止一切消费——译名真数据用 TranslatedTitles。
	ForeignTitle string `json:"foreign_title"`
	// TranslatedTitles §59.237: ◎译名行完整内容（多语言串——豆瓣又名 aka 同源，
	// 从 bbcode 提取——端点结构化字段无此数据）。
	TranslatedTitles string `json:"translated_titles"`
	Year         string       `json:"year"`
	Region       []string     `json:"region"`
	Genre        []string     `json:"genre"`
	Language     []string     `json:"language"`
	Episodes     string       `json:"episodes"`
	Season       string       `json:"season"` // §59.226: ◎季数（裸数字季号消歧门）
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

	// ArrayFields §59.269: 同名多值字段（checkbox 数组 tags[4][] 等——
	// map 无法承载重复键）。GetEditForm 采集已勾选项，SubmitEdit 原样回放。
	ArrayFields []TagKV `json:"array_fields,omitempty"`
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
