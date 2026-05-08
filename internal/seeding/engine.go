package seeding

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
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
	freeEndMonitor *FreeEndMonitor
	mu             sync.RWMutex
	recordMap      map[string]*model.SeedingTorrentRecord
	emaStates      map[string]*emaState
}

type emaState struct {
	UploadSpeed   float64
	DownloadSpeed float64
}

const emaAlpha = 0.3

func NewEngine(db *gorm.DB, logger *zap.Logger) *Engine {
	e := &Engine{
		db:        db,
		logger:    logger,
		recordMap: make(map[string]*model.SeedingTorrentRecord),
		emaStates: make(map[string]*emaState),
	}
	e.freeEndMonitor = NewFreeEndMonitor(db, nil, logger)
	e.freeEndMonitor.SetEngine(e)
	return e
}

func (e *Engine) SetClientProvider(cp model.DownloaderProvider) {
	e.clientProvider = cp
	if e.freeEndMonitor != nil {
		e.freeEndMonitor.client = cp
	}
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
		return seedingError(ErrSeedingDB, "load seeding records", err)
	}

	e.mu.Lock()
	for i := range records {
		key := recordKey(records[i].ClientID, records[i].InfoHash)
		e.recordMap[key] = &records[i]
	}
	e.mu.Unlock()

	e.logger.Info("seeding engine started", zap.Int("records", len(records)))

	if e.freeEndMonitor != nil {
		e.freeEndMonitor.RecoverOnStartup(ctx)
	}

	return nil
}

func (e *Engine) Stop(_ context.Context) error {
	if e.freeEndMonitor != nil {
		e.freeEndMonitor.StopAll()
	}
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
	e.mu.Unlock()

	record.Status = model.SeedingStatusSeeding
	if err := e.db.WithContext(ctx).Create(record).Error; err != nil {
		return err
	}

	e.mu.Lock()
	e.recordMap[key] = record
	e.mu.Unlock()

	if e.freeEndMonitor != nil {
		e.freeEndMonitor.Schedule(record)
	}
	return nil
}

func (e *Engine) RemoveSeedingRecord(ctx context.Context, clientID, infoHash string) error {
	key := recordKey(clientID, infoHash)

	e.mu.Lock()
	delete(e.recordMap, key)
	e.mu.Unlock()

	if e.freeEndMonitor != nil {
		e.freeEndMonitor.Cancel(clientID, infoHash)
	}

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
			FreeEndAt: ev.FreeEndAt,
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

	scoringCutoff := time.Now().Add(-72 * time.Hour)
	e.db.WithContext(ctx).Where("created_at < ?", scoringCutoff).Delete(&model.ScoringLog{})

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

	if cfg != nil {
		minScore, minAgeHours, weights = e.applyConfig(cfg, minScore, minAgeHours, weights)
	}

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

	e.updateEMA(ctx, clientID, maindata, torrentMap)

	var cascadeRules []model.DeleteRule
	if cfg != nil && cfg.DeleteRuleIDs != "" {
		e.db.WithContext(ctx).
			Where("id IN (?) AND enabled = ? AND cascade_delete = ?", splitRuleIDs(cfg.DeleteRuleIDs), true, true).
			Find(&cascadeRules)
	}

	cycleID := time.Now().Format("20060102-150405")

	for i := range records {
		rec := &records[i]
		if rec.Status != model.SeedingStatusSeeding {
			continue
		}

		result.Evaluated++

		ti, ok := torrentMap[rec.InfoHash]
		if !ok || ti == nil {
			continue
		}

		ageHours := time.Since(rec.CreatedAt).Hours()
		seedTimeHours := float64(ti.SeedTime) / 3600.0

		hrStrategy := "protect"
		if e.siteProvider != nil && rec.SiteName != "" {
			if siteCfg, err := e.siteProvider.GetSiteConfig(ctx, rec.SiteName); err == nil && siteCfg != nil {
				if siteCfg.HRStrategy == "skip" || siteCfg.HRStrategy == "ignore" {
					hrStrategy = siteCfg.HRStrategy
				}
			}
		}

		candidate := CleanupCandidate{
			ID:            rec.ID,
			InfoHash:      rec.InfoHash,
			SeedTimeHours: seedTimeHours,
			AgeHours:      ageHours,
			IsFree:        rec.IsFree,
			HasHR:         rec.HasHR,
			HRSeedTimeH:   rec.HRSeedTimeH,
			HRStrategy:    hrStrategy,
			Discount:      rec.Discount,
			FreeEndAt:     rec.FreeEndAt,
		}
		candidate.UploadSpeed = ti.UploadSpeed

		score := CalculateCleanupScore(candidate, weights)
		candidate.Score = score

		if score < 5.0 {
			e.db.WithContext(ctx).Create(&model.ScoringLog{
				CycleID:  cycleID,
				ClientID: clientID,
				InfoHash: rec.InfoHash,
				SiteName: rec.SiteName,
				Score:    score,
			})
		}

		if !ShouldCleanup(candidate, minScore, minAgeHours) {
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
					if err := e.UpdateStatus(ctx, rec.ID, model.SeedingStatusPausedRule, "disk_protect"); err != nil {
						e.logger.Error("更新磁盘保护状态失败", zap.Uint("id", rec.ID), zap.Error(err))
					}
					result.Paused++
				}
			}
			continue
		}

		isDeleteFiles := true

		if HasSameFileTorrent(ti, torrents) {
			isDeleteFiles = false
			e.logger.Debug("辅种文件保护：共享文件，仅删种子不删文件",
				zap.String("infoHash", rec.InfoHash),
				zap.String("name", ti.Name),
			)
		}

		cascaded := false
		for _, rule := range cascadeRules {
			relatedHashes := FindRelatedByTagOrPath(ti, torrents, rule.CascadeMaxDepth)
			if len(relatedHashes) > 0 {
				allHashes := append([]string{rec.InfoHash}, relatedHashes...)
				if err := dlClient.BatchDeleteTorrents(ctx, allHashes, isDeleteFiles); err != nil {
					e.logger.Warn("级联删种失败",
						zap.Strings("hashes", allHashes),
						zap.Error(err),
					)
					result.Errors++
				} else {
					e.logger.Info("级联删种成功",
						zap.String("source", rec.InfoHash),
						zap.Int("related", len(relatedHashes)),
					)
				}
				if err := e.UpdateStatus(ctx, rec.ID, model.SeedingStatusDeleting, "auto_cleanup"); err != nil {
					e.logger.Error("更新级联删种状态失败", zap.Uint("id", rec.ID), zap.Error(err))
				}
				result.Deleted++
				cascaded = true
				break
			}
		}

		if !cascaded {
			if err := dlClient.DeleteTorrent(ctx, rec.InfoHash, isDeleteFiles); err != nil {
				e.logger.Warn("删种失败",
					zap.String("infoHash", rec.InfoHash),
					zap.Bool("deleteFiles", isDeleteFiles),
					zap.Error(err),
				)
				result.Errors++
				continue
			}

			if err := e.UpdateStatus(ctx, rec.ID, model.SeedingStatusDeleting, "auto_cleanup"); err != nil {
				e.logger.Error("更新删种状态失败", zap.Uint("id", rec.ID), zap.Error(err))
			}
			result.Deleted++
		}
	}

	return result, nil
}

func (e *Engine) ListConfigs(ctx context.Context) ([]*model.SeedingClientConfig, error) {
	var configs []*model.SeedingClientConfig
	err := e.db.WithContext(ctx).Where("enabled = ?", true).Find(&configs).Error
	return configs, err
}

func (e *Engine) GetConfigByID(ctx context.Context, id uint) (*model.SeedingClientConfig, error) {
	var cfg model.SeedingClientConfig
	if err := e.db.WithContext(ctx).First(&cfg, id).Error; err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (e *Engine) CreateConfig(ctx context.Context, cfg *model.SeedingClientConfig) error {
	return e.db.WithContext(ctx).Create(cfg).Error
}

func (e *Engine) UpdateConfig(ctx context.Context, cfg *model.SeedingClientConfig) error {
	return e.db.WithContext(ctx).Save(cfg).Error
}

func (e *Engine) DeleteConfig(ctx context.Context, id uint) error {
	return e.db.WithContext(ctx).Delete(&model.SeedingClientConfig{}, id).Error
}

func (e *Engine) Add(ctx context.Context, clientID string, event *model.TorrentEvent) error {
	record := &model.SeedingTorrentRecord{
		ClientID:    clientID,
		SiteName:    event.SiteName,
		TorrentID:   event.TorrentID,
		InfoHash:    event.InfoHash,
		Discount:    event.Discount,
		HasHR:       event.HasHR,
		HRSeedTimeH: event.HRSeedTimeH,
		Source:      "rss",
		Status:      model.SeedingStatusSeeding,
	}
	return e.db.WithContext(ctx).Create(record).Error
}

func (e *Engine) Flush(ctx context.Context, subscriptionID string) ([]*model.SeedingCandidate, error) {
	return []*model.SeedingCandidate{}, nil
}

func (e *Engine) Clear(ctx context.Context, clientID string) error {
	return e.db.WithContext(ctx).
		Where("client_id = ? AND source = ?", clientID, "rss").
		Delete(&model.SeedingTorrentRecord{}).Error
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
			totalUpload += t.UploadSpeed
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

func splitRuleIDs(s string) []string {
	parts := strings.Split(s, ",")
	var ids []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if _, err := strconv.Atoi(p); err == nil {
			ids = append(ids, p)
		}
	}
	return ids
}

func (e *Engine) applyConfig(cfg *model.SeedingClientConfig, defaultScore float64, defaultAge float64, defaultWeights CleanupWeights) (float64, float64, CleanupWeights) {
	minScore := defaultScore
	minAgeHours := defaultAge
	weights := defaultWeights

	if cfg.CleanupScoreWeights != "" {
		var w model.CleanupScoreWeights
		if err := json.Unmarshal([]byte(cfg.CleanupScoreWeights), &w); err == nil {
			if w.SeedHours > 0 {
				weights.SeedHours = w.SeedHours
			}
			if w.UploadSpeed > 0 {
				weights.UploadSpeed = w.UploadSpeed
			}
			if w.Ratio > 0 {
				weights.Ratio = w.Ratio
			}
			if w.DiskUsage > 0 {
				weights.DiskUsage = w.DiskUsage
			}
		}
	}

	return minScore, minAgeHours, weights
}

func (e *Engine) updateEMA(ctx context.Context, clientID string, maindata *model.Maindata, torrentMap map[string]*model.TorrentInfo) {
	var totalUp, totalDown int64
	for _, ti := range torrentMap {
		totalUp += ti.UploadSpeed
		totalDown += ti.DownloadSpeed
	}

	e.mu.Lock()
	state, ok := e.emaStates[clientID]
	if !ok {
		state = &emaState{}
		e.emaStates[clientID] = state
	}

	newUp := float64(totalUp)
	newDown := float64(totalDown)

	if state.UploadSpeed == 0 && state.DownloadSpeed == 0 {
		state.UploadSpeed = newUp
		state.DownloadSpeed = newDown
	} else {
		state.UploadSpeed = emaAlpha*newUp + (1-emaAlpha)*state.UploadSpeed
		state.DownloadSpeed = emaAlpha*newDown + (1-emaAlpha)*state.DownloadSpeed
	}
	e.mu.Unlock()

	var dbState model.SeedingClientState
	err := e.db.WithContext(ctx).Where("client_id = ?", clientID).First(&dbState).Error
	if err != nil {
		dbState = model.SeedingClientState{
			ClientID:         clientID,
			AvgUploadSpeed:   state.UploadSpeed,
			AvgDownloadSpeed: state.DownloadSpeed,
			Initialized:      true,
		}
		e.db.WithContext(ctx).Create(&dbState)
	} else {
		e.db.WithContext(ctx).Model(&dbState).Updates(map[string]interface{}{
			"avg_upload_speed":   state.UploadSpeed,
			"avg_download_speed": state.DownloadSpeed,
			"initialized":        true,
		})
	}
}
