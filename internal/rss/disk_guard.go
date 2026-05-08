package rss

import (
	"context"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type DiskGuard struct {
	db             *gorm.DB
	logger         *zap.Logger
	clientProvider model.DownloaderProvider
	checkInterval  time.Duration
}

func NewDiskGuard(db *gorm.DB, logger *zap.Logger) *DiskGuard {
	return &DiskGuard{
		db:            db,
		logger:        logger,
		checkInterval: 30 * time.Second,
	}
}

func (g *DiskGuard) SetClientProvider(cp model.DownloaderProvider) {
	g.clientProvider = cp
}

func (g *DiskGuard) Run(ctx context.Context) {
	ticker := time.NewTicker(g.checkInterval)
	defer ticker.Stop()

	g.logger.Info("disk guard started", zap.Duration("interval", g.checkInterval))

	for {
		select {
		case <-ctx.Done():
			g.logger.Info("disk guard stopped")
			return
		case <-ticker.C:
			g.check(ctx)
		}
	}
}

func (g *DiskGuard) check(ctx context.Context) {
	if g.clientProvider == nil {
		return
	}

	var subs []model.RSSSubscription
	if err := g.db.WithContext(ctx).
		Where("enabled = ? AND disk_guard_enabled = ? AND deleted_at = ?", true, true, time.Time{}).
		Find(&subs).Error; err != nil {
		g.logger.Warn("disk guard: query subscriptions failed", zap.Error(err))
		return
	}

	if len(subs) == 0 {
		return
	}

	clientFreeSpace := make(map[string]int64)
	for _, sub := range subs {
		if sub.ClientID == "" {
			continue
		}

		freeSpace, ok := clientFreeSpace[sub.ClientID]
		if !ok {
			dlClient, err := g.clientProvider.Get(sub.ClientID)
			if err != nil {
				continue
			}
			md, err := dlClient.GetMainData(ctx)
			if err != nil {
				continue
			}
			freeSpace = md.FreeSpace
			clientFreeSpace[sub.ClientID] = freeSpace
		}

		threshold := sub.DiskGuardThreshold
		if threshold <= 0 {
			threshold = 1073741824
		}

		isLow := freeSpace < int64(threshold)

		if isLow && !sub.Paused {
			now := time.Now()
			g.db.WithContext(ctx).Model(&model.RSSSubscription{}).
				Where("id = ?", sub.ID).
				Updates(map[string]interface{}{
					"paused":       true,
					"pause_reason": "disk_guard",
					"paused_at":    now,
				})
			g.logger.Warn("disk guard: 暂停订阅（磁盘空间不足）",
				zap.String("subscription", sub.Name),
				zap.String("client", sub.ClientID),
				zap.Int64("freeSpace", freeSpace),
				zap.Float64("threshold", threshold),
			)
		} else if !isLow && sub.Paused && sub.PauseReason == "disk_guard" {
			g.db.WithContext(ctx).Model(&model.RSSSubscription{}).
				Where("id = ?", sub.ID).
				Updates(map[string]interface{}{
					"paused":       false,
					"pause_reason": "",
					"paused_at":    nil,
				})
			g.logger.Info("disk guard: 恢复订阅（磁盘空间充足）",
				zap.String("subscription", sub.Name),
				zap.String("client", sub.ClientID),
				zap.Int64("freeSpace", freeSpace),
			)
		}
	}
}
