package site

import (
	"context"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/adapter"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.562 刀3 锚：StatsSyncService baseCtx 接线——停机取消传播到同步链。
func newStatsSyncTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Site{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// TestStatsSyncBaseCtxCancelled: 已取消 baseCtx + StartSyncAll →
// SyncAllSites 首个 DB 查询即失败（WithContext 取消链通）→ syncing 短内回落 false。
func TestStatsSyncBaseCtxCancelled(t *testing.T) {
	db := newStatsSyncTestDB(t)
	baseCtx, cancel := context.WithCancel(context.Background())
	cancel() // 模拟停机先于同步

	svc := NewStatsSyncService(baseCtx, db, &adapter.Factory{}, zap.NewNop())
	if !svc.StartSyncAll() {
		t.Fatal("首次 StartSyncAll 应返回 true")
	}

	// 等待 goroutine 完成（取消链通 → 首查即败快速返回）
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !svc.GetSyncAllStatus().Running {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if svc.GetSyncAllStatus().Running {
		t.Fatal("取消 baseCtx 下同步应快速终止（Running 回落 false）")
	}
	// 取消链效果：synced=0（无站点被同步——ctx 在首个查询即生效）
	if got := svc.GetSyncAllStatus().Synced; got != 0 {
		t.Fatalf("取消 ctx 下 synced 应=0, got %d", got)
	}
}

// TestStatsSyncNilCtxFallback: nil ctx 兜底 Background（附录A §2.5）。
func TestStatsSyncNilCtxFallback(t *testing.T) {
	db := newStatsSyncTestDB(t)
	svc := NewStatsSyncService(nil, db, &adapter.Factory{}, zap.NewNop())
	if svc.baseCtx == nil {
		t.Fatal("nil ctx 应兜底 Background")
	}
}
