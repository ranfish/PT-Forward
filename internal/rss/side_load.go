package rss

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ranfish/pt-forward/internal/fingerprint"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

type SideLoadManager struct {
	pendingQueue  chan *SideLoadTask
	activeTasks   sync.Map
	siteProvider  model.SiteInfoProvider
	eventEmitter  *SideLoadEventEmitter
	maxConcurrent int
	logger        *zap.Logger
	bencodeCache  *BencodeCache
	wg            sync.WaitGroup
	stopOnce      sync.Once
}

type BencodeCacheEntry struct {
	InfoHash  string
	Size      int64
	ExpiresAt time.Time
}

type BencodeCache struct {
	mu    sync.RWMutex
	items map[string]*BencodeCacheEntry
	max   int
	ttl   time.Duration
}

func NewBencodeCache(maxItems int, ttl time.Duration) *BencodeCache {
	if maxItems <= 0 {
		maxItems = 500
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &BencodeCache{
		items: make(map[string]*BencodeCacheEntry, maxItems),
		max:   maxItems,
		ttl:   ttl,
	}
}

func (c *BencodeCache) Get(key string) (*BencodeCacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.items[key]
	if !ok || time.Now().After(entry.ExpiresAt) {
		return nil, false
	}
	return entry, true
}

func (c *BencodeCache) Set(key string, infoHash string, size int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.items) >= c.max {
		for k, v := range c.items {
			if time.Now().After(v.ExpiresAt) {
				delete(c.items, k)
			}
		}
		if len(c.items) >= c.max {
			for k := range c.items {
				delete(c.items, k)
				break
			}
		}
	}
	c.items[key] = &BencodeCacheEntry{
		InfoHash:  infoHash,
		Size:      size,
		ExpiresAt: time.Now().Add(c.ttl),
	}
}

type SideLoadTask struct {
	TorrentEvent *model.TorrentEvent
	SiteName     string
	Status       model.SideLoadStatus
	CreatedAt    time.Time
	StartedAt    *time.Time
	CompletedAt  *time.Time
	FailedReason string
}

func NewSideLoadManager(siteProvider model.SiteInfoProvider, emitter *SideLoadEventEmitter, logger *zap.Logger) *SideLoadManager {
	return &SideLoadManager{
		pendingQueue:  make(chan *SideLoadTask, 1000),
		siteProvider:  siteProvider,
		eventEmitter:  emitter,
		maxConcurrent: 5,
		logger:        logger,
		bencodeCache:  NewBencodeCache(500, 24*time.Hour),
	}
}

func (m *SideLoadManager) Start(ctx context.Context) {
	for i := 0; i < m.maxConcurrent; i++ {
		m.wg.Add(1)
		go m.worker(ctx, i)
	}
	m.logger.Info("side load manager started", zap.Int("workers", m.maxConcurrent))
}

func (m *SideLoadManager) Stop() {
	m.stopOnce.Do(func() {
		close(m.pendingQueue)
		m.wg.Wait()
		m.logger.Info("side load manager stopped")
	})
}

func (m *SideLoadManager) Enqueue(event *model.TorrentEvent, siteName string) error {
	task := &SideLoadTask{
		TorrentEvent: event,
		SiteName:     siteName,
		Status:       model.SideLoadPending,
		CreatedAt:    time.Now(),
	}
	select {
	case m.pendingQueue <- task:
		m.logger.Debug("side load task enqueued",
			zap.String("site", siteName),
			zap.String("torrent_id", event.TorrentID),
		)
		return nil
	default:
		return fmt.Errorf("side load queue full")
	}
}

func (m *SideLoadManager) worker(ctx context.Context, _ int) {
	defer m.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case task, ok := <-m.pendingQueue:
			if !ok {
				return
			}
			m.processTask(ctx, task)
		}
	}
}

func (m *SideLoadManager) processTask(ctx context.Context, task *SideLoadTask) {
	taskID := task.TorrentEvent.SiteName + ":" + task.TorrentEvent.TorrentID
	m.activeTasks.Store(taskID, task)
	defer m.activeTasks.Delete(taskID)

	now := time.Now()
	task.StartedAt = &now
	task.Status = model.SideLoading
	task.TorrentEvent.SideLoadStatus = model.SideLoading
	task.TorrentEvent.SideLoadStartedAt = &now

	m.eventEmitter.Emit(SideLoadEvent{
		TorrentID: task.TorrentEvent.TorrentID,
		SiteName:  task.SiteName,
		Status:    model.SideLoading,
	}, m.logger)

	cacheKey := task.SiteName + ":" + task.TorrentEvent.TorrentID
	if entry, ok := m.bencodeCache.Get(cacheKey); ok {
		task.TorrentEvent.InfoHash = entry.InfoHash
		task.TorrentEvent.Size = entry.Size
		task.TorrentEvent.SideLoadStatus = model.SideLoadCompleted
		completedAt := time.Now()
		task.TorrentEvent.SideLoadFinishedAt = &completedAt
		task.Status = model.SideLoadCompleted
		task.CompletedAt = &completedAt

		m.logger.Debug("side load cache hit",
			zap.String("site", task.SiteName),
			zap.String("torrent_id", task.TorrentEvent.TorrentID),
			zap.String("info_hash", entry.InfoHash),
		)

		m.eventEmitter.Emit(SideLoadEvent{
			TorrentID:    task.TorrentEvent.TorrentID,
			SiteName:     task.SiteName,
			Status:       model.SideLoadCompleted,
			TorrentEvent: task.TorrentEvent,
		}, m.logger)
		return
	}

	adapter, err := m.siteProvider.GetAdapter(ctx, task.SiteName)
	if err != nil {
		m.markFailed(task, fmt.Errorf("get site adapter: %w", err))
		return
	}

	siteCfg, err := m.siteProvider.GetSiteConfig(ctx, task.SiteName)
	if err != nil || siteCfg == nil {
		m.markFailed(task, fmt.Errorf("get site config: %w", err))
		return
	}

	torrentData, err := adapter.DownloadTorrent(ctx, siteCfg, task.TorrentEvent.TorrentID)
	if err != nil {
		m.markFailed(task, fmt.Errorf("download torrent: %w", err))
		return
	}

	meta, err := fingerprint.ComputeFromTorrent(torrentData)
	if err != nil {
		m.markFailed(task, fmt.Errorf("compute torrent meta: %w", err))
		return
	}

	task.TorrentEvent.TorrentData = torrentData
	task.TorrentEvent.InfoHash = meta.InfoHash
	task.TorrentEvent.Size = meta.TotalSize
	task.TorrentEvent.SideLoadStatus = model.SideLoadCompleted

	m.bencodeCache.Set(cacheKey, meta.InfoHash, meta.TotalSize)

	completedAt := time.Now()
	task.TorrentEvent.SideLoadFinishedAt = &completedAt
	task.Status = model.SideLoadCompleted
	task.CompletedAt = &completedAt

	m.logger.Info("side load completed",
		zap.String("site", task.SiteName),
		zap.String("torrent_id", task.TorrentEvent.TorrentID),
		zap.String("info_hash", meta.InfoHash),
		zap.Int64("size", meta.TotalSize),
	)

	m.eventEmitter.Emit(SideLoadEvent{
		TorrentID:    task.TorrentEvent.TorrentID,
		SiteName:     task.SiteName,
		Status:       model.SideLoadCompleted,
		TorrentEvent: task.TorrentEvent,
	}, m.logger)
}

func (m *SideLoadManager) markFailed(task *SideLoadTask, err error) {
	now := time.Now()
	task.Status = model.SideLoadFailed
	task.FailedReason = err.Error()
	task.CompletedAt = &now
	task.TorrentEvent.SideLoadStatus = model.SideLoadFailed
	task.TorrentEvent.SideLoadFinishedAt = &now

	m.logger.Warn("side load failed",
		zap.String("site", task.SiteName),
		zap.String("torrent_id", task.TorrentEvent.TorrentID),
		zap.Error(err),
	)

	m.eventEmitter.Emit(SideLoadEvent{
		TorrentID:    task.TorrentEvent.TorrentID,
		SiteName:     task.SiteName,
		Status:       model.SideLoadFailed,
		FailedReason: err.Error(),
	}, m.logger)
}
