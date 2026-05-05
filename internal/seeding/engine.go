package seeding

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ranfish/pt-forward/internal/dispatcher"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Engine struct {
	db             *gorm.DB
	logger         *zap.Logger
	clientProvider model.DownloaderProvider
	siteProvider   model.SiteInfoProvider
	mu             sync.RWMutex
	recordMap      map[string]*model.SeedingTorrentRecord
}

func NewEngine(db *gorm.DB, logger *zap.Logger) *Engine {
	return &Engine{
		db:        db,
		logger:    logger,
		recordMap: make(map[string]*model.SeedingTorrentRecord),
	}
}

func (e *Engine) SetClientProvider(cp model.DownloaderProvider) {
	e.clientProvider = cp
}

func (e *Engine) SetSiteProvider(sp model.SiteInfoProvider) {
	e.siteProvider = sp
}

func recordKey(clientID, infoHash string) string {
	return clientID + ":" + infoHash
}

func (e *Engine) Start(ctx context.Context) error {
	var records []model.SeedingTorrentRecord
	if err := e.db.WithContext(ctx).
		Where("status IN ?", []string{"seeding", "paused_free_end", "paused_rule"}).
		Find(&records).Error; err != nil {
		return fmt.Errorf("load seeding records: %w", err)
	}

	e.mu.Lock()
	for i := range records {
		key := recordKey(records[i].ClientID, records[i].InfoHash)
		e.recordMap[key] = &records[i]
	}
	e.mu.Unlock()

	e.logger.Info("seeding engine started", zap.Int("records", len(records)))
	return nil
}

func (e *Engine) Stop(_ context.Context) error {
	e.logger.Info("seeding engine stopped")
	return nil
}

func (e *Engine) AddSeedingRecord(ctx context.Context, record *model.SeedingTorrentRecord) error {
	if record.ClientID == "" || record.InfoHash == "" {
		return &model.AppError{Code: 40001, Message: "client_id and info_hash are required"}
	}

	key := recordKey(record.ClientID, record.InfoHash)

	e.mu.Lock()
	if _, exists := e.recordMap[key]; exists {
		e.mu.Unlock()
		return &model.AppError{Code: 40900, Message: fmt.Sprintf("seeding record already exists: %s", key)}
	}
	e.recordMap[key] = record
	e.mu.Unlock()

	record.Status = model.SeedingStatusSeeding
	return e.db.WithContext(ctx).Create(record).Error
}

func (e *Engine) RemoveSeedingRecord(ctx context.Context, clientID, infoHash string) error {
	key := recordKey(clientID, infoHash)

	e.mu.Lock()
	delete(e.recordMap, key)
	e.mu.Unlock()

	return e.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).
		Where("client_id = ? AND info_hash = ?", clientID, infoHash).
		Update("status", model.SeedingStatusDeleted).Error
}

func (e *Engine) GetActiveCount(clientID string) int {
	e.mu.RLock()
	defer e.mu.RUnlock()

	count := 0
	for _, r := range e.recordMap {
		if r.ClientID == clientID && r.Status == model.SeedingStatusSeeding {
			count++
		}
	}
	return count
}

func (e *Engine) GetRecord(clientID, infoHash string) (*model.SeedingTorrentRecord, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	r, ok := e.recordMap[recordKey(clientID, infoHash)]
	if !ok {
		return nil, false
	}
	return r, true
}

func (e *Engine) ListByClient(ctx context.Context, clientID string) ([]model.SeedingTorrentRecord, error) {
	var records []model.SeedingTorrentRecord
	err := e.db.WithContext(ctx).
		Where("client_id = ? AND status IN ?", clientID,
			[]string{string(model.SeedingStatusSeeding), "paused_free_end", "paused_rule"}).
		Find(&records).Error
	return records, err
}

func (e *Engine) UpdateStatus(ctx context.Context, id uint, status model.SeedingTorrentStatus, actionBy string) error {
	return e.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":         status,
			"last_action_by": actionBy,
			"updated_at":     time.Now(),
		}).Error
}

func (e *Engine) PauseForFreeEnd(ctx context.Context, clientID, infoHash string) error {
	key := recordKey(clientID, infoHash)
	e.mu.Lock()
	if r, ok := e.recordMap[key]; ok {
		r.Status = model.SeedingStatusPausedFreeEnd
	}
	e.mu.Unlock()

	return e.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).
		Where("client_id = ? AND info_hash = ?", clientID, infoHash).
		Updates(map[string]interface{}{
			"status":         model.SeedingStatusPausedFreeEnd,
			"last_action_by": "free_end_pauser",
		}).Error
}

func (e *Engine) TotalActiveCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()

	count := 0
	for _, r := range e.recordMap {
		if r.Status == model.SeedingStatusSeeding {
			count++
		}
	}
	return count
}

func (e *Engine) OnTorrents(ctx context.Context, events []model.TorrentEvent) error {
	for i := range events {
		ev := &events[i]
		if ev.SourceID == "" {
			continue
		}

		record := &model.SeedingTorrentRecord{
			ClientID:  dispatcher.GetClientName(ev),
			InfoHash:  ev.InfoHash,
			SiteName:  ev.SiteName,
			TorrentID: ev.TorrentID,
			Status:    model.SeedingStatusSeeding,
			Source:    "rss",
			Discount:  ev.Discount,
			IsFree:    ev.Discount == model.DiscountFree || ev.Discount == model.Discount2xFree,
		}
		if ev.HasHR {
			record.HasHR = true
			record.HRSeedTimeH = ev.HRSeedTimeH
		}

		if err := e.AddSeedingRecord(ctx, record); err != nil {
			e.logger.Debug("skip adding seeding record (may already exist)",
				zap.String("info_hash", ev.InfoHash),
				zap.Error(err),
			)
		}
	}
	return nil
}

func (e *Engine) CleanupStale(ctx context.Context) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -30)
	result := e.db.WithContext(ctx).
		Where("status IN ? AND updated_at < ?", []string{"paused_free_end", "paused_rule", "stopped"}, cutoff).
		Delete(&model.SeedingTorrentRecord{})
	if result.Error != nil {
		return 0, &model.AppError{Code: 50001, Message: "cleanup stale seeding records failed", Cause: result.Error}
	}

	freeExpired := e.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).
		Where("status = ? AND is_free = ? AND free_end_at IS NOT NULL AND free_end_at < ?", "seeding", true, time.Now()).
		Updates(map[string]interface{}{
			"status":     "paused_free_end",
			"updated_at": time.Now(),
		})
	if freeExpired.Error != nil {
		return result.RowsAffected, &model.AppError{Code: 50001, Message: "pause free-expired records failed", Cause: freeExpired.Error}
	}

	e.logger.Info("seeding cleanup completed",
		zap.Int64("deleted_stale", result.RowsAffected),
		zap.Int64("paused_free_expired", freeExpired.RowsAffected),
	)

	return result.RowsAffected + freeExpired.RowsAffected, nil
}

type EvaluateResult struct {
	Evaluated int
	Paused    int
	Deleted   int
	Errors    int
}

func (e *Engine) Evaluate(ctx context.Context, clientID string, cfg *model.SeedingClientConfig) (*EvaluateResult, error) {
	if e.clientProvider == nil {
		return nil, &model.AppError{Code: 50001, Message: "client provider not configured"}
	}

	result := &EvaluateResult{}

	dlClient, err := e.clientProvider.Get(clientID)
	if err != nil {
		return nil, &model.AppError{Code: 50001, Message: "获取下载器失败", Cause: err}
	}

	records, err := e.ListByClient(ctx, clientID)
	if err != nil {
		return nil, &model.AppError{Code: 50001, Message: "查询做种记录失败", Cause: err}
	}

	if len(records) == 0 {
		return result, nil
	}

	weights := DefaultCleanupWeights()
	minScore := 0.3
	minAgeHours := 48.0

	torrents, err := dlClient.GetSeedingTorrents(ctx)
	if err != nil {
		return nil, &model.AppError{Code: 50001, Message: "获取做种列表失败", Cause: err}
	}

	torrentMap := make(map[string]*model.TorrentInfo, len(torrents))
	for _, t := range torrents {
		torrentMap[t.Hash] = t
	}

	maindata, mdErr := dlClient.GetMainData(ctx)
	freeSpace := int64(-1)
	if mdErr == nil && maindata != nil {
		freeSpace = maindata.FreeSpace
	}

	for i := range records {
		rec := &records[i]
		if rec.Status != model.SeedingStatusSeeding {
			continue
		}

		result.Evaluated++

		ti, ok := torrentMap[rec.InfoHash]
		if !ok {
			continue
		}

		ageHours := time.Since(rec.CreatedAt).Hours()
		seedTimeHours := float64(ti.SeedTime) / 3600.0

		candidate := CleanupCandidate{
			ID:            rec.ID,
			InfoHash:      rec.InfoHash,
			SeedTimeHours: seedTimeHours,
			AgeHours:      ageHours,
			IsFree:        rec.IsFree,
			HasHR:         rec.HasHR,
		}

		if cfg != nil {
			_ = minScore
		}

		score := CalculateCleanupScore(candidate, weights)
		candidate.Score = score

		if ShouldCleanup(candidate, minScore, minAgeHours) {
			if err := dlClient.DeleteTorrent(ctx, rec.InfoHash, true); err != nil {
				e.logger.Warn("删种失败",
					zap.String("infoHash", rec.InfoHash),
					zap.Error(err),
				)
				result.Errors++
				continue
			}

			_ = e.UpdateStatus(ctx, rec.ID, model.SeedingStatusDeleting, "auto_cleanup")
			result.Deleted++
		}

		diskProtect := cfg != nil && cfg.DiskProtectEnabled
		if diskProtect && freeSpace >= 0 && cfg.MinDiskSpaceGB > 0 {
			minBytes := int64(cfg.MinDiskSpaceGB * 1024 * 1024 * 1024)
			if freeSpace < minBytes {
				e.logger.Warn("磁盘空间不足，触发磁盘保护",
					zap.Int64("freeSpace", freeSpace),
					zap.Float64("minGB", cfg.MinDiskSpaceGB),
				)
				if err := dlClient.PauseTorrent(ctx, rec.InfoHash); err != nil {
					result.Errors++
					continue
				}
				_ = e.UpdateStatus(ctx, rec.ID, model.SeedingStatusPausedRule, "disk_protect")
				result.Paused++
			}
		}
	}

	return result, nil
}

func (e *Engine) ListConfigs(ctx context.Context) ([]model.SeedingClientConfig, error) {
	var configs []model.SeedingClientConfig
	err := e.db.WithContext(ctx).Where("enabled = ?", true).Find(&configs).Error
	return configs, err
}

func (e *Engine) CollectTrafficStats(ctx context.Context) error {
	if e.clientProvider == nil {
		return nil
	}

	clients := e.clientProvider.ListClients()
	now := time.Now()

	for _, clientID := range clients {
		dlClient, err := e.clientProvider.Get(clientID)
		if err != nil {
			e.logger.Debug("获取下载器失败，跳过", zap.String("clientID", clientID), zap.Error(err))
			continue
		}

		md, err := dlClient.GetMainData(ctx)
		if err != nil {
			e.logger.Debug("获取下载器数据失败，跳过", zap.String("clientID", clientID), zap.Error(err))
			continue
		}

		var totalUpload, totalDownload int64
		for _, t := range md.Torrents {
			totalUpload += t.Uploaded
			totalDownload += t.DownloadSpeed
		}

		snapshot := &model.DownloaderSpeedSnapshot{
			ClientID:       clientID,
			UploadSpeed:    totalUpload,
			DownloadSpeed:  totalDownload,
			FreeSpaceBytes: md.FreeSpace,
			ActiveTorrents: len(md.Torrents),
			RecordedAt:     now,
		}
		if err := e.db.WithContext(ctx).Create(snapshot).Error; err != nil {
			e.logger.Warn("写入下载器速度快照失败", zap.String("clientID", clientID), zap.Error(err))
		}

		e.collectSiteTrafficDaily(ctx, clientID, md, now)
	}

	return nil
}

func (e *Engine) collectSiteTrafficDaily(ctx context.Context, clientID string, md *model.Maindata, now time.Time) {
	today := now.Truncate(24 * time.Hour)

	siteTraffic := make(map[string]*model.SiteTrafficDaily)

	var records []model.SeedingTorrentRecord
	e.db.WithContext(ctx).
		Where("client_id = ? AND status IN ?", clientID,
			[]string{string(model.SeedingStatusSeeding), "paused_free_end", "paused_rule"}).
		Find(&records)

	siteCount := make(map[string]int)
	for _, rec := range records {
		siteCount[rec.SiteName]++
	}

	for siteName, count := range siteCount {
		var existing model.SiteTrafficDaily
		err := e.db.WithContext(ctx).
			Where("site_name = ? AND date = ?", siteName, today).
			First(&existing).Error

		if err != nil {
			entry := &model.SiteTrafficDaily{
				SiteName:     siteName,
				Date:         today,
				SeedingCount: count,
				TorrentCount: count,
			}
			_ = e.db.WithContext(ctx).Create(entry).Error
			siteTraffic[siteName] = entry
		} else {
			e.db.WithContext(ctx).Model(&existing).Updates(map[string]interface{}{
				"seeding_count": count,
			})
			siteTraffic[siteName] = &existing
		}
	}
}
