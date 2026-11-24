package seeding

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var rootctxtestDBCounter atomic.Uint64 // §59.567 刀B 补修三：unique+cache=shared（goroutine 并发 DB 访问防连接分裂）

// §59.562 刀4 锚：Engine rootCtx 接线——service ctx cancel 收敛 refresh 族 worker。
// 构造：pusher=nil（Start 只起 refreshMaindataLoop+archiveLoop 两 worker·wg 精确
// 锁定修复目标）；不调 Stop（隔离 refreshCancel 通道·纯证 root 派生链）。
func TestEngineRootCtxCancelsWorkers(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:rootctxtest_%d?mode=memory&cache=shared&_busy_timeout=5000", rootctxtestDBCounter.Add(1))), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.SeedingTorrentRecord{}, &model.DeleteRule{}, &model.SeedingClientConfig{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	e := NewEngine(db, zap.NewNop())

	rootCtx, cancel := context.WithCancel(context.Background())
	if err := e.Start(rootCtx); err != nil {
		t.Fatalf("start: %v", err)
	}

	cancel() // 模拟停机：只 cancel service ctx·不调 Stop

	// wg.Wait 必须回归——修复前 refresh 族只听 refreshCancel（Background 局部
	// 根）·root cancel 不达 worker·Wait 恒挂
	done := make(chan struct{})
	go func() { e.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("root ctx cancel 后 worker 应收敛（refreshCtx 应派生自 rootCtx）")
	}

	// Stop 幂等可用（refreshCancel 双通道不冲突）
	stopDone := make(chan error, 1)
	go func() { stopDone <- e.Stop(context.Background()) }()
	select {
	case <-stopDone:
	case <-time.After(3 * time.Second):
		t.Fatal("Stop 应正常返回（双取消通道幂等）")
	}
}

// TestEngineRootCtxStored: Start 注入的 ctx 存入 e.rootCtx（派生单源）。
func TestEngineRootCtxStored(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:rootctxtest_%d?mode=memory&cache=shared&_busy_timeout=5000", rootctxtestDBCounter.Add(1))), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.SeedingTorrentRecord{}, &model.DeleteRule{}, &model.SeedingClientConfig{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	e := NewEngine(db, zap.NewNop())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := e.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	if e.rootCtx != ctx {
		t.Fatal("Start 注入 ctx 应存入 rootCtx（worker 派生单源）")
	}
	e.Stop(context.Background())
}

// §59.567 刀C 锚：Stop 幂等——双 Stop 不 panic 不挂（停机链重入安全）。
func TestEngineStopIdempotent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:rootctxtest_%d?mode=memory&cache=shared&_busy_timeout=5000", rootctxtestDBCounter.Add(1))), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.SeedingTorrentRecord{}, &model.DeleteRule{}, &model.SeedingClientConfig{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	e := NewEngine(db, zap.NewNop())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := e.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := e.Stop(context.Background()); err != nil {
		t.Fatalf("first stop: %v", err)
	}
	if err := e.Stop(context.Background()); err != nil { // 重入
		t.Fatalf("second stop should be idempotent: %v", err)
	}
}
