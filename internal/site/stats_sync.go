package site

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/ranfish/pt-forward/internal/adapter"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type StatsSyncService struct {
	db      *gorm.DB
	factory *adapter.Factory
	logger  *zap.Logger

	syncing     bool
	synced      int
	failedSites []string
	syncMu      sync.Mutex
}

func NewStatsSyncService(db *gorm.DB, factory *adapter.Factory, logger *zap.Logger) *StatsSyncService {
	return &StatsSyncService{db: db, factory: factory, logger: logger}
}

type SyncAllResult struct {
	Running     bool     `json:"running"`
	Synced      int      `json:"synced"`
	Failed      int      `json:"failed"`
	FailedSites []string `json:"failedSites"`
}

func (s *StatsSyncService) GetSyncAllStatus() SyncAllResult {
	s.syncMu.Lock()
	defer s.syncMu.Unlock()
	return SyncAllResult{
		Running:     s.syncing,
		Synced:      s.synced,
		Failed:      len(s.failedSites),
		FailedSites: s.failedSites,
	}
}

func (s *StatsSyncService) StartSyncAll() bool {
	s.syncMu.Lock()
	if s.syncing {
		s.syncMu.Unlock()
		return false
	}
	s.syncing = true
	s.synced = 0
	s.failedSites = nil
	s.syncMu.Unlock()

	go func() {
		synced, failedSites := s.SyncAllSites(context.Background())
		s.syncMu.Lock()
		s.synced = synced
		s.failedSites = failedSites
		s.syncing = false
		s.syncMu.Unlock()
	}()
	return true
}

func (s *StatsSyncService) SyncSiteStats(ctx context.Context, siteID uint) error {
	var site model.Site
	if err := s.db.WithContext(ctx).First(&site, siteID).Error; err != nil {
		return err
	}

	return s.syncSingleSite(ctx, &site)
}

func (s *StatsSyncService) SyncSelectedSites(ctx context.Context, ids []uint) (synced int, failedSites []string) {
	var sites []model.Site
	if err := s.db.WithContext(ctx).Where("id IN ?", ids).Find(&sites).Error; err != nil {
		s.logger.Warn("query selected sites for sync", zap.Error(err))
		return 0, nil
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 5)

	for i := range sites {
		site := &sites[i]
		creds := siteCreds(site)
		if !hasCreds(creds, site.AuthType) {
			mu.Lock()
			failedSites = append(failedSites, site.Name)
			mu.Unlock()
			continue
		}

		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			siteCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
			defer cancel()

			if err := s.syncSingleSite(siteCtx, site); err != nil {
				s.logger.Warn("site stats sync failed",
					zap.String("site", site.Name),
					zap.Error(err))
				mu.Lock()
				failedSites = append(failedSites, site.Name)
				mu.Unlock()
			} else {
				mu.Lock()
				synced++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return
}

func (s *StatsSyncService) SyncAllSites(ctx context.Context) (synced int, failedSites []string) {
	var sites []model.Site
	if err := s.db.WithContext(ctx).Where("enabled = ?", true).Find(&sites).Error; err != nil {
		s.logger.Warn("query enabled sites for sync", zap.Error(err))
		return 0, nil
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 10)

	for i := range sites {
		site := &sites[i]
		creds := siteCreds(site)
		if !hasCreds(creds, site.AuthType) {
			continue
		}

		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			siteCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
			defer cancel()

			if err := s.syncSingleSite(siteCtx, site); err != nil {
				s.logger.Warn("site stats sync failed",
					zap.String("site", site.Name),
					zap.Error(err))
				mu.Lock()
				failedSites = append(failedSites, site.Name)
				mu.Unlock()
			} else {
				mu.Lock()
				synced++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return
}

func (s *StatsSyncService) syncSingleSite(ctx context.Context, site *model.Site) error {
	transport := &http.Transport{
		DialContext:         (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: site.SkipSSLVerify},
		TLSHandshakeTimeout: 10 * time.Second,
		ForceAttemptHTTP2:   true,
		MaxIdleConnsPerHost: 5,
	}
	if site.ProxyURL != "" {
		if pu, err := url.Parse(site.ProxyURL); err == nil {
			transport.Proxy = http.ProxyURL(pu)
		}
	}
	doer := &adapter.HTTPDoer{Client: &http.Client{Timeout: 20 * time.Second, Transport: transport}}
	a := s.factory.Create(site.Framework, doer)

	config := &model.SiteConfig{
		Domain:        site.BaseURL,
		Enabled:       site.Enabled,
		Cookie:        site.Cookie,
		APIKey:        site.APIKey,
		Passkey:       site.Passkey,
		BearerToken:   site.BearerToken,
		AuthKey:       site.AuthKey,
		AuthHash:      site.AuthHash,
		UserID:        site.UserID,
		ProxyURL:      site.ProxyURL,
		SkipSSLVerify: site.SkipSSLVerify,
	}

	var alts []string
	if site.AlternativeDomains != "" {
		json.Unmarshal([]byte(site.AlternativeDomains), &alts)
		config.AlternativeDomains = alts
	}

	stats, err := a.FetchUserStats(ctx, config)
	if err != nil && len(alts) > 0 {
		config.Domain = alts[0]
		stats, err = a.FetchUserStats(ctx, config)
	}
	if err != nil {
		return err
	}

	now := time.Now()
	updates := map[string]interface{}{
		"username":        stats.Username,
		"user_class":      stats.UserClass,
		"upload_bytes":    stats.UploadBytes,
		"download_bytes":  stats.DownloadBytes,
		"ratio":           stats.Ratio,
		"bonus_points":    stats.BonusPoints,
		"seeding_points":  stats.SeedingPoints,
		"seeding_size":    stats.SeedingSize,
		"seeding_count":   stats.SeedingCount,
		"stats_synced_at": now,
	}
	if stats.Passkey != "" {
		updates["passkey"] = stats.Passkey
	}
	if stats.RSSKey != "" {
		updates["rss_key"] = stats.RSSKey
	}
	if stats.AuthKey != "" {
		updates["auth_key"] = stats.AuthKey
	}

	return s.db.WithContext(ctx).Model(site).Updates(updates).Error
}

func siteCreds(site *model.Site) *model.SiteConfig {
	return &model.SiteConfig{
		Cookie:      site.Cookie,
		APIKey:      site.APIKey,
		Passkey:     site.Passkey,
		BearerToken: site.BearerToken,
	}
}

func hasCreds(config *model.SiteConfig, authType string) bool {
	switch authType {
	case "cookie":
		return config.Cookie != ""
	case "apikey":
		return config.APIKey != ""
	case "passkey":
		return config.Passkey != ""
	default:
		return config.Cookie != ""
	}
}
