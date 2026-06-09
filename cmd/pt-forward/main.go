package main

import (
	"compress/gzip"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"sync"

	"github.com/ranfish/pt-forward/frontend"
	"github.com/ranfish/pt-forward/internal/adapter"
	"github.com/ranfish/pt-forward/internal/api"
	"github.com/ranfish/pt-forward/internal/audit"
	"github.com/ranfish/pt-forward/internal/auth"
	"github.com/ranfish/pt-forward/internal/client"
	"github.com/ranfish/pt-forward/internal/config"
	"github.com/ranfish/pt-forward/internal/cookiecloud"
	"github.com/ranfish/pt-forward/internal/crypto"
	dbpkg "github.com/ranfish/pt-forward/internal/db"
	"github.com/ranfish/pt-forward/internal/dispatcher"
	"github.com/ranfish/pt-forward/internal/event"
	"github.com/ranfish/pt-forward/internal/filter"
	"github.com/ranfish/pt-forward/internal/fingerprint"
	"github.com/ranfish/pt-forward/internal/health"
	"github.com/ranfish/pt-forward/internal/httpclient"
	"github.com/ranfish/pt-forward/internal/iyuu"
	"github.com/ranfish/pt-forward/internal/metrics"
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
	"github.com/ranfish/pt-forward/internal/util"
	"github.com/ranfish/pt-forward/internal/watcher"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var version = "dev"

func main() {
	configPath := flag.String("config", "", "path to config file")
	showVersion := flag.Bool("version", false, "print version and exit")
	resetPassword := flag.String("reset-password", "", "reset admin password to the given value")
	flag.Parse()

	if *showVersion {
		fmt.Println("pt-forward", version)
		return
	}

	log := newDefaultLogger()

	log.Info("starting pt-forward", zap.String("version", version))

	cfg, err := config.Load(*configPath, log)
	if err != nil {
		log.Error("failed to load config", zap.Error(err))
		_ = log.Sync()
		os.Exit(1) //nolint:gocritic // defers intentionally skipped on fatal config error
	}

	defer func() { _ = log.Sync() }()

	log = reconfigureLogger(cfg.Log)
	defer func() { _ = log.Sync() }()

	db, err := initDB(cfg, log)
	if err != nil {
		log.Error("failed to init database", zap.Error(err))
		os.Exit(1) //nolint:gocritic
	}

	if err := model.AutoMigrate(db); err != nil {
		log.Error("failed to run auto migration", zap.Error(err))
		os.Exit(1) //nolint:gocritic
	}

	if err := initEncryption(cfg, db, log); err != nil {
		log.Error("failed to init encryption", zap.Error(err))
		os.Exit(1) //nolint:gocritic
	}

	var writeQueue *dbpkg.WriteQueue
	if cfg.Database.Driver == "mysql" {
		writeQueue = dbpkg.NewPassthroughWriteQueue(db)
	} else {
		writeQueue = dbpkg.NewWriteQueue(db, 256)
	}

	memMonitor := dbpkg.NewMemoryMonitor(dbpkg.MemoryConfig{
		MaxTotalMB:  int(cfg.Memory.MaxTotalMB),
		WarnPercent: cfg.Memory.WarnPercent,
	}, log)

	if *resetPassword != "" {
		if err := resetAdminPassword(db, *resetPassword, log); err != nil {
			log.Error("failed to reset password", zap.Error(err))
			os.Exit(1) //nolint:gocritic
		}
		log.Info("admin password reset successfully")
		return
	}

	httpclient.Init(log)
	metrics.Init(version)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	writeQueue.Start(ctx)
	memMonitor.Start(ctx)

	settingsRepo := setting.NewRepository(db)
	authRepo := auth.NewGormAuthRepository(db)
	authManager, err := auth.NewAuthManagerWithSettings(authRepo, settingsRepo, log)
	if err != nil {
		log.Error("failed to init auth manager", zap.Error(err))
		os.Exit(1) //nolint:gocritic
	}
	defer authManager.Stop()

	if err := auth.EnsureAdminUser(ctx, authRepo, log); err != nil {
		log.Error("failed to ensure admin user", zap.Error(err))
		os.Exit(1) //nolint:gocritic
	}

	setting.SeedDefaults(ctx, settingsRepo, setting.DefaultSeeds, log)

	auditLogger := audit.NewLogger(db, log)
	auditLogger.Start(ctx)
	audit.SetDefault(auditLogger)

	go func() {
		if err := site.SeedSites(db); err != nil {
			log.Warn("site seed warning", zap.Error(err))
		}
		if err := site.SyncSiteTemplates(db); err != nil {
			log.Warn("sync site templates warning", zap.Error(err))
		}
		if err := site.SeedFieldMappings(db); err != nil {
			log.Warn("field mapping seed warning", zap.Error(err))
		}
		if err := site.SeedExclusions(db); err != nil {
			log.Warn("exclusion seed warning", zap.Error(err))
		}
		if err := site.SeedFormFieldOverrides(db); err != nil {
			log.Warn("form override seed warning", zap.Error(err))
		}
		log.Info("site seed completed")
	}()

	var allSites []model.Site
	if err := db.Find(&allSites).Error; err == nil {
		for _, s := range allSites {
			if s.MaxConcurrent > 0 {
				// Scale MaxReqs with MaxConcurrent (default 30 reqs/60s for
				// maxConcurrent=2). See applySiteMaxConcurrent for rationale.
				const defaultMaxConcurrent = 2
				const defaultMaxReqs = 30
				// DomainRateLimiter keys by "https://<domain>" (see transport.extractDomain),
				// so prepend the scheme here to match.
				rateKey := "https://" + s.Domain
				httpclient.GlobalLimiter.SetDomainConfig(rateKey, httpclient.DomainLimitConfig{
					MaxConcurrent: s.MaxConcurrent,
					MaxReqs:       defaultMaxReqs * s.MaxConcurrent / defaultMaxConcurrent,
					WindowSecs:    60,
				})
			}
		}
		log.Info("domain rate limiter initialized from site configs", zap.Int("sites", len(allSites)))
	}

	eventDispatcher := event.NewDispatcher(log)

	adapterFactory := adapter.NewFactory(log)
	siteProvider := site.NewProvider(db, adapterFactory, log)

	clientManager := client.NewManager(db, log)

	notifyService := notification.NewService(db, log)

	rssEngine := rss.NewEngine(db, log)

	filterEngine := filter.NewEngine(filter.NewRepository(db), log)

	reseedEngine := reseed.NewEngine(db, log)

	publishPipeline := publish.NewPipeline(db, log)

	seedingEngine := seeding.NewEngine(db, log)

	iyuuService := iyuu.NewService(db, log)

	fpRepo := fingerprint.NewRepository(db, log)

	completionWatcher := watcher.NewCompletionWatcher(db, clientManager, publishPipeline, log)

	wsHub := api.NewHub()
	stopCh := make(chan struct{})
	go wsHub.Run(stopCh)

	eventDispatcher.SetWSBroadcaster(wsHub)

	rssEngine.SetFilterEngine(filterEngine)
	rssEngine.SetDispatcher(eventDispatcher)
	rssEngine.SetSiteProvider(siteProvider)
	rssEngine.SetClientProvider(clientManager)
	rssEngine.SetSeedingCounter(seedingEngine)
	rssEngine.SetWSBroadcaster(wsHub)

	sideLoadEmitter := rss.NewSideLoadEventEmitter()
	sideLoadMgr := rss.NewSideLoadManager(siteProvider, sideLoadEmitter, log)
	rssEngine.SetSideLoadManager(sideLoadMgr)
	rssEngine.SetConfigEventBus(rss.NewConfigEventBus())
	configEventBus := rss.NewConfigEventBus()
	rssEngine.SetConfigEventBus(configEventBus)
	rssEngine.SetPushLimiter(rss.NewPushLimiter(0))
	sideLoadEventCh := sideLoadEmitter.Subscribe()
	rss.StartSideLoadRepeater(ctx, sideLoadEventCh, func(ctx context.Context, events []model.TorrentEvent) error {
		return eventDispatcher.Dispatch(ctx, "rss_new", events)
	}, log)

	reseedEngine.SetSiteProvider(siteProvider)
	reseedEngine.SetFingerprintRepo(fpRepo)
	reseedEngine.SetClientProvider(clientManager)
	reseedEngine.SetIYUUService(iyuuService)

	trackerResolver := reseed.NewTrackerSiteResolver()
	var siteRecords []*model.Site
	if err := db.Find(&siteRecords).Error; err != nil {
		log.Warn("加载站点列表用于 tracker 解析失败", zap.Error(err))
	} else {
		trackerResolver.BuildIndex(siteRecords)
		log.Info("tracker→站点映射索引已构建", zap.Int("sites", len(siteRecords)))
	}
	reseedEngine.SetTrackerResolver(trackerResolver)

	publishPipeline.SetSiteProvider(siteProvider)
	publishPipeline.SetClientProvider(clientManager)
	publishPipeline.SetCompletionWatcher(completionWatcher)
	publishPipeline.SetNotifyService(notifyService)
	cacheDir := filepath.Join(filepath.Dir(cfg.Database.SQLitePath), "cache")
	_ = os.MkdirAll(filepath.Join(cacheDir, "artifacts"), 0o750)
	_ = os.MkdirAll(filepath.Join(cacheDir, "torrents"), 0o750)
	publishPipeline.SetArtifactCache(publish.NewArtifactCache(filepath.Join(cacheDir, "artifacts"), log))
	publishPipeline.SetTorrentCache(publish.NewTorrentCache(filepath.Join(cacheDir, "torrents"), log))

	lifecycleManager := publish.NewLifecycleManager(db, log)
	lifecycleManager.SetClientProvider(clientManager)

	seedingConfirmation := publish.NewSeedingConfirmation(db, log)

	seedingEngine.SetClientProvider(clientManager)
	seedingEngine.SetSiteProvider(siteProvider)
	seedingEngine.SetWSBroadcaster(wsHub)
	seedingEngine.SetReseedTrigger(reseedEngine)

	freeWaitMonitor := seeding.NewFreeWaitMonitor(db, log)
	freeWaitMonitor.SetEngine(seedingEngine)

	torrentDispatcher := dispatcher.NewTorrentDispatcher(db, clientManager, log)
	torrentDispatcher.SetCircuitBreaker(dispatcher.NewCircuitBreaker(log, notifyService))
	torrentDispatcher.RegisterHandler(dispatcher.RoleSeeding, seedingEngine)
	torrentDispatcher.RegisterHandler(dispatcher.RoleDownload, publishPipeline)
	torrentDispatcher.RegisterHandler(dispatcher.RoleSource, publishPipeline)

	eventDispatcher.Register("rss_new", torrentDispatcher)

	taskRegistry := scheduler.NewRegistry(log)

	router := api.NewRouter(
		authManager,
		db,
		rssEngine,
		notifyService,
		reseedEngine,
		publishPipeline,
		seedingEngine,
		clientManager,
		taskRegistry,
		iyuuService,
		version,
		wsHub,
		log,
	)
	router.SetSiteProvider(siteProvider)
	router.SetConfigEventBus(configEventBus)

	mux := http.NewServeMux()
	router.RegisterWithEndpointLimits(mux, cfg.Server.CORSOrigins, true, 120, 60, 60)

	healthChecker := health.NewHealthChecker(version)
	if sqlDB, err := db.DB(); err == nil {
		healthChecker.SetDBPinger(health.NewSQLPinger(sqlDB))
	}
	mux.HandleFunc("/healthz", healthChecker.Handler)
	mux.Handle("/metrics", promhttp.Handler())

	spaHandler := frontend.SPAHandler()
	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isAPIOrWS(r.URL.Path) {
			mux.ServeHTTP(w, r)
			return
		}
		spaHandler.ServeHTTP(w, r)
	})

	var handler http.Handler = finalHandler
	handler = middleware.Metrics()(handler)
	handler = middleware.RequestLogger(log)(handler)

	if len(cfg.Security.TrustedProxies) > 0 {
		if err := middleware.SetTrustedProxies(cfg.Security.TrustedProxies); err != nil {
			log.Warn("failed to set trusted proxies", zap.Error(err))
		}
	}

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	syncManager := scheduler.NewSyncManager(setting.NewRuntimeConfig(settingsRepo, log), log)
	statsSyncSvc := site.NewStatsSyncService(db, adapterFactory, log)
	registerSchedulerTasks(taskRegistry, syncManager, siteProvider, clientManager, rssEngine, seedingEngine, reseedEngine, publishPipeline, lifecycleManager, seedingConfirmation, freeWaitMonitor, statsSyncSvc, notifyService, settingsRepo, db, log)
	api.ApplySchedulerOverrides(ctx, db, taskRegistry, log)

	if err := clientManager.LoadClients(ctx); err != nil {
		log.Warn("some clients failed to connect", zap.Error(err))
	}

	if err := seedingEngine.Start(ctx); err != nil {
		log.Warn("seeding engine start warning", zap.Error(err))
	}

	if err := rssEngine.Start(ctx); err != nil {
		log.Warn("rss engine start warning", zap.Error(err))
	}

	if err := reseedEngine.Start(ctx); err != nil {
		log.Warn("reseed engine start warning", zap.Error(err))
	}

	if err := completionWatcher.Start(ctx); err != nil {
		log.Warn("completion watcher start warning", zap.Error(err))
	}

	if err := taskRegistry.Start(ctx); err != nil {
		log.Error("failed to start task registry", zap.Error(err))
	}
	syncManager.Start(ctx)
	sideLoadMgr.Start(ctx)

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigCh
		log.Info("received shutdown signal", zap.String("signal", sig.String()))
		cancel()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer shutdownCancel()

		_ = taskRegistry.Stop()
		syncManager.Stop()
		rssEngine.Stop()
		sideLoadMgr.Stop()
		_ = seedingEngine.Stop(ctx)
		reseedEngine.Stop()
		completionWatcher.Stop()
		authManager.Stop()
		memMonitor.Stop()
		_ = writeQueue.Stop(ctx)
		close(stopCh)

		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Error("server shutdown error", zap.Error(err))
		}
	}()

	log.Info("server listening", zap.String("addr", addr))
	if cfg.Server.TLSEnabled {
		log.Info("TLS enabled", zap.String("cert", cfg.Server.TLSCert))
		if err := srv.ListenAndServeTLS(cfg.Server.TLSCert, cfg.Server.TLSKey); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server error", zap.Error(err))
			os.Exit(1) //nolint:gocritic
		}
	} else {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server error", zap.Error(err))
			os.Exit(1) //nolint:gocritic
		}
	}

	log.Info("pt-forward stopped")
}

func newDefaultLogger() *zap.Logger {
	encCfg := zap.NewProductionEncoderConfig()
	encCfg.TimeKey = "ts"
	encCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encCfg),
		zapcore.AddSync(os.Stdout),
		zapcore.InfoLevel,
	)
	return zap.New(util.NewSanitizerCore(core), zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
}

type dateRotatingWriter struct {
	mu       sync.Mutex
	dir      string
	prefix   string
	current  *os.File
	curDate  string
	compress bool
}

func newDateRotatingWriter(dir, prefix string, compress bool) *dateRotatingWriter {
	return &dateRotatingWriter{dir: dir, prefix: prefix, compress: compress}
}

func (w *dateRotatingWriter) todayFile() string {
	return filepath.Join(w.dir, w.prefix+"-"+time.Now().Format("2006-01-02")+".log")
}

func (w *dateRotatingWriter) Write(p []byte) (int, error) {
	today := time.Now().Format("2006-01-02")
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.curDate != today {
		if w.current != nil {
			oldName := w.current.Name()
			_ = w.current.Close()
			if w.compress {
				compressFile(oldName)
			}
		}
		f, err := os.OpenFile(w.todayFile(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return 0, err
		}
		w.current = f
		w.curDate = today
	}
	return w.current.Write(p)
}

func (w *dateRotatingWriter) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.current != nil {
		return w.current.Sync()
	}
	return nil
}

func (w *dateRotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.current != nil {
		err := w.current.Close()
		w.current = nil
		return err
	}
	return nil
}

func reconfigureLogger(cfg model.LogConfig) *zap.Logger {
	var level zapcore.Level
	if err := level.Set(cfg.Level); err != nil {
		level = zapcore.InfoLevel
	}

	_ = os.MkdirAll(cfg.Directory, 0o750)

	encCfg := zap.NewProductionEncoderConfig()
	encCfg.TimeKey = "ts"
	encCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	var encoder zapcore.Encoder
	if cfg.Format == "console" {
		encoder = zapcore.NewConsoleEncoder(encCfg)
	} else {
		encoder = zapcore.NewJSONEncoder(encCfg)
	}

	cores := []zapcore.Core{
		zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), level),
	}

	w := newDateRotatingWriter(cfg.Directory, "pt-forward", cfg.Compress)
	cores = append(cores, zapcore.NewCore(encoder, zapcore.AddSync(w), level))

	cleanupOldLogs(cfg)

	core := zapcore.NewTee(cores...)
	return zap.New(util.NewSanitizerCore(core), zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
}

func cleanupOldLogs(cfg model.LogConfig) {
	if cfg.MaxAgeDays <= 0 {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -cfg.MaxAgeDays)
	entries, err := os.ReadDir(cfg.Directory)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(cfg.Directory, entry.Name()))
		}
	}
}

func compressFile(path string) {
	src, err := os.Open(path) //nolint:gosec
	if err != nil {
		return
	}

	dst, err := os.Create(path + ".gz") //nolint:gosec
	if err != nil {
		_ = src.Close()
		return
	}

	gw := gzip.NewWriter(dst)

	_, copyErr := io.Copy(gw, src)

	_ = gw.Close()
	_ = dst.Close()
	_ = src.Close()

	if copyErr != nil {
		_ = os.Remove(path + ".gz")
		return
	}
	_ = os.Remove(path)
}

func initDB(cfg *config.Config, log *zap.Logger) (*gorm.DB, error) {
	var logLevel logger.LogLevel
	switch cfg.Database.LogLevel {
	case "silent":
		logLevel = logger.Silent
	case "error":
		logLevel = logger.Error
	case "warn":
		logLevel = logger.Warn
	case "info":
		logLevel = logger.Info
	default:
		logLevel = logger.Warn
	}

	gormCfg := &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	}

	var db *gorm.DB
	var err error

	switch cfg.Database.Driver {
	case "mysql":
		db, err = gorm.Open(mysql.Open(cfg.Database.MySQLDSN), gormCfg)
	case "sqlite":
		dbDir := filepath.Dir(cfg.Database.SQLitePath)
		if dbDir != "" && dbDir != "." {
			_ = os.MkdirAll(dbDir, 0o750)
		}
		dsn := cfg.Database.SQLitePath + "?_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL"
		db, err = gorm.Open(sqlite.Open(dsn), gormCfg)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.Database.Driver)
	}

	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get underlying sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)

	return db, nil
}

func initEncryption(cfg *config.Config, db *gorm.DB, log *zap.Logger) error {
	key := cfg.Security.EncryptionKey
	if key == "" {
		settingsRepo := setting.NewRepository(db)
		ctx := context.Background()
		stored, err := settingsRepo.Get(ctx, "encryption_key")
		if err == nil && stored != "" {
			key = stored
		}
	}
	if key == "" {
		return fmt.Errorf("encryption_key is required: set security.encryption_key in config or system_settings table")
	}
	enc, err := crypto.NewCredentialEncryptor(key)
	if err != nil {
		return fmt.Errorf("init credential encryptor: %w", err)
	}
	if err := crypto.RegisterCallbacks(db, enc, log); err != nil {
		return fmt.Errorf("register crypto callbacks: %w", err)
	}
	if err := crypto.MigratePlaintext(db, enc, log); err != nil {
		log.Warn("plaintext migration warning", zap.Error(err))
	}
	log.Info("credential encryption initialized")
	return nil
}

func resetAdminPassword(db *gorm.DB, newPassword string, log *zap.Logger) error {
	if err := auth.ValidatePasswordStrength(newPassword); err != nil {
		return fmt.Errorf("密码强度不足: %w", err)
	}
	repo := auth.NewGormAuthRepository(db)
	ctx := context.Background()
	user, err := repo.GetByUsername(ctx, "admin")
	if err != nil {
		return fmt.Errorf("find admin user: %w", err)
	}
	hash, err := auth.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	user.PasswordHash = hash
	return repo.Update(ctx, user)
}

func isAPIOrWS(path string) bool {
	return path == "/healthz" ||
		path == "/metrics" ||
		len(path) >= 5 && path[:5] == "/api/" ||
		len(path) >= 3 && path[:3] == "/ws"
}

func registerSchedulerTasks(
	registry *scheduler.Registry,
	syncMgr *scheduler.SyncManager,
	siteProvider *site.Provider,
	clientMgr *client.Manager,
	rssEngine *rss.Engine,
	seedingEngine *seeding.Engine,
	reseedEngine *reseed.Engine,
	publishPipeline *publish.Pipeline,
	lifecycleMgr *publish.LifecycleManager,
	seedingConfirm *publish.SeedingConfirmation,
	freeWaitMonitor *seeding.FreeWaitMonitor,
	statsSync *site.StatsSyncService,
	notifyService *notification.Service,
	settingsRepo *setting.Repository,
	db *gorm.DB,
	log *zap.Logger,
) {
	_ = syncMgr
	_ = siteProvider

	register := func(name, taskType, schedule string, handler scheduler.TaskFunc) {
		if err := registry.Register(name, taskType, schedule, handler); err != nil {
			log.Error("failed to register scheduler task", zap.String("name", name), zap.Error(err))
		}
	}

	register("client_ping", "maintenance", "0 */5 * * * *", func(ctx context.Context) error {
		clientMgr.PingAll(ctx)
		return nil
	})

	register("publish_pending", "publish", "0 */30 * * * *", func(ctx context.Context) error {
		return publishPipeline.ProcessPending(ctx)
	})

	register("publish_groups", "publish", "0 */60 * * * *", func(ctx context.Context) error {
		return publishPipeline.ProcessPendingGroups(ctx)
	})

	register("publish_lifecycle", "publish", "0 */30 * * * *", func(ctx context.Context) error {
		_, err := lifecycleMgr.CheckOnce(ctx)
		return err
	})

	register("publish_seeding_confirm", "publish", "0 */15 * * * *", func(ctx context.Context) error {
		return seedingConfirm.CheckOnce(ctx, clientMgr)
	})

	register("reseed_tasks", "reseed", "0 */5 * * * *", func(ctx context.Context) error {
		return reseedEngine.RunEnabledTasks(ctx)
	})

	register("reseed_retry", "reseed", "0 0 */1 * * *", func(ctx context.Context) error {
		retried, succeeded, err := reseedEngine.RetryFailedMatches(ctx)
		if err != nil {
			return err
		}
		if retried > 0 {
			log.Info("reseed retry processed",
				zap.Int("retried", retried),
				zap.Int("succeeded", succeeded))
		}
		return nil
	})

	register("seeding_cleanup", "seeding", "0 0 */6 * * *", func(ctx context.Context) error {
		cleaned, err := seedingEngine.CleanupStale(ctx)
		if err != nil {
			return err
		}
		if cleaned > 0 {
			log.Info("seeding cleanup removed stale records", zap.Int64("count", cleaned))
		}
		return nil
	})

	register("seeding_auto_delete", "seeding", "0 */30 * * * *", func(ctx context.Context) error {
		configs, err := seedingEngine.ListConfigs(ctx)
		if err != nil {
			return err
		}
		for _, cfg := range configs {
			result, evalErr := seedingEngine.Evaluate(ctx, cfg.ClientID, cfg)
			if evalErr != nil {
				log.Warn("seeding auto-delete evaluate failed", zap.String("clientId", cfg.ClientID), zap.Error(evalErr))
				continue
			}
			if result.Paused > 0 || result.Deleted > 0 {
				log.Info("seeding auto-delete",
					zap.String("clientId", cfg.ClientID),
					zap.Int("evaluated", result.Evaluated),
					zap.Int("paused", result.Paused),
					zap.Int("deleted", result.Deleted),
				)
			}
		}
		return nil
	})

	register("seeding_traffic_stats", "seeding", "0 */10 * * * *", func(ctx context.Context) error {
		return seedingEngine.CollectTrafficStats(ctx)
	})

	register("rss_disk_budget_expire", "rss", "0 */5 * * * *", func(_ context.Context) error {
		rssEngine.ExpireDiskBudget()
		return nil
	})

	register("rss_recheck_waiting", "rss", "0 */15 * * * *", func(ctx context.Context) error {
		return rssEngine.RecheckWaiting(ctx)
	})

	register("cookiecloud_sync", "maintenance", "0 */5 * * * *", func(ctx context.Context) error {
		var cfg model.CookieCloudConfig
		if err := db.WithContext(ctx).First(&cfg).Error; err != nil {
			return nil
		}
		if !cfg.SyncEnabled {
			return nil
		}
		interval := cfg.SyncInterval
		if interval < 5 {
			interval = 5
		}
		if cfg.LastSyncAt != nil && time.Since(*cfg.LastSyncAt) < time.Duration(interval)*time.Minute {
			return nil
		}
		syncSvc := cookiecloud.NewSyncService(db, log)
		history, err := syncSvc.SyncAll(ctx)
		if err != nil {
			log.Warn("cookiecloud scheduled sync failed", zap.Error(err))
			return nil
		}
		log.Info("cookiecloud scheduled sync completed",
			zap.Int("synced", history.SyncedSites),
			zap.Int("skipped", history.SkippedSites),
		)
		return nil
	})

	register("rss_cleanup_old_seen", "maintenance", "0 0 3 * * *", func(ctx context.Context) error {
		cleaned, err := rssEngine.CleanupOldData(ctx)
		if err != nil {
			return err
		}
		if cleaned > 0 {
			log.Info("cleaned old RSS seen records", zap.Int64("count", cleaned))
		}
		return nil
	})

	register("traffic_data_cleanup", "maintenance", "0 0 3 * * *", func(ctx context.Context) error {
		retentionDays := 30
		if v, err := settingsRepo.Get(ctx, setting.KeyTorrentTrafficRetentionDays); err == nil && v != "" {
			if d, pErr := strconv.Atoi(v); pErr == nil && d >= 7 {
				retentionDays = d
			}
		}
		cutoff := time.Now().AddDate(0, 0, -retentionDays)
		tc := db.Where("recorded_at < ?", cutoff).Delete(&model.TorrentTraffic{})
		sc := db.Where("recorded_at < ?", cutoff).Delete(&model.DownloaderSpeedSnapshot{})
		if tc.RowsAffected > 0 || sc.RowsAffected > 0 {
			log.Info("cleaned old traffic data",
				zap.Int64("torrent_traffic", tc.RowsAffected),
				zap.Int64("speed_snapshots", sc.RowsAffected),
				zap.Int("retention_days", retentionDays))
		}
		return nil
	})

	register("notification_cleanup_history", "maintenance", "0 0 4 * * *", func(ctx context.Context) error {
		return notifyService.CleanupHistory(ctx, 30)
	})

	if statsSync != nil {
		register("site_stats_sync", "maintenance", "0 0 */6 * * *", func(ctx context.Context) error {
			synced, failedSites := statsSync.SyncAllSites(ctx)
			if synced > 0 || len(failedSites) > 0 {
				log.Info("site stats sync completed", zap.Int("synced", synced), zap.Int("failed", len(failedSites)), zap.Strings("failedSites", failedSites))
			}
			return nil
		})
	}

	if freeWaitMonitor != nil && siteProvider != nil {
		register("seeding_free_wait_check", "seeding", "0 */15 * * * *", func(ctx context.Context) error {
			processed := seedingEngine.FreeWaitCheckOnce(ctx)
			if processed > 0 {
				log.Info("free wait check completed", zap.Int("processed", processed))
			}
			return nil
		})
	}
}
