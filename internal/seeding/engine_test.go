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

func setupEngineTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.SeedingTorrentRecord{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestEngine_StartEmpty(t *testing.T) {
	db := setupEngineTestDB(t)
	e := NewEngine(db, zap.NewNop())
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if e.TotalActiveCount() != 0 {
		t.Error("should have 0 records")
	}
}

func TestEngine_AddAndRemoveRecord(t *testing.T) {
	db := setupEngineTestDB(t)
	e := NewEngine(db, zap.NewNop())
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	record := &model.SeedingTorrentRecord{
		ClientID:  "client-1",
		InfoHash:  "abc123",
		SiteName:  "site1",
		TorrentID: "42",
		Status:    model.SeedingStatusSeeding,
	}
	if err := e.AddSeedingRecord(context.Background(), record); err != nil {
		t.Fatal(err)
	}

	if e.GetActiveCount("client-1") != 1 {
		t.Errorf("expected 1, got %d", e.GetActiveCount("client-1"))
	}

	got, ok := e.GetRecord("client-1", "abc123")
	if !ok {
		t.Fatal("record should exist")
	}
	if got.TorrentID != "42" {
		t.Errorf("expected 42, got %s", got.TorrentID)
	}

	if err := e.RemoveSeedingRecord(context.Background(), "client-1", "abc123"); err != nil {
		t.Fatal(err)
	}

	if e.GetActiveCount("client-1") != 0 {
		t.Error("should be 0 after removal")
	}
}

func TestEngine_DuplicateRecord(t *testing.T) {
	db := setupEngineTestDB(t)
	e := NewEngine(db, zap.NewNop())
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	r1 := &model.SeedingTorrentRecord{ClientID: "c1", InfoHash: "h1", SiteName: "s", TorrentID: "1"}
	r2 := &model.SeedingTorrentRecord{ClientID: "c1", InfoHash: "h1", SiteName: "s", TorrentID: "2"}

	if err := e.AddSeedingRecord(context.Background(), r1); err != nil {
		t.Fatal(err)
	}
	if err := e.AddSeedingRecord(context.Background(), r2); err == nil {
		t.Error("expected error for duplicate")
	}
}

func TestEngine_MissingFields(t *testing.T) {
	db := setupEngineTestDB(t)
	e := NewEngine(db, zap.NewNop())

	r := &model.SeedingTorrentRecord{ClientID: "", InfoHash: "h1"}
	if err := e.AddSeedingRecord(context.Background(), r); err == nil {
		t.Error("expected error for empty client_id")
	}
}

func TestEngine_ListByClient(t *testing.T) {
	db := setupEngineTestDB(t)
	e := NewEngine(db, zap.NewNop())
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	if err := e.AddSeedingRecord(context.Background(), &model.SeedingTorrentRecord{ClientID: "c1", InfoHash: "h1", SiteName: "s", TorrentID: "1"}); err != nil {
		t.Fatal(err)
	}
	if err := e.AddSeedingRecord(context.Background(), &model.SeedingTorrentRecord{ClientID: "c1", InfoHash: "h2", SiteName: "s", TorrentID: "2"}); err != nil {
		t.Fatal(err)
	}
	if err := e.AddSeedingRecord(context.Background(), &model.SeedingTorrentRecord{ClientID: "c2", InfoHash: "h3", SiteName: "s", TorrentID: "3"}); err != nil {
		t.Fatal(err)
	}

	records, err := e.ListByClient(context.Background(), "c1")
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2, got %d", len(records))
	}
}

func setupEngineTestDBFull(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&model.SeedingTorrentRecord{},
		&model.SeedingClientConfig{},
		&model.DownloaderSpeedSnapshot{},
		&model.SiteTrafficDaily{},
		&model.DeleteRule{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestEngine_StartWithRecords(t *testing.T) {
	db := setupEngineTestDB(t)
	ctx := context.Background()

	db.Create(&model.SeedingTorrentRecord{
		ClientID:  "c1",
		InfoHash:  "hash1",
		SiteName:  "site1",
		TorrentID: "100",
		Status:    model.SeedingStatusSeeding,
	})
	db.Create(&model.SeedingTorrentRecord{
		ClientID:  "c2",
		InfoHash:  "hash2",
		SiteName:  "site2",
		TorrentID: "200",
		Status:    "paused_free_end",
	})

	e := NewEngine(db, zap.NewNop())
	if err := e.Start(ctx); err != nil {
		t.Fatal(err)
	}

	if e.TotalActiveCount() != 1 {
		t.Errorf("expected 1 active, got %d", e.TotalActiveCount())
	}
	_, ok := e.GetRecord("c1", "hash1")
	if !ok {
		t.Error("c1:hash1 should exist in recordMap")
	}
	_, ok = e.GetRecord("c2", "hash2")
	if !ok {
		t.Error("c2:hash2 should exist in recordMap")
	}
}

func TestEngine_TotalActiveCount(t *testing.T) {
	db := setupEngineTestDB(t)
	e := NewEngine(db, zap.NewNop())
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	if err := e.AddSeedingRecord(context.Background(), &model.SeedingTorrentRecord{ClientID: "c1", InfoHash: "h1", SiteName: "s", TorrentID: "1"}); err != nil {
		t.Fatal(err)
	}
	if err := e.AddSeedingRecord(context.Background(), &model.SeedingTorrentRecord{ClientID: "c1", InfoHash: "h2", SiteName: "s", TorrentID: "2"}); err != nil {
		t.Fatal(err)
	}
	if err := e.AddSeedingRecord(context.Background(), &model.SeedingTorrentRecord{ClientID: "c2", InfoHash: "h3", SiteName: "s", TorrentID: "3"}); err != nil {
		t.Fatal(err)
	}

	if e.TotalActiveCount() != 3 {
		t.Errorf("expected 3, got %d", e.TotalActiveCount())
	}
}

func TestEngine_PauseForFreeEnd(t *testing.T) {
	db := setupEngineTestDB(t)
	e := NewEngine(db, zap.NewNop())
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	if err := e.AddSeedingRecord(context.Background(), &model.SeedingTorrentRecord{ClientID: "c1", InfoHash: "h1", SiteName: "s", TorrentID: "1"}); err != nil {
		t.Fatal(err)
	}

	if err := e.PauseForFreeEnd(context.Background(), "c1", "h1"); err != nil {
		t.Fatal(err)
	}

	rec, ok := e.GetRecord("c1", "h1")
	if !ok {
		t.Fatal("record should exist")
	}
	if rec.Status != model.SeedingStatusPausedFreeEnd {
		t.Errorf("expected paused_free_end, got %s", rec.Status)
	}
}

func TestEngine_CleanupStale_DeletesOld(t *testing.T) {
	db := setupEngineTestDB(t)
	ctx := context.Background()

	rec := &model.SeedingTorrentRecord{
		ClientID:  "c1",
		InfoHash:  "h1",
		SiteName:  "s",
		TorrentID: "1",
		Status:    "paused_free_end",
	}
	db.Create(rec)
	db.Model(rec).Update("updated_at", time.Now().AddDate(0, 0, -31))

	db.Create(&model.SeedingTorrentRecord{
		ClientID:  "c2",
		InfoHash:  "h2",
		SiteName:  "s",
		TorrentID: "2",
		Status:    model.SeedingStatusSeeding,
	})

	e := NewEngine(db, zap.NewNop())
	if err := e.Start(ctx); err != nil {
		t.Fatal(err)
	}

	deleted, err := e.CleanupStale(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 deleted, got %d", deleted)
	}

	var count int64
	db.Model(&model.SeedingTorrentRecord{}).Where("status = ?", model.SeedingStatusSeeding).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 remaining, got %d", count)
	}
}

func TestEngine_CleanupStale_PausesFreeExpired(t *testing.T) {
	db := setupEngineTestDB(t)
	ctx := context.Background()

	past := time.Now().Add(-1 * time.Hour)
	db.Create(&model.SeedingTorrentRecord{
		ClientID:  "c1",
		InfoHash:  "h1",
		SiteName:  "s",
		TorrentID: "1",
		Status:    model.SeedingStatusSeeding,
		IsFree:    true,
		FreeEndAt: &past,
	})

	e := NewEngine(db, zap.NewNop())
	if err := e.Start(ctx); err != nil {
		t.Fatal(err)
	}

	var updated model.SeedingTorrentRecord
	db.Where("info_hash = ?", "h1").First(&updated)
	if updated.Status != model.SeedingStatusPausedFreeEnd {
		t.Errorf("expected paused_free_end (via RecoverOnStartup), got %s", updated.Status)
	}
}

func TestEngine_OnTorrents(t *testing.T) {
	db := setupEngineTestDB(t)
	e := NewEngine(db, zap.NewNop())
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	events := []model.TorrentEvent{
		{
			SourceID:  "client-1",
			SiteName:  "site1",
			TorrentID: "42",
			InfoHash:  "abc123",
			Discount:  model.DiscountFree,
		},
	}

	if err := e.OnTorrents(context.Background(), events); err != nil {
		t.Fatal(err)
	}

	rec, ok := e.GetRecord("client-1", "abc123")
	if !ok {
		t.Fatal("record should exist")
	}
	if !rec.IsFree {
		t.Error("record should be free")
	}
	if rec.SiteName != "site1" {
		t.Errorf("expected site1, got %s", rec.SiteName)
	}
}

func TestEngine_OnTorrents_NoSourceID(t *testing.T) {
	db := setupEngineTestDB(t)
	e := NewEngine(db, zap.NewNop())
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	events := []model.TorrentEvent{
		{
			SourceID:  "",
			SiteName:  "site1",
			TorrentID: "42",
			InfoHash:  "abc123",
		},
	}

	if err := e.OnTorrents(context.Background(), events); err != nil {
		t.Fatal(err)
	}

	if e.TotalActiveCount() != 0 {
		t.Error("no records should be created for empty SourceID")
	}
}

func TestEngine_CollectTrafficStats_NoProvider(t *testing.T) {
	db := setupEngineTestDB(t)
	e := NewEngine(db, zap.NewNop())

	err := e.CollectTrafficStats(context.Background())
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestEngine_Evaluate_NoProvider(t *testing.T) {
	db := setupEngineTestDB(t)
	e := NewEngine(db, zap.NewNop())

	result, err := e.Evaluate(context.Background(), "c1", nil)
	if err == nil {
		t.Error("expected error")
	}
	if result != nil {
		t.Error("expected nil result")
	}
}

func TestEngine_ListConfigs(t *testing.T) {
	db := setupEngineTestDBFull(t)
	ctx := context.Background()

	db.Create(&model.SeedingClientConfig{
		ClientID: "c1",
		Enabled:  true,
	})
	db.Exec("INSERT INTO seeding_client_configs (client_id, enabled, created_at, updated_at) VALUES (?, false, datetime('now'), datetime('now'))", "c2")

	e := NewEngine(db, zap.NewNop())
	configs, err := e.ListConfigs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 1 {
		t.Fatalf("expected 1, got %d", len(configs))
	}
	if configs[0].ClientID != "c1" {
		t.Errorf("expected c1, got %s", configs[0].ClientID)
	}
}
