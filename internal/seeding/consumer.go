package seeding

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/pusher"
	"go.uber.org/zap"
	"gorm.io/gorm/clause"
)

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

	if !IsWithinActiveWindow(cfg.ActiveTimeWindows) {
		e.logger.Debug("OnPushed: outside active time windows, skipping",
			zap.String("client_id", event.ClientID))
		return
	}

	key := recordKey(event.ClientID, event.InfoHash)
	e.mu.RLock()
	_, exists := e.recordMap[key]
	e.mu.RUnlock()
	if exists {
		return
	}

	record := &model.SeedingTorrentRecord{
		ClientID:    event.ClientID,
		SiteName:    event.SiteName,
		TorrentID:   event.TorrentID,
		InfoHash:    event.InfoHash,
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
		e.logger.Warn("OnPushed: create record failed",
			zap.String("client_id", event.ClientID),
			zap.String("info_hash", event.InfoHash),
			zap.Error(result.Error))
		return
	}

	if result.RowsAffected == 0 {
		var loaded model.SeedingTorrentRecord
		if err := e.db.WithContext(ctx).
			Where("client_id = ? AND info_hash = ?", event.ClientID, event.InfoHash).
			First(&loaded).Error; err != nil {
			e.logger.Warn("OnPushed: load existing record failed",
				zap.String("info_hash", event.InfoHash),
				zap.Error(err))
			return
		}
		if loaded.Status == model.SeedingStatusDeleted {
			e.logger.Info("OnPushed: restoring deleted record",
				zap.String("client_id", event.ClientID),
				zap.String("info_hash", event.InfoHash))
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

	e.logger.Debug("OnPushed: seeding record created",
		zap.String("client_id", event.ClientID),
		zap.String("info_hash", event.InfoHash),
		zap.String("site_name", event.SiteName))
}

var _ = fmt.Sprintf
var _ = json.Marshal
