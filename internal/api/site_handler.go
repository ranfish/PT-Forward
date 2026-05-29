package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/httpclient"
	"github.com/ranfish/pt-forward/internal/metrics"
	"github.com/ranfish/pt-forward/internal/middleware"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/site"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type SiteHandler struct {
	repo      *site.Repository
	db        *gorm.DB
	logger    *zap.Logger
	provider  SiteProvider
	statsSync *site.StatsSyncService
}

type SiteProvider interface {
	GetAdapter(ctx context.Context, domain string) (model.SiteAdapter, error)
	GetSiteConfig(ctx context.Context, domain string) (*model.SiteConfig, error)
}

func (h *SiteHandler) SetProvider(p SiteProvider) {
	h.provider = p
}

func (h *SiteHandler) SetStatsSync(s *site.StatsSyncService) {
	h.statsSync = s
}

func NewSiteHandler(repo *site.Repository, logger *zap.Logger, db *gorm.DB) *SiteHandler {
	return &SiteHandler{repo: repo, db: db, logger: logger}
}

type createSiteRequest struct {
	Name      string `json:"name"`
	Domain    string `json:"domain"`
	BaseURL   string `json:"baseUrl"`
	Framework string `json:"framework"`
	AuthType  string `json:"authType,omitempty"`

	Passkey     string `json:"passkey,omitempty"`
	Cookie      string `json:"cookie,omitempty"`
	APIKey      string `json:"apiKey,omitempty"`
	BearerToken string `json:"bearerToken,omitempty"`
	AuthKey     string `json:"authKey,omitempty"`
	AuthHash    string `json:"authHash,omitempty"`
	UserID      int    `json:"userId,omitempty"`
	RSSKey      string `json:"rssKey,omitempty"`

	HashStrategy     string `json:"hashStrategy,omitempty"`
	SizeStrategy     string `json:"sizeStrategy,omitempty"`
	IDStrategy       string `json:"idStrategy,omitempty"`
	IDPattern        string `json:"idPattern,omitempty"`
	HashXMLTagName   string `json:"hashXmlTagName,omitempty"`
	SizeXMLTagName   string `json:"sizeXmlTagName,omitempty"`
	HashURLParamName string `json:"hashUrlParamName,omitempty"`
	SizeDescRegex    string `json:"sizeDescRegex,omitempty"`
	SizeTitleRegex   string `json:"sizeTitleRegex,omitempty"`
	SizeBaseUnit     int    `json:"sizeBaseUnit,omitempty"`

	DownloadMode        string `json:"downloadMode,omitempty"`
	DownloadURLTemplate string `json:"downloadUrlTemplate,omitempty"`
	DownloadPagePattern string `json:"downloadPagePattern,omitempty"`

	RequiresSideLoading bool `json:"requiresSideLoading"`

	IsSource               bool `json:"isSource"`
	IsTarget               bool `json:"isTarget"`
	ParticipateAutoPublish bool `json:"participateAutoPublish"`

	CookieCloudSync   bool   `json:"cookieCloudSync"`
	CookieCloudDomain string `json:"cookieCloudDomain,omitempty"`
	Enabled           bool   `json:"enabled"`

	AlternativeDomains string `json:"alternativeDomains,omitempty"`

	OverrideRSSURL   string `json:"overrideRssUrl,omitempty"`
	OverrideSavePath string `json:"overrideSavePath,omitempty"`

	ProxyURL      string `json:"proxyUrl,omitempty"`
	SkipSSLVerify bool   `json:"skipSslVerify"`
	MaxConcurrent int    `json:"maxConcurrent,omitempty"`

	HRStrategy string `json:"hrStrategy,omitempty"`
}

func applySiteMaxConcurrent(domain string, maxConcurrent int) {
	if maxConcurrent > 0 {
		httpclient.GlobalLimiter.SetDomainConfig(domain, httpclient.DomainLimitConfig{
			MaxConcurrent: maxConcurrent,
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
	DownloadPagePattern *string `json:"downloadPagePattern,omitempty"`

	RequiresSideLoading *bool `json:"requiresSideLoading,omitempty"`

	IsSource               *bool `json:"isSource,omitempty"`
	IsTarget               *bool `json:"isTarget,omitempty"`
	ParticipateAutoPublish *bool `json:"participateAutoPublish,omitempty"`

	CookieCloudSync    *bool   `json:"cookieCloudSync,omitempty"`
	CookieCloudDomain  *string `json:"cookieCloudDomain,omitempty"`
	AlternativeDomains *string `json:"alternativeDomains,omitempty"`
	Enabled            *bool   `json:"enabled,omitempty"`

	OverrideRSSURL   *string `json:"overrideRssUrl,omitempty"`
	OverrideSavePath *string `json:"overrideSavePath,omitempty"`

	ProxyURL      *string `json:"proxyUrl,omitempty"`
	SkipSSLVerify *bool   `json:"skipSslVerify,omitempty"`
	MaxConcurrent *int    `json:"maxConcurrent,omitempty"`

	HRStrategy *string `json:"hrStrategy,omitempty"`
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
	ParticipateAutoPublish bool `json:"participateAutoPublish"`

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
	DownloadPagePattern string `json:"downloadPagePattern,omitempty"`
	RequiresSideLoading bool   `json:"requiresSideLoading"`

	CookieCloudSync   bool       `json:"cookieCloudSync"`
	CookieCloudDomain string     `json:"cookieCloudDomain,omitempty"`
	LastSyncAt        *time.Time `json:"lastSyncAt,omitempty"`

	AlternativeDomains string `json:"alternativeDomains,omitempty"`

	OverrideRSSURL   string `json:"overrideRssUrl,omitempty"`
	OverrideSavePath string `json:"overrideSavePath,omitempty"`

	ProxyURL      string `json:"proxyUrl,omitempty"`
	SkipSSLVerify bool   `json:"skipSslVerify"`
	MaxConcurrent int    `json:"maxConcurrent"`

	HRStrategy string `json:"hrStrategy,omitempty"`

	HasPasskey     bool `json:"hasPasskey"`
	HasCookie      bool `json:"hasCookie"`
	HasAPIKey      bool `json:"hasApiKey"`
	HasBearerToken bool `json:"hasBearerToken"`
	HasAuthKey     bool `json:"hasAuthKey"`
	HasAuthHash    bool `json:"hasAuthHash"`
	HasRSSKey      bool `json:"hasRssKey"`

	UserID int `json:"userId,omitempty"`

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

func (h *SiteHandler) toResponse(s *model.Site) siteResponse {
	return siteResponse{
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
		ParticipateAutoPublish: s.ParticipateAutoPublish,

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
		DownloadPagePattern: s.DownloadPagePattern,
		RequiresSideLoading: s.RequiresSideLoading,

		CookieCloudSync:   s.CookieCloudSync,
		CookieCloudDomain: s.CookieCloudDomain,
		LastSyncAt:        s.LastSyncAt,

		AlternativeDomains: s.AlternativeDomains,

		OverrideRSSURL:   s.OverrideRSSURL,
		OverrideSavePath: s.OverrideSavePath,

		ProxyURL:      s.ProxyURL,
		SkipSSLVerify: s.SkipSSLVerify,
		MaxConcurrent: s.MaxConcurrent,

		HRStrategy: s.HRStrategy,

		HasPasskey:     s.Passkey != "",
		HasCookie:      s.Cookie != "",
		HasAPIKey:      s.APIKey != "",
		HasBearerToken: s.BearerToken != "",
		HasAuthKey:     s.AuthKey != "",
		HasAuthHash:    s.AuthHash != "",
		HasRSSKey:      s.RSSKey != "",

		UserID: s.UserID,

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
}

var validFrameworks = map[string]bool{
	string(model.FrameworkNexusPHP):  true,
	string(model.FrameworkUnit3D):    true,
	string(model.FrameworkGazelle):   true,
	string(model.FrameworkMTeam):     true,
	string(model.FrameworkTNode):     true,
	string(model.FrameworkLuminance): true,
	string(model.FrameworkRousi):     true,
	string(model.FrameworkGeneric):   true,
}

func (h *SiteHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.handleRouteByPath(w, r)
}

func (h *SiteHandler) handleRouteByPath(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	trimmed := strings.TrimRight(path, "/")

	if trimmed == "/api/v1/sites" {
		if r.Method == http.MethodGet {
			h.handleList(w, r)
			return
		}
		if r.Method == http.MethodPost {
			h.handleCreate(w, r)
			return
		}
		Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		return
	}

	remaining := strings.TrimPrefix(trimmed, "/api/v1/sites/")
	if remaining == "" || remaining == "/" {
		switch r.Method {
		case http.MethodGet:
			h.handleList(w, r)
		case http.MethodPost:
			h.handleCreate(w, r)
		default:
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
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
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
		return
	}

	if remaining == "batch-update" {
		if r.Method == http.MethodPost {
			h.handleBatchUpdate(w, r)
		} else {
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
		return
	}

	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			h.handleGet(w, r)
		case http.MethodPut:
			h.handleUpdate(w, r)
		case http.MethodDelete:
			h.handleDelete(w, r)
		default:
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
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
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
	case "stats":
		switch r.Method {
		case http.MethodGet:
			h.handleGetStats(w, r, idStr)
		case http.MethodPost:
			h.handleSyncSiteStats(w, r, idStr)
		default:
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
	case "overrides":
		h.handleOverrides(w, r, idStr)
	case "freeze":
		h.handleDomainFreezeByID(w, r, idStr)
	default:
		Error(w, http.StatusNotFound, 40400, "路径不存在")
	}
}

func (h *SiteHandler) handleList(w http.ResponseWriter, r *http.Request) {
	page, size := parsePagination(r)
	q := r.URL.Query()
	search := q.Get("search")
	framework := q.Get("framework")
	enabled := q.Get("enabled")
	isSource := q.Get("isSource")
	isTarget := q.Get("isTarget")

	query := h.db.Model(&model.Site{})
	if search != "" {
		escaped := strings.NewReplacer("%", "\\%", "_", "\\_").Replace(search)
		like := "%" + escaped + "%"
		query = query.Where("name LIKE ? OR domain LIKE ? ESCAPE '\\'", like, like)
	}
	if framework != "" {
		query = query.Where("framework = ?", framework)
	}
	if enabled == "true" {
		query = query.Where("enabled = ?", true)
	} else if enabled == "false" {
		query = query.Where("enabled = ?", false)
	}
	if isSource == "true" {
		query = query.Where("is_source = ?", true)
	} else if isSource == "false" {
		query = query.Where("is_source = ?", false)
	}
	if isTarget == "true" {
		query = query.Where("is_target = ?", true)
	} else if isTarget == "false" {
		query = query.Where("is_target = ?", false)
	}

	var total int64
	query.Count(&total)

	var sites []model.Site
	query.Order("name ASC").
		Offset(offset(page, size)).Limit(size).
		Find(&sites)

	items := make([]siteResponse, 0, len(sites))
	for i := range sites {
		items = append(items, h.toResponse(&sites[i]))
	}

	Success(w, PaginatedResult{
		Items: items,
		Total: total,
		Page:  page,
		Size:  size,
	})
}

func (h *SiteHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req createSiteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}

	if req.Name == "" || req.Domain == "" || req.BaseURL == "" {
		Error(w, http.StatusBadRequest, 40001, "name, domain, baseUrl 为必填项")
		return
	}

	if err := middleware.ValidatePublicURL(req.BaseURL); err != nil {
		Error(w, http.StatusBadRequest, 40001, "baseUrl 不合法: "+err.Error())
		return
	}
	if req.ProxyURL != "" {
		if err := middleware.ValidateProxyURL(req.ProxyURL); err != nil {
			Error(w, http.StatusBadRequest, 40001, "proxyUrl 不合法: "+err.Error())
			return
		}
	}
	if req.OverrideRSSURL != "" {
		if err := middleware.ValidatePublicURL(req.OverrideRSSURL); err != nil {
			Error(w, http.StatusBadRequest, 40001, "overrideRssUrl 不合法: "+err.Error())
			return
		}
	}

	if req.Framework == "" {
		req.Framework = "generic"
	}
	if !validFrameworks[req.Framework] {
		Error(w, http.StatusBadRequest, 40001, "framework 必须为 nexusphp/unit3d/gazelle/mteam/tnode/luminance/rousi/generic")
		return
	}

	if req.AuthType == "" {
		req.AuthType = "cookie"
	}
	if !model.ValidAuthType(req.AuthType) {
		Error(w, http.StatusBadRequest, 40001, "authType 必须为 cookie/apikey/passkey")
		return
	}

	exists, err := h.repo.ExistsByDomain(r.Context(), req.Domain, 0)
	if err != nil {
		Error(w, http.StatusInternalServerError, 50000, "检查站点域名失败")
		return
	}
	if exists {
		Error(w, http.StatusConflict, 40900, "站点域名已存在")
		return
	}
	exists, err = h.repo.ExistsByName(r.Context(), req.Name, 0)
	if err != nil {
		Error(w, http.StatusInternalServerError, 50000, "检查站点名称失败")
		return
	}
	if exists {
		Error(w, http.StatusConflict, 40900, "站点名称已存在")
		return
	}

	s := model.Site{
		Name:      req.Name,
		Domain:    req.Domain,
		BaseURL:   req.BaseURL,
		Framework: req.Framework,
		AuthType:  req.AuthType,
		Enabled:   req.Enabled,

		Passkey:     req.Passkey,
		Cookie:      req.Cookie,
		APIKey:      req.APIKey,
		BearerToken: req.BearerToken,
		AuthKey:     req.AuthKey,
		AuthHash:    req.AuthHash,
		UserID:      req.UserID,
		RSSKey:      req.RSSKey,

		HashStrategy:     defaultStr(req.HashStrategy, "guid"),
		SizeStrategy:     defaultStr(req.SizeStrategy, "enclosure"),
		IDStrategy:       defaultStr(req.IDStrategy, "query_param"),
		IDPattern:        req.IDPattern,
		HashXMLTagName:   req.HashXMLTagName,
		SizeXMLTagName:   req.SizeXMLTagName,
		HashURLParamName: req.HashURLParamName,
		SizeDescRegex:    req.SizeDescRegex,
		SizeTitleRegex:   req.SizeTitleRegex,
		SizeBaseUnit:     req.SizeBaseUnit,

		DownloadMode:        defaultStr(req.DownloadMode, "template"),
		DownloadURLTemplate: req.DownloadURLTemplate,
		DownloadPagePattern: req.DownloadPagePattern,
		RequiresSideLoading: req.RequiresSideLoading,

		IsSource:               req.IsSource,
		IsTarget:               req.IsTarget,
		ParticipateAutoPublish: req.ParticipateAutoPublish,

		CookieCloudSync:   req.CookieCloudSync,
		CookieCloudDomain: req.CookieCloudDomain,

		AlternativeDomains: req.AlternativeDomains,

		OverrideRSSURL:   req.OverrideRSSURL,
		OverrideSavePath: req.OverrideSavePath,

		ProxyURL:      req.ProxyURL,
		SkipSSLVerify: req.SkipSSLVerify,
		MaxConcurrent: req.MaxConcurrent,

		HRStrategy: req.HRStrategy,
	}

	if err := h.repo.Create(r.Context(), &s); err != nil {
		Error(w, http.StatusInternalServerError, 50000, "创建站点失败")
		return
	}

	applySiteMaxConcurrent(s.Domain, s.MaxConcurrent)

	h.logger.Info("site created", zap.String("name", s.Name), zap.String("domain", s.Domain))
	Success(w, h.toResponse(&s))
}

func (h *SiteHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	id, err := h.extractID(r.URL.Path, "/api/v1/sites/")
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的站点 ID")
		return
	}

	s, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, 12001, "站点不存在")
		return
	}

	Success(w, h.toResponse(s))
}

func (h *SiteHandler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := h.extractID(r.URL.Path, "/api/v1/sites/")
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的站点 ID")
		return
	}

	s, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, 12001, "站点不存在")
		return
	}

	var req updateSiteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}

	if req.Name != nil && *req.Name != "" {
		exists, _ := h.repo.ExistsByName(r.Context(), *req.Name, id)
		if exists {
			Error(w, http.StatusConflict, 40900, "站点名称已存在")
			return
		}
		s.Name = *req.Name
	}
	if req.Domain != nil && *req.Domain != "" {
		exists, _ := h.repo.ExistsByDomain(r.Context(), *req.Domain, id)
		if exists {
			Error(w, http.StatusConflict, 40900, "站点域名已存在")
			return
		}
		s.Domain = *req.Domain
	}
	if req.BaseURL != nil && *req.BaseURL != "" {
		if err := middleware.ValidatePublicURL(*req.BaseURL); err != nil {
			Error(w, http.StatusBadRequest, 40001, "baseUrl 不合法: "+err.Error())
			return
		}
		s.BaseURL = *req.BaseURL
	}
	if req.Framework != nil && *req.Framework != "" {
		if !validFrameworks[*req.Framework] {
			Error(w, http.StatusBadRequest, 40001, "不支持的 framework")
			return
		}
		s.Framework = *req.Framework
	}
	if req.AuthType != nil {
		if !model.ValidAuthType(*req.AuthType) {
			Error(w, http.StatusBadRequest, 40001, "不支持的 authType")
			return
		}
		s.AuthType = *req.AuthType
	}
	if req.Enabled != nil {
		s.Enabled = *req.Enabled
	}
	if req.IsSource != nil {
		s.IsSource = *req.IsSource
	}
	if req.IsTarget != nil {
		s.IsTarget = *req.IsTarget
	}
	if req.ParticipateAutoPublish != nil {
		s.ParticipateAutoPublish = *req.ParticipateAutoPublish
	}
	if req.CookieCloudSync != nil {
		s.CookieCloudSync = *req.CookieCloudSync
	}
	if req.CookieCloudDomain != nil {
		s.CookieCloudDomain = *req.CookieCloudDomain
	}
	if req.AlternativeDomains != nil {
		s.AlternativeDomains = *req.AlternativeDomains
	}
	if req.Passkey != nil && *req.Passkey != "" {
		s.Passkey = *req.Passkey
	}
	if req.Cookie != nil && *req.Cookie != "" {
		s.Cookie = *req.Cookie
	}
	if req.APIKey != nil && *req.APIKey != "" {
		s.APIKey = *req.APIKey
	}
	if req.HashStrategy != nil {
		s.HashStrategy = *req.HashStrategy
	}
	if req.SizeStrategy != nil {
		s.SizeStrategy = *req.SizeStrategy
	}
	if req.IDStrategy != nil {
		s.IDStrategy = *req.IDStrategy
	}
	if req.IDPattern != nil {
		s.IDPattern = *req.IDPattern
	}
	if req.HashXMLTagName != nil {
		s.HashXMLTagName = *req.HashXMLTagName
	}
	if req.SizeXMLTagName != nil {
		s.SizeXMLTagName = *req.SizeXMLTagName
	}
	if req.HashURLParamName != nil {
		s.HashURLParamName = *req.HashURLParamName
	}
	if req.SizeDescRegex != nil {
		s.SizeDescRegex = *req.SizeDescRegex
	}
	if req.SizeTitleRegex != nil {
		s.SizeTitleRegex = *req.SizeTitleRegex
	}
	if req.SizeBaseUnit != nil {
		s.SizeBaseUnit = *req.SizeBaseUnit
	}
	if req.DownloadMode != nil {
		s.DownloadMode = *req.DownloadMode
	}
	if req.DownloadURLTemplate != nil {
		s.DownloadURLTemplate = *req.DownloadURLTemplate
	}
	if req.DownloadPagePattern != nil {
		s.DownloadPagePattern = *req.DownloadPagePattern
	}
	if req.RequiresSideLoading != nil {
		s.RequiresSideLoading = *req.RequiresSideLoading
	}
	if req.OverrideRSSURL != nil {
		if *req.OverrideRSSURL != "" {
			if err := middleware.ValidatePublicURL(*req.OverrideRSSURL); err != nil {
				Error(w, http.StatusBadRequest, 40001, "overrideRssUrl 不合法: "+err.Error())
				return
			}
		}
		s.OverrideRSSURL = *req.OverrideRSSURL
	}
	if req.OverrideSavePath != nil {
		s.OverrideSavePath = *req.OverrideSavePath
	}
	if req.ProxyURL != nil {
		if *req.ProxyURL != "" {
			if err := middleware.ValidateProxyURL(*req.ProxyURL); err != nil {
				Error(w, http.StatusBadRequest, 40001, "proxyUrl 不合法: "+err.Error())
				return
			}
		}
		s.ProxyURL = *req.ProxyURL
	}
	if req.SkipSSLVerify != nil {
		s.SkipSSLVerify = *req.SkipSSLVerify
	}
	if req.MaxConcurrent != nil {
		s.MaxConcurrent = *req.MaxConcurrent
	}
	if req.HRStrategy != nil {
		s.HRStrategy = *req.HRStrategy
	}

	if err := h.repo.Update(r.Context(), s); err != nil {
		Error(w, http.StatusInternalServerError, 50000, "更新站点失败")
		return
	}

	applySiteMaxConcurrent(s.Domain, s.MaxConcurrent)

	h.logger.Info("site updated", zap.String("name", s.Name))
	Success(w, h.toResponse(s))
}

func (h *SiteHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, err := h.extractID(r.URL.Path, "/api/v1/sites/")
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的站点 ID")
		return
	}

	s, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, 12001, "站点不存在")
		return
	}

	if err := h.repo.Delete(r.Context(), id); err != nil {
		Error(w, http.StatusInternalServerError, 50000, "删除站点失败")
		return
	}

	h.logger.Info("site deleted", zap.String("name", s.Name))
	Success(w, nil)
}

func (h *SiteHandler) handleTest(w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的站点 ID")
		return
	}

	s, err := h.repo.GetByID(r.Context(), uint(id))
	if err != nil {
		Error(w, http.StatusNotFound, 12001, "站点不存在")
		return
	}

	ok, message := h.testSiteConnection(s)

	if !ok {
		Error(w, http.StatusBadGateway, 30001, message)
		return
	}

	if h.statsSync != nil {
		go func() { //nolint:gosec // G118: intentional — must outlive request context
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := h.statsSync.SyncSiteStats(ctx, s.ID); err != nil {
				h.logger.Debug("test-triggered stats sync failed", zap.String("site", s.Name), zap.Error(err))
			}
		}()
	}

	Success(w, map[string]interface{}{
		"ok":      ok,
		"message": message,
	})
}

func (h *SiteHandler) handleDetect(w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的站点 ID")
		return
	}

	s, err := h.repo.GetByID(r.Context(), uint(id))
	if err != nil {
		Error(w, http.StatusNotFound, 12001, "站点不存在")
		return
	}

	result := h.detectFramework(s)
	Success(w, result)
}

func (h *SiteHandler) handleUpdateCredentials(w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的站点 ID")
		return
	}

	_, err = h.repo.GetByID(r.Context(), uint(id))
	if err != nil {
		Error(w, http.StatusNotFound, 12001, "站点不存在")
		return
	}

	var req updateCredentialsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}

	creds := map[string]interface{}{}
	if req.Passkey != nil {
		creds["passkey"] = *req.Passkey
	}
	if req.Cookie != nil {
		creds["cookie"] = *req.Cookie
	}
	if req.APIKey != nil {
		creds["api_key"] = *req.APIKey
	}
	if req.BearerToken != nil {
		creds["bearer_token"] = *req.BearerToken
	}
	if req.AuthKey != nil {
		creds["auth_key"] = *req.AuthKey
	}
	if req.AuthHash != nil {
		creds["auth_hash"] = *req.AuthHash
	}
	if req.UserID != nil {
		creds["user_id"] = *req.UserID
	}
	if req.RSSKey != nil {
		creds["rss_key"] = *req.RSSKey
	}

	if len(creds) == 0 {
		Error(w, http.StatusBadRequest, 40001, "未提供任何凭据字段")
		return
	}

	if err := h.repo.UpdateCredentials(r.Context(), uint(id), creds); err != nil {
		Error(w, http.StatusInternalServerError, 50000, "更新凭据失败")
		return
	}

	updated, err := h.repo.GetByID(r.Context(), uint(id))
	if err != nil {
		Error(w, http.StatusInternalServerError, 50000, "重新获取站点失败")
		return
	}
	h.logger.Info("site credentials updated", zap.String("domain", updated.Domain))
	Success(w, h.toResponse(updated))
}

func (h *SiteHandler) handleGetStats(w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的站点 ID")
		return
	}

	s, err := h.repo.GetByID(r.Context(), uint(id))
	if err != nil {
		Error(w, http.StatusNotFound, 12001, "站点不存在")
		return
	}

	Success(w, map[string]interface{}{
		"username":      s.Username,
		"uploadBytes":   s.UploadBytes,
		"downloadBytes": s.DownloadBytes,
		"seedingPoints": s.SeedingPoints,
		"seedingSize":   s.SeedingSize,
		"seedingCount":  s.SeedingCount,
		"userClass":     s.UserClass,
		"ratio":         s.Ratio,
		"bonusPoints":   s.BonusPoints,
		"statsSyncedAt": s.StatsSyncedAt,
	})
}

func (h *SiteHandler) handleBatchUpdate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs    []uint                 `json:"ids"`
		Fields map[string]interface{} `json:"fields"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}
	if len(req.IDs) == 0 {
		Error(w, http.StatusBadRequest, 40001, "ids 不能为空")
		return
	}
	if len(req.Fields) == 0 {
		Error(w, http.StatusBadRequest, 40001, "fields 不能为空")
		return
	}

	updates := make(map[string]interface{})
	boolFields := map[string]bool{
		"enabled": true, "is_source": true, "is_target": true,
		"participate_auto_publish": true, "cookie_cloud_sync": true,
	}
	for k, v := range req.Fields {
		if !boolFields[k] {
			Error(w, http.StatusBadRequest, 40001, "不允许批量更新字段: "+k)
			return
		}
		bVal, ok := v.(bool)
		if !ok {
			Error(w, http.StatusBadRequest, 40001, "字段 "+k+" 必须为布尔值")
			return
		}
		updates[k] = bVal
	}
	if len(updates) == 0 {
		Error(w, http.StatusBadRequest, 40001, "没有有效的更新字段")
		return
	}

	result := h.db.WithContext(r.Context()).Model(&model.Site{}).Where("id IN ?", req.IDs).Updates(updates)
	if result.Error != nil {
		Error(w, http.StatusInternalServerError, 50000, "批量更新失败: "+result.Error.Error())
		return
	}

	Success(w, map[string]interface{}{
		"updated": result.RowsAffected,
	})
}

func (h *SiteHandler) handleSyncAllStats(w http.ResponseWriter, r *http.Request) {
	if h.statsSync == nil {
		Error(w, http.StatusServiceUnavailable, 50001, "Stats 同步服务未初始化")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	synced, failedSites := h.statsSync.SyncAllSites(ctx)
	Success(w, map[string]interface{}{
		"synced":      synced,
		"failed":      len(failedSites),
		"failedSites": failedSites,
	})
}

func (h *SiteHandler) handleSyncSiteStats(w http.ResponseWriter, r *http.Request, idStr string) {
	if h.statsSync == nil {
		Error(w, http.StatusServiceUnavailable, 50001, "Stats 同步服务未初始化")
		return
	}
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的站点 ID")
		return
	}

	if err := h.statsSync.SyncSiteStats(r.Context(), uint(id)); err != nil {
		updated, _ := h.repo.GetByID(r.Context(), uint(id))
		if updated != nil {
			resp := h.toResponse(updated)
			Success(w, map[string]interface{}{
				"site":         resp,
				"syncWarning":  err.Error(),
			})
			return
		}
		Error(w, http.StatusBadGateway, 50000, "同步站点统计失败: "+err.Error())
		return
	}

	updated, err := h.repo.GetByID(r.Context(), uint(id))
	if err != nil {
		Error(w, http.StatusInternalServerError, 50000, "重新获取站点失败")
		return
	}
	Success(w, h.toResponse(updated))
}

func (h *SiteHandler) testSiteConnection(s *model.Site) (bool, string) {
	switch s.AuthType {
	case "cookie":
		if s.Cookie == "" {
			return false, "未配置 Cookie"
		}
	case "apikey":
		if s.APIKey == "" {
			return false, "未配置 API Key"
		}
	case "passkey":
		if s.Passkey == "" {
			return false, "未配置 Passkey"
		}
	}

	client := buildSiteHTTPClient(s, 15*time.Second)

	switch s.AuthType {
	case "cookie":
		return h.testCookieAuth(client, s)
	case "apikey":
		return h.testAPIKeyAuth(client, s)
	case "passkey":
		return h.testPasskeyAuth(client, s)
	}
	return h.testCookieAuth(client, s)
}

func (h *SiteHandler) testCookieAuth(client *http.Client, s *model.Site) (bool, string) {
	req, err := http.NewRequest("GET", s.BaseURL, nil) //nolint:gosec // admin test endpoint, URL from site config
	if err != nil {
		return false, "构造请求失败: " + err.Error()
	}
	req.Header.Set("Cookie", s.Cookie)
	resp, err := client.Do(req) //nolint:gosec // admin test endpoint
	if err != nil {
		return false, "连接失败: " + err.Error()
	}
	defer func() { httpclient.DrainBody(resp) }()
	if resp.StatusCode >= 400 {
		return false, fmt.Sprintf("HTTP %d", resp.StatusCode)
	}

	bodyData, _ := io.ReadAll(io.LimitReader(resp.Body, 128*1024))
	bodyStr := string(bodyData)
	lower := strings.ToLower(bodyStr)

	loginIndicators := []string{
		`type="password"`,
		`name="password"`,
		`id="password"`,
		`name="login"`,
		`action="login"`,
		`action="/login"`,
		`action="/takelogin"`,
		`action="takelogin"`,
		`please login`,
		`please log in`,
		`you must login`,
		`you must log in`,
		`请先登录`,
		`login_return`,
		`login_form`,
	}
	for _, indicator := range loginIndicators {
		if strings.Contains(lower, indicator) {
			return false, "Cookie 无效或已过期（页面返回登录表单）"
		}
	}

	finalURL := resp.Request.URL.String()
	if strings.Contains(finalURL, "login") && !strings.Contains(finalURL, "userdetails") {
		return false, "Cookie 无效或已过期（页面重定向到登录页）"
	}

	if !strings.Contains(bodyStr, "userdetails") && !strings.Contains(bodyStr, "logout") {
		return false, "Cookie 无效或已过期（页面缺少登录态标识）"
	}

	return true, "连接成功"
}

func (h *SiteHandler) testAPIKeyAuth(client *http.Client, s *model.Site) (bool, string) {
	testURL := s.BaseURL
	if s.Framework == "mteam" {
		testURL = strings.TrimRight(s.BaseURL, "/") + "/api/member/profile"
	}
	req, err := http.NewRequest("POST", testURL, nil) //nolint:gosec // admin test endpoint, URL from site config
	if err != nil {
		return false, "构造请求失败: " + err.Error()
	}
	req.Header.Set("x-api-key", s.APIKey)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")

	noRedirectClient := &http.Client{
		Timeout:   client.Timeout,
		Transport: client.Transport,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := noRedirectClient.Do(req) //nolint:gosec // admin test endpoint
	if err != nil {
		return false, "连接失败: " + err.Error()
	}
	defer func() { httpclient.DrainBody(resp) }()

	bodyData, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	bodyStr := string(bodyData)

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return false, "API Key 无效或已过期"
	}
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return false, "API Key 无效或已过期（被重定向）"
	}
	if resp.StatusCode >= 400 {
		return false, fmt.Sprintf("HTTP %d", resp.StatusCode)
	}

	lower := strings.ToLower(bodyStr)

	if strings.Contains(lower, `"code":1`) || strings.Contains(lower, `"code":401`) || strings.Contains(lower, `"code":403`) {
		return false, "API Key 无效或已过期"
	}
	if strings.Contains(lower, `"error"`) || strings.Contains(lower, "無效") || strings.Contains(lower, "无效") {
		return false, "API Key 无效或已过期"
	}

	return true, "连接成功"
}

func (h *SiteHandler) testPasskeyAuth(client *http.Client, s *model.Site) (bool, string) {
	req, err := http.NewRequest("GET", strings.TrimRight(s.BaseURL, "/")+"/api/v1/torrents", nil) //nolint:gosec // admin test endpoint
	if err != nil {
		return false, "构造请求失败: " + err.Error()
	}
	req.Header.Set("Authorization", "Bearer "+s.Passkey)

	resp, err := client.Do(req) //nolint:gosec // admin test endpoint
	if err != nil {
		return false, "连接失败: " + err.Error()
	}
	defer func() { httpclient.DrainBody(resp) }()

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return false, "Passkey 无效"
	}
	if resp.StatusCode >= 400 {
		return false, fmt.Sprintf("HTTP %d", resp.StatusCode)
	}

	bodyData, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	bodyStr := string(bodyData)
	lower := strings.ToLower(bodyStr)

	if strings.Contains(lower, `"code":401`) || strings.Contains(lower, `"code":403`) {
		return false, "Passkey 无效"
	}
	if strings.Contains(lower, "缺少认证") || strings.Contains(lower, "unauthorized") || strings.Contains(lower, "invalid") {
		return false, "Passkey 无效"
	}

	return true, "连接成功"
}

func (h *SiteHandler) detectFramework(s *model.Site) *model.DetectResult {
	client := buildSiteHTTPClient(s, 15*time.Second)
	req, err := http.NewRequest("GET", s.BaseURL, nil) //nolint:gosec // admin detect endpoint, URL from site config
	if err != nil {
		return &model.DetectResult{
			Framework:       s.Framework,
			Confidence:      0,
			DetectionDetail: "无法访问站点: " + err.Error(),
		}
	}
	if s.Cookie != "" {
		req.Header.Set("Cookie", s.Cookie)
	}
	resp, err := client.Do(req) //nolint:gosec // admin detect endpoint
	if err != nil {
		return &model.DetectResult{
			Framework:       s.Framework,
			Confidence:      0,
			DetectionDetail: "无法访问站点: " + err.Error(),
		}
	}
	defer func() { httpclient.DrainBody(resp) }()

	bodyData, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	bodyStr := string(bodyData)

	framework := "generic"
	confidence := 0.3
	detail := ""

	switch {
	case strings.Contains(bodyStr, "NexusPHP") || strings.Contains(bodyStr, "nexusphp"):
		framework = "nexusphp"
		confidence = 0.9
		detail = "检测到 NexusPHP 标识"
	case strings.Contains(bodyStr, "UNIT3D") || strings.Contains(bodyStr, "unit3d"):
		framework = "unit3d"
		confidence = 0.9
		detail = "检测到 Unit3D 标识"
	case strings.Contains(bodyStr, "Gazelle") || strings.Contains(bodyStr, "gazelle"):
		framework = "gazelle"
		confidence = 0.9
		detail = "检测到 Gazelle 标识"
	case strings.Contains(bodyStr, "M-Team") || strings.Contains(bodyStr, "mteam"):
		framework = "mteam"
		confidence = 0.9
		detail = "检测到 M-Team 标识"
	case strings.Contains(bodyStr, "TNode") || strings.Contains(bodyStr, "tnode") || strings.Contains(bodyStr, "朱雀"):
		framework = "tnode"
		confidence = 0.9
		detail = "检测到 TNode 标识"
	case strings.Contains(bodyStr, "Luminance") || strings.Contains(bodyStr, "luminance"):
		framework = "luminance"
		confidence = 0.9
		detail = "检测到 Luminance 标识"
	case strings.Contains(bodyStr, "Rousi") || strings.Contains(bodyStr, "rousi"):
		framework = "rousi"
		confidence = 0.9
		detail = "检测到 Rousi 标识"
	case strings.Contains(bodyStr, "Nexus"):
		framework = "nexusphp"
		confidence = 0.7
		detail = "检测到 Nexus 字样（可能是 NexusPHP）"
	default:
		detail = "未能识别框架，使用 generic"
	}

	defaults := model.FrameworkDefaults{
		HashStrategy: frameworkDefaultHash(framework),
		SizeStrategy: frameworkDefaultSize(framework),
		IDStrategy:   frameworkDefaultID(framework),
	}
	switch framework {
	case "nexusphp", "mteam":
		defaults.DownloadURLTemplate = s.BaseURL + "/download.php?id={id}&passkey={passkey}"
	case "unit3d":
		defaults.IDPattern = `\/torrent\/(\d+)`
		defaults.DownloadURLTemplate = s.BaseURL + "/torrent/download/{id}"
	case "gazelle":
		defaults.DownloadURLTemplate = s.BaseURL + "/torrents.php?action=download&id={id}&authkey={authkey}&torrent_pass={passkey}"
	case "luminance":
		defaults.DownloadURLTemplate = s.BaseURL + "/torrents.php?action=download&id={id}&authkey={authkey}&torrent_pass={passkey}"
	case "tnode":
		defaults.DownloadURLTemplate = s.BaseURL + "/download.php?id={id}&passkey={passkey}"
	case "rousi":
		defaults.IDPattern = "uuid"
		defaults.DownloadURLTemplate = s.BaseURL + "/api/torrent/{id}/download/{passkey}"
	}

	return &model.DetectResult{
		Framework:       framework,
		Confidence:      confidence,
		DetectionDetail: detail,
		Defaults:        defaults,
	}
}

func (h *SiteHandler) extractID(path string, prefix string) (uint, error) {
	p := strings.TrimPrefix(path, prefix)
	p = strings.TrimRight(p, "/")
	n, err := strconv.ParseUint(p, 10, 32)
	if err != nil {
		return 0, apiError(ErrAPIInvalidID, "invalid id", nil)
	}
	return uint(n), nil
}

func defaultStr(val, def string) string {
	if val == "" {
		return def
	}
	return val
}

func buildSiteHTTPClient(s *model.Site, timeout time.Duration) *http.Client {
	return httpclient.NewSiteHTTPClient(httpclient.SiteHTTPConfig{
		Domain:        s.Domain,
		Timeout:       timeout,
		ProxyURL:      s.ProxyURL,
		SkipSSLVerify: s.SkipSSLVerify,
	})
}

func frameworkDefaultHash(fw string) string {
	switch fw {
	case "gazelle", "luminance":
		return "xml_tag"
	case "unit3d":
		return "fake_from_id"
	default:
		return "guid"
	}
}

func frameworkDefaultSize(fw string) string {
	switch fw {
	case "unit3d":
		return "desc_regex"
	case "gazelle", "luminance":
		return "xml_tag"
	default:
		return "enclosure"
	}
}

func frameworkDefaultID(fw string) string {
	switch fw {
	case "unit3d", "gazelle":
		return "link_regex"
	default:
		return "query_param"
	}
}

func (h *SiteHandler) handleOverrides(w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "invalid site id")
		return
	}

	var site model.Site
	if err := h.repo.DB().First(&site, id).Error; err != nil {
		Error(w, http.StatusNotFound, 40400, "站点不存在")
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handleListOverrides(w, r, site.Name)
	case http.MethodPut:
		h.handleUpsertOverride(w, r, site.Name)
	case http.MethodDelete:
		h.handleDeleteOverride(w, r, site.Name)
	default:
		Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
	}
}

func (h *SiteHandler) handleListOverrides(w http.ResponseWriter, _ *http.Request, siteName string) {
	var overrides []model.SiteConfigOverride
	if err := h.repo.DB().Where("site_name = ?", siteName).Find(&overrides).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "获取站点配置覆盖失败")
		return
	}
	Success(w, map[string]interface{}{
		"items": overrides,
		"total": len(overrides),
	})
}

func (h *SiteHandler) handleUpsertOverride(w http.ResponseWriter, r *http.Request, siteName string) {
	var req struct {
		FieldPath  string `json:"fieldPath"`
		FieldValue string `json:"fieldValue"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}
	if req.FieldPath == "" {
		Error(w, http.StatusBadRequest, 40001, "fieldPath 为必填项")
		return
	}

	var existing model.SiteConfigOverride
	err := h.repo.DB().
		Where("site_name = ? AND field_path = ?", siteName, req.FieldPath).
		First(&existing).Error

	if err != nil {
		override := model.SiteConfigOverride{
			SiteName:   siteName,
			FieldPath:  req.FieldPath,
			FieldValue: req.FieldValue,
			Source:     "web_ui",
		}
		if err := h.repo.DB().Create(&override).Error; err != nil {
			Error(w, http.StatusInternalServerError, 50000, "创建覆盖失败")
			return
		}
		Success(w, map[string]interface{}{"id": override.ID, "message": "覆盖已创建"})
	} else {
		if err := h.repo.DB().Model(&existing).
			Update("field_value", req.FieldValue).Error; err != nil {
			Error(w, http.StatusInternalServerError, 50000, "更新覆盖失败")
			return
		}
		Success(w, map[string]interface{}{"id": existing.ID, "message": "覆盖已更新"})
	}
}

func (h *SiteHandler) handleDeleteOverride(w http.ResponseWriter, r *http.Request, siteName string) {
	fieldPath := r.URL.Query().Get("fieldPath")
	q := h.repo.DB().Where("site_name = ?", siteName)
	if fieldPath != "" {
		q = q.Where("field_path = ?", fieldPath)
	}
	result := q.Delete(&model.SiteConfigOverride{})
	if result.Error != nil {
		Error(w, http.StatusInternalServerError, 50000, "删除覆盖配置失败")
		return
	}
	Success(w, map[string]interface{}{
		"deleted": result.RowsAffected,
	})
}

func (h *SiteHandler) handleDomainFreezeByID(w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "invalid site id")
		return
	}
	s, err := h.repo.GetByID(r.Context(), uint(id))
	if err != nil {
		Error(w, http.StatusNotFound, 40400, "站点不存在")
		return
	}
	h.handleDomainFreeze(w, r, s.Domain)
}

func (h *SiteHandler) handleFreezeStatus(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		statuses := httpclient.GlobalLimiter.GetDomainStatuses()
		Success(w, statuses)
	case http.MethodDelete:
		var req struct {
			Domain string `json:"domain"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			Error(w, http.StatusBadRequest, 40001, "invalid request body")
			return
		}
		if req.Domain == "" {
			Error(w, http.StatusBadRequest, 40001, "domain is required")
			return
		}
		httpclient.GlobalLimiter.ManualUnfreeze(req.Domain)
		Success(w, map[string]string{"status": "unfrozen", "domain": req.Domain})
	default:
		Error(w, http.StatusMethodNotAllowed, 40001, "method not allowed")
	}
}

func (h *SiteHandler) handleCircuitStatus(w http.ResponseWriter, r *http.Request) {
	if httpclient.GlobalCircuitBreaker == nil {
		Success(w, map[string]interface{}{})
		return
	}

	switch r.Method {
	case http.MethodGet:
		statuses := httpclient.GlobalCircuitBreaker.GetAllCircuitStatuses()
		Success(w, statuses)

	case http.MethodPost:
		var req struct {
			Domain string `json:"domain"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			Error(w, http.StatusBadRequest, 40001, "invalid request body")
			return
		}
		if req.Domain == "" {
			Error(w, http.StatusBadRequest, 40001, "domain is required")
			return
		}
		httpclient.GlobalCircuitBreaker.ResetCircuit(req.Domain)
		Success(w, map[string]string{"status": "reset", "domain": req.Domain})

	default:
		Error(w, http.StatusMethodNotAllowed, 40001, "method not allowed")
	}
}

func (h *SiteHandler) handleDomainFreeze(w http.ResponseWriter, r *http.Request, domain string) {
	switch r.Method {
	case http.MethodPost:
		var req struct {
			Duration string `json:"duration"`
			Reason   string `json:"reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			Error(w, http.StatusBadRequest, 40001, "invalid request body")
			return
		}
		dur, err := time.ParseDuration(req.Duration)
		if err != nil {
			Error(w, http.StatusBadRequest, 40001, "invalid duration format")
			return
		}
		httpclient.GlobalLimiter.ManualFreeze(domain, dur, req.Reason)
		Success(w, map[string]string{"status": "frozen", "domain": domain})

	case http.MethodDelete:
		httpclient.GlobalLimiter.ManualUnfreeze(domain)
		Success(w, map[string]string{"status": "unfrozen", "domain": domain})

	case http.MethodGet:
		status := httpclient.GlobalLimiter.GetFreezeStatus(domain)
		Success(w, status)

	default:
		Error(w, http.StatusMethodNotAllowed, 40001, "method not allowed")
	}
}

func (h *SiteHandler) handleExclusions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		var exclusions []model.PublishExclusion
		if err := h.db.WithContext(r.Context()).Find(&exclusions).Error; err != nil {
			Error(w, http.StatusInternalServerError, 50000, "failed to list exclusions")
			return
		}
		Success(w, exclusions)

	case http.MethodPost:
		var req struct {
			TargetSite string `json:"target_site"`
			SourceSite string `json:"source_site"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			Error(w, http.StatusBadRequest, 40001, "invalid request body")
			return
		}
		if req.TargetSite == "" || req.SourceSite == "" {
			Error(w, http.StatusBadRequest, 40001, "target_site and source_site are required")
			return
		}
		exclusion := model.PublishExclusion{
			TargetSite: req.TargetSite,
			SourceSite: req.SourceSite,
		}
		if err := h.db.WithContext(r.Context()).Create(&exclusion).Error; err != nil {
			if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "UNIQUE") {
				Error(w, http.StatusConflict, 40900, "exclusion already exists")
				return
			}
			Error(w, http.StatusInternalServerError, 50000, "failed to create exclusion")
			return
		}
		Success(w, exclusion)

	case http.MethodDelete:
		var req struct {
			TargetSite string `json:"target_site"`
			SourceSite string `json:"source_site"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			Error(w, http.StatusBadRequest, 40001, "invalid request body")
			return
		}
		result := h.db.WithContext(r.Context()).
			Where("target_site = ? AND source_site = ?", req.TargetSite, req.SourceSite).
			Delete(&model.PublishExclusion{})
		if result.Error != nil {
			Error(w, http.StatusInternalServerError, 50000, "删除排除规则失败")
			return
		}
		if result.RowsAffected == 0 {
			Error(w, http.StatusNotFound, 40400, "exclusion not found")
			return
		}
		Success(w, map[string]string{"status": "deleted"})

	default:
		Error(w, http.StatusMethodNotAllowed, 40001, "method not allowed")
	}
}

type searchRequest struct {
	Query      string `json:"query"`
	Category   string `json:"category,omitempty"`
	FreeOnly   bool   `json:"freeOnly,omitempty"`
	SortBy     string `json:"sortBy,omitempty"`
	MaxResults int    `json:"maxResults,omitempty"`
}

func (h *SiteHandler) handleSearch(w http.ResponseWriter, r *http.Request, idStr string) {
	if r.Method != http.MethodPost {
		Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		return
	}
	if h.provider == nil {
		Error(w, http.StatusServiceUnavailable, 50001, "站点服务未就绪")
		return
	}

	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, 40002, "无效的站点ID")
		return
	}

	var site model.Site
	if err := h.db.WithContext(r.Context()).First(&site, id).Error; err != nil {
		Error(w, http.StatusNotFound, 40400, "站点不存在")
		return
	}

	var req searchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40003, "请求参数错误")
		return
	}
	if req.Query == "" {
		Error(w, http.StatusBadRequest, 40004, "搜索关键词不能为空")
		return
	}

	adapter, err := h.provider.GetAdapter(r.Context(), site.Domain)
	if err != nil {
		Error(w, http.StatusInternalServerError, 50002, "获取站点适配器失败")
		return
	}

	config, err := h.provider.GetSiteConfig(r.Context(), site.Domain)
	if err != nil {
		Error(w, http.StatusInternalServerError, 50003, "获取站点配置失败")
		return
	}

	opts := &model.SearchOptions{
		Category:   req.Category,
		FreeOnly:   req.FreeOnly,
		SortBy:     req.SortBy,
		MaxResults: req.MaxResults,
	}
	if opts.MaxResults <= 0 {
		opts.MaxResults = 50
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	results, err := adapter.SearchTorrents(ctx, config, req.Query, opts)
	if err != nil {
		h.logger.Warn("search torrents failed",
			zap.String("site", site.Domain),
			zap.String("query", req.Query),
			zap.Error(err))
		metrics.SiteRequestErrors.WithLabelValues(site.Domain, "search").Inc()
		Error(w, http.StatusInternalServerError, 50004, "搜索失败")
		return
	}

	if results == nil {
		results = []*model.SeedingSearchResult{}
	}

	metrics.SiteRequestsTotal.WithLabelValues(site.Domain, "search").Inc()
	Success(w, results)
}

type discountRequest struct {
	TorrentID string `json:"torrentId"`
}

func (h *SiteHandler) handleDetectDiscount(w http.ResponseWriter, r *http.Request, idStr string) {
	if r.Method != http.MethodPost {
		Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		return
	}
	if h.provider == nil {
		Error(w, http.StatusServiceUnavailable, 50001, "站点服务未就绪")
		return
	}

	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, 40002, "无效的站点ID")
		return
	}

	var site model.Site
	if err := h.db.WithContext(r.Context()).First(&site, id).Error; err != nil {
		Error(w, http.StatusNotFound, 40400, "站点不存在")
		return
	}

	var req discountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40003, "请求参数错误")
		return
	}
	if req.TorrentID == "" {
		Error(w, http.StatusBadRequest, 40005, "种子ID不能为空")
		return
	}

	adapter, err := h.provider.GetAdapter(r.Context(), site.Domain)
	if err != nil {
		Error(w, http.StatusInternalServerError, 50002, "获取站点适配器失败")
		return
	}

	config, err := h.provider.GetSiteConfig(r.Context(), site.Domain)
	if err != nil {
		Error(w, http.StatusInternalServerError, 50003, "获取站点配置失败")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	result, err := adapter.DetectDiscount(ctx, config, req.TorrentID)
	if err != nil {
		h.logger.Warn("detect discount failed",
			zap.String("site", site.Domain),
			zap.String("torrentId", req.TorrentID),
			zap.Error(err))
		metrics.SiteRequestErrors.WithLabelValues(site.Domain, "discount").Inc()
		Error(w, http.StatusInternalServerError, 50005, "检测折扣失败")
		return
	}

	metrics.SiteRequestsTotal.WithLabelValues(site.Domain, "discount").Inc()
	Success(w, result)
}
