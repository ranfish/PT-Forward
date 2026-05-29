package cookiecloud

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type SyncService struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewSyncService(db *gorm.DB, logger *zap.Logger) *SyncService {
	return &SyncService{db: db, logger: logger}
}

func (s *SyncService) SyncAll(ctx context.Context) (*model.CookieCloudSyncHistory, error) {
	history := &model.CookieCloudSyncHistory{
		Status:     "running",
		UpdateTime: time.Now(),
		CreatedAt:  time.Now(),
	}
	if err := s.db.Create(history).Error; err != nil {
		s.logger.Error("create sync history failed", zap.Error(err))
		return nil, ccError(ErrCCSync, "create history", err)
	}

	start := time.Now()

	var sites []model.Site
	if err := s.db.WithContext(ctx).
		Where("cookie_cloud_sync = ? AND enabled = ?", true, true).
		Find(&sites).Error; err != nil {
		history.Status = "failed"
		history.ErrorMessage = err.Error()
		if saveErr := s.db.Save(history).Error; saveErr != nil {
			s.logger.Error("save sync history failed", zap.Error(saveErr))
		}
		return history, ccError(ErrCCSync, "query sites", err)
	}

	if len(sites) == 0 {
		history.Status = "completed"
		history.SyncedSites = 0
		if saveErr := s.db.Save(history).Error; saveErr != nil {
			s.logger.Error("save sync history failed", zap.Error(saveErr))
		}
		return history, nil
	}

	cfg, err := s.getConfig(ctx)
	if err != nil {
		history.Status = "failed"
		history.ErrorMessage = err.Error()
		if saveErr := s.db.Save(history).Error; saveErr != nil {
			s.logger.Error("save sync history failed", zap.Error(saveErr))
		}
		return history, err
	}

	cookies, err := FetchAndDecrypt(cfg.ServerURL, cfg.UUID, cfg.Password)
	if err != nil {
		history.Status = "failed"
		history.ErrorMessage = fmt.Sprintf("fetch/decrypt failed: %v", err)
		if saveErr := s.db.Save(history).Error; saveErr != nil {
			s.logger.Error("save sync history failed", zap.Error(saveErr))
		}
		return history, err
	}

	synced := 0
	skipped := 0
	var errors []model.CookieCloudSyncError

	for i := range sites {
		site := &sites[i]
		domains := s.buildCookieDomains(site)
		if len(domains) == 0 {
			skipped++
			continue
		}

		var matched []CookieData
		seen := make(map[string]bool)
		for _, domain := range domains {
			for _, c := range FilterCookiesByDomain(cookies, domain) {
				key := c.Name + "@" + c.Domain
				if !seen[key] {
					seen[key] = true
					matched = append(matched, c)
				}
			}
		}
		if len(matched) == 0 {
			skipped++
			continue
		}

		cookieStr := CookiesToString(matched)
		if err := s.db.WithContext(ctx).Model(site).
			Update("cookie", cookieStr).Error; err != nil {
			skipped++
			errors = append(errors, model.CookieCloudSyncError{
				Phase:     "update_cookie",
				Message:   fmt.Sprintf("site %s: %v", site.Name, err),
				Retryable: true,
				Timestamp: time.Now(),
			})
			continue
		}

		synced++
		s.logger.Info("cookie synced",
			zap.String("site", site.Name),
			zap.Int("cookies", len(matched)),
		)
	}

	history.Status = "completed"
	history.SyncedSites = synced
	history.SkippedSites = skipped
	history.SyncDuration = time.Since(start)
	if len(errors) > 0 {
		history.Errors = errors
		errJSON, _ := json.Marshal(errors)
		history.ErrorMessage = string(errJSON)
	}

	if saveErr := s.db.Save(history).Error; saveErr != nil {
		s.logger.Error("save sync history failed", zap.Error(saveErr))
	}

	s.updateConfigSyncTime(ctx)

	s.logger.Info("cookie cloud sync completed",
		zap.Int("synced", synced),
		zap.Int("skipped", skipped),
	)

	return history, nil
}

func (s *SyncService) getConfig(ctx context.Context) (*model.CookieCloudConfig, error) {
	var cfg model.CookieCloudConfig
	if err := s.db.WithContext(ctx).First(&cfg).Error; err != nil {
		return nil, ccError(ErrCCConfig, "cookie cloud config not found", err)
	}
	if !cfg.SyncEnabled {
		return nil, ccError(ErrCCConfig, "cookie cloud sync is disabled", nil)
	}
	if cfg.ServerURL == "" || cfg.UUID == "" || cfg.Password == "" {
		return nil, ccError(ErrCCConfig, "cookie cloud config incomplete (url/uuid/password required)", nil)
	}
	return &cfg, nil
}

func (s *SyncService) updateConfigSyncTime(ctx context.Context) {
	now := time.Now()
	s.db.Model(&model.CookieCloudConfig{}).
		Where("1 = 1").
		Update("last_sync_at", &now)
}

func extractDomain(baseURL string) string {
	baseURL = strings.TrimPrefix(baseURL, "https://")
	baseURL = strings.TrimPrefix(baseURL, "http://")
	baseURL = strings.TrimSuffix(baseURL, "/")
	parts := strings.SplitN(baseURL, "/", 2)
	return parts[0]
}

func (s *SyncService) buildCookieDomains(site *model.Site) []string {
	var domains []string
	primary := ""
	if site.CookieCloudDomain != "" {
		primary = site.CookieCloudDomain
	} else {
		primary = extractDomain(site.BaseURL)
	}
	if primary != "" {
		domains = append(domains, primary)
	}
	if site.AlternativeDomains != "" {
		var alts []string
		if err := json.Unmarshal([]byte(site.AlternativeDomains), &alts); err == nil {
			for _, alt := range alts {
				domain := extractDomain(alt)
				if domain != "" && domain != primary {
					domains = append(domains, domain)
				}
			}
		}
	}
	return domains
}
