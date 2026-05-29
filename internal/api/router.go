package api

import (
	"context"
	"net/http"

	"github.com/ranfish/pt-forward/internal/adapter"
	"github.com/ranfish/pt-forward/internal/auth"
	"github.com/ranfish/pt-forward/internal/client"
	"github.com/ranfish/pt-forward/internal/filter"
	"github.com/ranfish/pt-forward/internal/middleware"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/notification"
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
	authHandler        *AuthHandler
	clientHandler      *ClientHandler
	siteHandler        *SiteHandler
	rssHandler         *RSSHandler
	filterHandler      *FilterHandler
	notifyHandler      *NotifyHandler
	settingsHandler    *SettingsHandler
	seedingHandler     *SeedingHandler
	deleteRuleHandler  *DeleteRuleHandler
	reseedHandler      *ReseedHandler
	publishHandler      *PublishHandler
	manualForwardHandler *ManualForwardHandler
	dashboardHandler   *DashboardHandler
	systemHandler      *SystemHandler
	iyuuHandler        *IYUUHandler
	fingerprintHandler *FingerprintHandler
	trackerHandler     *TrackerHandler
	lifecycleHandler   *LifecycleHandler
	cookiecloudHandler *CookieCloudHandler
	ptgenHandler       *PTGenHandler
	schedulerHandler   *SchedulerHandler
	wsHandler          *WSHandler
	hub                *Hub
	authManager        *auth.AuthManager
	logger             *zap.Logger
	corsMW             func(http.Handler) http.Handler
	recoveryMW         func(http.Handler) http.Handler
	secMW              func(http.Handler) http.Handler
	authMW             func(http.Handler) http.Handler
	rateLimitMW        func(http.Handler) http.Handler
	publicRateLimitMW  func(http.Handler) http.Handler
}

func NewRouter(authManager *auth.AuthManager, db *gorm.DB, rssEngine *rss.Engine, notifyService *notification.Service, reseedEngine *reseed.Engine, publishPipeline *publish.Pipeline, seedingEngine *seeding.Engine, clientMgr *client.Manager, taskRegistry *scheduler.Registry, iyuuSvc IYUUQueryService, appVersion string, hub *Hub, logger *zap.Logger) *Router {
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
	adapterFactory := adapter.NewFactory(logger)
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
	return &Router{
		authHandler:        NewAuthHandler(authManager),
		clientHandler:      NewClientHandler(db, logger, clientMgrIface),
		siteHandler:        siteHandler,
		rssHandler:         NewRSSHandler(rssRepo, rssEngine, db, logger),
		filterHandler:      NewFilterHandler(filterRepo, filterEng, db, logger),
		notifyHandler:      NewNotifyHandler(notifyRepo, notifyService, logger),
		settingsHandler:    NewSettingsHandler(settingsRepo, logger),
		seedingHandler:     NewSeedingHandler(db, logger, seedingEngine),
		deleteRuleHandler:  NewDeleteRuleHandler(db, logger, clientMgrIface),
		reseedHandler:      NewReseedHandler(reseedEngine, logger),
		publishHandler:       NewPublishHandler(publishPipeline, logger, db),
		manualForwardHandler: NewManualForwardHandler(db, logger),
		dashboardHandler:   dashHandler,
		systemHandler:      sysHandler,
		iyuuHandler:        NewIYUUHandler(db, logger, iyuuSvc),
		fingerprintHandler: NewFingerprintHandler(db, logger),
		trackerHandler:     NewTrackerHandler(db, logger),
		lifecycleHandler:   NewLifecycleHandler(db, logger),
		cookiecloudHandler: NewCookieCloudHandler(db, logger),
		ptgenHandler:       NewPTGenHandler(db, logger),
		schedulerHandler:   NewSchedulerHandler(taskRegistry, db, logger),
		wsHandler:          NewWSHandler(hub, authManager, nil),
		hub:                hub,
		authManager:        authManager,
		logger:             logger,
	}
}

func (rt *Router) Register(mux *http.ServeMux, corsOrigins []string, rateLimitEnabled bool, rateLimitGlobal int) {
	rt.RegisterWithEndpointLimits(mux, corsOrigins, rateLimitEnabled, rateLimitGlobal, 0, 0)
}

func (rt *Router) Start(_ context.Context) {}

func (rt *Router) Stop() {}

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
		rt.rateLimitMW = middleware.RateLimit(rateLimitGlobal, 60)
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
		publicRateLimitMW = middleware.RateLimit(30, 60)
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
	mux.Handle("/api/v1/seeding/torrents", seedingHandler)
	mux.Handle("/api/v1/seeding/torrents/", seedingHandler)
	mux.Handle("/api/v1/seeding/clients", seedingHandler)
	mux.Handle("/api/v1/seeding/clients/", seedingHandler)
	mux.Handle("/api/v1/seeding/scoring-config", seedingHandler)
	mux.Handle("/api/v1/seeding/scoring-config/", seedingHandler)
	mux.Handle("/api/v1/seeding/scoring-logs", seedingHandler)
	mux.Handle("/api/v1/seeding/scoring-logs/", seedingHandler)
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

	dashboardHandler := rt.chain(rt.rateLimitMW, rt.dashboardHandler.ServeHTTP)
	mux.Handle("/api/v1/dashboard/overview", dashboardHandler)
	mux.Handle("/api/v1/dashboard/overview/", dashboardHandler)
	mux.Handle("/api/v1/dashboard/activities", dashboardHandler)
	mux.Handle("/api/v1/dashboard/activities/", dashboardHandler)
	mux.Handle("/api/v1/dashboard/trends", dashboardHandler)
	mux.Handle("/api/v1/dashboard/trends/", dashboardHandler)

	systemHandler := rt.chain(rt.rateLimitMW, rt.systemHandler.ServeHTTP)
	publicSystemHandler := rt.public(rt.systemHandler.ServeHTTP)
	mux.Handle("/api/v1/system/info", systemHandler)
	mux.Handle("/api/v1/system/info/", systemHandler)
	mux.Handle("/api/v1/system/logs", systemHandler)
	mux.Handle("/api/v1/system/logs/", systemHandler)
	mux.Handle("/api/v1/system/health", publicSystemHandler)
	mux.Handle("/api/v1/system/health/", publicSystemHandler)

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
