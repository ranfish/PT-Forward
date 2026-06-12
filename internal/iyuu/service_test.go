package iyuu

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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
	if err := db.AutoMigrate(&model.IYUUConfig{}, &model.IYUUSiteMapping{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func createTestConfig(t *testing.T, db *gorm.DB) {
	t.Helper()
	cfg := &model.IYUUConfig{
		Token:             "IYUU2436Tfb5654907b6cb659aed15648720eb0fc6ebef8dd",
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
		if r.Header.Get("Token") != "IYUU2436Tfb5654907b6cb659aed15648720eb0fc6ebef8dd" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Path != "/reseed/sites/index" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if err := json.NewEncoder(w).Encode(map[string]any{"code": 0, "msg": "ok", "data": map[string]any{"count": 0, "sites": []any{}}}); err != nil {
			t.Errorf("encode: %v", err)
		}
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
		if err := json.NewEncoder(w).Encode(map[string]any{"code": 403, "msg": "token invalid"}); err != nil {
			t.Errorf("encode: %v", err)
		}
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

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/reseed/index/index" {
			t.Errorf("expected /reseed/index/index, got %s", r.URL.Path)
		}
		if r.Header.Get("Token") == "" {
			t.Error("expected Token header")
		}
		if !strings.Contains(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
			t.Errorf("expected form content type, got %s", r.Header.Get("Content-Type"))
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		if r.Form.Get("hash") == "" || r.Form.Get("sha1") == "" || r.Form.Get("timestamp") == "" || r.Form.Get("version") == "" {
			t.Errorf("missing required form fields: %v", r.Form)
		}
		if err := json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"msg":  "ok",
			"data": map[string]any{
				"abc123": map[string]any{
					"torrent": []map[string]any{
						{"sid": 1, "torrent_id": 42, "info_hash": "def456", "group": 0},
					},
				},
			},
		}); err != nil {
			t.Errorf("encode: %v", err)
		}
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
	if results[0].Targets[0].Group != 0 {
		t.Errorf("expected group 0, got %d", results[0].Targets[0].Group)
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

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/reseed/sites/index" {
			t.Errorf("expected /reseed/sites/index, got %s", r.URL.Path)
		}
		if r.Header.Get("Token") == "" {
			t.Error("expected Token header")
		}
		if err := json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"msg":  "ok",
			"data": map[string]any{
				"count": 2,
				"sites": []map[string]any{
					{"id": 1, "nickname": "SiteA", "base_url": "https://sitea.com", "site": "sitea"},
					{"id": 2, "nickname": "SiteB", "base_url": "https://siteb.com", "site": "siteb"},
				},
			},
		}); err != nil {
			t.Errorf("encode: %v", err)
		}
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
		if r.URL.Path != "/reseed/sites/reportExisting" {
			t.Errorf("expected /reseed/sites/reportExisting, got %s", r.URL.Path)
		}
		if r.Header.Get("Token") == "" {
			t.Error("expected Token header")
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		ct := r.Header.Get("Content-Type")
		if !strings.Contains(ct, "application/json") {
			t.Errorf("expected JSON content type, got %s", ct)
		}
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		sids, ok := req["sid_list"].([]any)
		if !ok || len(sids) != 2 {
			t.Errorf("expected 2 sids, got %v", req["sid_list"])
		}
		if err := json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"msg":  "ok",
			"data": map[string]any{"sid_sha1": "abc123def456"},
		}); err != nil {
			t.Errorf("encode: %v", err)
		}
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

func TestService_QueryReseed_AutoEnsureSidSha1(t *testing.T) {
	db := setupIYUUDB(t)
	createTestConfig(t, db)

	reportCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/reseed/sites/reportExisting" {
			reportCalled = true
			if err := json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"msg":  "ok",
				"data": map[string]any{"sid_sha1": "auto_cached_sha1"},
			}); err != nil {
				t.Errorf("encode: %v", err)
			}
			return
		}
		if r.URL.Path == "/reseed/index/index" {
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form: %v", err)
			}
			if r.Form.Get("sid_sha1") != "auto_cached_sha1" {
				t.Errorf("expected sid_sha1='auto_cached_sha1', got '%s'", r.Form.Get("sid_sha1"))
			}
			if err := json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"msg":  "ok",
				"data": map[string]any{
					"abc123": map[string]any{
						"torrent": []map[string]any{
							{"sid": 1, "torrent_id": 42, "info_hash": "def456"},
						},
					},
				},
			}); err != nil {
				t.Errorf("encode: %v", err)
			}
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	db.Model(&model.IYUUConfig{}).Where("id = 1").Update("base_url", server.URL)
	db.Create(&model.IYUUSiteMapping{IYUUSid: 1, SiteDomain: "sitea.com", SiteName: "SiteA", Enabled: true})

	svc := NewService(db, zap.NewNop())
	results, err := svc.QueryReseed(context.Background(), []string{"abc123"})
	if err != nil {
		t.Fatal(err)
	}
	if !reportCalled {
		t.Error("expected ReportExisting to be called automatically")
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestService_QueryReseed_LegacyFormat(t *testing.T) {
	db := setupIYUUDB(t)
	createTestConfig(t, db)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/reseed/sites/reportExisting" {
			if err := json.NewEncoder(w).Encode(map[string]any{
				"code": 0, "msg": "ok",
				"data": map[string]any{"sid_sha1": "sha1"},
			}); err != nil {
				t.Errorf("encode: %v", err)
			}
			return
		}
		if r.URL.Path == "/reseed/index/index" {
			if err := json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"msg":  "ok",
				"data": map[string]any{
					"abc123": []map[string]any{
						{"sid": 1, "torrent_id": 42, "info_hash": "def456"},
					},
				},
			}); err != nil {
				t.Errorf("encode: %v", err)
			}
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	db.Model(&model.IYUUConfig{}).Where("id = 1").Update("base_url", server.URL)
	db.Create(&model.IYUUSiteMapping{IYUUSid: 1, SiteDomain: "sitea.com", SiteName: "SiteA", Enabled: true})

	svc := NewService(db, zap.NewNop())
	results, err := svc.QueryReseed(context.Background(), []string{"abc123"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Targets[0].Sid != 1 {
		t.Errorf("expected sid 1, got %d", results[0].Targets[0].Sid)
	}
}

func TestService_QueryReseed_BatchSplit(t *testing.T) {
	db := setupIYUUDB(t)
	createTestConfig(t, db)

	var batchSizes []int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/reseed/sites/reportExisting" {
			if err := json.NewEncoder(w).Encode(map[string]any{
				"code": 0, "msg": "ok",
				"data": map[string]any{"sid_sha1": "sha1"},
			}); err != nil {
				t.Errorf("encode: %v", err)
			}
			return
		}
		if r.URL.Path == "/reseed/index/index" {
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form: %v", err)
			}
			var hashes []string
			if err := json.Unmarshal([]byte(r.Form.Get("hash")), &hashes); err != nil {
				t.Fatalf("unmarshal hash: %v", err)
			}
			batchSizes = append(batchSizes, len(hashes))
			if err := json.NewEncoder(w).Encode(map[string]any{
				"code": 0, "msg": "ok", "data": map[string]any{},
			}); err != nil {
				t.Errorf("encode: %v", err)
			}
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	db.Model(&model.IYUUConfig{}).Where("id = 1").Update("base_url", server.URL)
	db.Create(&model.IYUUSiteMapping{IYUUSid: 1, SiteDomain: "sitea.com", SiteName: "SiteA", Enabled: true})

	svc := NewService(db, zap.NewNop())

	hashes := make([]string, 350)
	for i := range hashes {
		hashes[i] = fmt.Sprintf("hash_%040d", i)
	}

	results, err := svc.QueryReseed(context.Background(), hashes)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results from empty data, got %d", len(results))
	}
	if len(batchSizes) != 2 {
		t.Fatalf("expected 2 batches, got %d", len(batchSizes))
	}
	if batchSizes[0] != 200 {
		t.Errorf("expected first batch of 200, got %d", batchSizes[0])
	}
	if batchSizes[1] != 150 {
		t.Errorf("expected second batch of 150, got %d", batchSizes[1])
	}
}

func TestService_SendNotification(t *testing.T) {
	db := setupIYUUDB(t)
	createTestConfig(t, db)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Token") == "" {
			t.Error("expected Token header")
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		if r.Form.Get("text") != "test title" {
			t.Errorf("expected text 'test title', got %s", r.Form.Get("text"))
		}
		if r.Form.Get("desp") != "test body" {
			t.Errorf("expected desp 'test body', got %s", r.Form.Get("desp"))
		}
		if err := json.NewEncoder(w).Encode(map[string]any{"code": 0, "msg": "ok"}); err != nil {
			t.Errorf("encode: %v", err)
		}
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

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"msg":  "ok",
			"data": map[string]any{
				"abc123": map[string]any{
					"torrent": []map[string]any{
						{"sid": 1, "torrent_id": 1, "info_hash": "x"},
						{"sid": 2, "torrent_id": 2, "info_hash": "y"},
					},
				},
			},
		}); err != nil {
			t.Errorf("encode: %v", err)
		}
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

func TestService_doGetWithToken_SetsHeader(t *testing.T) {
	db := setupIYUUDB(t)
	createTestConfig(t, db)

	var receivedToken string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedToken = r.Header.Get("Token")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	db.Model(&model.IYUUConfig{}).Where("id = 1").Update("base_url", server.URL)

	svc := NewService(db, zap.NewNop())
	resp, err := svc.doGetWithToken(context.Background(), server.URL+"/test", "my-token-123")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	if receivedToken != "my-token-123" {
		t.Errorf("expected token 'my-token-123', got '%s'", receivedToken)
	}
}

func TestService_doPostFormWithToken_SetsHeaders(t *testing.T) {
	db := setupIYUUDB(t)
	createTestConfig(t, db)

	var receivedToken string
	var receivedCT string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedToken = r.Header.Get("Token")
		receivedCT = r.Header.Get("Content-Type")
		if err := json.NewEncoder(w).Encode(map[string]any{"code": 0}); err != nil {
			t.Errorf("encode: %v", err)
		}
	}))
	defer server.Close()

	db.Model(&model.IYUUConfig{}).Where("id = 1").Update("base_url", server.URL)

	svc := NewService(db, zap.NewNop())
	form := url.Values{}
	form.Set("key", "value")
	var result map[string]any
	if err := svc.doPostFormWithToken(context.Background(), server.URL+"/test", "my-token-456", form, &result); err != nil {
		t.Fatal(err)
	}

	if receivedToken != "my-token-456" {
		t.Errorf("expected token 'my-token-456', got '%s'", receivedToken)
	}
	if !strings.Contains(receivedCT, "application/x-www-form-urlencoded") {
		t.Errorf("expected form content type, got '%s'", receivedCT)
	}
}
