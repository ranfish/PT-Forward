package api

import (
	"net/http"

	"github.com/ranfish/pt-forward/internal/auth"
	"github.com/ranfish/pt-forward/internal/client"
	"github.com/ranfish/pt-forward/internal/filter"
	"github.com/ranfish/pt-forward/internal/middleware"
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
	publishHandler     *PublishHandler
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
}

func NewRouter(authManager *auth.AuthManager, db *gorm.DB, rssEngine *rss.Engine, notifyService *notification.Service, reseedEngine *reseed.Engine, publishPipeline *publish.Pipeline, seedingEngine *seeding.Engine, clientMgr *client.Manager, taskRegistry *scheduler.Registry, iyuuSvc IYUUQueryService, appVersion string, logger *zap.Logger) *Router {
	siteRepo := site.NewRepository(db)
	rssRepo := rss.NewRepository(db)
	filterRepo := filter.NewRepository(db)
	filterEng := filter.NewEngine(filterRepo, logger)
	notifyRepo := notification.NewRepository(db)
	settingsRepo := setting.NewRepository(db)
	hub := NewHub()
	var dashChecker clientOnlineChecker
	if clientMgr != nil {
		dashChecker = clientMgr
	}
	return &Router{
		authHandler:        NewAuthHandler(authManager),
		clientHandler:      NewClientHandler(db, logger, clientMgr),
		siteHandler:        NewSiteHandler(siteRepo, logger, db),
		rssHandler:         NewRSSHandler(rssRepo, rssEngine, db, logger),
		filterHandler:      NewFilterHandler(filterRepo, filterEng, db, logger),
		notifyHandler:      NewNotifyHandler(notifyRepo, notifyService, logger),
		settingsHandler:    NewSettingsHandler(settingsRepo, logger),
		seedingHandler:     NewSeedingHandler(db, logger, seedingEngine),
		deleteRuleHandler:  NewDeleteRuleHandler(db, logger),
		reseedHandler:      NewReseedHandler(reseedEngine, logger),
		publishHandler:     NewPublishHandler(publishPipeline, logger, db),
		dashboardHandler:   NewDashboardHandler(db, logger, appVersion, dashChecker),
		systemHandler:      NewSystemHandler(appVersion, db, clientMgr, logger),
		iyuuHandler:        NewIYUUHandler(db, logger, iyuuSvc),
		fingerprintHandler: NewFingerprintHandler(db, logger),
		trackerHandler:     NewTrackerHandler(db, logger),
		lifecycleHandler:   NewLifecycleHandler(db, logger),
		cookiecloudHandler: NewCookieCloudHandler(db, logger),
		ptgenHandler:       NewPTGenHandler(db, logger),
		schedulerHandler:   NewSchedulerHandler(taskRegistry, logger),
		wsHandler:          NewWSHandler(hub, authManager, nil),
		hub:                hub,
		authManager:        authManager,
		logger:             logger,
	}
}

func (rt *Router) Register(mux *http.ServeMux, corsOrigins []string, rateLimitEnabled bool, rateLimitGlobal int) {
	rt.RegisterWithEndpointLimits(mux, corsOrigins, rateLimitEnabled, rateLimitGlobal, 0, 0)
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

	dlHandler := rt.corsMW(rt.recoveryMW(downloadLimitMW(rt.authMW(rt.secMW(http.HandlerFunc(rt.clientHandler.ServeHTTP))))))
	mux.Handle("/api/v1/downloaders", dlHandler)
	mux.Handle("/api/v1/downloaders/", dlHandler)

	publishTargetHandler := rt.corsMW(rt.recoveryMW(rt.rateLimitMW(rt.authMW(rt.secMW(http.HandlerFunc(rt.clientHandler.handlePublishTargets))))))
	mux.Handle("/api/v1/downloaders/publish-targets", publishTargetHandler)

	siteHandler := rt.corsMW(rt.recoveryMW(rt.rateLimitMW(rt.authMW(rt.secMW(http.HandlerFunc(rt.siteHandler.ServeHTTP))))))
	mux.Handle("/api/v1/sites", siteHandler)
	mux.Handle("/api/v1/sites/", siteHandler)

	freezeStatusHandler := rt.corsMW(rt.recoveryMW(rt.rateLimitMW(rt.authMW(rt.secMW(http.HandlerFunc(rt.siteHandler.handleFreezeStatus))))))
	mux.Handle("/api/v1/httpclient/freeze-status", freezeStatusHandler)

	circuitStatusHandler := rt.corsMW(rt.recoveryMW(rt.rateLimitMW(rt.authMW(rt.secMW(http.HandlerFunc(rt.siteHandler.handleCircuitStatus))))))
	mux.Handle("/api/v1/httpclient/circuit-status", circuitStatusHandler)

	exclusionHandler := rt.corsMW(rt.recoveryMW(rt.rateLimitMW(rt.authMW(rt.secMW(http.HandlerFunc(rt.siteHandler.handleExclusions))))))
	mux.Handle("/api/v1/publish/exclusions", exclusionHandler)

	rssHandler := rt.corsMW(rt.recoveryMW(rt.rateLimitMW(rt.authMW(rt.secMW(http.HandlerFunc(rt.rssHandler.ServeHTTP))))))
	mux.Handle("/api/v1/rss/subscriptions", rssHandler)
	mux.Handle("/api/v1/rss/subscriptions/", rssHandler)

	filterHandler := rt.corsMW(rt.recoveryMW(rt.rateLimitMW(rt.authMW(rt.secMW(http.HandlerFunc(rt.filterHandler.ServeHTTP))))))
	mux.Handle("/api/v1/filters/rules", filterHandler)
	mux.Handle("/api/v1/filters/rules/", filterHandler)

	notifyHandler := rt.corsMW(rt.recoveryMW(rt.rateLimitMW(rt.authMW(rt.secMW(http.HandlerFunc(rt.notifyHandler.ServeHTTP))))))
	mux.Handle("/api/v1/notifications/channels", notifyHandler)
	mux.Handle("/api/v1/notifications/channels/", notifyHandler)

	settingsHandler := rt.corsMW(rt.recoveryMW(rt.rateLimitMW(rt.authMW(rt.secMW(http.HandlerFunc(rt.settingsHandler.ServeHTTP))))))
	mux.Handle("/api/v1/settings", settingsHandler)
	mux.Handle("/api/v1/settings/", settingsHandler)

	seedingHandler := rt.corsMW(rt.recoveryMW(rt.rateLimitMW(rt.authMW(rt.secMW(http.HandlerFunc(rt.seedingHandler.ServeHTTP))))))
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
	mux.Handle("/api/v1/seeding/rules", seedingHandler)
	mux.Handle("/api/v1/seeding/rules/", seedingHandler)
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

	deleteRuleHandler := rt.corsMW(rt.recoveryMW(rt.rateLimitMW(rt.authMW(rt.secMW(http.HandlerFunc(rt.deleteRuleHandler.ServeHTTP))))))
	mux.Handle("/api/v1/seeding/delete-rules", deleteRuleHandler)
	mux.Handle("/api/v1/seeding/delete-rules/", deleteRuleHandler)

	reseedHandler := rt.corsMW(rt.recoveryMW(rt.rateLimitMW(rt.authMW(rt.secMW(http.HandlerFunc(rt.reseedHandler.ServeHTTP))))))
	mux.Handle("/api/v1/reseed/tasks", reseedHandler)
	mux.Handle("/api/v1/reseed/tasks/", reseedHandler)

	publishHandler := rt.corsMW(rt.recoveryMW(writeLimitMW(rt.authMW(rt.secMW(http.HandlerFunc(rt.publishHandler.ServeHTTP))))))
	mux.Handle("/api/v1/publish/tasks", publishHandler)
	mux.Handle("/api/v1/publish/tasks/", publishHandler)
	mux.Handle("/api/v1/publish/candidates", publishHandler)
	mux.Handle("/api/v1/publish/candidates/", publishHandler)
	mux.Handle("/api/v1/publish/results", publishHandler)
	mux.Handle("/api/v1/publish/results/", publishHandler)
	mux.Handle("/api/v1/publish/groups", publishHandler)
	mux.Handle("/api/v1/publish/groups/", publishHandler)

	dashboardHandler := rt.corsMW(rt.recoveryMW(rt.rateLimitMW(rt.authMW(rt.secMW(http.HandlerFunc(rt.dashboardHandler.ServeHTTP))))))
	mux.Handle("/api/v1/dashboard/overview", dashboardHandler)
	mux.Handle("/api/v1/dashboard/overview/", dashboardHandler)
	mux.Handle("/api/v1/dashboard/activities", dashboardHandler)
	mux.Handle("/api/v1/dashboard/activities/", dashboardHandler)
	mux.Handle("/api/v1/dashboard/trends", dashboardHandler)
	mux.Handle("/api/v1/dashboard/trends/", dashboardHandler)

	systemHandler := rt.corsMW(rt.recoveryMW(rt.rateLimitMW(rt.authMW(rt.secMW(http.HandlerFunc(rt.systemHandler.ServeHTTP))))))
	mux.Handle("/api/v1/system/info", systemHandler)
	mux.Handle("/api/v1/system/info/", systemHandler)
	mux.Handle("/api/v1/system/logs", systemHandler)
	mux.Handle("/api/v1/system/logs/", systemHandler)
	mux.Handle("/api/v1/system/health", systemHandler)
	mux.Handle("/api/v1/system/health/", systemHandler)

	torrentEventHandler := rt.corsMW(rt.recoveryMW(rt.rateLimitMW(rt.authMW(rt.secMW(http.HandlerFunc(rt.dashboardHandler.ServeHTTP))))))
	mux.Handle("/api/v1/torrent-events", torrentEventHandler)
	mux.Handle("/api/v1/torrent-events/", torrentEventHandler)

	iyuuHandler := rt.corsMW(rt.recoveryMW(rt.rateLimitMW(rt.authMW(rt.secMW(http.HandlerFunc(rt.iyuuHandler.ServeHTTP))))))
	mux.Handle("/api/v1/iyuu/config", iyuuHandler)
	mux.Handle("/api/v1/iyuu/config/", iyuuHandler)
	mux.Handle("/api/v1/iyuu/sites", iyuuHandler)
	mux.Handle("/api/v1/iyuu/sites/", iyuuHandler)
	mux.Handle("/api/v1/iyuu/query", iyuuHandler)
	mux.Handle("/api/v1/iyuu/query/", iyuuHandler)
	mux.Handle("/api/v1/iyuu/test", iyuuHandler)
	mux.Handle("/api/v1/iyuu/test/", iyuuHandler)

	fingerprintHandler := rt.corsMW(rt.recoveryMW(rt.rateLimitMW(rt.authMW(rt.secMW(http.HandlerFunc(rt.fingerprintHandler.ServeHTTP))))))
	mux.Handle("/api/v1/fingerprints", fingerprintHandler)
	mux.Handle("/api/v1/fingerprints/", fingerprintHandler)

	trackerHandler := rt.corsMW(rt.recoveryMW(rt.rateLimitMW(rt.authMW(rt.secMW(http.HandlerFunc(rt.trackerHandler.ServeHTTP))))))
	mux.Handle("/api/v1/tracker/members", trackerHandler)
	mux.Handle("/api/v1/tracker/members/", trackerHandler)
	mux.Handle("/api/v1/tracker/history", trackerHandler)
	mux.Handle("/api/v1/tracker/history/", trackerHandler)

	lifecycleHandler := rt.corsMW(rt.recoveryMW(rt.rateLimitMW(rt.authMW(rt.secMW(http.HandlerFunc(rt.lifecycleHandler.ServeHTTP))))))
	mux.Handle("/api/v1/lifecycle/config", lifecycleHandler)
	mux.Handle("/api/v1/lifecycle/config/", lifecycleHandler)
	mux.Handle("/api/v1/lifecycle/backpressure", lifecycleHandler)
	mux.Handle("/api/v1/lifecycle/backpressure/", lifecycleHandler)

	cookiecloudHandler := rt.corsMW(rt.recoveryMW(rt.rateLimitMW(rt.authMW(rt.secMW(http.HandlerFunc(rt.cookiecloudHandler.ServeHTTP))))))
	mux.Handle("/api/v1/cookiecloud/config", cookiecloudHandler)
	mux.Handle("/api/v1/cookiecloud/config/", cookiecloudHandler)
	mux.Handle("/api/v1/cookiecloud/sync", cookiecloudHandler)
	mux.Handle("/api/v1/cookiecloud/sync/", cookiecloudHandler)
	mux.Handle("/api/v1/cookiecloud/history", cookiecloudHandler)
	mux.Handle("/api/v1/cookiecloud/history/", cookiecloudHandler)
	mux.Handle("/api/v1/cookiecloud/test", cookiecloudHandler)
	mux.Handle("/api/v1/cookiecloud/test/", cookiecloudHandler)

	ptgenHandler := rt.corsMW(rt.recoveryMW(rt.rateLimitMW(rt.authMW(rt.secMW(http.HandlerFunc(rt.ptgenHandler.ServeHTTP))))))
	mux.Handle("/api/v1/ptgen/query", ptgenHandler)
	mux.Handle("/api/v1/ptgen/query/", ptgenHandler)
	mux.Handle("/api/v1/ptgen/cache", ptgenHandler)
	mux.Handle("/api/v1/ptgen/cache/", ptgenHandler)

	schedulerHandler := rt.corsMW(rt.recoveryMW(rt.rateLimitMW(rt.authMW(rt.secMW(http.HandlerFunc(rt.schedulerHandler.ServeHTTP))))))
	mux.Handle("/api/v1/scheduler/tasks", schedulerHandler)
	mux.Handle("/api/v1/scheduler/tasks/", schedulerHandler)
}

func (rt *Router) public(fn http.HandlerFunc) http.HandlerFunc {
	chain := rt.corsMW(rt.recoveryMW(rt.secMW(fn)))
	return chain.ServeHTTP
}

func (rt *Router) protected(fn http.HandlerFunc) http.Handler {
	return rt.corsMW(rt.recoveryMW(rt.rateLimitMW(rt.authMW(rt.secMW(fn)))))
}

func (rt *Router) Hub() *Hub {
	return rt.hub
}
