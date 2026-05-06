package seeding

import (
	"context"
	"sync"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type FreeEndMonitor struct {
	mu     sync.Mutex
	timers map[string]*time.Timer
	db     *gorm.DB
	client model.DownloaderProvider
	logger *zap.Logger
	engine *Engine
}

func NewFreeEndMonitor(db *gorm.DB, client model.DownloaderProvider, logger *zap.Logger) *FreeEndMonitor {
	return &FreeEndMonitor{
		timers: make(map[string]*time.Timer),
		db:     db,
		client: client,
		logger: logger,
	}
}

func (m *FreeEndMonitor) SetEngine(e *Engine) {
	m.engine = e
}

func (m *FreeEndMonitor) Schedule(record *model.SeedingTorrentRecord) {
	if !record.IsFree || record.FreeEndAt == nil {
		return
	}

	key := record.ClientID + "|" + record.InfoHash
	delay := time.Until(*record.FreeEndAt)

	if delay <= 0 {
		go m.handleFreeEnded(record)
		return
	}

	delay += 5 * time.Minute

	m.mu.Lock()
	if old, ok := m.timers[key]; ok {
		old.Stop()
	}
	m.timers[key] = time.AfterFunc(delay, func() {
		m.handleFreeEnded(record)
	})
	m.mu.Unlock()

	m.logger.Debug("free end monitor: scheduled",
		zap.String("client", record.ClientID),
		zap.String("info_hash", record.InfoHash),
		zap.Time("free_end_at", *record.FreeEndAt),
		zap.Duration("delay", delay),
	)
}

func (m *FreeEndMonitor) Cancel(clientID, infoHash string) {
	key := clientID + "|" + infoHash
	m.mu.Lock()
	if t, ok := m.timers[key]; ok {
		t.Stop()
		delete(m.timers, key)
	}
	m.mu.Unlock()
}

func (m *FreeEndMonitor) StopAll() {
	m.mu.Lock()
	for key, t := range m.timers {
		t.Stop()
		delete(m.timers, key)
	}
	m.mu.Unlock()
}

func (m *FreeEndMonitor) handleFreeEnded(record *model.SeedingTorrentRecord) {
	defer func() {
		if r := recover(); r != nil {
			m.logger.Error("free end monitor: panic in handleFreeEnded",
				zap.String("client", record.ClientID),
				zap.String("info_hash", record.InfoHash),
				zap.Any("recover", r),
			)
			m.mu.Lock()
			delete(m.timers, record.ClientID+"|"+record.InfoHash)
			m.mu.Unlock()
		}
	}()

	key := record.ClientID + "|" + record.InfoHash
	m.mu.Lock()
	delete(m.timers, key)
	m.mu.Unlock()

	var current model.SeedingTorrentRecord
	if err := m.db.WithContext(context.Background()).Where("client_id = ? AND info_hash = ?", record.ClientID, record.InfoHash).First(&current).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return
		}
		m.logger.Error("free end monitor: query record failed", zap.Error(err))
		return
	}

	if current.Status == model.SeedingStatusDeleted ||
		current.Status == model.SeedingStatusArchived ||
		current.Status == model.SeedingStatusPausedFreeEnd {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if m.engine != nil {
		if err := m.engine.PauseForFreeEnd(ctx, record.ClientID, record.InfoHash); err != nil {
			m.logger.Error("free end monitor: pause failed",
				zap.String("client", record.ClientID),
				zap.String("info_hash", record.InfoHash),
				zap.Error(err),
			)
			return
		}
	}

	if m.client != nil {
		dlClient, err := m.client.Get(record.ClientID)
		if err == nil {
			if err := dlClient.PauseTorrent(ctx, record.InfoHash); err != nil {
				m.logger.Warn("free end monitor: downloader pause failed", zap.Error(err))
			}
		}
	}

	m.logger.Info("free end monitor: 种子免费到期，已暂停",
		zap.String("client", record.ClientID),
		zap.String("info_hash", record.InfoHash),
		zap.String("site", record.SiteName),
	)
}

func (m *FreeEndMonitor) RecoverOnStartup(ctx context.Context) {
	var records []model.SeedingTorrentRecord
	cutoff := time.Now().Add(-5 * time.Minute)
	if err := m.db.WithContext(ctx).
		Where("is_free = ? AND free_end_at IS NOT NULL AND free_end_at < ? AND status NOT IN ?",
			true, cutoff, []string{string(model.SeedingStatusDeleted), string(model.SeedingStatusArchived), string(model.SeedingStatusPausedFreeEnd)}).
		Find(&records).Error; err != nil {
		m.logger.Error("free end monitor: recover startup query failed", zap.Error(err))
		return
	}

	for i := range records {
		m.handleFreeEnded(&records[i])
	}

	var upcoming []model.SeedingTorrentRecord
	if err := m.db.WithContext(ctx).
		Where("is_free = ? AND free_end_at IS NOT NULL AND free_end_at > ? AND status = ?",
			true, cutoff, string(model.SeedingStatusSeeding)).
		Find(&upcoming).Error; err != nil {
		m.logger.Error("free end monitor: recover upcoming query failed", zap.Error(err))
		return
	}

	for i := range upcoming {
		m.Schedule(&upcoming[i])
	}

	m.logger.Info("free end monitor: startup recovery completed",
		zap.Int("expired", len(records)),
		zap.Int("scheduled", len(upcoming)),
	)
}

func (m *FreeEndMonitor) ActiveTimerCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.timers)
}
