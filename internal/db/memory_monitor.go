package db

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

type MemoryConfig struct {
	MaxTotalMB int     `json:"max_total_mb"`
	WarnPercent float64 `json:"warn_percent"`
}

type EvictFunc func(ctx context.Context, targetBytes uint64) uint64

type MemoryMonitor struct {
	cfg        MemoryConfig
	logger     *zap.Logger
	evictors   []evictorEntry
	mu         sync.Mutex
	wg         sync.WaitGroup
	running    atomic.Bool
	lastRSS    atomic.Uint64
	warnLevel  atomic.Bool
	stopCh     chan struct{}
}

type evictorEntry struct {
	name string
	fn   EvictFunc
}

func NewMemoryMonitor(cfg MemoryConfig, logger *zap.Logger) *MemoryMonitor {
	if cfg.WarnPercent <= 0 {
		cfg.WarnPercent = 0.7
	}
	return &MemoryMonitor{
		cfg:    cfg,
		logger: logger,
		stopCh: make(chan struct{}),
	}
}

func (m *MemoryMonitor) Start(ctx context.Context) {
	if m.cfg.MaxTotalMB <= 0 {
		return
	}
	if !m.running.CompareAndSwap(false, true) {
		return
	}

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-m.stopCh:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.collect()
				m.checkLevels(ctx)
			}
		}
	}()
}

func (m *MemoryMonitor) Stop() {
	if m.running.CompareAndSwap(true, false) {
		close(m.stopCh)
	}
	m.wg.Wait()
}

func (m *MemoryMonitor) CurrentRSS() uint64 {
	return m.lastRSS.Load()
}

func (m *MemoryMonitor) IsWarnLevel() bool {
	return m.warnLevel.Load()
}

func (m *MemoryMonitor) RegisterEvictor(name string, fn EvictFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.evictors = append(m.evictors, evictorEntry{name: name, fn: fn})
}

func (m *MemoryMonitor) collect() {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	m.lastRSS.Store(ms.Sys)
}

func (m *MemoryMonitor) checkLevels(ctx context.Context) {
	rss := m.lastRSS.Load()
	maxBytes := uint64(m.cfg.MaxTotalMB) * 1024 * 1024 //nolint:gosec
	if maxBytes == 0 {
		return
	}

	ratio := float64(rss) / float64(maxBytes)
	wasWarn := m.warnLevel.Load()

	switch {
	case ratio >= 0.9:
		m.warnLevel.Store(true)
		if m.logger != nil {
			m.logger.Error("memory emergency level",
				zap.Uint64("rss_mb", rss/1024/1024),
				zap.Float64("ratio", ratio))
		}
		m.triggerEvict(ctx, 0.5)
	case ratio >= m.cfg.WarnPercent:
		m.warnLevel.Store(true)
		if !wasWarn && m.logger != nil {
			m.logger.Warn("memory warn level",
				zap.Uint64("rss_mb", rss/1024/1024),
				zap.Float64("ratio", ratio))
		}
		m.triggerEvict(ctx, 0.3)
	default:
		m.warnLevel.Store(false)
	}
}

func (m *MemoryMonitor) triggerEvict(ctx context.Context, fraction float64) {
	m.mu.Lock()
	evictors := make([]evictorEntry, len(m.evictors))
	copy(evictors, m.evictors)
	m.mu.Unlock()

	rss := m.lastRSS.Load()
	targetBytes := uint64(float64(rss) * fraction)

	for _, e := range evictors {
		freed := e.fn(ctx, targetBytes)
		if freed > 0 && m.logger != nil {
			m.logger.Debug("memory evictor released bytes",
				zap.String("evictor", e.name),
				zap.Uint64("freed_mb", freed/1024/1024))
		}
	}
}
