package api

import (
	"context"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type CredentialDetector struct {
	db     *gorm.DB
	logger *zap.Logger
	hub    *Hub
}

func NewCredentialDetector(db *gorm.DB, logger *zap.Logger, hub *Hub) *CredentialDetector {
	return &CredentialDetector{db: db, logger: logger, hub: hub}
}

func (d *CredentialDetector) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.check(ctx)
		}
	}
}

func (d *CredentialDetector) CheckNow(ctx context.Context) {
	d.check(ctx)
}

func (d *CredentialDetector) check(ctx context.Context) {
	var sites []model.Site
	if err := d.db.WithContext(ctx).Where("enabled = ?", true).Find(&sites).Error; err != nil {
		d.logger.Error("credential detector: query sites failed", zap.Error(err))
		return
	}

	now := time.Now()

	for i := range sites {
		site := &sites[i]

		if site.Cookie != "" {
			if site.UpdatedAt.IsZero() || now.Sub(site.UpdatedAt) > 30*24*time.Hour {
				d.logger.Warn("site cookie may be expired",
					zap.String("site", site.Name),
					zap.Time("updated_at", site.UpdatedAt),
				)
				d.notify(site.Name, "cookie", "站点 Cookie 可能已过期（超过 30 天未更新）")
			}
		}

		if site.Passkey == "" && site.Cookie == "" {
			d.logger.Warn("site has no credentials",
				zap.String("site", site.Name),
			)
		}
	}
}

func (d *CredentialDetector) notify(siteName, credType, message string) {
	if d.hub != nil {
		d.hub.BroadcastWS("system.site.frozen", map[string]interface{}{
			"siteName": siteName,
			"type":     credType,
			"message":  message,
		})
	}
}
