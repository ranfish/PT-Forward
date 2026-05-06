package seeding

import (
	"context"
	"sync"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type FreeWaitMonitor struct {
	mu      sync.Mutex
	pending map[string]*freeWaitEntry
	db      *gorm.DB
	logger  *zap.Logger
	engine  *Engine
}

type freeWaitEntry struct {
	SiteName    string
	TorrentID   string
	InfoHash    string
	Title       string
	Size        int64
	AddedAt     time.Time
	CheckBefore *time.Time
	CheckCount  int
}

func NewFreeWaitMonitor(db *gorm.DB, logger *zap.Logger) *FreeWaitMonitor {
	return &FreeWaitMonitor{
		pending: make(map[string]*freeWaitEntry),
		db:      db,
		logger:  logger,
	}
}

func (m *FreeWaitMonitor) SetEngine(e *Engine) {
	m.engine = e
}

func (m *FreeWaitMonitor) Add(siteName, torrentID, infoHash, title string, size int64, checkBefore *time.Time) {
	if torrentID == "" {
		return
	}

	key := siteName + "|" + torrentID

	m.mu.Lock()
	if _, exists := m.pending[key]; exists {
		m.mu.Unlock()
		return
	}
	m.pending[key] = &freeWaitEntry{
		SiteName:    siteName,
		TorrentID:   torrentID,
		InfoHash:    infoHash,
		Title:       title,
		Size:        size,
		AddedAt:     time.Now(),
		CheckBefore: checkBefore,
	}
	m.mu.Unlock()

	if m.logger != nil {
		m.logger.Debug("free wait: added",
			zap.String("site", siteName),
			zap.String("torrent_id", torrentID),
			zap.String("title", title),
		)
	}
}

func (m *FreeWaitMonitor) Remove(siteName, torrentID string) {
	key := siteName + "|" + torrentID
	m.mu.Lock()
	delete(m.pending, key)
	m.mu.Unlock()
}

type DiscountChecker interface {
	CheckDiscount(ctx context.Context, siteName, torrentID string) (model.DiscountLevel, error)
}

func (m *FreeWaitMonitor) CheckOnce(ctx context.Context, checker DiscountChecker, addFunc func(ctx context.Context, entry *freeWaitEntry) error) int {
	m.mu.Lock()
	entries := make([]*freeWaitEntry, 0, len(m.pending))
	for _, e := range m.pending {
		entries = append(entries, e)
	}
	m.mu.Unlock()

	processed := 0
	now := time.Now()

	for _, e := range entries {
		if ctx.Err() != nil {
			break
		}

		if e.CheckBefore != nil && now.After(*e.CheckBefore) {
			m.Remove(e.SiteName, e.TorrentID)
			m.logger.Debug("free wait: expired",
				zap.String("site", e.SiteName),
				zap.String("torrent_id", e.TorrentID),
			)
			continue
		}

		discount, err := checker.CheckDiscount(ctx, e.SiteName, e.TorrentID)
		if err != nil {
			m.logger.Warn("free wait: check discount failed",
				zap.String("site", e.SiteName),
				zap.String("torrent_id", e.TorrentID),
				zap.Error(err),
			)
			continue
		}

		if discount == model.DiscountFree || discount == model.Discount2xFree ||
			discount == model.Discount2xUp || discount == model.Discount2x50 ||
			discount == model.DiscountPercent50 {
			if addFunc != nil {
				if err := addFunc(ctx, e); err != nil {
					m.logger.Error("free wait: add torrent failed",
						zap.String("site", e.SiteName),
						zap.String("torrent_id", e.TorrentID),
						zap.Error(err),
					)
					continue
				}
			}

			m.Remove(e.SiteName, e.TorrentID)
			processed++

			m.logger.Info("free wait: torrent became free, adding",
				zap.String("site", e.SiteName),
				zap.String("torrent_id", e.TorrentID),
				zap.String("title", e.Title),
				zap.String("discount", string(discount)),
			)
		} else {
			m.mu.Lock()
			if entry, ok := m.pending[e.SiteName+"|"+e.TorrentID]; ok {
				entry.CheckCount++
			}
			m.mu.Unlock()
		}
	}

	return processed
}

func (m *FreeWaitMonitor) PendingCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.pending)
}

func (m *FreeWaitMonitor) ClearAll() {
	m.mu.Lock()
	m.pending = make(map[string]*freeWaitEntry)
	m.mu.Unlock()
}
