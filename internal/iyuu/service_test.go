package iyuu

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupIYUUDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.AutoMigrate(&model.IYUUConfig{}, &model.IYUUSiteMapping{})
	return db
}

func createTestConfig(t *testing.T, db *gorm.DB) {
	t.Helper()
	cfg := &model.IYUUConfig{
		Token:             "test-token-12345678",
		BaseURL:           "https://2025.iyuu.cn",
		Enabled:           true,
		RequestTimeoutSec: 60,
	}
	cfg.ID = 1
	db.Create(cfg)
}

func TestService_Ping_Success(t *testing.T) {
	db := setupIYUUDB(t)
	createTestConfig(t, db)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("token") != "test-token-12345678" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"ret": 200, "msg": "ok", "data": []any{}})
	}))
	defer server.Close()

	db.Model(&model.IYUUConfig{}).Where("id = 1").Update("base_url", server.URL)

	svc := NewService(db, zap.NewNop())
	if err := svc.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestService_Ping_NoConfig(t *testing.T) {
	db := setupIYUUDB(t)
	svc := NewService(db, zap.NewNop())
	if err := svc.Ping(context.Background()); err == nil {
		t.Error("expected error for missing config")
	}
}

func TestService_Ping_Error(t *testing.T) {
	db := setupIYUUDB(t)
	createTestConfig(t, db)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"ret": 400, "msg": "token invalid"})
	}))
	defer server.Close()

	db.Model(&model.IYUUConfig{}).Where("id = 1").Update("base_url", server.URL)

	svc := NewService(db, zap.NewNop())
	if err := svc.Ping(context.Background()); err == nil {
		t.Error("expected error for invalid token")
	}
}

func TestService_QueryReseed_Success(t *testing.T) {
	db := setupIYUUDB(t)
	createTestConfig(t, db)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"ret": 200,
			"msg": "ok",
			"data": map[string]any{
				"abc123": []map[string]any{
					{"sid": 1, "torrent_id": 42, "info_hash": "def456"},
				},
			},
		})
	}))
	defer server.Close()

	db.Model(&model.IYUUConfig{}).Where("id = 1").Update("base_url", server.URL)

	svc := NewService(db, zap.NewNop())
	results, err := svc.QueryReseed(context.Background(), []string{"abc123"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].SourceInfoHash != "abc123" {
		t.Errorf("expected hash abc123, got %s", results[0].SourceInfoHash)
	}
	if len(results[0].Targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(results[0].Targets))
	}
	if results[0].Targets[0].Sid != 1 {
		t.Errorf("expected sid 1, got %d", results[0].Targets[0].Sid)
	}
}

func TestService_QueryReseed_Empty(t *testing.T) {
	db := setupIYUUDB(t)
	createTestConfig(t, db)

	svc := NewService(db, zap.NewNop())
	results, err := svc.QueryReseed(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if results != nil {
		t.Error("expected nil for empty input")
	}
}

func TestService_GetSiteList_Success(t *testing.T) {
	db := setupIYUUDB(t)
	createTestConfig(t, db)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"ret": 200,
			"msg": "ok",
			"data": []map[string]any{
				{"sid": 1, "nickname": "SiteA", "base_url": "https://sitea.com", "site": "sitea"},
				{"sid": 2, "nickname": "SiteB", "base_url": "https://siteb.com", "site": "siteb"},
			},
		})
	}))
	defer server.Close()

	db.Model(&model.IYUUConfig{}).Where("id = 1").Update("base_url", server.URL)

	svc := NewService(db, zap.NewNop())
	sites, err := svc.GetSiteList(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 2 {
		t.Fatalf("expected 2 sites, got %d", len(sites))
	}
	if sites[0].Sid != 1 {
		t.Errorf("expected sid 1, got %d", sites[0].Sid)
	}

	var count int64
	db.Model(&model.IYUUSiteMapping{}).Count(&count)
	if count != 2 {
		t.Errorf("expected 2 site mappings, got %d", count)
	}
}

func TestService_ReportExisting(t *testing.T) {
	db := setupIYUUDB(t)
	createTestConfig(t, db)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		sids, ok := req["sid"].([]any)
		if !ok || len(sids) != 2 {
			t.Errorf("expected 2 sids, got %v", req["sid"])
		}
		json.NewEncoder(w).Encode(map[string]any{"ret": 200, "msg": "ok"})
	}))
	defer server.Close()

	db.Model(&model.IYUUConfig{}).Where("id = 1").Update("base_url", server.URL)

	svc := NewService(db, zap.NewNop())
	if err := svc.ReportExisting(context.Background(), []int{1, 2}); err != nil {
		t.Fatal(err)
	}
}

func TestService_ReportExisting_Empty(t *testing.T) {
	db := setupIYUUDB(t)
	createTestConfig(t, db)
	svc := NewService(db, zap.NewNop())
	if err := svc.ReportExisting(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
}

func TestService_SendNotification(t *testing.T) {
	db := setupIYUUDB(t)
	createTestConfig(t, db)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"ret": 200, "msg": "ok"})
	}))
	defer server.Close()

	db.Model(&model.IYUUConfig{}).Where("id = 1").Update("base_url", server.URL)

	svc := NewService(db, zap.NewNop())
	if err := svc.SendNotification(context.Background(), "test title", "test body"); err != nil {
		t.Fatal(err)
	}
}

func TestService_GetSeededSites(t *testing.T) {
	db := setupIYUUDB(t)
	createTestConfig(t, db)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"ret": 200,
			"msg": "ok",
			"data": map[string]any{
				"abc123": []map[string]any{
					{"sid": 1, "torrent_id": 1, "info_hash": "x"},
					{"sid": 2, "torrent_id": 2, "info_hash": "y"},
				},
			},
		})
	}))
	defer server.Close()

	db.Model(&model.IYUUConfig{}).Where("id = 1").Update("base_url", server.URL)

	db.Create(&model.IYUUSiteMapping{IYUUSid: 1, SiteDomain: "sitea.com", SiteName: "SiteA"})
	db.Create(&model.IYUUSiteMapping{IYUUSid: 2, SiteDomain: "siteb.com", SiteName: "SiteB"})

	svc := NewService(db, zap.NewNop())
	sites, err := svc.GetSeededSites(context.Background(), "abc123")
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 2 {
		t.Fatalf("expected 2 sites, got %d", len(sites))
	}
}
