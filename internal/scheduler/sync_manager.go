package scheduler

import (
	"context"
	"sync"
	"time"

	"github.com/ranfish/pt-forward/internal/setting"
	"go.uber.org/zap"
)

type SyncManager struct {
	runtimeCfg *setting.RuntimeConfig
	logger     *zap.Logger
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

func NewSyncManager(runtimeCfg *setting.RuntimeConfig, logger *zap.Logger) *SyncManager {
	return &SyncManager{
		runtimeCfg: runtimeCfg,
		logger:     logger,
	}
}

func (sm *SyncManager) Start(ctx context.Context) {
	ctx, sm.cancel = context.WithCancel(ctx)

	sm.wg.Add(1)
	go func() {
		defer sm.wg.Done()
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := sm.runtimeCfg.Reload(ctx); err != nil {
					sm.logger.Warn("sync manager: runtime config reload failed", zap.Error(err))
				}
			}
		}
	}()

	sm.logger.Info("sync manager started", zap.Duration("interval", 5*time.Minute))
}

func (sm *SyncManager) Stop() {
	if sm.cancel != nil {
		sm.cancel()
	}
	sm.wg.Wait()
	sm.logger.Info("sync manager stopped", zap.String("component", "sync_manager"))
}
