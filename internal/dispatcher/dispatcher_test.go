package dispatcher

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/client"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.AutoMigrate(
		&model.RSSSubscription{},
		&model.ClientConfig{},
	)
	return db
}

func createSubscription(t *testing.T, db *gorm.DB, id uint, clientName string) {
	t.Helper()
	sub := &model.RSSSubscription{
		Name:     "sub-" + clientName,
		Enabled:  true,
		SiteName: "testsite",
		ClientID: clientName,
		URLs:     []string{"http://example.com/rss"},
	}
	sub.ID = id
	if err := db.Create(sub).Error; err != nil {
		t.Fatalf("create subscription: %v", err)
	}
}

func createClientConfig(t *testing.T, db *gorm.DB, name, role string) {
	t.Helper()
	cfg := &model.ClientConfig{
		Name:    name,
		Type:    "qbittorrent",
		URL:     "http://localhost:8080",
		Enabled: true,
		Role:    role,
	}
	if err := db.Create(cfg).Error; err != nil {
		t.Fatalf("create client config: %v", err)
	}
}

func newTestDispatcher(t *testing.T, db *gorm.DB) *TorrentDispatcher {
	t.Helper()
	mgr := client.NewManager(db, zap.NewNop())
	return NewTorrentDispatcher(db, mgr, zap.NewNop())
}

func TestDispatcher_EmptyEvents(t *testing.T) {
	db := setupTestDB(t)
	d := newTestDispatcher(t, db)
	if err := d.OnTorrents(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
}

func TestDispatcher_RoutesByRole(t *testing.T) {
	db := setupTestDB(t)
	createSubscription(t, db, 1, "qb-seed")
	createSubscription(t, db, 2, "qb-dl")
	createClientConfig(t, db, "qb-seed", "seeding")
	createClientConfig(t, db, "qb-dl", "download")

	d := newTestDispatcher(t, db)

	var seedingMu sync.Mutex
	var seedingEvents []model.TorrentEvent
	var downloadMu sync.Mutex
	var downloadEvents []model.TorrentEvent

	d.RegisterHandler(RoleSeeding, &mockHandler{
		fn: func(ctx context.Context, events []model.TorrentEvent) error {
			seedingMu.Lock()
			seedingEvents = append(seedingEvents, events...)
			seedingMu.Unlock()
			return nil
		},
	})
	d.RegisterHandler(RoleDownload, &mockHandler{
		fn: func(ctx context.Context, events []model.TorrentEvent) error {
			downloadMu.Lock()
			downloadEvents = append(downloadEvents, events...)
			downloadMu.Unlock()
			return nil
		},
	})

	events := []model.TorrentEvent{
		{SourceID: "1", SiteName: "testsite", TorrentID: "100", Title: "seed torrent"},
		{SourceID: "2", SiteName: "testsite", TorrentID: "200", Title: "dl torrent"},
	}

	if err := d.OnTorrents(context.Background(), events); err != nil {
		t.Fatal(err)
	}

	seedingMu.Lock()
	if len(seedingEvents) != 1 {
		t.Fatalf("expected 1 seeding event, got %d", len(seedingEvents))
	}
	if seedingEvents[0].TorrentID != "100" {
		t.Errorf("expected torrent 100, got %s", seedingEvents[0].TorrentID)
	}
	if GetClientName(&seedingEvents[0]) != "qb-seed" {
		t.Errorf("expected client qb-seed, got %s", GetClientName(&seedingEvents[0]))
	}
	if GetClientRole(&seedingEvents[0]) != "seeding" {
		t.Errorf("expected role seeding, got %s", GetClientRole(&seedingEvents[0]))
	}
	seedingMu.Unlock()

	downloadMu.Lock()
	if len(downloadEvents) != 1 {
		t.Fatalf("expected 1 download event, got %d", len(downloadEvents))
	}
	if downloadEvents[0].TorrentID != "200" {
		t.Errorf("expected torrent 200, got %s", downloadEvents[0].TorrentID)
	}
	if GetClientName(&downloadEvents[0]) != "qb-dl" {
		t.Errorf("expected client qb-dl, got %s", GetClientName(&downloadEvents[0]))
	}
	downloadMu.Unlock()
}

func TestDispatcher_SkipsInvalidSourceID(t *testing.T) {
	db := setupTestDB(t)
	d := newTestDispatcher(t, db)

	var called bool
	d.RegisterHandler(RoleSeeding, &mockHandler{
		fn: func(ctx context.Context, events []model.TorrentEvent) error {
			called = true
			return nil
		},
	})

	events := []model.TorrentEvent{
		{SourceID: "not-a-number", SiteName: "testsite", TorrentID: "100"},
	}

	if err := d.OnTorrents(context.Background(), events); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Error("handler should not be called for invalid source_id")
	}
}

func TestDispatcher_SkipsMissingSubscription(t *testing.T) {
	db := setupTestDB(t)
	d := newTestDispatcher(t, db)

	var called bool
	d.RegisterHandler(RoleSeeding, &mockHandler{
		fn: func(ctx context.Context, events []model.TorrentEvent) error {
			called = true
			return nil
		},
	})

	events := []model.TorrentEvent{
		{SourceID: "999", SiteName: "testsite", TorrentID: "100"},
	}

	if err := d.OnTorrents(context.Background(), events); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Error("handler should not be called for missing subscription")
	}
}

func TestDispatcher_SkipsMissingClient(t *testing.T) {
	db := setupTestDB(t)
	createSubscription(t, db, 1, "nonexistent-client")

	d := newTestDispatcher(t, db)

	var called bool
	d.RegisterHandler(RoleSeeding, &mockHandler{
		fn: func(ctx context.Context, events []model.TorrentEvent) error {
			called = true
			return nil
		},
	})

	events := []model.TorrentEvent{
		{SourceID: "1", SiteName: "testsite", TorrentID: "100"},
	}

	if err := d.OnTorrents(context.Background(), events); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Error("handler should not be called for missing client config")
	}
}

func TestDispatcher_SkipsDisabledSubscription(t *testing.T) {
	db := setupTestDB(t)
	sub := &model.RSSSubscription{
		Name:     "disabled-sub",
		Enabled:  true,
		SiteName: "testsite",
		ClientID: "qb-seed",
		URLs:     []string{"http://example.com/rss"},
	}
	sub.ID = 1
	db.Create(sub)
	db.Model(sub).Update("enabled", false)
	createClientConfig(t, db, "qb-seed", "seeding")

	d := newTestDispatcher(t, db)

	var called bool
	d.RegisterHandler(RoleSeeding, &mockHandler{
		fn: func(ctx context.Context, events []model.TorrentEvent) error {
			called = true
			return nil
		},
	})

	events := []model.TorrentEvent{
		{SourceID: "1", SiteName: "testsite", TorrentID: "100"},
	}

	if err := d.OnTorrents(context.Background(), events); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Error("handler should not be called for disabled subscription")
	}
}

func TestDispatcher_SkipsDisabledClient(t *testing.T) {
	db := setupTestDB(t)
	createSubscription(t, db, 1, "qb-off")
	cfg := &model.ClientConfig{
		Name:    "qb-off",
		Type:    "qbittorrent",
		URL:     "http://localhost:8080",
		Enabled: true,
		Role:    "download",
	}
	db.Create(cfg)
	db.Model(cfg).Update("enabled", false)

	d := newTestDispatcher(t, db)

	var called bool
	d.RegisterHandler(RoleDownload, &mockHandler{
		fn: func(ctx context.Context, events []model.TorrentEvent) error {
			called = true
			return nil
		},
	})

	events := []model.TorrentEvent{
		{SourceID: "1", SiteName: "testsite", TorrentID: "100"},
	}

	if err := d.OnTorrents(context.Background(), events); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Error("handler should not be called for disabled client")
	}
}

func TestDispatcher_NoHandlerForRole(t *testing.T) {
	db := setupTestDB(t)
	createSubscription(t, db, 1, "qb-src")
	createClientConfig(t, db, "qb-src", "source")

	d := newTestDispatcher(t, db)

	events := []model.TorrentEvent{
		{SourceID: "1", SiteName: "testsite", TorrentID: "100"},
	}

	if err := d.OnTorrents(context.Background(), events); err != nil {
		t.Fatal(err)
	}
}

func TestDispatcher_HandlerError(t *testing.T) {
	db := setupTestDB(t)
	createSubscription(t, db, 1, "qb-seed")
	createClientConfig(t, db, "qb-seed", "seeding")

	d := newTestDispatcher(t, db)

	d.RegisterHandler(RoleSeeding, &mockHandler{
		fn: func(ctx context.Context, events []model.TorrentEvent) error {
			return &model.AppError{Code: 50001, Message: "internal error"}
		},
	})

	events := []model.TorrentEvent{
		{SourceID: "1", SiteName: "testsite", TorrentID: "100"},
	}

	if err := d.OnTorrents(context.Background(), events); err == nil {
		t.Error("expected error from handler")
	}
}

func TestDispatcher_SourceRoleRoutesToSourceHandler(t *testing.T) {
	db := setupTestDB(t)
	createSubscription(t, db, 1, "qb-source")
	createClientConfig(t, db, "qb-source", "source")

	d := newTestDispatcher(t, db)

	var mu sync.Mutex
	var gotEvents []model.TorrentEvent
	d.RegisterHandler(RoleSource, &mockHandler{
		fn: func(ctx context.Context, events []model.TorrentEvent) error {
			mu.Lock()
			gotEvents = append(gotEvents, events...)
			mu.Unlock()
			return nil
		},
	})

	events := []model.TorrentEvent{
		{SourceID: "1", SiteName: "testsite", TorrentID: "100"},
	}

	if err := d.OnTorrents(context.Background(), events); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	if len(gotEvents) != 1 {
		t.Fatalf("expected 1 event, got %d", len(gotEvents))
	}
	if GetClientRole(&gotEvents[0]) != "source" {
		t.Errorf("expected role source, got %s", GetClientRole(&gotEvents[0]))
	}
	mu.Unlock()
}

func TestDispatcher_MetadataEnrichment(t *testing.T) {
	db := setupTestDB(t)
	createSubscription(t, db, 42, "qb-test")
	createClientConfig(t, db, "qb-test", "download")

	d := newTestDispatcher(t, db)

	var got *model.TorrentEvent
	d.RegisterHandler(RoleDownload, &mockHandler{
		fn: func(ctx context.Context, events []model.TorrentEvent) error {
			got = &events[0]
			return nil
		},
	})

	events := []model.TorrentEvent{
		{SourceID: "42", SiteName: "testsite", TorrentID: "999"},
	}

	if err := d.OnTorrents(context.Background(), events); err != nil {
		t.Fatal(err)
	}

	if got == nil {
		t.Fatal("expected event")
	}
	if GetClientName(got) != "qb-test" {
		t.Errorf("expected client_name=qb-test, got %s", GetClientName(got))
	}
	if GetClientRole(got) != "download" {
		t.Errorf("expected client_role=download, got %s", GetClientRole(got))
	}
	if GetSubscriptionID(got) != "42" {
		t.Errorf("expected subscription_id=42, got %s", GetSubscriptionID(got))
	}
}

func TestDispatcher_MultipleRolesSameSubscription(t *testing.T) {
	db := setupTestDB(t)
	createSubscription(t, db, 1, "qb-mix")
	createClientConfig(t, db, "qb-mix", "seeding")

	d := newTestDispatcher(t, db)

	var count int
	d.RegisterHandler(RoleSeeding, &mockHandler{
		fn: func(ctx context.Context, events []model.TorrentEvent) error {
			count += len(events)
			return nil
		},
	})

	events := []model.TorrentEvent{
		{SourceID: "1", SiteName: "testsite", TorrentID: "100"},
		{SourceID: "1", SiteName: "testsite", TorrentID: "101"},
		{SourceID: "1", SiteName: "testsite", TorrentID: "102"},
	}

	if err := d.OnTorrents(context.Background(), events); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Errorf("expected 3 events routed, got %d", count)
	}
}

func TestGetClientName_Fallback(t *testing.T) {
	ev := &model.TorrentEvent{SourceID: "fallback-id"}
	if got := GetClientName(ev); got != "fallback-id" {
		t.Errorf("expected fallback-id, got %s", got)
	}
}

func TestGetClientRole_Fallback(t *testing.T) {
	ev := &model.TorrentEvent{SourceID: "1"}
	if got := GetClientRole(ev); got != "" {
		t.Errorf("expected empty, got %s", got)
	}
}

func TestGetSubscriptionID_Fallback(t *testing.T) {
	ev := &model.TorrentEvent{SourceID: "99"}
	if got := GetSubscriptionID(ev); got != "99" {
		t.Errorf("expected 99, got %s", got)
	}
}

func TestDispatcher_ConcurrentRegister(t *testing.T) {
	db := setupTestDB(t)
	d := newTestDispatcher(t, db)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d.RegisterHandler(RoleSeeding, &mockHandler{
				fn: func(ctx context.Context, events []model.TorrentEvent) error { return nil },
			})
		}()
	}
	wg.Wait()
}

func TestDispatcher_SoftDeletedSubscription(t *testing.T) {
	db := setupTestDB(t)
	sub := &model.RSSSubscription{
		Name:     "deleted-sub",
		Enabled:  true,
		SiteName: "testsite",
		ClientID: "qb-seed",
		URLs:     []string{"http://example.com/rss"},
	}
	sub.ID = 1
	db.Create(sub)
	db.Model(sub).Update("deleted_at", time.Now())
	createClientConfig(t, db, "qb-seed", "seeding")

	d := newTestDispatcher(t, db)

	var called bool
	d.RegisterHandler(RoleSeeding, &mockHandler{
		fn: func(ctx context.Context, events []model.TorrentEvent) error {
			called = true
			return nil
		},
	})

	events := []model.TorrentEvent{
		{SourceID: "1", SiteName: "testsite", TorrentID: "100"},
	}

	if err := d.OnTorrents(context.Background(), events); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Error("handler should not be called for soft-deleted subscription")
	}
}

type mockHandler struct {
	fn func(ctx context.Context, events []model.TorrentEvent) error
}

func (h *mockHandler) OnTorrents(ctx context.Context, events []model.TorrentEvent) error {
	return h.fn(ctx, events)
}
