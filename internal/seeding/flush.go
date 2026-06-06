package seeding

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

type flushContext struct {
	subscriptionID   string
	sub              *model.RSSSubscription
	scoringCfg       model.SeedingScoringConfig
	clientID         string
	client           model.DownloaderClient
	records          []model.SeedingTorrentRecord
	freeSpace        int64
	clientCfg        *model.SeedingClientConfig
	assumeFreeSites  map[string]bool
}

func (e *Engine) buildFlushContext(ctx context.Context, subscriptionID string) (*flushContext, error) {
	subID, err := strconv.ParseUint(subscriptionID, 10, 64)
	if err != nil {
		return nil, nil
	}

	var sub model.RSSSubscription
	if err := e.db.WithContext(ctx).Where("id = ? AND enabled = ?", uint(subID), true).First(&sub).Error; err != nil {
		return nil, nil
	}

	scoringCfg := sub.ScoringConfig
	if !sub.Enabled {
		return nil, nil
	}

	clientID := sub.ClientID
	if clientID == "" {
		return nil, nil
	}

	if e.clientProvider == nil {
		return nil, nil
	}

	dlClient, err := e.clientProvider.Get(clientID)
	if err != nil {
		e.logger.Warn("flush: client not available",
			zap.String("client_id", clientID),
			zap.Error(err))
		return nil, nil
	}

	var cfg model.SeedingClientConfig
	hasConfig := true
	if err := e.db.WithContext(ctx).Where("client_id = ?", clientID).First(&cfg).Error; err != nil {
		hasConfig = false
	}

	if hasConfig && !IsWithinActiveWindow(cfg.ActiveTimeWindows) {
		e.logger.Debug("flush: outside active time windows",
			zap.String("client_id", clientID))
		return nil, nil
	}

	var records []model.SeedingTorrentRecord
	if dbErr := e.db.WithContext(ctx).
		Where("client_id = ? AND status IN ? AND source IN ? AND subscription_id = ?", clientID, []model.SeedingTorrentStatus{model.SeedingStatusPending, model.SeedingStatusSeeding}, []string{"rss", "free_wait"}, subscriptionID).
		Find(&records).Error; dbErr != nil {
		e.logger.Warn("flush: load records failed",
			zap.String("client_id", clientID),
			zap.String("subscription_id", subscriptionID),
			zap.Error(dbErr))
		return nil, nil
	}

	freeSpace := int64(-1)
	if md, mdErr := dlClient.GetMainData(ctx); mdErr == nil && md != nil {
		freeSpace = md.FreeSpace
	}

	if hasConfig && cfg.DiskProtectEnabled && cfg.MinDiskSpaceGB > 0 && freeSpace >= 0 {
		minBytes := int64(cfg.MinDiskSpaceGB * 1024 * 1024 * 1024)
		if freeSpace < minBytes {
			e.logger.Warn("flush: disk protect active, skipping push",
				zap.String("client_id", clientID),
				zap.Int64("freeSpace", freeSpace),
				zap.Float64("minGB", cfg.MinDiskSpaceGB))
			return nil, nil
		}
	}

	assumeFreeSites := make(map[string]bool)
	{
		names := make(map[string]struct{}, len(records))
		for i := range records {
			names[records[i].SiteName] = struct{}{}
		}
		if len(names) > 0 {
			list := make([]string, 0, len(names))
			for n := range names {
				list = append(list, n)
			}
			type siteRow struct {
				Name       string `gorm:"column:name"`
				AssumeFree bool   `gorm:"column:assume_free"`
			}
			var rows []siteRow
			if err := e.db.WithContext(ctx).Table("sites").
				Select("name, assume_free").
				Where("name IN ?", list).
				Find(&rows).Error; err != nil {
				e.logger.Warn("flush: query assume_free failed",
					zap.Error(err))
			}
			for _, r := range rows {
				if r.AssumeFree {
					assumeFreeSites[r.Name] = true
				}
			}
		}
	}

	return &flushContext{
		subscriptionID:  subscriptionID,
		sub:             &sub,
		scoringCfg:      scoringCfg,
		clientID:        clientID,
		client:          dlClient,
		records:         records,
		freeSpace:       freeSpace,
		clientCfg:       &cfg,
		assumeFreeSites: assumeFreeSites,
	}, nil
}

type flushCandidate struct {
	Record    *model.SeedingTorrentRecord
	Score     float64
	TorrentID string
	SiteName  string
	InfoHash  string
}

func (e *Engine) collectCandidates(fc *flushContext) []*flushCandidate {
	scoringCfg := fc.scoringCfg
	maxCandidates := scoringCfg.MaxCandidates
	if maxCandidates <= 0 {
		maxCandidates = 50
	}

	var candidates []*flushCandidate
	now := time.Now()
	for i := range fc.records {
		rec := &fc.records[i]

		if rec.FlushedAt != nil {
			continue
		}

		if rec.IsFree && rec.FreeEndAt != nil && rec.FreeEndAt.Before(now) {
			continue
		}

		discount := rec.Discount
		if discount == "" {
			if rec.IsFree {
				discount = model.DiscountFree
			} else {
				discount = model.DiscountNone
			}
		}

		isFree := discount.IsFree()
		is2xUp := discount == model.Discount2xUp || discount == model.Discount2x50

		assumeFree := fc.assumeFreeSites[rec.SiteName]
		if assumeFree && !isFree {
			rec.IsFree = true
			rec.Discount = model.DiscountAssumeFree
			isFree = true
			if err := e.db.WithContext(context.Background()).Model(&model.SeedingTorrentRecord{}).
				Where("id = ?", rec.ID).
				Updates(map[string]interface{}{
					"discount": model.DiscountAssumeFree,
					"is_free":  true,
				}).Error; err != nil {
				e.logger.Warn("flush: backfill assume_free failed",
					zap.Uint("id", rec.ID),
					zap.String("site", rec.SiteName),
					zap.Error(err))
			}
		}

		if !isFree && (!scoringCfg.Include2xUp || !is2xUp) {
			continue
		}

		candidates = append(candidates, &flushCandidate{
			Record:    rec,
			TorrentID: rec.TorrentID,
			SiteName:  rec.SiteName,
			InfoHash:  rec.InfoHash,
		})

		if len(candidates) >= maxCandidates {
			break
		}
	}

	return candidates
}

func (e *Engine) scoreCandidates(ctx context.Context, candidates []*flushCandidate, scoringCfg model.SeedingScoringConfig) {
	siteWeights := make(map[string]float64)
	if scoringCfg.SiteWeightsJSON != "" {
		if err := json.Unmarshal([]byte(scoringCfg.SiteWeightsJSON), &siteWeights); err != nil {
			e.logger.Warn("parse siteWeightsJSON failed, using empty weights",
				zap.String("json", scoringCfg.SiteWeightsJSON),
				zap.Error(err))
		}
	}

	halfLife := scoringCfg.HalfLifeHours
	if halfLife <= 0 {
		halfLife = 2.0
	}

	slMap := e.fetchBatchSLData(ctx, candidates)

	for _, c := range candidates {
		siteWeight := 1.0
		if w, ok := siteWeights[c.Record.SiteName]; ok && w > 0 {
			siteWeight = w
		}

		ageHours := time.Since(c.Record.CreatedAt).Hours()

		seeders, leechers := 0, 0
		key := c.Record.SiteName + ":" + c.Record.TorrentID
		if sl, ok := slMap[key]; ok && sl != nil {
			seeders = sl.Seeders
			leechers = sl.Leechers
		}

		input := ScoreInput{
			Seeders:       seeders,
			Leechers:      leechers,
			AgeHours:      ageHours,
			Size:          0,
			Discount:      c.Record.Discount,
			HalfLifeHours: halfLife,
			SiteWeight:    siteWeight,
		}

		result := CalculateScore(input)
		c.Score = result.EffectiveScore
	}
}

func (e *Engine) fetchBatchSLData(ctx context.Context, candidates []*flushCandidate) map[string]*model.SLData {
	result := make(map[string]*model.SLData)
	if e.siteProvider == nil {
		return result
	}

	groupedBySite := make(map[string][]string)
	for _, c := range candidates {
		site := c.Record.SiteName
		tid := c.Record.TorrentID
		if site == "" || tid == "" {
			continue
		}
		groupedBySite[site] = append(groupedBySite[site], tid)
	}

	for site, tids := range groupedBySite {
		adapter, err := e.siteProvider.GetAdapter(ctx, site)
		if err != nil {
			e.logger.Debug("scoreCandidates: get adapter failed", zap.String("site", site), zap.Error(err))
			continue
		}
		siteCfg, cfgErr := e.siteProvider.GetSiteConfig(ctx, site)
		if cfgErr != nil {
			e.logger.Debug("scoreCandidates: get site config failed", zap.String("site", site), zap.Error(cfgErr))
			continue
		}
		slData, slErr := adapter.GetBatchSLData(ctx, siteCfg, tids)
		if slErr != nil || slData == nil {
			e.logger.Debug("scoreCandidates: batch SL failed", zap.String("site", site), zap.Error(slErr))
			continue
		}
		for tid, sl := range slData {
			result[site+":"+tid] = sl
		}
	}

	return result
}

func (e *Engine) confirmTopN(ctx context.Context, candidates []*flushCandidate, topN int, scoringCfg model.SeedingScoringConfig) {
	if e.siteProvider == nil || topN <= 0 || len(candidates) == 0 {
		return
	}

	if topN > len(candidates) {
		topN = len(candidates)
	}

	for i := 0; i < topN; i++ {
		c := candidates[i]
		site := c.Record.SiteName
		tid := c.Record.TorrentID
		if site == "" || tid == "" {
			continue
		}

		adapter, err := e.siteProvider.GetAdapter(ctx, site)
		if err != nil {
			continue
		}
		siteCfg, cfgErr := e.siteProvider.GetSiteConfig(ctx, site)
		if cfgErr != nil {
			continue
		}

		sl, slErr := adapter.GetPreciseSLData(ctx, siteCfg, tid)
		if slErr != nil || sl == nil {
			continue
		}

		halfLife := scoringCfg.HalfLifeHours
		if halfLife <= 0 {
			halfLife = 2.0
		}
		input := ScoreInput{
			Seeders:       sl.Seeders,
			Leechers:      sl.Leechers,
			AgeHours:      time.Since(c.Record.CreatedAt).Hours(),
			Discount:      c.Record.Discount,
			HalfLifeHours: halfLife,
			SiteWeight:    1.0,
		}
		c.Score = CalculateScore(input).EffectiveScore
	}
}

func (e *Engine) Flush(ctx context.Context, subscriptionID string) ([]*model.SeedingCandidate, error) {
	fc, err := e.buildFlushContext(ctx, subscriptionID)
	if err != nil {
		return nil, err
	}
	if fc == nil {
		return []*model.SeedingCandidate{}, nil
	}

	activeCount := e.GetActiveCount(fc.clientID)
	maxActive := fc.scoringCfg.MaxActiveSeeding
	if fc.clientCfg != nil && fc.clientCfg.MaxActiveSeeding != 0 {
		maxActive = fc.clientCfg.MaxActiveSeeding
	}
	if maxActive == 0 {
		maxActive = 100
	}

	var remaining int
	if maxActive < 0 {
		remaining = math.MaxInt32 - activeCount
	} else {
		remaining = maxActive - activeCount
	}
	if remaining <= 0 {
		e.logger.Info("flush: max active seeding reached, checking disk_recover only",
			zap.String("client_id", fc.clientID),
			zap.Int("active", activeCount),
			zap.Int("max", maxActive))

		var diskRecoverCandidates []*flushCandidate
		for i := range fc.records {
			rec := &fc.records[i]
			if rec.FlushedAt == nil && rec.LastActionBy == "disk_recover" {
				diskRecoverCandidates = append(diskRecoverCandidates, &flushCandidate{
					Record:    rec,
					TorrentID: rec.TorrentID,
					SiteName:  rec.SiteName,
					InfoHash:  rec.InfoHash,
					Score:     1.0,
				})
			}
		}
		if len(diskRecoverCandidates) == 0 {
			return []*model.SeedingCandidate{}, nil
		}

		var results []*model.SeedingCandidate
		for _, c := range diskRecoverCandidates {
			if ctx.Err() != nil {
				break
			}
			candidate, pushed := e.pushOne(ctx, fc, c)
			results = append(results, candidate)
			if pushed {
				e.logger.Info("flush: disk_recover pushed despite maxActiveSeeding",
					zap.String("client_id", fc.clientID),
					zap.String("info_hash", c.InfoHash))
			}
		}
		return results, nil
	}

	candidates := e.collectCandidates(fc)
	if len(candidates) == 0 {
		return []*model.SeedingCandidate{}, nil
	}

	e.scoreCandidates(ctx, candidates, fc.scoringCfg)

	if !fc.scoringCfg.Enabled {
		for _, c := range candidates {
			c.Score = 1.0
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	topN := fc.scoringCfg.TopNConfirm
	if topN > 0 && fc.scoringCfg.Enabled {
		e.confirmTopN(ctx, candidates, topN, fc.scoringCfg)

		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].Score > candidates[j].Score
		})
	}

	minScore := fc.scoringCfg.MinScore
	if !fc.scoringCfg.Enabled {
		minScore = 0
	}

	var qualified []*flushCandidate
	for _, c := range candidates {
		if c.Score < minScore {
			break
		}
		qualified = append(qualified, c)
	}

	if len(qualified) == 0 && len(candidates) > 0 {
		for _, c := range candidates {
			if c.Record.LastActionBy == "disk_recover" {
				c.Score = 1.0
				qualified = append(qualified, c)
			}
		}
	}

	if len(qualified) > remaining {
		qualified = qualified[:remaining]
	}

	batchLimit := fc.scoringCfg.BatchLimit
	if batchLimit <= 0 {
		batchLimit = 10
	}
	if len(qualified) > batchLimit {
		qualified = qualified[:batchLimit]
	}

	var results []*model.SeedingCandidate
	var batchPendingBytes int64
	for _, c := range qualified {
		if ctx.Err() != nil {
			break
		}

		if fc.clientCfg != nil && fc.clientCfg.DiskProtectEnabled && fc.clientCfg.MinDiskSpaceGB > 0 {
			if md, mdErr := fc.client.GetMainData(ctx); mdErr == nil && md != nil {
				minBytes := int64(fc.clientCfg.MinDiskSpaceGB * 1024 * 1024 * 1024)
				inflightBytes := md.InflightBytes + batchPendingBytes
				effectiveFree := md.FreeSpace - inflightBytes
				if effectiveFree < minBytes {
					e.logger.Warn("flush: disk protect triggered during batch, stopping",
						zap.String("client_id", fc.clientID),
						zap.Int64("freeSpace", md.FreeSpace),
						zap.Int64("inflightBytes", inflightBytes),
						zap.Int64("batchPendingBytes", batchPendingBytes),
						zap.Int64("effectiveFree", effectiveFree),
						zap.Float64("minGB", fc.clientCfg.MinDiskSpaceGB),
						zap.Int("pushed", len(results)))
					break
				}
				torrentSize := c.Record.TorrentSize
				if torrentSize > 0 && effectiveFree-torrentSize < minBytes {
					e.logger.Info("flush: disk budget insufficient for torrent, skipping",
						zap.String("client_id", fc.clientID),
						zap.Int64("freeSpace", md.FreeSpace),
						zap.Int64("inflightBytes", inflightBytes),
						zap.Int64("effectiveFree", effectiveFree),
						zap.Int64("torrentSize", torrentSize),
						zap.Float64("minGB", fc.clientCfg.MinDiskSpaceGB),
						zap.String("torrent_id", c.TorrentID))
					continue
				}
			}
		}

		candidate, pushed := e.pushOne(ctx, fc, c)
		results = append(results, candidate)
		if pushed {
			batchPendingBytes += c.Record.TorrentSize
			e.logger.Info("flush: pushed seeding torrent",
				zap.String("client_id", fc.clientID),
				zap.String("site", c.SiteName),
				zap.String("torrent_id", c.TorrentID),
				zap.String("info_hash", c.InfoHash),
				zap.Float64("score", c.Score))
		}
	}

	return results, nil
}

func (e *Engine) pushOne(ctx context.Context, fc *flushContext, c *flushCandidate) (*model.SeedingCandidate, bool) {
	rec := c.Record
	candidate := &model.SeedingCandidate{
		SubscriptionID: fc.subscriptionID,
		ClientID:       fc.clientID,
		CollectedAt:    time.Now(),
	}

	pushSub := fc.sub
	if rec.SubscriptionID != "" && rec.SubscriptionID != fc.subscriptionID {
		var altSub model.RSSSubscription
		if err := e.db.WithContext(ctx).Where("id = ?", rec.SubscriptionID).First(&altSub).Error; err == nil {
			pushSub = &altSub
		}
	}

	hrStrategy := "protect"
	if e.siteProvider != nil && rec.SiteName != "" {
		siteCfgCtx, siteCfgCancel := context.WithTimeout(ctx, 5*time.Second)
		if siteCfg, err := e.siteProvider.GetSiteConfig(siteCfgCtx, rec.SiteName); err == nil && siteCfg != nil {
			if siteCfg.HRStrategy == "skip" || siteCfg.HRStrategy == "ignore" {
				hrStrategy = siteCfg.HRStrategy
			}
		}
		siteCfgCancel()
	}
	if rec.HasHR && hrStrategy == "protect" {
		e.logger.Debug("flush: skipping HR torrent",
			zap.String("torrent_id", rec.TorrentID),
			zap.String("site", rec.SiteName))
		return candidate, false
	}

	if rec.IsFree && rec.FreeEndAt == nil && rec.Discount != model.DiscountAssumeFree && e.siteProvider != nil && rec.SiteName != "" {
		if adapter, adpErr := e.siteProvider.GetAdapter(ctx, rec.SiteName); adpErr == nil {
			if siteCfg, cfgErr := e.siteProvider.GetSiteConfig(ctx, rec.SiteName); cfgErr == nil && siteCfg != nil {
				recheckCtx, recheckCancel := context.WithTimeout(ctx, 10*time.Second)
				discResult, discErr := adapter.DetectDiscount(recheckCtx, siteCfg, rec.TorrentID)
				recheckCancel()
				if discErr != nil {
					e.logger.Warn("flush: discount recheck failed, treating as non-free for safety",
						zap.String("torrent_id", rec.TorrentID),
						zap.String("site", rec.SiteName),
						zap.Error(discErr))
					return candidate, false
				}
				if discResult == nil || !discResult.Level.IsFree() {
					e.logger.Info("flush: free status expired before push, skipping",
						zap.String("torrent_id", rec.TorrentID),
						zap.String("site", rec.SiteName))
					return candidate, false
				}
			}
		}
	}

	checkCtx, checkCancel := context.WithTimeout(ctx, 10*time.Second)
	exists, err := fc.client.CheckExists(checkCtx, rec.InfoHash)
	checkCancel()
	if err == nil && exists {
		if rec.LastActionBy == "disk_recover" && rec.Status == model.SeedingStatusPending {
			e.logger.Info("flush: disk_recover torrent already in client, restoring to seeding",
				zap.String("info_hash", rec.InfoHash))
			if pushSub.Category != "" {
				if catErr := fc.client.SetCategory(ctx, rec.InfoHash, pushSub.Category); catErr != nil {
					e.logger.Warn("flush: disk_recover set category failed",
						zap.String("info_hash", rec.InfoHash),
						zap.Error(catErr))
				}
			}
			if len(pushSub.Tags) > 0 {
				if tagErr := fc.client.SetTorrentTags(ctx, rec.InfoHash, pushSub.Tags); tagErr != nil {
					e.logger.Warn("flush: disk_recover set tags failed",
						zap.String("info_hash", rec.InfoHash),
						zap.Error(tagErr))
				}
			}
			now := time.Now()
			if dbErr := e.db.WithContext(ctx).Model(rec).Updates(map[string]interface{}{
				"status":         model.SeedingStatusSeeding,
				"last_action_by": "disk_recover_restored",
				"flushed_at":     now,
				"updated_at":     now,
			}).Error; dbErr != nil {
				e.logger.Error("flush: disk_recover DB update failed",
					zap.String("info_hash", rec.InfoHash),
					zap.Error(dbErr))
			}
			rec.Status = model.SeedingStatusSeeding
			rec.FlushedAt = &now
			e.mu.Lock()
			key := recordKey(rec.ClientID, rec.InfoHash)
			if r, ok := e.recordMap[key]; ok {
				r.Status = model.SeedingStatusSeeding
				r.LastActionBy = "disk_recover_restored"
				r.FlushedAt = &now
			}
			e.mu.Unlock()
		} else {
			e.logger.Debug("flush: torrent already exists in client",
				zap.String("info_hash", rec.InfoHash))
		}
		return candidate, false
	}

	if pushSub.SkipSameSize && rec.InfoHash != "" {
		if e.skipBySize(ctx, fc.client, rec) {
			e.logger.Debug("flush: skipped by same size",
				zap.String("torrent_id", rec.TorrentID))
			return candidate, false
		}
	}

	if fc.freeSpace >= 0 && pushSub.DiskBudgetEnabled {
		minBytes := int64(pushSub.DiskBudgetMinGB * 1024 * 1024 * 1024)
		if fc.freeSpace < minBytes {
			e.logger.Warn("flush: disk space insufficient",
				zap.Int64("free", fc.freeSpace),
				zap.Float64("min_gb", pushSub.DiskBudgetMinGB))
			return candidate, false
		}
	}

	if rec.IsFree && rec.FreeEndAt != nil && rec.FreeEndAt.Before(time.Now()) {
		e.logger.Debug("flush: skipping torrent with expired free period",
			zap.String("torrent_id", rec.TorrentID),
			zap.String("site", rec.SiteName),
			zap.Time("free_end_at", *rec.FreeEndAt))
		return candidate, false
	}

	dlCtx, dlCancel := context.WithTimeout(ctx, 30*time.Second)
	torrentData, err := e.downloadTorrent(dlCtx, rec.SiteName, rec.TorrentID)
	dlCancel()
	if err != nil || len(torrentData) == 0 {
		e.logger.Warn("flush: download torrent failed",
			zap.String("site", rec.SiteName),
			zap.String("torrent_id", rec.TorrentID),
			zap.Error(err))
		return candidate, false
	}

	opts := model.AddTorrentOptions{
		SavePath: pushSub.SavePath,
		Category: pushSub.Category,
		Tags:     pushSub.Tags,
		Paused:   pushSub.AddPaused,
		AutoTMM:  pushSub.AutoTMM,
	}
	if pushSub.UploadLimitKB > 0 {
		opts.UploadLimit = pushSub.UploadLimitKB * 1024
	}
	if pushSub.DownloadLimitKB > 0 {
		opts.DownloadLimit = pushSub.DownloadLimitKB * 1024
	}

	addResult, err := fc.client.AddFromFile(ctx, torrentData, opts)
	if err != nil {
		e.logger.Warn("flush: add from file failed",
			zap.String("torrent_id", rec.TorrentID),
			zap.Error(err))
		return candidate, false
	}

	e.mu.Lock()
	key := recordKey(fc.clientID, rec.InfoHash)
	if _, ok := e.recordMap[key]; ok {
		e.mu.Unlock()
		if addResult != nil && addResult.InfoHash != "" && addResult.InfoHash != rec.InfoHash {
			e.mu.Lock()
			delete(e.recordMap, key)
			e.mu.Unlock()
			if dbErr := e.db.WithContext(ctx).Model(rec).Updates(map[string]interface{}{
				"info_hash": addResult.InfoHash,
				"status":    model.SeedingStatusSeeding,
			}).Error; dbErr != nil {
				e.logger.Warn("flush: update info_hash after mismatch failed",
					zap.String("torrent_id", rec.TorrentID),
					zap.Error(dbErr))
			}
			if dbErr := e.db.WithContext(ctx).Model(&model.RSSTorrentSeen{}).
				Where("site_name = ? AND torrent_id = ?", rec.SiteName, rec.TorrentID).
				Update("info_hash", addResult.InfoHash).Error; dbErr != nil {
				e.logger.Warn("flush: backfill rss_torrent_seen info_hash failed",
					zap.String("torrent_id", rec.TorrentID),
					zap.Error(dbErr))
			}
			realKey := recordKey(fc.clientID, addResult.InfoHash)
			rec.InfoHash = addResult.InfoHash
			e.mu.Lock()
			e.recordMap[realKey] = rec
			e.mu.Unlock()
		}
	} else if addResult != nil && addResult.InfoHash != "" && addResult.InfoHash != rec.InfoHash {
		altKey := recordKey(fc.clientID, addResult.InfoHash)
		if _, altOk := e.recordMap[altKey]; altOk {
			e.mu.Unlock()
		} else {
			e.mu.Unlock()
			newRecord := &model.SeedingTorrentRecord{
				ClientID:       fc.clientID,
				SiteName:       rec.SiteName,
				TorrentID:      rec.TorrentID,
				InfoHash:       addResult.InfoHash,
				Discount:       rec.Discount,
				HasHR:          rec.HasHR,
				HRSeedTimeH:    rec.HRSeedTimeH,
				IsFree:         rec.Discount.IsFree(),
				FreeLevel:      string(rec.Discount),
				FreeEndAt:      rec.FreeEndAt,
				Source:         "rss",
				Status:         model.SeedingStatusSeeding,
				SubscriptionID: rec.SubscriptionID,
			}
			if err := e.db.WithContext(ctx).Create(newRecord).Error; err != nil {
				e.logger.Warn("flush: create seeding record failed, rolling back torrent from downloader",
					zap.String("torrent_id", rec.TorrentID),
					zap.Error(err))
				if delErr := fc.client.DeleteTorrent(ctx, addResult.InfoHash, true); delErr != nil {
					e.logger.Error("flush: rollback delete failed, phantom torrent may exist",
						zap.String("info_hash", addResult.InfoHash),
						zap.Error(delErr))
				}
				return candidate, false
			}
			if dbErr := e.db.WithContext(ctx).Model(&model.RSSTorrentSeen{}).
				Where("site_name = ? AND torrent_id = ?", rec.SiteName, rec.TorrentID).
				Update("info_hash", addResult.InfoHash).Error; dbErr != nil {
				e.logger.Warn("flush: backfill rss_torrent_seen info_hash failed",
					zap.String("torrent_id", rec.TorrentID),
					zap.Error(dbErr))
			}
			e.mu.Lock()
			e.recordMap[altKey] = newRecord
			e.mu.Unlock()

			if e.freeEndMonitor != nil && newRecord.FreeEndAt != nil {
				e.freeEndMonitor.Schedule(newRecord)
			}
		}
	} else {
		e.mu.Unlock()
	}

	now := time.Now()
	if dbErr := e.db.WithContext(ctx).Model(rec).Updates(map[string]interface{}{
		"flushed_at": now,
		"status":     model.SeedingStatusSeeding,
	}).Error; dbErr != nil {
		e.logger.Error("flush: failed to update record status after push",
			zap.String("info_hash", rec.InfoHash),
			zap.Error(dbErr))
	}
	rec.FlushedAt = &now
	rec.Status = model.SeedingStatusSeeding

	if e.reseedTrigger != nil && pushSub.AutoReseed && len(pushSub.ReseedClientIDs) > 0 {
		recordCopy := *rec
		clientIDs := make([]string, len(pushSub.ReseedClientIDs))
		copy(clientIDs, pushSub.ReseedClientIDs)
		go e.reseedTrigger.OnTorrentSeeding(context.Background(), recordCopy, clientIDs)
	}

	return candidate, true
}

func (e *Engine) downloadTorrent(ctx context.Context, siteName, torrentID string) ([]byte, error) {
	siteCfg, err := e.siteProvider.GetSiteConfig(ctx, siteName)
	if err != nil {
		return nil, fmt.Errorf("get site config: %w", err)
	}

	adapter, err := e.siteProvider.GetAdapter(ctx, siteName)
	if err != nil {
		return nil, fmt.Errorf("get adapter: %w", err)
	}

	return adapter.DownloadTorrent(ctx, siteCfg, torrentID)
}

func (e *Engine) skipBySize(ctx context.Context, client model.DownloaderClient, rec *model.SeedingTorrentRecord) bool {
	torrents, err := client.GetSeedingTorrents(ctx)
	if err != nil {
		return false
	}

	var recSize int64
	for _, t := range torrents {
		if t.Hash == rec.InfoHash {
			recSize = t.TotalSize
			break
		}
	}
	if recSize <= 0 {
		return false
	}

	for _, t := range torrents {
		if t.Hash == rec.InfoHash {
			continue
		}
		if t.TotalSize == recSize && t.SavePath != "" {
			return true
		}
	}
	return false
}
