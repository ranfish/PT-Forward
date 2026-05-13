package model

import "time"

// §33.1.4 — Site: 站点模型（C1 消解，ID uint PK）
type Site struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Domain    string    `json:"domain" gorm:"uniqueIndex;size:100;not null"`
	Name      string    `json:"name" gorm:"uniqueIndex;size:50;not null"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	BaseURL   string `json:"base_url" gorm:"size:512;not null"`
	Framework string `json:"framework" gorm:"size:20;not null;index"`
	AuthType  string `json:"auth_type" gorm:"size:20;not null;default:'cookie'"`
	Enabled   bool   `json:"enabled" gorm:"default:true"`

	Passkey     string `json:"passkey,omitempty" gorm:"size:200" encrypted:"true"`
	Cookie      string `json:"cookie,omitempty" gorm:"type:text" encrypted:"true"`
	APIKey      string `json:"api_key,omitempty" gorm:"size:200" encrypted:"true"`
	BearerToken string `json:"bearer_token,omitempty" gorm:"size:200" encrypted:"true"`
	AuthKey     string `json:"auth_key,omitempty" gorm:"size:200" encrypted:"true"`
	AuthHash    string `json:"auth_hash,omitempty" gorm:"size:200" encrypted:"true"`
	UserID      int    `json:"user_id,omitempty" gorm:"default:0"`
	RSSKey      string `json:"rss_key,omitempty" gorm:"size:200" encrypted:"true"`

	HashStrategy     string `json:"hash_strategy" gorm:"size:20;not null;default:'xml_tag'"`
	SizeStrategy     string `json:"size_strategy" gorm:"size:20;not null;default:'xml_tag'"`
	IDStrategy       string `json:"id_strategy" gorm:"size:20;not null;default:'query_param'"`
	IDPattern        string `json:"id_pattern" gorm:"size:128;not null"`
	HashXMLTagName   string `json:"hash_xml_tag_name,omitempty" gorm:"size:64"`
	SizeXMLTagName   string `json:"size_xml_tag_name,omitempty" gorm:"size:64"`
	HashURLParamName string `json:"hash_url_param_name,omitempty" gorm:"size:64"`
	SizeDescRegex    string `json:"size_desc_regex,omitempty" gorm:"size:256"`
	SizeTitleRegex   string `json:"size_title_regex,omitempty" gorm:"size:256"`
	SizeBaseUnit     int    `json:"size_base_unit,omitempty" gorm:"default:0"`

	RequiresSideLoading bool `json:"requires_side_loading" gorm:"default:false"`

	DownloadMode        string `json:"download_mode" gorm:"size:20;default:'template'"`
	DownloadURLTemplate string `json:"download_url_template" gorm:"size:512"`
	DownloadPagePattern string `json:"download_page_pattern" gorm:"size:512"`

	FrameworkDetected bool   `json:"framework_detected" gorm:"default:false"`
	FrameworkVerified bool   `json:"framework_verified" gorm:"default:false"`
	DetectionDetail   string `json:"detection_detail,omitempty" gorm:"size:1024"`

	CookieCloudSync    bool       `json:"cookie_cloud_sync" gorm:"default:false"`
	CookieCloudDomain  string     `json:"cookie_cloud_domain,omitempty" gorm:"size:200"`
	AlternativeDomains string     `json:"alternative_domains,omitempty" gorm:"type:text"`
	MirrorDomain       string     `json:"mirror_domain,omitempty" gorm:"size:200"`
	LastSyncAt         *time.Time `json:"last_sync_at"`

	IsSource               bool `json:"is_source" gorm:"default:false"`
	IsTarget               bool `json:"is_target" gorm:"default:false"`
	ParticipateAutoPublish bool `json:"participate_auto_publish" gorm:"default:true"`

	OverrideRSSURL   string `json:"override_rss_url,omitempty" gorm:"size:500"`
	OverrideSavePath string `json:"override_save_path,omitempty" gorm:"size:500"`
	HRStrategy       string `json:"hr_strategy" gorm:"size:20;not null;default:'protect'"`

	ProxyURL      string `json:"proxy_url,omitempty" gorm:"size:500"`
	SkipSSLVerify bool   `json:"skip_ssl_verify" gorm:"default:false"`

	UploadBytes   int64      `json:"upload_bytes" gorm:"default:0"`
	DownloadBytes int64      `json:"download_bytes" gorm:"default:0"`
	SeedingPoints float64    `json:"seeding_points" gorm:"default:0"`
	SeedingSize   int64      `json:"seeding_size" gorm:"default:0"`
	SeedingCount  int        `json:"seeding_count" gorm:"default:0"`
	UserClass     string     `json:"user_class" gorm:"size:64"`
	Ratio         float64    `json:"ratio" gorm:"default:0"`
	BonusPoints   float64    `json:"bonus_points" gorm:"default:0"`
	StatsSyncedAt *time.Time `json:"stats_synced_at"`
}

func (Site) TableName() string { return "sites" }

// §33.1.59 — SiteConfigOverride: 站点配置字段级用户覆盖（Sprint 89 决策 #232）
type SiteConfigOverride struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	SiteName   string `json:"site_name" gorm:"size:100;not null;uniqueIndex:idx_site_field"`
	FieldPath  string `json:"field_path" gorm:"size:255;not null;uniqueIndex:idx_site_field"`
	FieldValue string `json:"field_value" gorm:"type:text;not null" encrypted:"true"`
	Source     string `json:"source" gorm:"size:20;not null;default:'web_ui'"`
}

func (SiteConfigOverride) TableName() string { return "site_config_overrides" }

// §33.1.48 — SiteFieldMapping: 发布字段值映射
type SiteFieldMapping struct {
	ID          uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	SiteName    string    `json:"site_name" gorm:"size:100;not null;uniqueIndex:idx_field_mapping,composite:site_name"`
	FieldType   string    `json:"field_type" gorm:"size:50;not null;uniqueIndex:idx_field_mapping,composite:field_type"`
	SourceValue string    `json:"source_value" gorm:"size:100;not null;uniqueIndex:idx_field_mapping,composite:source_value"`
	TargetValue string    `json:"target_value" gorm:"size:100;not null"`
	Fallbacks   string    `json:"fallbacks" gorm:"type:text"`
	Priority    int       `json:"priority" gorm:"default:0"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (SiteFieldMapping) TableName() string { return "site_field_mappings" }

// §33.1.37 — SiteInfo: 站点运行时信息
type SiteInfo struct {
	Name      string `json:"name"`
	BaseURL   string `json:"base_url"`
	Framework string `json:"framework"`
	Enabled   bool   `json:"enabled"`

	Passkey     string `json:"passkey,omitempty"`
	Cookie      string `json:"cookie,omitempty"`
	APIKey      string `json:"api_key,omitempty"`
	BearerToken string `json:"bearer_token,omitempty"`
	AuthKey     string `json:"auth_key,omitempty"`
	AuthHash    string `json:"auth_hash,omitempty"`
	UserID      int    `json:"user_id,omitempty"`

	HashStrategy string `json:"hash_strategy"`
	SizeStrategy string `json:"size_strategy"`
	IDStrategy   string `json:"id_strategy"`
	IDPattern    string `json:"id_pattern"`

	RSSKey           string `json:"rss_key,omitempty"`
	HashXMLTagName   string `json:"hash_xml_tag_name"`
	SizeXMLTagName   string `json:"size_xml_tag_name"`
	HashURLParamName string `json:"hash_url_param_name"`
	SizeDescRegex    string `json:"size_desc_regex"`
	SizeTitleRegex   string `json:"size_title_regex"`
	SizeBaseUnit     int    `json:"size_base_unit"`

	DownloadMode        string `json:"download_mode"`
	DownloadURLTemplate string `json:"download_url_template"`
	DownloadPagePattern string `json:"download_page_pattern"`

	ProxyURL      string `json:"proxy_url"`
	SkipSSLVerify bool   `json:"skip_ssl_verify"`
}

// §33.1.60 — SiteDefault: go:embed YAML 默认配置
type SiteDefault struct {
	SiteName          string                      `yaml:"site_name"`
	Domain            string                      `yaml:"domain"`
	Framework         string                      `yaml:"framework"`
	Auth              SiteAuthConfig              `yaml:"auth"`
	Paths             SitePathsConfig             `yaml:"paths"`
	RSS               SiteRSSConfig               `yaml:"rss"`
	Discount          SiteDiscountConfig          `yaml:"discount"`
	DiscountDetection SiteDiscountDetectionConfig `yaml:"discount_detection"`
	HR                SiteHRConfig                `yaml:"hr"`
	SourceParse       SiteSourceParseConfig       `yaml:"source_parse"`
	Publish           SitePublishFullConfig       `yaml:"publish"`
}

type SiteAuthConfig struct {
	DownloadMode   string `yaml:"download_mode"`
	RSSMode        string `yaml:"rss_mode"`
	PasskeyExpires int    `yaml:"passkey_expires"`
	Cookie         string `yaml:"cookie"`
}

type SitePathsConfig struct {
	Browse     string `yaml:"browse"`
	Detail     string `yaml:"detail"`
	Upload     string `yaml:"upload"`
	TakeUpload string `yaml:"takeupload"`
	RSS        string `yaml:"rss"`
}

type SiteRSSConfig struct {
	URLTemplate      string       `yaml:"url_template"`
	HashStrategy     HashStrategy `yaml:"hash_strategy"`
	SizeStrategy     SizeStrategy `yaml:"size_strategy"`
	IDStrategy       IDStrategy   `yaml:"id_strategy"`
	IDPattern        string       `yaml:"id_pattern"`
	HashXMLTagName   string       `yaml:"hash_xml_tag_name,omitempty"`
	SizeXMLTagName   string       `yaml:"size_xml_tag_name,omitempty"`
	HashURLParamName string       `yaml:"hash_url_param_name,omitempty"`
	SizeDescRegex    string       `yaml:"size_desc_regex,omitempty"`
	SizeTitleRegex   string       `yaml:"size_title_regex,omitempty"`
	SizeBaseUnit     int          `yaml:"size_base_unit,omitempty"`
}

type SiteDiscountConfig struct {
	HasAPI    bool     `yaml:"has_api"`
	APIURL    string   `yaml:"api_url"`
	Selectors []string `yaml:"selectors"`
}

type SiteDiscountDetectionConfig struct {
	Enabled              bool              `yaml:"enabled"`
	Mode                 string            `yaml:"mode"`
	APIEndpoint          string            `yaml:"api_endpoint"`
	DiscountIconSelector string            `yaml:"discount_icon_selector"`
	DiscountEndSelector  string            `yaml:"discount_end_selector"`
	HREnabled            bool              `yaml:"hr_enabled"`
	HRIconSelector       string            `yaml:"hr_icon_selector"`
	CacheTTL             int               `yaml:"cache_ttl"`
	PriorityChain        []string          `yaml:"priority_chain"`
	APIFields            map[string]string `yaml:"api_fields"`
	DiscountClassMapping map[string]string `yaml:"discount_class_mapping"`
}

type SiteHRConfig struct {
	DefaultSeedTimeH int      `yaml:"default_seed_time_h"`
	Selectors        []string `yaml:"selectors"`
}

func (h SiteHRConfig) SeedTimeH() int {
	if h.DefaultSeedTimeH > 0 {
		return h.DefaultSeedTimeH
	}
	return 72
}

type SiteSourceParseConfig struct {
	StandardKeys map[string]map[string]string `yaml:"standard_keys"`
	SourceParams map[string]map[string]string `yaml:"source_params"`
}

type SitePublishFullConfig struct {
	UploadURL              string                       `yaml:"upload_url"`
	HasNFOField            bool                         `yaml:"has_nfo_field"`
	Description            SiteDescConfig               `yaml:"description"`
	Links                  map[string]string            `yaml:"links"`
	FormFields             map[string]string            `yaml:"form_fields"`
	Mappings               map[string]map[string]string `yaml:"mappings"`
	SkipCategories         []string                     `yaml:"skip_categories"`
	SkipModes              []string                     `yaml:"skip_modes"`
	SkipTags               []string                     `yaml:"skip_tags"`
	DualMode               *DualModeConfig              `yaml:"dual_mode"`
	UploadSpeedLimit       int                          `yaml:"upload_speed_limit"`
	MaxConsecutiveFailures int                          `yaml:"max_consecutive_failures"`
	ExistingStrategy       string                       `yaml:"existing_strategy"`
	PublishInterval        string                       `yaml:"publish_interval"`
	SupportsPiecesHashAPI  bool                         `yaml:"supports_pieces_hash_api"`
}

type DualModeConfig struct {
	DefaultMode int                       `yaml:"default_mode"`
	Modes       map[int]map[string]string `yaml:"modes"`
	ModeField   string                    `yaml:"mode_field"`
}

// §33.1.69 — SiteDescConfig: 站点描述渲染配置
type SiteDescConfig struct {
	Format           string                `yaml:"format"`
	Sections         []string              `yaml:"sections"`
	MediaInfo        SiteMediaInfoConfig   `yaml:"mediainfo"`
	Screenshots      SiteScreenshotsConfig `yaml:"screenshots"`
	TemplateOverride string                `yaml:"template_override"`
}

type SiteMediaInfoConfig struct {
	Location  string `yaml:"location"`
	Wrapper   string `yaml:"wrapper"`
	HideTitle string `yaml:"hide_title"`
	FieldName string `yaml:"field_name"`
}

type SiteScreenshotsConfig struct {
	Location  string `yaml:"location"`
	FieldName string `yaml:"field_name"`
}

// §33.1.32 — SiteConfig: 运行时配置（适配器使用）
type SiteConfig struct {
	SiteDefault
	Domain          string `json:"domain"`
	Enabled         bool   `json:"enabled"`
	IsSource        bool   `json:"is_source"`
	IsTarget        bool   `json:"is_target"`
	CookieCloudSync bool   `json:"cookie_cloud_sync"`
	HRStrategy      string `json:"hr_strategy"`

	Passkey     string `json:"passkey,omitempty"`
	Cookie      string `json:"cookie,omitempty"`
	APIKey      string `json:"api_key,omitempty"`
	AuthKey     string `json:"auth_key,omitempty"`
	AuthHash    string `json:"auth_hash,omitempty"`
	UserID      int    `json:"user_id,omitempty"`
	RSSKey      string `json:"rss_key,omitempty"`
	BearerToken string `json:"bearer_token,omitempty"`

	ProxyURL      string `json:"proxy_url,omitempty"`
	SkipSSLVerify bool   `json:"skip_ssl_verify"`
}

// §33.1.72 — DetectResult: 框架检测结果
type DetectResult struct {
	Framework       string            `json:"framework"`
	Confidence      float64           `json:"confidence"`
	DetectionDetail string            `json:"detection_detail"`
	Defaults        FrameworkDefaults `json:"defaults"`
}

type FrameworkDefaults struct {
	HashStrategy        string `json:"hash_strategy"`
	SizeStrategy        string `json:"size_strategy"`
	IDStrategy          string `json:"id_strategy"`
	IDPattern           string `json:"id_pattern"`
	DownloadURLTemplate string `json:"download_url_template"`
}

// §33.1.47 — PublishExclusion: 发布源站互斥
type PublishExclusion struct {
	ID         uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	TargetSite string    `json:"target_site" gorm:"size:100;not null;uniqueIndex:idx_exclusion,composite:target_site"`
	SourceSite string    `json:"source_site" gorm:"size:100;not null;uniqueIndex:idx_exclusion,composite:source_site"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (PublishExclusion) TableName() string { return "publish_exclusions" }
