package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/setting"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupSyncManagerDB(t *testing.T) (*gorm.DB, *setting.Repository) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
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
	sm.Stop()
}

func TestSyncManager_StopWithoutStart(t *testing.T) {
	_, repo := setupSyncManagerDB(t)
	rc := setting.NewRuntimeConfig(repo, zap.NewNop())
	sm := NewSyncManager(rc, zap.NewNop())

	sm.Stop()
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

	sm.Stop()
}
