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
	db.AutoMigrate(&model.ClientConfig{}, &model.ClientPathMapping{})
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

	m.LoadClients(context.Background())
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

	m.LoadClients(context.Background())

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
