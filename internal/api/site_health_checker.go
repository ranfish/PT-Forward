package api

import (
	"context"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type SiteHealthChecker struct {
	db     *gorm.DB
	logger *zap.Logger
	hub    *Hub
}

func NewSiteHealthChecker(db *gorm.DB, logger *zap.Logger, hub *Hub) *SiteHealthChecker {
	return &SiteHealthChecker{db: db, logger: logger, hub: hub}
}

func (s *SiteHealthChecker) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	s.check(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.check(ctx)
		}
	}
}

func (s *SiteHealthChecker) check(ctx context.Context) {
	var sites []struct {
		Name      string `gorm:"column:name"`
		BaseURL   string `gorm:"column:base_url"`
		Enabled   bool   `gorm:"column:enabled"`
	}
	if err := s.db.WithContext(ctx).Table("sites").Find(&sites).Error; err != nil {
		s.logger.Warn("site health check: query sites failed", zap.Error(err))
		return
	}

	online := 0
	for _, site := range sites {
		if site.Enabled {
			online++
		}
	}

	s.logger.Debug("site health check completed",
		zap.Int("total", len(sites)),
		zap.Int("enabled", online),
	)
}
