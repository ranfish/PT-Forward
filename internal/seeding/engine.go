package seeding

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ranfish/pt-forward/internal/dispatcher"
	"github.com/ranfish/pt-forward/internal/event"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	scoringCutoffHours = 72 * time.Hour
	syncGracePeriod    = 15 * time.Minute
	syncHardTimeout    = 2 * time.Hour
	sqliteVarLimit     = 500
)

func (e *Engine) syncStaleRecords(ctx context.Context, clientID string, torrentMap map[string]*model.TorrentInfo) {
	hashSet := make(map[string]bool, len(torrentMap))
	lowerMap := make(map[string]*model.TorrentInfo, len(torrentMap))
	for hash, ti := range torrentMap {
		lh := strings.ToLower(hash)
		hashSet[lh] = true
		lowerMap[lh] = ti
	}

	e.mu.Lock()
	var staleKeys []string
	var staleHashes []string
	now := time.Now()
	for key, rec := range e.recordMap {
		if rec.ClientID != clientID {
			continue
		}
		if rec.Status == model.SeedingStatusDeleted {
			continue
		}
		if rec.Status == model.SeedingStatusDeleting {
			if !hashSet[strings.ToLower(rec.InfoHash)] && now.Sub(rec.UpdatedAt) > 5*time.Minute {
				staleKeys = append(staleKeys, key)
				staleHashes = append(staleHashes, rec.InfoHash)
				rec.Status = model.SeedingStatusDeleted
			}
			continue
		}
		if !hashSet[strings.ToLower(rec.InfoHash)] {
			if rec.FlushedAt == nil && now.Sub(rec.CreatedAt) < syncHardTimeout {
				continue
			}
			if rec.FlushedAt != nil && now.Sub(*rec.FlushedAt) < syncGracePeriod {
				continue
			}
			staleKeys = append(staleKeys, key)
			staleHashes = append(staleHashes, rec.InfoHash)
			rec.Status = model.SeedingStatusDeleted
		}
	}
	for _, key := range staleKeys {
		delete(e.recordMap, key)
	}
	e.mu.Unlock()

	if len(staleHashes) > 0 {
		e.logger.Info("synced stale seeding records",
			zap.String("client_id", clientID),
			zap.Int("stale_count", len(staleHashes)))
		for i := 0; i < len(staleHashes); i += sqliteVarLimit {
			end := i + sqliteVarLimit
			if end > len(staleHashes) {
				end = len(staleHashes)
			}
			e.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).
				Where("client_id = ? AND info_hash IN ?", clientID, staleHashes[i:end]).
				Updates(map[string]interface{}{
					"status":     model.SeedingStatusDeleted,
					"updated_at": time.Now(),
				})
		}
	}

	var orphanHashes []string
	for hash := range hashSet {
		orphanHashes = append(orphanHashes, hash)
	}
	if len(orphanHashes) == 0 {
		return
	}

	var existingRecords []model.SeedingTorrentRecord
	for i := 0; i < len(orphanHashes); i += sqliteVarLimit {
		end := i + sqliteVarLimit
		if end > len(orphanHashes) {
			end = len(orphanHashes)
		}
		var partial []model.SeedingTorrentRecord
		if dbErr := e.db.WithContext(ctx).
			Where("client_id = ? AND LOWER(info_hash) IN ? AND status = ?", clientID, orphanHashes[i:end], model.SeedingStatusDeleted).
			Find(&partial).Error; dbErr != nil {
			e.logger.Warn("syncStale: query orphan records failed", zap.String("client_id", clientID), zap.Error(dbErr))
			return
		}
		existingRecords = append(existingRecords, partial...)
	}

	if len(existingRecords) > 0 {
		var recoverHashes []string
		deadStates := map[string]bool{
			"stalledDL":    true,
			"missingFiles": true,
			"error":        true,
			"unknown":      true,
		}
		for _, rec := range existingRecords {
			lowerHash := strings.ToLower(rec.InfoHash)
			if !hashSet[lowerHash] {
				continue
			}
			if ti := lowerMap[lowerHash]; ti != nil {
				if deadStates[ti.State] && ti.Progress == 0 {
					continue
				}
			}
			recoverHashes = append(recoverHashes, rec.InfoHash)
		}
		if len(recoverHashes) > 0 {
			e.logger.Info("recovering orphan torrents: deleted records still present in downloader",
				zap.String("client_id", clientID),
				zap.Int("count", len(recoverHashes)))
			for i := 0; i < len(recoverHashes); i += sqliteVarLimit {
				end := i + sqliteVarLimit
				if end > len(recoverHashes) {
					end = len(recoverHashes)
				}
				e.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).
					Where("client_id = ? AND info_hash IN ?", clientID, recoverHashes[i:end]).
					Updates(map[string]interface{}{
						"status":         model.SeedingStatusSeeding,
						"last_action_by": "",
						"updated_at":     time.Now(),
					})
			}

			e.mu.Lock()
			for _, hash := range recoverHashes {
				key := recordKey(clientID, hash)
				if _, exists := e.recordMap[key]; !exists {
					var rec model.SeedingTorrentRecord
					if err := e.db.WithContext(ctx).
						Where("client_id = ? AND info_hash = ?", clientID, hash).
						First(&rec).Error; err == nil {
						e.recordMap[key] = &rec
					}
				}
			}
			e.mu.Unlock()
		}
	}
}

type Engine struct {
	db                *gorm.DB
	logger            *zap.Logger
	clientProvider    model.DownloaderProvider
	clientProviderMu  sync.RWMutex
	siteProvider      model.SiteInfoProvider
	freeEndMonitor    *FreeEndMonitor
	wsBroadcaster     event.WSBroadcaster
	mu                sync.RWMutex
	recordMap         map[string]*model.SeedingTorrentRecord
	emaStates         map[string]*emaState
	maindataMu        sync.RWMutex
	maindataCache     map[string]*maindataEntry
	fitTimer          *FitTimer
	freeWaitMonitor   *FreeWaitMonitor
	refreshCancel     context.CancelFunc
	wg                sync.WaitGroup
	reseedTrigger     ReseedTrigger

	unregisteredCursor   atomic.Int64
	unregisteredChecking atomic.Bool
}

type ReseedTrigger interface {
	OnTorrentSeeding(ctx context.Context, record model.SeedingTorrentRecord, reseedClientIDs []string)
}

func (e *Engine) SetReseedTrigger(trigger ReseedTrigger) {
	e.reseedTrigger = trigger
}

type maindataEntry struct {
	Maindata       *model.Maindata
	FreeSpace      int64
	TotalDiskSpace int64
	UpdatedAt      time.Time
}

type emaState struct {
	UploadSpeed   float64
	DownloadSpeed float64
}

const emaAlpha = 0.3

func NewEngine(db *gorm.DB, logger *zap.Logger) *Engine {
	e := &Engine{
		db:              db,
		logger:          logger,
		recordMap:       make(map[string]*model.SeedingTorrentRecord),
		emaStates:       make(map[string]*emaState),
		maindataCache:   make(map[string]*maindataEntry),
		fitTimer:        NewFitTimer(),
		freeWaitMonitor: NewFreeWaitMonitor(db, logger),
	}
	e.freeEndMonitor = NewFreeEndMonitor(db, nil, logger)
	e.freeEndMonitor.SetEngine(e)
	e.freeWaitMonitor.SetEngine(e)
	return e
}

func (e *Engine) SetClientProvider(cp model.DownloaderProvider) {
	e.clientProviderMu.Lock()
	defer e.clientProviderMu.Unlock()
	e.clientProvider = cp
	if e.freeEndMonitor != nil {
		e.freeEndMonitor.client = cp
	}
}

func (e *Engine) getClientProvider() model.DownloaderProvider {
	e.clientProviderMu.RLock()
	defer e.clientProviderMu.RUnlock()
	return e.clientProvider
}

func (e *Engine) GetGlobalTransferStats(ctx context.Context) *model.GlobalTransferStats {
	result := &model.GlobalTransferStats{}
	if e.getClientProvider() == nil {
		return result
	}
	configs, err := e.ListConfigs(ctx)
	if err != nil {
		e.logger.Debug("list seeding configs failed for global stats", zap.Error(err))
		return result
	}
	for _, cfg := range configs {
		if !cfg.Enabled {
			continue
		}
		client, err := e.getClientProvider().Get(cfg.ClientID)
		if err != nil {
			continue
		}
		stats, err := client.GetGlobalTransferStats(ctx)
		if err != nil {
			e.logger.Debug("get global transfer stats failed",
				zap.String("client", client.GetName()),
				zap.Error(err))
			continue
		}
		result.AllTimeUpload += stats.AllTimeUpload
		result.AllTimeDownload += stats.AllTimeDownload
	}
	return result
}

func (e *Engine) GetTodayTransferDelta(ctx context.Context) *model.GlobalTransferStats {
	result := &model.GlobalTransferStats{}
	if e.getClientProvider() == nil {
		return result
	}
	configs, err := e.ListConfigs(ctx)
	if err != nil {
		return result
	}
	for _, cfg := range configs {
		if !cfg.Enabled {
			continue
		}
		client, err := e.getClientProvider().Get(cfg.ClientID)
		if err != nil {
			continue
		}
		currentStats, err := client.GetGlobalTransferStats(ctx)
		if err != nil {
			continue
		}
		var dbState model.SeedingClientState
		if err := e.db.WithContext(ctx).Where("client_id = ?", cfg.ClientID).First(&dbState).Error; err != nil {
			continue
		}
		result.AllTimeUpload += currentStats.AllTimeUpload - dbState.DayStartUpload
		result.AllTimeDownload += currentStats.AllTimeDownload - dbState.DayStartDownload
	}
	return result
}

func (e *Engine) SetSiteProvider(sp model.SiteInfoProvider) {
	e.siteProvider = sp
}

func (e *Engine) SetWSBroadcaster(b event.WSBroadcaster) {
	e.wsBroadcaster = b
}

func recordKey(clientID, infoHash string) string {
	return clientID + ":" + infoHash
}

func (e *Engine) Start(ctx context.Context) error {
	var records []model.SeedingTorrentRecord
	if err := e.db.WithContext(ctx).
		Where("status IN ?", []string{"pending", "seeding", "paused_free_end", "paused_rule", "deleting"}).
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

	var failedRecords []model.SeedingTorrentRecord
	if err := e.db.WithContext(ctx).
		Where("status = ?", "delete_failed").
		Find(&failedRecords).Error; err != nil {
		e.logger.Warn("query failed records for cleanup", zap.Error(err))
	}
	for i := range failedRecords {
		rec := &failedRecords[i]
		e.logger.Info("cleaning up delete_failed record",
			zap.Uint("id", rec.ID),
			zap.String("info_hash", rec.InfoHash))
		if err := e.db.WithContext(ctx).Model(rec).Update("status", model.SeedingStatusDeleted).Error; err != nil {
			e.logger.Warn("update record status to deleted failed",
				zap.Uint("id", rec.ID),
				zap.Error(err))
		}
	}

	if e.freeEndMonitor != nil {
		e.freeEndMonitor.RecoverOnStartup(ctx)
	}

	if e.freeWaitMonitor != nil {
		e.freeWaitMonitor.RecoverOnStartup(ctx)
	}

	var recoveryRules []model.DeleteRule
	if err := e.db.WithContext(ctx).Where("enabled = ?", true).Find(&recoveryRules).Error; err != nil {
		e.logger.Warn("failed to load recovery rules, continuing without", zap.Error(err))
	}
	ruleMap := make(map[uint]model.DeleteRule)
	for _, r := range recoveryRules {
		ruleMap[r.ID] = r
	}

	var configs []model.SeedingClientConfig
	if err := e.db.WithContext(ctx).Where("enabled = ?", true).Find(&configs).Error; err != nil {
		e.logger.Warn("failed to load seeding configs, continuing without", zap.Error(err))
	}
	clientRuleIDs := make(map[string]map[uint]bool)
	for _, cfg := range configs {
		ids := splitRuleIDs(cfg.DeleteRuleIDs)
		ruleSet := make(map[uint]bool, len(ids))
		for _, idStr := range ids {
			if id, err := strconv.ParseUint(idStr, 10, 32); err == nil {
				ruleSet[uint(id)] = true
			}
		}
		clientRuleIDs[cfg.ClientID] = ruleSet
	}

	for i := range records {
		if records[i].FirstMatchedAt == nil {
			continue
		}
		t := *records[i].FirstMatchedAt
		ruleSet := clientRuleIDs[records[i].ClientID]
		for ruleID := range ruleSet {
			if _, ok := ruleMap[ruleID]; ok {
				e.fitTimer.MarkMatched(ruleID, records[i].InfoHash, t)
			}
		}
	}

	refreshCtx, cancel := context.WithCancel(context.Background())
	e.refreshCancel = cancel
	e.wg.Add(1)
	go func() { defer e.wg.Done(); e.refreshMaindataLoop(refreshCtx) }()

	return nil
}

func (e *Engine) Stop(_ context.Context) error {
	if e.refreshCancel != nil {
		e.refreshCancel()
	}
	e.wg.Wait()
	if e.freeEndMonitor != nil {
		e.freeEndMonitor.StopAll()
	}
	e.logger.Info("seeding engine stopped", zap.Int("active_records", len(e.recordMap)))
	return nil
}

func (e *Engine) refreshMaindataLoop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	e.refreshMaindataOnce(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.refreshMaindataOnce(ctx)
		}
	}
}

func (e *Engine) refreshMaindataOnce(ctx context.Context) {
	if e.getClientProvider() == nil {
		return
	}

	clients := e.getClientProvider().ListClients()
	for _, clientID := range clients {
		if ctx.Err() != nil {
			return
		}

		dlClient, err := e.getClientProvider().Get(clientID)
		if err != nil {
			continue
		}

		md, mdErr := dlClient.GetMainData(ctx)
		if mdErr != nil || md == nil {
			continue
		}

		entry := &maindataEntry{
			Maindata:       md,
			FreeSpace:      md.FreeSpace,
			TotalDiskSpace: md.TotalDiskSpace,
			UpdatedAt:      time.Now(),
		}

		e.maindataMu.Lock()
		e.maindataCache[clientID] = entry
		e.maindataMu.Unlock()

		torrentMap := make(map[string]*model.TorrentInfo, len(md.Torrents))
		for hash, t := range md.Torrents {
			tCopy := t
			torrentMap[hash] = &tCopy
		}
		e.updateEMA(ctx, clientID, md, torrentMap)
		e.syncStaleRecords(ctx, clientID, torrentMap)
		e.checkUnregisteredTorrents(ctx, clientID, dlClient)
	}
}

func (e *Engine) getCachedMaindata(clientID string) *maindataEntry {
	e.maindataMu.RLock()
	defer e.maindataMu.RUnlock()
	return e.maindataCache[clientID]
}

func (e *Engine) FreeWaitCheckOnce(ctx context.Context) int {
	if e.freeWaitMonitor == nil || e.freeWaitMonitor.PendingCount() == 0 {
		return 0
	}
	if e.siteProvider == nil {
		return 0
	}

	checker := &siteDiscountChecker{provider: e.siteProvider}
	return e.freeWaitMonitor.CheckOnce(ctx, checker, func(ctx context.Context, entry *freeWaitEntry) error {
		record := &model.SeedingTorrentRecord{
			ClientID:       entry.ClientID,
			InfoHash:       entry.InfoHash,
			SiteName:       entry.SiteName,
			TorrentID:      entry.TorrentID,
			Status:         model.SeedingStatusPending,
			Source:         "free_wait",
			IsFree:         true,
			HasHR:          entry.HasHR,
			HRSeedTimeH:    entry.HRSeedTimeH,
			SubscriptionID: entry.SubscriptionID,
			TorrentSize:    entry.Size,
		}
		if err := e.AddSeedingRecord(ctx, record); err != nil {
			return err
		}

		if entry.ClientID != "" && entry.SubscriptionID != "" {
			e.pushFreeWaitTorrent(ctx, entry)
		}

		return nil
	})
}

func (e *Engine) pushFreeWaitTorrent(ctx context.Context, entry *freeWaitEntry) {
	if e.getClientProvider() == nil {
		return
	}
	dlClient, err := e.getClientProvider().Get(entry.ClientID)
	if err != nil {
		e.logger.Warn("free wait push: client not available",
			zap.String("client_id", entry.ClientID),
			zap.Error(err))
		return
	}

	exists, err := dlClient.CheckExists(ctx, entry.InfoHash)
	if err == nil && exists {
		e.logger.Debug("free wait push: already exists in client, ensuring status=seeding",
			zap.String("info_hash", entry.InfoHash))
		now := time.Now()
		e.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).
			Where("client_id = ? AND info_hash = ? AND status = ?", entry.ClientID, entry.InfoHash, model.SeedingStatusPending).
			Updates(map[string]interface{}{
				"status":     model.SeedingStatusSeeding,
				"flushed_at": now,
			})
		e.mu.Lock()
		key := recordKey(entry.ClientID, entry.InfoHash)
		if r, ok := e.recordMap[key]; ok && r.Status == model.SeedingStatusPending {
			r.Status = model.SeedingStatusSeeding
			r.FlushedAt = &now
		}
		e.mu.Unlock()
		return
	}

	torrentData, err := e.downloadTorrent(ctx, entry.SiteName, entry.TorrentID)
	if err != nil || len(torrentData) == 0 {
		e.logger.Warn("free wait push: download torrent failed",
			zap.String("site", entry.SiteName),
			zap.String("torrent_id", entry.TorrentID),
			zap.Error(err))
		return
	}

	var sub model.RSSSubscription
	subID, _ := strconv.ParseUint(entry.SubscriptionID, 10, 64)
	if err := e.db.WithContext(ctx).Where("id = ?", uint(subID)).First(&sub).Error; err != nil {
		e.logger.Warn("free wait push: subscription not found",
			zap.String("subscription_id", entry.SubscriptionID),
			zap.Error(err))
		return
	}

	opts := model.AddTorrentOptions{
		SavePath: sub.SavePath,
		Category: sub.Category,
		Tags:     sub.Tags,
		Paused:   sub.AddPaused,
		AutoTMM:  sub.AutoTMM,
	}
	if sub.UploadLimitKB > 0 {
		opts.UploadLimit = sub.UploadLimitKB * 1024
	}
	if sub.DownloadLimitKB > 0 {
		opts.DownloadLimit = sub.DownloadLimitKB * 1024
	}

	addResult, err := dlClient.AddFromFile(ctx, torrentData, opts)
	if err != nil {
		e.logger.Warn("free wait push: add from file failed",
			zap.String("torrent_id", entry.TorrentID),
			zap.Error(err))
		return
	}

	now := time.Now()

	e.logger.Info("free wait push: pushed to downloader",
		zap.String("client_id", entry.ClientID),
		zap.String("site", entry.SiteName),
		zap.String("torrent_id", entry.TorrentID),
		zap.String("info_hash", entry.InfoHash))

	if addResult != nil && addResult.InfoHash != "" && addResult.InfoHash != entry.InfoHash {
		e.mu.Lock()
		altKey := recordKey(entry.ClientID, addResult.InfoHash)
		if _, ok := e.recordMap[altKey]; !ok {
			oldKey := recordKey(entry.ClientID, entry.InfoHash)
			delete(e.recordMap, oldKey)
			e.mu.Unlock()

			e.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).
				Where("client_id = ? AND info_hash = ?", entry.ClientID, entry.InfoHash).
				Updates(map[string]interface{}{
					"status":     model.SeedingStatusDeleted,
					"updated_at": now,
				})

			newRecord := &model.SeedingTorrentRecord{
				ClientID:       entry.ClientID,
				SiteName:       entry.SiteName,
				TorrentID:      entry.TorrentID,
				InfoHash:       addResult.InfoHash,
				IsFree:         true,
				Source:         "free_wait",
				Status:         model.SeedingStatusSeeding,
				FlushedAt:      &now,
				HasHR:          entry.HasHR,
				HRSeedTimeH:    entry.HRSeedTimeH,
				SubscriptionID: entry.SubscriptionID,
				TorrentSize:    entry.Size,
			}
			if dbErr := e.db.WithContext(ctx).Create(newRecord).Error; dbErr != nil {
				e.logger.Warn("free wait push: create alt record failed",
					zap.String("torrent_id", entry.TorrentID),
					zap.Error(dbErr))
				if delErr := dlClient.DeleteTorrent(ctx, addResult.InfoHash, false); delErr != nil {
					e.logger.Error("free wait push: rollback delete failed",
						zap.String("info_hash", addResult.InfoHash),
						zap.Error(delErr))
				}
			} else {
				e.mu.Lock()
				e.recordMap[altKey] = newRecord
				e.mu.Unlock()
				if dbErr := e.db.WithContext(ctx).Model(&model.RSSTorrentSeen{}).
					Where("site_name = ? AND torrent_id = ?", entry.SiteName, entry.TorrentID).
					Update("info_hash", addResult.InfoHash).Error; dbErr != nil {
					e.logger.Warn("free wait push: backfill rss_torrent_seen info_hash failed",
						zap.String("torrent_id", entry.TorrentID),
						zap.Error(dbErr))
				}
				if e.freeEndMonitor != nil && newRecord.FreeEndAt != nil {
					e.freeEndMonitor.Schedule(newRecord)
				}
			}
		} else {
			e.mu.Unlock()
		}
	} else {
		e.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).
			Where("client_id = ? AND info_hash = ?", entry.ClientID, entry.InfoHash).
			Updates(map[string]interface{}{
				"status":     model.SeedingStatusSeeding,
				"flushed_at": now,
			})
		e.mu.Lock()
		key := recordKey(entry.ClientID, entry.InfoHash)
		if r, ok := e.recordMap[key]; ok {
			r.Status = model.SeedingStatusSeeding
			r.FlushedAt = &now
		}
		e.mu.Unlock()
	}
}

type siteDiscountChecker struct {
	provider model.SiteInfoProvider
}

func (c *siteDiscountChecker) CheckDiscount(ctx context.Context, siteName, torrentID string) (model.DiscountLevel, error) {
	adapter, err := c.provider.GetAdapter(ctx, siteName)
	if err != nil {
		return model.DiscountNone, err
	}
	cfg, cfgErr := c.provider.GetSiteConfig(ctx, siteName)
	if cfgErr != nil {
		return model.DiscountNone, cfgErr
	}
	result, err := adapter.DetectDiscount(ctx, cfg, torrentID)
	if err != nil || result == nil {
		return model.DiscountNone, err
	}
	return result.Level, nil
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

	if err := e.db.WithContext(ctx).Create(record).Error; err != nil {
		e.mu.Unlock()
		return err
	}

	e.recordMap[key] = record
	e.mu.Unlock()

	if e.freeEndMonitor != nil {
		e.freeEndMonitor.Schedule(record)
	}
	return nil
}

func (e *Engine) RemoveSeedingRecord(ctx context.Context, clientID, infoHash string) error {
	if err := e.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).
		Where("client_id = ? AND info_hash = ?", clientID, infoHash).
		Update("status", model.SeedingStatusDeleted).Error; err != nil {
		return err
	}

	e.mu.Lock()
	delete(e.recordMap, recordKey(clientID, infoHash))
	e.mu.Unlock()

	if e.freeEndMonitor != nil {
		e.freeEndMonitor.Cancel(clientID, infoHash)
	}

	return nil
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
			[]string{string(model.SeedingStatusPending), string(model.SeedingStatusSeeding), "paused_free_end", "paused_rule", "delete_failed"}).
		Find(&records).Error
	return records, err
}

func (e *Engine) saveFinalTraffic(ctx context.Context, rec *model.SeedingTorrentRecord, ti *model.TorrentInfo) {
	if ti == nil || rec == nil {
		return
	}

	traffic := &model.TorrentTraffic{
		ClientID:      rec.ClientID,
		InfoHash:      rec.InfoHash,
		SiteName:      rec.SiteName,
		Uploaded:      ti.Uploaded,
		Downloaded:    ti.Downloaded,
		UploadSpeed:   ti.UploadSpeed,
		DownloadSpeed: ti.DownloadSpeed,
		Ratio:         ti.Ratio,
		RecordedAt:    time.Now(),
	}
	if err := e.db.WithContext(ctx).Create(traffic).Error; err != nil {
		e.logger.Warn("save final torrent_traffic failed",
			zap.String("infoHash", rec.InfoHash),
			zap.Error(err))
	}

	if err := e.db.WithContext(ctx).Model(rec).Updates(map[string]interface{}{
		"final_uploaded":   ti.Uploaded,
		"final_downloaded": ti.Downloaded,
	}).Error; err != nil {
		e.logger.Warn("save final_uploaded to record failed",
			zap.Uint("id", rec.ID),
			zap.Error(err))
	}
}

func (e *Engine) UpdateStatus(ctx context.Context, id uint, status model.SeedingTorrentStatus, actionBy string) error {
	if err := e.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":         status,
			"last_action_by": actionBy,
			"updated_at":     time.Now(),
		}).Error; err != nil {
		return err
	}

	e.mu.Lock()
	for _, r := range e.recordMap {
		if r.ID == id {
			r.Status = status
			r.LastActionBy = actionBy
			break
		}
	}
	e.mu.Unlock()
	return nil
}

func (e *Engine) PauseForFreeEnd(ctx context.Context, clientID, infoHash string) error {
	key := recordKey(clientID, infoHash)
	e.mu.Lock()
	if r, ok := e.recordMap[key]; ok {
		r.Status = model.SeedingStatusPausedFreeEnd
		r.IsFree = false
	}
	e.mu.Unlock()

	return e.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).
		Where("client_id = ? AND info_hash = ?", clientID, infoHash).
		Updates(map[string]interface{}{
			"status":         model.SeedingStatusPausedFreeEnd,
			"is_free":        false,
			"last_action_by": "free_end_pauser",
		}).Error
}

func (e *Engine) MarkFreeExpired(ctx context.Context, clientID, infoHash string) error {
	return e.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).
		Where("client_id = ? AND info_hash = ?", clientID, infoHash).
		Updates(map[string]interface{}{
			"is_free":        false,
			"last_action_by": "free_end_keeper",
		}).Error
}

type ManagedCounts struct {
	Active int `json:"active"`
	Paused int `json:"paused"`
}

func (e *Engine) GetManagedCounts() *ManagedCounts {
	e.mu.RLock()
	defer e.mu.RUnlock()

	counts := &ManagedCounts{}
	for _, r := range e.recordMap {
		switch r.Status {
		case model.SeedingStatusSeeding:
			counts.Active++
		case model.SeedingStatusPausedFreeEnd, model.SeedingStatusPausedRule:
			counts.Paused++
		}
	}
	return counts
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
	subIDs := make([]string, 0)
	for i := range events {
		if events[i].SourceID != "" {
			subIDs = append(subIDs, events[i].SourceID)
		}
	}

	subMap := make(map[string]*model.RSSSubscription, len(subIDs))
	if len(subIDs) > 0 {
		var subs []model.RSSSubscription
		if err := e.db.WithContext(ctx).Where("id IN ?", subIDs).Find(&subs).Error; err != nil {
			e.logger.Warn("query subscriptions for scoring", zap.Error(err))
		}
		for i := range subs {
			subMap[fmt.Sprintf("%d", subs[i].ID)] = &subs[i]
		}
	}

	for i := range events {
		ev := &events[i]
		if ev.SourceID == "" {
			continue
		}

		clientID := dispatcher.GetClientName(ev)

		isQualified := ev.Discount == model.DiscountFree || ev.Discount == model.Discount2xFree

		if !isQualified {
			sub, ok := subMap[ev.SourceID]
			if ok {
				if sub.ScoringConfig.Include2xUp && (ev.Discount == model.Discount2xUp || ev.Discount == model.Discount2x50) {
					isQualified = true
				}

				if !isQualified && sub.FreeWaitEnabled {
					var checkBefore *time.Time
					if sub.FreeWaitMaxWaitSec > 0 {
						deadline := time.Now().Add(time.Duration(sub.FreeWaitMaxWaitSec) * time.Second)
						checkBefore = &deadline
					}
					e.freeWaitMonitor.Add(ev.SiteName, ev.TorrentID, ev.InfoHash, ev.Title, ev.Size, checkBefore, clientID, ev.SourceID, ev.HasHR, ev.HRSeedTimeH)
					continue
				}
			}
		}

		if !isQualified {
			continue
		}

		record := &model.SeedingTorrentRecord{
			ClientID:       clientID,
			InfoHash:       ev.InfoHash,
			SiteName:       ev.SiteName,
			TorrentID:      ev.TorrentID,
			Status:         model.SeedingStatusPending,
			Source:         "rss",
			SubscriptionID: ev.SourceID,
			Discount:       ev.Discount,
			IsFree:         ev.Discount == model.DiscountFree || ev.Discount == model.Discount2xFree,
			FreeEndAt:      ev.FreeEndAt,
			TorrentSize:    ev.Size,
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
	staleStatuses := []string{
		string(model.SeedingStatusPausedFreeEnd),
		string(model.SeedingStatusPausedRule),
		string(model.SeedingStatusDeleted),
		string(model.SeedingStatusUnregistered),
	}

	type staleKey struct {
		ClientID string
		InfoHash string
	}
	var stalePairs []staleKey
	if dbErr := e.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).
		Where("status IN ? AND updated_at < ?", staleStatuses, cutoff).
		Select("client_id, info_hash").
		Find(&stalePairs).Error; dbErr != nil {
		return 0, dbErr
	}

	result := e.db.WithContext(ctx).
		Where("status IN ? AND updated_at < ?", staleStatuses, cutoff).
		Delete(&model.SeedingTorrentRecord{})
	if result.Error != nil {
		return 0, &model.AppError{Code: 50001, Message: "cleanup stale seeding records failed", Cause: result.Error}
	}

	e.mu.Lock()
	for _, p := range stalePairs {
		delete(e.recordMap, recordKey(p.ClientID, p.InfoHash))
	}
	e.mu.Unlock()

	freeExpired := e.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).
		Where("status = ? AND is_free = ? AND free_end_at IS NOT NULL AND free_end_at < ?", "seeding", true, time.Now()).
		Updates(map[string]interface{}{
			"is_free":    false,
			"updated_at": time.Now(),
		})
	if freeExpired.Error != nil {
		return result.RowsAffected, &model.AppError{Code: 50001, Message: "mark free-expired records failed", Cause: freeExpired.Error}
	}

	e.mu.Lock()
	for _, r := range e.recordMap {
		if r.Status == model.SeedingStatusSeeding && r.IsFree && r.FreeEndAt != nil && r.FreeEndAt.Before(time.Now()) {
			r.IsFree = false
		}
	}
	activeClients := make(map[string]bool, len(e.recordMap))
	for _, r := range e.recordMap {
		activeClients[r.ClientID] = true
	}
	for cid := range e.emaStates {
		if !activeClients[cid] {
			delete(e.emaStates, cid)
		}
	}
	e.mu.Unlock()

	e.maindataMu.Lock()
	for cid := range e.maindataCache {
		if !activeClients[cid] {
			delete(e.maindataCache, cid)
		}
	}
	e.maindataMu.Unlock()

	scoringCutoff := time.Now().Add(-scoringCutoffHours)
	if dbErr := e.db.WithContext(ctx).Where("created_at < ?", scoringCutoff).Delete(&model.ScoringLog{}).Error; dbErr != nil {
		e.logger.Warn("cleanup scoring logs failed", zap.Error(dbErr))
	}

	e.logger.Info("seeding cleanup completed",
		zap.Int64("deleted_stale", result.RowsAffected),
		zap.Int64("paused_free_expired", freeExpired.RowsAffected),
		zap.Int("purged_from_memory", len(stalePairs)),
	)

	return result.RowsAffected + freeExpired.RowsAffected, nil
}

type EvaluateResult struct {
	Evaluated int
	Deleted   int
	Paused    int
	Limited   int
	Errors    int
}

type evaluateContext struct {
	clientID     string
	client       model.DownloaderClient
	records      []model.SeedingTorrentRecord
	torrents     []*model.TorrentInfo
	torrentMap   map[string]*model.TorrentInfo
	maindata     *model.Maindata
	freeSpace    int64
	totalSpace   int64
	weights      CleanupWeights
	minScore     float64
	minAge       float64
	cfg          *model.SeedingClientConfig
	cascadeRules []model.DeleteRule
	deleteRules  []model.DeleteRule
}

func (e *Engine) prepareEvaluateContext(ctx context.Context, clientID string, cfg *model.SeedingClientConfig) (*evaluateContext, error) {
	if e.getClientProvider() == nil {
		return nil, &model.AppError{Code: 50001, Message: "client provider not configured"}
	}

	dlClient, err := e.getClientProvider().Get(clientID)
	if err != nil {
		return nil, &model.AppError{Code: 50001, Message: "获取下载器失败", Cause: err}
	}

	records, err := e.ListByClient(ctx, clientID)
	if err != nil {
		return nil, &model.AppError{Code: 50001, Message: "查询做种记录失败", Cause: err}
	}

	weights := DefaultCleanupWeights()
	minScore := 0.3
	minAgeHours := 48.0

	if cfg != nil {
		minScore, minAgeHours, weights = e.applyConfig(cfg, minScore, minAgeHours, weights)
	}

	torrents, err := dlClient.GetAllTorrents(ctx)
	if err != nil {
		return nil, &model.AppError{Code: 50001, Message: "获取种子列表失败", Cause: err}
	}

	torrentMap := make(map[string]*model.TorrentInfo, len(torrents))
	for _, t := range torrents {
		torrentMap[t.Hash] = t
	}

	var maindata *model.Maindata
	freeSpace := int64(-1)
	totalSpace := int64(0)
	if cached := e.getCachedMaindata(clientID); cached != nil {
		maindata = cached.Maindata
		freeSpace = cached.FreeSpace
		totalSpace = cached.TotalDiskSpace
	} else {
		md, mdErr := dlClient.GetMainData(ctx)
		if mdErr == nil && md != nil {
			maindata = md
			freeSpace = md.FreeSpace
			totalSpace = md.TotalDiskSpace
		}
		e.updateEMA(ctx, clientID, md, torrentMap)
	}

	var cascadeRules []model.DeleteRule
	var deleteRules []model.DeleteRule

	// Check global switch: when enabled, all DeleteRules apply to all seeding clients
	var globalDeleteRules string
	e.db.Raw("SELECT value FROM system_settings WHERE key = 'seeding_delete_rules_global' LIMIT 1").Scan(&globalDeleteRules)

	if globalDeleteRules == "true" {
		if dbErr := e.db.WithContext(ctx).
			Where("enabled = ? AND cascade_delete = ?", true, true).
			Find(&cascadeRules).Error; dbErr != nil {
			e.logger.Warn("load cascade delete rules (global) failed", zap.String("client_id", clientID), zap.Error(dbErr))
		}
		if dbErr := e.db.WithContext(ctx).
			Where("enabled = ?", true).
			Order("priority DESC").
			Find(&deleteRules).Error; dbErr != nil {
			e.logger.Warn("load delete rules (global) failed", zap.String("client_id", clientID), zap.Error(dbErr))
		}
	} else if cfg != nil && cfg.DeleteRuleIDs != "" {
		ruleIDs := splitRuleIDs(cfg.DeleteRuleIDs)
		if dbErr := e.db.WithContext(ctx).
			Where("id IN (?) AND enabled = ? AND cascade_delete = ?", ruleIDs, true, true).
			Find(&cascadeRules).Error; dbErr != nil {
			e.logger.Warn("load cascade delete rules failed", zap.String("client_id", clientID), zap.Error(dbErr))
		}
		if dbErr := e.db.WithContext(ctx).
			Where("id IN (?) AND enabled = ?", ruleIDs, true).
			Order("priority DESC").
			Find(&deleteRules).Error; dbErr != nil {
			e.logger.Warn("load delete rules failed", zap.String("client_id", clientID), zap.Error(dbErr))
		}
	}

	return &evaluateContext{
		clientID:     clientID,
		client:       dlClient,
		records:      records,
		torrents:     torrents,
		torrentMap:   torrentMap,
		maindata:     maindata,
		freeSpace:    freeSpace,
		totalSpace:   totalSpace,
		weights:      weights,
		minScore:     minScore,
		minAge:       minAgeHours,
		cfg:          cfg,
		cascadeRules: cascadeRules,
		deleteRules:  deleteRules,
	}, nil
}

func (e *Engine) evaluateRecord(ctx context.Context, rec *model.SeedingTorrentRecord, ec *evaluateContext, cycleID string) (candidate *CleanupCandidate, evaluated bool, shouldCleanup bool) {
	if rec.Status != model.SeedingStatusSeeding && rec.Status != model.SeedingStatusDeleteFailed && rec.Status != model.SeedingStatusPausedRule && rec.Status != model.SeedingStatusPausedFreeEnd {
		return nil, false, false
	}

	if rec.Status == model.SeedingStatusPausedRule && rec.LastActionBy == "disk_protect" {
		ti, ok := ec.torrentMap[rec.InfoHash]
		if ok && ti != nil && ti.State != "" && ti.State != "pausedUP" && ti.State != "pausedDL" {
			resumedBy := "resumed_in_downloader"
			if ti.Progress < 1.0 {
				resumedBy = "resumed_downloading"
			}
			if err := e.UpdateStatus(ctx, rec.ID, model.SeedingStatusSeeding, resumedBy); err != nil {
				e.logger.Warn("sync paused_rule→seeding failed", zap.Uint("id", rec.ID), zap.Error(err))
			} else {
				rec.Status = model.SeedingStatusSeeding
				e.logger.Info("检测到下载器中种子已手动恢复，同步状态为 seeding",
					zap.String("info_hash", rec.InfoHash),
					zap.String("state", ti.State))
			}
		}
	}

	if rec.Status == model.SeedingStatusDeleteFailed {
		ti, ok := ec.torrentMap[rec.InfoHash]
		if ok && ti != nil {
			return &CleanupCandidate{ID: rec.ID, InfoHash: rec.InfoHash}, true, true
		}
		if err := e.db.WithContext(ctx).Model(rec).Update("status", model.SeedingStatusDeleted).Error; err != nil {
			e.logger.Warn("update delete-failed record status", zap.Uint("id", rec.ID), zap.Error(err))
		}
		return nil, true, false
	}

	evaluated = true

	ti, ok := ec.torrentMap[rec.InfoHash]
	if !ok || ti == nil {
		if rec.FlushedAt != nil && time.Since(*rec.FlushedAt) < 30*time.Minute {
			e.logger.Debug("seeding record: torrent not in downloader but within flush grace period",
				zap.String("info_hash", rec.InfoHash),
				zap.String("client_id", rec.ClientID),
				zap.Time("flushed_at", *rec.FlushedAt))
			return nil, evaluated, false
		}
		if err := e.db.WithContext(ctx).Model(rec).Update("status", model.SeedingStatusDeleted).Error; err != nil {
			e.logger.Warn("update record status to deleted", zap.Uint("id", rec.ID), zap.Error(err))
		}
		e.mu.Lock()
		delete(e.recordMap, recordKey(rec.ClientID, rec.InfoHash))
		e.mu.Unlock()
		e.logger.Debug("seeding record cleaned: torrent not in downloader",
			zap.String("info_hash", rec.InfoHash),
			zap.String("client_id", rec.ClientID))
		return nil, evaluated, false
	}

	if rec.Status == model.SeedingStatusSeeding && ti.Progress < 1.0 {
		return nil, evaluated, false
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

	candidate = &CleanupCandidate{
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
		UploadSpeed:   ti.UploadSpeed,
	}

	score := CalculateCleanupScore(*candidate, ec.weights)
	candidate.Score = score

	if score < 5.0 {
		if dbErr := e.db.WithContext(ctx).Create(&model.ScoringLog{
			CycleID:     cycleID,
			ClientID:    ec.clientID,
			InfoHash:    rec.InfoHash,
			SiteName:    rec.SiteName,
			TorrentID:   rec.TorrentID,
			Score:       score,
			AgeHours:    ageHours,
			Discount:    string(rec.Discount),
			IsFree:      rec.IsFree,
			HasHR:       rec.HasHR,
			UploadSpeed: ti.UploadSpeed,
		}).Error; dbErr != nil {
			e.logger.Warn("create scoring log failed", zap.String("info_hash", rec.InfoHash), zap.Error(dbErr))
		}
	}

	shouldCleanup = ShouldCleanup(*candidate, ec.minScore, ec.minAge)
	return candidate, evaluated, shouldCleanup
}

func (e *Engine) recoverDiskProtectPaused(ctx context.Context, clientID string) {
	var paused []model.SeedingTorrentRecord
	if err := e.db.WithContext(ctx).
		Where("client_id = ? AND status = ? AND last_action_by = ?", clientID, model.SeedingStatusPausedRule, "disk_protect").
		Find(&paused).Error; err != nil || len(paused) == 0 {
		return
	}

	for i := range paused {
		rec := &paused[i]

		if rec.IsFree && rec.FreeEndAt != nil && rec.FreeEndAt.Before(time.Now()) {
			stillFree := e.recheckDiscountForRecover(ctx, rec)
			if !stillFree {
				e.logger.Info("disk_protect 恢复: 免费期已过且不再免费，删除记录",
					zap.String("info_hash", rec.InfoHash),
					zap.String("site", rec.SiteName),
					zap.Time("free_end_at", *rec.FreeEndAt))
				if err := e.db.WithContext(ctx).Model(rec).Updates(map[string]interface{}{
					"status":         model.SeedingStatusDeleted,
					"last_action_by": "free_expired_while_paused",
				}).Error; err != nil {
					e.logger.Warn("disk_protect 恢复: 删除过期记录失败",
						zap.Uint("id", rec.ID), zap.Error(err))
				}
				e.mu.Lock()
				delete(e.recordMap, recordKey(clientID, rec.InfoHash))
				e.mu.Unlock()
				continue
			}
			e.logger.Info("disk_protect 恢复: 免费期已过但实时检测仍免费，继续恢复",
				zap.String("info_hash", rec.InfoHash),
				zap.String("site", rec.SiteName))
		}

		rec.FlushedAt = nil
		if err := e.db.WithContext(ctx).Model(rec).Updates(map[string]interface{}{
			"status":         model.SeedingStatusPending,
			"last_action_by": "disk_recover",
			"flushed_at":     nil,
		}).Error; err != nil {
			e.logger.Warn("recover disk_protect paused record failed",
				zap.Uint("id", rec.ID), zap.Error(err))
			continue
		}
		e.mu.Lock()
		key := recordKey(clientID, rec.InfoHash)
		if r, ok := e.recordMap[key]; ok {
			r.Status = model.SeedingStatusPending
			r.LastActionBy = "disk_recover"
			r.FlushedAt = nil
		}
		e.mu.Unlock()
		e.logger.Info("disk_protect 恢复：种子重新进入 pending 队列",
			zap.String("info_hash", rec.InfoHash),
			zap.String("client_id", clientID))
	}
}

func (e *Engine) recheckDiscountForRecover(ctx context.Context, rec *model.SeedingTorrentRecord) bool {
	if e.siteProvider == nil || rec.SiteName == "" {
		return false
	}
	adapter, err := e.siteProvider.GetAdapter(ctx, rec.SiteName)
	if err != nil {
		return false
	}
	siteCfg, err := e.siteProvider.GetSiteConfig(ctx, rec.SiteName)
	if err != nil || siteCfg == nil {
		return false
	}
	recheckCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	result, err := adapter.DetectDiscount(recheckCtx, siteCfg, rec.TorrentID)
	if err != nil {
		return false
	}
	return result != nil && result.Level.IsFree()
}

func (e *Engine) executeCleanup(ctx context.Context, rec *model.SeedingTorrentRecord, ti *model.TorrentInfo, ec *evaluateContext, result *EvaluateResult) {
	isDeleteFiles := true

	if HasSameFileTorrent(ti, ec.torrents) {
		isDeleteFiles = false
		e.logger.Debug("辅种文件保护：共享文件，仅删种子不删文件",
			zap.String("infoHash", rec.InfoHash),
			zap.String("name", ti.Name),
		)
	}

	e.reannounceBeforeDelete(ctx, ec.client, rec.InfoHash, ec.cfg)

	cascaded := false
	for _, rule := range ec.cascadeRules {
		relatedHashes := FindRelatedByTagOrPath(ti, ec.torrents, rule.CascadeMaxDepth)
		if len(relatedHashes) > 0 {
			allHashes := append([]string{rec.InfoHash}, relatedHashes...)
			if err := ec.client.BatchDeleteTorrents(ctx, allHashes, isDeleteFiles); err != nil {
				e.logger.Warn("级联删种失败",
					zap.Strings("hashes", allHashes),
					zap.Error(err),
				)
				result.Errors++
				if usErr := e.UpdateStatus(ctx, rec.ID, model.SeedingStatusDeleteFailed, "auto_cleanup"); usErr != nil {
					e.logger.Error("更新级联删种失败状态出错", zap.Uint("id", rec.ID), zap.Error(usErr))
				}
			} else {
				e.saveFinalTraffic(ctx, rec, ti)
				e.logger.Info("级联删种成功",
					zap.String("source", rec.InfoHash),
					zap.Int("related", len(relatedHashes)),
				)
				if usErr := e.UpdateStatus(ctx, rec.ID, model.SeedingStatusDeleting, "auto_cleanup"); usErr != nil {
					e.logger.Error("更新级联删种状态出错", zap.Uint("id", rec.ID), zap.Error(usErr))
				}
				e.markRelatedDeleted(ctx, relatedHashes, ec.clientID, "cascade_cleanup")
				result.Deleted++
			}
			cascaded = true
			break
		}
	}

	if !cascaded {
		e.saveFinalTraffic(ctx, rec, ti)
		if err := ec.client.DeleteTorrent(ctx, rec.InfoHash, isDeleteFiles); err != nil {
			e.logger.Warn("删种失败",
				zap.String("infoHash", rec.InfoHash),
				zap.Bool("deleteFiles", isDeleteFiles),
				zap.Error(err),
			)
			result.Errors++
			if usErr := e.UpdateStatus(ctx, rec.ID, model.SeedingStatusDeleteFailed, "auto_cleanup"); usErr != nil {
				e.logger.Error("更新删种失败状态出错", zap.Uint("id", rec.ID), zap.Error(usErr))
			}
			return
		}

		if err := e.UpdateStatus(ctx, rec.ID, model.SeedingStatusDeleting, "auto_cleanup"); err != nil {
			e.logger.Error("更新删种状态失败", zap.Uint("id", rec.ID), zap.Error(err))
		}
		result.Deleted++
	}
}

func (e *Engine) markRelatedDeleted(ctx context.Context, relatedHashes []string, clientID, actionBy string) {
	if len(relatedHashes) == 0 {
		return
	}
	e.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).
		Where("client_id = ? AND info_hash IN ? AND status = ?", clientID, relatedHashes, model.SeedingStatusSeeding).
		Updates(map[string]interface{}{
			"status":         model.SeedingStatusDeleting,
			"last_action_by": actionBy,
			"updated_at":     time.Now(),
		})

	e.mu.Lock()
	for _, hash := range relatedHashes {
		key := recordKey(clientID, hash)
		if r, ok := e.recordMap[key]; ok {
			r.Status = model.SeedingStatusDeleting
		}
	}
	e.mu.Unlock()
}

func (e *Engine) reannounceBeforeDelete(ctx context.Context, client model.DownloaderClient, infoHash string, cfg *model.SeedingClientConfig) bool {
	if cfg == nil || !cfg.ReannounceBefore {
		return false
	}

	retries := cfg.ReannounceRetries
	if retries <= 0 {
		retries = 2
	}
	interval := time.Duration(cfg.ReannounceIntervalMs) * time.Millisecond
	if interval <= 0 {
		interval = 3 * time.Second
	}

	for i := 0; i < retries; i++ {
		if ctx.Err() != nil {
			return false
		}

		if err := client.Reannounce(ctx, infoHash); err != nil {
			e.logger.Debug("reannounce failed",
				zap.String("infoHash", infoHash),
				zap.Int("attempt", i+1),
				zap.Error(err))
			continue
		}

		select {
		case <-ctx.Done():
			return false
		case <-time.After(interval):
		}

		ti, err := client.GetTorrentByHash(ctx, infoHash)
		if err != nil || ti == nil {
			continue
		}

		if ti.NumIncomplete > 0 || ti.UploadSpeed > 0 {
			e.logger.Info("reannounce before delete: peers still active, stats reported",
				zap.String("infoHash", infoHash),
				zap.Int("leechers", ti.NumIncomplete),
				zap.Int64("uploadSpeed", ti.UploadSpeed))
			return true
		}
	}

	return false
}

func (e *Engine) Evaluate(ctx context.Context, clientID string, cfg *model.SeedingClientConfig) (*EvaluateResult, error) {
	return e.evaluate(ctx, clientID, cfg, false)
}

func (e *Engine) DryRunEvaluate(ctx context.Context, clientID string, cfg *model.SeedingClientConfig) (int, error) {
	result, err := e.evaluate(ctx, clientID, cfg, true)
	if err != nil {
		return 0, err
	}
	return result.Evaluated, nil
}

func (e *Engine) evaluate(ctx context.Context, clientID string, cfg *model.SeedingClientConfig, dryRun bool) (*EvaluateResult, error) {
	ec, err := e.prepareEvaluateContext(ctx, clientID, cfg)
	if err != nil {
		return nil, err
	}

	if len(ec.records) == 0 {
		return &EvaluateResult{}, nil
	}

	result := &EvaluateResult{}
	cycleID := time.Now().Format("20060102-150405")

	if !dryRun && ec.cfg != nil && ec.cfg.DiskProtectEnabled && ec.cfg.MinDiskSpaceGB > 0 && ec.freeSpace >= 0 {
		minBytes := int64(ec.cfg.MinDiskSpaceGB * 1024 * 1024 * 1024)
		if ec.freeSpace >= minBytes {
			e.recoverDiskProtectPaused(ctx, ec.clientID)
		} else {
			e.logger.Warn("disk_protect: 磁盘空间不足，flush 将暂停推送新种子",
				zap.Int64("freeSpace", ec.freeSpace),
				zap.Float64("minGB", ec.cfg.MinDiskSpaceGB))
		}
	}

	if !dryRun {
		e.refreshDiscountStatus(ctx, ec.records)
	}

	handledHashes := e.evaluateRulesPhase(ctx, ec, dryRun, result)

	for i := range ec.records {
		rec := &ec.records[i]

		if handledHashes[rec.InfoHash] {
			continue
		}

		if rec.Status == model.SeedingStatusPending {
			pendingMaxAge := 24 * time.Hour
			if rec.CreatedAt.Add(pendingMaxAge).Before(time.Now()) {
				if dryRun {
					result.Deleted++
					continue
				}
				e.logger.Info("pending 超时清理",
					zap.Uint("id", rec.ID),
					zap.String("info_hash", rec.InfoHash),
					zap.String("client_id", rec.ClientID),
					zap.Time("created_at", rec.CreatedAt),
				)
				ti := ec.torrentMap[rec.InfoHash]
				if ti != nil {
					if err := ec.client.DeleteTorrent(ctx, rec.InfoHash, true); err != nil {
						e.logger.Warn("pending 超时清理：删除下载器种子失败", zap.Error(err))
					}
				}
				if err := e.db.WithContext(ctx).Model(rec).Updates(map[string]interface{}{
					"status":         model.SeedingStatusDeleted,
					"last_action_by": "pending_timeout",
				}).Error; err != nil {
					e.logger.Warn("pending 超时清理：更新状态失败", zap.Uint("id", rec.ID), zap.Error(err))
				}
				e.mu.Lock()
				delete(e.recordMap, recordKey(rec.ClientID, rec.InfoHash))
				e.mu.Unlock()
				result.Deleted++
			}
			continue
		}

		candidate, evaluated, shouldCleanup := e.evaluateRecord(ctx, rec, ec, cycleID)
		if !evaluated {
			continue
		}

		result.Evaluated++

		if candidate == nil {
			continue
		}

		if !shouldCleanup {
			continue
		}

		if dryRun {
			result.Deleted++
			continue
		}

		ti := ec.torrentMap[rec.InfoHash]
		e.executeCleanup(ctx, rec, ti, ec, result)
	}

	return result, nil
}

func (e *Engine) evaluateRulesPhase(ctx context.Context, ec *evaluateContext, dryRun bool, result *EvaluateResult) map[string]bool {
	handled := make(map[string]bool)
	if len(ec.deleteRules) == 0 {
		return handled
	}

	now := time.Now()
	minSeedDur := 1 * time.Hour
	if ec.cfg != nil && ec.cfg.MinSeedHoursBeforeDelete > 0 {
		minSeedDur = time.Duration(ec.cfg.MinSeedHoursBeforeDelete * float64(time.Hour))
	}

	var seedingRecords []model.SeedingTorrentRecord
	for _, rec := range ec.records {
		ti := ec.torrentMap[rec.InfoHash]
		if ti == nil {
			continue
		}
		if rec.Status == model.SeedingStatusSeeding {
			if rec.FlushedAt != nil && now.Sub(*rec.FlushedAt) < minSeedDur {
				continue
			}
			seedingRecords = append(seedingRecords, rec)
		} else if rec.Status == model.SeedingStatusPausedRule && rec.LastActionBy == "disk_protect" {
			seedingRecords = append(seedingRecords, rec)
		} else if rec.Status == model.SeedingStatusPausedFreeEnd {
			seedingRecords = append(seedingRecords, rec)
		} else if rec.Status == model.SeedingStatusPausedRule {
			seedingRecords = append(seedingRecords, rec)
		}
	}
	if len(seedingRecords) == 0 {
		return handled
	}

	evaluator := NewRuleEvaluator(e.db, e.logger)
	matches := evaluator.MatchRules(ctx, ec.deleteRules, seedingRecords, ec.torrentMap, ec.freeSpace, ec.totalSpace)

	for _, match := range matches {
		rule := match.Rule

		activeHashes := make(map[string]bool)
		var matchedIDs []uint
		for _, rec := range match.Records {
			activeHashes[rec.InfoHash] = true
			if e.fitTimer.MarkMatchedAndReturn(rule.ID, rec.InfoHash, now) {
				matchedIDs = append(matchedIDs, rec.ID)
			}
		}
		if len(matchedIDs) > 0 {
			e.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).
				Where("id IN ? AND first_matched_at IS NULL", matchedIDs).
				Update("first_matched_at", now)
		}

		unmatchedHashes := e.fitTimer.ClearUnmatchedAndGet(rule.ID, activeHashes)
		if len(unmatchedHashes) > 0 {
			unmatchedIDs := make([]uint, 0)
			for _, hash := range unmatchedHashes {
				for _, rec := range seedingRecords {
					if rec.InfoHash == hash {
						unmatchedIDs = append(unmatchedIDs, rec.ID)
						break
					}
				}
			}
			if len(unmatchedIDs) > 0 {
				e.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).
					Where("id IN ?", unmatchedIDs).
					Update("first_matched_at", nil)
			}
		}

		for i := range match.Records {
			rec := &match.Records[i]
			if handled[rec.InfoHash] {
				continue
			}
			if !e.fitTimer.IsFit(rule.ID, rec.InfoHash, rule.FitTime, now) {
				continue
			}

			result.Evaluated++

			if dryRun {
				switch rule.Action {
				case "delete":
					result.Deleted++
				case "pause":
					result.Paused++
				case "limit_speed":
					result.Limited++
				}
				handled[rec.InfoHash] = true
				continue
			}

			ti := ec.torrentMap[rec.InfoHash]
			e.executeRuleAction(ctx, rec, ti, ec, &rule, result)
			handled[rec.InfoHash] = true
		}
	}

	return handled
}

func (e *Engine) executeRuleAction(ctx context.Context, rec *model.SeedingTorrentRecord, ti *model.TorrentInfo, ec *evaluateContext, rule *model.DeleteRule, result *EvaluateResult) {
	switch rule.Action {
	case "delete":
		e.executeRuleDelete(ctx, rec, ti, ec, rule, result)
	case "pause":
		e.executeRulePause(ctx, rec, ec, rule, result)
	case "limit_speed":
		e.executeRuleLimitSpeed(ctx, rec, ec, rule, result)
	default:
		e.logger.Warn("unknown rule action", zap.String("action", rule.Action), zap.Uint("ruleID", rule.ID))
	}
}

func (e *Engine) executeRuleDelete(ctx context.Context, rec *model.SeedingTorrentRecord, ti *model.TorrentInfo, ec *evaluateContext, rule *model.DeleteRule, result *EvaluateResult) {
	deleteFiles := rule.RemoveData && !rule.OnlyDeleteTorrent

	if ti != nil && HasSameFileTorrent(ti, ec.torrents) {
		deleteFiles = false
	}

	if rule.ReannounceBefore && ti != nil {
		e.reannounceRuleBeforeDelete(ctx, ec.client, rec.InfoHash, rule)
	}

	cascaded := false
	for _, cr := range ec.cascadeRules {
		if cr.CascadeDelete && ti != nil {
			relatedHashes := FindRelatedByTagOrPath(ti, ec.torrents, cr.CascadeMaxDepth)
			if len(relatedHashes) > 0 {
				allHashes := append([]string{rec.InfoHash}, relatedHashes...)
				if err := ec.client.BatchDeleteTorrents(ctx, allHashes, deleteFiles); err != nil {
					e.logger.Warn("rule cascade delete failed",
						zap.Strings("hashes", allHashes),
						zap.Error(err))
					result.Errors++
					if usErr := e.UpdateStatus(ctx, rec.ID, model.SeedingStatusDeleteFailed, "rule:"+rule.Alias); usErr != nil {
						e.logger.Error("update rule cascade fail status error", zap.Uint("id", rec.ID), zap.Error(usErr))
					}
				} else {
					e.saveFinalTraffic(ctx, rec, ti)
					e.logger.Info("rule cascade delete success",
						zap.String("source", rec.InfoHash),
						zap.Int("related", len(relatedHashes)),
						zap.Uint("ruleID", rule.ID))
					if usErr := e.UpdateStatus(ctx, rec.ID, model.SeedingStatusDeleting, "rule:"+rule.Alias); usErr != nil {
						e.logger.Error("update rule cascade status error", zap.Uint("id", rec.ID), zap.Error(usErr))
					}
					e.markRelatedDeleted(ctx, relatedHashes, ec.clientID, "rule:"+rule.Alias)
					result.Deleted++
				}
				cascaded = true
				break
			}
		}
	}

	if !cascaded {
		e.saveFinalTraffic(ctx, rec, ti)
		if err := ec.client.DeleteTorrent(ctx, rec.InfoHash, deleteFiles); err != nil {
			e.logger.Warn("rule delete failed",
				zap.String("infoHash", rec.InfoHash),
				zap.Bool("deleteFiles", deleteFiles),
				zap.Error(err))
			result.Errors++
			if usErr := e.UpdateStatus(ctx, rec.ID, model.SeedingStatusDeleteFailed, "rule:"+rule.Alias); usErr != nil {
				e.logger.Error("update rule delete fail status error", zap.Uint("id", rec.ID), zap.Error(usErr))
			}
			e.fitTimer.Remove(rec.InfoHash)
			return
		}
		if err := e.UpdateStatus(ctx, rec.ID, model.SeedingStatusDeleting, "rule:"+rule.Alias); err != nil {
			e.logger.Error("update rule delete status failed", zap.Uint("id", rec.ID), zap.Error(err))
		}
		result.Deleted++
	}

	e.fitTimer.Remove(rec.InfoHash)
}

func (e *Engine) executeRulePause(ctx context.Context, rec *model.SeedingTorrentRecord, ec *evaluateContext, rule *model.DeleteRule, result *EvaluateResult) {
	if err := ec.client.PauseTorrent(ctx, rec.InfoHash); err != nil {
		e.logger.Warn("rule pause failed",
			zap.String("infoHash", rec.InfoHash),
			zap.Error(err))
		result.Errors++
		return
	}
	if err := e.UpdateStatus(ctx, rec.ID, model.SeedingStatusPausedRule, "rule:"+rule.Alias); err != nil {
		e.logger.Error("update rule pause status failed", zap.Uint("id", rec.ID), zap.Error(err))
	}
	result.Paused++
	e.fitTimer.Remove(rec.InfoHash)
}

func (e *Engine) executeRuleLimitSpeed(ctx context.Context, rec *model.SeedingTorrentRecord, ec *evaluateContext, rule *model.DeleteRule, result *EvaluateResult) {
	if rule.LimitSpeedBytes <= 0 {
		return
	}
	actionKey := "limit_speed:" + rule.Alias
	if rec.LastActionBy == actionKey {
		return
	}
	if err := ec.client.SetUploadLimit(ctx, rec.InfoHash, rule.LimitSpeedBytes); err != nil {
		e.logger.Warn("rule limit speed failed",
			zap.String("infoHash", rec.InfoHash),
			zap.Int64("limit", rule.LimitSpeedBytes),
			zap.Error(err))
		result.Errors++
		return
	}
	e.logger.Info("rule limited speed",
		zap.String("infoHash", rec.InfoHash),
		zap.Int64("limitBytes", rule.LimitSpeedBytes),
		zap.Uint("ruleID", rule.ID))
	e.fitTimer.Remove(rec.InfoHash)
	if err := e.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).
		Where("id = ?", rec.ID).
		Updates(map[string]interface{}{
			"last_action_by": actionKey,
			"updated_at":     time.Now(),
		}).Error; err != nil {
		e.logger.Warn("update limit_speed action_by failed", zap.Uint("id", rec.ID), zap.Error(err))
	}
	result.Limited++
}

func (e *Engine) reannounceRuleBeforeDelete(ctx context.Context, client model.DownloaderClient, infoHash string, rule *model.DeleteRule) bool {
	retries := rule.ReannounceRetries
	if retries <= 0 {
		retries = 2
	}
	interval := time.Duration(rule.ReannounceIntervalMs) * time.Millisecond
	if interval <= 0 {
		interval = 3 * time.Second
	}

	for i := 0; i < retries; i++ {
		if ctx.Err() != nil {
			return false
		}
		if err := client.Reannounce(ctx, infoHash); err != nil {
			e.logger.Debug("rule reannounce failed",
				zap.String("infoHash", infoHash),
				zap.Int("attempt", i+1),
				zap.Error(err))
			continue
		}

		select {
		case <-ctx.Done():
			return false
		case <-time.After(interval):
		}

		ti, err := client.GetTorrentByHash(ctx, infoHash)
		if err != nil || ti == nil {
			continue
		}
		if ti.NumIncomplete > 0 || ti.UploadSpeed > 0 {
			e.logger.Info("rule reannounce before delete: peers still active, stats reported",
				zap.String("infoHash", infoHash),
				zap.Int("leechers", ti.NumIncomplete),
				zap.Int64("uploadSpeed", ti.UploadSpeed))
			return true
		}
	}

	return false
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
	var cfg model.SeedingClientConfig
	if err := e.db.WithContext(ctx).Where("client_id = ?", clientID).First(&cfg).Error; err == nil {
		if !IsWithinActiveWindow(cfg.ActiveTimeWindows) {
			e.logger.Debug("outside active time windows, skipping add",
				zap.String("client_id", clientID),
				zap.String("windows", cfg.ActiveTimeWindows))
			return nil
		}
	}

	key := recordKey(clientID, event.InfoHash)

	e.mu.RLock()
	_, exists := e.recordMap[key]
	e.mu.RUnlock()
	if exists {
		return nil
	}

	record := &model.SeedingTorrentRecord{
		ClientID:       clientID,
		SiteName:       event.SiteName,
		TorrentID:      event.TorrentID,
		InfoHash:       event.InfoHash,
		Discount:       event.Discount,
		HasHR:          event.HasHR,
		HRSeedTimeH:    event.HRSeedTimeH,
		Source:         "rss",
		Status:         model.SeedingStatusPending,
		SubscriptionID: event.SourceID,
		IsFree:         event.Discount == model.DiscountFree || event.Discount == model.Discount2xFree,
		FreeEndAt:      event.FreeEndAt,
		TorrentSize:    event.Size,
	}

	if !record.IsFree && record.Discount != model.DiscountAssumeFree && e.siteProvider != nil && record.SiteName != "" {
		if siteInfo, infoErr := e.siteProvider.GetSiteInfo(ctx, record.SiteName); infoErr == nil && siteInfo != nil && siteInfo.AssumeFree {
			record.IsFree = true
			record.Discount = model.DiscountAssumeFree
		}
	}

	result := e.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "client_id"}, {Name: "info_hash"}},
		DoNothing: true,
	}).Create(record)
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		var loaded model.SeedingTorrentRecord
		if err := e.db.WithContext(ctx).
			Where("client_id = ? AND info_hash = ?", clientID, event.InfoHash).
			First(&loaded).Error; err != nil {
			return err
		}
		record = &loaded
	}

	e.mu.Lock()
	e.recordMap[key] = record
	e.mu.Unlock()

	if e.freeEndMonitor != nil && record.FreeEndAt != nil {
		e.freeEndMonitor.Schedule(record)
	}
	return nil
}

func (e *Engine) Clear(ctx context.Context, clientID string) error {
	e.mu.Lock()
	for key, r := range e.recordMap {
		if r.ClientID == clientID && r.Source == "rss" {
			delete(e.recordMap, key)
		}
	}
	e.mu.Unlock()

	return e.db.WithContext(ctx).
		Where("client_id = ? AND source = ?", clientID, "rss").
		Delete(&model.SeedingTorrentRecord{}).Error
}

func (e *Engine) CollectTrafficStats(ctx context.Context) error {
	if e.getClientProvider() == nil {
		return nil
	}

	clients := e.getClientProvider().ListClients()
	now := time.Now()

	for _, clientID := range clients {
		var md *model.Maindata

		if cached := e.getCachedMaindata(clientID); cached != nil {
			md = cached.Maindata
		}

		if md == nil {
			dlClient, err := e.getClientProvider().Get(clientID)
			if err != nil {
				e.logger.Debug("获取下载器失败，跳过", zap.String("clientID", clientID), zap.Error(err))
				continue
			}

			var fetchErr error
			md, fetchErr = dlClient.GetMainData(ctx)
			if fetchErr != nil || md == nil {
				e.logger.Debug("获取下载器数据失败，跳过", zap.String("clientID", clientID), zap.Error(fetchErr))
				continue
			}
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

		e.collectTorrentTraffic(ctx, clientID, md, now)
	}

	return nil
}

func (e *Engine) collectTorrentTraffic(ctx context.Context, clientID string, md *model.Maindata, now time.Time) {
	var records []model.SeedingTorrentRecord
	e.db.WithContext(ctx).
		Where("client_id = ? AND status IN ?", clientID,
			[]string{string(model.SeedingStatusPending), string(model.SeedingStatusSeeding), "paused_free_end", "paused_rule"}).
		Find(&records)

	if len(records) == 0 || md == nil {
		return
	}

	trafficBatch := make([]*model.TorrentTraffic, 0, len(records))
	for _, rec := range records {
		ti, ok := md.Torrents[rec.InfoHash]
		if !ok {
			continue
		}
		trafficBatch = append(trafficBatch, &model.TorrentTraffic{
			ClientID:      clientID,
			InfoHash:      rec.InfoHash,
			SiteName:      rec.SiteName,
			Uploaded:      ti.Uploaded,
			Downloaded:    ti.Downloaded,
			UploadSpeed:   ti.UploadSpeed,
			DownloadSpeed: ti.DownloadSpeed,
			Ratio:         ti.Ratio,
			RecordedAt:    now,
		})
	}

	if len(trafficBatch) > 0 {
		if err := e.db.WithContext(ctx).Create(&trafficBatch).Error; err != nil {
			e.logger.Warn("batch write torrent_traffic failed",
				zap.String("clientID", clientID),
				zap.Int("count", len(trafficBatch)),
				zap.Error(err))
		}
	}
}

// collectSiteTrafficDaily writes instantaneous cumulative upload SUM to site_traffic_daily.
// Note: upload_delta is NOT an incremental value — it's overwritten each cycle with the
// current SUM of ti.Uploaded for active torrents. The stats API no longer uses this field
// for "today's upload" display (uses torrent_traffic max-min instead).
// site_traffic_daily is retained for site-level trend charts only.
func (e *Engine) collectSiteTrafficDaily(ctx context.Context, clientID string, md *model.Maindata, now time.Time) {
	today := now.Truncate(24 * time.Hour)

	var records []model.SeedingTorrentRecord
	e.db.WithContext(ctx).
		Where("client_id = ? AND status IN ?", clientID,
			[]string{string(model.SeedingStatusPending), string(model.SeedingStatusSeeding), "paused_free_end", "paused_rule"}).
		Find(&records)

	siteCount := make(map[string]int)
	siteUpload := make(map[string]int64)
	for _, rec := range records {
		siteCount[rec.SiteName]++
		if md != nil {
			if ti, ok := md.Torrents[rec.InfoHash]; ok {
				siteUpload[rec.SiteName] += ti.Uploaded
			}
		}
	}

	if len(siteCount) > 0 {
		siteNames := make([]string, 0, len(siteCount))
		for name := range siteCount {
			siteNames = append(siteNames, name)
		}
		var existingList []model.SiteTrafficDaily
		e.db.WithContext(ctx).
			Where("site_name IN ? AND date = ?", siteNames, today).
			Find(&existingList)
		existingMap := make(map[string]*model.SiteTrafficDaily, len(existingList))
		for i := range existingList {
			existingMap[existingList[i].SiteName] = &existingList[i]
		}

		for siteName, count := range siteCount {
			uploaded := siteUpload[siteName]
			if existing, ok := existingMap[siteName]; ok {
				updates := map[string]interface{}{
					"seeding_count": count,
					"upload_delta":  uploaded,
				}
				if err := e.db.WithContext(ctx).Model(existing).Updates(updates).Error; err != nil {
					e.logger.Warn("update site traffic daily failed",
						zap.String("site", siteName),
						zap.Error(err))
				}
			} else {
				entry := &model.SiteTrafficDaily{
					SiteName:     siteName,
					Date:         today,
					SeedingCount: count,
					TorrentCount: count,
					UploadDelta:  uploaded,
				}
				if createErr := e.db.WithContext(ctx).Create(entry).Error; createErr != nil {
					e.logger.Warn("create site traffic daily failed",
						zap.String("site", siteName),
						zap.Error(createErr),
					)
				}
			}
		}
	}
}

type RealTorrentCounts struct {
	Seeding     int   `json:"seeding"`
	Downloading int   `json:"downloading"`
	Paused      int   `json:"paused"`
	Total       int   `json:"total"`
	TotalSize   int64 `json:"totalSize"`
}

func (e *Engine) GetRealTorrentCounts() map[string]*RealTorrentCounts {
	e.maindataMu.RLock()
	defer e.maindataMu.RUnlock()

	result := make(map[string]*RealTorrentCounts)
	for clientID, entry := range e.maindataCache {
		counts := &RealTorrentCounts{}
		for _, ti := range entry.Maindata.Torrents {
			if ti.Removed {
				continue
			}
			counts.Total++
			counts.TotalSize += ti.TotalSize
			state := strings.ToLower(ti.State)
			switch {
			case state == "uploading" || state == "stalledup" || state == "forcedup" ||
				state == "queuedup" || state == "checkingup":
				counts.Seeding++
			case state == "downloading" || state == "stalleddl" || state == "forceddl" ||
				state == "metadl" || state == "forcedmetadl" || state == "checkingdl" || state == "queueddl":
				counts.Downloading++
			case ti.IsPaused || state == "pausedup" || state == "stoppedup" ||
				strings.HasSuffix(state, "dl"):
				counts.Paused++
			}
		}
		result[clientID] = counts
	}
	return result
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
	snapshotUp := state.UploadSpeed
	snapshotDown := state.DownloadSpeed
	e.mu.Unlock()

	var dbState model.SeedingClientState
	err := e.db.WithContext(ctx).Where("client_id = ?", clientID).First(&dbState).Error

	var globalStats *model.GlobalTransferStats
	if e.getClientProvider() != nil {
		if client, cErr := e.getClientProvider().Get(clientID); cErr == nil {
			if gs, gsErr := client.GetGlobalTransferStats(ctx); gsErr == nil {
				globalStats = gs
			}
		}
	}

	if err != nil {
		dbState = model.SeedingClientState{
			ClientID:         clientID,
			AvgUploadSpeed:   snapshotUp,
			AvgDownloadSpeed: snapshotDown,
			Initialized:      true,
		}
		if globalStats != nil {
			dbState.AllTimeUpload = globalStats.AllTimeUpload
			dbState.AllTimeDownload = globalStats.AllTimeDownload
		}
		if err := e.db.WithContext(ctx).Create(&dbState).Error; err != nil {
			e.logger.Warn("create seeding client state failed",
				zap.String("client_id", clientID),
				zap.Error(err))
		}
	} else {
		updates := map[string]interface{}{
			"avg_upload_speed":   snapshotUp,
			"avg_download_speed": snapshotDown,
			"initialized":        true,
		}
		if globalStats != nil {
			updates["all_time_upload"] = globalStats.AllTimeUpload
			updates["all_time_download"] = globalStats.AllTimeDownload

			today := time.Now().Format("2006-01-02")
			if dbState.DayStartDate != today {
				updates["day_start_upload"] = globalStats.AllTimeUpload
				updates["day_start_download"] = globalStats.AllTimeDownload
				updates["day_start_date"] = today
			}
		}
		if err := e.db.WithContext(ctx).Model(&dbState).Updates(updates).Error; err != nil {
			e.logger.Warn("update seeding client state failed",
				zap.String("client_id", clientID),
				zap.Error(err))
		}
	}
}

func (e *Engine) refreshDiscountStatus(ctx context.Context, records []model.SeedingTorrentRecord) {
	if e.siteProvider == nil {
		return
	}

	var targets []model.SeedingTorrentRecord
	for i := range records {
		r := &records[i]
		if r.IsFree && r.FreeEndAt == nil && r.SiteName != "" && r.TorrentID != "" &&
			(r.Status == model.SeedingStatusSeeding || r.Status == model.SeedingStatusPending) {
			targets = append(targets, *r)
		}
	}
	if len(targets) == 0 {
		return
	}

	siteGroups := make(map[string][]*model.SeedingTorrentRecord)
	for i := range targets {
		t := &targets[i]
		siteGroups[t.SiteName] = append(siteGroups[t.SiteName], t)
	}

	for siteName, recs := range siteGroups {
		adapter, adpErr := e.siteProvider.GetAdapter(ctx, siteName)
		if adpErr != nil {
			continue
		}
		siteCfg, cfgErr := e.siteProvider.GetSiteConfig(ctx, siteName)
		if cfgErr != nil || siteCfg == nil {
			continue
		}

		for _, rec := range recs {
			if ctx.Err() != nil {
				return
			}
			recheckCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			discResult, discErr := adapter.DetectDiscount(recheckCtx, siteCfg, rec.TorrentID)
			cancel()
			if discErr != nil {
				e.logger.Debug("refresh discount: check failed",
					zap.String("site", siteName),
					zap.String("torrent_id", rec.TorrentID),
					zap.Error(discErr))
				continue
			}

			nowFree := discResult != nil && discResult.Level.IsFree()
			if nowFree {
				continue
			}

			newDiscount := model.DiscountNone
			if discResult != nil {
				newDiscount = discResult.Level
			}

			e.logger.Info("refresh discount: 种子已不再免费",
				zap.String("site", siteName),
				zap.String("torrent_id", rec.TorrentID),
				zap.String("info_hash", rec.InfoHash),
				zap.String("new_discount", string(newDiscount)))

			if dbErr := e.db.WithContext(ctx).Model(rec).Updates(map[string]interface{}{
				"is_free":    false,
				"discount":   newDiscount,
				"updated_at": time.Now(),
			}).Error; dbErr != nil {
				e.logger.Warn("refresh discount: DB update failed",
					zap.String("torrent_id", rec.TorrentID),
					zap.Error(dbErr))
				continue
			}

			e.mu.Lock()
			key := recordKey(rec.ClientID, rec.InfoHash)
			if r, ok := e.recordMap[key]; ok {
				r.IsFree = false
				r.Discount = newDiscount
			}
			e.mu.Unlock()
		}
	}
}

var defaultUnregisteredKeywords = []string{
	"unregistered torrent",
	"unregistered",
	"torrent not found",
	"torrent not exist",
	"not registered",
	"unknown torrent",
	"invalid torrent",
	"torrent has been deleted",
}

func (e *Engine) getUnregisteredKeywords() []string {
	var val string
	row := e.db.Raw("SELECT value FROM system_settings WHERE key = 'seeding.unregistered_keywords' LIMIT 1").Row()
	if err := row.Scan(&val); err != nil || val == "" {
		return defaultUnregisteredKeywords
	}
	var keywords []string
	if json.Unmarshal([]byte(val), &keywords) == nil && len(keywords) > 0 {
		return keywords
	}
	return defaultUnregisteredKeywords
}

func (e *Engine) checkUnregisteredTorrents(ctx context.Context, clientID string, dlClient model.DownloaderClient) {
	if !e.unregisteredChecking.CompareAndSwap(false, true) {
		return
	}
	defer e.unregisteredChecking.Store(false)

	keywords := e.getUnregisteredKeywords()

	e.mu.RLock()
	var candidates []*model.SeedingTorrentRecord
	for _, rec := range e.recordMap {
		if rec.ClientID == clientID && !rec.Unregistered && (rec.Status == model.SeedingStatusSeeding || rec.Status == model.SeedingStatusPausedFreeEnd || rec.Status == model.SeedingStatusPausedRule) {
			candidates = append(candidates, rec)
		}
	}
	e.mu.RUnlock()

	if len(candidates) == 0 {
		return
	}

	batchSize := 20
	cursor := int(e.unregisteredCursor.Load())
	if cursor >= len(candidates) {
		cursor = 0
	}
	end := cursor + batchSize
	if end > len(candidates) {
		end = len(candidates)
	}
	batch := candidates[cursor:end]
	e.unregisteredCursor.Store(int64(end))

	for _, rec := range batch {
		if ctx.Err() != nil {
			return
		}
		msg, err := dlClient.GetTrackerMessages(ctx, rec.InfoHash)
		if err != nil || msg == "" {
			continue
		}
		msgLowered := strings.ToLower(msg)
		matched := false
		for _, kw := range keywords {
			if strings.Contains(msgLowered, strings.ToLower(kw)) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}

		e.logger.Info("unregistered torrent detected",
			zap.String("client_id", clientID),
			zap.String("site", rec.SiteName),
			zap.String("torrent_id", rec.TorrentID),
			zap.String("info_hash", rec.InfoHash),
			zap.String("tracker_msg", msg))

		if err := dlClient.PauseTorrent(ctx, rec.InfoHash); err != nil {
			e.logger.Warn("unregistered: pause failed", zap.String("hash", rec.InfoHash), zap.Error(err))
		}

		now := time.Now()
		if err := e.db.WithContext(ctx).Model(rec).Updates(map[string]interface{}{
			"status":           model.SeedingStatusUnregistered,
			"unregistered":     true,
			"unregistered_at":  now,
			"unregistered_msg": msg,
			"last_action_by":   "unregistered_patrol",
		}).Error; err != nil {
			e.logger.Warn("unregistered: update DB failed", zap.Error(err))
			continue
		}

		e.mu.Lock()
		rec.Status = model.SeedingStatusUnregistered
		rec.Unregistered = true
		rec.UnregisteredAt = &now
		rec.UnregisteredMsg = msg
		e.mu.Unlock()

		if err := dlClient.DeleteTorrent(ctx, rec.InfoHash, true); err != nil {
			e.logger.Warn("unregistered: delete torrent+files failed", zap.String("hash", rec.InfoHash), zap.Error(err))
		} else {
			e.logger.Info("unregistered: deleted torrent and files",
				zap.String("info_hash", rec.InfoHash),
				zap.String("site", rec.SiteName),
				zap.String("torrent_id", rec.TorrentID))
		}

		key := recordKey(rec.ClientID, rec.InfoHash)
		e.mu.Lock()
		delete(e.recordMap, key)
		e.mu.Unlock()

		e.db.WithContext(ctx).Model(rec).Updates(map[string]interface{}{
			"status":         model.SeedingStatusDeleted,
			"last_action_by": "unregistered_patrol",
		})
	}
}
