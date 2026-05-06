package rss

import (
	"context"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/mocks"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupDiskGuardDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.AutoMigrate(&model.RSSSubscription{})
	return db
}

func TestDiskGuard_PauseOnLowSpace(t *testing.T) {
	db := setupDiskGuardDB(t)
	guard := NewDiskGuard(db, zap.NewNop())

	client := &mocks.DownloaderClient{GetMainDataFn: func(_ context.Context) (*model.Maindata, error) { return &model.Maindata{FreeSpace: 100}, nil }}
	guard.SetClientProvider(&mocks.DownloaderProvider{Client: client})

	sub := &model.RSSSubscription{
		Enabled:            true,
		DiskGuardEnabled:   true,
		DiskGuardThreshold: 1073741824,
		ClientID:           "test",
		Name:               "test-sub",
		Paused:             false,
	}
	db.Create(sub)

	guard.check(context.Background())

	var updated model.RSSSubscription
	db.First(&updated, sub.ID)
	if !updated.Paused {
		t.Error("subscription should be paused when disk is low")
	}
	if updated.PauseReason != "disk_guard" {
		t.Errorf("pause_reason = %q, want disk_guard", updated.PauseReason)
	}
}

func TestDiskGuard_ResumeOnSufficientSpace(t *testing.T) {
	db := setupDiskGuardDB(t)
	guard := NewDiskGuard(db, zap.NewNop())

	client := &mocks.DownloaderClient{GetMainDataFn: func(_ context.Context) (*model.Maindata, error) {
		return &model.Maindata{FreeSpace: 10 * 1024 * 1024 * 1024}, nil
	}}
	guard.SetClientProvider(&mocks.DownloaderProvider{Client: client})

	now := time.Now()
	sub := &model.RSSSubscription{
		Enabled:            true,
		DiskGuardEnabled:   true,
		DiskGuardThreshold: 1073741824,
		ClientID:           "test",
		Name:               "test-sub",
		Paused:             true,
		PauseReason:        "disk_guard",
		PausedAt:           &now,
	}
	db.Create(sub)

	guard.check(context.Background())

	var updated model.RSSSubscription
	db.First(&updated, sub.ID)
	if updated.Paused {
		t.Error("subscription should be resumed when disk is sufficient")
	}
	if updated.PauseReason != "" {
		t.Errorf("pause_reason should be empty, got %q", updated.PauseReason)
	}
}

func TestDiskGuard_SkipNonDiskGuardSubs(t *testing.T) {
	db := setupDiskGuardDB(t)
	guard := NewDiskGuard(db, zap.NewNop())

	client := &mocks.DownloaderClient{GetMainDataFn: func(_ context.Context) (*model.Maindata, error) { return &model.Maindata{FreeSpace: 100}, nil }}
	guard.SetClientProvider(&mocks.DownloaderProvider{Client: client})

	sub := &model.RSSSubscription{
		Enabled:  true,
		ClientID: "test",
		Name:     "normal-sub",
		Paused:   false,
	}
	db.Create(sub)
	db.Model(&model.RSSSubscription{}).Where("id = ?", sub.ID).Update("disk_guard_enabled", false)

	guard.check(context.Background())

	var updated model.RSSSubscription
	db.First(&updated, sub.ID)
	if updated.Paused {
		t.Error("non-disk-guard subscription should not be paused")
	}
}

func TestDiskGuard_NoTouchManualPause(t *testing.T) {
	db := setupDiskGuardDB(t)
	guard := NewDiskGuard(db, zap.NewNop())

	client := &mocks.DownloaderClient{GetMainDataFn: func(_ context.Context) (*model.Maindata, error) {
		return &model.Maindata{FreeSpace: 10 * 1024 * 1024 * 1024}, nil
	}}
	guard.SetClientProvider(&mocks.DownloaderProvider{Client: client})

	now := time.Now()
	sub := &model.RSSSubscription{
		Enabled:            true,
		DiskGuardEnabled:   true,
		DiskGuardThreshold: 1073741824,
		ClientID:           "test",
		Name:               "manual-paused",
		Paused:             true,
		PauseReason:        "manual",
		PausedAt:           &now,
	}
	db.Create(sub)

	guard.check(context.Background())

	var updated model.RSSSubscription
	db.First(&updated, sub.ID)
	if !updated.Paused {
		t.Error("manually paused subscription should not be auto-resumed")
	}
	if updated.PauseReason != "manual" {
		t.Errorf("pause_reason = %q, want manual", updated.PauseReason)
	}
}
