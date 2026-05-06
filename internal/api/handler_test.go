package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/auth"
	"github.com/ranfish/pt-forward/internal/model"
	notificationPkg "github.com/ranfish/pt-forward/internal/notification"
	"github.com/ranfish/pt-forward/internal/publish"
	"github.com/ranfish/pt-forward/internal/reseed"
	"github.com/ranfish/pt-forward/internal/rss"
	"github.com/ranfish/pt-forward/internal/scheduler"
	"github.com/ranfish/pt-forward/internal/seeding"
	"github.com/ranfish/pt-forward/internal/setting"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type testEnv struct {
	db           *gorm.DB
	authManager  *auth.AuthManager
	router       *Router
	mux          *http.ServeMux
	token        string
	taskRegistry *scheduler.Registry
}

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	models := []interface{}{
		&model.User{},
		&model.ClientConfig{},
		&model.ClientPathMapping{},
		&model.ClientPublishTarget{},
		&model.Site{},
		&model.RSSSubscription{},
		&model.RSSTorrentSeen{},
		&model.FilterRule{},
		&model.NotificationChannel{},
		&model.NotificationHistory{},
		&model.SeedingClientConfig{},
		&model.SeedingTorrentRecord{},
		&model.DeleteRule{},
		&model.ReseedTask{},
		&model.PublishTask{},
		&model.PublishCandidate{},
		&model.PublishResultRecord{},
		&model.TorrentEvent{},
		&model.IYUUConfig{},
		&model.IYUUSiteMapping{},
		&model.ReseedMatch{},
		&model.ReseedNegativeCache{},
		&model.ContentFingerprint{},
		&model.SearchCache{},
		&model.PublishGroup{},
		&model.PublishGroupMember{},
		&model.PublishGroupStatusHistory{},
		&model.DownloaderSpeedSnapshot{},
		&model.SiteTrafficDaily{},
		&model.TorrentTraffic{},
		&model.SeedingClientState{},
		&model.FreezeEventRecord{},
		&model.SiteConfigOverride{},
		&model.CookieCloudConfig{},
		&model.CookieCloudSyncHistory{},
		&model.PTGenCache{},
		setting.Setting{},
	}
	for _, m := range models {
		if err := db.AutoMigrate(m); err != nil {
			t.Fatalf("migrate %T: %v", m, err)
		}
	}

	logger := zap.NewNop()
	authRepo := auth.NewGormAuthRepository(db)
	authManager, err := auth.NewAuthManager(authRepo, logger)
	if err != nil {
		t.Fatalf("auth manager: %v", err)
	}

	rssEngine := rss.NewEngine(db, logger)
	notifyService := notificationPkg.NewService(db, logger)
	reseedEngine := reseed.NewEngine(db, logger)
	publishPipeline := publish.NewPipeline(db, logger)
	seedingEngine := seeding.NewEngine(db, logger)
	taskRegistry := scheduler.NewRegistry(logger)

	router := NewRouter(authManager, db, rssEngine, notifyService, reseedEngine, publishPipeline, seedingEngine, nil, taskRegistry, &mockIYUUQueryService{}, "test", logger)
	mux := http.NewServeMux()
	router.Register(mux, []string{"*"}, true, 120)

	env := &testEnv{
		db:          db,
		authManager: authManager,
		router:      router,
		mux:         mux,
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte("TestP@ss1"), 10)
	db.Create(&model.User{
		Username:     "admin",
		DisplayName:  "Admin",
		PasswordHash: string(hash),
	})

	pair, err := authManager.IssueTokenPair()
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	env.token = pair.AccessToken
	env.taskRegistry = taskRegistry

	return env
}

func (e *testEnv) doRequest(method, path string, body interface{}) *httptest.ResponseRecorder {
	var bodyReader io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(data)
	}
	req := httptest.NewRequest(method, path, bodyReader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if e.token != "" {
		req.Header.Set("Authorization", "Bearer "+e.token)
	}
	w := httptest.NewRecorder()
	e.mux.ServeHTTP(w, req)
	return w
}

func parseResponse(t *testing.T, w *httptest.ResponseRecorder) Response {
	t.Helper()
	var resp Response
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

func TestAuth_Login_Success(t *testing.T) {
	env := setupTestEnv(t)

	body := map[string]string{"username": "admin", "password": "TestP@ss1"}
	w := env.doRequest("POST", "/api/v1/auth/login", body)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	if resp.Code != 0 {
		t.Errorf("expected code 0, got %d", resp.Code)
	}
	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("response data is not a map")
	}
	if _, hasToken := data["accessToken"]; !hasToken {
		t.Error("missing accessToken")
	}
	if _, hasRefresh := data["refreshToken"]; !hasRefresh {
		t.Error("missing refreshToken")
	}
}

func TestAuth_Login_WrongPassword(t *testing.T) {
	env := setupTestEnv(t)

	body := map[string]string{"username": "admin", "password": "wrong"}
	w := env.doRequest("POST", "/api/v1/auth/login", body)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuth_Login_EmptyFields(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/auth/login", map[string]string{"username": "", "password": ""})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAuth_Login_MethodNotAllowed(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/auth/login", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestAuth_Status(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/auth/status", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["initialized"] != true {
		t.Error("should be initialized")
	}
}

func TestAuth_RefreshToken(t *testing.T) {
	env := setupTestEnv(t)

	loginBody := map[string]string{"username": "admin", "password": "TestP@ss1"}
	loginW := env.doRequest("POST", "/api/v1/auth/login", loginBody)
	loginResp := parseResponse(t, loginW)
	loginData := loginResp.Data.(map[string]interface{})
	refreshToken := loginData["refreshToken"].(string)

	w := env.doRequest("POST", "/api/v1/auth/refresh", map[string]string{"refreshToken": refreshToken})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAuth_RefreshToken_Invalid(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/auth/refresh", map[string]string{"refreshToken": "invalid"})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuth_Password(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("PUT", "/api/v1/auth/password", map[string]string{
		"oldPassword": "TestP@ss1",
		"newPassword": "NewP@ss2!",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAuth_Password_WrongOld(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("PUT", "/api/v1/auth/password", map[string]string{
		"oldPassword": "wrong",
		"newPassword": "NewP@ss2!",
	})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuth_Profile(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("PUT", "/api/v1/auth/profile", map[string]string{
		"displayName": "NewAdmin",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAuth_Profile_EmptyName(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("PUT", "/api/v1/auth/profile", map[string]string{
		"displayName": "",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestProtectedEndpoint_NoToken(t *testing.T) {
	env := setupTestEnv(t)
	env.token = ""

	w := env.doRequest("GET", "/api/v1/settings", nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestProtectedEndpoint_InvalidToken(t *testing.T) {
	env := setupTestEnv(t)
	env.token = "invalid.jwt.token"

	w := env.doRequest("GET", "/api/v1/settings", nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestSettings_CRUD(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("PUT", "/api/v1/settings/app.name", map[string]string{"value": "PT-Forward"})
	if w.Code != http.StatusOK {
		t.Fatalf("set: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("GET", "/api/v1/settings/app.name", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d", w.Code)
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["value"] != "PT-Forward" {
		t.Errorf("expected PT-Forward, got %v", data["value"])
	}

	w = env.doRequest("GET", "/api/v1/settings", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", w.Code)
	}

	w = env.doRequest("DELETE", "/api/v1/settings/app.name", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete: expected 200, got %d", w.Code)
	}

	w = env.doRequest("GET", "/api/v1/settings/app.name", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("after delete: expected 404, got %d", w.Code)
	}
}

func TestSettings_MethodNotAllowed(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/settings/app.name", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestSites_CRUD(t *testing.T) {
	env := setupTestEnv(t)

	createBody := map[string]interface{}{
		"name":      "TestSite",
		"domain":    "test.com",
		"baseUrl":   "https://test.com",
		"framework": "nexusphp",
	}
	w := env.doRequest("POST", "/api/v1/sites", createBody)
	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Fatalf("create: expected 200/201, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	siteID := data["id"].(float64)

	w = env.doRequest("GET", "/api/v1/sites", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", w.Code)
	}
	listResp := parseResponse(t, w)
	listData, _ := listResp.Data.(map[string]interface{})
	items, _ := listData["items"].([]interface{})
	if len(items) != 1 {
		t.Fatalf("expected 1 site, got %d", len(items))
	}

	w = env.doRequest("GET", fmt.Sprintf("/api/v1/sites/%d", int(siteID)), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	updateBody := map[string]interface{}{
		"name":      "UpdatedSite",
		"framework": "unit3d",
	}
	w = env.doRequest("PUT", fmt.Sprintf("/api/v1/sites/%d", int(siteID)), updateBody)
	if w.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("DELETE", fmt.Sprintf("/api/v1/sites/%d", int(siteID)), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFilters_CRUD(t *testing.T) {
	env := setupTestEnv(t)

	createBody := map[string]interface{}{
		"name":     "HD Filter",
		"ruleType": "accept",
		"priority": 10,
		"enabled":  true,
		"conditions": []map[string]interface{}{
			{"key": "title", "compare_type": "contain", "value": "HD"},
		},
	}
	w := env.doRequest("POST", "/api/v1/filters/rules", createBody)
	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Fatalf("create: expected 200/201, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	ruleID := data["id"].(float64)

	w = env.doRequest("GET", "/api/v1/filters/rules", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", w.Code)
	}

	w = env.doRequest("GET", fmt.Sprintf("/api/v1/filters/rules/%d", int(ruleID)), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("DELETE", fmt.Sprintf("/api/v1/filters/rules/%d", int(ruleID)), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDownloaders_CRUD(t *testing.T) {
	env := setupTestEnv(t)

	createBody := map[string]interface{}{
		"name":     "Test QB",
		"type":     "qbittorrent",
		"url":      "http://localhost:8080",
		"username": "admin",
		"password": "pass",
		"role":     "seeding",
		"enabled":  true,
	}
	w := env.doRequest("POST", "/api/v1/downloaders", createBody)
	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Fatalf("create: expected 200/201, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	clientID := data["id"].(float64)

	w = env.doRequest("GET", "/api/v1/downloaders", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", w.Code)
	}

	w = env.doRequest("GET", fmt.Sprintf("/api/v1/downloaders/%d", int(clientID)), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("DELETE", fmt.Sprintf("/api/v1/downloaders/%d", int(clientID)), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestNotifications_CRUD(t *testing.T) {
	env := setupTestEnv(t)

	createBody := map[string]interface{}{
		"type":    "webhook",
		"name":    "TestHook",
		"enabled": true,
		"config":  `{"url":"https://example.com/hook"}`,
		"events":  "rss,publish",
	}
	w := env.doRequest("POST", "/api/v1/notifications/channels", createBody)
	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Fatalf("create: expected 200/201, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	chID := data["id"].(float64)

	w = env.doRequest("GET", "/api/v1/notifications/channels", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", w.Code)
	}

	w = env.doRequest("GET", fmt.Sprintf("/api/v1/notifications/channels/%d", int(chID)), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("DELETE", fmt.Sprintf("/api/v1/notifications/channels/%d", int(chID)), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRSS_CRUD(t *testing.T) {
	env := setupTestEnv(t)

	siteBody := map[string]interface{}{
		"name":      "RSSSite",
		"domain":    "rss-site.com",
		"baseUrl":   "https://rss-site.com",
		"framework": "nexusphp",
	}
	env.doRequest("POST", "/api/v1/sites", siteBody)

	createBody := map[string]interface{}{
		"name":      "TestRSS",
		"siteName":  "RSSSite",
		"urls":      []string{"https://rss-site.com/rss"},
		"cron":      "*/10 * * * *",
		"enabled":   true,
		"client_id": "",
	}
	w := env.doRequest("POST", "/api/v1/rss/subscriptions", createBody)
	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Fatalf("create: expected 200/201, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	subID := data["id"].(float64)

	w = env.doRequest("GET", "/api/v1/rss/subscriptions", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", w.Code)
	}

	w = env.doRequest("GET", fmt.Sprintf("/api/v1/rss/subscriptions/%d", int(subID)), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("DELETE", fmt.Sprintf("/api/v1/rss/subscriptions/%d", int(subID)), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCORS_Headers(t *testing.T) {
	env := setupTestEnv(t)

	req := httptest.NewRequest("OPTIONS", "/api/v1/auth/status", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "GET")
	w := httptest.NewRecorder()
	env.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent && w.Code != http.StatusOK {
		t.Fatalf("CORS preflight: expected 204/200, got %d", w.Code)
	}
}

func TestSecurityHeaders(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/auth/status", nil)
	if v := w.Header().Get("X-Content-Type-Options"); v != "nosniff" {
		t.Errorf("expected nosniff, got %q", v)
	}
	if v := w.Header().Get("X-Frame-Options"); v != "DENY" {
		t.Errorf("expected DENY, got %q", v)
	}
}

func TestMapCodeToHTTP(t *testing.T) {
	tests := []struct {
		code     int
		expected int
	}{
		{40001, http.StatusBadRequest},
		{40050, http.StatusBadRequest},
		{40100, http.StatusUnauthorized},
		{40150, http.StatusUnauthorized},
		{40900, http.StatusConflict},
		{42901, http.StatusTooManyRequests},
		{40400, http.StatusNotFound},
		{50000, http.StatusInternalServerError},
		{99999, http.StatusInternalServerError},
	}

	for _, tt := range tests {
		got := mapCodeToHTTP(tt.code)
		if got != tt.expected {
			t.Errorf("mapCodeToHTTP(%d) = %d, want %d", tt.code, got, tt.expected)
		}
	}
}

func TestSeedingConfigs_CRUD(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/seeding/configs", map[string]interface{}{
		"clientId":       "seeding-client-1",
		"enabled":        true,
		"minDiskSpaceGB": 30,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	configID := data["id"].(float64)

	w = env.doRequest("GET", "/api/v1/seeding/configs", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", w.Code)
	}

	w = env.doRequest("GET", fmt.Sprintf("/api/v1/seeding/configs/%d", int(configID)), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("PUT", fmt.Sprintf("/api/v1/seeding/configs/%d", int(configID)), map[string]interface{}{
		"enabled": false,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("DELETE", fmt.Sprintf("/api/v1/seeding/configs/%d", int(configID)), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeedingRecords_List(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/seeding/records", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list records: expected 200, got %d", w.Code)
	}
}

func TestDeleteRules_CRUD(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/seeding/delete-rules", map[string]interface{}{
		"alias":      "Test Rule",
		"priority":   10,
		"enabled":    true,
		"action":     "delete",
		"deleteNum":  3,
		"removeData": true,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	ruleID := data["id"].(float64)

	w = env.doRequest("GET", "/api/v1/seeding/delete-rules", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", w.Code)
	}

	w = env.doRequest("GET", fmt.Sprintf("/api/v1/seeding/delete-rules/%d", int(ruleID)), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("DELETE", fmt.Sprintf("/api/v1/seeding/delete-rules/%d", int(ruleID)), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestReseedTasks_CRUD(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/reseed/tasks", map[string]interface{}{
		"name":          "Test Reseed",
		"enabled":       false,
		"clientIds":     "c1,c2",
		"sourceSiteIds": "1,2",
		"targetSiteIds": "3,4",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	taskID := data["id"].(float64)

	w = env.doRequest("GET", "/api/v1/reseed/tasks", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", w.Code)
	}

	w = env.doRequest("GET", fmt.Sprintf("/api/v1/reseed/tasks/%d", int(taskID)), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("PUT", fmt.Sprintf("/api/v1/reseed/tasks/%d", int(taskID)), map[string]interface{}{
		"enabled": true,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("DELETE", fmt.Sprintf("/api/v1/reseed/tasks/%d", int(taskID)), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPublishTasks_List(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/publish/tasks", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("GET", "/api/v1/publish/candidates", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list candidates: expected 200, got %d", w.Code)
	}

	w = env.doRequest("GET", "/api/v1/publish/results", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list results: expected 200, got %d", w.Code)
	}
}

func TestDashboard_Overview(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/dashboard/overview", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("overview: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data := resp["data"].(map[string]interface{})
	if _, ok := data["sites"]; !ok {
		t.Error("expected sites field")
	}
	if _, ok := data["downloaders"]; !ok {
		t.Error("expected downloaders field")
	}
	if _, ok := data["publish"]; !ok {
		t.Error("expected publish field")
	}
	if _, ok := data["reseed"]; !ok {
		t.Error("expected reseed field")
	}
	if _, ok := data["system"]; !ok {
		t.Error("expected system field")
	}
}

func TestDashboard_Activities(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/dashboard/activities", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("activities: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTorrentEvents_List(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/torrent-events", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list events: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSystem_Info(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/system/info", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("info: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data := resp["data"].(map[string]interface{})
	if data["version"] != "test" {
		t.Errorf("expected version=test, got %v", data["version"])
	}
	if _, ok := data["goVersion"]; !ok {
		t.Error("expected goVersion field")
	}
}

func TestIYUU_GetConfig(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/iyuu/config", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get config: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestIYUU_UpdateConfig(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("PUT", "/api/v1/iyuu/config", map[string]interface{}{
		"token":   "test-token-12345",
		"enabled": true,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("update config: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var cfg model.IYUUConfig
	env.db.First(&cfg)
	if cfg.Token != "test-token-12345" {
		t.Errorf("expected token=test-token-12345, got %q", cfg.Token)
	}
}

func TestIYUU_ListSites(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/iyuu/sites", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list sites: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestIYUU_Query(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/iyuu/query", map[string]interface{}{
		"infoHashes": []string{"abc123def456789"},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("query: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_Stats(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/seeding/stats", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("stats: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	data := resp["data"].(map[string]interface{})
	if _, ok := data["activeRecords"]; !ok {
		t.Error("expected activeRecords field")
	}
}

func TestSeeding_ScoringDryrun(t *testing.T) {
	env := setupTestEnv(t)

	body, _ := json.Marshal(map[string]interface{}{
		"seeders":       10,
		"leechers":      50,
		"uploadBytes":   1073741824,
		"seedTimeHours": 168,
		"ageHours":      48,
		"size":          1073741824,
		"isFree":        true,
	})
	w := env.doRequest("POST", "/api/v1/seeding/scoring-dryrun", bytes.NewReader(body))
	if w.Code != http.StatusOK {
		t.Fatalf("scoring dryrun: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	data := resp["data"].(map[string]interface{})
	if _, ok := data["effectiveScore"]; !ok {
		t.Error("expected effectiveScore field")
	}
	if _, ok := data["shouldCleanup"]; !ok {
		t.Error("expected shouldCleanup field")
	}
}

func TestRSS_Dryrun(t *testing.T) {
	env := setupTestEnv(t)

	env.db.Create(&model.Site{Name: "testsite2", Domain: "testsite2.com", BaseURL: "https://testsite2.com", Framework: "nexusphp", Enabled: true, Passkey: "pk"})

	w := env.doRequest("POST", "/api/v1/rss/subscriptions", map[string]interface{}{
		"name":     "dryrun-sub",
		"enabled":  true,
		"urls":     []string{"https://example.com/rss"},
		"siteName": "testsite2",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create sub: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var subResp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&subResp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	subData := subResp["data"].(map[string]interface{})
	subID := uint(subData["id"].(float64))

	w = env.doRequest("POST", fmt.Sprintf("/api/v1/rss/subscriptions/%d/dryrun", subID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("dryrun: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSystem_Ping(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/system/ping", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("data not map")
	}
	if data["status"] != "ok" {
		t.Errorf("expected status ok, got %v", data["status"])
	}
}

func TestSystem_Health(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/system/health", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSettings_BackupRestore(t *testing.T) {
	env := setupTestEnv(t)

	env.doRequest("PUT", "/api/v1/settings/test_key", map[string]string{"value": "test_value"})

	w := env.doRequest("GET", "/api/v1/settings/backup", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("backup: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("data not map")
	}
	if data["exported"] != true {
		t.Error("expected exported true")
	}

	w = env.doRequest("POST", "/api/v1/settings/restore", map[string]interface{}{
		"settings": map[string]string{"key_a": "val_a", "key_b": "val_b"},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("restore: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDashboard_Trends(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/dashboard/trends?days=3", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("data not map")
	}
	if data["days"] != float64(3) {
		t.Errorf("expected days=3, got %v", data["days"])
	}
}

func TestFingerprint_CRUD(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/fingerprints", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	env.db.Create(&model.ContentFingerprint{
		InfoHash: "abc123", SiteName: "site1", TorrentID: "t1",
		PiecesHash: "ph1", TotalSize: 1024, FileCount: 5, Title: "test",
	})

	w = env.doRequest("GET", "/api/v1/fingerprints", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list after create: expected 200, got %d", w.Code)
	}
	resp := parseResponse(t, w)
	data := resp.Data.(map[string]interface{})
	if data["total"] != float64(1) {
		t.Errorf("expected total 1, got %v", data["total"])
	}

	w = env.doRequest("GET", "/api/v1/fingerprints/search?infoHash=abc123", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("search: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("GET", "/api/v1/fingerprints/search?piecesHash=ph1", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("search by piecesHash: expected 200, got %d", w.Code)
	}

	w = env.doRequest("GET", "/api/v1/fingerprints/search", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("search without params: expected 400, got %d", w.Code)
	}

	w = env.doRequest("GET", "/api/v1/fingerprints/1", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("DELETE", "/api/v1/fingerprints/1", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete: expected 200, got %d", w.Code)
	}

	w = env.doRequest("DELETE", "/api/v1/fingerprints/cache", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete cache: expected 200, got %d", w.Code)
	}
}

func TestTracker_Members(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/tracker/members", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("GET", "/api/v1/tracker/members?groupId=999", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list by group: expected 200, got %d", w.Code)
	}

	w = env.doRequest("GET", "/api/v1/tracker/members/nonexistenthash", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("get nonexistent: expected 404, got %d", w.Code)
	}

	w = env.doRequest("GET", "/api/v1/tracker/history", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("history: expected 200, got %d", w.Code)
	}

	w = env.doRequest("GET", "/api/v1/tracker/history?groupId=1", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("history by group: expected 200, got %d", w.Code)
	}
}

func TestLifecycle_Config(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/lifecycle/config", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get config: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("PUT", "/api/v1/lifecycle/config", map[string]interface{}{
		"pauseSeeders":        false,
		"deleteSeedHours":     360,
		"checkInterval":       "10m",
		"maxConcurrentChecks": 5,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("update config: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("GET", "/api/v1/lifecycle/backpressure", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("backpressure: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("data not map")
	}
	if _, ok := data["isThrottled"]; !ok {
		t.Error("missing isThrottled")
	}
}

func TestSeeding_Status(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/seeding/status", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("data not map")
	}
	if data["running"] != true {
		t.Error("expected running true")
	}
}

func TestSeeding_RulesCRUD(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/seeding/rules", map[string]interface{}{
		"alias":      "rule1",
		"priority":   1,
		"enabled":    true,
		"type":       "normal",
		"conditions": `{"field":"seeders","op":"gt","value":5}`,
		"action":     "delete",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("GET", "/api/v1/seeding/rules", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", w.Code)
	}

	w = env.doRequest("GET", "/api/v1/seeding/rules/1", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("PUT", "/api/v1/seeding/rules/1", map[string]interface{}{
		"alias": "rule1-updated",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("POST", "/api/v1/seeding/rules/1/test", map[string]interface{}{
		"torrentName": "test.torrent",
		"size":        1024,
		"seeders":     10,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("test: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("DELETE", "/api/v1/seeding/rules/1", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete: expected 200, got %d", w.Code)
	}
}

func TestSeeding_TorrentsAndStats(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/seeding/torrents", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("torrents: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("GET", "/api/v1/seeding/stats/overview", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("stats overview: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("GET", "/api/v1/seeding/stats/by-site", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("stats by-site: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("GET", "/api/v1/seeding/stats/torrents", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("stats torrents: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("GET", "/api/v1/seeding/stats/by-site/mysite/trend", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("site trend: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("GET", "/api/v1/seeding/clients", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("clients: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("POST", "/api/v1/seeding/clients/test-client/trigger", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("trigger: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_ScoringConfigAndLogs(t *testing.T) {
	env := setupTestEnv(t)

	env.db.Create(&model.Site{Name: "scoringsite", Domain: "scoringsite.com", BaseURL: "https://scoringsite.com", Framework: "nexusphp", Enabled: true, Passkey: "pk"})

	w := env.doRequest("POST", "/api/v1/rss/subscriptions", map[string]interface{}{
		"name":     "scoring-sub",
		"enabled":  true,
		"urls":     []string{"https://scoringsite.com/rss"},
		"siteName": "scoringsite",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create sub: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var subResp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&subResp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	subData := subResp["data"].(map[string]interface{})
	subID := uint(subData["id"].(float64))

	w = env.doRequest("GET", fmt.Sprintf("/api/v1/seeding/scoring-config/%d", subID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get scoring config: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("PUT", fmt.Sprintf("/api/v1/seeding/scoring-config/%d", subID), map[string]interface{}{
		"enabled":       true,
		"halfLifeHours": 4.0,
		"minScore":      2.0,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("update scoring config: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("GET", "/api/v1/seeding/scoring-logs", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("scoring logs: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("GET", "/api/v1/seeding/scoring-logs/cycles/test-cycle-1", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("scoring cycle: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("POST", fmt.Sprintf("/api/v1/seeding/dryrun/%d", subID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("dryrun: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPublish_GroupsAndLifecycle(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/publish/groups", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list groups: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	env.db.Create(&model.PublishGroup{SubscriptionID: "sub1", SourceHash: "hash1", SourceSite: "site1", Status: "active"})
	env.db.Create(&model.PublishGroupMember{PublishGroupID: 1, InfoHash: "ih1", SiteName: "site1", Role: "source", Status: "new"})

	w = env.doRequest("GET", "/api/v1/publish/groups/1", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get group: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("data not map")
	}
	if _, ok := data["members"]; !ok {
		t.Error("expected members field")
	}

	w = env.doRequest("POST", "/api/v1/publish/groups/1/lifecycle/pause", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("pause: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("POST", "/api/v1/publish/groups/1/lifecycle/resume", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("resume: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("POST", "/api/v1/publish/groups/1/lifecycle/delete", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete lifecycle: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("DELETE", "/api/v1/publish/groups/1", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete group: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPublish_CandidateDeleteAndPublish(t *testing.T) {
	env := setupTestEnv(t)

	env.db.Create(&model.PublishCandidate{
		SourceSite: "site1", SourceTorrentID: "t1", InfoHash: "ih1",
		TorrentName: "test-torrent", PublishStatus: model.CandidatePending,
	})

	w := env.doRequest("DELETE", "/api/v1/publish/candidates/1", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete candidate: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	env.db.Create(&model.PublishCandidate{
		SourceSite: "site2", SourceTorrentID: "t2", InfoHash: "ih2",
		TorrentName: "good-torrent", PublishStatus: model.CandidatePending,
	})

	w = env.doRequest("POST", "/api/v1/publish/candidates/2/publish", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("manual publish: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPublish_TaskCancel(t *testing.T) {
	env := setupTestEnv(t)

	env.db.Create(&model.Site{Name: "pubsite", Domain: "pubsite.com", BaseURL: "https://pubsite.com", Framework: "nexusphp", Enabled: true, Passkey: "pk"})

	w := env.doRequest("POST", "/api/v1/publish/tasks", map[string]interface{}{
		"type":         "manual",
		"sourceSiteId": 1,
		"targetSites":  []string{"target.com"},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create task: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("POST", "/api/v1/publish/tasks/1/cancel", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("cancel task: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestReseed_RetryAndNegativeCache(t *testing.T) {
	env := setupTestEnv(t)

	env.db.Create(&model.ReseedTask{Name: "reseed1", Enabled: true, Status: "idle"})
	env.db.Create(&model.ReseedMatch{
		ClientID: "c1", SourceSite: "s1", SourceTorrentID: "t1", SourceInfoHash: "ih1",
		TargetSite: "s2", TargetTorrentID: "t2", MatchMethod: "pieces_hash",
		Confidence: 0.9, Status: model.MatchStatusFailed, FailReason: "test failure",
	})

	w := env.doRequest("POST", "/api/v1/reseed/tasks/1/matches/1/retry", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("retry match: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("DELETE", "/api/v1/reseed/tasks/1/negative-cache?infoHash=ih1&site=s1", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete negative cache: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("GET", "/api/v1/reseed/tasks/1/matches/1", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get match: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFilter_SubscriptionsRules(t *testing.T) {
	env := setupTestEnv(t)

	env.db.Create(&model.Site{Name: "rulesite", Domain: "rulesite.com", BaseURL: "https://rulesite.com", Framework: "nexusphp", Enabled: true, Passkey: "pk"})

	w := env.doRequest("POST", "/api/v1/rss/subscriptions", map[string]interface{}{
		"name":     "rule-sub",
		"enabled":  true,
		"urls":     []string{"https://rulesite.com/rss"},
		"siteName": "rulesite",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create sub: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var subResp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&subResp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	subData := subResp["data"].(map[string]interface{})
	subID := uint(subData["id"].(float64))

	w = env.doRequest("PUT", fmt.Sprintf("/api/v1/rss/subscriptions/%d/rules", subID), map[string]interface{}{
		"acceptRuleIds": []uint{1, 2},
		"rejectRuleIds": []uint{3},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("update rules: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSystem_PingFull(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/system/ping", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestSystem_HealthFull(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/system/health", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("expected map")
	}
	if data["status"] != "healthy" {
		t.Errorf("expected healthy, got %v", data["status"])
	}
	dbInfo, _ := data["database"].(map[string]interface{})
	if dbInfo["ok"] != true {
		t.Error("expected db ok=true")
	}
}

func TestSystem_InfoFull(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/system/info", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("expected map")
	}
	if data["version"] != "test" {
		t.Errorf("expected test, got %v", data["version"])
	}
}

func TestSystem_Logs(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/system/logs", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestSystem_Logs_Clear(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("DELETE", "/api/v1/system/logs", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestSystem_Logs_MethodNotAllowed(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("PATCH", "/api/v1/system/logs", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestSystem_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/system/nonexistent", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestCredentialDetector_CheckNow(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err := db.AutoMigrate(&model.Site{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	hub := NewHub()
	d := NewCredentialDetector(db, zap.NewNop(), hub)

	db.Create(&model.Site{
		Name: "test", Domain: "test.com", BaseURL: "https://test.com",
		Framework: "nexusphp", Enabled: true, Cookie: "old_cookie",
	})

	d.CheckNow(context.Background())
}

func TestCredentialDetector_NoCredentials(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err := db.AutoMigrate(&model.Site{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	d := NewCredentialDetector(db, zap.NewNop(), nil)

	db.Create(&model.Site{
		Name: "bare", Domain: "bare.com", BaseURL: "https://bare.com",
		Framework: "generic", Enabled: true,
	})

	d.CheckNow(context.Background())
}

func TestCredentialDetector_RunCancel(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err := db.AutoMigrate(&model.Site{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	d := NewCredentialDetector(db, zap.NewNop(), nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	d.Run(ctx, 1*time.Hour)
}

func TestSeeding_Clients(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/seeding/configs", map[string]interface{}{
		"clientId":       "seeding-client-1",
		"enabled":        true,
		"minDiskSpaceGB": 30,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create config: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("GET", "/api/v1/seeding/clients", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list clients: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_ScoringConfig(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/seeding/scoring-config", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_ScoringDryrunFull(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/seeding/scoring-dryrun", map[string]interface{}{
		"clientId": 1,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_Dryrun(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/seeding/dryrun/1", map[string]interface{}{
		"clientId": 1,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_StatusFull(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/seeding/status", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteRule_CreateAndList(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/seeding/delete-rules", map[string]interface{}{
		"alias": "rule1", "enabled": true, "condition": "seeders < 2", "action": "delete",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("GET", "/api/v1/seeding/delete-rules", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	items, _ := data["items"].([]interface{})
	if len(items) != 1 {
		t.Errorf("expected 1 rule, got %d", len(items))
	}
}

func TestPublish_Groups(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/publish/groups", map[string]interface{}{
		"name": "group1", "sourceSite": "s1", "targetSites": []string{"t1", "t2"},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create group: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("GET", "/api/v1/publish/groups", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list groups: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPublish_Results(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/publish/results", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list results: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestIYUU_GetConfigFull(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/iyuu/config", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestIYUU_Sites(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/iyuu/sites", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestIYUU_UpdateConfigFull(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("PUT", "/api/v1/iyuu/config", map[string]interface{}{
		"enabled": true, "apiBase": "https://iyuu.cn",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSettings_List(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/settings", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSettings_Update(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("PUT", "/api/v1/settings/test_key", map[string]interface{}{
		"value": "test_val",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTracker_MembersFull(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/tracker/members", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTracker_History(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/tracker/history", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFingerprint_List(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/fingerprints", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLifecycle_ConfigFull(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/lifecycle/config", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLifecycle_Backpressure(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/lifecycle/backpressure", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTorrentEvents(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/torrent-events", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestErrorWithDetail(t *testing.T) {
	w := httptest.NewRecorder()
	ErrorWithDetail(w, http.StatusBadRequest, 40001, "bad request", "field X is invalid")
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	var resp Response
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Code != 40001 {
		t.Errorf("expected 40001, got %d", resp.Code)
	}
	if resp.Detail != "field X is invalid" {
		t.Errorf("expected detail, got %s", resp.Detail)
	}
}

func TestSiteOverrides_CRUD(t *testing.T) {
	env := setupTestEnv(t)

	createBody := map[string]interface{}{
		"name":      "OverrideTestSite",
		"domain":    "override-test.com",
		"baseUrl":   "https://override-test.com",
		"framework": "nexusphp",
	}
	w := env.doRequest("POST", "/api/v1/sites", createBody)
	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Fatalf("create site: expected 200/201, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	siteID := data["id"].(float64)

	w = env.doRequest("GET", fmt.Sprintf("/api/v1/sites/%d/overrides", int(siteID)), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list overrides: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	listResp := parseResponse(t, w)
	listData, _ := listResp.Data.(map[string]interface{})
	if listData["total"].(float64) != 0 {
		t.Errorf("expected 0 overrides initially, got %v", listData["total"])
	}

	overrideBody := map[string]interface{}{
		"fieldPath":  "rss.free_scrape",
		"fieldValue": "true",
	}
	w = env.doRequest("PUT", fmt.Sprintf("/api/v1/sites/%d/overrides", int(siteID)), overrideBody)
	if w.Code != http.StatusOK {
		t.Fatalf("upsert override: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("GET", fmt.Sprintf("/api/v1/sites/%d/overrides", int(siteID)), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list overrides after upsert: expected 200, got %d", w.Code)
	}
	listResp = parseResponse(t, w)
	listData, _ = listResp.Data.(map[string]interface{})
	if listData["total"].(float64) != 1 {
		t.Errorf("expected 1 override, got %v", listData["total"])
	}

	overrideBody2 := map[string]interface{}{
		"fieldPath":  "rss.free_scrape",
		"fieldValue": "false",
	}
	w = env.doRequest("PUT", fmt.Sprintf("/api/v1/sites/%d/overrides", int(siteID)), overrideBody2)
	if w.Code != http.StatusOK {
		t.Fatalf("update override: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("GET", fmt.Sprintf("/api/v1/sites/%d/overrides", int(siteID)), nil)
	listResp = parseResponse(t, w)
	listData, _ = listResp.Data.(map[string]interface{})
	if listData["total"].(float64) != 1 {
		t.Errorf("expected still 1 override after update, got %v", listData["total"])
	}

	w = env.doRequest("DELETE", fmt.Sprintf("/api/v1/sites/%d/overrides?fieldPath=rss.free_scrape", int(siteID)), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete override: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("GET", fmt.Sprintf("/api/v1/sites/%d/overrides", int(siteID)), nil)
	listResp = parseResponse(t, w)
	listData, _ = listResp.Data.(map[string]interface{})
	if listData["total"].(float64) != 0 {
		t.Errorf("expected 0 after delete, got %v", listData["total"])
	}

	w = env.doRequest("PUT", fmt.Sprintf("/api/v1/sites/%d/overrides", int(siteID)), map[string]interface{}{
		"fieldPath":  "",
		"fieldValue": "x",
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("empty fieldPath: expected 400, got %d", w.Code)
	}
}

func TestCookieCloud_GetConfig_Empty(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/cookiecloud/config", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("data not map")
	}
	if data["serverUrl"] != "" {
		t.Errorf("expected empty serverUrl, got %v", data["serverUrl"])
	}
	if data["syncEnabled"] != false {
		t.Error("expected syncEnabled false")
	}
}

func TestCookieCloud_UpdateConfig_Create(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("PUT", "/api/v1/cookiecloud/config", map[string]interface{}{
		"serverUrl":   "https://cc.example.com",
		"uuid":        "test-uuid-123",
		"password":    "test-password",
		"syncEnabled": true,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCookieCloud_GetConfig_AfterCreate(t *testing.T) {
	env := setupTestEnv(t)
	env.doRequest("PUT", "/api/v1/cookiecloud/config", map[string]interface{}{
		"serverUrl":   "https://cc.example.com",
		"uuid":        "test-uuid-123",
		"password":    "test-password",
		"syncEnabled": true,
	})
	w := env.doRequest("GET", "/api/v1/cookiecloud/config", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("data not map")
	}
	if data["serverUrl"] != "https://cc.example.com" {
		t.Errorf("expected https://cc.example.com, got %v", data["serverUrl"])
	}
	if data["hasPassword"] != true {
		t.Error("expected hasPassword true")
	}
	if data["syncEnabled"] != true {
		t.Error("expected syncEnabled true")
	}
}

func TestCookieCloud_UpdateConfig_Update(t *testing.T) {
	env := setupTestEnv(t)
	env.doRequest("PUT", "/api/v1/cookiecloud/config", map[string]interface{}{
		"serverUrl":   "https://cc.example.com",
		"uuid":        "test-uuid-123",
		"password":    "test-password",
		"syncEnabled": true,
	})
	w := env.doRequest("PUT", "/api/v1/cookiecloud/config", map[string]interface{}{
		"serverUrl": "https://cc2.example.com",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	w = env.doRequest("GET", "/api/v1/cookiecloud/config", nil)
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["serverUrl"] != "https://cc2.example.com" {
		t.Errorf("expected updated serverUrl, got %v", data["serverUrl"])
	}
}

func TestCookieCloud_Sync_NoSites(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/cookiecloud/sync", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCookieCloud_ListHistory_Empty(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/cookiecloud/history", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("data not map")
	}
	items, ok := data["items"].([]interface{})
	if !ok {
		t.Fatal("items not array")
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestCookieCloud_TestConnection_NoConfig(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/cookiecloud/test", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCookieCloud_BadMethod(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("DELETE", "/api/v1/cookiecloud/config", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestCookieCloud_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/cookiecloud/nonexistent", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestPTGen_Query_EmptyQuery(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/ptgen/query", map[string]interface{}{
		"query": "",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestPTGen_Query_BadJSON(t *testing.T) {
	env := setupTestEnv(t)
	req := httptest.NewRequest("POST", "/api/v1/ptgen/query", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.token)
	w := httptest.NewRecorder()
	env.mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestPTGen_ListCache_Empty(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/ptgen/cache", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("data not map")
	}
	items, ok := data["items"].([]interface{})
	if !ok {
		t.Fatal("items not array")
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestPTGen_CleanCache(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("DELETE", "/api/v1/ptgen/cache", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPTGen_BadMethod(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("PUT", "/api/v1/ptgen/query", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestPTGen_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/ptgen/nonexistent", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestPublish_GetTask_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/publish/tasks/99999", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	if resp.Code != 40400 {
		t.Errorf("expected code 40400, got %d", resp.Code)
	}
}

func TestPublish_DeleteTask_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("DELETE", "/api/v1/publish/tasks/99999", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	if resp.Code != 40400 {
		t.Errorf("expected code 40400, got %d", resp.Code)
	}
}

func TestPublish_GetCandidate_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/publish/candidates/99999", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	if resp.Code != 40400 {
		t.Errorf("expected code 40400, got %d", resp.Code)
	}
}

func TestFilter_Update(t *testing.T) {
	env := setupTestEnv(t)

	createBody := map[string]interface{}{
		"name":     "FilterUpdTest",
		"ruleType": "accept",
		"enabled":  true,
		"conditions": []map[string]interface{}{
			{"key": "title", "compare_type": "contain", "value": "HD"},
		},
	}
	w := env.doRequest("POST", "/api/v1/filters/rules", createBody)
	if w.Code != http.StatusOK {
		t.Fatalf("create: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	ruleID := data["id"].(float64)

	updateBody := map[string]interface{}{
		"name":    "FilterUpdModified",
		"enabled": false,
	}
	w = env.doRequest("PUT", fmt.Sprintf("/api/v1/filters/rules/%d", int(ruleID)), updateBody)
	if w.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp = parseResponse(t, w)
	data, _ = resp.Data.(map[string]interface{})
	if data["name"] != "FilterUpdModified" {
		t.Errorf("expected FilterUpdModified, got %v", data["name"])
	}
	if data["enabled"] != false {
		t.Errorf("expected enabled false, got %v", data["enabled"])
	}
}

func TestFilter_Update_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("PUT", "/api/v1/filters/rules/99999", map[string]interface{}{
		"name": "NotExist",
	})
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteRule_CreateAndUpdate(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/seeding/delete-rules", map[string]interface{}{
		"alias":      "UpdDelRule",
		"priority":   5,
		"enabled":    true,
		"action":     "delete",
		"deleteNum":  1,
		"removeData": false,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	ruleID := data["id"].(float64)

	w = env.doRequest("PUT", fmt.Sprintf("/api/v1/seeding/delete-rules/%d", int(ruleID)), map[string]interface{}{
		"alias":    "UpdDelRuleMod",
		"priority": 20,
		"enabled":  false,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp = parseResponse(t, w)
	data, _ = resp.Data.(map[string]interface{})
	if data["alias"] != "UpdDelRuleMod" {
		t.Errorf("expected UpdDelRuleMod, got %v", data["alias"])
	}
}

func TestDeleteRule_Update_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("PUT", "/api/v1/seeding/delete-rules/99999", map[string]interface{}{
		"alias": "NotExist",
	})
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSettings_SetAndGet(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("PUT", "/api/v1/settings/setgetkey", map[string]string{"value": "setgetval"})
	if w.Code != http.StatusOK {
		t.Fatalf("set: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["key"] != "setgetkey" {
		t.Errorf("expected key setgetkey, got %v", data["key"])
	}

	w = env.doRequest("GET", "/api/v1/settings/setgetkey", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp = parseResponse(t, w)
	data, _ = resp.Data.(map[string]interface{})
	if data["value"] != "setgetval" {
		t.Errorf("expected setgetval, got %v", data["value"])
	}
}

func TestRSS_CreateAndUpdate(t *testing.T) {
	env := setupTestEnv(t)

	env.db.Create(&model.Site{Name: "rssupd", Domain: "rssupd.com", BaseURL: "https://rssupd.com", Framework: "nexusphp", Enabled: true, Passkey: "pk"})

	createBody := map[string]interface{}{
		"name":     "RSSUpdTest",
		"enabled":  true,
		"urls":     []string{"https://rssupd.com/rss"},
		"siteName": "rssupd",
	}
	w := env.doRequest("POST", "/api/v1/rss/subscriptions", createBody)
	if w.Code != http.StatusOK {
		t.Fatalf("create: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	subID := data["id"].(float64)

	updateBody := map[string]interface{}{
		"name":    "RSSUpdModified",
		"enabled": false,
	}
	w = env.doRequest("PUT", fmt.Sprintf("/api/v1/rss/subscriptions/%d", int(subID)), updateBody)
	if w.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp = parseResponse(t, w)
	data, _ = resp.Data.(map[string]interface{})
	if data["name"] != "RSSUpdModified" {
		t.Errorf("expected RSSUpdModified, got %v", data["name"])
	}
	if data["enabled"] != false {
		t.Errorf("expected enabled false, got %v", data["enabled"])
	}
}

func TestScheduler_ListTasks(t *testing.T) {
	env := setupTestEnv(t)

	_ = env.taskRegistry.Register("test_task_1", "test", "*/5 * * * *", func(taskCtx context.Context) error {
		return nil
	})

	w := env.doRequest("GET", "/api/v1/scheduler/tasks", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Items []struct {
				Name string `json:"name"`
			} `json:"items"`
			Total int `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Data.Total != 1 {
		t.Errorf("expected 1 task, got %d", resp.Data.Total)
	}
	if len(resp.Data.Items) != 1 || resp.Data.Items[0].Name != "test_task_1" {
		t.Errorf("unexpected items: %+v", resp.Data.Items)
	}
}

func TestScheduler_GetTask(t *testing.T) {
	env := setupTestEnv(t)

	_ = env.taskRegistry.Register("test_task_2", "test", "*/5 * * * *", func(taskCtx context.Context) error {
		return nil
	})

	w := env.doRequest("GET", "/api/v1/scheduler/tasks/test_task_2", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Data.Name != "test_task_2" {
		t.Errorf("expected test_task_2, got %s", resp.Data.Name)
	}
}

func TestScheduler_GetTask_NotFound(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/scheduler/tasks/nonexistent", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestScheduler_PauseResume(t *testing.T) {
	env := setupTestEnv(t)

	_ = env.taskRegistry.Register("test_task_3", "test", "*/5 * * * *", func(taskCtx context.Context) error {
		return nil
	})

	w := env.doRequest("POST", "/api/v1/scheduler/tasks/test_task_3/pause", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("pause: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	task, _ := env.taskRegistry.Get("test_task_3")
	if !task.Paused {
		t.Error("task should be paused")
	}

	w = env.doRequest("POST", "/api/v1/scheduler/tasks/test_task_3/resume", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("resume: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	task, _ = env.taskRegistry.Get("test_task_3")
	if task.Paused {
		t.Error("task should not be paused after resume")
	}
}

func TestScheduler_Trigger(t *testing.T) {
	env := setupTestEnv(t)

	_ = env.taskRegistry.Register("test_task_4", "test", "*/5 * * * *", func(taskCtx context.Context) error {
		return nil
	})

	w := env.doRequest("POST", "/api/v1/scheduler/tasks/test_task_4/trigger", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("trigger: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	task, _ := env.taskRegistry.Get("test_task_4")
	if task.SuccessCount != 1 {
		t.Errorf("expected 1 success, got %d", task.SuccessCount)
	}
}

func TestScheduler_Reschedule(t *testing.T) {
	env := setupTestEnv(t)

	_ = env.taskRegistry.Register("test_task_5", "test", "*/5 * * * *", func(taskCtx context.Context) error {
		return nil
	})

	w := env.doRequest("PUT", "/api/v1/scheduler/tasks/test_task_5/schedule", map[string]interface{}{
		"schedule": "*/10 * * * *",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("reschedule: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	task, _ := env.taskRegistry.Get("test_task_5")
	if task.Schedule != "*/10 * * * *" {
		t.Errorf("expected schedule */10 * * * *, got %q", task.Schedule)
	}
}

func TestScheduler_Reschedule_InvalidCron(t *testing.T) {
	env := setupTestEnv(t)

	_ = env.taskRegistry.Register("test_task_6", "test", "*/5 * * * *", func(taskCtx context.Context) error {
		return nil
	})

	w := env.doRequest("PUT", "/api/v1/scheduler/tasks/test_task_6/schedule", map[string]interface{}{
		"schedule": "invalid",
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}

	task, _ := env.taskRegistry.Get("test_task_6")
	if task.Schedule != "*/5 * * * *" {
		t.Errorf("schedule should not change on invalid input, got %q", task.Schedule)
	}
}

type mockIYUUQueryService struct{}

func (m *mockIYUUQueryService) QueryReseed(_ context.Context, infoHashes []string) ([]*model.IYUUReseedResult, error) {
	return []*model.IYUUReseedResult{}, nil
}
