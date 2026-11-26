package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/httpclient"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/site"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type SiteHandler struct {
	repo           *site.Repository
	db             *gorm.DB
	logger         *zap.Logger
	provider       SiteProvider
	statsSync      *site.StatsSyncService
	sourceDetector SourceDetectorHook
}

type SourceDetectorHook interface {
	HasGroupMappings(ctx context.Context, site *model.Site) bool
}

type SiteProvider interface {
	GetAdapter(ctx context.Context, domain string) (model.SiteAdapter, error)
	GetSiteConfig(ctx context.Context, domain string) (*model.SiteConfig, error)
	// InvalidateAdapter §59.367 附二: 站点配置变更后失效 adapter 缓存
	// （doer 构造参数 proxy/skip_ssl/tls_pretend 持旧值问题）。
	InvalidateAdapter(keys ...string)
}

func (h *SiteHandler) SetProvider(p SiteProvider) {
	h.provider = p
}

func (h *SiteHandler) SetSourceDetector(s SourceDetectorHook) {
	h.sourceDetector = s
}

func (h *SiteHandler) SetStatsSync(s *site.StatsSyncService) {
	h.statsSync = s
}

func NewSiteHandler(repo *site.Repository, logger *zap.Logger, db *gorm.DB) *SiteHandler {
	return &SiteHandler{repo: repo, db: db, logger: logger}
}

func applySiteMaxConcurrent(domain string, maxConcurrent int) {
	if maxConcurrent > 0 {
		// Default limiter is 30 reqs/60s for maxConcurrent=2 (15 reqs per concurrent
		// slot per minute). Scale MaxReqs proportionally so high-concurrency sites
		// (e.g. TTG with max_concurrent=20 for batch RSS detect) don't get queued
		// behind the default rate window.
		// 12 刀1（v0.0.1654）: 删手拼 "https://"+domain——DomainRateLimiter
		// 入口统一 OriginKey 规范化（F-01 第三套口径收编）。
		const (
			defaultMaxConcurrent = 2
			defaultMaxReqs       = 30
			windowSecs           = 60
		)
		maxReqs := defaultMaxReqs * maxConcurrent / defaultMaxConcurrent
		httpclient.GlobalLimiter.SetDomainConfig(domain, httpclient.DomainLimitConfig{
			MaxConcurrent: maxConcurrent,
			MaxReqs:       maxReqs,
			WindowSecs:    windowSecs,
		})
	}
}

type updateSiteRequest struct {
	Name      *string `json:"name,omitempty"`
	Domain    *string `json:"domain,omitempty"`
	BaseURL   *string `json:"baseUrl,omitempty"`
	Framework *string `json:"framework,omitempty"`
	AuthType  *string `json:"authType,omitempty"`

	Passkey     *string `json:"passkey,omitempty"`
	Cookie      *string `json:"cookie,omitempty"`
	APIKey      *string `json:"apiKey,omitempty"`
	BearerToken *string `json:"bearerToken,omitempty"`
	AuthKey     *string `json:"authKey,omitempty"`
	AuthHash    *string `json:"authHash,omitempty"`
	UserID      *int    `json:"userId,omitempty"`
	RSSKey      *string `json:"rssKey,omitempty"`
	UploadAuth  *string `json:"uploadAuth,omitempty"` // §59.501

	HashStrategy     *string `json:"hashStrategy,omitempty"`
	SizeStrategy     *string `json:"sizeStrategy,omitempty"`
	IDStrategy       *string `json:"idStrategy,omitempty"`
	IDPattern        *string `json:"idPattern,omitempty"`
	HashXMLTagName   *string `json:"hashXmlTagName,omitempty"`
	SizeXMLTagName   *string `json:"sizeXmlTagName,omitempty"`
	HashURLParamName *string `json:"hashUrlParamName,omitempty"`
	SizeDescRegex    *string `json:"sizeDescRegex,omitempty"`
	SizeTitleRegex   *string `json:"sizeTitleRegex,omitempty"`
	SizeBaseUnit     *int    `json:"sizeBaseUnit,omitempty"`

	DownloadMode        *string `json:"downloadMode,omitempty"`
	DownloadURLTemplate *string `json:"downloadUrlTemplate,omitempty"`
	DetailsURLTemplate  *string `json:"detailsUrlTemplate,omitempty"`
	DownloadPagePattern *string `json:"downloadPagePattern,omitempty"`

	RequiresSideLoading *bool `json:"requiresSideLoading,omitempty"`

	IsSource               *bool `json:"isSource,omitempty"`
	IsTarget               *bool `json:"isTarget,omitempty"`
	ParticipateAutoPublish *bool `json:"participateAutoPublish,omitempty"`
	AssumeFree             *bool `json:"assumeFree,omitempty"`

	CookieCloudSync        *bool   `json:"cookieCloudSync,omitempty"`
	CookieCloudDomain      *string `json:"cookieCloudDomain,omitempty"`
	AlternativeDomains     *string `json:"alternativeDomains,omitempty"`
	Enabled                *bool   `json:"enabled,omitempty"`
	PublishIntervalSeconds *int    `json:"publishIntervalSeconds,omitempty"` // §59.166 一站多种批量发布种间间隔（1-60 秒）

	SupportsPiecesHashAPI *bool `json:"supportsPiecesHashApi,omitempty"`

	OverrideRSSURL   *string `json:"overrideRssUrl,omitempty"`
	OverrideSavePath *string `json:"overrideSavePath,omitempty"`

	ProxyURL       *string `json:"proxyUrl,omitempty"`
	UseGlobalProxy *bool   `json:"useGlobalProxy,omitempty"`
	SkipSSLVerify  *bool   `json:"skipSslVerify,omitempty"`
	MaxConcurrent  *int    `json:"maxConcurrent,omitempty"`
	// §59.336: 浏览器 fallback（详情页 HTTP 失败时内置 Chromium 兜底）
	BrowserFallback *bool `json:"browserFallback,omitempty"`
	// §59.183: 站点级 .torrent 下载限流（次/小时，0=全局默认 95）
	DownloadHourlyLimit *int `json:"downloadHourlyLimit,omitempty"`

	HRStrategy          *string `json:"hrStrategy,omitempty"`
	IsReseedTarget      *bool   `json:"isReseedTarget,omitempty"`
	ReseedLimitCount    *int    `json:"reseedLimitCount,omitempty"`
	ReseedLimitInterval *int    `json:"reseedLimitInterval,omitempty"`
	IYUULimitCount      *int    `json:"iyuuLimitCount,omitempty"`
	IYUULimitInterval   *int    `json:"iyuuLimitInterval,omitempty"`
}

type updateCredentialsRequest struct {
	Passkey     *string `json:"passkey,omitempty"`
	Cookie      *string `json:"cookie,omitempty"`
	APIKey      *string `json:"apiKey,omitempty"`
	BearerToken *string `json:"bearerToken,omitempty"`
	AuthKey     *string `json:"authKey,omitempty"`
	AuthHash    *string `json:"authHash,omitempty"`
	UserID      *int    `json:"userId,omitempty"`
	RSSKey      *string `json:"rssKey,omitempty"`
	UploadAuth  *string `json:"uploadAuth,omitempty"` // §59.501 临时上传 Basic Auth
}

type siteResponse struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	Domain    string    `json:"domain"`
	BaseURL   string    `json:"baseUrl"`
	Framework string    `json:"framework"`
	AuthType  string    `json:"authType"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	IsSource               bool `json:"isSource"`
	IsTarget               bool `json:"isTarget"`
	IsReseedTarget         bool `json:"isReseedTarget"`
	PublishSupported       bool `json:"publishSupported"`
	ParticipateAutoPublish bool `json:"participateAutoPublish"`
	AssumeFree             bool `json:"assumeFree"`

	HashStrategy     string `json:"hashStrategy"`
	SizeStrategy     string `json:"sizeStrategy"`
	IDStrategy       string `json:"idStrategy"`
	IDPattern        string `json:"idPattern"`
	HashXMLTagName   string `json:"hashXmlTagName,omitempty"`
	SizeXMLTagName   string `json:"sizeXmlTagName,omitempty"`
	HashURLParamName string `json:"hashUrlParamName,omitempty"`
	SizeDescRegex    string `json:"sizeDescRegex,omitempty"`
	SizeTitleRegex   string `json:"sizeTitleRegex,omitempty"`
	SizeBaseUnit     int    `json:"sizeBaseUnit,omitempty"`

	DownloadMode        string `json:"downloadMode"`
	DownloadURLTemplate string `json:"downloadUrlTemplate,omitempty"`
	DetailsURLTemplate  string `json:"detailsUrlTemplate,omitempty"`
	DownloadPagePattern string `json:"downloadPagePattern,omitempty"`
	RequiresSideLoading bool   `json:"requiresSideLoading"`

	CookieCloudSync   bool       `json:"cookieCloudSync"`
	CookieCloudDomain string     `json:"cookieCloudDomain,omitempty"`
	LastSyncAt        *time.Time `json:"lastSyncAt,omitempty"`

	AlternativeDomains string `json:"alternativeDomains,omitempty"`

	OverrideRSSURL   string `json:"overrideRssUrl,omitempty"`
	OverrideSavePath string `json:"overrideSavePath,omitempty"`

	ProxyURL            string `json:"proxyUrl,omitempty"`
	UseGlobalProxy      bool   `json:"useGlobalProxy"`
	SkipSSLVerify       bool   `json:"skipSslVerify"`
	BrowserFallback     bool   `json:"browserFallback"` // §59.336
	MaxConcurrent       int    `json:"maxConcurrent"`
	DownloadHourlyLimit int    `json:"downloadHourlyLimit"`

	HRStrategy          string `json:"hrStrategy,omitempty"`
	TargetTypes         string `json:"targetTypes,omitempty"`
	ReseedLimitCount    int    `json:"reseedLimitCount"`
	ReseedLimitInterval int    `json:"reseedLimitInterval"`
	IYUULimitCount      int    `json:"iyuuLimitCount"`
	IYUULimitInterval   int    `json:"iyuuLimitInterval"`

	HasPasskey     bool   `json:"hasPasskey"`
	PasskeyMasked  string `json:"passkeyMasked,omitempty"`
	PasskeyAlias   string `json:"passkeyAlias,omitempty"`
	PasskeyHint    string `json:"passkeyHint,omitempty"`
	HasCookie      bool   `json:"hasCookie"`
	CookieMasked   string `json:"cookieMasked,omitempty"`
	HasAPIKey      bool   `json:"hasApiKey"`
	HasUploadAuth  bool   `json:"hasUploadAuth"` // §59.501
	APIKeyMasked   string `json:"apiKeyMasked,omitempty"`
	HasBearerToken bool   `json:"hasBearerToken"`
	HasAuthKey     bool   `json:"hasAuthKey"`
	AuthKeyMasked  string `json:"authKeyMasked,omitempty"`
	HasAuthHash    bool   `json:"hasAuthHash"`
	HasRSSKey      bool   `json:"hasRssKey"`
	RSSKeyMasked   string `json:"rssKeyMasked,omitempty"`

	UserID int `json:"userId,omitempty"`

	SupportsPiecesHashAPI bool `json:"supportsPiecesHashApi"`

	UploadBytes   int64      `json:"uploadBytes,string"`
	DownloadBytes int64      `json:"downloadBytes,string"`
	SeedingPoints float64    `json:"seedingPoints"`
	SeedingSize   int64      `json:"seedingSize,string"`
	SeedingCount  int        `json:"seedingCount"`
	Username      string     `json:"username,omitempty"`
	UserClass     string     `json:"userClass,omitempty"`
	Ratio         float64    `json:"ratio"`
	BonusPoints   float64    `json:"bonusPoints"`
	StatsSyncedAt *time.Time `json:"statsSyncedAt,omitempty"`

	FrameworkDetected bool   `json:"frameworkDetected"`
	FrameworkVerified bool   `json:"frameworkVerified"`
	DetectionDetail   string `json:"detectionDetail,omitempty"`
}

func maskPasskey(pk string) string {
	if len(pk) <= 8 {
		if pk == "" {
			return ""
		}
		return "****"
	}
	return pk[:4] + "****" + pk[len(pk)-4:]
}

func maskCookie(c string) string {
	if c == "" {
		return ""
	}
	var names []string
	for _, part := range strings.Split(c, ";") {
		p := strings.TrimSpace(part)
		if eq := strings.Index(p, "="); eq > 0 {
			names = append(names, p[:eq])
		}
		if len(names) >= 3 {
			break
		}
	}
	summary := strings.Join(names, ", ")
	if len(names) >= 3 {
		summary += ", ..."
	}
	return fmt.Sprintf("已配置（%d 字节，含 %s）", len(c), summary)
}

func (h *SiteHandler) toResponse(s *model.Site) siteResponse {
	resp := siteResponse{
		ID:        s.ID,
		Name:      s.Name,
		Domain:    s.Domain,
		BaseURL:   s.BaseURL,
		Framework: s.Framework,
		AuthType:  s.AuthType,
		Enabled:   s.Enabled,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,

		IsSource:               s.IsSource,
		IsTarget:               s.IsTarget,
		IsReseedTarget:         s.IsReseedTarget,
		PublishSupported:       site.IsPublishSupported(s.Domain),
		ParticipateAutoPublish: s.ParticipateAutoPublish,
		AssumeFree:             s.AssumeFree,

		HashStrategy:     s.HashStrategy,
		SizeStrategy:     s.SizeStrategy,
		IDStrategy:       s.IDStrategy,
		IDPattern:        s.IDPattern,
		HashXMLTagName:   s.HashXMLTagName,
		SizeXMLTagName:   s.SizeXMLTagName,
		HashURLParamName: s.HashURLParamName,
		SizeDescRegex:    s.SizeDescRegex,
		SizeTitleRegex:   s.SizeTitleRegex,
		SizeBaseUnit:     s.SizeBaseUnit,

		DownloadMode:        s.DownloadMode,
		DownloadURLTemplate: s.DownloadURLTemplate,
		DetailsURLTemplate:  s.DetailsURLTemplate,
		DownloadPagePattern: s.DownloadPagePattern,
		RequiresSideLoading: s.RequiresSideLoading,

		CookieCloudSync:   s.CookieCloudSync,
		CookieCloudDomain: s.CookieCloudDomain,
		LastSyncAt:        s.LastSyncAt,

		AlternativeDomains: s.AlternativeDomains,

		OverrideRSSURL:   s.OverrideRSSURL,
		OverrideSavePath: s.OverrideSavePath,

		ProxyURL:            s.ProxyURL,
		UseGlobalProxy:      s.UseGlobalProxy,
		SkipSSLVerify:       s.SkipSSLVerify,
		BrowserFallback:     s.BrowserFallback, // §59.336
		MaxConcurrent:       s.MaxConcurrent,
		DownloadHourlyLimit: s.DownloadHourlyLimit,

		HRStrategy:          s.HRStrategy,
		ReseedLimitCount:    s.ReseedLimitCount,
		ReseedLimitInterval: s.ReseedLimitInterval,
		IYUULimitCount:      s.IYUULimitCount,
		IYUULimitInterval:   s.IYUULimitInterval,

		HasPasskey:     s.Passkey != "",
		PasskeyMasked:  maskPasskey(s.Passkey),
		HasCookie:      s.Cookie != "",
		CookieMasked:   maskCookie(s.Cookie),
		HasAPIKey:      s.APIKey != "",
		HasUploadAuth:  s.UploadAuth != "",
		APIKeyMasked:   maskPasskey(s.APIKey),
		HasBearerToken: s.BearerToken != "",
		HasAuthKey:     s.AuthKey != "",
		AuthKeyMasked:  maskPasskey(s.AuthKey),
		HasAuthHash:    s.AuthHash != "",
		HasRSSKey:      s.RSSKey != "",
		RSSKeyMasked:   maskPasskey(s.RSSKey),

		UserID: s.UserID,

		SupportsPiecesHashAPI: s.SupportsPiecesHashAPI,

		UploadBytes:   s.UploadBytes,
		DownloadBytes: s.DownloadBytes,
		SeedingPoints: s.SeedingPoints,
		SeedingSize:   s.SeedingSize,
		SeedingCount:  s.SeedingCount,
		Username:      s.Username,
		UserClass:     s.UserClass,
		Ratio:         s.Ratio,
		BonusPoints:   s.BonusPoints,
		StatsSyncedAt: s.StatsSyncedAt,

		FrameworkDetected: s.FrameworkDetected,
		FrameworkVerified: s.FrameworkVerified,
		DetectionDetail:   s.DetectionDetail,
	}

	// passkey_label / passkey_hint 从 supported_sites seed 读取（替代 site_config_overrides 表）
	// 这是 §30.6 严格白名单设计：seed 是唯一真相源，UI 与 adapter 字段映射一致
	if seed, ok := site.GetSupportedSite(s.Domain); ok {
		resp.PasskeyAlias = seed.PasskeyLabel
		resp.PasskeyHint = seed.PasskeyHint
	}

	return resp
}

func (h *SiteHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.handleRouteByPath(w, r)
}

func (h *SiteHandler) handleRouteByPath(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	trimmed := strings.TrimRight(path, "/")

	// §59.318 N1 收权：站点全集由开发者随版本管理（sites.json 种子+migration），
	// 用户无创建/删除权——POST（创建）与 DELETE（删除）端点收回，仅留
	// GET/PUT（启用/不启用/配置）。新站=版本更新后种子落"未启用"列表。
	if trimmed == "/api/v1/sites" {
		if r.Method == http.MethodGet {
			h.handleList(w, r)
			return
		}
		methodNotAllowed(w)
		return
	}

	remaining := strings.TrimPrefix(trimmed, "/api/v1/sites/")
	if remaining == "" || remaining == "/" {
		if r.Method == http.MethodGet {
			h.handleList(w, r)
		} else {
			methodNotAllowed(w)
		}
		return
	}

	remaining = strings.TrimPrefix(path, "/api/v1/sites/")
	remaining = strings.TrimRight(remaining, "/")
	parts := strings.SplitN(remaining, "/", 3)

	if remaining == "stats-sync" {
		if r.Method == http.MethodPost {
			h.handleSyncAllStats(w, r)
		} else {
			methodNotAllowed(w)
		}
		return
	}

	if remaining == "batch-update" {
		if r.Method == http.MethodPost {
			h.handleBatchUpdate(w, r)
		} else {
			methodNotAllowed(w)
		}
		return
	}

	if remaining == "batch-sync" {
		if r.Method == http.MethodPost {
			h.handleBatchSyncSiteStats(w, r)
		} else {
			methodNotAllowed(w)
		}
		return
	}

	if remaining == "export" {
		if r.Method == http.MethodGet {
			h.handleExport(w, r)
		} else {
			methodNotAllowed(w)
		}
		return
	}

	if remaining == "import" {
		if r.Method == http.MethodPost {
			h.handleImport(w, r)
		} else {
			methodNotAllowed(w)
		}
		return
	}

	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			h.handleGet(w, r)
		case http.MethodPut:
			h.handleUpdate(w, r)
		default:
			methodNotAllowed(w)
		}
		return
	}

	idStr := parts[0]
	subResource := parts[1]

	switch subResource {
	case "test":
		h.handleTest(w, r, idStr)
	case "detect":
		h.handleDetect(w, r, idStr)
	case "search":
		h.handleSearch(w, r, idStr)
	case "discount":
		h.handleDetectDiscount(w, r, idStr)
	case "credentials":
		if r.Method == http.MethodPut {
			h.handleUpdateCredentials(w, r, idStr)
		} else {
			methodNotAllowed(w)
		}
	case "stats":
		switch r.Method {
		case http.MethodGet:
			h.handleGetStats(w, r, idStr)
		case http.MethodPost:
			h.handleSyncSiteStats(w, r, idStr)
		default:
			methodNotAllowed(w)
		}
	case "overrides":
		h.handleOverrides(w, r, idStr)
	case "freeze":
		h.handleDomainFreezeByID(w, r, idStr)
	case "download-test":
		h.handleDownloadTest(w, r, idStr)
	default:
		Error(w, http.StatusNotFound, 40400, "路径不存在")
	}
}

type siteExportImport struct {
	Domain string `json:"domain"`
	Name   string `json:"name"`

	Passkey     string `json:"passkey"`
	APIKey      string `json:"api_key"`
	BearerToken string `json:"bearer_token"`
	AuthKey     string `json:"auth_key"`
	AuthHash    string `json:"auth_hash"`
	UserID      int    `json:"user_id"`
	RSSKey      string `json:"rss_key"`

	BaseURL  string `json:"base_url"`
	AuthType string `json:"auth_type"`
	Enabled  bool   `json:"enabled"`

	CookieCloudSync   bool   `json:"cookie_cloud_sync"`
	CookieCloudDomain string `json:"cookie_cloud_domain"`

	IsSource               bool `json:"is_source"`
	IsTarget               bool `json:"is_target"`
	IsReseedTarget         bool `json:"is_reseed_target"`
	ParticipateAutoPublish bool `json:"participate_auto_publish"`

	HRStrategy       string `json:"hr_strategy"`
	OverrideRSSURL   string `json:"override_rss_url"`
	OverrideSavePath string `json:"override_save_path"`

	AssumeFree bool `json:"assume_free"`

	ProxyURL       string `json:"proxy_url"`
	UseGlobalProxy bool   `json:"use_global_proxy"`
	SkipSSLVerify  bool   `json:"skip_ssl_verify"`
	MaxConcurrent  int    `json:"max_concurrent"`
	// §59.183: 站点级 .torrent 下载限流
	DownloadHourlyLimit int `json:"download_hourly_limit"`

	ReseedLimitCount    int `json:"reseed_limit_count"`
	ReseedLimitInterval int `json:"reseed_limit_interval"`
	IYUULimitCount      int `json:"iyuu_limit_count"`
	IYUULimitInterval   int `json:"iyuu_limit_interval"`

	AlternativeDomains    string `json:"alternative_domains"`
	TrackerDomains        string `json:"tracker_domains"`
	SupportsPiecesHashAPI bool   `json:"supports_pieces_hash_api"`
	APIDomain             string `json:"api_domain"`
	PublishFormConfig     string `json:"publish_form_config,omitempty"`
}

var siteImportFields = []string{
	"passkey", "api_key", "bearer_token", "auth_key", "auth_hash", "user_id", "rss_key",
	"upload_auth",
	"base_url", "auth_type", "enabled",
	"cookie_cloud_sync", "cookie_cloud_domain",
	"is_source", "is_target", "is_reseed_target", "participate_auto_publish",
	"hr_strategy", "override_rss_url", "override_save_path",
	"assume_free",
	"proxy_url", "use_global_proxy", "skip_ssl_verify", "max_concurrent", "publish_interval_seconds", "download_hourly_limit",
	"reseed_limit_count", "reseed_limit_interval", "iyuu_limit_count", "iyuu_limit_interval", "publish_form_config",
	"alternative_domains", "tracker_domains", "supports_pieces_hash_api", "api_domain",
}

type siteImportResult struct {
	Total    int      `json:"total"`
	Updated  int      `json:"updated"`
	Skipped  []string `json:"skipped"`
	Warnings []string `json:"warnings"`
	Errors   []string `json:"errors"`
}

func (h *SiteHandler) handleExport(w http.ResponseWriter, r *http.Request) {
	var sites []model.Site
	if err := h.db.WithContext(r.Context()).Where("enabled = ?", true).Find(&sites).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询站点失败")
		return
	}
	out := make([]siteExportImport, 0, len(sites))
	for i := range sites {
		s := &sites[i]
		// §59.512: 已下线站点不导出（与导入过滤双侧防线——历史潜伏幽灵
		// 行即使 enabled=1 也不随文件扩散）
		if _, ok := site.GetSupportedSite(s.Domain); !ok {
			continue
		}
		out = append(out, siteExportImport{
			Domain: s.Domain, Name: s.Name,
			Passkey:                s.Passkey,
			APIKey:                 s.APIKey,
			BearerToken:            s.BearerToken,
			AuthKey:                s.AuthKey,
			AuthHash:               s.AuthHash,
			UserID:                 s.UserID,
			RSSKey:                 s.RSSKey,
			BaseURL:                s.BaseURL,
			AuthType:               s.AuthType,
			Enabled:                s.Enabled,
			CookieCloudSync:        s.CookieCloudSync,
			CookieCloudDomain:      s.CookieCloudDomain,
			IsSource:               s.IsSource,
			IsTarget:               s.IsTarget,
			IsReseedTarget:         s.IsReseedTarget,
			ParticipateAutoPublish: s.ParticipateAutoPublish,
			HRStrategy:             s.HRStrategy,
			OverrideRSSURL:         s.OverrideRSSURL,
			OverrideSavePath:       s.OverrideSavePath,
			AssumeFree:             s.AssumeFree,
			ProxyURL:               s.ProxyURL,
			UseGlobalProxy:         s.UseGlobalProxy,
			SkipSSLVerify:          s.SkipSSLVerify,
			MaxConcurrent:          s.MaxConcurrent,
			DownloadHourlyLimit:    s.DownloadHourlyLimit,
			ReseedLimitCount:       s.ReseedLimitCount,
			ReseedLimitInterval:    s.ReseedLimitInterval,
			IYUULimitCount:         s.IYUULimitCount,
			IYUULimitInterval:      s.IYUULimitInterval,
			AlternativeDomains:     s.AlternativeDomains,
			TrackerDomains:         s.TrackerDomains,
			SupportsPiecesHashAPI:  s.SupportsPiecesHashAPI,
			APIDomain:              s.APIDomain,

			PublishFormConfig: s.PublishFormConfig,
		})
	}
	w.Header().Set("Content-Disposition", `attachment; filename="pt-forward-sites-export.json"`)
	Success(w, map[string]interface{}{
		"version":     1,
		"exported_at": time.Now().Format(time.RFC3339),
		"warning":     "本文件含明文凭证（passkey/apikey 等），请妥善保管，勿传云盘/入 git",
		"sites":       out,
	})
}

func (h *SiteHandler) handleImport(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Sites []siteExportImport `json:"sites"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	result := siteImportResult{Skipped: []string{}, Warnings: []string{}, Errors: []string{}}
	for i := range req.Sites {
		in := &req.Sites[i]
		result.Total++
		if in.Domain == "" {
			result.Errors = append(result.Errors, "缺少 domain")
			continue
		}
		// §59.512: 已下线站点过滤（用户报 Case：历史导出文件导入后下线站
		// 复活——243 pt.cdy.skin 实证：幽灵行潜伏 enabled=0+导入整行替换
		// 刷回 1）。supported_sites.json 不认识的 domain 一律拒入——连
		// "更新 DB 已潜伏的幽灵行"也不允许（防续命；幽灵清理另行走删除）。
		if _, ok := site.GetSupportedSite(in.Domain); !ok {
			result.Skipped = append(result.Skipped, in.Domain)
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s: 已下线站点（不在支持集）——已过滤，导入跳过", in.Domain))
			continue
		}
		var existing model.Site
		if err := h.db.WithContext(r.Context()).Where("domain = ?", in.Domain).First(&existing).Error; err != nil {
			result.Skipped = append(result.Skipped, in.Domain)
			continue
		}
		// §59.318 D7: 导入降级——未适配发布站携带 is_target=true 时降为 false
		// 记警告（§59.318 附四：预期内降级归 warnings，不硬拒也不算失败）
		if in.IsTarget && !site.IsPublishSupported(existing.Domain) {
			in.IsTarget = false
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s: 未适配发布，is_target 已降级为 false", in.Domain))
		}
		// §59.318 附六: 值先并入 existing 再 Updates(&existing)（指针）——GORM
		// 加密 callback 只加密 Statement.Model 的 encrypted 字段：Updates(in)
		// 的 SET 取自无 tag 的 siteExportImport（明文直落库）；Updates(existing)
		// 按值传参则 SET 用回调前拷贝，同样绕过（Save/指针形态实证加密生效）。
		existing.Passkey = in.Passkey
		existing.APIKey = in.APIKey
		existing.BearerToken = in.BearerToken
		existing.AuthKey = in.AuthKey
		existing.AuthHash = in.AuthHash
		existing.UserID = in.UserID
		existing.RSSKey = in.RSSKey
		existing.BaseURL = in.BaseURL
		existing.AuthType = in.AuthType
		existing.Enabled = in.Enabled
		existing.CookieCloudSync = in.CookieCloudSync
		existing.CookieCloudDomain = in.CookieCloudDomain
		existing.IsSource = in.IsSource
		existing.IsTarget = in.IsTarget
		existing.IsReseedTarget = in.IsReseedTarget
		existing.ParticipateAutoPublish = in.ParticipateAutoPublish
		existing.HRStrategy = in.HRStrategy
		existing.OverrideRSSURL = in.OverrideRSSURL
		existing.OverrideSavePath = in.OverrideSavePath
		existing.AssumeFree = in.AssumeFree
		existing.ProxyURL = in.ProxyURL
		existing.UseGlobalProxy = in.UseGlobalProxy
		existing.SkipSSLVerify = in.SkipSSLVerify
		existing.MaxConcurrent = in.MaxConcurrent
		existing.DownloadHourlyLimit = in.DownloadHourlyLimit
		existing.ReseedLimitCount = in.ReseedLimitCount
		existing.ReseedLimitInterval = in.ReseedLimitInterval
		existing.IYUULimitCount = in.IYUULimitCount
		existing.IYUULimitInterval = in.IYUULimitInterval
		existing.PublishFormConfig = in.PublishFormConfig
		existing.AlternativeDomains = in.AlternativeDomains
		existing.TrackerDomains = in.TrackerDomains
		existing.SupportsPiecesHashAPI = in.SupportsPiecesHashAPI
		existing.APIDomain = in.APIDomain
		if err := h.db.WithContext(r.Context()).Model(&existing).Select(siteImportFields).Updates(&existing).Error; err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", in.Domain, err))
			continue
		}
		result.Updated++
	}
	auditLog(r, "site", "import", "sites", "", fmt.Sprintf("total=%d updated=%d skipped=%d", result.Total, result.Updated, len(result.Skipped)), "success")
	h.logger.Info("sites imported", zap.Int("total", result.Total), zap.Int("updated", result.Updated), zap.Int("skipped", len(result.Skipped)))
	Success(w, result)
}
