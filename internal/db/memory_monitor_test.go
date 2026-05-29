package db

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestMemoryMonitor_Disabled(t *testing.T) {
	m := NewMemoryMonitor(MemoryConfig{MaxTotalMB: 0}, nil)
	m.Start(context.Background())
	time.Sleep(50 * time.Millisecond)
	m.Stop()
}

func TestMemoryMonitor_StartStop(t *testing.T) {
	logger := zap.NewNop()
	m := NewMemoryMonitor(MemoryConfig{MaxTotalMB: 256, WarnPercent: 0.7}, logger)
	m.Start(context.Background())

	m.collect()
	rss := m.CurrentRSS()
	if rss == 0 {
		t.Error("expected non-zero RSS after collect")
	}
	m.Stop()
}

func TestMemoryMonitor_Evictor(t *testing.T) {
	logger := zap.NewNop()
	m := NewMemoryMonitor(MemoryConfig{MaxTotalMB: 1, WarnPercent: 0.1}, logger)

	var evicted atomic.Uint64
	m.RegisterEvictor("test", func(_ context.Context, target uint64) uint64 {
		evicted.Store(target)
		return target / 2
	})

	m.collect()
	m.checkLevels(context.Background())

	if evicted.Load() == 0 {
		t.Error("evictor was not called")
	}
	m.Stop()
}

func TestMemoryMonitor_IsWarnLevel(t *testing.T) {
	m := NewMemoryMonitor(MemoryConfig{MaxTotalMB: 1, WarnPercent: 0.1}, zap.NewNop())
	m.collect()
	m.checkLevels(context.Background())
	_ = m.IsWarnLevel()
}

func TestMemoryMonitor_NoDoubleStart(t *testing.T) {
	m := NewMemoryMonitor(MemoryConfig{MaxTotalMB: 256}, zap.NewNop())
	m.Start(context.Background())
	m.Start(context.Background())
	time.Sleep(50 * time.Millisecond)
	m.Stop()
}
