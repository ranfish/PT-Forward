package seeding

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type mockDownloaderClient struct {
	maindata *model.Maindata
	err      error
	seeds    []*model.TorrentInfo
	seedErr  error
	delErr   error
	pauseErr error
}

func (m *mockDownloaderClient) GetMainData(_ context.Context) (*model.Maindata, error) {
	return m.maindata, m.err
}
func (m *mockDownloaderClient) GetTorrentList(_ context.Context) ([]model.TorrentInfo, error) {
	return nil, nil
}
func (m *mockDownloaderClient) AddTorrent(_ context.Context, _ string, _ string, _ string, _ []string) error {
	return nil
}
func (m *mockDownloaderClient) AddFromFile(_ context.Context, _ []byte, _ model.AddTorrentOptions) (*model.AddResult, error) {
	return nil, nil
}
func (m *mockDownloaderClient) ExportTorrent(_ context.Context, _ string) ([]byte, error) {
	return nil, nil
}
func (m *mockDownloaderClient) GetTorrentByHash(_ context.Context, _ string) (*model.TorrentInfo, error) {
	return nil, nil
}
func (m *mockDownloaderClient) GetSeedingTorrents(_ context.Context) ([]*model.TorrentInfo, error) {
	return m.seeds, m.seedErr
}
func (m *mockDownloaderClient) GetTorrentsByPath(_ context.Context, _ string) ([]*model.TorrentInfo, error) {
	return nil, nil
}
func (m *mockDownloaderClient) DeleteTorrent(_ context.Context, _ string, _ bool) error {
	return m.delErr
}
func (m *mockDownloaderClient) BatchDeleteTorrents(_ context.Context, _ []string, _ bool) error {
	return nil
}
func (m *mockDownloaderClient) PauseTorrent(_ context.Context, _ string) error  { return m.pauseErr }
func (m *mockDownloaderClient) ResumeTorrent(_ context.Context, _ string) error { return nil }
func (m *mockDownloaderClient) Reannounce(_ context.Context, _ string) error    { return nil }
func (m *mockDownloaderClient) Recheck(_ context.Context, _ string) error       { return nil }
func (m *mockDownloaderClient) SetTorrentTags(_ context.Context, _ string, _ []string) error {
	return nil
}
func (m *mockDownloaderClient) RemoveTorrentTags(_ context.Context, _ string, _ []string) error {
	return nil
}
func (m *mockDownloaderClient) SetCategory(_ context.Context, _ string, _ string) error   { return nil }
func (m *mockDownloaderClient) SetSavePath(_ context.Context, _ string, _ string) error   { return nil }
func (m *mockDownloaderClient) SetSuperSeeding(_ context.Context, _ string, _ bool) error { return nil }
func (m *mockDownloaderClient) SetUploadLimit(_ context.Context, _ string, _ int64) error { return nil }
func (m *mockDownloaderClient) PauseAllDownloads(_ context.Context) error                 { return nil }
func (m *mockDownloaderClient) ResumeAllDownloads(_ context.Context) error                { return nil }
func (m *mockDownloaderClient) GetFreeSpace(_ context.Context) (int64, error)             { return 0, nil }
func (m *mockDownloaderClient) CheckExists(_ context.Context, _ string) (bool, error) {
	return false, nil
}
func (m *mockDownloaderClient) GetMainDataIncremental(_ context.Context, _ int) (*model.Maindata, int, error) {
	return nil, 0, nil
}
func (m *mockDownloaderClient) GetName() string                           { return "mock" }
func (m *mockDownloaderClient) GetRole() string                           { return "" }
func (m *mockDownloaderClient) GetReseedTargetID() string                 { return "" }
func (m *mockDownloaderClient) GetID() uint                               { return 0 }
func (m *mockDownloaderClient) GetSharedPaths() []model.SharedPathMapping { return nil }

type mockDownloaderProvider struct {
	clients map[string]*mockDownloaderClient
	list    []string
}

func (p *mockDownloaderProvider) Get(id string) (model.DownloaderClient, error) {
	c, ok := p.clients[id]
	if !ok {
		return nil, fmt.Errorf("not found: %s", id)
	}
	return c, nil
}
func (p *mockDownloaderProvider) ListClients() []string { return p.list }

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

func TestEngine_SetProviders(t *testing.T) {
	db := setupEngineTestDB(t)
	e := NewEngine(db, zap.NewNop())
	e.SetClientProvider(nil)
	e.SetSiteProvider(nil)
}

func TestEngine_Stop(t *testing.T) {
	db := setupEngineTestDB(t)
	e := NewEngine(db, zap.NewNop())
	if err := e.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestEngine_GetConfigByID(t *testing.T) {
	db := setupEngineTestDBFull(t)
	ctx := context.Background()

	e := NewEngine(db, zap.NewNop())
	cfg := &model.SeedingClientConfig{ClientID: "c1", Enabled: true}
	if err := e.CreateConfig(ctx, cfg); err != nil {
		t.Fatal(err)
	}

	got, err := e.GetConfigByID(ctx, cfg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ClientID != "c1" {
		t.Errorf("expected c1, got %s", got.ClientID)
	}
}

func TestEngine_UpdateConfig(t *testing.T) {
	db := setupEngineTestDBFull(t)
	ctx := context.Background()

	e := NewEngine(db, zap.NewNop())
	cfg := &model.SeedingClientConfig{ClientID: "c1", Enabled: true}
	if err := e.CreateConfig(ctx, cfg); err != nil {
		t.Fatal(err)
	}

	cfg.Enabled = false
	if err := e.UpdateConfig(ctx, cfg); err != nil {
		t.Fatal(err)
	}

	got, _ := e.GetConfigByID(ctx, cfg.ID)
	if got.Enabled {
		t.Error("expected enabled=false")
	}
}

func TestEngine_DeleteConfig(t *testing.T) {
	db := setupEngineTestDBFull(t)
	ctx := context.Background()

	e := NewEngine(db, zap.NewNop())
	cfg := &model.SeedingClientConfig{ClientID: "c1", Enabled: true}
	if err := e.CreateConfig(ctx, cfg); err != nil {
		t.Fatal(err)
	}

	if err := e.DeleteConfig(ctx, cfg.ID); err != nil {
		t.Fatal(err)
	}

	_, err := e.GetConfigByID(ctx, cfg.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestEngine_UpdateStatus(t *testing.T) {
	db := setupEngineTestDBFull(t)
	ctx := context.Background()
	e := NewEngine(db, zap.NewNop())
	if err := e.Start(ctx); err != nil {
		t.Fatal(err)
	}

	rec := &model.SeedingTorrentRecord{
		ClientID: "c1", InfoHash: "hash1", SiteName: "s1", TorrentID: "1",
		Status: model.SeedingStatusSeeding,
	}
	if err := e.AddSeedingRecord(ctx, rec); err != nil {
		t.Fatal(err)
	}

	if err := e.UpdateStatus(ctx, rec.ID, model.SeedingStatusPausedRule, "test"); err != nil {
		t.Fatal(err)
	}

	var got model.SeedingTorrentRecord
	db.First(&got, rec.ID)
	if got.Status != model.SeedingStatusPausedRule {
		t.Errorf("expected paused_rule, got %s", got.Status)
	}
}

func TestEngine_FlushAndClear(t *testing.T) {
	db := setupEngineTestDBFull(t)
	ctx := context.Background()
	e := NewEngine(db, zap.NewNop())

	_, err := e.Flush(ctx, "sub1")
	if err != nil {
		t.Fatalf("flush with no data should not error: %v", err)
	}

	if err := e.Clear(ctx, "c1"); err != nil {
		t.Fatalf("clear with no data should not error: %v", err)
	}
}

func TestEngine_Add(t *testing.T) {
	db := setupEngineTestDBFull(t)
	ctx := context.Background()
	e := NewEngine(db, zap.NewNop())

	event := &model.TorrentEvent{
		SiteName:    "hdhome.org",
		TorrentID:   "42",
		InfoHash:    "abc123",
		Discount:    model.DiscountFree,
		HasHR:       false,
		HRSeedTimeH: 0,
	}

	if err := e.Add(ctx, "client-1", event); err != nil {
		t.Fatal(err)
	}

	var count int64
	db.Model(&model.SeedingTorrentRecord{}).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 record, got %d", count)
	}
}

func TestEngine_ApplyConfig(t *testing.T) {
	db := setupEngineTestDBFull(t)
	e := NewEngine(db, zap.NewNop())

	t.Run("empty weights string", func(t *testing.T) {
		cfg := &model.SeedingClientConfig{CleanupScoreWeights: ""}
		score, age, weights := e.applyConfig(cfg, 0.3, 48.0, DefaultCleanupWeights())
		if score != 0.3 || age != 48.0 {
			t.Errorf("defaults should be preserved: score=%v, age=%v", score, age)
		}
		_ = weights
	})

	t.Run("valid weights", func(t *testing.T) {
		cfg := &model.SeedingClientConfig{
			CleanupScoreWeights: `{"seed_hours":0.5,"upload_speed":0.3,"ratio":0.1,"disk_usage":0.1}`,
		}
		_, _, weights := e.applyConfig(cfg, 0.3, 48.0, DefaultCleanupWeights())
		if weights.SeedHours != 0.5 {
			t.Errorf("expected 0.5, got %v", weights.SeedHours)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		cfg := &model.SeedingClientConfig{CleanupScoreWeights: "invalid"}
		_, _, weights := e.applyConfig(cfg, 0.3, 48.0, DefaultCleanupWeights())
		if weights.SeedHours != 0.3 {
			t.Errorf("should keep defaults on invalid json, got %v", weights.SeedHours)
		}
	})
}

func setupEngineTestDBAll(t *testing.T) *gorm.DB {
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
		&model.SeedingClientState{},
		&model.ScoringLog{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestEngine_CollectTrafficStats(t *testing.T) {
	db := setupEngineTestDBAll(t)
	ctx := context.Background()
	e := NewEngine(db, zap.NewNop())

	mc := &mockDownloaderClient{
		maindata: &model.Maindata{
			Torrents: map[string]model.TorrentInfo{
				"h1": {UploadSpeed: 3000, DownloadSpeed: 500},
			},
			FreeSpace: 1024,
		},
	}
	e.SetClientProvider(&mockDownloaderProvider{
		clients: map[string]*mockDownloaderClient{"c1": mc},
		list:    []string{"c1"},
	})

	db.Create(&model.SeedingTorrentRecord{
		ClientID:  "c1",
		InfoHash:  "h1",
		SiteName:  "site1",
		TorrentID: "1",
		Status:    model.SeedingStatusSeeding,
	})

	if err := e.CollectTrafficStats(ctx); err != nil {
		t.Fatal(err)
	}

	var snap model.DownloaderSpeedSnapshot
	db.Where("client_id = ?", "c1").First(&snap)
	if snap.UploadSpeed != 3000 {
		t.Errorf("expected 3000, got %d", snap.UploadSpeed)
	}

	var traffic model.SiteTrafficDaily
	db.Where("site_name = ?", "site1").First(&traffic)
	if traffic.SeedingCount != 1 {
		t.Errorf("expected 1, got %d", traffic.SeedingCount)
	}
}

func TestEngine_CollectTrafficStats_ClientError(t *testing.T) {
	db := setupEngineTestDBAll(t)
	ctx := context.Background()
	e := NewEngine(db, zap.NewNop())

	e.SetClientProvider(&mockDownloaderProvider{
		clients: map[string]*mockDownloaderClient{},
		list:    []string{"c1"},
	})

	if err := e.CollectTrafficStats(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestEngine_CollectTrafficStats_GetMainDataError(t *testing.T) {
	db := setupEngineTestDBAll(t)
	ctx := context.Background()
	e := NewEngine(db, zap.NewNop())

	mc := &mockDownloaderClient{err: fmt.Errorf("connection refused")}
	e.SetClientProvider(&mockDownloaderProvider{
		clients: map[string]*mockDownloaderClient{"c1": mc},
		list:    []string{"c1"},
	})

	if err := e.CollectTrafficStats(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestEngine_CollectSiteTrafficDaily_Update(t *testing.T) {
	db := setupEngineTestDBAll(t)
	ctx := context.Background()
	now := time.Now()
	today := now.Truncate(24 * time.Hour)

	e := NewEngine(db, zap.NewNop())

	db.Create(&model.SeedingTorrentRecord{
		ClientID:  "c1",
		InfoHash:  "h1",
		SiteName:  "site1",
		TorrentID: "1",
		Status:    model.SeedingStatusSeeding,
	})

	db.Create(&model.SiteTrafficDaily{
		SiteName:     "site1",
		Date:         today,
		SeedingCount: 5,
		TorrentCount: 5,
	})

	md := &model.Maindata{
		Torrents: map[string]model.TorrentInfo{},
	}
	e.collectSiteTrafficDaily(ctx, "c1", md, now)

	var traffic model.SiteTrafficDaily
	db.Where("site_name = ? AND date = ?", "site1", today).First(&traffic)
	if traffic.SeedingCount != 1 {
		t.Errorf("expected 1 (updated), got %d", traffic.SeedingCount)
	}
}

func TestEngine_UpdateEMA_Initialize(t *testing.T) {
	db := setupEngineTestDBAll(t)
	ctx := context.Background()
	e := NewEngine(db, zap.NewNop())

	torrentMap := map[string]*model.TorrentInfo{
		"h1": {UploadSpeed: 2000, DownloadSpeed: 1000},
	}
	md := &model.Maindata{Torrents: map[string]model.TorrentInfo{"h1": {UploadSpeed: 2000}}}

	e.updateEMA(ctx, "c1", md, torrentMap)

	state, ok := e.emaStates["c1"]
	if !ok {
		t.Fatal("ema state should exist")
	}
	if state.UploadSpeed != 2000 {
		t.Errorf("expected 2000, got %v", state.UploadSpeed)
	}

	var dbState model.SeedingClientState
	db.Where("client_id = ?", "c1").First(&dbState)
	if !dbState.Initialized {
		t.Error("should be initialized")
	}
}

func TestEngine_UpdateEMA_Exponential(t *testing.T) {
	db := setupEngineTestDBAll(t)
	ctx := context.Background()
	e := NewEngine(db, zap.NewNop())

	e.emaStates["c1"] = &emaState{UploadSpeed: 1000, DownloadSpeed: 500}

	torrentMap := map[string]*model.TorrentInfo{
		"h1": {UploadSpeed: 2000, DownloadSpeed: 1000},
	}
	md := &model.Maindata{}

	e.updateEMA(ctx, "c1", md, torrentMap)

	state := e.emaStates["c1"]
	expected := 0.3*2000 + 0.7*1000
	if state.UploadSpeed != expected {
		t.Errorf("expected %v, got %v", expected, state.UploadSpeed)
	}
}

func TestEngine_UpdateEMA_UpdateExisting(t *testing.T) {
	db := setupEngineTestDBAll(t)
	ctx := context.Background()

	db.Create(&model.SeedingClientState{
		ClientID: "c1", AvgUploadSpeed: 500, AvgDownloadSpeed: 200, Initialized: true,
	})

	e := NewEngine(db, zap.NewNop())

	torrentMap := map[string]*model.TorrentInfo{
		"h1": {UploadSpeed: 1000, DownloadSpeed: 400},
	}
	md := &model.Maindata{}

	e.updateEMA(ctx, "c1", md, torrentMap)

	var dbState model.SeedingClientState
	db.Where("client_id = ?", "c1").First(&dbState)
	if dbState.AvgUploadSpeed == 500 {
		t.Error("upload speed should have been updated")
	}
}

func TestFreeWaitMonitor_SetEngine(t *testing.T) {
	db := setupEngineTestDBAll(t)
	m := NewFreeWaitMonitor(db, zap.NewNop())
	e := NewEngine(db, zap.NewNop())
	m.SetEngine(e)
	if m.engine != e {
		t.Error("engine should be set")
	}
}

func TestEngine_Evaluate_NoRecords(t *testing.T) {
	db := setupEngineTestDBAll(t)
	ctx := context.Background()
	e := NewEngine(db, zap.NewNop())

	mc := &mockDownloaderClient{}
	e.SetClientProvider(&mockDownloaderProvider{
		clients: map[string]*mockDownloaderClient{"c1": mc},
		list:    []string{"c1"},
	})

	result, err := e.Evaluate(ctx, "c1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Evaluated != 0 {
		t.Errorf("expected 0, got %d", result.Evaluated)
	}
}

func TestEngine_Evaluate_NoMatchingTorrent(t *testing.T) {
	db := setupEngineTestDBAll(t)
	ctx := context.Background()
	e := NewEngine(db, zap.NewNop())
	_ = e.Start(ctx)

	db.Create(&model.SeedingTorrentRecord{
		ClientID: "c1", InfoHash: "hash1", SiteName: "s1", TorrentID: "1",
		Status: model.SeedingStatusSeeding,
	})

	mc := &mockDownloaderClient{
		seeds: []*model.TorrentInfo{
			{Hash: "other_hash", UploadSpeed: 100},
		},
	}
	e.SetClientProvider(&mockDownloaderProvider{
		clients: map[string]*mockDownloaderClient{"c1": mc},
		list:    []string{"c1"},
	})

	result, err := e.Evaluate(ctx, "c1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Evaluated != 1 {
		t.Errorf("expected 1 evaluated, got %d", result.Evaluated)
	}
	if result.Deleted != 0 {
		t.Errorf("expected 0 deleted (no matching torrent), got %d", result.Deleted)
	}
}

func TestEngine_Evaluate_ClientNotFound(t *testing.T) {
	db := setupEngineTestDBAll(t)
	ctx := context.Background()
	e := NewEngine(db, zap.NewNop())

	e.SetClientProvider(&mockDownloaderProvider{
		clients: map[string]*mockDownloaderClient{},
		list:    []string{},
	})

	_, err := e.Evaluate(ctx, "missing", nil)
	if err == nil {
		t.Error("expected error for missing client")
	}
}

func TestEngine_Evaluate_GetSeedsError(t *testing.T) {
	db := setupEngineTestDBAll(t)
	ctx := context.Background()
	e := NewEngine(db, zap.NewNop())
	_ = e.Start(ctx)

	db.Create(&model.SeedingTorrentRecord{
		ClientID: "c1", InfoHash: "hash1", SiteName: "s1", TorrentID: "1",
		Status: model.SeedingStatusSeeding,
	})

	mc := &mockDownloaderClient{seedErr: fmt.Errorf("connection refused")}
	e.SetClientProvider(&mockDownloaderProvider{
		clients: map[string]*mockDownloaderClient{"c1": mc},
		list:    []string{"c1"},
	})

	_, err := e.Evaluate(ctx, "c1", nil)
	if err == nil {
		t.Error("expected error")
	}
}

func TestEngine_Evaluate_DiskProtection(t *testing.T) {
	db := setupEngineTestDBAll(t)
	ctx := context.Background()
	e := NewEngine(db, zap.NewNop())
	_ = e.Start(ctx)

	db.Create(&model.SeedingTorrentRecord{
		ClientID:  "c1",
		InfoHash:  "hash1",
		SiteName:  "s1",
		TorrentID: "1",
		Status:    model.SeedingStatusSeeding,
		CreatedAt: time.Now().Add(-100 * time.Hour),
	})

	mc := &mockDownloaderClient{
		maindata: &model.Maindata{FreeSpace: 100},
		seeds: []*model.TorrentInfo{
			{Hash: "hash1", UploadSpeed: 10000000, SeedTime: 360000},
		},
	}
	e.SetClientProvider(&mockDownloaderProvider{
		clients: map[string]*mockDownloaderClient{"c1": mc},
		list:    []string{"c1"},
	})

	cfg := &model.SeedingClientConfig{
		DiskProtectEnabled: true,
		MinDiskSpaceGB:     10,
	}
	result, err := e.Evaluate(ctx, "c1", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if result.Paused != 1 {
		t.Errorf("expected 1 paused (disk protect), got %d", result.Paused)
	}
}

func TestEngine_Evaluate_DeleteTorrent(t *testing.T) {
	db := setupEngineTestDBAll(t)
	ctx := context.Background()
	e := NewEngine(db, zap.NewNop())
	_ = e.Start(ctx)

	past := time.Now().Add(-200 * time.Hour)
	db.Create(&model.SeedingTorrentRecord{
		ClientID:  "c1",
		InfoHash:  "hash1",
		SiteName:  "s1",
		TorrentID: "1",
		Status:    model.SeedingStatusSeeding,
		IsFree:    false,
		HasHR:     false,
		CreatedAt: past,
	})

	mc := &mockDownloaderClient{
		maindata: &model.Maindata{FreeSpace: 100 * 1024 * 1024 * 1024},
		seeds: []*model.TorrentInfo{
			{Hash: "hash1", UploadSpeed: 0, SeedTime: 720000, Ratio: 5.0, TotalSize: 1024},
		},
	}
	e.SetClientProvider(&mockDownloaderProvider{
		clients: map[string]*mockDownloaderClient{"c1": mc},
		list:    []string{"c1"},
	})

	result, err := e.Evaluate(ctx, "c1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Deleted < 1 {
		t.Errorf("expected at least 1 deleted, got %d (evaluated=%d)", result.Deleted, result.Evaluated)
	}
}
