package seeding

import (
	"context"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupFreeEndDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(uniqueSQLiteDSN()), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.AutoMigrate(&model.SeedingTorrentRecord{})
	return db
}

func TestFreeEndMonitor_Schedule_PausedWhenDownloading(t *testing.T) {
	db := setupFreeEndDB(t)
	engine := NewEngine(db, zap.NewNop())
	dlClient := &mockDownloaderClient{
		torrentByHash: map[string]*model.TorrentInfo{
			"abc123": {Hash: "abc123", Progress: 0.3},
		},
	}
	engine.freeEndMonitor.client = &mockDownloaderProvider{
		clients: map[string]*mockDownloaderClient{"test-client": dlClient},
	}

	freeEnd := time.Now().Add(-10 * time.Second)
	record := &model.SeedingTorrentRecord{
		ClientID:  "test-client",
		InfoHash:  "abc123",
		SiteName:  "test-site",
		TorrentID: "12345",
		IsFree:    true,
		FreeEndAt: &freeEnd,
		Status:    model.SeedingStatusSeeding,
	}
	db.Create(record)

	engine.freeEndMonitor.Schedule(record)
	time.Sleep(200 * time.Millisecond)

	var updated model.SeedingTorrentRecord
	db.First(&updated, record.ID)
	if updated.Status != model.SeedingStatusPausedFreeEnd {
		t.Errorf("downloading torrent should be paused, got %s", updated.Status)
	}
}

func TestFreeEndMonitor_Schedule_KeepsSeedingWhenComplete(t *testing.T) {
	db := setupFreeEndDB(t)
	engine := NewEngine(db, zap.NewNop())
	dlClient := &mockDownloaderClient{
		torrentByHash: map[string]*model.TorrentInfo{
			"abc123": {Hash: "abc123", Progress: 1.0},
		},
	}
	engine.freeEndMonitor.client = &mockDownloaderProvider{
		clients: map[string]*mockDownloaderClient{"test-client": dlClient},
	}

	freeEnd := time.Now().Add(-10 * time.Second)
	record := &model.SeedingTorrentRecord{
		ClientID:  "test-client",
		InfoHash:  "abc123",
		SiteName:  "test-site",
		TorrentID: "12345",
		IsFree:    true,
		FreeEndAt: &freeEnd,
		Status:    model.SeedingStatusSeeding,
	}
	db.Create(record)

	engine.freeEndMonitor.Schedule(record)
	time.Sleep(200 * time.Millisecond)

	var updated model.SeedingTorrentRecord
	db.First(&updated, record.ID)
	if updated.Status != model.SeedingStatusSeeding {
		t.Errorf("completed torrent should keep seeding, got %s", updated.Status)
	}
	if updated.IsFree {
		t.Errorf("is_free should be cleared after free expired")
	}
}

func TestFreeEndMonitor_Cancel(t *testing.T) {
	db := setupFreeEndDB(t)
	mon := NewFreeEndMonitor(db, nil, zap.NewNop())

	freeEnd := time.Now().Add(10 * time.Second)
	record := &model.SeedingTorrentRecord{
		ClientID:  "test-client",
		InfoHash:  "abc123",
		SiteName:  "test-site",
		TorrentID: "12345",
		IsFree:    true,
		FreeEndAt: &freeEnd,
		Status:    model.SeedingStatusSeeding,
	}
	db.Create(record)

	mon.Schedule(record)
	if mon.ActiveTimerCount() != 1 {
		t.Fatalf("expected 1 timer")
	}

	mon.Cancel("test-client", "abc123")
	if mon.ActiveTimerCount() != 0 {
		t.Errorf("expected 0 timers after cancel, got %d", mon.ActiveTimerCount())
	}
}

func TestFreeEndMonitor_SkipNonFree(t *testing.T) {
	db := setupFreeEndDB(t)
	mon := NewFreeEndMonitor(db, nil, zap.NewNop())

	record := &model.SeedingTorrentRecord{
		ClientID: "test-client",
		InfoHash: "abc123",
		IsFree:   false,
		Status:   model.SeedingStatusSeeding,
	}

	mon.Schedule(record)
	if mon.ActiveTimerCount() != 0 {
		t.Errorf("non-free torrent should not be scheduled")
	}
}

func TestFreeEndMonitor_SkipNoFreeEndAt(t *testing.T) {
	db := setupFreeEndDB(t)
	mon := NewFreeEndMonitor(db, nil, zap.NewNop())

	record := &model.SeedingTorrentRecord{
		ClientID: "test-client",
		InfoHash: "abc123",
		IsFree:   true,
		Status:   model.SeedingStatusSeeding,
	}

	mon.Schedule(record)
	if mon.ActiveTimerCount() != 0 {
		t.Errorf("torrent without free_end_at should not be scheduled")
	}
}

func TestFreeEndMonitor_StopAll(t *testing.T) {
	db := setupFreeEndDB(t)
	mon := NewFreeEndMonitor(db, nil, zap.NewNop())

	freeEnd := time.Now().Add(1 * time.Hour)
	for i := 0; i < 3; i++ {
		record := &model.SeedingTorrentRecord{
			ClientID:  "test-client",
			InfoHash:  string(rune('a' + i)),
			IsFree:    true,
			FreeEndAt: &freeEnd,
			Status:    model.SeedingStatusSeeding,
		}
		mon.Schedule(record)
	}

	if mon.ActiveTimerCount() != 3 {
		t.Fatalf("expected 3 timers, got %d", mon.ActiveTimerCount())
	}

	mon.StopAll()
	if mon.ActiveTimerCount() != 0 {
		t.Errorf("expected 0 timers after stop all, got %d", mon.ActiveTimerCount())
	}
}

func TestFreeEndMonitor_RecoverOnStartup_Downloading(t *testing.T) {
	db := setupFreeEndDB(t)
	dlClient := &mockDownloaderClient{
		torrentByHash: map[string]*model.TorrentInfo{
			"expired": {Hash: "expired", Progress: 0.5},
		},
	}

	past := time.Now().Add(-1 * time.Hour)
	future := time.Now().Add(1 * time.Hour)

	expired := &model.SeedingTorrentRecord{
		ClientID:  "client1",
		InfoHash:  "expired",
		IsFree:    true,
		FreeEndAt: &past,
		Status:    model.SeedingStatusSeeding,
	}
	db.Create(expired)

	upcoming := &model.SeedingTorrentRecord{
		ClientID:  "client1",
		InfoHash:  "upcoming",
		IsFree:    true,
		FreeEndAt: &future,
		Status:    model.SeedingStatusSeeding,
	}
	db.Create(upcoming)

	engine := NewEngine(db, zap.NewNop())
	engine.freeEndMonitor.client = &mockDownloaderProvider{
		clients: map[string]*mockDownloaderClient{"client1": dlClient},
	}
	mon := engine.freeEndMonitor

	mon.RecoverOnStartup(context.Background())

	var updated model.SeedingTorrentRecord
	db.First(&updated, expired.ID)
	if updated.Status != model.SeedingStatusPausedFreeEnd {
		t.Errorf("expired downloading record should be paused, got %s", updated.Status)
	}

	if mon.ActiveTimerCount() != 1 {
		t.Errorf("upcoming record should have 1 timer, got %d", mon.ActiveTimerCount())
	}
}

func TestFreeEndMonitor_RecoverOnStartup_Completed(t *testing.T) {
	db := setupFreeEndDB(t)
	dlClient := &mockDownloaderClient{
		torrentByHash: map[string]*model.TorrentInfo{
			"expired": {Hash: "expired", Progress: 1.0},
		},
	}

	past := time.Now().Add(-1 * time.Hour)

	expired := &model.SeedingTorrentRecord{
		ClientID:  "client1",
		InfoHash:  "expired",
		IsFree:    true,
		FreeEndAt: &past,
		Status:    model.SeedingStatusSeeding,
	}
	db.Create(expired)

	engine := NewEngine(db, zap.NewNop())
	engine.freeEndMonitor.client = &mockDownloaderProvider{
		clients: map[string]*mockDownloaderClient{"client1": dlClient},
	}
	mon := engine.freeEndMonitor

	mon.RecoverOnStartup(context.Background())

	var updated model.SeedingTorrentRecord
	db.First(&updated, expired.ID)
	if updated.Status != model.SeedingStatusSeeding {
		t.Errorf("expired completed record should keep seeding, got %s", updated.Status)
	}
	if updated.IsFree {
		t.Errorf("is_free should be cleared")
	}
}

func TestFreeEndMonitor_AlreadyPaused(t *testing.T) {
	db := setupFreeEndDB(t)
	mon := NewFreeEndMonitor(db, nil, zap.NewNop())
	engine := NewEngine(db, zap.NewNop())
	mon.SetEngine(engine)

	past := time.Now().Add(-1 * time.Hour)
	record := &model.SeedingTorrentRecord{
		ClientID:  "client1",
		InfoHash:  "already_paused",
		IsFree:    true,
		FreeEndAt: &past,
		Status:    model.SeedingStatusPausedFreeEnd,
	}
	db.Create(record)

	mon.handleFreeEnded(record)

	var updated model.SeedingTorrentRecord
	db.First(&updated, record.ID)
	if updated.Status != model.SeedingStatusPausedFreeEnd {
		t.Errorf("already paused record should remain paused, got %s", updated.Status)
	}
}

func TestFreeEndMonitor_NoDownloader_PausedAsDefault(t *testing.T) {
	db := setupFreeEndDB(t)
	engine := NewEngine(db, zap.NewNop())
	engine.freeEndMonitor.client = nil

	freeEnd := time.Now().Add(-10 * time.Second)
	record := &model.SeedingTorrentRecord{
		ClientID:  "test-client",
		InfoHash:  "abc123",
		SiteName:  "test-site",
		TorrentID: "12345",
		IsFree:    true,
		FreeEndAt: &freeEnd,
		Status:    model.SeedingStatusSeeding,
	}
	db.Create(record)

	engine.freeEndMonitor.handleFreeEnded(record)

	var updated model.SeedingTorrentRecord
	db.First(&updated, record.ID)
	if updated.Status != model.SeedingStatusPausedFreeEnd {
		t.Errorf("no downloader = safe default = paused, got %s", updated.Status)
	}
}
