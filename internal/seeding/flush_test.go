package seeding

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/mocks"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type flushMockClient struct {
	maindata     *model.Maindata
	maindataErr  error
	seeds        []*model.TorrentInfo
	addResult    *model.AddResult
	addResultFn  func() *model.AddResult
	addErr       error
	existsMap    map[string]bool
	addCallCount int
}

func (m *flushMockClient) GetName() string                           { return "c1" }
func (m *flushMockClient) GetRole() string                           { return "seeding" }
func (m *flushMockClient) GetReseedTargetID() string                 { return "" }
func (m *flushMockClient) GetID() uint                               { return 1 }
func (m *flushMockClient) GetSharedPaths() []model.SharedPathMapping { return nil }
func (m *flushMockClient) GetMainData(_ context.Context) (*model.Maindata, error) {
	return m.maindata, m.maindataErr
}
func (m *flushMockClient) GetMainDataIncremental(_ context.Context, _ int) (*model.Maindata, int, error) {
	return nil, 0, nil
}
func (m *flushMockClient) GetTorrentByHash(_ context.Context, _ string) (*model.TorrentInfo, error) {
	return nil, nil
}
func (m *flushMockClient) GetSeedingTorrents(_ context.Context) ([]*model.TorrentInfo, error) {
	return m.seeds, nil
}
func (m *flushMockClient) GetAllTorrents(_ context.Context) ([]*model.TorrentInfo, error) {
	return m.seeds, nil
}
func (m *flushMockClient) GetTorrentsByPath(_ context.Context, _ string) ([]*model.TorrentInfo, error) {
	return nil, nil
}
func (m *flushMockClient) AddFromFile(_ context.Context, _ []byte, _ model.AddTorrentOptions) (*model.AddResult, error) {
	m.addCallCount++
	if m.addResultFn != nil {
		return m.addResultFn(), m.addErr
	}
	return m.addResult, m.addErr
}
func (m *flushMockClient) ExportTorrent(_ context.Context, _ string) ([]byte, error) { return nil, nil }
func (m *flushMockClient) DeleteTorrent(_ context.Context, _ string, _ bool) error   { return nil }
func (m *flushMockClient) BatchDeleteTorrents(_ context.Context, _ []string, _ bool) error {
	return nil
}
func (m *flushMockClient) PauseTorrent(_ context.Context, _ string) error  { return nil }
func (m *flushMockClient) ResumeTorrent(_ context.Context, _ string) error { return nil }
func (m *flushMockClient) Reannounce(_ context.Context, _ string) error    { return nil }
func (m *flushMockClient) Recheck(_ context.Context, _ string) error       { return nil }
func (m *flushMockClient) SetTorrentTags(_ context.Context, _ string, _ []string) error {
	return nil
}
func (m *flushMockClient) RemoveTorrentTags(_ context.Context, _ string, _ []string) error {
	return nil
}
func (m *flushMockClient) SetCategory(_ context.Context, _ string, _ string) error   { return nil }
func (m *flushMockClient) SetSavePath(_ context.Context, _ string, _ string) error   { return nil }
func (m *flushMockClient) SetSuperSeeding(_ context.Context, _ string, _ bool) error { return nil }
func (m *flushMockClient) SetUploadLimit(_ context.Context, _ string, _ int64) error { return nil }
func (m *flushMockClient) PauseAllDownloads(_ context.Context) error                 { return nil }
func (m *flushMockClient) ResumeAllDownloads(_ context.Context) error                { return nil }
func (m *flushMockClient) GetFreeSpace(_ context.Context) (int64, error)             { return 0, nil }
func (m *flushMockClient) CheckExists(_ context.Context, hash string) (bool, error) {
	if m.existsMap == nil {
		return false, nil
	}
	return m.existsMap[hash], nil
}
func (m *flushMockClient) GetGlobalTransferStats(_ context.Context) (*model.GlobalTransferStats, error) {
	return &model.GlobalTransferStats{}, nil
}
func (m *flushMockClient) GetTrackerMessages(_ context.Context, _ string) (string, error) {
	return "", nil
}

type flushMockProvider struct {
	clients map[string]model.DownloaderClient
	list    []string
}

func (p *flushMockProvider) Get(id string) (model.DownloaderClient, error) {
	c, ok := p.clients[id]
	if !ok {
		return nil, fmt.Errorf("not found: %s", id)
	}
	return c, nil
}
func (p *flushMockProvider) ListClients() []string { return p.list }

func newMockSiteProvider() *mocks.SiteInfoProvider {
	return &mocks.SiteInfoProvider{
		GetSiteInfoFn: func(_ context.Context, siteName string) (*model.SiteInfo, error) {
			return &model.SiteInfo{Name: siteName}, nil
		},
		GetSiteConfigFn: func(_ context.Context, _ string) (*model.SiteConfig, error) {
			return &model.SiteConfig{}, nil
		},
		GetAdapterFn: func(_ context.Context, _ string) (model.SiteAdapter, error) {
			return &mocks.SiteAdapter{
				DownloadTorrentFn: func(_ context.Context, _ *model.SiteConfig, _ string) ([]byte, error) {
					return []byte("mock-torrent-data"), nil
				},
				DetectDiscountFn: func(_ context.Context, _ *model.SiteConfig, _ string) (*model.DiscountResult, error) {
					return &model.DiscountResult{Level: model.DiscountFree}, nil
				},
			}, nil
		},
	}
}

func setupFlushTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(uniqueSQLiteDSN()), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&model.SeedingTorrentRecord{},
		&model.SeedingClientConfig{},
		&model.RSSSubscription{},
		&model.Site{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func seedSubscription(t *testing.T, db *gorm.DB, scoringEnabled bool, include2xUp bool) uint {
	t.Helper()
	sub := &model.RSSSubscription{
		Name:     "test-sub",
		Enabled:  true,
		SiteName: "site1",
		ClientID: "c1",
		ScoringConfig: model.SeedingScoringConfig{
			Enabled:          scoringEnabled,
			MaxCandidates:    50,
			MaxActiveSeeding: 100,
			BatchLimit:       10,
			MinScore:         -1,
			Include2xUp:      include2xUp,
			HalfLifeHours:    2.0,
		},
	}
	if err := db.Create(sub).Error; err != nil {
		t.Fatalf("create sub: %v", err)
	}

	db.Model(sub).Update("min_score", 0.0)

	site := &model.Site{
		Name:   "site1",
		Domain: "site1.com",
	}
	if err := db.Create(site).Error; err != nil {
		t.Fatalf("create site: %v", err)
	}

	return sub.ID
}

func TestFlush_ScoringNotEnabled(t *testing.T) {
	db := setupFlushTestDB(t)
	e := NewEngine(db, zap.NewNop())
	ctx := context.Background()

	subID := seedSubscription(t, db, false, false)

	candidates, err := e.Flush(ctx, fmt.Sprintf("%d", subID))
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates, got %d", len(candidates))
	}
}

func TestFlush_InvalidSubscriptionID(t *testing.T) {
	db := setupFlushTestDB(t)
	e := NewEngine(db, zap.NewNop())

	candidates, err := e.Flush(context.Background(), "not-a-number")
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates, got %d", len(candidates))
	}
}

func TestFlush_SubscriptionNotFound(t *testing.T) {
	db := setupFlushTestDB(t)
	e := NewEngine(db, zap.NewNop())

	candidates, err := e.Flush(context.Background(), "99999")
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates, got %d", len(candidates))
	}
}

func TestFlush_NoProvider(t *testing.T) {
	db := setupFlushTestDB(t)
	e := NewEngine(db, zap.NewNop())
	ctx := context.Background()

	subID := seedSubscription(t, db, true, false)
	_ = e.Start(ctx)

	candidates, err := e.Flush(ctx, fmt.Sprintf("%d", subID))
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates without provider, got %d", len(candidates))
	}
}

func TestFlush_NoClient(t *testing.T) {
	db := setupFlushTestDB(t)
	e := NewEngine(db, zap.NewNop())
	ctx := context.Background()

	subID := seedSubscription(t, db, true, false)
	_ = e.Start(ctx)

	e.SetClientProvider(&flushMockProvider{
		clients: map[string]model.DownloaderClient{},
		list:    []string{},
	})

	candidates, err := e.Flush(ctx, fmt.Sprintf("%d", subID))
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates without client, got %d", len(candidates))
	}
}

func TestFlush_NoRecords(t *testing.T) {
	db := setupFlushTestDB(t)
	e := NewEngine(db, zap.NewNop())
	ctx := context.Background()

	subID := seedSubscription(t, db, true, false)
	_ = e.Start(ctx)

	mc := &flushMockClient{
		maindata: &model.Maindata{FreeSpace: 100 * 1024 * 1024 * 1024},
	}
	e.SetClientProvider(&flushMockProvider{
		clients: map[string]model.DownloaderClient{"c1": mc},
		list:    []string{"c1"},
	})

	candidates, err := e.Flush(ctx, fmt.Sprintf("%d", subID))
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates with no records, got %d", len(candidates))
	}
}

func TestFlush_MaxActiveReached(t *testing.T) {
	db := setupFlushTestDB(t)
	e := NewEngine(db, zap.NewNop())
	ctx := context.Background()
	_ = e.Start(ctx)

	sub := &model.RSSSubscription{
		Name:     "test-sub",
		Enabled:  true,
		SiteName: "site1",
		ClientID: "c1",
		ScoringConfig: model.SeedingScoringConfig{
			Enabled:          true,
			MaxActiveSeeding: 0,
		},
	}
	db.Create(sub)
	db.Create(&model.Site{Name: "site1", Domain: "site1.com"})

	mc := &flushMockClient{
		maindata: &model.Maindata{FreeSpace: 100 * 1024 * 1024 * 1024},
	}
	e.SetClientProvider(&flushMockProvider{
		clients: map[string]model.DownloaderClient{"c1": mc},
		list:    []string{"c1"},
	})

	candidates, err := e.Flush(ctx, fmt.Sprintf("%d", sub.ID))
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 0 {
		t.Errorf("expected 0 when max active reached, got %d", len(candidates))
	}
}

func TestFlush_PushOne_HRProtect(t *testing.T) {
	db := setupFlushTestDB(t)
	e := NewEngine(db, zap.NewNop())
	ctx := context.Background()
	_ = e.Start(ctx)

	subID := seedSubscription(t, db, true, false)

	now := time.Now()
	db.Create(&model.SeedingTorrentRecord{
		ClientID:       "c1",
		InfoHash:       "hash1",
		SiteName:       "site1",
		TorrentID:      "42",
		Discount:       model.DiscountFree,
		HasHR:          true,
		HRSeedTimeH:    72,
		Status:         model.SeedingStatusSeeding,
		Source:         "rss",
		IsFree:         true,
		SubscriptionID: fmt.Sprintf("%d", subID),
		CreatedAt:      now,
	})

	mc := &flushMockClient{
		maindata:  &model.Maindata{FreeSpace: 100 * 1024 * 1024 * 1024},
		addResult: &model.AddResult{InfoHash: "hash0"},
	}
	e.SetClientProvider(&flushMockProvider{
		clients: map[string]model.DownloaderClient{"c1": mc},
		list:    []string{"c1"},
	})
	e.SetSiteProvider(newMockSiteProvider())
	e.SetSiteProvider(newMockSiteProvider())

	e.mu.Lock()
	e.recordMap[recordKey("c1", "hash1")] = &model.SeedingTorrentRecord{
		ClientID: "c1", InfoHash: "hash1", Status: model.SeedingStatusSeeding,
		SubscriptionID: fmt.Sprintf("%d", subID),
	}
	e.mu.Unlock()

	candidates, err := e.Flush(ctx, fmt.Sprintf("%d", subID))
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}

	var updated model.SeedingTorrentRecord
	db.Where("info_hash = ?", "hash1").First(&updated)
	if updated.Status != model.SeedingStatusSeeding {
		t.Errorf("expected status seeding (HR skipped, no push), got %s", updated.Status)
	}
}

func TestFlush_PushOne_TorrentExists(t *testing.T) {
	db := setupFlushTestDB(t)
	e := NewEngine(db, zap.NewNop())
	ctx := context.Background()
	_ = e.Start(ctx)

	subID := seedSubscription(t, db, true, false)

	now := time.Now()
	db.Create(&model.SeedingTorrentRecord{
		ClientID:       "c1",
		InfoHash:       "hash_exists",
		SiteName:       "site1",
		TorrentID:      "43",
		Discount:       model.DiscountFree,
		Status:         model.SeedingStatusSeeding,
		Source:         "rss",
		IsFree:         true,
		SubscriptionID: fmt.Sprintf("%d", subID),
		CreatedAt:      now,
	})

	mc := &flushMockClient{
		maindata:  &model.Maindata{FreeSpace: 100 * 1024 * 1024 * 1024},
		existsMap: map[string]bool{"hash_exists": true},
		addResult: &model.AddResult{InfoHash: "hash_exists"},
	}
	e.SetClientProvider(&flushMockProvider{
		clients: map[string]model.DownloaderClient{"c1": mc},
		list:    []string{"c1"},
	})
	e.SetSiteProvider(newMockSiteProvider())

	e.mu.Lock()
	e.recordMap[recordKey("c1", "hash_exists")] = &model.SeedingTorrentRecord{
		ClientID: "c1", InfoHash: "hash_exists", Status: model.SeedingStatusSeeding,
		SubscriptionID: fmt.Sprintf("%d", subID),
	}
	e.mu.Unlock()

	candidates, err := e.Flush(ctx, fmt.Sprintf("%d", subID))
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
}

func TestFlush_CollectsOnlyFree(t *testing.T) {
	db := setupFlushTestDB(t)
	e := NewEngine(db, zap.NewNop())
	ctx := context.Background()
	_ = e.Start(ctx)

	subID := seedSubscription(t, db, true, false)

	now := time.Now()
	db.Create(&model.SeedingTorrentRecord{
		ClientID: "c1", InfoHash: "h1", SiteName: "site1", TorrentID: "1",
		Discount: model.DiscountFree, Status: model.SeedingStatusSeeding,
		Source: "rss", IsFree: true, SubscriptionID: fmt.Sprintf("%d", subID), CreatedAt: now,
	})
	db.Create(&model.SeedingTorrentRecord{
		ClientID: "c1", InfoHash: "h2", SiteName: "site1", TorrentID: "2",
		Discount: model.DiscountNone, Status: model.SeedingStatusSeeding,
		Source: "rss", IsFree: false, SubscriptionID: fmt.Sprintf("%d", subID), CreatedAt: now,
	})
	db.Create(&model.SeedingTorrentRecord{
		ClientID: "c1", InfoHash: "h3", SiteName: "site1", TorrentID: "3",
		Discount: model.Discount2xFree, Status: model.SeedingStatusSeeding,
		Source: "rss", IsFree: true, SubscriptionID: fmt.Sprintf("%d", subID), CreatedAt: now,
	})

	mc := &flushMockClient{
		maindata:  &model.Maindata{FreeSpace: 100 * 1024 * 1024 * 1024},
		addResult: &model.AddResult{InfoHash: "h1"},
	}
	e.SetClientProvider(&flushMockProvider{
		clients: map[string]model.DownloaderClient{"c1": mc},
		list:    []string{"c1"},
	})
	e.SetSiteProvider(newMockSiteProvider())

	for _, hash := range []string{"h1", "h2", "h3"} {
		e.mu.Lock()
		e.recordMap[recordKey("c1", hash)] = &model.SeedingTorrentRecord{
			ClientID: "c1", InfoHash: hash, Status: model.SeedingStatusSeeding,
			SubscriptionID: fmt.Sprintf("%d", subID),
		}
		e.mu.Unlock()
	}

	candidates, err := e.Flush(ctx, fmt.Sprintf("%d", subID))
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 2 {
		var allRecords []model.SeedingTorrentRecord
		db.Where("client_id = ? AND status = ?", "c1", model.SeedingStatusSeeding).Find(&allRecords)
		for _, r := range allRecords {
			t.Logf("record: hash=%s discount=%s isFree=%v source=%q", r.InfoHash, r.Discount, r.IsFree, r.Source)
		}
		t.Errorf("expected 2 free candidates (h1, h3), got %d", len(candidates))
	}
}

func TestFlush_Include2xUp(t *testing.T) {
	db := setupFlushTestDB(t)
	e := NewEngine(db, zap.NewNop())
	ctx := context.Background()
	_ = e.Start(ctx)

	subID := seedSubscription(t, db, true, true)

	now := time.Now()
	db.Create(&model.SeedingTorrentRecord{
		ClientID: "c1", InfoHash: "h1", SiteName: "site1", TorrentID: "1",
		Discount: model.Discount2xUp, Status: model.SeedingStatusSeeding,
		Source: "rss", IsFree: false, SubscriptionID: fmt.Sprintf("%d", subID), CreatedAt: now,
	})
	db.Create(&model.SeedingTorrentRecord{
		ClientID: "c1", InfoHash: "h2", SiteName: "site1", TorrentID: "2",
		Discount: model.DiscountNone, Status: model.SeedingStatusSeeding,
		Source: "rss", IsFree: false, SubscriptionID: fmt.Sprintf("%d", subID), CreatedAt: now,
	})

	mc := &flushMockClient{
		maindata:  &model.Maindata{FreeSpace: 100 * 1024 * 1024 * 1024},
		addResult: &model.AddResult{InfoHash: "h1"},
	}
	e.SetClientProvider(&flushMockProvider{
		clients: map[string]model.DownloaderClient{"c1": mc},
		list:    []string{"c1"},
	})
	e.SetSiteProvider(newMockSiteProvider())

	for _, hash := range []string{"h1", "h2"} {
		e.mu.Lock()
		e.recordMap[recordKey("c1", hash)] = &model.SeedingTorrentRecord{
			ClientID: "c1", InfoHash: hash, Status: model.SeedingStatusSeeding,
		}
		e.mu.Unlock()
	}

	candidates, err := e.Flush(ctx, fmt.Sprintf("%d", subID))
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 {
		t.Errorf("expected 1 candidate (2xup only), got %d", len(candidates))
	}
}

func TestFlush_AssumeFreeSite(t *testing.T) {
	db := setupFlushTestDB(t)
	e := NewEngine(db, zap.NewNop())
	ctx := context.Background()
	_ = e.Start(ctx)

	subID := seedSubscription(t, db, true, false)

	if err := db.Create(&model.Site{
		Domain: "piggo.me", Name: "二师兄", BaseURL: "https://piggo.me",
		Framework: "nexusphp", AssumeFree: true,
	}).Error; err != nil {
		t.Fatalf("seed site: %v", err)
	}

	now := time.Now()
	db.Create(&model.SeedingTorrentRecord{
		ClientID: "c1", InfoHash: "h1", SiteName: "二师兄", TorrentID: "1",
		Discount: model.DiscountNone, Status: model.SeedingStatusPending,
		Source: "rss", IsFree: false, SubscriptionID: fmt.Sprintf("%d", subID), CreatedAt: now,
	})
	db.Create(&model.SeedingTorrentRecord{
		ClientID: "c1", InfoHash: "h2", SiteName: "二师兄", TorrentID: "2",
		Discount: model.DiscountNone, Status: model.SeedingStatusPending,
		Source: "rss", IsFree: false, SubscriptionID: fmt.Sprintf("%d", subID), CreatedAt: now,
	})

	mc := &flushMockClient{
		maindata:  &model.Maindata{FreeSpace: 100 * 1024 * 1024 * 1024},
		addResult: &model.AddResult{InfoHash: "h1"},
	}
	e.SetClientProvider(&flushMockProvider{
		clients: map[string]model.DownloaderClient{"c1": mc},
		list:    []string{"c1"},
	})
	e.SetSiteProvider(newMockSiteProvider())

	for _, hash := range []string{"h1", "h2"} {
		e.mu.Lock()
		e.recordMap[recordKey("c1", hash)] = &model.SeedingTorrentRecord{
			ClientID: "c1", InfoHash: hash, Status: model.SeedingStatusPending,
			SubscriptionID: fmt.Sprintf("%d", subID),
		}
		e.mu.Unlock()
	}

	candidates, err := e.Flush(ctx, fmt.Sprintf("%d", subID))
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 2 {
		t.Errorf("expected 2 candidates (assume_free site), got %d", len(candidates))
	}
}

func TestFlush_AssumeFreeNotSet(t *testing.T) {
	db := setupFlushTestDB(t)
	e := NewEngine(db, zap.NewNop())
	ctx := context.Background()
	_ = e.Start(ctx)

	subID := seedSubscription(t, db, true, false)

	if err := db.Create(&model.Site{
		Domain: "piggo.me", Name: "二师兄", BaseURL: "https://piggo.me",
		Framework: "nexusphp", AssumeFree: false,
	}).Error; err != nil {
		t.Fatalf("seed site: %v", err)
	}

	now := time.Now()
	db.Create(&model.SeedingTorrentRecord{
		ClientID: "c1", InfoHash: "h1", SiteName: "二师兄", TorrentID: "1",
		Discount: model.DiscountNone, Status: model.SeedingStatusPending,
		Source: "rss", IsFree: false, SubscriptionID: fmt.Sprintf("%d", subID), CreatedAt: now,
	})

	mc := &flushMockClient{
		maindata:  &model.Maindata{FreeSpace: 100 * 1024 * 1024 * 1024},
		addResult: &model.AddResult{InfoHash: "h1"},
	}
	e.SetClientProvider(&flushMockProvider{
		clients: map[string]model.DownloaderClient{"c1": mc},
		list:    []string{"c1"},
	})
	e.SetSiteProvider(newMockSiteProvider())

	e.mu.Lock()
	e.recordMap[recordKey("c1", "h1")] = &model.SeedingTorrentRecord{
		ClientID: "c1", InfoHash: "h1", Status: model.SeedingStatusPending,
		SubscriptionID: fmt.Sprintf("%d", subID),
	}
	e.mu.Unlock()

	candidates, err := e.Flush(ctx, fmt.Sprintf("%d", subID))
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates (assume_free not set), got %d", len(candidates))
	}
}

func TestFlush_BatchLimit(t *testing.T) {
	db := setupFlushTestDB(t)
	e := NewEngine(db, zap.NewNop())
	ctx := context.Background()
	_ = e.Start(ctx)

	sub := &model.RSSSubscription{
		Name: "test-sub", Enabled: true, SiteName: "site1", ClientID: "c1",
		ScoringConfig: model.SeedingScoringConfig{
			Enabled: true, MaxCandidates: 50, MaxActiveSeeding: 100,
			BatchLimit: 2, MinScore: 0.0, HalfLifeHours: 2.0,
		},
	}
	db.Create(sub)
	db.Create(&model.Site{Name: "site1", Domain: "site1.com"})

	now := time.Now()
	for i := 0; i < 5; i++ {
		hash := fmt.Sprintf("hash%d", i)
		db.Create(&model.SeedingTorrentRecord{
			ClientID: "c1", InfoHash: hash, SiteName: "site1",
			TorrentID: fmt.Sprintf("%d", i), Discount: model.DiscountFree,
			Status: model.SeedingStatusSeeding, Source: "rss",
			IsFree: true, CreatedAt: now,
		})
		e.mu.Lock()
		e.recordMap[recordKey("c1", hash)] = &model.SeedingTorrentRecord{
			ClientID: "c1", InfoHash: hash, Status: model.SeedingStatusSeeding,
		}
		e.mu.Unlock()
	}

	mc := &flushMockClient{
		maindata:  &model.Maindata{FreeSpace: 100 * 1024 * 1024 * 1024},
		addResult: &model.AddResult{InfoHash: "hash0"},
	}
	e.SetClientProvider(&flushMockProvider{
		clients: map[string]model.DownloaderClient{"c1": mc},
		list:    []string{"c1"},
	})
	e.SetSiteProvider(newMockSiteProvider())

	candidates, err := e.Flush(ctx, fmt.Sprintf("%d", sub.ID))
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) > 2 {
		t.Errorf("expected at most 2 candidates (batch limit), got %d", len(candidates))
	}
}

func TestFlush_Sorting(t *testing.T) {
	db := setupFlushTestDB(t)
	e := NewEngine(db, zap.NewNop())
	ctx := context.Background()
	_ = e.Start(ctx)

	subID := seedSubscription(t, db, true, false)

	now := time.Now()
	oldTime := now.Add(-10 * time.Hour)
	newTime := now.Add(-1 * time.Hour)

	db.Create(&model.SeedingTorrentRecord{
		ClientID: "c1", InfoHash: "old_hash", SiteName: "site1", TorrentID: "1",
		Discount: model.DiscountFree, Status: model.SeedingStatusSeeding,
		Source: "rss", IsFree: true, SubscriptionID: fmt.Sprintf("%d", subID), CreatedAt: oldTime,
	})
	db.Create(&model.SeedingTorrentRecord{
		ClientID: "c1", InfoHash: "new_hash", SiteName: "site1", TorrentID: "2",
		Discount: model.Discount2xFree, Status: model.SeedingStatusSeeding,
		Source: "rss", IsFree: true, SubscriptionID: fmt.Sprintf("%d", subID), CreatedAt: newTime,
	})

	mc := &flushMockClient{
		maindata:  &model.Maindata{FreeSpace: 100 * 1024 * 1024 * 1024},
		addResult: &model.AddResult{InfoHash: "new_hash"},
	}
	e.SetClientProvider(&flushMockProvider{
		clients: map[string]model.DownloaderClient{"c1": mc},
		list:    []string{"c1"},
	})
	e.SetSiteProvider(newMockSiteProvider())

	for _, h := range []string{"old_hash", "new_hash"} {
		e.mu.Lock()
		e.recordMap[recordKey("c1", h)] = &model.SeedingTorrentRecord{
			ClientID: "c1", InfoHash: h, Status: model.SeedingStatusSeeding,
			SubscriptionID: fmt.Sprintf("%d", subID),
		}
		e.mu.Unlock()
	}

	candidates, err := e.Flush(ctx, fmt.Sprintf("%d", subID))
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 2 {
		t.Fatalf("expected 2, got %d", len(candidates))
	}
}

func TestFlush_ContextCancelled(t *testing.T) {
	db := setupFlushTestDB(t)
	e := NewEngine(db, zap.NewNop())
	ctx, cancel := context.WithCancel(context.Background())
	_ = e.Start(ctx)

	subID := seedSubscription(t, db, true, false)

	now := time.Now()
	for i := 0; i < 5; i++ {
		hash := fmt.Sprintf("hash%d", i)
		db.Create(&model.SeedingTorrentRecord{
			ClientID: "c1", InfoHash: hash, SiteName: "site1",
			TorrentID: fmt.Sprintf("%d", i), Discount: model.DiscountFree,
			Status: model.SeedingStatusSeeding, Source: "rss",
			IsFree: true, CreatedAt: now,
		})
		e.mu.Lock()
		e.recordMap[recordKey("c1", hash)] = &model.SeedingTorrentRecord{
			ClientID: "c1", InfoHash: hash, Status: model.SeedingStatusSeeding,
		}
		e.mu.Unlock()
	}

	mc := &flushMockClient{
		maindata:  &model.Maindata{FreeSpace: 100 * 1024 * 1024 * 1024},
		addResult: &model.AddResult{InfoHash: "hash0"},
	}
	e.SetClientProvider(&flushMockProvider{
		clients: map[string]model.DownloaderClient{"c1": mc},
		list:    []string{"c1"},
	})

	cancel()

	candidates, err := e.Flush(ctx, fmt.Sprintf("%d", subID))
	if err != nil {
		t.Logf("context cancelled: %v (acceptable)", err)
	}
	_ = candidates
}

func TestFlush_ScoreWithBatchSLData(t *testing.T) {
	db := setupFlushTestDB(t)
	e := NewEngine(db, zap.NewNop())
	ctx := context.Background()
	_ = e.Start(ctx)

	subID := seedSubscription(t, db, true, false)

	now := time.Now()
	records := []model.SeedingTorrentRecord{
		{ClientID: "c1", InfoHash: "h1", SiteName: "site1", TorrentID: "10", Discount: model.DiscountFree, Status: model.SeedingStatusSeeding, Source: "rss", IsFree: true, SubscriptionID: fmt.Sprintf("%d", subID), CreatedAt: now},
		{ClientID: "c1", InfoHash: "h2", SiteName: "site1", TorrentID: "20", Discount: model.DiscountFree, Status: model.SeedingStatusSeeding, Source: "rss", IsFree: true, SubscriptionID: fmt.Sprintf("%d", subID), CreatedAt: now},
	}
	for i := range records {
		db.Create(&records[i])
		e.mu.Lock()
		e.recordMap[recordKey(records[i].ClientID, records[i].InfoHash)] = &model.SeedingTorrentRecord{
			ClientID: records[i].ClientID, InfoHash: records[i].InfoHash, Status: model.SeedingStatusSeeding,
			SubscriptionID: fmt.Sprintf("%d", subID),
		}
		e.mu.Unlock()
	}

	var addSeq int
	mc := &flushMockClient{
		maindata: &model.Maindata{FreeSpace: 100 * 1024 * 1024 * 1024},
		addResultFn: func() *model.AddResult {
			addSeq++
			return &model.AddResult{InfoHash: fmt.Sprintf("h_new_%d", addSeq)}
		},
	}
	e.SetClientProvider(&flushMockProvider{
		clients: map[string]model.DownloaderClient{"c1": mc},
		list:    []string{"c1"},
	})

	batchSL := map[string]*model.SLData{
		"10": {Seeders: 5, Leechers: 100},
		"20": {Seeders: 50, Leechers: 2},
	}

	var downloadOrder []string
	e.SetSiteProvider(&mocks.SiteInfoProvider{
		GetSiteInfoFn: func(_ context.Context, siteName string) (*model.SiteInfo, error) {
			return &model.SiteInfo{Name: siteName}, nil
		},
		GetSiteConfigFn: func(_ context.Context, _ string) (*model.SiteConfig, error) {
			return &model.SiteConfig{}, nil
		},
		GetAdapterFn: func(_ context.Context, _ string) (model.SiteAdapter, error) {
			return &mocks.SiteAdapter{
				DownloadTorrentFn: func(_ context.Context, _ *model.SiteConfig, tid string) ([]byte, error) {
					downloadOrder = append(downloadOrder, tid)
					return []byte("mock-torrent-data"), nil
				},
				GetBatchSLDataFn: func(_ context.Context, _ *model.SiteConfig, tids []string) (map[string]*model.SLData, error) {
					result := make(map[string]*model.SLData)
					for _, tid := range tids {
						if sl, ok := batchSL[tid]; ok {
							result[tid] = sl
						}
					}
					return result, nil
				},
				GetPreciseSLDataFn: func(_ context.Context, _ *model.SiteConfig, tid string) (*model.SLData, error) {
					if sl, ok := batchSL[tid]; ok {
						return sl, nil
					}
					return nil, nil
				},
				DetectDiscountFn: func(_ context.Context, _ *model.SiteConfig, _ string) (*model.DiscountResult, error) {
					return &model.DiscountResult{Level: model.DiscountFree}, nil
				},
			}, nil
		},
	})

	candidates, err := e.Flush(ctx, fmt.Sprintf("%d", subID))
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(candidates))
	}
	if len(downloadOrder) < 1 {
		t.Fatal("expected at least 1 download call")
	}
	if downloadOrder[0] != "10" {
		t.Errorf("expected torrent 10 (high demand: 100 leechers/5 seeders) to be downloaded first, got %s — batch SL data may not be used in scoring", downloadOrder[0])
	}
}

func TestFlush_ConfirmTopN_Rescores(t *testing.T) {
	db := setupFlushTestDB(t)
	e := NewEngine(db, zap.NewNop())
	ctx := context.Background()
	_ = e.Start(ctx)

	sub := &model.RSSSubscription{
		Name:     "test-sub-sl",
		Enabled:  true,
		SiteName: "site1",
		ClientID: "c1",
		ScoringConfig: model.SeedingScoringConfig{
			Enabled:          true,
			MaxCandidates:    50,
			MaxActiveSeeding: 100,
			BatchLimit:       10,
			MinScore:         -1,
			HalfLifeHours:    2.0,
			TopNConfirm:      2,
		},
	}
	if err := db.Create(sub).Error; err != nil {
		t.Fatalf("create sub: %v", err)
	}
	db.Model(sub).Update("min_score", 0.0)

	site := &model.Site{Name: "site1", Domain: "site1.com"}
	if err := db.Create(site).Error; err != nil {
		t.Fatalf("create site: %v", err)
	}

	now := time.Now()
	records := []model.SeedingTorrentRecord{
		{ClientID: "c1", InfoHash: "h1", SiteName: "site1", TorrentID: "10", Discount: model.DiscountFree, Status: model.SeedingStatusSeeding, Source: "rss", IsFree: true, SubscriptionID: fmt.Sprintf("%d", sub.ID), CreatedAt: now},
		{ClientID: "c1", InfoHash: "h2", SiteName: "site1", TorrentID: "20", Discount: model.DiscountFree, Status: model.SeedingStatusSeeding, Source: "rss", IsFree: true, SubscriptionID: fmt.Sprintf("%d", sub.ID), CreatedAt: now},
		{ClientID: "c1", InfoHash: "h3", SiteName: "site1", TorrentID: "30", Discount: model.DiscountFree, Status: model.SeedingStatusSeeding, Source: "rss", IsFree: true, SubscriptionID: fmt.Sprintf("%d", sub.ID), CreatedAt: now},
	}
	for i := range records {
		db.Create(&records[i])
		e.mu.Lock()
		e.recordMap[recordKey(records[i].ClientID, records[i].InfoHash)] = &model.SeedingTorrentRecord{
			ClientID: records[i].ClientID, InfoHash: records[i].InfoHash, Status: model.SeedingStatusSeeding,
			SubscriptionID: fmt.Sprintf("%d", sub.ID),
		}
		e.mu.Unlock()
	}

	var addSeq int
	mc := &flushMockClient{
		maindata: &model.Maindata{FreeSpace: 100 * 1024 * 1024 * 1024},
		addResultFn: func() *model.AddResult {
			addSeq++
			return &model.AddResult{InfoHash: fmt.Sprintf("h_new_%d", addSeq)}
		},
	}
	e.SetClientProvider(&flushMockProvider{
		clients: map[string]model.DownloaderClient{"c1": mc},
		list:    []string{"c1"},
	})

	batchSL := map[string]*model.SLData{
		"10": {Seeders: 5, Leechers: 100},
		"20": {Seeders: 10, Leechers: 50},
		"30": {Seeders: 1, Leechers: 1},
	}
	preciseSL := map[string]*model.SLData{
		"10": {Seeders: 200, Leechers: 1},
		"20": {Seeders: 1, Leechers: 200},
	}

	var downloadOrder []string
	e.SetSiteProvider(&mocks.SiteInfoProvider{
		GetSiteInfoFn: func(_ context.Context, siteName string) (*model.SiteInfo, error) {
			return &model.SiteInfo{Name: siteName}, nil
		},
		GetSiteConfigFn: func(_ context.Context, _ string) (*model.SiteConfig, error) {
			return &model.SiteConfig{}, nil
		},
		GetAdapterFn: func(_ context.Context, _ string) (model.SiteAdapter, error) {
			return &mocks.SiteAdapter{
				DownloadTorrentFn: func(_ context.Context, _ *model.SiteConfig, tid string) ([]byte, error) {
					downloadOrder = append(downloadOrder, tid)
					return []byte("mock-torrent-data"), nil
				},
				GetBatchSLDataFn: func(_ context.Context, _ *model.SiteConfig, tids []string) (map[string]*model.SLData, error) {
					result := make(map[string]*model.SLData)
					for _, tid := range tids {
						if sl, ok := batchSL[tid]; ok {
							result[tid] = sl
						}
					}
					return result, nil
				},
				GetPreciseSLDataFn: func(_ context.Context, _ *model.SiteConfig, tid string) (*model.SLData, error) {
					if sl, ok := preciseSL[tid]; ok {
						return sl, nil
					}
					return &model.SLData{Seeders: 0, Leechers: 0}, nil
				},
				DetectDiscountFn: func(_ context.Context, _ *model.SiteConfig, _ string) (*model.DiscountResult, error) {
					return &model.DiscountResult{Level: model.DiscountFree}, nil
				},
			}, nil
		},
	})

	candidates, err := e.Flush(ctx, fmt.Sprintf("%d", sub.ID))
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 3 {
		t.Fatalf("expected 3 candidates, got %d", len(candidates))
	}
	if len(downloadOrder) < 1 {
		t.Fatal("expected at least 1 download call")
	}
	if downloadOrder[0] != "20" {
		t.Errorf("expected torrent 20 (precise: 200 leechers, 1 seeder) to be downloaded first after confirmTopN, got %s", downloadOrder[0])
	}
}

func TestFlush_DiskProtectBlocks(t *testing.T) {
	db := setupFlushTestDB(t)
	ctx := context.Background()
	e := NewEngine(db, zap.NewNop())
	_ = e.Start(ctx)

	subID := seedSubscription(t, db, true, true)

	db.Create(&model.SeedingClientConfig{
		ClientID:           "c1",
		Enabled:            true,
		DiskProtectEnabled: true,
		MinDiskSpaceGB:     50,
	})

	db.Create(&model.SeedingTorrentRecord{
		ClientID:       "c1",
		InfoHash:       "hash_dp",
		SiteName:       "site1",
		TorrentID:      "100",
		Status:         model.SeedingStatusPending,
		Source:         "rss",
		SubscriptionID: fmt.Sprintf("%d", subID),
		Discount:       model.DiscountFree,
		IsFree:         true,
		CreatedAt:      time.Now().Add(-1 * time.Hour),
	})

	mc := &flushMockClient{
		maindata:  &model.Maindata{FreeSpace: 10 * 1024 * 1024 * 1024},
		addResult: &model.AddResult{InfoHash: "hash_dp"},
	}
	e.SetClientProvider(&flushMockProvider{
		clients: map[string]model.DownloaderClient{"c1": mc},
		list:    []string{"c1"},
	})
	e.SetSiteProvider(newMockSiteProvider())

	candidates, err := e.Flush(ctx, fmt.Sprintf("%d", subID))
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates when disk_protect active, got %d", len(candidates))
	}

	var rec model.SeedingTorrentRecord
	db.Where("info_hash = ?", "hash_dp").First(&rec)
	if rec.Status != model.SeedingStatusPending {
		t.Errorf("expected record to remain pending, got %s", rec.Status)
	}
}

func TestFlush_DiskProtectAllowsWhenSpaceOK(t *testing.T) {
	db := setupFlushTestDB(t)
	ctx := context.Background()
	e := NewEngine(db, zap.NewNop())
	_ = e.Start(ctx)

	subID := seedSubscription(t, db, true, true)

	db.Create(&model.SeedingClientConfig{
		ClientID:           "c1",
		Enabled:            true,
		DiskProtectEnabled: true,
		MinDiskSpaceGB:     10,
	})

	db.Create(&model.SeedingTorrentRecord{
		ClientID:       "c1",
		InfoHash:       "hash_ok",
		SiteName:       "site1",
		TorrentID:      "200",
		Status:         model.SeedingStatusPending,
		Source:         "rss",
		SubscriptionID: fmt.Sprintf("%d", subID),
		Discount:       model.DiscountFree,
		IsFree:         true,
		CreatedAt:      time.Now().Add(-1 * time.Hour),
	})

	mc := &flushMockClient{
		maindata:  &model.Maindata{FreeSpace: 100 * 1024 * 1024 * 1024},
		addResult: &model.AddResult{InfoHash: "hash_ok"},
	}
	e.SetClientProvider(&flushMockProvider{
		clients: map[string]model.DownloaderClient{"c1": mc},
		list:    []string{"c1"},
	})
	e.SetSiteProvider(newMockSiteProvider())

	candidates, err := e.Flush(ctx, fmt.Sprintf("%d", subID))
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 {
		t.Errorf("expected 1 candidate when disk space is OK, got %d", len(candidates))
	}
}

func TestFlush_DiskRecoverBypassMaxActive(t *testing.T) {
	db := setupFlushTestDB(t)
	e := NewEngine(db, zap.NewNop())
	ctx := context.Background()
	_ = e.Start(ctx)

	sub := &model.RSSSubscription{
		Name:     "test-sub",
		Enabled:  true,
		SiteName: "site1",
		ClientID: "c1",
		ScoringConfig: model.SeedingScoringConfig{
			Enabled:          true,
			MaxActiveSeeding: 2,
			BatchLimit:       10,
			MinScore:         0,
			HalfLifeHours:    2.0,
		},
	}
	db.Create(sub)
	db.Create(&model.Site{Name: "site1", Domain: "site1.com"})

	e.mu.Lock()
	e.recordMap[recordKey("c1", "existing1")] = &model.SeedingTorrentRecord{
		ClientID: "c1", InfoHash: "existing1", Status: model.SeedingStatusSeeding,
	}
	e.recordMap[recordKey("c1", "existing2")] = &model.SeedingTorrentRecord{
		ClientID: "c1", InfoHash: "existing2", Status: model.SeedingStatusSeeding,
	}
	e.mu.Unlock()

	db.Create(&model.SeedingTorrentRecord{
		ClientID:       "c1",
		InfoHash:       "dr_hash",
		SiteName:       "site1",
		TorrentID:      "500",
		Discount:       model.DiscountFree,
		Status:         model.SeedingStatusPending,
		Source:         "rss",
		SubscriptionID: fmt.Sprintf("%d", sub.ID),
		IsFree:         true,
		LastActionBy:   "disk_recover",
		CreatedAt:      time.Now().Add(-1 * time.Hour),
	})

	e.mu.Lock()
	e.recordMap[recordKey("c1", "dr_hash")] = &model.SeedingTorrentRecord{
		ClientID: "c1", InfoHash: "dr_hash", Status: model.SeedingStatusPending,
	}
	e.mu.Unlock()

	mc := &flushMockClient{
		maindata:  &model.Maindata{FreeSpace: 100 * 1024 * 1024 * 1024},
		existsMap: map[string]bool{"dr_hash": true},
		addResult: &model.AddResult{InfoHash: "dr_hash"},
	}
	e.SetClientProvider(&flushMockProvider{
		clients: map[string]model.DownloaderClient{"c1": mc},
		list:    []string{"c1"},
	})
	e.SetSiteProvider(newMockSiteProvider())

	candidates, err := e.Flush(ctx, fmt.Sprintf("%d", sub.ID))
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 {
		t.Fatalf("expected 1 disk_recover candidate bypassing maxActive, got %d", len(candidates))
	}

	var rec model.SeedingTorrentRecord
	db.Where("info_hash = ?", "dr_hash").First(&rec)
	if rec.Status != model.SeedingStatusSeeding {
		t.Errorf("expected disk_recover record to be restored to seeding, got %s", rec.Status)
	}
	if rec.LastActionBy != "disk_recover_restored" {
		t.Errorf("expected last_action_by=disk_recover_restored, got %s", rec.LastActionBy)
	}
}

func TestFlush_DiskRecoverMaxActiveNoRecoverCandidates(t *testing.T) {
	db := setupFlushTestDB(t)
	e := NewEngine(db, zap.NewNop())
	ctx := context.Background()
	_ = e.Start(ctx)

	sub := &model.RSSSubscription{
		Name:     "test-sub",
		Enabled:  true,
		SiteName: "site1",
		ClientID: "c1",
		ScoringConfig: model.SeedingScoringConfig{
			Enabled:          true,
			MaxActiveSeeding: 1,
			BatchLimit:       10,
			MinScore:         0,
			HalfLifeHours:    2.0,
		},
	}
	db.Create(sub)
	db.Create(&model.Site{Name: "site1", Domain: "site1.com"})

	e.mu.Lock()
	e.recordMap[recordKey("c1", "existing1")] = &model.SeedingTorrentRecord{
		ClientID: "c1", InfoHash: "existing1", Status: model.SeedingStatusSeeding,
	}
	e.mu.Unlock()

	db.Create(&model.SeedingTorrentRecord{
		ClientID:       "c1",
		InfoHash:       "normal_hash",
		SiteName:       "site1",
		TorrentID:      "600",
		Discount:       model.DiscountFree,
		Status:         model.SeedingStatusPending,
		Source:         "rss",
		SubscriptionID: fmt.Sprintf("%d", sub.ID),
		IsFree:         true,
		CreatedAt:      time.Now().Add(-1 * time.Hour),
	})

	e.mu.Lock()
	e.recordMap[recordKey("c1", "normal_hash")] = &model.SeedingTorrentRecord{
		ClientID: "c1", InfoHash: "normal_hash", Status: model.SeedingStatusPending,
	}
	e.mu.Unlock()

	mc := &flushMockClient{
		maindata:  &model.Maindata{FreeSpace: 100 * 1024 * 1024 * 1024},
		addResult: &model.AddResult{InfoHash: "normal_hash"},
	}
	e.SetClientProvider(&flushMockProvider{
		clients: map[string]model.DownloaderClient{"c1": mc},
		list:    []string{"c1"},
	})
	e.SetSiteProvider(newMockSiteProvider())

	candidates, err := e.Flush(ctx, fmt.Sprintf("%d", sub.ID))
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates when maxActive reached with no disk_recover, got %d", len(candidates))
	}
}
