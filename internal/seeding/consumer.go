package seeding

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/pusher"
	"go.uber.org/zap"
	"gorm.io/gorm/clause"
)

type pendingCandidate struct {
	Event     *pusher.PushedEvent
	Score     float64
	CreatedAt time.Time
}

func (e *Engine) SetPusher(p *pusher.Pusher) {
	e.pusher = p
}

// LoadActiveClientConfig 按 clientID 加载启用的客户端配置（§55.15 补全 §55.14 阶段3 遗漏）。
// 先查 seeding_client_configs；未命中再查 download_client_configs 并经 downloadConfigToSeeding
// 适配为 SeedingClientConfig 视图。统一 role=seeding 与 role≠seeding 下载器的配置加载入口，
// 避免调用方各自内联查单表导致 role≠seeding 种子被静默丢弃。
func (e *Engine) LoadActiveClientConfig(ctx context.Context, clientID string) (model.SeedingClientConfig, bool) {
	var cfg model.SeedingClientConfig
	if err := e.db.WithContext(ctx).Where("client_id = ? AND enabled = ?", clientID, true).First(&cfg).Error; err == nil {
		return cfg, true
	}
	var dlCfg model.DownloadClientConfig
	if err := e.db.WithContext(ctx).Where("client_id = ? AND enabled = ?", clientID, true).First(&dlCfg).Error; err == nil {
		return *downloadConfigToSeeding(&dlCfg), true
	}
	return model.SeedingClientConfig{}, false
}

func (e *Engine) OnPushed(ctx context.Context, event *pusher.PushedEvent) {
	if event == nil {
		return
	}

	// §55.14 阶段3：查 seeding_client_configs 或 download_client_configs（统一管理 role=seeding 和 role≠seeding）
	if _, ok := e.LoadActiveClientConfig(ctx, event.ClientID); !ok {
		return
	}

	select {
	case e.pendingEvents <- event:
	default:
		e.logger.Warn("pending events channel full, dropping event",
			zap.String("client_id", event.ClientID),
			zap.String("info_hash", event.InfoHash))
	}
}

func (e *Engine) consumeLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	batch := make([]*pusher.PushedEvent, 0, 100)

	for {
		select {
		case <-ctx.Done():
			if len(batch) > 0 {
				e.scoreAndPush(ctx, batch)
			}
			return
		case event := <-e.pendingEvents:
			batch = append(batch, event)
		case <-ticker.C:
			if len(batch) > 0 {
				e.scoreAndPush(ctx, batch)
				batch = batch[:0]
			}
		}
	}
}

func (e *Engine) scoreAndPush(ctx context.Context, events []*pusher.PushedEvent) {
	type groupKey struct {
		clientID       string
		subscriptionID string
	}
	groups := make(map[groupKey][]*pendingCandidate)
	for _, ev := range events {
		key := groupKey{clientID: ev.ClientID, subscriptionID: ev.SubscriptionID}
		groups[key] = append(groups[key], &pendingCandidate{
			Event:     ev,
			CreatedAt: ev.PushedAt,
		})
	}

	for key, candidates := range groups {
		e.scoreAndPushForClient(ctx, key.clientID, key.subscriptionID, candidates)
	}
}

func (e *Engine) scoreAndPushForClient(ctx context.Context, clientID, subscriptionID string, candidates []*pendingCandidate) {
	clientCfg, ok := e.LoadActiveClientConfig(ctx, clientID)
	if !ok {
		return
	}

	if !IsWithinActiveWindow(clientCfg.ActiveTimeWindows) {
		e.logger.Debug("scoreAndPush: outside active time windows",
			zap.String("client_id", clientID))
		return
	}

	activeCount := e.GetActiveCount(clientID)
	maxActive := 100
	if clientCfg.MaxActiveSeeding != 0 {
		maxActive = clientCfg.MaxActiveSeeding
	}
	if maxActive < 0 {
		maxActive = 999999
	}
	remaining := maxActive - activeCount
	if remaining <= 0 {
		e.logger.Info("scoreAndPush: max active reached, skipping",
			zap.String("client_id", clientID),
			zap.Int("active", activeCount),
			zap.Int("max", maxActive))
		return
	}

	if clientCfg.DiskProtectEnabled && e.clientProvider != nil {
		dlClient, err := e.clientProvider.Get(clientID)
		if err == nil && dlClient != nil {
			freeSpace, _ := dlClient.GetFreeSpace(ctx)
			totalSpace := int64(0)
			if md, mdErr := dlClient.GetMainData(ctx); mdErr == nil && md != nil {
				totalSpace = md.TotalDiskSpace
			}
			minBytes := calcDiskMinBytes(&clientCfg, totalSpace)
			if minBytes > 0 && freeSpace >= 0 && freeSpace < minBytes {
				e.logger.Warn("scoreAndPush: disk space insufficient, pausing push",
					zap.String("client_id", clientID),
					zap.Int64("free_space", freeSpace),
					zap.Int64("min_bytes", minBytes),
					zap.Float64("min_gb", clientCfg.MinDiskSpaceGB),
					zap.Float64("emergency_buffer", clientCfg.EmergencyBuffer))
				return
			}
		}
	}

	// §55.14 阶段3：按下载器 role 决定是否评分（role≠seeding 顺序推送，不挑种子）
	var roleClient model.ClientConfig
	isSeedingRole := true
	if err := e.db.WithContext(ctx).Where("name = ?", clientID).First(&roleClient).Error; err == nil {
		isSeedingRole = roleClient.Role == "seeding"
	}

	scoringCfg := model.SeedingScoringConfig{Enabled: false}
	if isSeedingRole {
		scoringCfg = e.loadScoringConfig(ctx, subscriptionID)
		siteWeights := e.parseSiteWeights(scoringCfg.SiteWeightsJSON)
		e.scoreCandidatesFull(ctx, candidates, scoringCfg, siteWeights)
	} else {
		// role≠seeding：不评分，Score=1.0 顺序推送（普通下载不挑种子）
		for _, c := range candidates {
			c.Score = 1.0
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	topN := remaining
	if topN > len(candidates) {
		topN = len(candidates)
	}

	pushed := 0
	for i := 0; i < topN && pushed < remaining; i++ {
		c := candidates[i]

		if scoringCfg.Enabled && c.Score < scoringCfg.MinScore {
			// §55.19: Debug 改 Info——评分跳过是种子命运决定点，
			// 必须在生产 Info 级可见，否则同类 bug 会再次因日志缺失而漏诊。
			e.logger.Info("scoreAndPush: candidate below min score, skipping",
				zap.String("info_hash", c.Event.InfoHash),
				zap.Float64("score", c.Score),
				zap.Float64("min_score", scoringCfg.MinScore))
			continue
		}

		key := recordKey(clientID, c.Event.InfoHash)
		e.mu.RLock()
		_, exists := e.recordMap[key]
		e.mu.RUnlock()
		if exists {
			continue
		}

		req := e.buildPushRequest(ctx, clientID, c.Event)
		if req == nil {
			continue
		}

		result := e.pusher.Push(ctx, req)
		if !result.Success {
			if result.Error != nil {
				e.logger.Warn("scoreAndPush: push failed",
					zap.String("client_id", clientID),
					zap.String("info_hash", c.Event.InfoHash),
					zap.Float64("score", c.Score),
					zap.Error(result.Error))
			}
			continue
		}

		e.createRecordFromPush(ctx, clientID, c.Event, result.InfoHash)
		pushed++
	}

	e.logger.Info("scoreAndPush: batch complete",
		zap.String("client_id", clientID),
		zap.Int("candidates", len(candidates)),
		zap.Int("pushed", pushed),
		zap.Int("remaining_capacity", remaining-pushed))
}

func (e *Engine) scoreCandidatesFull(ctx context.Context, candidates []*pendingCandidate, scoringCfg model.SeedingScoringConfig, siteWeights map[string]float64) {
	if !scoringCfg.Enabled {
		for _, c := range candidates {
			c.Score = 1.0
		}
		return
	}

	// §55.19: 总是抓 SL 数据（不再按 IsFree/Discount 跳过）。
	// 评分公式 CalculateScore 必须靠 leechers/seeders 算 demandScore，
	// 免费种（刷流主力）也有 SL 数据，跳过会导致 score=0 < min_score 全过滤。
	// 与 flush.go 的 fetchBatchSLData 行为一致。
	needSLData := len(candidates) > 0 && e.siteProvider != nil

	var slDataMap map[string]*model.SLData
	if needSLData {
		siteGroups := make(map[string][]string)
		for _, c := range candidates {
			siteGroups[c.Event.SiteName] = append(siteGroups[c.Event.SiteName], c.Event.TorrentID)
		}
		slDataMap = make(map[string]*model.SLData)
		for site, ids := range siteGroups {
			adapter, err := e.siteProvider.GetAdapter(ctx, site)
			if err != nil || adapter == nil {
				continue
			}
			siteCfg, cfgErr := e.siteProvider.GetSiteConfig(ctx, site)
			if cfgErr != nil || siteCfg == nil {
				continue
			}
			slCtx, slCancel := context.WithTimeout(ctx, 15*time.Second)
			batch, slErr := adapter.GetBatchSLData(slCtx, siteCfg, ids)
			slCancel()
			if slErr == nil && batch != nil {
				for id, sl := range batch {
					if sl != nil {
						slDataMap[site+"|"+id] = sl
					}
				}
			}
		}
	}

	now := time.Now()
	for _, c := range candidates {
		ev := c.Event
		// §55.19 根本修复：优先用 event 自带 SL（detect 阶段顺便提取），
		// 缺失（=0）才 fallback 到 GetBatchSLData 二次抓取的 slDataMap。
		seeders, leechers := ev.Seeders, ev.Leechers
		if seeders == 0 && leechers == 0 {
			if sl, ok := slDataMap[ev.SiteName+"|"+ev.TorrentID]; ok && sl != nil {
				seeders = sl.Seeders
				leechers = sl.Leechers
			}
		}

		ageHours := 0.0
		if !ev.PushedAt.IsZero() {
			ageHours = now.Sub(ev.PushedAt).Hours()
		}

		discount := ev.Discount
		if discount == "" {
			discount = model.DiscountNone
		}

		siteWeight := 1.0
		if w, ok := siteWeights[ev.SiteName]; ok && w > 0 {
			siteWeight = w
		}

		input := ScoreInput{
			Seeders:       seeders,
			Leechers:      leechers,
			AgeHours:      ageHours,
			Size:          ev.Size,
			Discount:      discount,
			HalfLifeHours: scoringCfg.HalfLifeHours,
			SiteWeight:    siteWeight,
		}

		result := CalculateScore(input)
		c.Score = result.EffectiveScore
	}
}

func (e *Engine) loadScoringConfig(ctx context.Context, subscriptionID string) model.SeedingScoringConfig {
	if subscriptionID == "" {
		return model.SeedingScoringConfig{Enabled: false, MaxActiveSeeding: 0}
	}
	var sub model.RSSSubscription
	if err := e.db.WithContext(ctx).Where("id = ?", subscriptionID).First(&sub).Error; err != nil {
		return model.SeedingScoringConfig{Enabled: false}
	}
	// §55.10 评分配置是 RSSSubscription 的 embedded 字段（gorm:"embedded"），
	// 平铺存储于 rss_subscriptions 表（scoring_enabled/half_life_hours 等），
	// 不是独立表。直接返回 sub.ScoringConfig，避免查询不存在的 seeding_scoring_configs 表。
	return sub.ScoringConfig
}

func (e *Engine) parseSiteWeights(jsonStr string) map[string]float64 {
	weights := make(map[string]float64)
	if jsonStr == "" {
		return weights
	}
	if err := json.Unmarshal([]byte(jsonStr), &weights); err != nil {
		return weights
	}
	return weights
}

func (e *Engine) buildPushRequest(ctx context.Context, clientID string, event *pusher.PushedEvent) *pusher.PushRequest {
	req := &pusher.PushRequest{
		ClientID:    clientID,
		SiteName:    event.SiteName,
		TorrentID:   event.TorrentID,
		InfoHash:    event.InfoHash,
		Title:       event.Title,
		HasHR:       event.HasHR,
		Discount:    event.Discount,
		IsFree:      event.IsFree,
		FreeEndAt:   event.FreeEndAt,
	}

	if event.SubscriptionID != "" {
		var sub model.RSSSubscription
		if err := e.db.WithContext(ctx).Where("id = ?", event.SubscriptionID).First(&sub).Error; err == nil {
			req.SavePath = sub.SavePath
			req.Category = sub.Category
			if len(sub.Tags) > 0 {
				tagsStr := ""
				for i, t := range sub.Tags {
					if i > 0 {
						tagsStr += ","
					}
					tagsStr += t
				}
				req.Tags = tagsStr
			}
		req.AddPaused = sub.AddPaused
		// 智能默认（§55.18）：配了 SavePath 强制 AutoTMM=false（保证 savepath 生效，
		// 避免 qB 全局 TMM=true 覆盖 savepath）；没配 SavePath 保持用户配置（默认 false）。
		if sub.SavePath != "" {
			req.AutoTMM = false
		} else {
			req.AutoTMM = sub.AutoTMM
		}
		req.UploadLimitKB = sub.UploadLimitKB
			req.DownloadLimitKB = sub.DownloadLimitKB
		}
	}

	return req
}

func (e *Engine) createRecordFromPush(ctx context.Context, clientID string, event *pusher.PushedEvent, actualHash string) {
	infoHash := actualHash
	if infoHash == "" {
		infoHash = event.InfoHash
	}

	key := recordKey(clientID, infoHash)
	e.mu.RLock()
	_, exists := e.recordMap[key]
	e.mu.RUnlock()
	if exists {
		return
	}

	// §55.14 阶段2：查下载器 role 存入 record（consumeLoop 据此决定是否评分）
	var dlClient model.ClientConfig
	clientRole := ""
	if err := e.db.WithContext(ctx).Where("name = ?", clientID).First(&dlClient).Error; err == nil {
		clientRole = dlClient.Role
	}

	record := &model.SeedingTorrentRecord{
		ClientID:          clientID,
		SiteName:          event.SiteName,
		TorrentID:         event.TorrentID,
		InfoHash:          infoHash,
		HasHR:             event.HasHR,
		Source:            "rss",
		Status:            model.SeedingStatusSeeding,
		IsFree:            event.IsFree,
		FreeEndAt:         event.FreeEndAt,
		TorrentSize:       event.Size,
		SubscriptionID:    event.SubscriptionID,
		AutoTransfer:      event.AutoTransfer,
		TransferClientIDs: event.TransferClientIDs,
		Role:              clientRole,
	}
	if event.Discount != "" {
		record.Discount = event.Discount
	}

	now := time.Now()
	record.FlushedAt = &now

	result := e.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "client_id"}, {Name: "info_hash"}},
		DoNothing: true,
	}).Create(record)
	if result.Error != nil {
		e.logger.Warn("createRecordFromPush: create failed",
			zap.String("info_hash", infoHash),
			zap.Error(result.Error))
		return
	}

	if result.RowsAffected == 0 {
		var loaded model.SeedingTorrentRecord
		if err := e.db.WithContext(ctx).
			Where("client_id = ? AND info_hash = ?", clientID, infoHash).
			First(&loaded).Error; err != nil {
			return
		}
		if loaded.Status == model.SeedingStatusDeleted {
			e.db.WithContext(ctx).Model(&loaded).Updates(map[string]interface{}{
				"status":         model.SeedingStatusSeeding,
				"flushed_at":     now,
				"last_action_by": "",
				"updated_at":     now,
			})
			loaded.Status = model.SeedingStatusSeeding
			loaded.FlushedAt = &now
			record = &loaded
		} else {
			record = &loaded
		}
	}

	e.mu.Lock()
	e.recordMap[key] = record
	e.mu.Unlock()

	if e.freeEndMonitor != nil && record.FreeEndAt != nil {
		e.freeEndMonitor.Schedule(record)
	}

	e.logger.Debug("createRecordFromPush: record created",
		zap.String("client_id", clientID),
		zap.String("info_hash", infoHash))

	// §55.5 P1 修复：推送成功后回写 rss_torrent_seen.status="pushed"，避免下轮 RSS 重复投递。
	// 仅在 pusher.Push 成功（即调用 createRecordFromPush）后执行；推送失败保持 seen，下轮重试。
	// §55.19 补丁：同时回写 push_time（之前只写 status，导致诊断时看不到推送时间）。
	if err := e.db.WithContext(ctx).Model(&model.RSSTorrentSeen{}).
		Where("site_name = ? AND torrent_id = ?", event.SiteName, event.TorrentID).
		Updates(map[string]interface{}{
			"status":    "pushed",
			"push_time": now,
		}).Error; err != nil {
		e.logger.Debug("mark rss seen as pushed failed",
			zap.String("site", event.SiteName),
			zap.String("torrent_id", event.TorrentID),
			zap.Error(err))
	}
}

var _ = fmt.Sprintf
