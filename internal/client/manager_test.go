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
	if len(names) != 1 {
		t.Errorf("expected 1 client, got %d: %v", len(names), names)
	}
	if names[0] != "test-qb" {
		t.Errorf("expected test-qb, got %s", names[0])
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
	if m.ConnectedCount() != 1 {
		t.Errorf("expected 1, got %d", m.ConnectedCount())
	}

	err := m.Reload(context.Background())
	if err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if m.ConnectedCount() != 1 {
		t.Errorf("expected 1 after reload, got %d", m.ConnectedCount())
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
	if err := m.LoadClients(context.Background()); err != nil {
		t.Fatalf("LoadClients: %v", err)
	}
	if m.ConnectedCount() != 1 {
		t.Fatalf("expected 1, got %d", m.ConnectedCount())
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
