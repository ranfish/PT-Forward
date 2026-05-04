package cookiecloud

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupCookieCloudDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.AutoMigrate(
		&model.CookieCloudConfig{},
		&model.CookieCloudSyncHistory{},
		&model.Site{},
	)
	return db
}

func TestDecryptCryptoJSAES(t *testing.T) {
	key := "0123456789abcdef"
	plaintext := `{"cookie_data":{}}`

	_ = base64.StdEncoding.EncodeToString([]byte("Salted__" + string(make([]byte, 8)) + plaintext))
	_, err := decryptCryptoJSAES(key, "invalid")
	if err == nil {
		t.Error("expected error for invalid ciphertext")
	}
}

func TestDomainMatches(t *testing.T) {
	tests := []struct {
		cookie string
		target string
		expect bool
	}{
		{".example.com", "example.com", true},
		{"example.com", "example.com", true},
		{".sub.example.com", "example.com", true},
		{".other.com", "example.com", false},
		{"example.com", "sub.example.com", false},
	}
	for _, tt := range tests {
		got := DomainMatches(tt.cookie, tt.target)
		if got != tt.expect {
			t.Errorf("DomainMatches(%q, %q) = %v, want %v", tt.cookie, tt.target, got, tt.expect)
		}
	}
}

func TestFilterCookiesByDomain(t *testing.T) {
	cookies := map[string][]CookieData{
		"browser1": {
			{Name: "session", Value: "abc", Domain: ".example.com"},
			{Name: "csrf", Value: "def", Domain: ".example.com"},
			{Name: "other", Value: "xyz", Domain: ".other.com"},
		},
	}

	result := FilterCookiesByDomain(cookies, "example.com")
	if len(result) != 2 {
		t.Fatalf("expected 2, got %d", len(result))
	}
}

func TestFilterCookiesByDomain_Empty(t *testing.T) {
	result := FilterCookiesByDomain(nil, "example.com")
	if result != nil {
		t.Error("expected nil for nil input")
	}
}

func TestCookiesToString(t *testing.T) {
	cookies := []CookieData{
		{Name: "a", Value: "1"},
		{Name: "b", Value: "2"},
	}
	result := CookiesToString(cookies)
	if result != "a=1; b=2" {
		t.Errorf("expected 'a=1; b=2', got %q", result)
	}
}

func TestExtractDomain(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{"https://example.com/path", "example.com"},
		{"http://sub.example.com:8080/", "sub.example.com:8080"},
		{"example.com/", "example.com"},
		{"", ""},
	}
	for _, tt := range tests {
		got := extractDomain(tt.input)
		if got != tt.expect {
			t.Errorf("extractDomain(%q) = %q, want %q", tt.input, got, tt.expect)
		}
	}
}

func TestSyncService_NoConfig_NoSites(t *testing.T) {
	db := setupCookieCloudDB(t)
	svc := NewSyncService(db, zap.NewNop())

	history, err := svc.SyncAll(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if history.Status != "completed" {
		t.Errorf("expected completed (no sites to sync), got %s", history.Status)
	}
}

func TestSyncService_ConfigMissingWithSite(t *testing.T) {
	db := setupCookieCloudDB(t)
	svc := NewSyncService(db, zap.NewNop())

	site := &model.Site{
		Name:            "testsite",
		BaseURL:         "https://example.com",
		Enabled:         true,
		CookieCloudSync: true,
	}
	db.Create(site)
	db.Model(site).Update("cookie_cloud_sync", true)

	history, err := svc.SyncAll(context.Background())
	if err == nil {
		t.Error("expected error when config is missing but sites need sync")
	}
	if history.Status != "failed" {
		t.Errorf("expected failed, got %s", history.Status)
	}
}

func TestSyncService_NoSites(t *testing.T) {
	db := setupCookieCloudDB(t)
	svc := NewSyncService(db, zap.NewNop())

	cfg := &model.CookieCloudConfig{
		ServerURL:   "http://localhost:8088",
		UUID:        "test-uuid",
		Password:    "test-password",
		SyncEnabled: true,
	}
	db.Create(cfg)

	history, err := svc.SyncAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if history.Status != "completed" {
		t.Errorf("expected completed, got %s", history.Status)
	}
	if history.SyncedSites != 0 {
		t.Errorf("expected 0 synced, got %d", history.SyncedSites)
	}
}

func TestSyncService_DisabledConfig(t *testing.T) {
	db := setupCookieCloudDB(t)
	svc := NewSyncService(db, zap.NewNop())

	cfg := &model.CookieCloudConfig{
		ServerURL:   "http://localhost:8088",
		UUID:        "test-uuid",
		Password:    "test-password",
		SyncEnabled: true,
	}
	db.Create(cfg)
	db.Model(cfg).Update("sync_enabled", false)

	site := &model.Site{
		Name:            "testsite",
		BaseURL:         "https://example.com",
		Enabled:         true,
		CookieCloudSync: true,
	}
	db.Create(site)
	db.Model(site).Update("cookie_cloud_sync", true)

	_, err := svc.SyncAll(context.Background())
	if err == nil {
		t.Error("expected error for disabled sync")
	}
}

func TestMd5String(t *testing.T) {
	result := md5String("test", "-", "key")
	if len(result) != 32 {
		t.Errorf("expected 32 char hex, got %d", len(result))
	}
}

func TestPkcs7Strip(t *testing.T) {
	data := []byte{1, 2, 3, 4, 5, 3, 3, 3}
	result, err := pkcs7Strip(data, 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 5 {
		t.Errorf("expected 5, got %d", len(result))
	}

	_, err = pkcs7Strip(nil, 16)
	if err == nil {
		t.Error("expected error for nil data")
	}

	_, err = pkcs7Strip([]byte{1, 2, 3}, 16)
	if err == nil {
		t.Error("expected error for non-aligned data")
	}
}
