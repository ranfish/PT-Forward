package seeding

import (
	"context"
	"fmt"
	"time"

	"github.com/ranfish/pt-forward/internal/audit"
	"github.com/ranfish/pt-forward/internal/companion"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

func (e *Engine) CleanupStale(ctx context.Context) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -30)
	staleStatuses := []string{
		string(model.SeedingStatusPausedFreeEnd),
		string(model.SeedingStatusPausedRule),
		string(model.SeedingStatusDeleted),
		string(model.SeedingStatusUnregistered),
	}

	type staleKey struct {
		ClientUID uint
		InfoHash  string
	}
	var stalePairs []staleKey
	if dbErr := e.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).
		Where("status IN ? AND updated_at < ?", staleStatuses, cutoff).
		Select("client_uid, info_hash").
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
		delete(e.recordMap, recordKey(p.ClientUID, p.InfoHash))
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
	activeClients := make(map[uint]bool, len(e.recordMap))
	for _, r := range e.recordMap {
		activeClients[r.ClientUID] = true
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
	// §59.37: 分批删除——单事务 DELETE 24 万行（243 实证 rows:240936, 2.5s 长事务）
	// 阻塞 WAL checkpoint（WAL 涨至 4.5GB 主因之一）。每批 5 万行循环删至清空，
	// 批间释放锁给 checkpoint 窗口。
	{
		const batch = 50000
		var totalDeleted int64
		for {
			res := e.db.WithContext(ctx).Where("id IN (SELECT id FROM scoring_logs WHERE created_at < ? LIMIT ?)",
				scoringCutoff, batch).Delete(&model.ScoringLog{})
			if res.Error != nil {
				e.logger.Warn("cleanup scoring logs batch failed", zap.Error(res.Error))
				break
			}
			totalDeleted += res.RowsAffected
			if res.RowsAffected < batch {
				break
			}
		}
		if totalDeleted > 0 {
			e.logger.Info("cleanup scoring logs completed", zap.Int64("deleted", totalDeleted))
		}
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
	clientUID   uint
	client      model.DownloaderClient
	records     []model.SeedingTorrentRecord
	torrents    []*model.TorrentInfo
	torrentMap  map[string]*model.TorrentInfo
	maindata    *model.Maindata
	freeSpace   int64
	totalSpace  int64
	weights     CleanupWeights
	minScore    float64
	minAge      float64
	cfg         *model.SeedingClientConfig
	deleteRules []model.DeleteRule
}

func (e *Engine) prepareEvaluateContext(ctx context.Context, clientUID uint, cfg *model.SeedingClientConfig) (*evaluateContext, error) {
	if e.getClientProvider() == nil {
		return nil, &model.AppError{Code: 50001, Message: "client provider not configured"}
	}

	dlClient, err := e.getClientProvider().Get(clientUID)
	if err != nil {
		return nil, &model.AppError{Code: 50001, Message: "failed to get downloader", Cause: err}
	}

	records, err := e.ListByClient(ctx, clientUID)
	if err != nil {
		return nil, &model.AppError{Code: 50001, Message: "查询做种记录失败", Cause: err}
	}

	weights := DefaultCleanupWeights()
	minScore := 0.3
	minAgeHours := 48.0

	if cfg != nil {
		minScore, minAgeHours, weights = e.applyConfig(cfg, minScore, minAgeHours, weights)
	}

	// 评分清理只对 Role=seeding 的下载器生效。
	// Role=download 的下载器（原 download_client_configs）仅执行删除规则，不做评分清理。
	if cfg != nil && cfg.Role != "seeding" {
		minScore = -1
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
	if cached := e.getCachedMaindata(clientUID); cached != nil {
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
		e.updateEMA(ctx, clientUID, md, torrentMap)
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
			e.logger.Warn("load delete rules (global) failed", zap.Uint("client_uid", clientUID), zap.Error(dbErr))
		}
	} else if cfg != nil && cfg.DeleteRuleIDs != "" {
		ruleIDs := splitRuleIDs(cfg.DeleteRuleIDs)
		if dbErr := e.db.WithContext(ctx).
			Where("id IN (?) AND enabled = ?", ruleIDs, true).
			Order("priority DESC").
			Find(&deleteRules).Error; dbErr != nil {
			e.logger.Warn("load delete rules failed", zap.Uint("client_uid", clientUID), zap.Error(dbErr))
		}
	}

	return &evaluateContext{
		clientUID:   clientUID,
		client:      dlClient,
		records:     records,
		torrents:    torrents,
		torrentMap:  torrentMap,
		maindata:    maindata,
		freeSpace:   freeSpace,
		totalSpace:  totalSpace,
		weights:     weights,
		minScore:    minScore,
		minAge:      minAgeHours,
		cfg:         cfg,
		deleteRules: deleteRules,
	}, nil
}

// §59.566 刀3 定性：状态推进多步写无事务=周期自愈型（evaluate 循环每轮
// 重判重写·中间态下轮收敛——05 篇 §2.4 分类 B·事务包状态机反而引入长事务）。
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
				zap.Uint("client_uid", rec.ClientUID),
				zap.Time("flushed_at", *rec.FlushedAt))
			return nil, evaluated, false
		}
		if err := e.db.WithContext(ctx).Model(rec).Update("status", model.SeedingStatusDeleted).Error; err != nil {
			e.logger.Warn("update record status to deleted", zap.Uint("id", rec.ID), zap.Error(err))
		}
		e.mu.Lock()
		delete(e.recordMap, recordKey(rec.ClientUID, rec.InfoHash))
		e.mu.Unlock()
		e.logger.Debug("seeding record cleaned: torrent not in downloader",
			zap.String("info_hash", rec.InfoHash),
			zap.Uint("client_uid", rec.ClientUID))
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
			ClientUID:   ec.clientUID,
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
		// §59.153: UPSERT 最新评分到 record——rule_evaluator preload 直读列
		e.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).
			Where("info_hash = ?", rec.InfoHash).
			Updates(map[string]interface{}{
				"last_score":    score,
				"last_cycle_id": cycleID,
			})
	}

	shouldCleanup = ShouldCleanup(*candidate, ec.minScore, ec.minAge)
	return candidate, evaluated, shouldCleanup
}

func (e *Engine) recoverDiskProtectPaused(ctx context.Context, clientUID uint) {
	var paused []model.SeedingTorrentRecord
	if err := e.db.WithContext(ctx).
		Where("client_uid = ? AND status = ? AND last_action_by = ?", clientUID, model.SeedingStatusPausedRule, "disk_protect").
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
				delete(e.recordMap, recordKey(clientUID, rec.InfoHash))
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
		key := recordKey(clientUID, rec.InfoHash)
		if r, ok := e.recordMap[key]; ok {
			r.Status = model.SeedingStatusPending
			r.LastActionBy = "disk_recover"
			r.FlushedAt = nil
		}
		e.mu.Unlock()
		e.logger.Info("disk_protect recovery: torrent re-enters pending queue",
			zap.String("info_hash", rec.InfoHash),
			zap.Uint("client_uid", clientUID))
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
		zap.Uint("client_uid", rec.ClientUID),
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
		zap.Uint("client_uid", rec.ClientUID),
		zap.String("info_hash", rec.InfoHash),
		zap.String("site_name", rec.SiteName),
		zap.String("reason", rec.LastActionBy))

	if err := e.UpdateStatus(ctx, rec.ID, model.SeedingStatusDeleting, "auto_cleanup"); err != nil {
		e.logger.Error("failed to update delete status", zap.Uint("id", rec.ID), zap.Error(err))
	}
	audit.Log("system", "seeding", "delete", "torrent", rec.InfoHash,
		fmt.Sprintf("自动删种 client=%d site=%s reason=%s", rec.ClientUID, rec.SiteName, rec.LastActionBy), "success")
	result.Deleted++
}

func (e *Engine) markRelatedDeleted(ctx context.Context, relatedHashes []string, clientUID uint, actionBy string) {
	if len(relatedHashes) == 0 {
		return
	}
	e.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).
		Where("client_uid = ? AND info_hash IN ? AND status = ?", clientUID, relatedHashes, model.SeedingStatusSeeding).
		Updates(map[string]interface{}{
			"status":         model.SeedingStatusDeleting,
			"last_action_by": actionBy,
			"updated_at":     time.Now(),
		})

	e.mu.Lock()
	for _, hash := range relatedHashes {
		key := recordKey(clientUID, hash)
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
		e.markRelatedDeleted(ctx, plan.CompanionHashes, ec.clientUID, "companion_cascade")
	}

	err := ec.client.DeleteTorrent(ctx, plan.MainHash, plan.DeleteData)
	if err == nil {
		allDeleted := append(append([]string{}, plan.CompanionHashes...), plan.MainHash)
		companion.RemoveFromSnapshot(&ec.torrents, ec.torrentMap, allDeleted)
	}
	return err
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

func (e *Engine) Evaluate(ctx context.Context, clientUID uint, cfg *model.SeedingClientConfig) (*EvaluateResult, error) {
	return e.evaluate(ctx, clientUID, cfg, false)
}

func (e *Engine) DryRunEvaluate(ctx context.Context, clientUID uint, cfg *model.SeedingClientConfig) (int, error) {
	result, err := e.evaluate(ctx, clientUID, cfg, true)
	if err != nil {
		return 0, err
	}
	return result.Evaluated, nil
}

func (e *Engine) evaluate(ctx context.Context, clientUID uint, cfg *model.SeedingClientConfig, dryRun bool) (*EvaluateResult, error) {
	ec, err := e.prepareEvaluateContext(ctx, clientUID, cfg)
	if err != nil {
		return nil, err
	}

	if len(ec.records) == 0 {
		e.logger.Debug("evaluate: no records to evaluate",
			zap.Uint("client_uid", clientUID))
		return &EvaluateResult{}, nil
	}

	e.logger.Debug("evaluate: starting cycle",
		zap.Uint("client_uid", clientUID),
		zap.Int("records", len(ec.records)),
		zap.Int("torrents_in_downloader", len(ec.torrentMap)),
		zap.Bool("dry_run", dryRun))

	result := &EvaluateResult{}
	cycleID := time.Now().Format("20060102-150405")

	if !dryRun && ec.cfg != nil && ec.cfg.DiskProtectEnabled && ec.cfg.MinDiskSpaceGB > 0 && ec.freeSpace >= 0 {
		minBytes := int64(ec.cfg.MinDiskSpaceGB * 1024 * 1024 * 1024)
		if ec.freeSpace >= minBytes {
			e.recoverDiskProtectPaused(ctx, ec.clientUID)
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
					zap.Uint("client_uid", rec.ClientUID),
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
						e.markRelatedDeleted(ctx, companions, ec.clientUID, "pending_companion_cascade")
					}
				}
				if err := e.db.WithContext(ctx).Model(rec).Updates(map[string]interface{}{
					"status":         model.SeedingStatusDeleted,
					"last_action_by": "pending_timeout",
				}).Error; err != nil {
					e.logger.Warn("pending timeout cleanup: failed to update status", zap.Uint("id", rec.ID), zap.Error(err))
				}
				e.mu.Lock()
				delete(e.recordMap, recordKey(rec.ClientUID, rec.InfoHash))
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
			"client_id": clientUID,
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
		zap.Uint("client_uid", rec.ClientUID),
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
		zap.Uint("client_uid", rec.ClientUID),
		zap.String("info_hash", rec.InfoHash),
		zap.String("site_name", rec.SiteName),
		zap.String("rule_alias", rule.Alias))
	if e.wsBroadcaster != nil {
		e.wsBroadcaster.BroadcastWS("seeding.deleted", map[string]interface{}{
			"info_hash":  rec.InfoHash,
			"site_name":  rec.SiteName,
			"rule_alias": rule.Alias,
			"client_id":  rec.ClientUID,
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
