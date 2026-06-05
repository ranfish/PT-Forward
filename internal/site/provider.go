package site

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/ranfish/pt-forward/internal/adapter"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Provider struct {
	db      *gorm.DB
	repo    *Repository
	factory *adapter.Factory
	doer    *adapter.HTTPDoer
	logger  *zap.Logger

	mu       sync.RWMutex
	adapters map[string]model.SiteAdapter
}

func NewProvider(db *gorm.DB, factory *adapter.Factory, logger *zap.Logger) *Provider {
	return &Provider{
		db:       db,
		repo:     NewRepository(db),
		factory:  factory,
		doer:     adapter.NewHTTPDoer(),
		logger:   logger,
		adapters: make(map[string]model.SiteAdapter),
	}
}

func (p *Provider) GetSiteInfo(ctx context.Context, siteName string) (*model.SiteInfo, error) {
	site, err := p.repo.GetByName(ctx, siteName)
	if err != nil {
		return nil, &model.AppError{Code: 12001, Message: "站点不存在: " + siteName}
	}
	info := siteToInfo(site)
	p.applyOverridesToInfo(ctx, siteName, info)
	return info, nil
}

func (p *Provider) applyOverridesToInfo(ctx context.Context, siteName string, info *model.SiteInfo) {
	var overrides []model.SiteConfigOverride
	if err := p.db.WithContext(ctx).
		Where("site_name = ?", siteName).
		Find(&overrides).Error; err != nil {
		p.logger.Warn("query site config overrides failed, using defaults",
			zap.String("site", siteName), zap.Error(err))
		return
	}

	for _, o := range overrides {
		switch o.FieldPath {
		case "cookie":
			info.Cookie = o.FieldValue
		case "passkey":
			info.Passkey = o.FieldValue
		case "api_key":
			info.APIKey = o.FieldValue
		case "auth_key":
			info.AuthKey = o.FieldValue
		case "auth_hash":
			info.AuthHash = o.FieldValue
		case "rss_key":
			info.RSSKey = o.FieldValue
		case "bearer_token":
			info.BearerToken = o.FieldValue
		case "user_id":
			var uid int
			if _, err := fmt.Sscanf(o.FieldValue, "%d", &uid); err == nil {
				info.UserID = uid
			}
		case "download_url_template":
			info.DownloadURLTemplate = o.FieldValue
		case "hash_strategy":
			info.HashStrategy = o.FieldValue
		case "size_strategy":
			info.SizeStrategy = o.FieldValue
		case "id_strategy":
			info.IDStrategy = o.FieldValue
		case "id_pattern":
			info.IDPattern = o.FieldValue
		}
	}
}

func (p *Provider) GetSiteConfig(ctx context.Context, domain string) (*model.SiteConfig, error) {
	site := p.repo.GetByDomain(ctx, domain)
	if site == nil {
		return nil, &model.AppError{Code: 12001, Message: "站点不存在: " + domain}
	}
	config := siteToConfig(site)
	p.applyOverrides(ctx, site.Name, config)
	return config, nil
}

func (p *Provider) applyOverrides(ctx context.Context, siteName string, config *model.SiteConfig) {
	var overrides []model.SiteConfigOverride
	if err := p.db.WithContext(ctx).
		Where("site_name = ?", siteName).
		Find(&overrides).Error; err != nil {
		p.logger.Warn("query site config overrides failed, using defaults",
			zap.String("site", siteName), zap.Error(err))
		return
	}

	for _, o := range overrides {
		switch o.FieldPath {
		case "cookie":
			config.Cookie = o.FieldValue
		case "passkey":
			config.Passkey = o.FieldValue
		case "api_key":
			config.APIKey = o.FieldValue
		case "auth_key":
			config.AuthKey = o.FieldValue
		case "auth_hash":
			config.AuthHash = o.FieldValue
		case "rss_key":
			config.RSSKey = o.FieldValue
		case "bearer_token":
			config.BearerToken = o.FieldValue
		case "user_id":
			var uid int
			if _, err := fmt.Sscanf(o.FieldValue, "%d", &uid); err == nil {
				config.UserID = uid
			}
		case "download_url_template":
			config.RSS.URLTemplate = o.FieldValue
			config.DownloadURLTemplate = o.FieldValue
		case "hash_strategy":
			config.RSS.HashStrategy = model.HashStrategy(o.FieldValue)
		case "size_strategy":
			config.RSS.SizeStrategy = model.SizeStrategy(o.FieldValue)
		case "id_strategy":
			config.RSS.IDStrategy = model.IDStrategy(o.FieldValue)
		case "id_pattern":
			config.RSS.IDPattern = o.FieldValue
		case "paths.upload":
			config.Paths.Upload = o.FieldValue
		case "paths.takeupload":
			config.Paths.TakeUpload = o.FieldValue
		case "paths.browse":
			config.Paths.Browse = o.FieldValue
		case "paths.detail":
			config.Paths.Detail = o.FieldValue
		default:
			if strings.HasPrefix(o.FieldPath, "publish.form_fields.") {
				key := strings.TrimPrefix(o.FieldPath, "publish.form_fields.")
				if config.Publish.FormFields == nil {
					config.Publish.FormFields = make(map[string]string)
				}
				config.Publish.FormFields[key] = o.FieldValue
			}
		}
	}
}

func (p *Provider) GetSiteDefault(ctx context.Context, domain string) (*model.SiteDefault, error) {
	site := p.repo.GetByDomain(ctx, domain)
	if site == nil {
		return nil, &model.AppError{Code: 12001, Message: "站点不存在: " + domain}
	}

	defs, ok := adapter.FrameworkDefaults[site.Framework]
	if !ok {
		defs = adapter.FrameworkDefaults["generic"]
	}

	rssCfg := model.SiteRSSConfig{
		HashStrategy: model.HashStrategy(defs.HashStrategy),
		SizeStrategy: model.SizeStrategy(defs.SizeStrategy),
		IDStrategy:   model.IDStrategy(defs.IDStrategy),
		IDPattern:    defs.IDPattern,
		URLTemplate:  defs.DownloadURLTemplate,
	}

	return &model.SiteDefault{
		Domain:    site.Domain,
		Framework: site.Framework,
		RSS:       rssCfg,
	}, nil
}

func (p *Provider) GetAdapter(ctx context.Context, domain string) (model.SiteAdapter, error) {
	p.mu.RLock()
	if a, ok := p.adapters[domain]; ok {
		p.mu.RUnlock()
		return a, nil
	}
	p.mu.RUnlock()

	site := p.repo.GetByDomain(ctx, domain)
	if site == nil {
		return nil, &model.AppError{Code: 12001, Message: "站点不存在: " + domain}
	}

	a := p.factory.Create(site.Framework, adapter.NewHTTPDoerWithSite(site.ProxyURL, site.SkipSSLVerify))

	p.mu.Lock()
	p.adapters[domain] = a
	p.mu.Unlock()

	return a, nil
}

func (p *Provider) ListSites(ctx context.Context) ([]*model.SiteInfo, error) {
	sites, err := p.repo.List(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*model.SiteInfo, 0, len(sites))
	for i := range sites {
		result = append(result, siteToInfo(&sites[i]))
	}
	return result, nil
}

type SiteResources struct {
	Configs  map[string]*model.SiteConfig
	Adapters map[string]model.SiteAdapter
}

func (p *Provider) BatchLoadSiteResources(ctx context.Context, domains []string) (*SiteResources, error) {
	var sites []model.Site
	if err := p.db.WithContext(ctx).Where("base_url IN ?", domains).Find(&sites).Error; err != nil {
		return nil, &model.AppError{Code: 12001, Message: "批量查询站点失败"}
	}

	siteNames := make([]string, len(sites))
	for i, s := range sites {
		siteNames[i] = s.Name
	}

	var allOverrides []model.SiteConfigOverride
	if len(siteNames) > 0 {
		if err := p.db.WithContext(ctx).Where("site_name IN ?", siteNames).Find(&allOverrides).Error; err != nil {
			p.logger.Warn("batch load site overrides failed", zap.Error(err))
			allOverrides = nil
		}
	}

	overridesBySite := make(map[string][]model.SiteConfigOverride)
	for _, o := range allOverrides {
		overridesBySite[o.SiteName] = append(overridesBySite[o.SiteName], o)
	}

	configs := make(map[string]*model.SiteConfig, len(sites))
	adapters := make(map[string]model.SiteAdapter, len(sites))

	for i := range sites {
		s := &sites[i]
		config := siteToConfig(s)
		if ov, ok := overridesBySite[s.Name]; ok {
			p.applyOverridesFromList(config, ov)
		}
		configs[s.BaseURL] = config

		p.mu.RLock()
		if a, ok := p.adapters[s.BaseURL]; ok {
			p.mu.RUnlock()
			adapters[s.BaseURL] = a
		} else {
			p.mu.RUnlock()
			a := p.factory.Create(s.Framework, adapter.NewHTTPDoerWithSite(s.ProxyURL, s.SkipSSLVerify))
			p.mu.Lock()
			p.adapters[s.BaseURL] = a
			p.mu.Unlock()
			adapters[s.BaseURL] = a
		}
	}

	return &SiteResources{Configs: configs, Adapters: adapters}, nil
}

func (p *Provider) applyOverridesFromList(config *model.SiteConfig, overrides []model.SiteConfigOverride) {
	for _, o := range overrides {
		switch o.FieldPath {
		case "cookie":
			config.Cookie = o.FieldValue
		case "passkey":
			config.Passkey = o.FieldValue
		case "api_key":
			config.APIKey = o.FieldValue
		case "auth_key":
			config.AuthKey = o.FieldValue
		case "auth_hash":
			config.AuthHash = o.FieldValue
		case "rss_key":
			config.RSSKey = o.FieldValue
		case "bearer_token":
			config.BearerToken = o.FieldValue
		case "user_id":
			var uid int
			if _, err := fmt.Sscanf(o.FieldValue, "%d", &uid); err == nil {
				config.UserID = uid
			}
		case "download_url_template":
			config.RSS.URLTemplate = o.FieldValue
			config.DownloadURLTemplate = o.FieldValue
		case "hash_strategy":
			config.RSS.HashStrategy = model.HashStrategy(o.FieldValue)
		case "size_strategy":
			config.RSS.SizeStrategy = model.SizeStrategy(o.FieldValue)
		case "id_strategy":
			config.RSS.IDStrategy = model.IDStrategy(o.FieldValue)
		case "id_pattern":
			config.RSS.IDPattern = o.FieldValue
		case "paths.upload":
			config.Paths.Upload = o.FieldValue
		case "paths.takeupload":
			config.Paths.TakeUpload = o.FieldValue
		case "paths.browse":
			config.Paths.Browse = o.FieldValue
		case "paths.detail":
			config.Paths.Detail = o.FieldValue
		default:
			if strings.HasPrefix(o.FieldPath, "publish.form_fields.") {
				key := strings.TrimPrefix(o.FieldPath, "publish.form_fields.")
				if config.Publish.FormFields == nil {
					config.Publish.FormFields = make(map[string]string)
				}
				config.Publish.FormFields[key] = o.FieldValue
			}
		}
	}
}

func (p *Provider) GetSiteInfoByURL(ctx context.Context, baseURL string) (*model.SiteInfo, error) {
	var site model.Site
	if err := p.db.WithContext(ctx).Where("base_url = ?", baseURL).First(&site).Error; err != nil {
		return nil, &model.AppError{Code: 12001, Message: "站点不存在: " + baseURL}
	}
	return siteToInfo(&site), nil
}

func (p *Provider) DetectFramework(ctx context.Context, domain string) (*model.DetectResult, error) {
	site := p.repo.GetByDomain(ctx, domain)
	if site == nil {
		return nil, &model.AppError{Code: 12001, Message: "站点不存在: " + domain}
	}

	defs, ok := adapter.FrameworkDefaults[site.Framework]
	if !ok {
		defs = adapter.FrameworkDefaults["generic"]
	}

	return &model.DetectResult{
		Framework:       site.Framework,
		Confidence:      1.0,
		DetectionDetail: "使用已配置的框架",
		Defaults: model.FrameworkDefaults{
			HashStrategy:        defs.HashStrategy,
			SizeStrategy:        defs.SizeStrategy,
			IDStrategy:          defs.IDStrategy,
			IDPattern:           defs.IDPattern,
			DownloadURLTemplate: defs.DownloadURLTemplate,
		},
	}, nil
}

func (p *Provider) InvalidateAdapter(domain string) {
	p.mu.Lock()
	delete(p.adapters, domain)
	p.mu.Unlock()
}

func siteToInfo(s *model.Site) *model.SiteInfo {
	return &model.SiteInfo{
		Name:                s.Name,
		BaseURL:             s.BaseURL,
		Framework:           s.Framework,
		Enabled:             s.Enabled,
		Passkey:             s.Passkey,
		Cookie:              s.Cookie,
		APIKey:              s.APIKey,
		BearerToken:         s.BearerToken,
		AuthKey:             s.AuthKey,
		AuthHash:            s.AuthHash,
		UserID:              s.UserID,
		HashStrategy:        s.HashStrategy,
		SizeStrategy:        s.SizeStrategy,
		IDStrategy:          s.IDStrategy,
		IDPattern:           s.IDPattern,
		RSSKey:              s.RSSKey,
		HashXMLTagName:      s.HashXMLTagName,
		SizeXMLTagName:      s.SizeXMLTagName,
		HashURLParamName:    s.HashURLParamName,
		SizeDescRegex:       s.SizeDescRegex,
		SizeTitleRegex:      s.SizeTitleRegex,
		SizeBaseUnit:        s.SizeBaseUnit,
		DownloadMode:        s.DownloadMode,
		DownloadURLTemplate: s.DownloadURLTemplate,
		DownloadPagePattern: s.DownloadPagePattern,
		ProxyURL:            s.ProxyURL,
		SkipSSLVerify:       s.SkipSSLVerify,
	}
}

func siteToConfig(s *model.Site) *model.SiteConfig {
	defs, ok := adapter.FrameworkDefaults[s.Framework]
	if !ok {
		defs = adapter.FrameworkDefaults["generic"]
	}

	paths := defaultPaths(s.Framework)
	pub := defaultPublishConfig(s.Framework)

	cfg := &model.SiteConfig{
		SiteDefault: model.SiteDefault{
			Domain:    s.Domain,
			Framework: s.Framework,
			Auth: model.SiteAuthConfig{
				DownloadMode: s.DownloadMode,
			},
			Paths:   paths,
			Publish: pub,
			RSS: model.SiteRSSConfig{
				HashStrategy: model.HashStrategy(defs.HashStrategy),
				SizeStrategy: model.SizeStrategy(defs.SizeStrategy),
				IDStrategy:   model.IDStrategy(defs.IDStrategy),
				IDPattern:    defs.IDPattern,
				URLTemplate:  defs.DownloadURLTemplate,
			},
		},
		Domain:      s.Domain,
		BaseURL:     s.BaseURL,
		Enabled:     s.Enabled,
		IsSource:    s.IsSource,
		IsTarget:    s.IsTarget,
		Passkey:     s.Passkey,
		Cookie:      s.Cookie,
		APIKey:      s.APIKey,
		AuthKey:     s.AuthKey,
		AuthHash:    s.AuthHash,
		UserID:      s.UserID,
		RSSKey:      s.RSSKey,
		BearerToken: s.BearerToken,

		ProxyURL:      s.ProxyURL,
		SkipSSLVerify: s.SkipSSLVerify,
		HRStrategy:    s.HRStrategy,

		DownloadURLTemplate:   s.DownloadURLTemplate,
		DownloadMode:          s.DownloadMode,
		SupportsPiecesHashAPI: s.SupportsPiecesHashAPI,
	}

	if s.AlternativeDomains != "" {
		var alts []string
		if json.Unmarshal([]byte(s.AlternativeDomains), &alts) == nil {
			cfg.AlternativeDomains = alts
		}
	}

	return cfg
}

func defaultPaths(framework string) model.SitePathsConfig {
	switch framework {
	case "nexusphp":
		return model.SitePathsConfig{
			Browse:     "/browse.php",
			Detail:     "/details.php",
			Upload:     "/upload.php",
			TakeUpload: "/takeupload.php",
			RSS:        "/torrss.php",
		}
	case "unit3d":
		return model.SitePathsConfig{
			Browse: "/torrents",
			Detail: "/torrents/{id}",
			Upload: "/upload",
			RSS:    "/rss",
		}
	case "gazelle":
		return model.SitePathsConfig{
			Browse: "/torrents.php",
			Detail: "/torrents.php?torrentid={id}",
			Upload: "/upload.php",
			RSS:    "/feeds.php",
		}
	case "tnode":
		return model.SitePathsConfig{
			Browse: "/torrent/list",
			Detail: "/torrent/info/{id}",
			Upload: "/api/torrent/upload",
			RSS:    "/rss",
		}
	case "mteam":
		return model.SitePathsConfig{
			Browse:     "/browse",
			Detail:     "/detail/{id}",
			Upload:     "/upload.php",
			TakeUpload: "/upload_music.php",
			RSS:        "/api/rss",
		}
	case "rousi":
		return model.SitePathsConfig{
			Browse: "/api/v1/torrents",
			Detail: "/api/v1/torrents/{uuid}",
			Upload: "/api/v1/torrents",
			RSS:    "/rss",
		}
	case "yemapt":
		return model.SitePathsConfig{
			Browse: "/torrent/list",
			Detail: "/torrent/detail/{id}",
			Upload: "/api/torrent/upload",
			RSS:    "/rss",
		}
	default:
		return model.SitePathsConfig{
			Browse: "/browse.php",
			Detail: "/details.php",
			Upload: "/upload.php",
			RSS:    "/rss",
		}
	}
}

func defaultPublishConfig(framework string) model.SitePublishFullConfig {
	switch framework {
	case "mteam":
		return model.SitePublishFullConfig{
			FormFields: map[string]string{
				"category": "category",
				"source":   "source",
				"medium":   "medium",
				"standard": "standard",
				"codec":    "videoCodec",
				"team":     "team",
			},
		}
	case "unit3d":
		return model.SitePublishFullConfig{
			FormFields: map[string]string{
				"category":   "category_id",
				"resolution": "resolution_id",
				"source":     "type_id",
				"codec":      "codec_id",
			},
		}
	case "nexusphp":
		return model.SitePublishFullConfig{
			FormFields: map[string]string{
				"category":      "type",
				"source":        "source_sel",
				"resolution":    "standard_sel",
				"codec":         "codec_sel",
				"audioCodec":    "audiocodec_sel",
				"medium":        "medium_sel",
				"team":          "team_sel",
				"processing":    "processing_sel",
				"music_artist":  "artists",
				"music_album":   "album",
				"music_year":    "year",
				"music_format":  "format_type",
				"music_medium":  "medium_type",
				"music_publish": "publish_type",
			},
		}
	case "gazelle":
		return model.SitePublishFullConfig{
			FormFields: map[string]string{
				"category":   "releasetype",
				"medium":     "media",
				"codec":      "format",
				"audioCodec": "bitrate",
			},
		}
	case "rousi":
		return model.SitePublishFullConfig{
			Description: model.SiteDescConfig{
				Format: "markdown",
			},
		}
	default:
		return model.SitePublishFullConfig{}
	}
}
