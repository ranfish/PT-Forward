package scheduler

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/setting"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var smtestDBCounter atomic.Uint64 // §59.567 刀B 补修三：unique+cache=shared（goroutine 并发 DB 访问防连接分裂）

func setupSyncManagerDB(t *testing.T) (*gorm.DB, *setting.Repository) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:smtest_%d?mode=memory&cache=shared&_busy_timeout=5000", smtestDBCounter.Add(1))), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&setting.Setting{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := setting.NewRepository(db)
	return db, repo
}

func TestNewSyncManager(t *testing.T) {
	_, repo := setupSyncManagerDB(t)
	rc := setting.NewRuntimeConfig(repo, zap.NewNop())
	sm := NewSyncManager(rc, zap.NewNop())
	if sm == nil {
		t.Fatal("expected non-nil SyncManager")
	}
}

func TestSyncManager_StartAndStop(t *testing.T) {
	_, repo := setupSyncManagerDB(t)
	rc := setting.NewRuntimeConfig(repo, zap.NewNop())
	sm := NewSyncManager(rc, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sm.Start(ctx)
	time.Sleep(50 * time.Millisecond)

	done := make(chan struct{})
	go func() {
		sm.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() hung for more than 2s")
	}
}

func TestSyncManager_StopWithoutStart(t *testing.T) {
	_, repo := setupSyncManagerDB(t)
	rc := setting.NewRuntimeConfig(repo, zap.NewNop())
	sm := NewSyncManager(rc, zap.NewNop())

	done := make(chan struct{})
	go func() {
		sm.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() without Start() hung for more than 2s")
	}
}

func TestSyncManager_ContextCancel(t *testing.T) {
	_, repo := setupSyncManagerDB(t)
	rc := setting.NewRuntimeConfig(repo, zap.NewNop())
	sm := NewSyncManager(rc, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	sm.Start(ctx)
	time.Sleep(50 * time.Millisecond)

	cancel()
	time.Sleep(100 * time.Millisecond)

	done := make(chan struct{})
	go func() {
		sm.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() after cancel hung for more than 2s")
	}
}
