package api

import (
	"context"
	"net/http"

	"github.com/ranfish/pt-forward/internal/adapter"
	"github.com/ranfish/pt-forward/internal/auth"
	"github.com/ranfish/pt-forward/internal/client"
	"github.com/ranfish/pt-forward/internal/compliance"
	"github.com/ranfish/pt-forward/internal/coverage"
	"github.com/ranfish/pt-forward/internal/filter"
	"github.com/ranfish/pt-forward/internal/imagehost"
	"github.com/ranfish/pt-forward/internal/metadata"
	"github.com/ranfish/pt-forward/internal/middleware"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/notification"
	"github.com/ranfish/pt-forward/internal/orphan"
	"github.com/ranfish/pt-forward/internal/publish"
	"github.com/ranfish/pt-forward/internal/reseed"
	"github.com/ranfish/pt-forward/internal/rss"
	"github.com/ranfish/pt-forward/internal/scheduler"
	"github.com/ranfish/pt-forward/internal/seeding"
	"github.com/ranfish/pt-forward/internal/setting"
	"github.com/ranfish/pt-forward/internal/site"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Router struct {
	authHandler          *AuthHandler
	clientHandler        *ClientHandler
	siteHandler          *SiteHandler
	rssHandler           *RSSHandler
	filterHandler        *FilterHandler
	notifyHandler        *NotifyHandler
	cookiecloudServer     http.Handler
	settingsHandler      *SettingsHandler
	seedingHandler       *SeedingHandler
	deleteRuleHandler    *DeleteRuleHandler
	reseedHandler        *ReseedHandler
	publishHandler       *PublishHandler
	manualForwardHandler *ManualForwardHandler
	publishTorrentsHandler *PublishTorrentsHandler
	complianceHandler    *ComplianceHandler
	dashboardHandler     *DashboardHandler
	systemHandler        *SystemHandler
	iyuuHandler          *IYUUHandler
	cloudFPHandler       *CloudFPHandler
	fingerprintHandler   *FingerprintHandler
	trackerHandler       *TrackerHandler
	lifecycleHandler     *LifecycleHandler
	cookiecloudHandler   *CookieCloudHandler
	ptgenHandler         *PTGenHandler
	schedulerHandler     *SchedulerHandler
	supportedSitesHandler *SupportedSitesHandler
	downloadHandler       *DownloadHandler
	publishLimitHandler   *PublishLimitHandler
	imageHostHandler      *ImageHostHandler
	metadataHandler       *MetadataHandler
	sseLogHandler         *SSELogHandler
	orphanHandler         *OrphanHandler
	debugSiteProvider     *site.Provider
	logBroadcaster        *LogBroadcaster
	wsHandler            *WSHandler
	hub                  *Hub
	authManager          *auth.AuthManager
	logger               *zap.Logger
	corsMW               func(http.Handler) http.Handler
	recoveryMW           func(http.Handler) http.Handler
	secMW                func(http.Handler) http.Handler
	authMW               func(http.Handler) http.Handler
	rateLimitMW          func(http.Handler) http.Handler
	publicRateLimitMW    func(http.Handler) http.Handler
	rateLimitCfg         middleware.RateLimitConfigFunc
}

func NewRouter(authManager *auth.AuthManager, db *gorm.DB, rssEngine *rss.Engine, notifyService *notification.Service, reseedEngine *reseed.Engine, publishPipeline *publish.Pipeline, seedingEngine *seeding.Engine, clientMgr *client.Manager, taskRegistry *scheduler.Registry, iyuuSvc IYUUQueryService, appVersion string, hub *Hub, imageHostMgr *imagehost.Manager, logBroadcaster *LogBroadcaster, logger *zap.Logger) *Router {
	siteRepo := site.NewRepository(db)
	rssRepo := rss.NewRepository(db)
	filterRepo := filter.NewRepository(db)
	filterEng := filter.NewEngine(filterRepo, logger)
	notifyRepo := notification.NewRepository(db)
	settingsRepo := setting.NewRepository(db)
	if hub == nil {
		hub = NewHub()
	}
	var dashChecker clientOnlineChecker
	if clientMgr != nil {
		dashChecker = clientMgr
	}
	adapterFactory := adapter.NewFactory(logger, nil) // router 内部用于 StatsSync，不需要 Engine
	siteHandler := NewSiteHandler(siteRepo, logger, db)
	siteHandler.SetStatsSync(site.NewStatsSyncService(db, adapterFactory, logger))
	var clientMgrIface ClientManager
	if clientMgr != nil {
		clientMgrIface = clientMgr
	}
	dashHandler := NewDashboardHandler(db, logger, appVersion, dashChecker)
	sysHandler := NewSystemHandler(appVersion, db, clientMgr, logger)
	if seedingEngine != nil {
		dashHandler.SetSeedingEngine(seedingEngine)
		sysHandler.SetSeedingEngine(seedingEngine)
	}
	manualForwardHandler := NewManualForwardHandler(db, logger)
	if seedingEngine != nil {
		manualForwardHandler.SetSeedingCache(seedingEngine)
	}
	rt := &Router{
		authHandler:          NewAuthHandler(authManager),
		clientHandler:        NewClientHandler(db, logger, clientMgrIface),
		siteHandler:          siteHandler,
		rssHandler:           NewRSSHandler(rssRepo, rssEngine, db, logger),
		filterHandler:        NewFilterHandler(filterRepo, filterEng, db, logger),
		notifyHandler:        NewNotifyHandler(notifyRepo, notifyService, logger),
		settingsHandler:      NewSettingsHandler(settingsRepo, logger),
		seedingHandler:       NewSeedingHandler(db, logger, seedingEngine),
		deleteRuleHandler:    NewDeleteRuleHandler(db, logger, clientMgrIface),
		reseedHandler:        NewReseedHandler(reseedEngine, logger),
		publishHandler:         NewPublishHandler(publishPipeline, logger, db),
		manualForwardHandler:   manualForwardHandler,
		publishTorrentsHandler: NewPublishTorrentsHandler(db, logger),
		complianceHandler:      NewComplianceHandler(db, logger),		publishLimitHandler:   NewPublishLimitHandler(db, logger),
		imageHostHandler:      NewImageHostHandler(imageHostMgr, settingsRepo, logger),
		metadataHandler:       NewMetadataHandler(db, logger),
		logBroadcaster:        logBroadcaster,
		sseLogHandler:         NewSSELogHandler(logBroadcaster, logger),
		dashboardHandler:     dashHandler,
		systemHandler:        sysHandler,
		iyuuHandler:          NewIYUUHandler(db, logger, iyuuSvc),
		cloudFPHandler:       NewCloudFPHandler(db, logger),
		fingerprintHandler:   NewFingerprintHandler(db, logger),
		trackerHandler:       NewTrackerHandler(db, logger),
		lifecycleHandler:     NewLifecycleHandler(db, logger),
		cookiecloudHandler:   NewCookieCloudHandler(db, logger),
		ptgenHandler:         NewPTGenHandler(db, logger),
		schedulerHandler:     NewSchedulerHandler(taskRegistry, db, logger),
		supportedSitesHandler: NewSupportedSitesHandler(logger),
		downloadHandler:       NewDownloadHandler(db, clientMgr, logger),
		wsHandler:            NewWSHandler(hub, authManager, nil),
		hub:                  hub,
		authManager:          authManager,
		logger:               logger,
	}
	rt.publishTorrentsHandler.SetReseedEngine(reseedEngine)
	return rt
}

func (rt *Router) Register(mux *http.ServeMux, corsOrigins []string, rateLimitEnabled bool, rateLimitGlobal int) {
	rt.RegisterWithEndpointLimits(mux, corsOrigins, rateLimitEnabled, rateLimitGlobal, 0, 0)
}

func (rt *Router) Start(_ context.Context) {}

func (rt *Router) Stop() {
	if rt.manualForwardHandler != nil {
		rt.manualForwardHandler.Close()
	}
}

func (rt *Router) SetCloudFPBreakerFn(fn func() bool) {
	if rt.cloudFPHandler != nil {
		rt.cloudFPHandler.SetBreakerFn(fn)
	}
}

// SetRateLimitConfig 注入动态限流配置回调（§56.36 热更新）。
// 必须在 Register 之前调用。
func (rt *Router) SetRateLimitConfig(cfg middleware.RateLimitConfigFunc) {
	rt.rateLimitCfg = cfg
}


// SetupManualForward 注入手动转发向导所需的依赖
func (rt *Router) SetupManualForward(pipeline *publish.Pipeline, siteProvider *site.Provider, clientMgr *client.Manager, declFilter *publish.DeclarationFilter, bdinfoScanner *publish.BDInfoScanner, metadataFetcher *metadata.Fetcher, coverageSvc *coverage.Service, sourceDetector *publish.SourceSiteDetector, complianceChecker *compliance.Checker, imageHostMgr *imagehost.Manager) {
	rt.manualForwardHandler.SetPipeline(pipeline)
	rt.manualForwardHandler.SetSiteManager(siteProvider)
	rt.manualForwardHandler.SetClientProvider(clientMgr)
	rt.manualForwardHandler.SetDeclarationFilter(declFilter)
	rt.manualForwardHandler.SetBDInfoScanner(bdinfoScanner)
	if metadataFetcher != nil {
		rt.manualForwardHandler.SetMetadataFetcher(metadataFetcher)
	}
	if coverageSvc != nil {
		rt.manualForwardHandler.SetCoverageService(coverageSvc)
	}
	if sourceDetector != nil {
		rt.manualForwardHandler.SetSourceDetector(sourceDetector)
	}
	if complianceChecker != nil {
		rt.manualForwardHandler.SetComplianceChecker(complianceChecker)
	}
	if imageHostMgr != nil {
		rt.manualForwardHandler.SetImageHostManager(imageHostMgr)
	}
	rt.publishTorrentsHandler.SetClientProvider(clientMgr)
	rt.publishTorrentsHandler.SetSiteProvider(siteProvider)
	// §59.38: 观察期定时清理（日级，7 天滞后期；ctx 自管理，进程退出即止）
	rt.publishTorrentsHandler.StartObservingCleanup()
	rt.publishTorrentsHandler.SetDeclarationFilter(declFilter)
	if metadataFetcher != nil {
		rt.publishTorrentsHandler.SetMetadataFetcher(metadataFetcher)
	}
	if complianceChecker != nil {
		rt.publishTorrentsHandler.SetComplianceChecker(complianceChecker)
	}
	if pipeline != nil {
		rt.publishTorrentsHandler.SetSeedPipeline(pipeline)
	// §59.42: PTGen 海报 fallback 链
	rt.publishTorrentsHandler.SetPTGenAnalyzer(pipeline)
	}
}

func (rt *Router) SetCookieCloudServer(srv http.Handler) {
	rt.cookiecloudServer = srv
}

func (rt *Router) SetupOrphan(scanner *orphan.Scanner, recovery *orphan.Recovery, db *gorm.DB) {
	rt.orphanHandler = NewOrphanHandler(scanner, recovery, db, rt.authManager, rt.logger)
}

func (rt *Router) SetupDebug(siteProvider *site.Provider) {
	rt.debugSiteProvider = siteProvider
}

// SetupCompliance §56.39: 注入 compliance.Checker（用于 CRUD 后 InvalidateCache）。
func (rt *Router) SetupCompliance(checker *compliance.Checker) {
	if rt.complianceHandler != nil && checker != nil {
		rt.complianceHandler.SetChecker(checker)
	}
}

func (rt *Router) SetupPublishTorrents(coverageSvc *coverage.Service, clientMgr *client.Manager, sourceDetector *publish.SourceSiteDetector) {
	rt.publishTorrentsHandler.SetCoverageService(coverageSvc)
	rt.publishTorrentsHandler.SetClientProvider(clientMgr)
	rt.publishTorrentsHandler.SetSourceDetector(sourceDetector)
	rt.siteHandler.SetSourceDetector(sourceDetector)
}

func (rt *Router) StartCoverageRefresh(scheduler *scheduler.Registry) error {
	return scheduler.Register("coverage-refresh", "coverage", "0 */12 * * *", rt.publishTorrentsHandler.ScheduledRefresh)
}

func (rt *Router) SetSiteProvider(p interface {
	GetAdapter(ctx context.Context, domain string) (model.SiteAdapter, error)
	GetSiteConfig(ctx context.Context, domain string) (*model.SiteConfig, error)
}) {
	rt.siteHandler.SetProvider(p)
}

func (rt *Router) SetConfigEventBus(bus *rss.ConfigEventBus) {
	rt.settingsHandler.SetConfigEventBus(bus)
}

func (rt *Router) RegisterWithEndpointLimits(mux *http.ServeMux, corsOrigins []string, rateLimitEnabled bool, rateLimitGlobal, rateLimitWrite, rateLimitDownload int) {
	rt.corsMW = middleware.CORS(corsOrigins)
	rt.recoveryMW = middleware.Recovery(rt.logger)
	rt.secMW = middleware.SecurityHeaders
	rt.authMW = middleware.JWTAuth(rt.authManager)

	if rateLimitEnabled && rateLimitGlobal > 0 {
		defaultGlobal := rateLimitGlobal
		if defaultGlobal <= 0 {
			defaultGlobal = 600
		}
		// §56.36: DynamicRateLimit 热更新——修改 system_settings 后立即生效
		if rt.rateLimitCfg != nil {
			rt.rateLimitMW = middleware.DynamicRateLimit(rt.rateLimitCfg, defaultGlobal, 60)
		} else {
			rt.rateLimitMW = middleware.RateLimit(defaultGlobal, 60)
		}
	} else {
		rt.rateLimitMW = func(next http.Handler) http.Handler { return next }
	}

	var publicRateLimitMW func(http.Handler) http.Handler
	if rateLimitEnabled {
		publicRL := rateLimitGlobal
		if publicRL <= 0 || publicRL > 30 {
			publicRL = 30
		}
		publicRateLimitMW = middleware.RateLimit(publicRL, 60)
	} else {
		publicRateLimitMW = func(next http.Handler) http.Handler { return next }
	}
	rt.publicRateLimitMW = publicRateLimitMW

	var writeLimitMW func(http.Handler) http.Handler
	if rateLimitEnabled && rateLimitWrite > 0 {
		writeLimitMW = middleware.RateLimit(rateLimitWrite, 60)
	} else {
		writeLimitMW = rt.rateLimitMW
	}

	var downloadLimitMW func(http.Handler) http.Handler
	if rateLimitEnabled && rateLimitDownload > 0 {
		downloadLimitMW = middleware.RateLimit(rateLimitDownload, 60)
	} else {
		downloadLimitMW = rt.rateLimitMW
	}

	rt.wsHandler = NewWSHandler(rt.hub, rt.authManager, corsOrigins)

	mux.HandleFunc("/api/v1/ws", rt.wsHandler.ServeHTTP)

	mux.HandleFunc("/api/v1/auth/login", rt.public(rt.authHandler.HandleLogin))
	mux.HandleFunc("/api/v1/auth/setup", rt.public(rt.authHandler.HandleSetup))
	mux.HandleFunc("/api/v1/auth/status", rt.public(rt.authHandler.HandleStatus))
	mux.HandleFunc("/api/v1/auth/refresh", rt.public(rt.authHandler.HandleRefresh))
	mux.HandleFunc("/api/v1/system/ping", rt.public(rt.systemHandler.HandlePing))

	mux.Handle("/api/v1/auth/password", rt.protected(rt.authHandler.HandlePassword))
	mux.Handle("/api/v1/auth/profile", rt.protected(rt.authHandler.HandleProfile))

	dlHandler := rt.chain(downloadLimitMW, rt.clientHandler.ServeHTTP)
	mux.Handle("/api/v1/downloaders", dlHandler)
	mux.Handle("/api/v1/downloaders/", dlHandler)

	publishTargetHandler := rt.chain(rt.rateLimitMW, rt.clientHandler.handlePublishTargets)
	mux.Handle("/api/v1/downloaders/publish-targets", publishTargetHandler)

	siteHandler := rt.chain(rt.rateLimitMW, rt.siteHandler.ServeHTTP)
	mux.Handle("/api/v1/sites", siteHandler)
	mux.Handle("/api/v1/sites/", siteHandler)

	freezeStatusHandler := rt.chain(rt.rateLimitMW, rt.siteHandler.handleFreezeStatus)
	mux.Handle("/api/v1/httpclient/freeze-status", freezeStatusHandler)

	circuitStatusHandler := rt.chain(rt.rateLimitMW, rt.siteHandler.handleCircuitStatus)
	mux.Handle("/api/v1/httpclient/circuit-status", circuitStatusHandler)

	exclusionHandler := rt.chain(rt.rateLimitMW, rt.siteHandler.handleExclusions)
	mux.Handle("/api/v1/publish/exclusions", exclusionHandler)

	complianceHandler := rt.chain(rt.rateLimitMW, rt.complianceHandler.ServeHTTP)
	mux.Handle("/api/v1/compliance/rules", complianceHandler)
	mux.Handle("/api/v1/compliance/rules/", complianceHandler)
	mux.Handle("/api/v1/compliance/test", complianceHandler)

	publishLimitHandler := rt.chain(rt.rateLimitMW, rt.publishLimitHandler.ServeHTTP)
	mux.Handle("/api/v1/publish/limits", publishLimitHandler)
	mux.Handle("/api/v1/publish/limits/", publishLimitHandler)

	imageHostHandler := rt.chain(rt.rateLimitMW, rt.imageHostHandler.ServeHTTP)
	mux.Handle("/api/v1/settings/image-host", imageHostHandler)
	mux.Handle("/api/v1/settings/image-host/", imageHostHandler)

	metadataHandler := rt.chain(rt.rateLimitMW, rt.metadataHandler.ServeHTTP)
	mux.Handle("/api/v1/metadata", metadataHandler)
	mux.Handle("/api/v1/metadata/", metadataHandler)

	rssHandler := rt.chain(rt.rateLimitMW, rt.rssHandler.ServeHTTP)
	mux.Handle("/api/v1/rss/subscriptions", rssHandler)
	mux.Handle("/api/v1/rss/subscriptions/", rssHandler)

	filterHandler := rt.chain(rt.rateLimitMW, rt.filterHandler.ServeHTTP)
	mux.Handle("/api/v1/filters/rules", filterHandler)
	mux.Handle("/api/v1/filters/rules/", filterHandler)

	notifyHandler := rt.chain(rt.rateLimitMW, rt.notifyHandler.ServeHTTP)
	mux.Handle("/api/v1/notifications/channels", notifyHandler)
	mux.Handle("/api/v1/notifications/channels/", notifyHandler)

	settingsHandler := rt.chain(rt.rateLimitMW, rt.settingsHandler.ServeHTTP)
	mux.Handle("/api/v1/settings", settingsHandler)
	mux.Handle("/api/v1/settings/", settingsHandler)

	seedingHandler := rt.chain(rt.rateLimitMW, rt.seedingHandler.ServeHTTP)
	mux.Handle("/api/v1/seeding/configs", seedingHandler)
	mux.Handle("/api/v1/seeding/configs/", seedingHandler)
	mux.Handle("/api/v1/seeding/records", seedingHandler)
	mux.Handle("/api/v1/seeding/records/", seedingHandler)
	mux.Handle("/api/v1/seeding/stats", seedingHandler)
	mux.Handle("/api/v1/seeding/stats/", seedingHandler)
	mux.Handle("/api/v1/seeding/scoring-dryrun", seedingHandler)
	mux.Handle("/api/v1/seeding/scoring-dryrun/", seedingHandler)
	mux.Handle("/api/v1/seeding/status", seedingHandler)
	mux.Handle("/api/v1/seeding/status/", seedingHandler)
	mux.Handle("/api/v1/seeding/free-wait-queue", seedingHandler)
	mux.Handle("/api/v1/seeding/free-wait-queue/", seedingHandler)
	mux.Handle("/api/v1/seeding/torrents", seedingHandler)
	mux.Handle("/api/v1/seeding/torrents/", seedingHandler)
	mux.Handle("/api/v1/seeding/clients", seedingHandler)
	mux.Handle("/api/v1/seeding/clients/", seedingHandler)
	mux.Handle("/api/v1/seeding/history", seedingHandler)
	mux.Handle("/api/v1/seeding/history/", seedingHandler)
	mux.Handle("/api/v1/seeding/scoring-config", seedingHandler)
	mux.Handle("/api/v1/seeding/scoring-config/", seedingHandler)
	mux.Handle("/api/v1/seeding/scoring-logs", seedingHandler)
	mux.Handle("/api/v1/seeding/scoring-logs/", seedingHandler)
	mux.Handle("/api/v1/seeding/unregistered-keywords", seedingHandler)
	mux.Handle("/api/v1/seeding/unregistered-keywords/", seedingHandler)
	mux.Handle("/api/v1/seeding/dryrun", seedingHandler)
	mux.Handle("/api/v1/seeding/dryrun/", seedingHandler)

	deleteRuleHandler := rt.chain(rt.rateLimitMW, rt.deleteRuleHandler.ServeHTTP)
	mux.Handle("/api/v1/seeding/delete-rules", deleteRuleHandler)
	mux.Handle("/api/v1/seeding/delete-rules/", deleteRuleHandler)
	mux.Handle("/api/v1/seeding/rules", deleteRuleHandler)
	mux.Handle("/api/v1/seeding/rules/", deleteRuleHandler)

	reseedHandler := rt.chain(rt.rateLimitMW, rt.reseedHandler.ServeHTTP)
	mux.Handle("/api/v1/reseed/tasks", reseedHandler)
	mux.Handle("/api/v1/reseed/tasks/", reseedHandler)

	publishHandler := rt.chain(writeLimitMW, rt.publishHandler.ServeHTTP)
	mux.Handle("/api/v1/publish/tasks", publishHandler)
	mux.Handle("/api/v1/publish/tasks/", publishHandler)
	mux.Handle("/api/v1/publish/candidates", publishHandler)
	mux.Handle("/api/v1/publish/candidates/", publishHandler)
	mux.Handle("/api/v1/publish/results", publishHandler)
	mux.Handle("/api/v1/publish/results/", publishHandler)
	mux.Handle("/api/v1/publish/groups", publishHandler)
	mux.Handle("/api/v1/publish/groups/", publishHandler)

	mfHandler := rt.chain(writeLimitMW, rt.manualForwardHandler.ServeHTTP)
	mux.Handle("/api/v1/manual-forward/seeded-torrents", mfHandler)
	mux.Handle("/api/v1/manual-forward/seeded-torrents/", mfHandler)
	mux.Handle("/api/v1/manual-forward/analyze", mfHandler)
	mux.Handle("/api/v1/manual-forward/analyze/", mfHandler)
	mux.Handle("/api/v1/manual-forward/eligible-targets", mfHandler)
	mux.Handle("/api/v1/manual-forward/eligible-targets/", mfHandler)
	mux.Handle("/api/v1/manual-forward/submit", mfHandler)
	mux.Handle("/api/v1/manual-forward/submit/", mfHandler)
	mux.Handle("/api/v1/manual-forward/batch-submit", mfHandler)
	mux.Handle("/api/v1/manual-forward/batch-submit/", mfHandler)
	mux.Handle("/api/v1/manual-forward/merge", mfHandler)
	mux.Handle("/api/v1/manual-forward/merge/", mfHandler)
	mux.Handle("/api/v1/manual-forward/preview", mfHandler)
	mux.Handle("/api/v1/manual-forward/preview/", mfHandler)
	mux.Handle("/api/v1/manual-forward/refresh", mfHandler)
	mux.Handle("/api/v1/manual-forward/refresh/", mfHandler)
	// §59.51: 后台截图任务 + 轮询（长任务脱离 HTTP 请求生命周期）
	mux.Handle("/api/v1/manual-forward/screenshot-capture", mfHandler)
	mux.Handle("/api/v1/manual-forward/screenshot-capture-progress", mfHandler)
	mux.Handle("/api/v1/manual-forward/parse-title", mfHandler)

	if rt.orphanHandler != nil {
		orphanH := rt.chain(rt.rateLimitMW, rt.orphanHandler.ServeHTTP)
		mux.Handle("/api/v1/orphans", orphanH)
		mux.Handle("/api/v1/orphans/", orphanH)
	}

	if rt.debugSiteProvider != nil {
		debugH := rt.protected(NewDebugSearchHandler(rt.debugSiteProvider))
		mux.Handle("/api/v1/debug/search", debugH)
	}

	ptHandler := rt.chain(writeLimitMW, rt.publishTorrentsHandler.ServeHTTP)
	mux.Handle("/api/v1/publish/torrents", ptHandler)
	mux.Handle("/api/v1/publish/torrents/", ptHandler)
	mux.Handle("/api/v1/publish/cached-sites", ptHandler)
	mux.Handle("/api/v1/publish/cached-sites/", ptHandler)
	mux.Handle("/api/v1/publish/seed-data", ptHandler)
	mux.Handle("/api/v1/publish/seed-data/", ptHandler)
	mux.Handle("/api/v1/publish/stats", ptHandler)
	mux.Handle("/api/v1/publish/stats/", ptHandler)
	mux.Handle("/api/v1/publish/coverage-cache", ptHandler)
	mux.Handle("/api/v1/publish/coverage-cache/", ptHandler)
	mux.Handle("/api/v1/publish/source-priority", ptHandler)
	mux.Handle("/api/v1/publish/fetch-priority", ptHandler)
	mux.Handle("/api/v1/publish/seeds", ptHandler)
	mux.Handle("/api/v1/publish/seeds/", ptHandler)

	dashboardHandler := rt.chain(rt.rateLimitMW, rt.dashboardHandler.ServeHTTP)
	mux.Handle("/api/v1/dashboard/overview", dashboardHandler)
	mux.Handle("/api/v1/dashboard/overview/", dashboardHandler)
	mux.Handle("/api/v1/dashboard/activities", dashboardHandler)
	mux.Handle("/api/v1/dashboard/activities/", dashboardHandler)
	mux.Handle("/api/v1/dashboard/trends", dashboardHandler)
	mux.Handle("/api/v1/dashboard/trends/", dashboardHandler)
	mux.Handle("/api/v1/system/dashboard", dashboardHandler)
	mux.Handle("/api/v1/stats/traffic/hourly", dashboardHandler)
	mux.Handle("/api/v1/seeding/monitor", dashboardHandler)
	mux.Handle("/api/v1/reseed/monitor", dashboardHandler)
	mux.Handle("/api/v1/publish/monitor", dashboardHandler)
	mux.Handle("/api/v1/system/tasks/action", dashboardHandler)

	systemHandler := rt.chain(rt.rateLimitMW, rt.systemHandler.ServeHTTP)
	publicSystemHandler := rt.public(rt.systemHandler.ServeHTTP)
	mux.Handle("/api/v1/system/info", systemHandler)
	mux.Handle("/api/v1/system/info/", systemHandler)
	mux.Handle("/api/v1/system/logs", systemHandler)
	mux.Handle("/api/v1/system/logs/", systemHandler)

	sseLogHandler := rt.chain(rt.rateLimitMW, rt.sseLogHandler.ServeHTTP)
	mux.Handle("/api/v1/system/logs/stream", sseLogHandler)
	mux.Handle("/api/v1/system/audit-logs", systemHandler)
	mux.Handle("/api/v1/system/audit-logs/", systemHandler)
	mux.Handle("/api/v1/system/check-update", systemHandler)
	mux.Handle("/api/v1/system/check-update/", systemHandler)
	mux.Handle("/api/v1/system/update", systemHandler)
	mux.Handle("/api/v1/system/update/", systemHandler)
	mux.Handle("/api/v1/system/encryption-key", systemHandler)
	mux.Handle("/api/v1/system/encryption-key/", systemHandler)
	mux.Handle("/api/v1/system/health", publicSystemHandler)
	mux.Handle("/api/v1/system/health/", publicSystemHandler)

	if rt.cookiecloudServer != nil {
		mux.Handle("/cookiecloud/update", rt.cookiecloudServer)
		mux.Handle("/cookiecloud/update/", rt.cookiecloudServer)
		mux.Handle("/cookiecloud/get/", rt.cookiecloudServer)
		mux.Handle("/cookiecloud/get", rt.cookiecloudServer)
	}

	torrentEventHandler := rt.chain(rt.rateLimitMW, rt.dashboardHandler.ServeHTTP)
	mux.Handle("/api/v1/torrent-events", torrentEventHandler)
	mux.Handle("/api/v1/torrent-events/", torrentEventHandler)

	iyuuHandler := rt.chain(rt.rateLimitMW, rt.iyuuHandler.ServeHTTP)
	mux.Handle("/api/v1/iyuu/config", iyuuHandler)
	mux.Handle("/api/v1/iyuu/config/", iyuuHandler)
	mux.Handle("/api/v1/iyuu/sites", iyuuHandler)
	mux.Handle("/api/v1/iyuu/sites/", iyuuHandler)
	mux.Handle("/api/v1/iyuu/query", iyuuHandler)
	mux.Handle("/api/v1/iyuu/query/", iyuuHandler)
	mux.Handle("/api/v1/iyuu/test", iyuuHandler)
	mux.Handle("/api/v1/iyuu/test/", iyuuHandler)
	mux.Handle("/api/v1/iyuu/status", iyuuHandler)
	mux.Handle("/api/v1/iyuu/status/", iyuuHandler)
	mux.Handle("/api/v1/iyuu/supported-targets", iyuuHandler)
	mux.Handle("/api/v1/iyuu/supported-targets/", iyuuHandler)

	cloudFPHandler := rt.chain(rt.rateLimitMW, rt.cloudFPHandler.ServeHTTP)
	mux.Handle("/api/v1/cloud-fp/config", cloudFPHandler)
	mux.Handle("/api/v1/cloud-fp/config/", cloudFPHandler)
	mux.Handle("/api/v1/cloud-fp/test", cloudFPHandler)
	mux.Handle("/api/v1/cloud-fp/test/", cloudFPHandler)
	mux.Handle("/api/v1/cloud-fp/status", cloudFPHandler)
	mux.Handle("/api/v1/cloud-fp/status/", cloudFPHandler)

	fingerprintHandler := rt.chain(rt.rateLimitMW, rt.fingerprintHandler.ServeHTTP)
	mux.Handle("/api/v1/fingerprints", fingerprintHandler)
	mux.Handle("/api/v1/fingerprints/", fingerprintHandler)

	trackerHandler := rt.chain(rt.rateLimitMW, rt.trackerHandler.ServeHTTP)
	mux.Handle("/api/v1/tracker/members", trackerHandler)
	mux.Handle("/api/v1/tracker/members/", trackerHandler)
	mux.Handle("/api/v1/tracker/history", trackerHandler)
	mux.Handle("/api/v1/tracker/history/", trackerHandler)

	lifecycleHandler := rt.chain(rt.rateLimitMW, rt.lifecycleHandler.ServeHTTP)
	mux.Handle("/api/v1/lifecycle/config", lifecycleHandler)
	mux.Handle("/api/v1/lifecycle/config/", lifecycleHandler)
	mux.Handle("/api/v1/lifecycle/backpressure", lifecycleHandler)
	mux.Handle("/api/v1/lifecycle/backpressure/", lifecycleHandler)

	cookiecloudHandler := rt.chain(rt.rateLimitMW, rt.cookiecloudHandler.ServeHTTP)
	mux.Handle("/api/v1/cookiecloud/config", cookiecloudHandler)
	mux.Handle("/api/v1/cookiecloud/config/", cookiecloudHandler)
	mux.Handle("/api/v1/cookiecloud/sync", cookiecloudHandler)
	mux.Handle("/api/v1/cookiecloud/sync/", cookiecloudHandler)
	mux.Handle("/api/v1/cookiecloud/history", cookiecloudHandler)
	mux.Handle("/api/v1/cookiecloud/history/", cookiecloudHandler)
	mux.Handle("/api/v1/cookiecloud/test", cookiecloudHandler)
	mux.Handle("/api/v1/cookiecloud/test/", cookiecloudHandler)

	ptgenHandler := rt.chain(rt.rateLimitMW, rt.ptgenHandler.ServeHTTP)
	mux.Handle("/api/v1/ptgen/query", ptgenHandler)
	mux.Handle("/api/v1/ptgen/query/", ptgenHandler)
	mux.Handle("/api/v1/ptgen/cache", ptgenHandler)
	mux.Handle("/api/v1/ptgen/cache/", ptgenHandler)

	schedulerHandler := rt.chain(rt.rateLimitMW, rt.schedulerHandler.ServeHTTP)
	mux.Handle("/api/v1/scheduler/tasks", schedulerHandler)
	mux.Handle("/api/v1/scheduler/tasks/", schedulerHandler)

	supportedSitesHandler := rt.chain(rt.rateLimitMW, rt.supportedSitesHandler.ServeHTTP)
	mux.Handle("/api/v1/supported-sites", supportedSitesHandler)
	mux.Handle("/api/v1/supported-sites/", supportedSitesHandler)

	downloadHandler := rt.chain(rt.rateLimitMW, rt.downloadHandler.ServeHTTP)
	mux.Handle("/api/v1/downloads", downloadHandler)
	mux.Handle("/api/v1/downloads/", downloadHandler)
}

func (rt *Router) public(fn http.HandlerFunc) http.HandlerFunc {
	chain := rt.corsMW(rt.recoveryMW(rt.secMW(middleware.MaxBodySize(rt.publicRateLimitMW(fn)))))
	return chain.ServeHTTP
}

func (rt *Router) protected(fn http.HandlerFunc) http.Handler {
	return rt.corsMW(rt.recoveryMW(rt.rateLimitMW(rt.authMW(rt.secMW(middleware.MaxBodySize(fn))))))
}

func (rt *Router) chain(rlMW func(http.Handler) http.Handler, fn http.HandlerFunc) http.Handler {
	return rt.corsMW(rt.recoveryMW(rlMW(rt.authMW(rt.secMW(middleware.MaxBodySize(fn))))))
}

func (rt *Router) Hub() *Hub {
	return rt.hub
}
