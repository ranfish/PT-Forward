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
		if saveErr := s.db.Save(history).Error; err != nil {
			s.logger.Error("save sync history failed", zap.Error(saveErr))
		}
		return history, err
	}

	var cookies map[string][]CookieData
	if cfg.Mode == "builtin" {
		if cfg.LastEncrypted == "" {
			history.Status = "completed"
			history.SkippedSites = len(sites)
			history.SyncDuration = time.Since(start)
			s.db.Save(history)
			s.logger.Info("builtin mode: no encrypted data yet, skipping sync")
			return history, nil
		}
		cookies, err = DecryptFromEncrypted(cfg.LastEncrypted, cfg.UUID, cfg.Password)
	} else {
		cookies, err = FetchAndDecrypt(cfg.ServerURL, cfg.UUID, cfg.Password)
	}
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

func (s *SyncService) SyncFromEncrypted(ctx context.Context, encrypted string) (*model.CookieCloudSyncHistory, error) {
	start := time.Now()
	history := &model.CookieCloudSyncHistory{
		Status:     "running",
		UpdateTime: start,
		CreatedAt:  start,
	}
	s.db.Create(history)

	cfg, err := s.getConfig(ctx)
	if err != nil {
		history.Status = "failed"
		history.ErrorMessage = err.Error()
		s.db.Save(history)
		return history, err
	}

	cookies, err := DecryptFromEncrypted(encrypted, cfg.UUID, cfg.Password)
	if err != nil {
		history.Status = "failed"
		history.ErrorMessage = fmt.Sprintf("decrypt failed: %v", err)
		s.db.Save(history)
		return history, err
	}

	var sites []model.Site
	s.db.WithContext(ctx).Where("cookie_cloud_sync = ? AND enabled = ?", true, true).Find(&sites)
	if len(sites) == 0 {
		history.Status = "completed"
		s.db.Save(history)
		return history, nil
	}

	synced := 0
	skipped := 0
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
		if err := s.db.WithContext(ctx).Model(site).Update("cookie", cookieStr).Error; err != nil {
			skipped++
			continue
		}
		synced++
		s.logger.Info("cookie synced (builtin push)",
			zap.String("site", site.Name),
			zap.Int("cookies", len(matched)))
	}

	history.Status = "completed"
	history.SyncedSites = synced
	history.SkippedSites = skipped
	history.SyncDuration = time.Since(start)
	s.db.Save(history)
	s.updateConfigSyncTime(ctx)

	s.logger.Info("cookie cloud builtin sync completed",
		zap.Int("synced", synced),
		zap.Int("skipped", skipped))

	return history, nil
}

func (s *SyncService) GetConfig(ctx context.Context) (*model.CookieCloudConfig, error) {
	return s.getConfig(ctx)
}

// syncLoopIdleWait 未启用/builtin/耗时≥interval 时的短轮询间隔：
// 配置变更（开启自动同步/改间隔）最快 30s 内被循环感知。
const syncLoopIdleWait = 30 * time.Second

// cookiecloudLoopWait §59.498: 同步循环等待计算（纯函数·测试锚）。
// 语义 = 用户心智模型的"每 interval 分钟执行一次"（cron 式周期）：
// 开始到开始 = interval（SyncAll 耗时从间隔中扣除）。
func cookiecloudLoopWait(enabled bool, mode string, intervalMin int, elapsed time.Duration) time.Duration {
	if !enabled || mode == "builtin" {
		return syncLoopIdleWait
	}
	interval := time.Duration(intervalMin) * time.Minute
	if interval < 5*time.Minute {
		interval = 5 * time.Minute // 与旧调度任务同款下限（handler 另钳 >0→60）
	}
	if d := interval - elapsed; d > syncLoopIdleWait {
		return d
	}
	return syncLoopIdleWait // 耗时≥interval-30s：兜底防热循环
}

// StartSyncLoop §59.498: CookieCloud 周期同步循环（对齐 IYUU StartSyncLoop
// 模式——系统内两处"用户可配间隔"调度需求同款解法）。
//
// 替代旧 cookiecloud_sync 调度任务（固定 */5 tick + 距【完成】时刻比较的
// 杂交模式）——其 off-by-epsilon 缺陷：LastSyncAt 记完成时刻+SyncAll 耗时
// ~3s → "正好 interval"的 tick 永远差 3 秒被严格小于判断 skip → 10 分钟
// 配置实际跑 15 分钟周期（29 实证 15:25→16:40 严格 15 分钟六条）。
//
// 语义：每 interval 分钟执行一次（开始到开始）；未启用/builtin 模式
// （浏览器插件推送·无需拉取）30s 短轮询感知配置变化；手动"立即同步"
// 不干扰循环节奏（LastSyncAt 仅作展示）。
func (s *SyncService) StartSyncLoop(ctx context.Context) {
	for {
		var cfg model.CookieCloudConfig
		start := time.Now()
		ran := false
		if err := s.db.WithContext(ctx).First(&cfg).Error; err == nil &&
			cfg.SyncEnabled && cfg.Mode != "builtin" {
			ran = true
			if history, err := s.SyncAll(ctx); err != nil {
				s.logger.Warn("cookiecloud scheduled sync failed", zap.Error(err))
			} else {
				s.logger.Info("cookiecloud scheduled sync completed",
					zap.Int("synced", history.SyncedSites),
					zap.Int("skipped", history.SkippedSites))
			}
		}
		var elapsed time.Duration
		if ran {
			elapsed = time.Since(start)
		}
		// cfg 零值（无配置行/读失败）→ SyncEnabled=false → 短轮询 ✓
		wait := cookiecloudLoopWait(cfg.SyncEnabled, cfg.Mode, cfg.SyncInterval, elapsed)
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return
		}
	}
}

func (s *SyncService) GetDB() *gorm.DB {
	return s.db
}

func (s *SyncService) getConfig(ctx context.Context) (*model.CookieCloudConfig, error) {
	var cfg model.CookieCloudConfig
	if err := s.db.WithContext(ctx).First(&cfg).Error; err != nil {
		return nil, ccError(ErrCCConfig, "cookie cloud config not found", err)
	}
	if !cfg.SyncEnabled {
		return nil, ccError(ErrCCConfig, "cookie cloud sync is disabled", nil)
	}
	if cfg.Mode == "builtin" {
		if cfg.UUID == "" || cfg.Password == "" {
			return nil, ccError(ErrCCConfig, "cookie cloud config incomplete (uuid/password required for builtin mode)", nil)
		}
		return &cfg, nil
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
