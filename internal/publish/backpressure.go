package publish

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/ranfish/pt-forward/internal/model"
)

type BackpressureSignal int

const (
	SignalBandwidth  BackpressureSignal = iota + 1
	SignalQueueDepth
	SignalSiteFailure
	SignalDiskIO
)

var (
	ErrBackpressureSitePaused  = fmt.Errorf("site paused due to consecutive failures")
	ErrBackpressureQueueFull   = fmt.Errorf("publish queue depth exceeded threshold")
	ErrBackpressureConcurrency = fmt.Errorf("max concurrent publish limit reached")
	ErrBackpressureDiskIO      = fmt.Errorf("disk IO slot limit reached")
)

type BackpressureConfig struct {
	MaxConcurrent           int     `json:"max_concurrent"`
	BandwidthThresholdPct   float64 `json:"bandwidth_threshold_pct"`
	BandwidthLimitSites     int     `json:"bandwidth_limit_sites"`
	QueueThreshold          int     `json:"queue_threshold"`
	SiteConsecutiveFailures int     `json:"site_consecutive_failures"`
	SiteCooldownMinutes     int     `json:"site_cooldown_minutes"`
	DiskIOCheckEnabled      bool    `json:"disk_io_check_enabled"`
	DiskIOMaxConcurrent     int     `json:"disk_io_max_concurrent"`
	EmaAlpha                float64 `json:"ema_alpha"`
}

func DefaultBackpressureConfig() BackpressureConfig {
	return BackpressureConfig{
		MaxConcurrent:           10,
		BandwidthThresholdPct:   0.8,
		BandwidthLimitSites:     3,
		QueueThreshold:          20,
		SiteConsecutiveFailures: 3,
		SiteCooldownMinutes:     30,
		DiskIOCheckEnabled:      true,
		DiskIOMaxConcurrent:     3,
		EmaAlpha:                0.1,
	}
}

type siteFailureTracker struct {
	consecutiveFailures int
	lastFailureAt       time.Time
	pausedUntil         time.Time
}

type BackpressureController struct {
	config            BackpressureConfig
	db                *gorm.DB
	logger            *zap.Logger
	activeCount       atomic.Int32
	diskIOActiveCount atomic.Int32
	siteFailures      map[string]*siteFailureTracker
	mu                sync.RWMutex
}

func NewBackpressureController(db *gorm.DB, cfg BackpressureConfig, logger *zap.Logger) *BackpressureController {
	return &BackpressureController{
		config:       cfg,
		db:           db,
		logger:       logger,
		siteFailures: make(map[string]*siteFailureTracker),
	}
}

func (c *BackpressureController) UpdateConfig(cfg BackpressureConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config = cfg
	c.info("backpressure config updated",
		zap.Int("max_concurrent", cfg.MaxConcurrent),
		zap.Int("queue_threshold", cfg.QueueThreshold))
}

func (c *BackpressureController) GetConfig() BackpressureConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.config
}

func (c *BackpressureController) warn(msg string, fields ...zap.Field) {
	if c.logger != nil {
		c.logger.Warn(msg, fields...)
	}
}

func (c *BackpressureController) info(msg string, fields ...zap.Field) {
	if c.logger != nil {
		c.logger.Info(msg, fields...)
	}
}

func (c *BackpressureController) AcquireSlot(ctx context.Context, targetSite string) error {
	if c.isSitePaused(targetSite) {
		c.warn("backpressure: site paused", zap.String("site", targetSite))
		return ErrBackpressureSitePaused
	}

	queueDepth := c.getQueueDepth()
	if queueDepth >= c.config.QueueThreshold {
		c.warn("backpressure: queue full",
			zap.Int("depth", queueDepth),
			zap.Int("threshold", c.config.QueueThreshold))
		return ErrBackpressureQueueFull
	}

	newCount := c.activeCount.Add(1)
	if int(newCount) > c.config.MaxConcurrent {
		c.activeCount.Add(-1)
		c.warn("backpressure: concurrency limit",
			zap.Int("active", int(newCount)-1),
			zap.Int("max", c.config.MaxConcurrent))
		return ErrBackpressureConcurrency
	}

	return nil
}

func (c *BackpressureController) ReleaseSlot(targetSite string, success bool) {
	c.activeCount.Add(-1)
	c.recordSiteResult(targetSite, success)
}

func (c *BackpressureController) AcquireDiskIOSlot() error {
	if !c.config.DiskIOCheckEnabled {
		return nil
	}
	newCount := c.diskIOActiveCount.Add(1)
	if int(newCount) > c.config.DiskIOMaxConcurrent {
		c.diskIOActiveCount.Add(-1)
		return ErrBackpressureDiskIO
	}
	return nil
}

func (c *BackpressureController) ReleaseDiskIOSlot() {
	c.diskIOActiveCount.Add(-1)
}

func (c *BackpressureController) Status() BackpressureStatus {
	c.mu.RLock()
	siteFailures := make(map[string]SiteFailureStatus)
	for site, tracker := range c.siteFailures {
		if time.Now().Before(tracker.pausedUntil) {
			siteFailures[site] = SiteFailureStatus{
				ConsecutiveFailures: tracker.consecutiveFailures,
				PausedUntil:         tracker.pausedUntil,
			}
		}
	}
	cfg := c.config
	c.mu.RUnlock()

	active := int(c.activeCount.Load())
	diskActive := int(c.diskIOActiveCount.Load())
	queueDepth := c.getQueueDepth()

	return BackpressureStatus{
		QueueDepth:             queueDepth,
		MaxQueueDepth:          cfg.QueueThreshold,
		ActivePublishes:        active,
		MaxConcurrentPublishes: cfg.MaxConcurrent,
		ActiveDiskIO:           diskActive,
		MaxDiskIO:              cfg.DiskIOMaxConcurrent,
		IsThrottled:            active >= cfg.MaxConcurrent,
		PauseOnPressure:        cfg.QueueThreshold > 0,
		PausedSites:            siteFailures,
	}
}

type SiteFailureStatus struct {
	ConsecutiveFailures int       `json:"consecutive_failures"`
	PausedUntil         time.Time `json:"paused_until"`
}

type BackpressureStatus struct {
	QueueDepth             int                           `json:"queue_depth"`
	MaxQueueDepth          int                           `json:"max_queue_depth"`
	ActivePublishes        int                           `json:"active_publishes"`
	MaxConcurrentPublishes int                           `json:"max_concurrent_publishes"`
	ActiveDiskIO           int                           `json:"active_disk_io"`
	MaxDiskIO              int                           `json:"max_disk_io"`
	IsThrottled            bool                          `json:"is_throttled"`
	PauseOnPressure        bool                          `json:"pause_on_pressure"`
	PausedSites            map[string]SiteFailureStatus  `json:"paused_sites,omitempty"`
}

func (c *BackpressureController) isSitePaused(site string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	tracker, ok := c.siteFailures[site]
	if !ok {
		return false
	}
	return time.Now().Before(tracker.pausedUntil)
}

func (c *BackpressureController) recordSiteResult(site string, success bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	tracker, ok := c.siteFailures[site]
	if !ok {
		tracker = &siteFailureTracker{}
		c.siteFailures[site] = tracker
	}

	if success {
		tracker.consecutiveFailures = 0
		tracker.pausedUntil = time.Time{}
		return
	}

	tracker.consecutiveFailures++
	tracker.lastFailureAt = time.Now()

	if tracker.consecutiveFailures >= c.config.SiteConsecutiveFailures {
		cooldown := time.Duration(c.config.SiteCooldownMinutes) * time.Minute
		tracker.pausedUntil = time.Now().Add(cooldown)
		c.warn("backpressure: site paused due to consecutive failures",
			zap.String("site", site),
			zap.Int("failures", tracker.consecutiveFailures),
			zap.Time("paused_until", tracker.pausedUntil))
	}
}

func (c *BackpressureController) getQueueDepth() int {
	if c.db == nil {
		return 0
	}
	staleCutoff := time.Now().Add(-30 * time.Minute)
	var count int64
	c.db.Model(&model.PublishCandidate{}).
		Where("publish_status = ? AND updated_at > ?", "publishing", staleCutoff).
		Count(&count)
	return int(count)
}

func (c *BackpressureController) StartCleanup(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.cleanupExpiredPauses()
			c.recoverStalePublishing()
		}
	}
}

func (c *BackpressureController) recoverStalePublishing() {
	staleCutoff := time.Now().Add(-30 * time.Minute)
	result := c.db.Model(&model.PublishCandidate{}).
		Where("publish_status = ? AND updated_at < ?", "publishing", staleCutoff).
		Updates(map[string]interface{}{
			"publish_status": "pending",
			"updated_at":     time.Now(),
		})
	if result.RowsAffected > 0 {
		c.info("recovered stale publishing candidates",
			zap.Int64("count", result.RowsAffected))
	}
}

func (c *BackpressureController) cleanupExpiredPauses() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	for site, tracker := range c.siteFailures {
		if now.After(tracker.pausedUntil) && tracker.consecutiveFailures == 0 {
			delete(c.siteFailures, site)
		}
	}
}
