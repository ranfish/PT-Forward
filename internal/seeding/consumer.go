package seeding

import (
	"context"
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

func (e *Engine) OnPushed(ctx context.Context, event *pusher.PushedEvent) {
	if event == nil {
		return
	}

	var cfg model.SeedingClientConfig
	hasCfg := false
	if err := e.db.WithContext(ctx).Where("client_id = ? AND enabled = ?", event.ClientID, true).First(&cfg).Error; err == nil {
		hasCfg = true
	}
	if !hasCfg {
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
	bySub := make(map[string][]*pendingCandidate)
	for _, ev := range events {
		subID := ""
		if ev.TorrentID != "" {
			var sub model.RSSSubscription
			if err := e.db.WithContext(ctx).
				Joins("JOIN rss_torrent_seen ON rss_torrent_seen.subscription_id = CAST(rss_subscriptions.id AS TEXT)").
				Where("rss_torrent_seen.site_name = ? AND rss_torrent_seen.torrent_id = ?", ev.SiteName, ev.TorrentID).
				First(&sub).Error; err == nil {
				subID = sub.ClientID
			}
		}
		key := ev.ClientID
		if subID != "" {
			key = subID
		}
		bySub[key] = append(bySub[key], &pendingCandidate{
			Event:     ev,
			CreatedAt: ev.PushedAt,
		})
	}

	for clientID, candidates := range bySub {
		e.scoreAndPushForClient(ctx, clientID, candidates)
	}
}

func (e *Engine) scoreAndPushForClient(ctx context.Context, clientID string, candidates []*pendingCandidate) {
	var clientCfg model.SeedingClientConfig
	if err := e.db.WithContext(ctx).Where("client_id = ? AND enabled = ?", clientID, true).First(&clientCfg).Error; err != nil {
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

	for _, c := range candidates {
		c.Score = e.calculateEventScore(ctx, c.Event)
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	topN := remaining
	if topN > len(candidates) {
		topN = len(candidates)
	}

	for i := 0; i < topN; i++ {
		c := candidates[i]
		if e.pusher == nil {
			e.logger.Warn("scoreAndPush: pusher not configured")
			return
		}

		key := recordKey(clientID, c.Event.InfoHash)
		e.mu.RLock()
		_, exists := e.recordMap[key]
		e.mu.RUnlock()
		if exists {
			continue
		}

		req := &pusher.PushRequest{
			ClientID:    clientID,
			SiteName:    c.Event.SiteName,
			TorrentID:   c.Event.TorrentID,
			InfoHash:    c.Event.InfoHash,
			Title:       c.Event.Title,
			HasHR:       c.Event.HasHR,
			Discount:    c.Event.Discount,
			IsFree:      c.Event.IsFree,
			FreeEndAt:   c.Event.FreeEndAt,
		}

		result := e.pusher.Push(ctx, req)
		if !result.Success {
			if result.Error != nil {
				e.logger.Warn("scoreAndPush: push failed",
					zap.String("client_id", clientID),
					zap.String("info_hash", c.Event.InfoHash),
					zap.Error(result.Error))
			}
			continue
		}

		e.createRecordFromPush(ctx, clientID, c.Event, result.InfoHash)
	}
}

func (e *Engine) calculateEventScore(ctx context.Context, event *pusher.PushedEvent) float64 {
	score := 1.0

	if event.IsFree {
		score += 50
	}
	if event.HasHR {
		score -= 30
	}

	if event.Size > 0 {
		if event.Size < 5*1024*1024*1024 {
			score += 20
		} else if event.Size > 50*1024*1024*1024 {
			score -= 20
		}
	}

	if e.siteProvider != nil && event.SiteName != "" {
		if siteInfo, err := e.siteProvider.GetSiteInfo(ctx, event.SiteName); err == nil && siteInfo != nil {
			if siteInfo.AssumeFree {
				score += 10
			}
		}
	}

	return score
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

	record := &model.SeedingTorrentRecord{
		ClientID:    clientID,
		SiteName:    event.SiteName,
		TorrentID:   event.TorrentID,
		InfoHash:    infoHash,
		HasHR:       event.HasHR,
		Source:      "rss",
		Status:      model.SeedingStatusSeeding,
		IsFree:      event.IsFree,
		FreeEndAt:   event.FreeEndAt,
		TorrentSize: event.Size,
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

	if event.AutoReseed && len(event.ReseedClientIDs) > 0 && e.reseedTrigger != nil {
		go e.reseedTrigger.OnTorrentSeeding(context.Background(), *record, event.ReseedClientIDs)
	}

	e.logger.Debug("createRecordFromPush: record created",
		zap.String("client_id", clientID),
		zap.String("info_hash", infoHash),
		zap.Float64("score", e.calculateEventScore(ctx, event)))
}
