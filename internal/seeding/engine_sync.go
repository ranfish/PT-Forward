package seeding

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/site"
	"go.uber.org/zap"
)

func (e *Engine) syncStaleRecords(ctx context.Context, clientUID uint, torrentMap map[string]*model.TorrentInfo) {
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
		if rec.ClientUID != clientUID {
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
				zap.Uint("client_uid", clientUID),
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
			zap.Uint("client_uid", clientUID),
			zap.Int("stale_count", len(staleHashes)))
		for i := 0; i < len(staleHashes); i += sqliteVarLimit {
			end := i + sqliteVarLimit
			if end > len(staleHashes) {
				end = len(staleHashes)
			}
			e.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).
				Where("client_uid = ? AND info_hash IN ?", clientUID, staleHashes[i:end]).
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
			Where("client_uid = ? AND LOWER(info_hash) IN ? AND status = ?", clientUID, orphanHashes[i:end], model.SeedingStatusDeleted).
			Find(&partial).Error; dbErr != nil {
			e.logger.Warn("syncStale: query orphan records failed", zap.Uint("client_uid", clientUID), zap.Error(dbErr))
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
						zap.Uint("client_uid", clientUID),
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
				zap.Uint("client_uid", clientUID),
				zap.Int("count", len(recoverHashes)),
				zap.Strings("info_hashes", recoverHashes))
			for i := 0; i < len(recoverHashes); i += sqliteVarLimit {
				end := i + sqliteVarLimit
				if end > len(recoverHashes) {
					end = len(recoverHashes)
				}
				e.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).
					Where("client_uid = ? AND info_hash IN ?", clientUID, recoverHashes[i:end]).
					Updates(map[string]interface{}{
						"status":         model.SeedingStatusSeeding,
						"last_action_by": "",
						"updated_at":     time.Now(),
					})
			}

			e.mu.Lock()
			for _, hash := range recoverHashes {
				key := recordKey(clientUID, hash)
				if _, exists := e.recordMap[key]; !exists {
					var rec model.SeedingTorrentRecord
					if err := e.db.WithContext(ctx).
						Where("client_uid = ? AND info_hash = ?", clientUID, hash).
						First(&rec).Error; err == nil {
						e.recordMap[key] = &rec
					}
				}
			}
			e.mu.Unlock()
		}
	}
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
	for _, clientUID := range clients {
		if ctx.Err() != nil {
			return
		}

		dlClient, err := e.getClientProvider().Get(clientUID)
		if err != nil {
			continue
		}

		e.maindataMu.RLock()
		prev := e.maindataCache[clientUID]
		e.maindataMu.RUnlock()

		torrentMap, freeSpace, totalDiskSpace, newRid, wasFull := e.syncMaindata(ctx, clientUID, dlClient, prev)

		if torrentMap == nil {
			continue
		}

		e.checkSpaceAlarm(ctx, clientUID, freeSpace)

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
		e.maindataCache[clientUID] = entry
		e.maindataMu.Unlock()

		e.updateEMA(ctx, clientUID, entry.Maindata, torrentMap)
		e.syncStaleRecords(ctx, clientUID, torrentMap)
		e.checkUnregisteredTorrents(ctx, clientUID, dlClient, torrentMap)
		e.logOrphanTorrents(ctx, clientUID, torrentMap)
		e.syncUnmanagedTorrents(ctx, clientUID, torrentMap)
		e.reconcileStaleRecords(ctx, clientUID, torrentMap)
		e.checkAutoTransfer(ctx, clientUID, torrentMap, dlClient)
	}
}

func (e *Engine) syncMaindata(ctx context.Context, clientUID uint, dlClient model.DownloaderClient, prev *maindataEntry) (torrentMap map[string]*model.TorrentInfo, freeSpace, totalDiskSpace int64, newRid int, wasFull bool) {
	forceFull := prev == nil || prev.Rid == 0 || len(prev.TorrentMap) == 0 || prev.CyclesSinceFull >= forceFullSyncInterval

	if forceFull {
		md, err := dlClient.GetMainData(ctx)
		if err != nil || md == nil {
			if err != nil {
				e.logger.Warn("refresh maindata (full) failed",
					zap.Uint("client_uid", clientUID),
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
				zap.Uint("client_uid", clientUID),
				zap.Error(err))
		}
		mdFull, fullErr := dlClient.GetMainData(ctx)
		if fullErr != nil || mdFull == nil {
			if fullErr != nil {
				e.logger.Warn("refresh maindata (fallback full) failed",
					zap.Uint("client_uid", clientUID),
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
			zap.Uint("client_uid", clientUID),
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
func (e *Engine) logOrphanTorrents(ctx context.Context, clientUID uint, torrentMap map[string]*model.TorrentInfo) {
	var configCount int64
	if err := e.db.WithContext(ctx).Model(&model.SeedingClientConfig{}).
		Where("client_uid = ? AND enabled = ?", clientUID, true).Count(&configCount).Error; err != nil {
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
			Where("client_uid = ? AND LOWER(info_hash) = ?", clientUID, lowerHash).
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
			zap.Uint("client_uid", clientUID),
			zap.Int("count", len(orphans)),
			zap.Int("total_in_downloader", len(torrentMap)),
			zap.Strings("orphans", logOrphans))
	}
}

// reconcileStaleRecords §59.227 场景1: record 状态机与下载器实际状态对账。
//
// 失联形态（249 生产案）：record ∈ {archived, pausedFreeEnd, pausedRule,
// deleted, deleting} 但 qb 实际做种中——评估白名单不含这些状态 → 永不
// 评估（妻本善良 191.7h 未删案）。对账铁律：下载器事实是唯一真相。
//
// 防抖（§59.227 ①用户定案）：状态持续 >10 分钟才对账——躲开 pause/
// delete 生效延迟窗（免费结束暂停的 record 在 qb 状态更新延迟内仍显示
// UP，误重激活会击穿免费结束保护）。
//
// 语义铁律（§59.227 ④用户定案）：刷流删除规则优先级 > 用户手动 resume
// （托管语义——手动恢复的种子被对账纠正回 seeding 后由规则处置）。
func (e *Engine) reconcileStaleRecords(ctx context.Context, clientUID uint, torrentMap map[string]*model.TorrentInfo) {
	// 只对有启用的 seeding 配置的下载器对账（与 logOrphanTorrents 同门）
	var configCount int64
	if err := e.db.WithContext(ctx).Model(&model.SeedingClientConfig{}).
		Where("client_uid = ? AND enabled = ?", clientUID, true).Count(&configCount).Error; err != nil || configCount == 0 {
		return
	}

	staleStatuses := []string{
		string(model.SeedingStatusArchived),
		string(model.SeedingStatusPausedFreeEnd),
		string(model.SeedingStatusPausedRule),
		string(model.SeedingStatusDeleted),
		string(model.SeedingStatusDeleting),
	}
	var staleRecords []model.SeedingTorrentRecord
	if err := e.db.WithContext(ctx).
		Where("client_uid = ? AND status IN ?", clientUID, staleStatuses).
		Find(&staleRecords).Error; err != nil {
		return
	}

	cutoff := time.Now().Add(-10 * time.Minute)
	reconciled := 0
	for i := range staleRecords {
		rec := &staleRecords[i]
		ti, ok := torrentMap[strings.ToLower(rec.InfoHash)]
		if !ok || ti == nil {
			continue
		}
		// 真做种中判定：uploading/stalledUP/forcedUP（pausedUP 已暂停不算——
		// 与"qb 实际 UP 做种"语义对齐）
		if ti.State != "uploading" && ti.State != "stalledUP" && ti.State != "forcedUP" {
			continue
		}
		// 防抖：状态写入未满 10 分钟不碰（pause/delete 生效延迟窗）
		if rec.UpdatedAt.After(cutoff) {
			continue
		}
		if err := e.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).
			Where("id = ?", rec.ID).
			Updates(map[string]interface{}{
				"status":         model.SeedingStatusSeeding,
				"last_action_by": "reconcile",
				"updated_at":     time.Now(),
			}).Error; err != nil {
			e.logger.Warn("reconcile: failed to reactivate record",
				zap.Uint("id", rec.ID),
				zap.String("info_hash", rec.InfoHash),
				zap.Error(err))
			continue
		}
		e.mu.Lock()
		if r, ok := e.recordMap[recordKey(clientUID, rec.InfoHash)]; ok {
			r.Status = model.SeedingStatusSeeding
			r.LastActionBy = "reconcile"
		} else {
			e.recordMap[recordKey(clientUID, rec.InfoHash)] = rec
		}
		e.mu.Unlock()
		reconciled++
		e.logger.Info("reconcile: stale record reactivated (downloader is seeding)",
			zap.Uint("client_uid", clientUID),
			zap.String("info_hash", rec.InfoHash),
			zap.String("prev_status", string(rec.Status)),
			zap.String("prev_last_action_by", rec.LastActionBy),
			zap.String("downloader_state", ti.State))
	}
}

// reDetailPageURL §59.227 ③: 站点详情页 URL（.torrent 元数据 comment 字段
// 自带——ubits.club/details.php?id=345299 实证）。取 id 参数作 torrent_id。
var reDetailPageURL = regexp.MustCompile(`(?i)https?://[^/\s]+/details\.php\?id=(\d+)`)

// extractTorrentIDFromComment 从种子 comment 提取详情页 tid（空=无详情页 URL）。
func extractTorrentIDFromComment(comment string) string {
	if m := reDetailPageURL.FindStringSubmatch(comment); m != nil {
		return m[1]
	}
	return ""
}

// syncUnmanagedTorrents: when client config scope=all, auto-register torrents
// that exist in the downloader but not in seeding_torrent_records.
func (e *Engine) syncUnmanagedTorrents(ctx context.Context, clientUID uint, torrentMap map[string]*model.TorrentInfo) {
	cfg, err := e.ListConfigs(ctx)
	if err != nil {
		return
	}
	var clientCfg *model.SeedingClientConfig
	for _, c := range cfg {
		if c.ClientUID == clientUID && c.Enabled {
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
		// §59.227: DB 为真相源——内存 recordMap 幽灵（写内存成功/DB 写失败的
		// 残留 key）不再短路导入；以 DB count 为准（Ashes 永久孤儿案根因之一）。
		// DB count 用 LOWER 归一（对齐 logOrphanTorrents 写法——RSS 事件 hash
		// 大写 vs qb 小写形态不一致时旧查询漏判）。
		var count int64
		e.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).
			Where("client_uid = ? AND LOWER(info_hash) = ?", clientUID, strings.ToLower(hash)).
			Count(&count)
		if count > 0 {
			continue
		}

		// Extract site name from tracker URL
		siteName := matcher.Match(ti.TrackerURL)
		if siteName == "" {
			siteName = "unknown"
		}

		// §59.227 ③: comment 详情页 URL 直达——.torrent 元数据自带站方
		// 详情页 URL（qb comment 字段），解析回填 torrent_id（比 tracker
		// 域名推断多拿到 tid，站方重查/追溯直达）。
		torrentID := extractTorrentIDFromComment(ti.Comment)

		newRecords = append(newRecords, &model.SeedingTorrentRecord{
			ClientUID:   clientUID,
			InfoHash:    hash,
			SiteName:    siteName,
			TorrentID:   torrentID,
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
		key := recordKey(rec.ClientUID, rec.InfoHash)
		e.mu.Lock()
		e.recordMap[key] = rec
		e.mu.Unlock()
		inserted++
	}

	if inserted > 0 {
		e.logger.Info("scope=all: imported unmanaged torrents",
			zap.Uint("client_uid", clientUID),
			zap.Int("imported", inserted),
			zap.Int("total_in_downloader", len(torrentMap)),
		)
	}
}

func (e *Engine) getCachedMaindata(clientUID uint) *maindataEntry {
	e.maindataMu.RLock()
	defer e.maindataMu.RUnlock()
	return e.maindataCache[clientUID]
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
			ClientUID:      entry.ClientUID,
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

		if entry.ClientUID != 0 && entry.SubscriptionID != "" {
			e.pushFreeWaitTorrent(ctx, entry)
		}

		return nil
	})
}

func (e *Engine) pushFreeWaitTorrent(ctx context.Context, entry *freeWaitEntry) {
	if e.getClientProvider() == nil {
		return
	}
	dlClient, err := e.getClientProvider().Get(entry.ClientUID)
	if err != nil {
		e.logger.Warn("free wait push: client not available",
			zap.Uint("client_uid", entry.ClientUID),
			zap.Error(err))
		return
	}

	exists, err := dlClient.CheckExists(ctx, entry.InfoHash)
	if err == nil && exists {
		e.logger.Debug("free wait push: already exists in client, ensuring status=seeding",
			zap.String("info_hash", entry.InfoHash))
		now := time.Now()
		e.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).
			Where("client_uid = ? AND info_hash = ? AND status = ?", entry.ClientUID, entry.InfoHash, model.SeedingStatusPending).
			Updates(map[string]interface{}{
				"status":     model.SeedingStatusSeeding,
				"flushed_at": now,
			})
		e.mu.Lock()
		key := recordKey(entry.ClientUID, entry.InfoHash)
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
		zap.Uint("client_uid", entry.ClientUID),
		zap.String("site", entry.SiteName),
		zap.String("torrent_id", entry.TorrentID),
		zap.String("info_hash", entry.InfoHash))

	if addResult != nil && addResult.InfoHash != "" && addResult.InfoHash != entry.InfoHash {
		e.mu.Lock()
		altKey := recordKey(entry.ClientUID, addResult.InfoHash)
		if _, ok := e.recordMap[altKey]; !ok {
			oldKey := recordKey(entry.ClientUID, entry.InfoHash)
			delete(e.recordMap, oldKey)
			e.mu.Unlock()

			e.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).
				Where("client_uid = ? AND info_hash = ?", entry.ClientUID, entry.InfoHash).
				Updates(map[string]interface{}{
					"status":     model.SeedingStatusDeleted,
					"updated_at": now,
				})

			newRecord := &model.SeedingTorrentRecord{
				ClientUID:      entry.ClientUID,
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
			Where("client_uid = ? AND info_hash = ?", entry.ClientUID, entry.InfoHash).
			Updates(map[string]interface{}{
				"status":     model.SeedingStatusSeeding,
				"flushed_at": now,
			})
		e.mu.Lock()
		key := recordKey(entry.ClientUID, entry.InfoHash)
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

func (e *Engine) updateEMA(ctx context.Context, clientUID uint, maindata *model.Maindata, torrentMap map[string]*model.TorrentInfo) {
	var totalUp, totalDown int64
	for _, ti := range torrentMap {
		totalUp += ti.UploadSpeed
		totalDown += ti.DownloadSpeed
	}

	alpha := emaAlpha
	var clientCfg model.SeedingClientConfig
	if err := e.db.WithContext(ctx).Where("client_uid = ?", clientUID).First(&clientCfg).Error; err == nil && clientCfg.EmaAlpha > 0 {
		alpha = clientCfg.EmaAlpha
	}

	e.mu.Lock()
	state, ok := e.emaStates[clientUID]
	if !ok {
		state = &emaState{}
		e.emaStates[clientUID] = state
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
	err := e.db.WithContext(ctx).Where("client_uid = ?", clientUID).First(&dbState).Error

	var globalStats *model.GlobalTransferStats
	if e.getClientProvider() != nil {
		if client, cErr := e.getClientProvider().Get(clientUID); cErr == nil {
			if gs, gsErr := client.GetGlobalTransferStats(ctx); gsErr == nil {
				globalStats = gs
			}
		}
	}

	if err != nil {
		dbState = model.SeedingClientState{
			ClientUID:        clientUID,
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
				zap.Uint("client_uid", clientUID),
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
				zap.Uint("client_uid", clientUID),
				zap.Error(err))
		}
	}
}

// checkSpaceAlarm §33.1.79 空间告警：剩余空间 < 阈值时发 WS 事件 + 日志（5 分钟节流）。
func (e *Engine) checkSpaceAlarm(ctx context.Context, clientUID uint, freeSpace int64) {
	if freeSpace < 0 {
		return
	}
	cfg, ok := e.LoadActiveClientConfig(ctx, clientUID)
	if !ok || !cfg.SpaceAlarmEnabled || cfg.SpaceAlarmGB <= 0 {
		return
	}
	threshold := int64(cfg.SpaceAlarmGB * 1024 * 1024 * 1024)
	if freeSpace >= threshold {
		return
	}

	e.spaceAlarmMu.Lock()
	if last, ok := e.spaceAlarmLast[clientUID]; ok && time.Since(last) < 5*time.Minute {
		e.spaceAlarmMu.Unlock()
		return
	}
	e.spaceAlarmLast[clientUID] = time.Now()
	e.spaceAlarmMu.Unlock()

	level := "warning"
	if freeSpace < threshold/2 {
		level = "critical"
	}

	e.logger.Warn("disk space alarm triggered",
		zap.Uint("client_uid", clientUID),
		zap.Int64("freeSpace", freeSpace),
		zap.Float64("thresholdGB", cfg.SpaceAlarmGB),
		zap.String("level", level))

	if e.wsBroadcaster != nil {
		e.wsBroadcaster.BroadcastWS("system.disk.warning", map[string]interface{}{
			"client_id": clientUID,
			"freeSpace": freeSpace,
			"level":     level,
			"threshold": threshold,
		})
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
			key := recordKey(rec.ClientUID, rec.InfoHash)
			if r, ok := e.recordMap[key]; ok {
				r.IsFree = false
				r.Discount = newDiscount
			}
			e.mu.Unlock()
		}
	}
}
