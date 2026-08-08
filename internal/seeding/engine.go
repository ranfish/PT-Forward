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

	"github.com/ranfish/pt-forward/internal/audit"
	"github.com/ranfish/pt-forward/internal/client"
	"github.com/ranfish/pt-forward/internal/companion"
	"github.com/ranfish/pt-forward/internal/dispatcher"
	"github.com/ranfish/pt-forward/internal/event"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/pusher"
	"github.com/ranfish/pt-forward/internal/site"
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
			e.logger.Debug("syncStale: marking record as deleted (torrent gone from downloader)",
				zap.String("client_id", clientID),
				zap.String("info_hash", rec.InfoHash),
				zap.String("site_name", rec.SiteName),
				zap.String("prev_status", string(rec.Status)),
				zap.Timep("flushed_at", rec.FlushedAt),
				zap.Time("created_at", rec.CreatedAt))
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
					e.logger.Debug("syncStale: skipping recovery of dead-state torrent",
						zap.String("client_id", clientID),
						zap.String("info_hash", rec.InfoHash),
						zap.String("qb_state", ti.State),
						zap.Float64("progress", ti.Progress))
					continue
				}
			}
			recoverHashes = append(recoverHashes, rec.InfoHash)
		}
		if len(recoverHashes) > 0 {
			e.logger.Info("recovering orphan torrents: deleted records still present in downloader",
				zap.String("client_id", clientID),
				zap.Int("count", len(recoverHashes)),
				zap.Strings("info_hashes", recoverHashes))
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
	pusher            *pusher.Pusher
	pendingEvents     chan *pusher.PushedEvent

	unregisteredCursor   atomic.Int64
	unregisteredChecking atomic.Bool

	spaceAlarmMu   sync.Mutex
	spaceAlarmLast map[string]time.Time

	discountCache   map[string]*discountCacheEntry
	discountCacheMu sync.Mutex
}

type discountCacheEntry struct {
	Result    *model.DiscountResult
	CheckedAt time.Time
}

type maindataEntry struct {
	Maindata       *model.Maindata
	FreeSpace      int64
	TotalDiskSpace int64
	UpdatedAt      time.Time
	Rid            int
	TorrentMap     map[string]*model.TorrentInfo
	CyclesSinceFull int
}

type emaState struct {
	UploadSpeed   float64
	DownloadSpeed float64
}

const emaAlpha = 0.3

func NewEngine(db *gorm.DB, logger *zap.Logger) *Engine {
	logger = logger.With(zap.String("component", "seeding"))
	e := &Engine{
		db:              db,
		logger:          logger,
		recordMap:       make(map[string]*model.SeedingTorrentRecord),
		emaStates:       make(map[string]*emaState),
		maindataCache:   make(map[string]*maindataEntry),
		fitTimer:          NewFitTimer(),
		freeWaitMonitor:  NewFreeWaitMonitor(db, logger),
		pendingEvents:    make(chan *pusher.PushedEvent, 1000),
		spaceAlarmLast:   make(map[string]time.Time),
		discountCache:    make(map[string]*discountCacheEntry),
	}
	e.freeEndMonitor = NewFreeEndMonitor(db, nil, logger)
	e.freeEndMonitor.SetEngine(e)
	e.freeWaitMonitor.SetEngine(e)
	return e
}

// GetCachedTorrents 返回某个下载器在 maindataCache 中的全部种子（全部状态）。
// 如果该下载器不在缓存中（未配置/engine 未启动/首次同步未完成），返回 nil。
func (e *Engine) GetCachedTorrents(clientName string) []*model.TorrentInfo {
	e.maindataMu.RLock()
	defer e.maindataMu.RUnlock()
	entry, ok := e.maindataCache[clientName]
	if !ok || entry == nil || len(entry.TorrentMap) == 0 {
		return nil
	}
	result := make([]*model.TorrentInfo, 0, len(entry.TorrentMap))
	for _, t := range entry.TorrentMap {
		if t != nil {
			result = append(result, t)
		}
	}
	return result
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
	for _, clientID := range e.getClientProvider().ListClients() {
		client, err := e.getClientProvider().Get(clientID)
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
	for _, clientID := range e.getClientProvider().ListClients() {
		client, err := e.getClientProvider().Get(clientID)
		if err != nil {
			continue
		}
		currentStats, err := client.GetGlobalTransferStats(ctx)
		if err != nil {
			continue
		}
		var dbState model.SeedingClientState
		if err := e.db.WithContext(ctx).Where("client_id = ?", clientID).First(&dbState).Error; err != nil {
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
		Where("status IN ?", []string{"pending", "seeding", "transferring", "paused_free_end", "paused_rule", "deleting"}).
		Find(&records).Error; err != nil {
		return seedingError(ErrSeedingDB, "load seeding records", err)
	}

	e.mu.Lock()
	for i := range records {
		// 重启时把 transferring 回退为 seeding（转移被中断，下轮 refreshMaindataLoop 重试）
		if records[i].Status == model.SeedingStatusTransferring {
			records[i].Status = model.SeedingStatusSeeding
		}
		key := recordKey(records[i].ClientID, records[i].InfoHash)
		e.recordMap[key] = &records[i]
	}
	e.mu.Unlock()
	// DB 层面把残留的 transferring 回退为 seeding
	e.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).
		Where("status = ?", model.SeedingStatusTransferring).
		Update("status", model.SeedingStatusSeeding)

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

	if e.pusher != nil {
		e.wg.Add(1)
		go func() { defer e.wg.Done(); e.consumeLoop(ctx) }()
	}

	e.wg.Add(1)
	go func() { defer e.wg.Done(); e.archiveLoop(refreshCtx) }()

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

const forceFullSyncInterval = 30

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

		e.maindataMu.RLock()
		prev := e.maindataCache[clientID]
		e.maindataMu.RUnlock()

		torrentMap, freeSpace, totalDiskSpace, newRid, wasFull := e.syncMaindata(ctx, clientID, dlClient, prev)

		if torrentMap == nil {
			continue
		}

		e.checkSpaceAlarm(ctx, clientID, freeSpace)

		cyclesSinceFull := 0
		if prev != nil && !wasFull {
			cyclesSinceFull = prev.CyclesSinceFull + 1
		}

		entry := &maindataEntry{
			FreeSpace:       freeSpace,
			TotalDiskSpace:  totalDiskSpace,
			UpdatedAt:       time.Now(),
			Rid:             newRid,
			TorrentMap:      torrentMap,
			CyclesSinceFull: cyclesSinceFull,
		}
		entry.Maindata = &model.Maindata{
			Torrents:       make(map[string]model.TorrentInfo, len(torrentMap)),
			FreeSpace:      freeSpace,
			TotalDiskSpace: totalDiskSpace,
		}
		for hash, t := range torrentMap {
			entry.Maindata.Torrents[hash] = *t
		}

		e.maindataMu.Lock()
		e.maindataCache[clientID] = entry
		e.maindataMu.Unlock()

		e.updateEMA(ctx, clientID, entry.Maindata, torrentMap)
		e.syncStaleRecords(ctx, clientID, torrentMap)
		e.checkUnregisteredTorrents(ctx, clientID, dlClient)
		e.logOrphanTorrents(ctx, clientID, torrentMap)
		e.syncUnmanagedTorrents(ctx, clientID, torrentMap)
		e.checkAutoTransfer(ctx, clientID, torrentMap, dlClient)
	}
}

func (e *Engine) syncMaindata(ctx context.Context, clientID string, dlClient model.DownloaderClient, prev *maindataEntry) (torrentMap map[string]*model.TorrentInfo, freeSpace, totalDiskSpace int64, newRid int, wasFull bool) {
	forceFull := prev == nil || prev.Rid == 0 || len(prev.TorrentMap) == 0 || prev.CyclesSinceFull >= forceFullSyncInterval

	if forceFull {
		md, err := dlClient.GetMainData(ctx)
		if err != nil || md == nil {
			if err != nil {
				e.logger.Warn("refresh maindata (full) failed",
					zap.String("client", clientID),
					zap.Error(err))
			}
			return nil, 0, 0, 0, false
		}
		torrentMap = make(map[string]*model.TorrentInfo, len(md.Torrents))
		for hash, t := range md.Torrents {
			tCopy := t
			torrentMap[strings.ToLower(hash)] = &tCopy
		}
		return torrentMap, md.FreeSpace, md.TotalDiskSpace, 0, true
	}

	md, rid, err := dlClient.GetMainDataIncremental(ctx, prev.Rid)
	if err != nil || md == nil {
		if err != nil {
			e.logger.Debug("incremental maindata failed, falling back to full",
				zap.String("client", clientID),
				zap.Error(err))
		}
		mdFull, fullErr := dlClient.GetMainData(ctx)
		if fullErr != nil || mdFull == nil {
			if fullErr != nil {
				e.logger.Warn("refresh maindata (fallback full) failed",
					zap.String("client", clientID),
					zap.Error(fullErr))
			}
			return nil, 0, 0, 0, false
		}
		torrentMap = make(map[string]*model.TorrentInfo, len(mdFull.Torrents))
		for hash, t := range mdFull.Torrents {
			tCopy := t
			torrentMap[strings.ToLower(hash)] = &tCopy
		}
		return torrentMap, mdFull.FreeSpace, mdFull.TotalDiskSpace, 0, true
	}

	if md.FullUpdate {
		torrentMap = make(map[string]*model.TorrentInfo, len(md.Torrents))
		for hash, t := range md.Torrents {
			tCopy := t
			torrentMap[strings.ToLower(hash)] = &tCopy
		}
		freeSpace = md.FreeSpace
		if freeSpace == 0 {
			freeSpace = prev.FreeSpace
		}
		totalDiskSpace = prev.TotalDiskSpace
		newRid = rid
		e.logger.Debug("maindata full_update received, rebuilt map",
			zap.String("client", clientID),
			zap.Int("torrents", len(torrentMap)))
		return torrentMap, freeSpace, totalDiskSpace, newRid, true
	}

	torrentMap = make(map[string]*model.TorrentInfo, len(prev.TorrentMap))
	for hash, t := range prev.TorrentMap {
		torrentMap[hash] = t
	}

	for _, hash := range md.TorrentsRemoved {
		delete(torrentMap, strings.ToLower(hash))
		delete(torrentMap, hash)
	}

	for hash, t := range md.Torrents {
		tCopy := t
		torrentMap[strings.ToLower(hash)] = &tCopy
	}

	freeSpace = md.FreeSpace
	if freeSpace == 0 {
		freeSpace = prev.FreeSpace
	}
	totalDiskSpace = prev.TotalDiskSpace
	newRid = rid

	return torrentMap, freeSpace, totalDiskSpace, newRid, false
}

// logOrphanTorrents: logs torrents that exist in the downloader but have no
// tracking record at all (not even deleted). This runs only for clients that
// have seeding configs (i.e., actively managed by the seeding engine) to avoid
// noise from non-seeding clients with many torrents. It only logs, never creates records.
func (e *Engine) logOrphanTorrents(ctx context.Context, clientID string, torrentMap map[string]*model.TorrentInfo) {
	var configCount int64
	if err := e.db.WithContext(ctx).Model(&model.SeedingClientConfig{}).
		Where("client_id = ? AND enabled = ?", clientID, true).Count(&configCount).Error; err != nil {
		return
	}
	if configCount == 0 {
		return
	}

	var orphans []string
	for hash, ti := range torrentMap {
		if ti.State == "error" || ti.Removed {
			continue
		}
		lowerHash := strings.ToLower(hash)

		var count int64
		e.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).
			Where("client_id = ? AND LOWER(info_hash) = ?", clientID, lowerHash).
			Count(&count)
		if count == 0 {
			shortHash := lowerHash
			if len(shortHash) > 12 {
				shortHash = shortHash[:12]
			}
			orphans = append(orphans, fmt.Sprintf("%s(%s)", ti.Name, shortHash))
		}
	}

	if len(orphans) > 0 {
		logOrphans := orphans
		if len(logOrphans) > 20 {
			logOrphans = append(logOrphans[:20], fmt.Sprintf("... and %d more", len(orphans)-20))
		}
		e.logger.Warn("orphan torrents detected: in downloader but not tracked",
			zap.String("client_id", clientID),
			zap.Int("count", len(orphans)),
			zap.Int("total_in_downloader", len(torrentMap)),
			zap.Strings("orphans", logOrphans))
	}
}

// syncUnmanagedTorrents: when client config scope=all, auto-register torrents
// that exist in the downloader but not in seeding_torrent_records.
func (e *Engine) syncUnmanagedTorrents(ctx context.Context, clientID string, torrentMap map[string]*model.TorrentInfo) {
	cfg, err := e.ListConfigs(ctx)
	if err != nil {
		return
	}
	var clientCfg *model.SeedingClientConfig
	for _, c := range cfg {
		if c.ClientID == clientID && c.Enabled {
			clientCfg = c
			break
		}
	}
	if clientCfg == nil || clientCfg.Scope != "all" {
		return
	}

	// v0.0.267: 统一用 site.TrackerMatcher（消除第 3 套内联匹配逻辑）
	matcher := site.NewTrackerMatcher(e.db)

	var newRecords []*model.SeedingTorrentRecord
	for hash, ti := range torrentMap {
		if ti.State == "error" || ti.Removed {
			continue
		}
		key := recordKey(clientID, hash)
		e.mu.RLock()
		_, exists := e.recordMap[key]
		e.mu.RUnlock()
		if exists {
			continue
		}

		// Extract site name from tracker URL
		siteName := matcher.Match(ti.TrackerURL)
		if siteName == "" {
			siteName = "unknown"
		}

		// Check DB to avoid duplicates (race condition safety)
		var count int64
		e.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).
			Where("client_id = ? AND info_hash = ?", clientID, hash).
			Count(&count)
		if count > 0 {
			continue
		}

		newRecords = append(newRecords, &model.SeedingTorrentRecord{
			ClientID:    clientID,
			InfoHash:    hash,
			SiteName:    siteName,
			TorrentID:   "",
			Status:      model.SeedingStatusSeeding,
			Source:      "imported",
			TorrentSize: ti.TotalSize,
			HasHR:       true,
			HRSeedTimeH: defaultHRSeedTimeH(ctx, e.siteProvider, siteName),
		})
	}

	if len(newRecords) == 0 {
		return
	}

	// Batch insert
	inserted := 0
	for _, rec := range newRecords {
		if err := e.db.WithContext(ctx).Create(rec).Error; err != nil {
			continue // Skip duplicates (unique constraint)
		}
		key := recordKey(rec.ClientID, rec.InfoHash)
		e.mu.Lock()
		e.recordMap[key] = rec
		e.mu.Unlock()
		inserted++
	}

	if inserted > 0 {
		e.logger.Info("scope=all: imported unmanaged torrents",
			zap.String("client_id", clientID),
			zap.Int("imported", inserted),
			zap.Int("total_in_downloader", len(torrentMap)),
		)
	}
}

func defaultHRSeedTimeH(ctx context.Context, provider model.SiteInfoProvider, siteName string) int {
	if provider == nil || siteName == "" || siteName == "unknown" {
		return 72
	}
	cfg, err := provider.GetSiteConfig(ctx, siteName)
	if err != nil || cfg == nil {
		return 72
	}
	return cfg.HR.SeedTimeH()
}

func (e *Engine) getCachedMaindata(clientID string) *maindataEntry {
	e.maindataMu.RLock()
	defer e.maindataMu.RUnlock()
	return e.maindataCache[clientID]
}

func (e *Engine) FreeWaitQueue() []FreeWaitEntryInfo {
	if e.freeWaitMonitor == nil {
		return nil
	}
	return e.freeWaitMonitor.ListPending()
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

func (c *siteDiscountChecker) CheckDiscount(ctx context.Context, siteName, torrentID string) (model.DiscountLevel, *time.Time, error) {
	adapter, err := c.provider.GetAdapter(ctx, siteName)
	if err != nil {
		return model.DiscountNone, nil, err
	}
	cfg, cfgErr := c.provider.GetSiteConfig(ctx, siteName)
	if cfgErr != nil {
		return model.DiscountNone, nil, cfgErr
	}
	result, err := adapter.DetectDiscount(ctx, cfg, torrentID)
	if err != nil || result == nil {
		return model.DiscountNone, nil, err
	}
	return result.Level, result.FreeEndAt, nil
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
	e.logger.Info("RemoveSeedingRecord: manually removing seeding record",
		zap.String("client_id", clientID),
		zap.String("info_hash", infoHash))
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

// OnTorrents is a legacy dispatcher handler. Seeding Engine now receives events
// via EventBus (OnPushed → pendingEvents → consumeLoop). This method is kept
// for backward compatibility with subscriptions that have no ClientID.
// Deprecated: Use pusher.EventBus + OnPushed instead.
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
					e.freeWaitMonitor.Add(ev.SiteName, ev.TorrentID, ev.InfoHash, ev.Title, ev.Size, checkBefore, clientID, ev.SourceID, ev.HasHR, ev.HRSeedTimeH, sub.FreeWaitRecheckSec, sub.FreeWaitMinRemain)
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
	totalSpace  int64
	weights     CleanupWeights
	minScore    float64
	minAge      float64
	cfg         *model.SeedingClientConfig
	deleteRules []model.DeleteRule
}

func (e *Engine) prepareEvaluateContext(ctx context.Context, clientID string, cfg *model.SeedingClientConfig) (*evaluateContext, error) {
	if e.getClientProvider() == nil {
		return nil, &model.AppError{Code: 50001, Message: "client provider not configured"}
	}

	dlClient, err := e.getClientProvider().Get(clientID)
	if err != nil {
		return nil, &model.AppError{Code: 50001, Message: "failed to get downloader", Cause: err}
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

	var deleteRules []model.DeleteRule

	// Check global switch: when enabled, all DeleteRules apply to all seeding clients
	var globalDeleteRules string
	e.db.Raw("SELECT value FROM system_settings WHERE key = 'seeding_delete_rules_global' LIMIT 1").Scan(&globalDeleteRules)

	if globalDeleteRules == "true" {
		if dbErr := e.db.WithContext(ctx).
			Where("enabled = ?", true).
			Order("priority DESC").
			Find(&deleteRules).Error; dbErr != nil {
			e.logger.Warn("load delete rules (global) failed", zap.String("client_id", clientID), zap.Error(dbErr))
		}
	} else if cfg != nil && cfg.DeleteRuleIDs != "" {
		ruleIDs := splitRuleIDs(cfg.DeleteRuleIDs)
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
				e.logger.Warn("sync paused_rule to seeding failed", zap.Uint("id", rec.ID), zap.Error(err))
			} else {
				rec.Status = model.SeedingStatusSeeding
				e.logger.Info("detected torrent manually restored in downloader, syncing status to seeding",
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
			ScoreType:   "cleanup", // §55.19 显式标记：这是删种评分（CalculateCleanupScore），不是推送评分
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
				e.logger.Info("disk_protect recovery: free period expired and no longer free, deleting record",
					zap.String("info_hash", rec.InfoHash),
					zap.String("site", rec.SiteName),
					zap.Time("free_end_at", *rec.FreeEndAt))
				if err := e.db.WithContext(ctx).Model(rec).Updates(map[string]interface{}{
					"status":         model.SeedingStatusDeleted,
					"last_action_by": "free_expired_while_paused",
				}).Error; err != nil {
					e.logger.Warn("disk_protect recovery: failed to delete expired record",
						zap.Uint("id", rec.ID), zap.Error(err))
				}
				e.mu.Lock()
				delete(e.recordMap, recordKey(clientID, rec.InfoHash))
				e.mu.Unlock()
				continue
			}
			e.logger.Info("disk_protect recovery: free period expired but still free, continuing recovery",
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
		e.logger.Info("disk_protect recovery: torrent re-enters pending queue",
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

	e.logger.Debug("executeCleanup: starting delete",
		zap.String("client_id", rec.ClientID),
		zap.String("info_hash", rec.InfoHash),
		zap.String("site_name", rec.SiteName),
		zap.String("torrent_id", rec.TorrentID),
		zap.String("last_action_by", rec.LastActionBy),
		zap.Bool("delete_files", isDeleteFiles))

	e.reannounceBeforeDelete(ctx, ec.client, rec.InfoHash, ec.cfg)

	e.saveFinalTraffic(ctx, rec, ti)
	if err := e.deleteTorrentWithCompanions(ctx, ec, rec.InfoHash, isDeleteFiles, true); err != nil {
		e.logger.Warn("delete torrent failed",
			zap.String("infoHash", rec.InfoHash),
			zap.Bool("deleteFiles", isDeleteFiles),
			zap.Error(err),
		)
		result.Errors++
		if usErr := e.UpdateStatus(ctx, rec.ID, model.SeedingStatusDeleteFailed, "auto_cleanup"); usErr != nil {
			e.logger.Error("failed to update delete-failed status", zap.Uint("id", rec.ID), zap.Error(usErr))
		}
		return
	}

	e.logger.Info("executeCleanup: torrent deleted successfully",
		zap.String("client_id", rec.ClientID),
		zap.String("info_hash", rec.InfoHash),
		zap.String("site_name", rec.SiteName),
		zap.String("reason", rec.LastActionBy))

	if err := e.UpdateStatus(ctx, rec.ID, model.SeedingStatusDeleting, "auto_cleanup"); err != nil {
		e.logger.Error("failed to update delete status", zap.Uint("id", rec.ID), zap.Error(err))
	}
	audit.Log("system", "seeding", "delete", "torrent", rec.InfoHash,
		fmt.Sprintf("自动删种 client=%s site=%s reason=%s", rec.ClientID, rec.SiteName, rec.LastActionBy), "success")
	result.Deleted++
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

// deleteTorrentWithCompanions: deletes a torrent and its cross-seeded companions.
// Uses companion.PlanDelete for shared planning logic.
// Companions sharing the same save path as the main torrent are deleted with
// deleteData=true to prevent orphan files; companions with different save
// paths are deleted without data to avoid deleting unrelated files.
func (e *Engine) deleteTorrentWithCompanions(ctx context.Context, ec *evaluateContext, infoHash string, deleteData bool, deleteCompanions bool) error {
	ti := ec.torrentMap[infoHash]
	plan := companion.PlanDelete(ti, ec.torrents, deleteCompanions, deleteData)

	for _, compHash := range plan.CompanionHashes {
		compDeleteData := false
		if plan.DeleteData && ti != nil {
			if compTi, ok := ec.torrentMap[compHash]; ok && compTi != nil {
				if compTi.SavePath == ti.SavePath {
					compDeleteData = true
				}
			}
		}
		if err := ec.client.DeleteTorrent(ctx, compHash, compDeleteData); err != nil {
			e.logger.Warn("companion delete failed (continuing)",
				zap.String("companion_hash", compHash),
				zap.String("main_hash", infoHash),
				zap.Bool("delete_data", compDeleteData),
				zap.Error(err))
		} else {
			e.logger.Info("companion deleted (cascade)",
				zap.String("companion_hash", compHash),
				zap.String("main_hash", infoHash),
				zap.Bool("delete_data", compDeleteData))
		}
	}
	if len(plan.CompanionHashes) > 0 {
		e.markRelatedDeleted(ctx, plan.CompanionHashes, ec.clientID, "companion_cascade")
	}

	err := ec.client.DeleteTorrent(ctx, plan.MainHash, plan.DeleteData)
	if err == nil {
		allDeleted := append(plan.CompanionHashes, plan.MainHash)
		companion.RemoveFromSnapshot(&ec.torrents, ec.torrentMap, allDeleted)
	}
	return err
}

func (e *Engine) removeFromSnapshot(ec *evaluateContext, hashes []string) {
	companion.RemoveFromSnapshot(&ec.torrents, ec.torrentMap, hashes)
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
		e.logger.Debug("evaluate: no records to evaluate",
			zap.String("client_id", clientID))
		return &EvaluateResult{}, nil
	}

	e.logger.Debug("evaluate: starting cycle",
		zap.String("client_id", clientID),
		zap.Int("records", len(ec.records)),
		zap.Int("torrents_in_downloader", len(ec.torrentMap)),
		zap.Bool("dry_run", dryRun))

	result := &EvaluateResult{}
	cycleID := time.Now().Format("20060102-150405")

	if !dryRun && ec.cfg != nil && ec.cfg.DiskProtectEnabled && ec.cfg.MinDiskSpaceGB > 0 && ec.freeSpace >= 0 {
		minBytes := int64(ec.cfg.MinDiskSpaceGB * 1024 * 1024 * 1024)
		if ec.freeSpace >= minBytes {
			e.recoverDiskProtectPaused(ctx, ec.clientID)
		} else {
			e.logger.Warn("disk_protect: insufficient disk space, flush will pause pushing new torrents",
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
				e.logger.Info("pending timeout cleanup",
					zap.Uint("id", rec.ID),
					zap.String("info_hash", rec.InfoHash),
					zap.String("client_id", rec.ClientID),
					zap.Time("created_at", rec.CreatedAt),
				)
				ti := ec.torrentMap[rec.InfoHash]
			if ti != nil {
				if err := ec.client.DeleteTorrent(ctx, rec.InfoHash, true); err != nil {
					e.logger.Warn("pending timeout cleanup: failed to delete downloader torrent", zap.Error(err))
				}
				// Cascade delete companions for pending timeout
				if ec.torrentMap[rec.InfoHash] != nil {
					companions := FindRelatedByTagOrPath(ec.torrentMap[rec.InfoHash], ec.torrents, 1)
					for _, compHash := range companions {
						if err := ec.client.DeleteTorrent(ctx, compHash, false); err != nil {
							e.logger.Warn("pending cleanup: companion delete failed", zap.String("hash", compHash), zap.Error(err))
						}
					}
					e.markRelatedDeleted(ctx, companions, ec.clientID, "pending_companion_cascade")
				}
			}
				if err := e.db.WithContext(ctx).Model(rec).Updates(map[string]interface{}{
					"status":         model.SeedingStatusDeleted,
					"last_action_by": "pending_timeout",
				}).Error; err != nil {
					e.logger.Warn("pending timeout cleanup: failed to update status", zap.Uint("id", rec.ID), zap.Error(err))
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

	if e.wsBroadcaster != nil && result.Deleted > 0 {
		e.wsBroadcaster.BroadcastWS("seeding.cleanup", map[string]interface{}{
			"client_id": clientID,
			"deleted":   result.Deleted,
			"errors":    result.Errors,
		})
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

	e.logger.Debug("executeRuleDelete: starting rule delete",
		zap.String("client_id", rec.ClientID),
		zap.String("info_hash", rec.InfoHash),
		zap.String("site_name", rec.SiteName),
		zap.String("rule_alias", rule.Alias),
		zap.Uint("rule_id", rule.ID),
		zap.Bool("delete_companions", rule.DeleteCompanions))

	if rule.ReannounceBefore && ti != nil {
		e.reannounceRuleBeforeDelete(ctx, ec.client, rec.InfoHash, rule)
	}

	e.saveFinalTraffic(ctx, rec, ti)
	if err := e.deleteTorrentWithCompanions(ctx, ec, rec.InfoHash, true, rule.DeleteCompanions); err != nil {
		e.logger.Warn("rule delete failed",
			zap.String("infoHash", rec.InfoHash),
			zap.Bool("deleteCompanions", rule.DeleteCompanions),
			zap.Error(err))
		result.Errors++
		if usErr := e.UpdateStatus(ctx, rec.ID, model.SeedingStatusDeleteFailed, "rule:"+rule.Alias); usErr != nil {
			e.logger.Error("update rule delete fail status error", zap.Uint("id", rec.ID), zap.Error(usErr))
		}
		e.fitTimer.Remove(rec.InfoHash)
		return
	}
	e.logger.Info("executeRuleDelete: torrent deleted by rule",
		zap.String("client_id", rec.ClientID),
		zap.String("info_hash", rec.InfoHash),
		zap.String("site_name", rec.SiteName),
		zap.String("rule_alias", rule.Alias))
	if e.wsBroadcaster != nil {
		e.wsBroadcaster.BroadcastWS("seeding.deleted", map[string]interface{}{
			"info_hash":  rec.InfoHash,
			"site_name":  rec.SiteName,
			"rule_alias": rule.Alias,
			"client_id":  rec.ClientID,
		})
	}
	if err := e.UpdateStatus(ctx, rec.ID, model.SeedingStatusDeleting, "rule:"+rule.Alias); err != nil {
		e.logger.Error("update rule delete status failed", zap.Uint("id", rec.ID), zap.Error(err))
	}
	result.Deleted++

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
	if err := e.db.WithContext(ctx).Where("enabled = ?", true).Find(&configs).Error; err != nil {
		return nil, err
	}
	return configs, nil
}

// downloadConfigToSeeding 将 DownloadClientConfig 适配为 SeedingClientConfig 视图（§55.14 阶段3）。
// role≠seeding 的下载器配置存于 download_client_configs，刷流引擎统一管理时需转换为 SeedingClientConfig。
// MaxActiveSeeding 从 MaxActiveUploads/Downloads 映射（download_client_configs 无此字段）。
func downloadConfigToSeeding(dc *model.DownloadClientConfig) *model.SeedingClientConfig {
	maxActive := dc.MaxActiveUploads
	if maxActive == 0 {
		maxActive = dc.MaxActiveDownloads
	}
	return &model.SeedingClientConfig{
		ClientID:            dc.ClientID,
		Enabled:             dc.Enabled,
		DeleteRuleIDs:       dc.DeleteRuleIDs,
		AutoDeleteCron:      dc.AutoDeleteCron,
		MainDataCron:        dc.MainDataCron,
		DiskProtectEnabled:  dc.DiskProtectEnabled,
		MinDiskSpaceGB:      dc.MinDiskSpaceGB,
		SpaceAlarmEnabled:   dc.SpaceAlarmEnabled,
		SpaceAlarmGB:        dc.SpaceAlarmGB,
		MinDiskSpacePercent: dc.MinDiskSpacePercent,
		MaxActiveUploads:    dc.MaxActiveUploads,
		MaxActiveDownloads:  dc.MaxActiveDownloads,
		MaxActiveSeeding:    maxActive,
		SuperSeedingDefault: dc.SuperSeedingDefault,
		Scope:               dc.Scope,
	}
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

// Deprecated: Add is legacy code. Seeding Engine now receives events via EventBus
// (OnPushed → pendingEvents → consumeLoop → Pusher.Push → createRecordFromPush).
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
		if loaded.Status == model.SeedingStatusDeleted {
			e.logger.Info("Add: restoring deleted seeding record",
				zap.String("client_id", clientID),
				zap.String("info_hash", event.InfoHash),
				zap.String("site_name", event.SiteName),
				zap.String("torrent_id", event.TorrentID),
				zap.String("prev_last_action_by", loaded.LastActionBy))
			now := time.Now()
			if err := e.db.WithContext(ctx).Model(&loaded).Updates(map[string]interface{}{
				"status":          model.SeedingStatusPending,
				"site_name":       event.SiteName,
				"torrent_id":      event.TorrentID,
				"discount":        event.Discount,
				"has_hr":          event.HasHR,
				"hr_seed_time_h":  event.HRSeedTimeH,
				"source":          "rss",
				"subscription_id": event.SourceID,
				"is_free":         record.IsFree,
				"free_end_at":     event.FreeEndAt,
				"torrent_size":    event.Size,
				"flushed_at":      nil,
				"last_action_by":  "",
				"updated_at":      now,
			}).Error; err != nil {
				return fmt.Errorf("restore deleted record: %w", err)
			}
			loaded.Status = model.SeedingStatusPending
			loaded.FlushedAt = nil
			loaded.LastActionBy = ""
			loaded.SiteName = event.SiteName
			loaded.TorrentID = event.TorrentID
			loaded.Discount = event.Discount
			loaded.HasHR = event.HasHR
			loaded.Source = "rss"
			loaded.SubscriptionID = event.SourceID
			loaded.IsFree = record.IsFree
			loaded.FreeEndAt = event.FreeEndAt
			loaded.TorrentSize = event.Size
			record = &loaded
		} else {
			e.logger.Debug("Add: record already exists, skipping",
				zap.String("client_id", clientID),
				zap.String("info_hash", event.InfoHash),
				zap.String("status", string(loaded.Status)))
			record = &loaded
		}
	} else {
		e.logger.Debug("Add: created new seeding record",
			zap.String("client_id", clientID),
			zap.String("info_hash", event.InfoHash),
			zap.String("site_name", event.SiteName),
			zap.String("torrent_id", event.TorrentID))
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

	return e.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).
		Where("client_id = ? AND source = ? AND status != ?", clientID, "rss", model.SeedingStatusDeleted).
		Updates(map[string]interface{}{
			"status":         model.SeedingStatusDeleted,
			"last_action_by": "clear",
			"updated_at":     time.Now(),
		}).Error
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
				e.logger.Debug("failed to get downloader, skipping", zap.String("clientID", clientID), zap.Error(err))
				continue
			}

			var fetchErr error
			md, fetchErr = dlClient.GetMainData(ctx)
			if fetchErr != nil || md == nil {
				e.logger.Debug("failed to get downloader data, skipping", zap.String("clientID", clientID), zap.Error(fetchErr))
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
			e.logger.Warn("failed to write downloader speed snapshot", zap.String("clientID", clientID), zap.Error(err))
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

	alpha := emaAlpha
	var clientCfg model.SeedingClientConfig
	if err := e.db.WithContext(ctx).Where("client_id = ?", clientID).First(&clientCfg).Error; err == nil && clientCfg.EmaAlpha > 0 {
		alpha = clientCfg.EmaAlpha
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
		state.UploadSpeed = alpha*newUp + (1-alpha)*state.UploadSpeed
		state.DownloadSpeed = alpha*newDown + (1-alpha)*state.DownloadSpeed
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

// checkSpaceAlarm §33.1.79 空间告警：剩余空间 < 阈值时发 WS 事件 + 日志（5 分钟节流）。
func (e *Engine) checkSpaceAlarm(ctx context.Context, clientID string, freeSpace int64) {
	if freeSpace < 0 {
		return
	}
	cfg, ok := e.LoadActiveClientConfig(ctx, clientID)
	if !ok || !cfg.SpaceAlarmEnabled || cfg.SpaceAlarmGB <= 0 {
		return
	}
	threshold := int64(cfg.SpaceAlarmGB * 1024 * 1024 * 1024)
	if freeSpace >= threshold {
		return
	}

	e.spaceAlarmMu.Lock()
	if last, ok := e.spaceAlarmLast[clientID]; ok && time.Since(last) < 5*time.Minute {
		e.spaceAlarmMu.Unlock()
		return
	}
	e.spaceAlarmLast[clientID] = time.Now()
	e.spaceAlarmMu.Unlock()

	level := "warning"
	if freeSpace < threshold/2 {
		level = "critical"
	}

	e.logger.Warn("disk space alarm triggered",
		zap.String("client_id", clientID),
		zap.Int64("freeSpace", freeSpace),
		zap.Float64("thresholdGB", cfg.SpaceAlarmGB),
		zap.String("level", level))

	if e.wsBroadcaster != nil {
		e.wsBroadcaster.BroadcastWS("system.disk.warning", map[string]interface{}{
			"client_id":  clientID,
			"freeSpace":  freeSpace,
			"level":      level,
			"threshold":  threshold,
		})
	}
}

// archiveLoop §14.16/S-P1-7: 每 6 小时归档已删除/暂停的旧记录。
func (e *Engine) archiveLoop(ctx context.Context) {
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.archiveOldRecords(ctx)
		}
	}
}

func (e *Engine) archiveOldRecords(ctx context.Context) {
	cutoff := time.Now().AddDate(0, 0, -14)
	result := e.db.WithContext(ctx).
		Model(&model.SeedingTorrentRecord{}).
		Where("status IN ? AND updated_at < ?",
			[]string{string(model.SeedingStatusDeleted), string(model.SeedingStatusPausedFreeEnd), string(model.SeedingStatusPausedRule)},
			cutoff).
		Update("status", string(model.SeedingStatusArchived))
	if result.RowsAffected > 0 {
		e.logger.Info("archive: records archived",
			zap.Int64("count", result.RowsAffected),
			zap.Time("cutoff", cutoff))
	}

	physicalCutoff := time.Now().AddDate(0, 0, -30)
	delResult := e.db.WithContext(ctx).
		Where("status = ? AND updated_at < ?", string(model.SeedingStatusArchived), physicalCutoff).
		Delete(&model.SeedingTorrentRecord{})
	if delResult.RowsAffected > 0 {
		e.logger.Info("archive: records physically deleted",
			zap.Int64("count", delResult.RowsAffected),
			zap.Time("cutoff", physicalCutoff))
	}

	// 清理过期的 discount 缓存（超过 30 分钟未访问）
	e.discountCacheMu.Lock()
	evicted := 0
	for k, v := range e.discountCache {
		if time.Since(v.CheckedAt) > 30*time.Minute {
			delete(e.discountCache, k)
			evicted++
		}
	}
	e.discountCacheMu.Unlock()
	if evicted > 0 {
		e.logger.Debug("archive: discount cache evicted", zap.Int("count", evicted))
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

			e.logger.Info("refresh discount: torrent no longer free",
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

// checkAutoTransfer 检测下载完成的 record，触发自动转移（§55.11 方案B）。
// 在 refreshMaindataOnce 里调用，复用已有的 torrentMap（10s 轮询），零额外网络成本。
func (e *Engine) checkAutoTransfer(ctx context.Context, clientID string, torrentMap map[string]*model.TorrentInfo, dlClient model.DownloaderClient) {
	if dlClient == nil {
		return
	}
	deadStates := map[string]bool{"stalledDL": true, "missingFiles": true, "error": true, "unknown": true}

	e.mu.RLock()
	var pending []string
	for key, rec := range e.recordMap {
		if rec.ClientID != clientID || !rec.AutoTransfer || rec.Status != model.SeedingStatusSeeding || len(rec.TransferClientIDs) == 0 {
			continue
		}
		ti, ok := torrentMap[strings.ToLower(rec.InfoHash)]
		if !ok || ti == nil || ti.Progress < 1.0 || deadStates[ti.State] {
			continue
		}
		pending = append(pending, key)
	}
	e.mu.RUnlock()

	for _, key := range pending {
		// CAS 状态 seeding→transferring（防 refreshMaindataLoop 重复触发）
		e.mu.Lock()
		rec, ok := e.recordMap[key]
		if !ok || rec.Status != model.SeedingStatusSeeding {
			e.mu.Unlock()
			continue
		}
		rec.Status = model.SeedingStatusTransferring
		e.mu.Unlock()
		e.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).Where("id = ?", rec.ID).Update("status", model.SeedingStatusTransferring)

		ti := torrentMap[strings.ToLower(rec.InfoHash)]
		if ti == nil {
			e.revertTransferring(ctx, key, rec.ID)
			continue
		}
		recCopy := *rec
		go e.transferRecord(context.Background(), recCopy, key)
	}
}

// transferRecord 转移种子到目标下载器：遍历目标 client.TransferTorrent → 删源(deleteData=false) → 删 record
func (e *Engine) transferRecord(ctx context.Context, rec model.SeedingTorrentRecord, key string) {
	if e.clientProvider == nil {
		e.logger.Warn("auto transfer: clientProvider nil", zap.String("hash", rec.InfoHash))
		e.revertTransferring(ctx, key, rec.ID)
		return
	}
	sourceClient, err := e.clientProvider.Get(rec.ClientID)
	if err != nil {
		e.logger.Warn("auto transfer: get source client failed", zap.String("client", rec.ClientID), zap.Error(err))
		e.revertTransferring(ctx, key, rec.ID)
		return
	}

	successCount := 0
	for _, targetID := range rec.TransferClientIDs {
		targetClient, err := e.clientProvider.Get(targetID)
		if err != nil {
			e.logger.Warn("auto transfer: get target client failed", zap.String("target", targetID), zap.Error(err))
			continue
		}
		if _, err := client.TransferTorrent(ctx, sourceClient, targetClient, rec.InfoHash); err != nil {
			e.logger.Warn("auto transfer: transfer to target failed", zap.String("target", targetID), zap.String("hash", rec.InfoHash), zap.Error(err))
			continue
		}
		successCount++
		e.logger.Info("auto transfer: added to target", zap.String("target", targetID), zap.String("hash", rec.InfoHash))
	}

	if successCount == 0 {
		e.logger.Warn("auto transfer: all targets failed, reverting", zap.String("hash", rec.InfoHash))
		e.revertTransferring(ctx, key, rec.ID)
		return
	}

	// 删除源种子（deleteData=false！数据已移交目标下载器，不能删文件）
	if err := sourceClient.DeleteTorrent(ctx, rec.InfoHash, false); err != nil {
		e.logger.Warn("auto transfer: delete source failed (targets already added)", zap.String("hash", rec.InfoHash), zap.Error(err))
	}

	// 删 record（源种子已移交，seedingEngine 不再管理）
	e.mu.Lock()
	delete(e.recordMap, key)
	e.mu.Unlock()
	e.db.WithContext(ctx).Where("id = ?", rec.ID).Delete(&model.SeedingTorrentRecord{})

	e.logger.Info("auto transfer: completed",
		zap.String("hash", rec.InfoHash),
		zap.String("source", rec.ClientID),
		zap.Int("targets", successCount))
}

// revertTransferring 转移失败回退 seeding，下轮 refreshMaindataLoop 重试
func (e *Engine) revertTransferring(ctx context.Context, key string, recID uint) {
	e.mu.Lock()
	if rec, ok := e.recordMap[key]; ok && rec.Status == model.SeedingStatusTransferring {
		rec.Status = model.SeedingStatusSeeding
	}
	e.mu.Unlock()
	e.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).Where("id = ?", recID).Update("status", model.SeedingStatusSeeding)
}
