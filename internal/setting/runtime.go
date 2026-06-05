package setting

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	KeyLoginLockoutEnabled = "login_lockout_enabled"
	KeyLoginMaxRetries     = "login_max_retries"
	KeyLoginLockoutMin     = "login_lockout_min"

	KeyRateLimitEnabled  = "rate_limit_enabled"
	KeyRateLimitGlobal   = "rate_limit_global"
	KeyRateLimitWrite    = "rate_limit_write"
	KeyRateLimitDownload = "rate_limit_download"

	KeyScreenshotEnabled     = "screenshot_enabled"
	KeyScreenshotMpvPath     = "screenshot_mpv_path"
	KeyScreenshotCount       = "screenshot_count"
	KeyScreenshotMinInterval = "screenshot_min_interval"
	KeyScreenshotJPEGQuality = "screenshot_jpeg_quality"

	KeyDataCleanupTorrentEventDays   = "data_cleanup_torrent_event_days"
	KeyDataCleanupPublishResultDays  = "data_cleanup_publish_result_days"
	KeyDataCleanupSeenRecordDays     = "data_cleanup_seen_record_days"
	KeyDataCleanupNotificationDays   = "data_cleanup_notification_days"
	KeyDataCleanupPublishTaskDays    = "data_cleanup_publish_task_days"
	KeyDataCleanupReseedMatchDays    = "data_cleanup_reseed_match_days"
	KeyDataCleanupPTGenCacheDays     = "data_cleanup_ptgen_cache_days"
	KeyDataCleanupSeedingArchiveDays = "data_cleanup_seeding_archive_days"
	KeyDataCleanupAuditLogDays       = "data_cleanup_audit_log_days"
	KeyTorrentTrafficRetentionDays   = "torrent_traffic_retention_days"
)

var DefaultSeeds = map[string]string{
	KeyLoginLockoutEnabled: "false",
	KeyLoginMaxRetries:     "5",
	KeyLoginLockoutMin:     "5",

	KeyRateLimitEnabled:  "false",
	KeyRateLimitGlobal:   "600",
	KeyRateLimitWrite:    "200",
	KeyRateLimitDownload: "50",

	KeyScreenshotEnabled:     "false",
	KeyScreenshotMpvPath:     "mpv",
	KeyScreenshotCount:       "6",
	KeyScreenshotMinInterval: "60",
	KeyScreenshotJPEGQuality: "85",

	KeyDataCleanupTorrentEventDays:   "30",
	KeyDataCleanupPublishResultDays:  "30",
	KeyDataCleanupSeenRecordDays:     "30",
	KeyDataCleanupNotificationDays:   "30",
	KeyDataCleanupPublishTaskDays:    "30",
	KeyDataCleanupReseedMatchDays:    "30",
	KeyDataCleanupPTGenCacheDays:     "90",
	KeyDataCleanupSeedingArchiveDays: "90",
	KeyDataCleanupAuditLogDays:       "90",
	KeyTorrentTrafficRetentionDays:   "30",
}

func SeedDefaults(ctx context.Context, repo *Repository, seeds map[string]string, logger *zap.Logger) {
	for key, value := range seeds {
		_, err := repo.Get(ctx, key)
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				logger.Warn("seed setting check failed", zap.String("key", key), zap.Error(err))
				continue
			}
			if err := repo.Set(ctx, key, value); err != nil {
				logger.Warn("seed setting failed", zap.String("key", key), zap.Error(err))
			}
		}
	}
}

type RuntimeConfig struct {
	repo   *Repository
	logger *zap.Logger
	mu     sync.RWMutex
	cache  map[string]string
	expiry time.Time
	ttl    time.Duration
}

func NewRuntimeConfig(repo *Repository, logger *zap.Logger) *RuntimeConfig {
	return &RuntimeConfig{
		repo:   repo,
		logger: logger,
		cache:  make(map[string]string),
		ttl:    30 * time.Second,
	}
}

func (rc *RuntimeConfig) Reload(ctx context.Context) error {
	all, err := rc.repo.ListAll(ctx)
	if err != nil {
		return err
	}
	rc.mu.Lock()
	rc.cache = all
	rc.expiry = time.Now().Add(rc.ttl)
	rc.mu.Unlock()
	return nil
}

func (rc *RuntimeConfig) get(ctx context.Context, key string) string {
	rc.mu.RLock()
	if time.Now().Before(rc.expiry) {
		v, ok := rc.cache[key]
		rc.mu.RUnlock()
		if ok {
			return v
		}
		return ""
	}
	rc.mu.RUnlock()

	rc.mu.Lock()
	defer rc.mu.Unlock()
	if time.Now().Before(rc.expiry) {
		v, ok := rc.cache[key]
		if ok {
			return v
		}
		return ""
	}

	all, err := rc.repo.ListAll(ctx)
	if err != nil {
		rc.logger.Warn("runtime config reload failed", zap.Error(err))
		v := rc.cache[key]
		return v
	}
	rc.cache = all
	rc.expiry = time.Now().Add(rc.ttl)
	v := rc.cache[key]
	return v
}

func (rc *RuntimeConfig) GetString(ctx context.Context, key string) string {
	return rc.get(ctx, key)
}

func (rc *RuntimeConfig) GetInt(ctx context.Context, key string) int {
	v := rc.get(ctx, key)
	if v == "" {
		return 0
	}
	n, _ := strconv.Atoi(v)
	return n
}

func (rc *RuntimeConfig) GetBool(ctx context.Context, key string) bool {
	v := rc.get(ctx, key)
	return v == "true" || v == "1"
}
