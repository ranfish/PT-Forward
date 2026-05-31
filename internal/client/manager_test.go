package client

import (
	"context"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupManagerDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.ClientConfig{}, &model.ClientPathMapping{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestNewManager(t *testing.T) {
	db := setupManagerDB(t)
	m := NewManager(db, zap.NewNop())
	if m == nil {
		t.Fatal("manager is nil")
	}
	if m.ConnectedCount() != 0 {
		t.Errorf("expected 0 connected clients, got %d", m.ConnectedCount())
	}
}

func TestManager_LoadClients_Empty(t *testing.T) {
	db := setupManagerDB(t)
	m := NewManager(db, zap.NewNop())

	err := m.LoadClients(context.Background())
	if err != nil {
		t.Fatalf("LoadClients: %v", err)
	}
	if m.ConnectedCount() != 0 {
		t.Errorf("expected 0, got %d", m.ConnectedCount())
	}
}

func TestManager_Get_NotFound(t *testing.T) {
	db := setupManagerDB(t)
	m := NewManager(db, zap.NewNop())

	_, err := m.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent client")
	}
}

func TestManager_ListClients_Empty(t *testing.T) {
	db := setupManagerDB(t)
	m := NewManager(db, zap.NewNop())

	names := m.ListClients()
	if len(names) != 0 {
		t.Errorf("expected empty list, got %v", names)
	}
}

func TestManager_LoadClients_WithConfig(t *testing.T) {
	db := setupManagerDB(t)
	m := NewManager(db, zap.NewNop())

	db.Create(&model.ClientConfig{
		Name:    "test-qb",
		Type:    "qbittorrent",
		URL:     "http://127.0.0.1:9999",
		Role:    "download",
		Enabled: true,
	})

	err := m.LoadClients(context.Background())
	if err != nil {
		t.Fatalf("LoadClients: %v", err)
	}

	names := m.ListClients()
	if len(names) != 0 {
		t.Errorf("expected 0 clients (connection to 127.0.0.1:9999 should fail), got %d: %v", len(names), names)
	}
}

func TestManager_Reload(t *testing.T) {
	db := setupManagerDB(t)
	m := NewManager(db, zap.NewNop())

	db.Create(&model.ClientConfig{
		Name:    "client-a",
		Type:    "qbittorrent",
		URL:     "http://127.0.0.1:9999",
		Role:    "download",
		Enabled: true,
	})

	if err := m.LoadClients(context.Background()); err != nil {
		t.Fatalf("LoadClients: %v", err)
	}
	if m.ConnectedCount() != 0 {
		t.Errorf("expected 0 (unreachable), got %d", m.ConnectedCount())
	}

	err := m.Reload(context.Background())
	if err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if m.ConnectedCount() != 0 {
		t.Errorf("expected 0 after reload (unreachable), got %d", m.ConnectedCount())
	}
}

func TestManager_GetByDBID(t *testing.T) {
	db := setupManagerDB(t)
	m := NewManager(db, zap.NewNop())

	cfg := &model.ClientConfig{
		Name:    "client-b",
		Type:    "qbittorrent",
		URL:     "http://127.0.0.1:9999",
		Role:    "download",
		Enabled: true,
	}
	db.Create(cfg)

	if err := m.LoadClients(context.Background()); err != nil {
		t.Fatalf("LoadClients: %v", err)
	}

	_, gotCfg, err := m.GetByDBID(context.Background(), cfg.ID)
	if err != nil {
		if gotCfg != nil && gotCfg.Name == "client-b" {
			return
		}
		t.Fatalf("GetByDBID: %v", err)
	}
	if gotCfg.Name != "client-b" {
		t.Errorf("expected client-b, got %s", gotCfg.Name)
	}
}

func TestManager_GetByDBID_NotFound(t *testing.T) {
	db := setupManagerDB(t)
	m := NewManager(db, zap.NewNop())

	_, _, err := m.GetByDBID(context.Background(), 9999)
	if err == nil {
		t.Error("expected error for nonexistent ID")
	}
}

func TestManager_GetByDBID_ClientNotLoaded(t *testing.T) {
	db := setupManagerDB(t)
	m := NewManager(db, zap.NewNop())

	cfg := &model.ClientConfig{
		Name:    "unloaded-qb",
		Type:    "qbittorrent",
		URL:     "http://127.0.0.1:9999",
		Role:    "download",
		Enabled: true,
	}
	db.Create(cfg)

	_, _, err := m.GetByDBID(context.Background(), cfg.ID)
	if err == nil {
		t.Error("expected error when client not loaded in manager")
	}
}

func TestManager_LoadClients_SkipsDisabled(t *testing.T) {
	db := setupManagerDB(t)
	m := NewManager(db, zap.NewNop())

	cfg := &model.ClientConfig{
		Name: "disabled-qb", Type: "qbittorrent", URL: "http://127.0.0.1:9999",
		Role: "download", Enabled: false,
	}
	db.Exec("INSERT INTO clients (name, type, url, role, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, ?, datetime('now'), datetime('now'))",
		cfg.Name, cfg.Type, cfg.URL, cfg.Role, false)

	if err := m.LoadClients(context.Background()); err != nil {
		t.Fatalf("LoadClients: %v", err)
	}
	if m.ConnectedCount() != 0 {
		t.Errorf("expected 0 clients (disabled), got %d", m.ConnectedCount())
	}
}

func TestManager_LoadClients_RemovesStale(t *testing.T) {
	db := setupManagerDB(t)
	m := NewManager(db, zap.NewNop())

	db.Create(&model.ClientConfig{
		Name: "stale-qb", Type: "qbittorrent", URL: "http://127.0.0.1:9999",
		Role: "download", Enabled: true,
	})

	m.mu.Lock()
	m.clients["stale-qb"] = &stubClient{name: "stale-qb"}
	m.mu.Unlock()
	if m.ConnectedCount() != 1 {
		t.Fatalf("expected 1 (injected), got %d", m.ConnectedCount())
	}

	db.Exec("DELETE FROM clients WHERE name = 'stale-qb'")
	if err := m.LoadClients(context.Background()); err != nil {
		t.Fatalf("LoadClients 2: %v", err)
	}
	if m.ConnectedCount() != 0 {
		t.Errorf("expected 0 after deletion, got %d", m.ConnectedCount())
	}
}

func TestManager_PingAll(t *testing.T) {
	db := setupManagerDB(t)
	m := NewManager(db, zap.NewNop())

	db.Create(&model.ClientConfig{
		Name: "ping-qb", Type: "qbittorrent", URL: "http://127.0.0.1:9999",
		Role: "download", Enabled: true,
	})
	if err := m.LoadClients(context.Background()); err != nil {
		t.Fatalf("LoadClients: %v", err)
	}

	m.PingAll(context.Background())
}

func TestManager_PingAll_Cancelled(t *testing.T) {
	db := setupManagerDB(t)
	m := NewManager(db, zap.NewNop())

	db.Create(&model.ClientConfig{
		Name: "cancel-qb", Type: "qbittorrent", URL: "http://127.0.0.1:9999",
		Role: "download", Enabled: true,
	})
	if err := m.LoadClients(context.Background()); err != nil {
		t.Fatalf("LoadClients: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m.PingAll(ctx)
}

func TestManager_CreateClient_UnsupportedType(t *testing.T) {
	db := setupManagerDB(t)
	m := NewManager(db, zap.NewNop())

	_, err := m.createClient(&model.ClientConfig{Type: "deluge"}, nil)
	if err == nil {
		t.Error("expected error for unsupported type")
	}
}

func TestManager_CreateClient_Transmission(t *testing.T) {
	db := setupManagerDB(t)
	m := NewManager(db, zap.NewNop())

	cfg := &model.ClientConfig{
		Type: "transmission", URL: "http://127.0.0.1:9998",
		Name: "test-tr", Role: "download", Enabled: true,
	}
	_, err := m.createClient(cfg, nil)
	if err != nil {
		t.Fatalf("createClient transmission: %v", err)
	}
}

type stubClient struct {
	name string
}

func (s *stubClient) GetName() string                                               { return s.name }
func (s *stubClient) GetRole() string                                               { return "download" }
func (s *stubClient) GetReseedTargetID() string                                     { return "" }
func (s *stubClient) GetID() uint                                                   { return 0 }
func (s *stubClient) GetSharedPaths() []model.SharedPathMapping                     { return nil }
func (s *stubClient) GetTorrentByHash(_ context.Context, _ string) (*model.TorrentInfo, error) {
	return nil, nil
}
func (s *stubClient) GetSeedingTorrents(_ context.Context) ([]*model.TorrentInfo, error) {
	return nil, nil
}
func (s *stubClient) GetAllTorrents(_ context.Context) ([]*model.TorrentInfo, error) {
	return nil, nil
}
func (s *stubClient) GetTorrentsByPath(_ context.Context, _ string) ([]*model.TorrentInfo, error) {
	return nil, nil
}
func (s *stubClient) GetMainData(_ context.Context) (*model.Maindata, error)        { return nil, nil }
func (s *stubClient) GetMainDataIncremental(_ context.Context, _ int) (*model.Maindata, int, error) {
	return nil, 0, nil
}
func (s *stubClient) AddFromFile(_ context.Context, _ []byte, _ model.AddTorrentOptions) (*model.AddResult, error) {
	return nil, nil
}
func (s *stubClient) ExportTorrent(_ context.Context, _ string) ([]byte, error) { return nil, nil }
func (s *stubClient) DeleteTorrent(_ context.Context, _ string, _ bool) error   { return nil }
func (s *stubClient) BatchDeleteTorrents(_ context.Context, _ []string, _ bool) error {
	return nil
}
func (s *stubClient) PauseTorrent(_ context.Context, _ string) error            { return nil }
func (s *stubClient) ResumeTorrent(_ context.Context, _ string) error           { return nil }
func (s *stubClient) Reannounce(_ context.Context, _ string) error              { return nil }
func (s *stubClient) Recheck(_ context.Context, _ string) error                 { return nil }
func (s *stubClient) SetTorrentTags(_ context.Context, _ string, _ []string) error {
	return nil
}
func (s *stubClient) RemoveTorrentTags(_ context.Context, _ string, _ []string) error {
	return nil
}
func (s *stubClient) SetCategory(_ context.Context, _ string, _ string) error    { return nil }
func (s *stubClient) SetSavePath(_ context.Context, _ string, _ string) error    { return nil }
func (s *stubClient) SetSuperSeeding(_ context.Context, _ string, _ bool) error   { return nil }
func (s *stubClient) SetUploadLimit(_ context.Context, _ string, _ int64) error   { return nil }
func (s *stubClient) PauseAllDownloads(_ context.Context) error                   { return nil }
func (s *stubClient) ResumeAllDownloads(_ context.Context) error                  { return nil }
func (s *stubClient) GetFreeSpace(_ context.Context) (int64, error)               { return 0, nil }
func (s *stubClient) CheckExists(_ context.Context, _ string) (bool, error)       { return false, nil }
