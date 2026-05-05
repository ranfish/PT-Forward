package cookiecloud

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
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
	if err := db.AutoMigrate(
		&model.CookieCloudConfig{},
		&model.CookieCloudSyncHistory{},
		&model.Site{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
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

	badPad := []byte{1, 2, 3, 4, 5, 6, 7, 2}
	_, err = pkcs7Strip(badPad, 8)
	if err == nil {
		t.Error("expected error for bad padding")
	}

	zeroPad := []byte{1, 2, 3, 4, 5, 6, 7, 0}
	_, err = pkcs7Strip(zeroPad, 8)
	if err == nil {
		t.Error("expected error for zero padding")
	}
}

func encryptCryptoJSAES(password string, plaintext []byte) string {
	salt := make([]byte, 8)
	_, _ = io.ReadFull(rand.Reader, salt)
	key, iv := bytesToKey(salt, []byte(password), md5.New(), aes256KeyLen, blockLen)
	block, _ := aes.NewCipher(key)
	padLen := blockLen - len(plaintext)%blockLen
	padded := append(plaintext, bytes.Repeat([]byte{byte(padLen)}, padLen)...)
	ct := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(ct, padded)
	raw := append([]byte("Salted__"), salt...)
	raw = append(raw, ct...)
	return base64.StdEncoding.EncodeToString(raw)
}

func TestBytesToKey(t *testing.T) {
	salt := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	data := []byte("password123")
	key, iv := bytesToKey(salt, data, md5.New(), aes256KeyLen, blockLen)
	if len(key) != aes256KeyLen {
		t.Errorf("key length = %d, want %d", len(key), aes256KeyLen)
	}
	if len(iv) != blockLen {
		t.Errorf("iv length = %d, want %d", len(iv), blockLen)
	}
}

func TestDecryptCryptoJSAES_RoundTrip(t *testing.T) {
	password := "testkey1234567890"
	plaintext := []byte(`{"cookie_data":{"example.com":[{"name":"session","value":"abc"}]}}`)
	encrypted := encryptCryptoJSAES(password, plaintext)

	decrypted, err := decryptCryptoJSAES(password, encrypted)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("roundtrip mismatch:\ngot  %s\nwant %s", decrypted, plaintext)
	}
}

func TestDecryptCryptoJSAES_WrongPassword(t *testing.T) {
	plaintext := []byte(`{"cookie_data":{}}`)
	encrypted := encryptCryptoJSAES("correctpassword", plaintext)
	_, err := decryptCryptoJSAES("wrongpassword", encrypted)
	if err == nil {
		t.Error("expected error with wrong password")
	}
}

func TestDecryptCryptoJSAES_InvalidInputs(t *testing.T) {
	_, err := decryptCryptoJSAES("key", "not-base64!!!")
	if err == nil {
		t.Error("expected error for invalid base64")
	}

	short := base64.StdEncoding.EncodeToString([]byte("short"))
	_, err = decryptCryptoJSAES("key", short)
	if err == nil {
		t.Error("expected error for too-short ciphertext")
	}

	noSalt := make([]byte, 32)
	noSalt[0] = 'X'
	_, err = decryptCryptoJSAES("key", base64.StdEncoding.EncodeToString(noSalt))
	if err == nil {
		t.Error("expected error for missing Salted__ prefix")
	}

	badAlign := append([]byte("Salted__"), make([]byte, 5)...)
	_, err = decryptCryptoJSAES("key", base64.StdEncoding.EncodeToString(badAlign))
	if err == nil {
		t.Error("expected error for non-block-aligned ciphertext")
	}
}

func TestFetchAndDecrypt_Success(t *testing.T) {
	password := "testpass"
	plaintext := []byte(`{"cookie_data":{".example.com":[{"name":"sid","value":"abc","domain":".example.com"}]}}`)
	keyPassword := md5String("test-uuid", "-", password)[:16]
	encrypted := encryptCryptoJSAES(keyPassword, plaintext)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/get/test-uuid" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]string{"encrypted": encrypted}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("encode: %v", err)
		}
	}))
	defer srv.Close()

	cookies, err := FetchAndDecrypt(srv.URL, "test-uuid", password)
	if err != nil {
		t.Fatalf("FetchAndDecrypt failed: %v", err)
	}
	if len(cookies) == 0 {
		t.Error("expected cookies")
	}
}

func TestFetchAndDecrypt_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := FetchAndDecrypt(srv.URL, "test-uuid", "pass")
	if err == nil {
		t.Error("expected error for 500")
	}
}

func TestFetchAndDecrypt_EmptyEncrypted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]string{"encrypted": ""}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("encode: %v", err)
		}
	}))
	defer srv.Close()

	_, err := FetchAndDecrypt(srv.URL, "test-uuid", "pass")
	if err == nil {
		t.Error("expected error for empty encrypted data")
	}
}

func TestFetchAndDecrypt_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()

	_, err := FetchAndDecrypt(srv.URL, "test-uuid", "pass")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestFetchAndDecrypt_TrailingSlash(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/get/uuid" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		keyPassword := md5String("uuid", "-", "pw")[:16]
		encrypted := encryptCryptoJSAES(keyPassword, []byte(`{"cookie_data":{}}`))
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]string{"encrypted": encrypted}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("encode: %v", err)
		}
	}))
	defer srv.Close()

	_, err := FetchAndDecrypt(srv.URL+"/", "uuid", "pw")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFilterCookiesByDomain_WWWPrefix(t *testing.T) {
	cookies := map[string][]CookieData{
		"b1": {
			{Name: "x", Value: "1", Domain: "www.example.com"},
			{Name: "y", Value: "2", Domain: ".example.com"},
		},
	}
	result := FilterCookiesByDomain(cookies, "www.example.com")
	if len(result) != 2 {
		t.Errorf("expected 2, got %d", len(result))
	}
}

func TestFilterCookiesByDomain_Dedup(t *testing.T) {
	cookies := map[string][]CookieData{
		"b1": {
			{Name: "sid", Value: "a", Domain: ".example.com"},
		},
		"b2": {
			{Name: "sid", Value: "a", Domain: ".example.com"},
		},
	}
	result := FilterCookiesByDomain(cookies, "example.com")
	if len(result) != 1 {
		t.Errorf("expected 1 deduplicated, got %d", len(result))
	}
}

func TestBuildCookieDomains_Primary(t *testing.T) {
	db := setupCookieCloudDB(t)
	svc := NewSyncService(db, zap.NewNop())

	site := &model.Site{
		Name:              "test",
		BaseURL:           "https://www.example.com",
		CookieCloudDomain: "example.com",
	}
	domains := svc.buildCookieDomains(site)
	if len(domains) != 1 || domains[0] != "example.com" {
		t.Errorf("expected [example.com], got %v", domains)
	}
}

func TestBuildCookieDomains_FromBaseURL(t *testing.T) {
	db := setupCookieCloudDB(t)
	svc := NewSyncService(db, zap.NewNop())

	site := &model.Site{
		Name:    "test",
		BaseURL: "https://sub.example.com/path",
	}
	domains := svc.buildCookieDomains(site)
	if len(domains) != 1 || domains[0] != "sub.example.com" {
		t.Errorf("expected [sub.example.com], got %v", domains)
	}
}

func TestBuildCookieDomains_WithAlternatives(t *testing.T) {
	db := setupCookieCloudDB(t)
	svc := NewSyncService(db, zap.NewNop())

	alts, _ := json.Marshal([]string{"https://alt1.com", "https://alt2.com"})
	site := &model.Site{
		Name:               "test",
		BaseURL:            "https://primary.com",
		AlternativeDomains: string(alts),
	}
	domains := svc.buildCookieDomains(site)
	if len(domains) != 3 {
		t.Errorf("expected 3 domains, got %d: %v", len(domains), domains)
	}
	found := make(map[string]bool)
	for _, d := range domains {
		found[d] = true
	}
	if !found["primary.com"] || !found["alt1.com"] || !found["alt2.com"] {
		t.Errorf("missing domains: %v", domains)
	}
}

func TestBuildCookieDomains_DeduplicatesAlternatives(t *testing.T) {
	db := setupCookieCloudDB(t)
	svc := NewSyncService(db, zap.NewNop())

	alts, _ := json.Marshal([]string{"https://primary.com"})
	site := &model.Site{
		Name:               "test",
		BaseURL:            "https://primary.com",
		AlternativeDomains: string(alts),
	}
	domains := svc.buildCookieDomains(site)
	if len(domains) != 1 {
		t.Errorf("expected 1 (deduplicated), got %d: %v", len(domains), domains)
	}
}

func TestBuildCookieDomains_Empty(t *testing.T) {
	db := setupCookieCloudDB(t)
	svc := NewSyncService(db, zap.NewNop())

	site := &model.Site{}
	domains := svc.buildCookieDomains(site)
	if len(domains) != 0 {
		t.Errorf("expected 0 domains for empty site, got %d", len(domains))
	}
}

func TestGetConfig_Incomplete(t *testing.T) {
	db := setupCookieCloudDB(t)
	svc := NewSyncService(db, zap.NewNop())

	cfg := &model.CookieCloudConfig{
		ServerURL:   "http://localhost",
		SyncEnabled: true,
	}
	if err := db.Create(cfg).Error; err != nil {
		t.Fatal(err)
	}

	_, err := svc.getConfig(context.Background())
	if err == nil {
		t.Error("expected error for incomplete config")
	}
	if fmt.Sprintf("%v", err) == "" {
		t.Error("error should mention incomplete")
	}
}

func TestUpdateConfigSyncTime(t *testing.T) {
	db := setupCookieCloudDB(t)
	svc := NewSyncService(db, zap.NewNop())

	cfg := &model.CookieCloudConfig{
		ServerURL:   "http://localhost",
		UUID:        "u",
		Password:    "p",
		SyncEnabled: true,
	}
	if err := db.Create(cfg).Error; err != nil {
		t.Fatal(err)
	}

	svc.updateConfigSyncTime(context.Background())

	var updated model.CookieCloudConfig
	if err := db.First(&updated).Error; err != nil {
		t.Fatal(err)
	}
	if updated.LastSyncAt == nil {
		t.Error("expected last_sync_at to be set")
	}
}

func TestSyncAll_HappyPath(t *testing.T) {
	password := "testpw"
	keyPassword := md5String("uuid1", "-", password)[:16]
	cookieData := map[string][]CookieData{
		".example.com": {
			{Name: "session", Value: "abc123", Domain: ".example.com"},
		},
	}
	plainBytes, _ := json.Marshal(map[string]map[string][]CookieData{"cookie_data": cookieData})
	encrypted := encryptCryptoJSAES(keyPassword, plainBytes)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]string{"encrypted": encrypted}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("encode: %v", err)
		}
	}))
	defer srv.Close()

	db := setupCookieCloudDB(t)
	svc := NewSyncService(db, zap.NewNop())

	cfg := &model.CookieCloudConfig{
		ServerURL:   srv.URL,
		UUID:        "uuid1",
		Password:    password,
		SyncEnabled: true,
	}
	if err := db.Create(cfg).Error; err != nil {
		t.Fatal(err)
	}

	site := &model.Site{
		Name:            "TestSite",
		BaseURL:         "https://example.com",
		Enabled:         true,
		CookieCloudSync: true,
	}
	if err := db.Create(site).Error; err != nil {
		t.Fatal(err)
	}

	history, err := svc.SyncAll(context.Background())
	if err != nil {
		t.Fatalf("SyncAll failed: %v", err)
	}
	if history.Status != "completed" {
		t.Errorf("expected completed, got %s", history.Status)
	}
	if history.SyncedSites != 1 {
		t.Errorf("expected 1 synced, got %d", history.SyncedSites)
	}

	var updated model.Site
	if err := db.First(&updated, site.ID).Error; err != nil {
		t.Fatal(err)
	}
	if updated.Cookie != "session=abc123" {
		t.Errorf("expected cookie 'session=abc123', got %q", updated.Cookie)
	}
}

func TestSyncAll_SkippedNoCookies(t *testing.T) {
	password := "pw2"
	keyPassword := md5String("uuid2", "-", password)[:16]
	plainBytes, _ := json.Marshal(map[string]map[string][]CookieData{"cookie_data": {}})
	encrypted := encryptCryptoJSAES(keyPassword, plainBytes)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]string{"encrypted": encrypted}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("encode: %v", err)
		}
	}))
	defer srv.Close()

	db := setupCookieCloudDB(t)
	svc := NewSyncService(db, zap.NewNop())

	cfg := &model.CookieCloudConfig{
		ServerURL:   srv.URL,
		UUID:        "uuid2",
		Password:    password,
		SyncEnabled: true,
	}
	if err := db.Create(cfg).Error; err != nil {
		t.Fatal(err)
	}

	site := &model.Site{
		Name:            "NoCookieSite",
		BaseURL:         "https://nodomain.com",
		Enabled:         true,
		CookieCloudSync: true,
	}
	if err := db.Create(site).Error; err != nil {
		t.Fatal(err)
	}

	history, err := svc.SyncAll(context.Background())
	if err != nil {
		t.Fatalf("SyncAll failed: %v", err)
	}
	if history.SkippedSites != 1 {
		t.Errorf("expected 1 skipped, got %d", history.SkippedSites)
	}
}

func TestSyncAll_AlternativeDomainMatch(t *testing.T) {
	password := "pw3"
	keyPassword := md5String("uuid3", "-", password)[:16]
	cookieData := map[string][]CookieData{
		".alt.example.com": {
			{Name: "token", Value: "xyz", Domain: ".alt.example.com"},
		},
	}
	plainBytes, _ := json.Marshal(map[string]map[string][]CookieData{"cookie_data": cookieData})
	encrypted := encryptCryptoJSAES(keyPassword, plainBytes)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]string{"encrypted": encrypted}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("encode: %v", err)
		}
	}))
	defer srv.Close()

	db := setupCookieCloudDB(t)
	svc := NewSyncService(db, zap.NewNop())

	cfg := &model.CookieCloudConfig{
		ServerURL:   srv.URL,
		UUID:        "uuid3",
		Password:    password,
		SyncEnabled: true,
	}
	if err := db.Create(cfg).Error; err != nil {
		t.Fatal(err)
	}

	alts, _ := json.Marshal([]string{"https://alt.example.com"})
	site := &model.Site{
		Name:               "AltSite",
		BaseURL:            "https://main.example.com",
		Enabled:            true,
		CookieCloudSync:    true,
		AlternativeDomains: string(alts),
	}
	if err := db.Create(site).Error; err != nil {
		t.Fatal(err)
	}

	history, err := svc.SyncAll(context.Background())
	if err != nil {
		t.Fatalf("SyncAll failed: %v", err)
	}
	if history.SyncedSites != 1 {
		t.Errorf("expected 1 synced via alt domain, got %d", history.SyncedSites)
	}
}

func TestSyncAll_FetchFailure(t *testing.T) {
	db := setupCookieCloudDB(t)
	svc := NewSyncService(db, zap.NewNop())

	cfg := &model.CookieCloudConfig{
		ServerURL:   "http://127.0.0.1:1",
		UUID:        "uuid",
		Password:    "pw",
		SyncEnabled: true,
	}
	if err := db.Create(cfg).Error; err != nil {
		t.Fatal(err)
	}

	site := &model.Site{
		Name:            "FailSite",
		BaseURL:         "https://example.com",
		Enabled:         true,
		CookieCloudSync: true,
	}
	if err := db.Create(site).Error; err != nil {
		t.Fatal(err)
	}

	history, err := svc.SyncAll(context.Background())
	if err == nil {
		t.Error("expected error for unreachable server")
	}
	if history.Status != "failed" {
		t.Errorf("expected failed, got %s", history.Status)
	}
}
