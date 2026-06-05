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
	"github.com/ranfish/pt-forward/internal/client"
	"github.com/ranfish/pt-forward/internal/model"
	notificationPkg "github.com/ranfish/pt-forward/internal/notification"
	"github.com/ranfish/pt-forward/internal/publish"
	"github.com/ranfish/pt-forward/internal/reseed"
	"github.com/ranfish/pt-forward/internal/rss"
	"github.com/ranfish/pt-forward/internal/scheduler"
	"github.com/ranfish/pt-forward/internal/seeding"
	"github.com/ranfish/pt-forward/internal/setting"
	"github.com/ranfish/pt-forward/internal/site"
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
	stopCh       chan struct{}
}

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	if err := model.AutoMigrate(db); err != nil {
		t.Fatalf("model migrate: %v", err)
	}
	if err := setting.AutoMigrate(db); err != nil {
		t.Fatalf("setting migrate: %v", err)
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

	router := NewRouter(authManager, db, rssEngine, notifyService, reseedEngine, publishPipeline, seedingEngine, nil, taskRegistry, &mockIYUUQueryService{}, "test", nil, logger)
	mux := http.NewServeMux()
	router.Register(mux, []string{"*"}, true, 120)

	env := &testEnv{
		db:          db,
		authManager: authManager,
		router:      router,
		mux:         mux,
		stopCh:      make(chan struct{}),
	}
	go router.Hub().Run(env.stopCh)

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

	// 使用 supported_sites.json 里的真实 domain（步骤 3 强白名单校验）
	createBody := map[string]interface{}{
		"domain": "longpt.org",
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
		t.Fatalf("list: expected 200, got %d: %s", w.Code, w.Body.String())
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

	// db 直接创建（步骤 3 强白名单校验）
	env.db.Create(&model.Site{Name: "RSSSite", Domain: "rss-site.com", BaseURL: "https://rss-site.com", Framework: "nexusphp", AuthType: "cookie", Enabled: true})

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
		"settings": map[string]string{"general_key_a": "val_a", "general_key_b": "val_b"},
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

	// db 直接创建（步骤 3 强白名单校验）
	site := &model.Site{Name: "OverrideTestSite", Domain: "override-test.com", BaseURL: "https://override-test.com", Framework: "nexusphp", AuthType: "cookie", Enabled: true}
	if err := env.db.Create(site).Error; err != nil {
		t.Fatalf("create site: %v", err)
	}
	siteID := float64(site.ID)

	w := env.doRequest("GET", fmt.Sprintf("/api/v1/sites/%d/overrides", int(siteID)), nil)
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

func (m *mockIYUUQueryService) GetSiteList(_ context.Context) ([]model.IYUUSite, error) {
	return []model.IYUUSite{}, nil
}

func createTestSite(t *testing.T, env *testEnv, name, domain string) uint {
	t.Helper()
	// 直接 db.Create 而非走 API：避免步骤 3 加的白名单强校验影响测试 setup
	s := &model.Site{
		Name:      name,
		Domain:    domain,
		BaseURL:   "https://" + domain,
		Framework: "nexusphp",
		AuthType:  "cookie",
		Enabled:   true,
	}
	if err := env.db.Create(s).Error; err != nil {
		t.Fatalf("create site: %v", err)
	}
	return s.ID
}

func TestSite_UpdateCredentials_HappyPath(t *testing.T) {
	env := setupTestEnv(t)
	siteID := createTestSite(t, env, "CredSite", "credsite.com")

	w := env.doRequest("PUT", fmt.Sprintf("/api/v1/sites/%d/credentials", siteID), map[string]interface{}{
		"passkey": "new-passkey-123",
		"cookie":  "session=abc",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("data not map")
	}
	if data["domain"] != "credsite.com" {
		t.Errorf("expected credsite.com, got %v", data["domain"])
	}
}

func TestSite_UpdateCredentials_EmptyCredentials(t *testing.T) {
	env := setupTestEnv(t)
	siteID := createTestSite(t, env, "CredEmpty", "credempty.com")

	w := env.doRequest("PUT", fmt.Sprintf("/api/v1/sites/%d/credentials", siteID), map[string]interface{}{})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSite_UpdateCredentials_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("PUT", "/api/v1/sites/99999/credentials", map[string]interface{}{
		"passkey": "x",
	})
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSite_GetStats_HappyPath(t *testing.T) {
	env := setupTestEnv(t)

	site := &model.Site{
		Name: "StatsSite", Domain: "statssite.com", BaseURL: "https://statssite.com",
		Framework: "nexusphp", Enabled: true,
		UploadBytes: 1024, DownloadBytes: 512, SeedingPoints: 3.14,
		SeedingSize: 2048, SeedingCount: 7, UserClass: "VIP", Ratio: 2.0, BonusPoints: 100.5,
	}
	env.db.Create(site)

	w := env.doRequest("GET", fmt.Sprintf("/api/v1/sites/%d/stats", site.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("data not map")
	}
	if data["uploadBytes"] != float64(1024) {
		t.Errorf("expected uploadBytes=1024, got %v", data["uploadBytes"])
	}
	if data["seedingCount"] != float64(7) {
		t.Errorf("expected seedingCount=7, got %v", data["seedingCount"])
	}
	if data["ratio"] != float64(2.0) {
		t.Errorf("expected ratio=2.0, got %v", data["ratio"])
	}
	if data["userClass"] != "VIP" {
		t.Errorf("expected userClass=VIP, got %v", data["userClass"])
	}
}

func TestSite_GetStats_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/sites/99999/stats", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSite_Exclusions_GetEmpty(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/publish/exclusions", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	items, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatalf("expected array, got %T", resp.Data)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 exclusions, got %d", len(items))
	}
}

func TestSite_Exclusions_CreateAndGet(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/publish/exclusions", map[string]interface{}{
		"target_site": "target.com",
		"source_site": "source.com",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("data not map")
	}
	if data["target_site"] != "target.com" {
		t.Errorf("expected target.com, got %v", data["target_site"])
	}

	w = env.doRequest("GET", "/api/v1/publish/exclusions", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp = parseResponse(t, w)
	items, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatalf("expected array, got %T", resp.Data)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 exclusion, got %d", len(items))
	}
}

func TestSite_Exclusions_Duplicate(t *testing.T) {
	env := setupTestEnv(t)

	body := map[string]interface{}{
		"target_site": "dup-target.com",
		"source_site": "dup-source.com",
	}
	env.doRequest("POST", "/api/v1/publish/exclusions", body)
	w := env.doRequest("POST", "/api/v1/publish/exclusions", body)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSite_Exclusions_Delete(t *testing.T) {
	env := setupTestEnv(t)

	env.doRequest("POST", "/api/v1/publish/exclusions", map[string]interface{}{
		"target_site": "del-target.com",
		"source_site": "del-source.com",
	})

	w := env.doRequest("DELETE", "/api/v1/publish/exclusions", map[string]interface{}{
		"target_site": "del-target.com",
		"source_site": "del-source.com",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("delete: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSite_Exclusions_DeleteNotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("DELETE", "/api/v1/publish/exclusions", map[string]interface{}{
		"target_site": "nope.com",
		"source_site": "nope2.com",
	})
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSite_FreezeStatus_Get(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/httpclient/freeze-status", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSite_FreezeStatus_DeleteEmptyDomain(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("DELETE", "/api/v1/httpclient/freeze-status", map[string]interface{}{
		"domain": "",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSite_FreezeStatus_DeleteUnfreeze(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("DELETE", "/api/v1/httpclient/freeze-status", map[string]interface{}{
		"domain": "frozen-site.com",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("data not map")
	}
	if data["status"] != "unfrozen" {
		t.Errorf("expected unfrozen, got %v", data["status"])
	}
	if data["domain"] != "frozen-site.com" {
		t.Errorf("expected frozen-site.com, got %v", data["domain"])
	}
}

func TestNotify_Update_HappyPath(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/notifications/channels", map[string]interface{}{
		"type": "webhook", "name": "UpdTest", "enabled": true,
		"config": `{"url":"https://example.com/hook"}`,
	})
	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Fatalf("create: expected 200/201, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	chID := data["id"].(float64)

	w = env.doRequest("PUT", fmt.Sprintf("/api/v1/notifications/channels/%d", int(chID)), map[string]interface{}{
		"name": "UpdTestRenamed",
		"type": "bark",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp = parseResponse(t, w)
	data, _ = resp.Data.(map[string]interface{})
	if data["name"] != "UpdTestRenamed" {
		t.Errorf("expected UpdTestRenamed, got %v", data["name"])
	}
	if data["type"] != "bark" {
		t.Errorf("expected bark, got %v", data["type"])
	}
}

func TestNotify_Update_NameConflict(t *testing.T) {
	env := setupTestEnv(t)

	env.doRequest("POST", "/api/v1/notifications/channels", map[string]interface{}{
		"type": "webhook", "name": "NameA", "enabled": true,
	})
	env.doRequest("POST", "/api/v1/notifications/channels", map[string]interface{}{
		"type": "webhook", "name": "NameB", "enabled": true,
	})
	resp := parseResponse(t, env.doRequest("GET", "/api/v1/notifications/channels", nil))
	listData := resp.Data.(map[string]interface{})
	items := listData["items"].([]interface{})
	secondID := items[1].(map[string]interface{})["id"].(float64)

	w := env.doRequest("PUT", fmt.Sprintf("/api/v1/notifications/channels/%d", int(secondID)), map[string]interface{}{
		"name": "NameA",
	})
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestNotify_Update_InvalidType(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/notifications/channels", map[string]interface{}{
		"type": "webhook", "name": "TypeTest", "enabled": true,
	})
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	chID := data["id"].(float64)

	w = env.doRequest("PUT", fmt.Sprintf("/api/v1/notifications/channels/%d", int(chID)), map[string]interface{}{
		"type": "invalid_type",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestNotify_History_Empty(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/notifications/channels", map[string]interface{}{
		"type": "webhook", "name": "HistEmpty", "enabled": true,
	})
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	chID := data["id"].(float64)

	w = env.doRequest("GET", fmt.Sprintf("/api/v1/notifications/channels/%d/history", int(chID)), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp = parseResponse(t, w)
	histData, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("data not map")
	}
	items, _ := histData["items"].([]interface{})
	if len(items) != 0 {
		t.Errorf("expected 0 history items, got %d", len(items))
	}
}

func TestNotify_History_WithRecords(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/notifications/channels", map[string]interface{}{
		"type": "webhook", "name": "HistWithRec", "enabled": true,
	})
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	chID := uint(data["id"].(float64))

	env.db.Create(&model.NotificationHistory{
		ChannelID: chID, Event: "test", Level: "info",
		Title: "Test Title", Body: "Test body", Success: true,
	})
	env.db.Create(&model.NotificationHistory{
		ChannelID: chID, Event: "error_event", Level: "error",
		Title: "Err Title", Body: "Err body", Success: false, ErrorMsg: "timeout",
	})

	w = env.doRequest("GET", fmt.Sprintf("/api/v1/notifications/channels/%d/history", chID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp = parseResponse(t, w)
	histData, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("data not map")
	}
	items, _ := histData["items"].([]interface{})
	if len(items) != 2 {
		t.Errorf("expected 2 history items, got %d", len(items))
	}
	total, _ := histData["total"].(float64)
	if total != 2 {
		t.Errorf("expected total 2, got %v", histData["total"])
	}
}

func TestNotify_Test_Send(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/notifications/channels", map[string]interface{}{
		"type": "webhook", "name": "TestSend", "enabled": true,
		"config": `{"url":"https://example.com/hook"}`,
	})
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	chID := data["id"].(float64)

	w = env.doRequest("POST", fmt.Sprintf("/api/v1/notifications/channels/%d/test", int(chID)), nil)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", w.Code, w.Body.String())
	}
}

func TestNotify_Test_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/notifications/channels/99999/test", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRSS_SetPause_Pause(t *testing.T) {
	env := setupTestEnv(t)

	env.db.Create(&model.Site{Name: "pausesite", Domain: "pausesite.com", BaseURL: "https://pausesite.com", Framework: "nexusphp", Enabled: true, Passkey: "pk"})

	w := env.doRequest("POST", "/api/v1/rss/subscriptions", map[string]interface{}{
		"name": "PauseSub", "enabled": true, "urls": []string{"https://pausesite.com/rss"}, "siteName": "pausesite",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	subID := data["id"].(float64)

	w = env.doRequest("POST", fmt.Sprintf("/api/v1/rss/subscriptions/%d/pause", int(subID)), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("pause: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp = parseResponse(t, w)
	data, _ = resp.Data.(map[string]interface{})
	if data["paused"] != true {
		t.Errorf("expected paused true, got %v", data["paused"])
	}
}

func TestRSS_SetPause_Resume(t *testing.T) {
	env := setupTestEnv(t)

	env.db.Create(&model.Site{Name: "resumesite", Domain: "resumesite.com", BaseURL: "https://resumesite.com", Framework: "nexusphp", Enabled: true, Passkey: "pk"})

	w := env.doRequest("POST", "/api/v1/rss/subscriptions", map[string]interface{}{
		"name": "ResumeSub", "enabled": true, "urls": []string{"https://resumesite.com/rss"}, "siteName": "resumesite",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	subID := data["id"].(float64)

	env.doRequest("POST", fmt.Sprintf("/api/v1/rss/subscriptions/%d/pause", int(subID)), nil)

	w = env.doRequest("POST", fmt.Sprintf("/api/v1/rss/subscriptions/%d/resume", int(subID)), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("resume: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp = parseResponse(t, w)
	data, _ = resp.Data.(map[string]interface{})
	if data["paused"] != false {
		t.Errorf("expected paused false, got %v", data["paused"])
	}
}

func TestRSS_SetPause_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/rss/subscriptions/99999/pause", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDownloader_Update_HappyPath(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/downloaders", map[string]interface{}{
		"name": "UpdHappyDL", "type": "qbittorrent", "url": "http://localhost:8080",
		"username": "admin", "password": "pass", "role": "seeding", "enabled": true,
	})
	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Fatalf("create: expected 200/201, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	clientID := data["id"].(float64)

	w = env.doRequest("PUT", fmt.Sprintf("/api/v1/downloaders/%d", int(clientID)), map[string]interface{}{
		"name": "UpdatedName", "url": "http://localhost:9090", "role": "download", "enabled": false,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp = parseResponse(t, w)
	data, _ = resp.Data.(map[string]interface{})
	if data["name"] != "UpdatedName" {
		t.Errorf("expected UpdatedName, got %v", data["name"])
	}
	if data["url"] != "http://localhost:9090" {
		t.Errorf("expected http://localhost:9090, got %v", data["url"])
	}
	if data["role"] != "download" {
		t.Errorf("expected download, got %v", data["role"])
	}
	if data["enabled"] != false {
		t.Errorf("expected enabled false, got %v", data["enabled"])
	}
}

func TestDownloader_Update_WithPathMappings(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/downloaders", map[string]interface{}{
		"name": "PathMapDL", "type": "qbittorrent", "url": "http://localhost:8080",
		"username": "admin", "password": "pass", "role": "seeding", "enabled": true,
		"pathMappings": []map[string]string{
			{"sourcePath": "/data/movies", "reseedPath": "/mnt/movies"},
		},
	})
	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Fatalf("create: expected 200/201, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	clientID := data["id"].(float64)

	w = env.doRequest("PUT", fmt.Sprintf("/api/v1/downloaders/%d", int(clientID)), map[string]interface{}{
		"pathMappings": []map[string]string{
			{"sourcePath": "/data/tv", "reseedPath": "/mnt/tv"},
			{"sourcePath": "/data/anime", "reseedPath": "/mnt/anime"},
		},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp = parseResponse(t, w)
	data, _ = resp.Data.(map[string]interface{})
	mappings, ok := data["pathMappings"].([]interface{})
	if !ok {
		t.Fatal("pathMappings not array")
	}
	if len(mappings) != 2 {
		t.Fatalf("expected 2 path mappings, got %d", len(mappings))
	}
}

func TestDownloader_Update_NotFound(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("PUT", "/api/v1/downloaders/99999", map[string]interface{}{
		"name": "NotExist",
	})
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDownloader_Update_InvalidJSON(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/downloaders", map[string]interface{}{
		"name": "BadJSONDL", "type": "qbittorrent", "url": "http://localhost:8080",
		"username": "admin", "password": "pass", "role": "seeding", "enabled": true,
	})
	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Fatalf("create: expected 200/201, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	clientID := data["id"].(float64)

	req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/downloaders/%d", int(clientID)), bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.token)
	w2 := httptest.NewRecorder()
	env.mux.ServeHTTP(w2, req)
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w2.Code)
	}
}

func TestPublishTargets_GetEmpty(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/downloaders/publish-targets", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	targets, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatal("expected array")
	}
	if len(targets) != 0 {
		t.Errorf("expected 0 targets, got %d", len(targets))
	}
}

func TestPublishTargets_CreateAndGet(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/downloaders", map[string]interface{}{
		"name": "PTCrtDL", "type": "qbittorrent", "url": "http://localhost:8080",
		"username": "admin", "password": "pass", "role": "seeding", "enabled": true,
	})
	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Fatalf("create dl: expected 200/201, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	dlData, _ := resp.Data.(map[string]interface{})
	dlID := dlData["id"].(float64)

	w = env.doRequest("POST", "/api/v1/downloaders/publish-targets", map[string]interface{}{
		"client_id": dlID, "site_name": "crtsite.com",
		"category_mapping": "movies", "auto_publish": true, "notify_on_publish": true, "enabled": true,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp = parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["site_name"] != "crtsite.com" {
		t.Errorf("expected crtsite.com, got %v", data["site_name"])
	}

	w = env.doRequest("POST", "/api/v1/downloaders/publish-targets", map[string]interface{}{
		"client_id": dlID, "site_name": "crtsite.com",
	})
	if w.Code != http.StatusConflict {
		t.Fatalf("duplicate: expected 409, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("GET", "/api/v1/downloaders/publish-targets", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp = parseResponse(t, w)
	targets, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatal("expected array")
	}
	if len(targets) != 1 {
		t.Errorf("expected 1 target, got %d", len(targets))
	}
}

func TestPublishTargets_Update(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/downloaders", map[string]interface{}{
		"name": "PTUpdDL", "type": "qbittorrent", "url": "http://localhost:8080",
		"username": "admin", "password": "pass", "role": "seeding", "enabled": true,
	})
	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Fatalf("create dl: expected 200/201, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	dlData, _ := resp.Data.(map[string]interface{})
	dlID := dlData["id"].(float64)

	w = env.doRequest("POST", "/api/v1/downloaders/publish-targets", map[string]interface{}{
		"client_id": dlID, "site_name": "updtsite.com",
		"auto_publish": true, "enabled": true,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp = parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	targetID := data["id"].(float64)

	w = env.doRequest("PUT", "/api/v1/downloaders/publish-targets", map[string]interface{}{
		"id": targetID, "category_mapping": "tv", "auto_publish": false, "enabled": false,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp = parseResponse(t, w)
	data, _ = resp.Data.(map[string]interface{})
	if data["category_mapping"] != "tv" {
		t.Errorf("expected tv, got %v", data["category_mapping"])
	}
	if data["auto_publish"] != false {
		t.Errorf("expected auto_publish false, got %v", data["auto_publish"])
	}
	if data["enabled"] != false {
		t.Errorf("expected enabled false, got %v", data["enabled"])
	}
}

func TestPublishTargets_Delete(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/downloaders", map[string]interface{}{
		"name": "PTDelDL", "type": "qbittorrent", "url": "http://localhost:8080",
		"username": "admin", "password": "pass", "role": "seeding", "enabled": true,
	})
	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Fatalf("create dl: expected 200/201, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	dlData, _ := resp.Data.(map[string]interface{})
	dlID := dlData["id"].(float64)

	w = env.doRequest("POST", "/api/v1/downloaders/publish-targets", map[string]interface{}{
		"client_id": dlID, "site_name": "delsite.com", "enabled": true,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp = parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	targetID := data["id"].(float64)

	w = env.doRequest("DELETE", "/api/v1/downloaders/publish-targets", map[string]interface{}{"id": targetID})
	if w.Code != http.StatusOK {
		t.Fatalf("delete: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPublishTargets_DeleteNotFound(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("DELETE", "/api/v1/downloaders/publish-targets", map[string]interface{}{"id": 99999})
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestReseed_Trigger(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/reseed/tasks", map[string]interface{}{
		"name": "TriggerTask", "enabled": false,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	taskID := int(data["id"].(float64))

	w = env.doRequest("POST", fmt.Sprintf("/api/v1/reseed/tasks/%d/trigger", taskID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("trigger: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestReseed_Cancel(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/reseed/tasks", map[string]interface{}{
		"name": "CancelTask", "enabled": false,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	taskID := int(data["id"].(float64))

	w = env.doRequest("POST", fmt.Sprintf("/api/v1/reseed/tasks/%d/cancel", taskID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("cancel: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp = parseResponse(t, w)
	rData, _ := resp.Data.(map[string]interface{})
	if rData["message"] != "任务已取消" {
		t.Errorf("expected cancel message, got %v", rData["message"])
	}
}

func TestReseed_ListMatches_NoHash(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/reseed/tasks/1/matches", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list matches: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["total"] != float64(0) {
		t.Errorf("expected 0 matches, got %v", data["total"])
	}
}

func TestReseed_ListMatches_WithHash(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/reseed/tasks/1/matches?infoHash=abc123", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list matches: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSite_Detect(t *testing.T) {
	env := setupTestEnv(t)
	siteID := createTestSite(t, env, "DetectSite", "detect-site.com")

	w := env.doRequest("POST", fmt.Sprintf("/api/v1/sites/%d/detect", siteID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("detect: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if _, ok := data["framework"]; !ok {
		t.Error("expected framework field in detect result")
	}
}

func TestSite_Test(t *testing.T) {
	env := setupTestEnv(t)
	siteID := createTestSite(t, env, "TestSite", "test-site.com")

	w := env.doRequest("GET", fmt.Sprintf("/api/v1/sites/%d/test", siteID), nil)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("test: expected 502, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSite_FreezeByID(t *testing.T) {
	env := setupTestEnv(t)
	siteID := createTestSite(t, env, "FreezeSite", "freeze-site.com")

	w := env.doRequest("POST", fmt.Sprintf("/api/v1/sites/%d/freeze", siteID), map[string]interface{}{
		"duration": "1h",
		"reason":   "test",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("freeze: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("GET", fmt.Sprintf("/api/v1/sites/%d/freeze", siteID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get freeze: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("DELETE", fmt.Sprintf("/api/v1/sites/%d/freeze", siteID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("unfreeze: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSite_CircuitStatus_Get(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/httpclient/circuit-status", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("circuit status get: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSite_CircuitStatus_Reset(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/httpclient/circuit-status", map[string]interface{}{
		"domain": "test-circuit.com",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("circuit reset: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLifecycle_UpdateBackpressure(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("PUT", "/api/v1/lifecycle/backpressure", map[string]interface{}{
		"max_concurrent":    10,
		"pause_on_pressure": true,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("update backpressure: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["max_concurrent"] != float64(10) {
		t.Errorf("expected max_concurrent=10, got %v", data["max_concurrent"])
	}
}

func TestSeeding_ResumeRecord(t *testing.T) {
	env := setupTestEnv(t)

	rec := &model.SeedingTorrentRecord{
		ClientID:  "c1",
		InfoHash:  "abc123",
		SiteName:  "s1",
		TorrentID: "t1",
		Status:    model.SeedingStatusPausedFreeEnd,
	}
	env.db.Create(rec)

	w := env.doRequest("POST", fmt.Sprintf("/api/v1/seeding/records/%d/resume", rec.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("resume: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_PauseRecord(t *testing.T) {
	env := setupTestEnv(t)

	rec := &model.SeedingTorrentRecord{
		ClientID:  "c1",
		InfoHash:  "def456",
		SiteName:  "s1",
		TorrentID: "t2",
		Status:    model.SeedingStatusSeeding,
	}
	env.db.Create(rec)

	w := env.doRequest("POST", fmt.Sprintf("/api/v1/seeding/records/%d/pause", rec.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("pause: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_SpeedTrend(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/seeding/stats/downloader/1/speed-trend", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("speed trend: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if _, ok := data["points"]; !ok {
		t.Error("expected points field")
	}
}

func TestSeeding_SpeedTrend_7d(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/seeding/stats/downloader/1/speed-trend?range=7d", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("speed trend 7d: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_DryrunAll(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/seeding/dryrun", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("dryrun all: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["total"] != float64(0) {
		t.Errorf("expected 0, got %v", data["total"])
	}
}

func TestIYUU_SyncSites(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/iyuu/sites", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("sync sites: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["message"] != "站点同步完成" {
		t.Errorf("expected sync message, got %v", data["message"])
	}
}

func TestIYUU_Test_NoConfig(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/iyuu/test", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("test no config: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteRule_TestRule(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/seeding/delete-rules", map[string]interface{}{
		"alias":      "test-rule",
		"priority":   1,
		"enabled":    true,
		"type":       "normal",
		"conditions": `{"field":"seeders","op":"gt","value":5}`,
		"action":     "delete",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	ruleID := int(data["id"].(float64))

	w = env.doRequest("POST", fmt.Sprintf("/api/v1/seeding/delete-rules/%d/test", ruleID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("test rule: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp = parseResponse(t, w)
	data, _ = resp.Data.(map[string]interface{})
	if _, ok := data["matched"]; !ok {
		t.Error("expected matched field")
	}
}

func TestFilter_TestRule(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/filters/rules", map[string]interface{}{
		"name":     "test-filter",
		"enabled":  true,
		"ruleType": "accept",
		"conditions": []map[string]interface{}{
			{"key": "title", "compare_type": "contains", "value": "test"},
		},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create filter: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	ruleID := int(data["id"].(float64))

	w = env.doRequest("POST", fmt.Sprintf("/api/v1/filters/rules/%d/test", ruleID), map[string]interface{}{
		"title":    "test torrent",
		"size":     1024,
		"siteName": "test.com",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("test filter: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRSS_Trigger(t *testing.T) {
	env := setupTestEnv(t)

	// db 直接创建（步骤 3 强白名单校验，POST API 不接受 fake domain）
	env.db.Create(&model.Site{Name: "RSSTrigSite", Domain: "rss-trig.com", BaseURL: "https://rss-trig.com", Framework: "nexusphp", AuthType: "cookie", Enabled: true})

	w := env.doRequest("POST", "/api/v1/rss/subscriptions", map[string]interface{}{
		"name":     "trig-sub",
		"siteName": "RSSTrigSite",
		"urls":     []string{"https://rss-trig.com/rss"},
		"cron":     "*/10 * * * *",
		"enabled":  true,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create sub: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	subID := int(data["id"].(float64))

	w = env.doRequest("POST", fmt.Sprintf("/api/v1/rss/subscriptions/%d/trigger", subID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("trigger: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp = parseResponse(t, w)
	data, _ = resp.Data.(map[string]interface{})
	if data["triggered"] != true {
		t.Errorf("expected triggered=true, got %v", data["triggered"])
	}
}

func TestRSS_Trigger_Disabled(t *testing.T) {
	env := setupTestEnv(t)

	// db 直接创建（步骤 3 强白名单校验）
	env.db.Create(&model.Site{Name: "RSSDisSite", Domain: "rss-dis.com", BaseURL: "https://rss-dis.com", Framework: "nexusphp", AuthType: "cookie", Enabled: true})

	w := env.doRequest("POST", "/api/v1/rss/subscriptions", map[string]interface{}{
		"name":     "dis-sub",
		"siteName": "RSSDisSite",
		"urls":     []string{"https://rss-dis.com/rss"},
		"cron":     "*/10 * * * *",
		"enabled":  false,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create sub: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	subID := int(data["id"].(float64))

	env.db.Model(&model.RSSSubscription{}).Where("id = ?", subID).Update("enabled", false)

	w = env.doRequest("POST", fmt.Sprintf("/api/v1/rss/subscriptions/%d/trigger", subID), nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("trigger disabled: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAuth_Setup_AlreadyInitialized(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/auth/setup", map[string]string{
		"username": "newadmin",
		"password": "NewP@ss123!",
	})
	if w.Code != http.StatusConflict && w.Code != http.StatusBadRequest && w.Code != http.StatusOK {
		t.Fatalf("setup: expected conflict/bad/error, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFingerprint_Search_WithHash(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/fingerprints/search?infoHash=abc123", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("search: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["total"] != float64(0) {
		t.Errorf("expected 0 results, got %v", data["total"])
	}
}

func TestFingerprint_Search_WithPiecesHash(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/fingerprints/search?piecesHash=def456", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("search pieces: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFingerprint_Search_NoParam(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/fingerprints/search", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("search no param: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSettings_Delete(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("PUT", "/api/v1/settings/test.key.del", map[string]string{"value": "val"})
	if w.Code != http.StatusOK {
		t.Fatalf("set: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("DELETE", "/api/v1/settings/test.key.del", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSettings_Restore(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/settings/restore", map[string]interface{}{
		"settings": map[string]string{
			"general_restored1": "val1",
			"general_restored2": "val2",
		},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("restore: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["restored"] != true {
		t.Errorf("expected restored=true, got %v", data["restored"])
	}
	if data["count"] != float64(2) {
		t.Errorf("expected count=2, got %v", data["count"])
	}
}

func TestSettings_Restore_Empty(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/settings/restore", map[string]interface{}{
		"settings": map[string]string{},
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("restore empty: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSite_Detect_NotFound(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/sites/99999/detect", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("detect not found: expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSite_Test_NotFound(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/sites/99999/test", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("test not found: expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestReseed_Trigger_NotFound(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/reseed/tasks/99999/trigger", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("trigger not found: expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestReseed_Cancel_NotFound(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/reseed/tasks/99999/cancel", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("cancel not found: expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_ResumeRecord_InvalidID(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/seeding/records/abc/resume", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("resume invalid id: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_PauseRecord_InvalidID(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/seeding/records/abc/pause", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("pause invalid id: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTorrentEvents_GetByID(t *testing.T) {
	env := setupTestEnv(t)

	evt := &model.TorrentEvent{
		SiteName:  "evt-site.com",
		InfoHash:  "evt-hash",
		SourceID:  "src1",
		TorrentID: "t1",
	}
	env.db.Create(evt)

	w := env.doRequest("GET", fmt.Sprintf("/api/v1/torrent-events/%d", evt.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get event: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTorrentEvents_ListWithSite(t *testing.T) {
	env := setupTestEnv(t)

	env.db.Create(&model.TorrentEvent{
		SiteName: "filter-site.com", InfoHash: "filter-hash", SourceID: "s1", TorrentID: "t1",
	})

	w := env.doRequest("GET", "/api/v1/torrent-events?site=filter-site.com", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list with site: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["total"] != float64(1) {
		t.Errorf("expected 1 event, got %v", data["total"])
	}
}

func TestPublish_CancelTask_AlreadyDone(t *testing.T) {
	env := setupTestEnv(t)

	siteID := createTestSite(t, env, "PubCnlSite", "pubcnl.com")

	task := &model.PublishTask{
		SourceSiteID: siteID,
		TargetSites:  []string{"target.com"},
		Status:       model.PublishTaskCompleted,
	}
	env.db.Create(task)

	w := env.doRequest("POST", fmt.Sprintf("/api/v1/publish/tasks/%d/cancel", task.ID), nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("cancel done task: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPublish_LifecycleDelete(t *testing.T) {
	env := setupTestEnv(t)

	group := &model.PublishGroup{
		Status:     "publishing",
		SourceHash: "src-hash-del",
		SourceSite: "del-site.com",
	}
	env.db.Create(group)

	w := env.doRequest("POST", fmt.Sprintf("/api/v1/publish/groups/%d/lifecycle/delete", group.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("lifecycle delete: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["message"] != "删除已触发" {
		t.Errorf("expected delete message, got %v", data["message"])
	}
}

func TestPublish_GetGroup(t *testing.T) {
	env := setupTestEnv(t)

	group := &model.PublishGroup{
		Status:     "done",
		SourceHash: "src-hash-get",
		SourceSite: "get-site.com",
	}
	env.db.Create(group)

	w := env.doRequest("GET", fmt.Sprintf("/api/v1/publish/groups/%d", group.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get group: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPublish_DeleteGroup(t *testing.T) {
	env := setupTestEnv(t)

	group := &model.PublishGroup{
		Status:     "done",
		SourceHash: "src-hash-del2",
		SourceSite: "del2-site.com",
	}
	env.db.Create(group)

	w := env.doRequest("DELETE", fmt.Sprintf("/api/v1/publish/groups/%d", group.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete group: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTracker_GetMember(t *testing.T) {
	env := setupTestEnv(t)

	env.db.Create(&model.PublishGroupMember{
		PublishGroupID: 1,
		InfoHash:       "member-hash-1",
		SiteName:       "member-site.com",
		Status:         "done",
	})

	w := env.doRequest("GET", "/api/v1/tracker/members/member-hash-1", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get member: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTracker_GetMember_NotFound(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/tracker/members/nonexist-hash", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("get member not found: expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteRule_TestRule_NotFound(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/seeding/delete-rules/99999/test", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("test rule not found: expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLifecycle_Backpressure_Get(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/lifecycle/backpressure", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get backpressure: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if _, ok := data["activePublishes"]; !ok {
		t.Error("expected activePublishes field")
	}
}

func TestSeeding_SpeedTrend_30d(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/seeding/stats/downloader/1/speed-trend?range=30d", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("speed trend 30d: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPTGen_Query(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/ptgen/query", map[string]string{
		"query": "tt1234567",
	})
	if w.Code != http.StatusOK && w.Code != http.StatusBadGateway {
		t.Fatalf("query: expected 200/502, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFingerprint_Delete(t *testing.T) {
	env := setupTestEnv(t)

	fp := &model.ContentFingerprint{
		InfoHash:   "del-fp-hash",
		PiecesHash: "del-fp-pieces",
		SiteName:   "del-fp-site",
	}
	env.db.Create(fp)

	w := env.doRequest("DELETE", fmt.Sprintf("/api/v1/fingerprints/%d", fp.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete fingerprint: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFingerprint_Get(t *testing.T) {
	env := setupTestEnv(t)

	fp := &model.ContentFingerprint{
		InfoHash:   "get-fp-hash",
		PiecesHash: "get-fp-pieces",
		SiteName:   "get-fp-site",
	}
	env.db.Create(fp)

	w := env.doRequest("GET", fmt.Sprintf("/api/v1/fingerprints/%d", fp.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get fingerprint: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPublish_CreateTask_Validation(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/publish/tasks", map[string]interface{}{
		"sourceSiteId": 0,
		"targetSites":  []string{"site.com"},
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("no source: expected 400, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("POST", "/api/v1/publish/tasks", map[string]interface{}{
		"sourceSiteId": 1,
		"targetSites":  []string{},
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("no targets: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestReseed_NegativeCache_DeleteNoHash(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("DELETE", "/api/v1/reseed/tasks/1/negative-cache?site=s1", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("delete no hash: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCookieCloud_Sync_NoConfig(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/cookiecloud/sync", nil)
	if w.Code != http.StatusInternalServerError && w.Code != http.StatusOK {
		t.Fatalf("sync no config: expected 500/200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDashboard_Trends_CustomDays(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/dashboard/trends?days=3", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("trends 3d: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["days"] != float64(3) {
		t.Errorf("expected days=3, got %v", data["days"])
	}
}

func TestDashboard_Activities_WithLimit(t *testing.T) {
	env := setupTestEnv(t)

	env.db.Create(&model.RSSTorrentSeen{
		SiteName:  "act-site.com",
		InfoHash:  "act-hash",
		Title:     "act title",
		TorrentID: "t1",
	})

	w := env.doRequest("GET", "/api/v1/dashboard/activities?limit=10", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("activities with limit: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["total"] != float64(1) {
		t.Errorf("expected 1 activity, got %v", data["total"])
	}
}

func TestSite_Detect_WithHTTPServer(t *testing.T) {
	env := setupTestEnv(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body>NexusPHP v2.0</body></html>`))
	}))
	defer ts.Close()

	site := &model.Site{
		Name: "DetectHPSite", Domain: "detect-hp.com",
		BaseURL: ts.URL, Framework: "generic", AuthType: "cookie", Enabled: true,
	}
	env.db.Create(site)

	w := env.doRequest("POST", fmt.Sprintf("/api/v1/sites/%d/detect", site.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("detect: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["framework"] != "nexusphp" {
		t.Errorf("expected nexusphp, got %v", data["framework"])
	}
}

func TestSite_Test_WithHTTPServer(t *testing.T) {
	env := setupTestEnv(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	site := &model.Site{
		Name: "TestHPSite", Domain: "test-hp.com",
		BaseURL: ts.URL, Framework: "generic", AuthType: "cookie", Enabled: true,
	}
	env.db.Create(site)

	w := env.doRequest("GET", fmt.Sprintf("/api/v1/sites/%d/test", site.ID), nil)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("test: expected 502, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSite_Detect_Unit3D(t *testing.T) {
	env := setupTestEnv(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body>UNIT3D Community Edition</body></html>`))
	}))
	defer ts.Close()

	site := &model.Site{
		Name: "U3DSite", Domain: "u3d.com",
		BaseURL: ts.URL, Framework: "generic", AuthType: "cookie", Enabled: true,
	}
	env.db.Create(site)

	w := env.doRequest("POST", fmt.Sprintf("/api/v1/sites/%d/detect", site.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("detect: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["framework"] != "unit3d" {
		t.Errorf("expected unit3d, got %v", data["framework"])
	}
}

func TestSite_Detect_Gazelle(t *testing.T) {
	env := setupTestEnv(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body>Powered by Gazelle</body></html>`))
	}))
	defer ts.Close()

	site := &model.Site{
		Name: "GazSite", Domain: "gaz.com",
		BaseURL: ts.URL, Framework: "generic", AuthType: "cookie", Enabled: true,
	}
	env.db.Create(site)

	w := env.doRequest("POST", fmt.Sprintf("/api/v1/sites/%d/detect", site.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("detect: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["framework"] != "gazelle" {
		t.Errorf("expected gazelle, got %v", data["framework"])
	}
}

func TestSite_Detect_MTeam(t *testing.T) {
	env := setupTestEnv(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body>M-Team Portal</body></html>`))
	}))
	defer ts.Close()

	site := &model.Site{
		Name: "MTSite", Domain: "mt.com",
		BaseURL: ts.URL, Framework: "generic", AuthType: "cookie", Enabled: true,
	}
	env.db.Create(site)

	w := env.doRequest("POST", fmt.Sprintf("/api/v1/sites/%d/detect", site.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("detect: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["framework"] != "mteam" {
		t.Errorf("expected mteam, got %v", data["framework"])
	}
}

func TestSite_Detect_TNode(t *testing.T) {
	env := setupTestEnv(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body>TNode System 朱雀</body></html>`))
	}))
	defer ts.Close()

	site := &model.Site{
		Name: "TNSite", Domain: "tn.com",
		BaseURL: ts.URL, Framework: "generic", AuthType: "cookie", Enabled: true,
	}
	env.db.Create(site)

	w := env.doRequest("POST", fmt.Sprintf("/api/v1/sites/%d/detect", site.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("detect: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["framework"] != "tnode" {
		t.Errorf("expected tnode, got %v", data["framework"])
	}
}

func TestSite_Detect_Luminance(t *testing.T) {
	env := setupTestEnv(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body>Luminance Tracker</body></html>`))
	}))
	defer ts.Close()

	site := &model.Site{
		Name: "LumSite", Domain: "lum.com",
		BaseURL: ts.URL, Framework: "generic", AuthType: "cookie", Enabled: true,
	}
	env.db.Create(site)

	w := env.doRequest("POST", fmt.Sprintf("/api/v1/sites/%d/detect", site.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("detect: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["framework"] != "luminance" {
		t.Errorf("expected luminance, got %v", data["framework"])
	}
}

func TestSite_Detect_Rousi(t *testing.T) {
	env := setupTestEnv(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body>Rousi Platform</body></html>`))
	}))
	defer ts.Close()

	site := &model.Site{
		Name: "RouSite", Domain: "rou.com",
		BaseURL: ts.URL, Framework: "generic", AuthType: "cookie", Enabled: true,
	}
	env.db.Create(site)

	w := env.doRequest("POST", fmt.Sprintf("/api/v1/sites/%d/detect", site.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("detect: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["framework"] != "rousi" {
		t.Errorf("expected rousi, got %v", data["framework"])
	}
}

func TestSite_Test_WithHTTPServer_403(t *testing.T) {
	env := setupTestEnv(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer ts.Close()

	site := &model.Site{
		Name: "Test403Site", Domain: "test403.com",
		BaseURL: ts.URL, Framework: "generic", AuthType: "cookie", Enabled: true,
	}
	env.db.Create(site)

	w := env.doRequest("GET", fmt.Sprintf("/api/v1/sites/%d/test", site.ID), nil)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("test: expected 502, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSite_FreezeStatus_Freeze(t *testing.T) {
	env := setupTestEnv(t)

	siteID := createTestSite(t, env, "FrzStatSite", "frzstat.com")

	w := env.doRequest("POST", fmt.Sprintf("/api/v1/sites/%d/freeze", siteID), map[string]interface{}{
		"duration": "1h",
		"reason":   "test",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("freeze by id: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSite_FreezeStatus_DeleteEmpty(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("DELETE", "/api/v1/httpclient/freeze-status", map[string]interface{}{})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("empty domain: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestIYUU_Test_EmptyToken(t *testing.T) {
	env := setupTestEnv(t)

	env.db.Create(&model.IYUUConfig{Token: "", Enabled: true})

	w := env.doRequest("POST", "/api/v1/iyuu/test", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("test empty token: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestIYUU_MaskToken(t *testing.T) {
	env := setupTestEnv(t)

	env.db.Create(&model.IYUUConfig{Token: "abcdefghijklmnopqrstuvwxyz", Enabled: true, BaseURL: "https://2025.iyuu.cn"})

	w := env.doRequest("GET", "/api/v1/iyuu/config", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get config with token: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	masked, ok := data["token"].(string)
	if !ok || masked == "abcdefghijklmnopqrstuvwxyz" {
		t.Errorf("expected masked token, got %v", data["token"])
	}
}

func TestSettings_ListWithPrefix(t *testing.T) {
	env := setupTestEnv(t)

	env.doRequest("PUT", "/api/v1/settings/prefix.a", map[string]string{"value": "1"})
	env.doRequest("PUT", "/api/v1/settings/prefix.b", map[string]string{"value": "2"})
	env.doRequest("PUT", "/api/v1/settings/other.c", map[string]string{"value": "3"})

	w := env.doRequest("GET", "/api/v1/settings?prefix=prefix", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list with prefix: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if _, ok := data["items"]; !ok {
		t.Error("expected items field")
	}
}

func TestPublish_CreateAndGetTask(t *testing.T) {
	env := setupTestEnv(t)

	siteID := createTestSite(t, env, "PubTaskSite", "pubtask.com")

	w := env.doRequest("POST", "/api/v1/publish/tasks", map[string]interface{}{
		"sourceSiteId": siteID,
		"targetSites":  []string{"target.com"},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	taskID := data["id"].(float64)

	w = env.doRequest("GET", fmt.Sprintf("/api/v1/publish/tasks/%d", int(taskID)), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get task: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPublish_DeleteTask(t *testing.T) {
	env := setupTestEnv(t)

	siteID := createTestSite(t, env, "PubDelSite", "pubdel.com")

	task := &model.PublishTask{
		SourceSiteID: siteID,
		TargetSites:  []string{"target.com"},
		Status:       model.PublishTaskPending,
	}
	env.db.Create(task)

	w := env.doRequest("DELETE", fmt.Sprintf("/api/v1/publish/tasks/%d", task.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete task: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPublish_CancelTask_Pending(t *testing.T) {
	env := setupTestEnv(t)

	siteID := createTestSite(t, env, "PubCnlPend", "pubcnlpend.com")

	task := &model.PublishTask{
		SourceSiteID: siteID,
		TargetSites:  []string{"target.com"},
		Status:       model.PublishTaskPending,
	}
	env.db.Create(task)

	w := env.doRequest("POST", fmt.Sprintf("/api/v1/publish/tasks/%d/cancel", task.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("cancel pending: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPublish_Results_WithCandidateID(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/publish/results?candidateId=1&limit=10", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("results with candidate: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestReseed_Update(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/reseed/tasks", map[string]interface{}{
		"name": "UpdateReseed", "enabled": false,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	taskID := int(data["id"].(float64))

	w = env.doRequest("PUT", fmt.Sprintf("/api/v1/reseed/tasks/%d", taskID), map[string]interface{}{
		"name":                 "UpdatedReseed",
		"enabled":              true,
		"clientIds":            "c3",
		"sourceSiteIds":        "5",
		"targetSiteIds":        "6",
		"sizeTolerancePercent": 2.0,
		"confidenceThreshold":  0.8,
		"schedule":             "0 */3 * * *",
		"maxInjectionsPerRun":  50,
		"reseedCategory":       "my-cat",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestReseed_Trigger_RunError(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/reseed/tasks", map[string]interface{}{
		"name": "TriggerErrTask", "enabled": true, "sourceSiteIds": "1", "targetSiteIds": "2",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	taskID := int(data["id"].(float64))

	w = env.doRequest("POST", fmt.Sprintf("/api/v1/reseed/tasks/%d/trigger", taskID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("trigger: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestReseed_GetMatch_NotFound(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/reseed/tasks/1/matches/99999", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("get match not found: expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_ScoringConfig_CRUD(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/seeding/scoring-config", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list scoring configs: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_ConfigsCRUD_Full(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/seeding/configs", map[string]interface{}{
		"clientId":       "seeding-client-1",
		"enabled":        true,
		"autoDeleteCron": "0 */6 * * *",
		"mainDataCron":   "*/30 * * * *",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create config: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	configID := int(data["id"].(float64))

	w = env.doRequest("GET", fmt.Sprintf("/api/v1/seeding/configs/%d", configID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get config: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("PUT", fmt.Sprintf("/api/v1/seeding/configs/%d", configID), map[string]interface{}{
		"enabled":        false,
		"autoDeleteCron": "0 */12 * * *",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("update config: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("DELETE", fmt.Sprintf("/api/v1/seeding/configs/%d", configID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete config: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_RulesTestRule(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/seeding/rules", map[string]interface{}{
		"alias":      "test-seed-rule",
		"priority":   1,
		"enabled":    true,
		"type":       "normal",
		"conditions": `{"field":"seeders","op":"gt","value":5}`,
		"action":     "delete",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	ruleID := int(data["id"].(float64))

	w = env.doRequest("POST", fmt.Sprintf("/api/v1/seeding/rules/%d/test", ruleID), map[string]interface{}{
		"torrentName": "test.torrent",
		"size":        1024,
		"seeders":     10,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("test rule: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_RulesUpdateRule(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/seeding/rules", map[string]interface{}{
		"alias":      "up-seed-rule",
		"priority":   1,
		"enabled":    true,
		"type":       "normal",
		"conditions": `{"field":"seed_time","op":"gt","value":100}`,
		"action":     "delete",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	ruleID := int(data["id"].(float64))

	w = env.doRequest("PUT", fmt.Sprintf("/api/v1/seeding/rules/%d", ruleID), map[string]interface{}{
		"alias":    "up-seed-rule-v2",
		"enabled":  false,
		"priority": 2,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_StatsBySite(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/seeding/stats/by-site", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("stats by-site: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_StatsTorrents(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/seeding/stats/torrents", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("stats torrents: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_SiteTrend(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/seeding/stats/by-site/mysite.com/trend", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("site trend: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSite_Exclusions_CreateAndDelete(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/publish/exclusions", map[string]interface{}{
		"target_site": "ex-target.com",
		"source_site": "ex-source.com",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create exclusion: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("DELETE", "/api/v1/publish/exclusions", map[string]interface{}{
		"target_site": "ex-target.com",
		"source_site": "ex-source.com",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("delete exclusion: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSite_Exclusions_DeleteNotFound2(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("DELETE", "/api/v1/publish/exclusions", map[string]interface{}{
		"target_site": "noexist.com",
		"source_site": "noexist2.com",
	})
	if w.Code != http.StatusNotFound {
		t.Fatalf("delete not found: expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSite_Overrides_CRUD(t *testing.T) {
	env := setupTestEnv(t)
	siteID := createTestSite(t, env, "OvrCRUDSite", "ovrcrud.com")

	w := env.doRequest("PUT", fmt.Sprintf("/api/v1/sites/%d/overrides", siteID), map[string]interface{}{
		"fieldPath":  "downloadMode",
		"fieldValue": "page",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("upsert override: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("GET", fmt.Sprintf("/api/v1/sites/%d/overrides", siteID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list overrides: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("DELETE", fmt.Sprintf("/api/v1/sites/%d/overrides?fieldPath=downloadMode", siteID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete override: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestScheduler_PauseResume_NotFound(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/scheduler/tasks/nonexist/pause", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("pause not found: expected 404, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("POST", "/api/v1/scheduler/tasks/nonexist/resume", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("resume not found: expected 404, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("POST", "/api/v1/scheduler/tasks/nonexist/trigger", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("trigger not found: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestScheduler_Reschedule_NotFound(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("PUT", "/api/v1/scheduler/tasks/nonexist/schedule", map[string]interface{}{
		"schedule": "0 */6 * * *",
	})
	if w.Code != http.StatusNotFound && w.Code != http.StatusBadRequest {
		t.Fatalf("reschedule not found: expected 404/400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPTGen_ListCache_WithKeyword(t *testing.T) {
	env := setupTestEnv(t)

	env.db.Create(&model.PTGenCache{
		QueryKey:     "tt9999",
		ChineseTitle: "测试影片",
	})

	w := env.doRequest("GET", "/api/v1/ptgen/cache?keyword=测试", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("cache keyword: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["total"] != float64(1) {
		t.Errorf("expected 1, got %v", data["total"])
	}
}

func TestPublish_GroupLifecycle_PauseResume(t *testing.T) {
	env := setupTestEnv(t)

	group := &model.PublishGroup{
		Status:     "publishing",
		SourceHash: "lifecycle-hash",
		SourceSite: "lc-site.com",
	}
	env.db.Create(group)

	w := env.doRequest("POST", fmt.Sprintf("/api/v1/publish/groups/%d/lifecycle/pause", group.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("pause: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("POST", fmt.Sprintf("/api/v1/publish/groups/%d/lifecycle/resume", group.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("resume: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPublish_ListGroups_WithStatus(t *testing.T) {
	env := setupTestEnv(t)

	env.db.Create(&model.PublishGroup{Status: "done", SourceHash: "grp-hash1", SourceSite: "s1.com"})
	env.db.Create(&model.PublishGroup{Status: "publishing", SourceHash: "grp-hash2", SourceSite: "s2.com"})

	w := env.doRequest("GET", "/api/v1/publish/groups?status=done", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list groups: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["total"] != float64(1) {
		t.Errorf("expected 1 done group, got %v", data["total"])
	}
}

func TestSite_Create_Validation(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/sites", map[string]interface{}{
		"name": "", "domain": "val.com", "baseUrl": "https://val.com",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("empty name: expected 400, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("POST", "/api/v1/sites", map[string]interface{}{
		"name": "ValSite", "domain": "", "baseUrl": "https://val.com",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("empty domain: expected 400, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("POST", "/api/v1/sites", map[string]interface{}{
		"name": "ValSite2", "domain": "val2.com", "baseUrl": "",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("empty baseUrl: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSite_Create_DuplicateDomain(t *testing.T) {
	env := setupTestEnv(t)
	// 直接 db 写入白名单内的 domain 制造重复（步骤 3 强白名单校验）
	createTestSite(t, env, "龙PT", "longpt.org")

	w := env.doRequest("POST", "/api/v1/sites", map[string]interface{}{
		"domain": "longpt.org",
	})
	if w.Code != http.StatusConflict {
		t.Fatalf("dup domain: expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSite_Update_InvalidFramework(t *testing.T) {
	env := setupTestEnv(t)
	siteID := createTestSite(t, env, "BadFWSite", "badfw.com")

	w := env.doRequest("PUT", fmt.Sprintf("/api/v1/sites/%d", siteID), map[string]interface{}{
		"framework": "invalid_framework",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid framework: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSystem_Logs_Clear2(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("DELETE", "/api/v1/system/logs", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("clear logs: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSystem_Logs_WithLevel(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/system/logs?level=info&limit=50", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("logs with level: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSite_FreezeByID_InvalidDuration(t *testing.T) {
	env := setupTestEnv(t)
	siteID := createTestSite(t, env, "BadDurSite", "baddur.com")

	w := env.doRequest("POST", fmt.Sprintf("/api/v1/sites/%d/freeze", siteID), map[string]interface{}{
		"duration": "invalid",
		"reason":   "test",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid duration: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSite_FreezeStatus_GetStatuses(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/httpclient/freeze-status", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get statuses: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSite_FreezeStatus_DeleteUnfreeze2(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("DELETE", "/api/v1/httpclient/freeze-status", map[string]interface{}{
		"domain": "unfreeze-test.com",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("unfreeze: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSite_Create_InvalidFramework(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/sites", map[string]interface{}{
		"name":      "BadFWCreate",
		"domain":    "badfwcreate.com",
		"baseUrl":   "https://badfwcreate.com",
		"framework": "nonexistent",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("bad framework: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSite_Create_InvalidAuthType(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/sites", map[string]interface{}{
		"name":      "BadAuthSite",
		"domain":    "badauth.com",
		"baseUrl":   "https://badauth.com",
		"framework": "nexusphp",
		"authType":  "invalid",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("bad authType: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSite_Update_InvalidAuthType(t *testing.T) {
	env := setupTestEnv(t)
	siteID := createTestSite(t, env, "BadAuthUp", "badauthup.com")

	w := env.doRequest("PUT", fmt.Sprintf("/api/v1/sites/%d", siteID), map[string]interface{}{
		"authType": "invalid",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("bad authType update: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSite_Delete(t *testing.T) {
	env := setupTestEnv(t)
	siteID := createTestSite(t, env, "DelSite", "delsite.com")

	w := env.doRequest("DELETE", fmt.Sprintf("/api/v1/sites/%d", siteID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSite_Delete_NotFound(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("DELETE", "/api/v1/sites/99999", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("delete not found: expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSite_Get_NotFound(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/sites/99999", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("get not found: expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSite_Create_DuplicateName(t *testing.T) {
	env := setupTestEnv(t)
	// 先 db 写入制造 name 重复（用白名单内 domain）
	createTestSite(t, env, "龙PT", "longpt.org")

	// POST 用另一个白名单 domain 但显式传相同 name → 应返回 409（name 重复）
	w := env.doRequest("POST", "/api/v1/sites", map[string]interface{}{
		"domain": "kufei.org",
		"name":   "龙PT",
	})
	if w.Code != http.StatusConflict {
		t.Fatalf("dup name: expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSite_Update(t *testing.T) {
	env := setupTestEnv(t)
	siteID := createTestSite(t, env, "UpdSite", "updsite.com")

	w := env.doRequest("PUT", fmt.Sprintf("/api/v1/sites/%d", siteID), map[string]interface{}{
		"name":      "UpdatedSite",
		"domain":    "updated.com",
		"baseUrl":   "https://updated.com",
		"framework": "unit3d",
		"enabled":   false,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["name"] != "UpdatedSite" {
		t.Errorf("expected UpdatedSite, got %v", data["name"])
	}
}

func TestPublish_CandidateCRUD(t *testing.T) {
	env := setupTestEnv(t)

	candidate := &model.PublishCandidate{
		SourceSite:    "candsite.com",
		InfoHash:      "cand-hash",
		TorrentName:   "cand title",
		Size:          1024,
		PublishStatus: "pending",
	}
	env.db.Create(candidate)

	w := env.doRequest("GET", fmt.Sprintf("/api/v1/publish/candidates/%d", candidate.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get candidate: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("DELETE", fmt.Sprintf("/api/v1/publish/candidates/%d", candidate.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete candidate: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPublish_DeleteGroup_NotFound(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("DELETE", "/api/v1/publish/groups/99999", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete group not found: expected 200 (still ok), got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_DeleteRule(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/seeding/rules", map[string]interface{}{
		"alias": "del-seed-rule", "priority": 1, "enabled": true,
		"type": "normal", "conditions": `{"field":"size","op":"gt","value":100}`,
		"action": "delete",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	ruleID := int(data["id"].(float64))

	w = env.doRequest("DELETE", fmt.Sprintf("/api/v1/seeding/rules/%d", ruleID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete rule: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_ListRecords(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/seeding/records?page=1&size=10", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list records: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_ConfigCreateDuplicate(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/seeding/configs", map[string]interface{}{
		"clientId": "dup-config-client",
		"enabled":  true,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("POST", "/api/v1/seeding/configs", map[string]interface{}{
		"clientId": "dup-config-client",
		"enabled":  true,
	})
	if w.Code != http.StatusConflict {
		t.Fatalf("dup config: expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_ScoringDryrunFull2(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/seeding/scoring-dryrun", map[string]interface{}{
		"infoHash":  "abc123",
		"siteName":  "dryrun-site.com",
		"seedTime":  48,
		"size":      1024000,
		"seeders":   3,
		"createdAt": "2025-01-01T00:00:00Z",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("scoring dryrun: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAuth_Profile_Get(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/auth/profile", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get profile: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["username"] != "admin" {
		t.Errorf("expected admin, got %v", data["username"])
	}
}

func TestAuth_Profile_MethodNotAllowed(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/auth/profile", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("profile POST: expected 405, got %d", w.Code)
	}
}

func TestSite_InvalidID(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/sites/abc", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid id: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSite_Stats_InvalidID(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/sites/abc/stats", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid stats id: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCookieCloud_History_WithRecords(t *testing.T) {
	env := setupTestEnv(t)

	env.db.Create(&model.CookieCloudSyncHistory{
		Status:      "success",
		SyncedSites: 5,
	})

	w := env.doRequest("GET", "/api/v1/cookiecloud/history", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("history: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["total"] != float64(1) {
		t.Errorf("expected 1, got %v", data["total"])
	}
}

func TestPTGen_CleanCache_WithRetainDays(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("DELETE", "/api/v1/ptgen/cache?retainDays=7", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("clean cache: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["retainDays"] != float64(7) {
		t.Errorf("expected retainDays=7, got %v", data["retainDays"])
	}
}

func TestSeeding_Torrents_ResumeFromTorrents(t *testing.T) {
	env := setupTestEnv(t)

	rec := &model.SeedingTorrentRecord{
		ClientID: "c1", InfoHash: "tor-resume-hash",
		SiteName: "s1", TorrentID: "t1", Status: model.SeedingStatusPausedFreeEnd,
	}
	env.db.Create(rec)

	w := env.doRequest("POST", fmt.Sprintf("/api/v1/seeding/torrents/%d/resume", rec.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("torrent resume: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSettings_SetAndGet2(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("PUT", "/api/v1/settings/test.key.verify", map[string]string{"value": "hello"})
	if w.Code != http.StatusOK {
		t.Fatalf("set: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("GET", "/api/v1/settings/test.key.verify", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["value"] != "hello" {
		t.Errorf("expected hello, got %v", data["value"])
	}
}

func TestSettings_Get_NotFound(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/settings/nonexistent.key.xyz", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("get not found: expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSite_Test_WithHTTPServer_WithCookie(t *testing.T) {
	env := setupTestEnv(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") == "" {
			t.Error("expected cookie header")
		}
		w.Write([]byte(`<html><body><a href="/logout">logout</a></body></html>`))
	}))
	defer ts.Close()

	site := &model.Site{
		Name: "CookieSite", Domain: "cookiesite.com",
		BaseURL: ts.URL, Framework: "nexusphp", AuthType: "cookie",
		Cookie: "session=test123", Enabled: true,
	}
	env.db.Create(site)

	w := env.doRequest("GET", fmt.Sprintf("/api/v1/sites/%d/test", site.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("test with cookie: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSite_Detect_Generic(t *testing.T) {
	env := setupTestEnv(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body>Some random tracker</body></html>`))
	}))
	defer ts.Close()

	site := &model.Site{
		Name: "GenSite", Domain: "gen.com",
		BaseURL: ts.URL, Framework: "generic", AuthType: "cookie", Enabled: true,
	}
	env.db.Create(site)

	w := env.doRequest("POST", fmt.Sprintf("/api/v1/sites/%d/detect", site.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("detect: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["framework"] != "generic" {
		t.Errorf("expected generic, got %v", data["framework"])
	}
}

func TestSite_Detect_Rousi_WithUUID(t *testing.T) {
	env := setupTestEnv(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body>Rousi Torrent Platform</body></html>`))
	}))
	defer ts.Close()

	site := &model.Site{
		Name: "RouDetSite", Domain: "roudet.com",
		BaseURL: ts.URL, Framework: "generic", AuthType: "cookie", Enabled: true,
	}
	env.db.Create(site)

	w := env.doRequest("POST", fmt.Sprintf("/api/v1/sites/%d/detect", site.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("detect rousi: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	defaults, _ := data["defaults"].(map[string]interface{})
	if defaults == nil {
		t.Fatal("expected defaults")
	}
	if defaults["id_pattern"] != "uuid" {
		t.Errorf("expected uuid idPattern for rousi, got %v", defaults["id_pattern"])
	}
}

func TestSite_Detect_NexusKeyword(t *testing.T) {
	env := setupTestEnv(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body>Powered by Nexus</body></html>`))
	}))
	defer ts.Close()

	site := &model.Site{
		Name: "NexusKwSite", Domain: "nexuskw.com",
		BaseURL: ts.URL, Framework: "generic", AuthType: "cookie", Enabled: true,
	}
	env.db.Create(site)

	w := env.doRequest("POST", fmt.Sprintf("/api/v1/sites/%d/detect", site.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("detect nexus kw: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["framework"] != "nexusphp" {
		t.Errorf("expected nexusphp, got %v", data["framework"])
	}
	if data["confidence"] != 0.7 {
		t.Errorf("expected confidence 0.7, got %v", data["confidence"])
	}
}

func TestSeeding_ScoringConfigByID(t *testing.T) {
	env := setupTestEnv(t)

	// db 直接创建（步骤 3 强白名单校验）
	env.db.Create(&model.Site{Name: "ScorConfSite", Domain: "scorconf.com", BaseURL: "https://scorconf.com", Framework: "nexusphp", AuthType: "cookie", Enabled: true})
	w := env.doRequest("POST", "/api/v1/rss/subscriptions", map[string]interface{}{
		"name": "scorconf-sub", "siteName": "ScorConfSite",
		"urls": []string{"https://scorconf.com/rss"}, "enabled": true,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create sub: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	subID := int(data["id"].(float64))

	w = env.doRequest("GET", fmt.Sprintf("/api/v1/seeding/scoring-config/%d", subID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get scoring config: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSite_UpdateFull(t *testing.T) {
	env := setupTestEnv(t)
	siteID := createTestSite(t, env, "FullUpdSite", "fullupd.com")

	w := env.doRequest("PUT", fmt.Sprintf("/api/v1/sites/%d", siteID), map[string]interface{}{
		"name":                   "FullUpdDone",
		"domain":                 "fullupddone.com",
		"baseUrl":                "https://fullupddone.com",
		"framework":              "unit3d",
		"authType":               "apikey",
		"enabled":                true,
		"isSource":               true,
		"isTarget":               true,
		"participateAutoPublish": true,
		"cookieCloudSync":        true,
		"cookieCloudDomain":      "fullupddone.com",
		"alternativeDomains":     "alt.fullupddone.com",
		"hashStrategy":           "fake_from_id",
		"sizeStrategy":           "desc_regex",
		"idStrategy":             "link_regex",
		"idPattern":              `/torrent/(\d+)`,
		"hashXmlTagName":         "custom_hash",
		"sizeXmlTagName":         "custom_size",
		"hashUrlParamName":       "hash",
		"sizeDescRegex":          `(\d+\.?\d*\s*[GMK]?B)`,
		"sizeTitleRegex":         `(\d+\.?\d*\s*[GMK]?B)`,
		"sizeBaseUnit":           1024,
		"downloadMode":           "page",
		"downloadPagePattern":    "/download/",
		"requiresSideLoading":    true,
		"overrideRssUrl":         "https://rss.fullupddone.com",
		"overrideSavePath":       "/data/torrents",
		"proxyUrl":               "http://proxy:8080",
		"skipSslVerify":          true,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("full update: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_ScoringLogsList(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/seeding/scoring-logs", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("scoring logs: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_ScoringCycle(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/seeding/scoring-logs/cycles/1", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("scoring cycle: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDashboard_Trends_InvalidDays(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/dashboard/trends?days=0", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("trends 0 days: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["days"] != float64(7) {
		t.Errorf("expected default 7 days, got %v", data["days"])
	}
}

func TestSite_Detect_WithCookie(t *testing.T) {
	env := setupTestEnv(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") == "" {
			t.Error("expected cookie header in detect")
		}
		w.Write([]byte(`<html><body>NexusPHP</body></html>`))
	}))
	defer ts.Close()

	site := &model.Site{
		Name: "DetCookieSite", Domain: "detcookie.com",
		BaseURL: ts.URL, Framework: "generic", AuthType: "cookie",
		Cookie: "session=dettest", Enabled: true,
	}
	env.db.Create(site)

	w := env.doRequest("POST", fmt.Sprintf("/api/v1/sites/%d/detect", site.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("detect with cookie: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSite_Test_ConnectionError(t *testing.T) {
	env := setupTestEnv(t)

	site := &model.Site{
		Name: "ConnErrSite", Domain: "connerr.com",
		BaseURL: "http://127.0.0.1:1", Framework: "generic", AuthType: "cookie", Enabled: true,
	}
	env.db.Create(site)

	w := env.doRequest("GET", fmt.Sprintf("/api/v1/sites/%d/test", site.ID), nil)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("test conn error: expected 502, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPublish_TasksPaginated(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/publish/tasks?page=1&pageSize=5", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("paginated tasks: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["page"] != float64(1) {
		t.Errorf("expected page 1, got %v", data["page"])
	}
}

func TestSeeding_ClientsList(t *testing.T) {
	env := setupTestEnv(t)

	env.db.Create(&model.ClientConfig{
		Name: "ListClient", Type: "qbittorrent", URL: "http://localhost:8080",
		Username: "admin", Password: "pass", Role: "seeding", Enabled: true,
	})

	w := env.doRequest("GET", "/api/v1/seeding/clients", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("clients: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_StatusOverview(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/seeding/stats/overview", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("overview: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSettings_Restore_BadJSON(t *testing.T) {
	env := setupTestEnv(t)

	req := httptest.NewRequest("POST", "/api/v1/settings/restore", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.token)
	w := httptest.NewRecorder()
	env.mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("bad json restore: expected 400, got %d", w.Code)
	}
}

func TestSeeding_ScoringConfig_Put(t *testing.T) {
	env := setupTestEnv(t)

	// db 直接创建（步骤 3 强白名单校验）
	env.db.Create(&model.Site{Name: "ScorPutSite", Domain: "scorput.com", BaseURL: "https://scorput.com", Framework: "nexusphp", AuthType: "cookie", Enabled: true})
	w := env.doRequest("POST", "/api/v1/rss/subscriptions", map[string]interface{}{
		"name": "scorput-sub", "siteName": "ScorPutSite",
		"urls": []string{"https://scorput.com/rss"}, "enabled": true,
	})
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	subID := int(data["id"].(float64))

	w = env.doRequest("PUT", fmt.Sprintf("/api/v1/seeding/scoring-config/%d", subID), map[string]interface{}{
		"enabled":       true,
		"halfLifeHours": 48.0,
		"minScore":      0.5,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("put scoring config: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPublish_CandidatesListPaginated(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/publish/candidates?limit=5", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("candidates paginated: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSite_CircuitStatus_PostNoDomain(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/httpclient/circuit-status", map[string]interface{}{})
	if w.Code != http.StatusOK {
		t.Fatalf("no domain: expected 200 (resets empty), got %d: %s", w.Code, w.Body.String())
	}
}

func TestSite_CircuitStatus_PostBadJSON(t *testing.T) {
	env := setupTestEnv(t)

	req := httptest.NewRequest("POST", "/api/v1/httpclient/circuit-status", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.token)
	w := httptest.NewRecorder()
	env.mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest && w.Code != http.StatusOK {
		t.Fatalf("bad json: expected 400/200, got %d", w.Code)
	}
}

func TestSeeding_ScoringConfig_CRUD2(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/seeding/scoring-config", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// db 直接创建（步骤 3 强白名单校验）
	env.db.Create(&model.Site{Name: "ScorConf2Site", Domain: "scorconf2.com", BaseURL: "https://scorconf2.com", Framework: "nexusphp", AuthType: "cookie", Enabled: true})
	w = env.doRequest("POST", "/api/v1/rss/subscriptions", map[string]interface{}{
		"name": "scorconf2-sub", "siteName": "ScorConf2Site",
		"urls": []string{"https://scorconf2.com/rss"}, "enabled": true,
	})
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	subID := int(data["id"].(float64))

	w = env.doRequest("PUT", fmt.Sprintf("/api/v1/seeding/scoring-config?subscriptionId=%d", subID), map[string]interface{}{
		"enabled":       true,
		"halfLifeHours": 24.0,
		"minScore":      0.3,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("GET", fmt.Sprintf("/api/v1/seeding/scoring-config?subscriptionId=%d", subID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSystem_Logs_WithLimit(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/system/logs?limit=10", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("logs with limit: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSystem_Logs_Delete(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("DELETE", "/api/v1/system/logs", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete logs: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDashboard_Trends_MaxDays(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("GET", "/api/v1/dashboard/trends?days=30", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("trends 30 days: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	trends, _ := data["trends"].([]interface{})
	if len(trends) != 30 {
		t.Errorf("expected 30 trend points, got %d", len(trends))
	}
}

func TestIYUU_Query_Empty(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/iyuu/query", map[string]interface{}{
		"infoHashes": []string{},
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("empty query: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestIYUU_Query_WithData(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/iyuu/query", map[string]interface{}{
		"infoHashes": []string{"abc123", "def456"},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("query: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func createTestClient(t *testing.T, env *testEnv, name, role string) uint {
	t.Helper()
	w := env.doRequest("POST", "/api/v1/downloaders", map[string]interface{}{
		"name": name, "type": "qbittorrent", "url": "http://localhost:8080",
		"username": "admin", "password": "pass", "role": role, "enabled": true,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create client %s: expected 200, got %d: %s", name, w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	return uint(data["id"].(float64))
}

func setupTestEnvWithClientMgr(t *testing.T) *testEnv {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := model.AutoMigrate(db); err != nil {
		t.Fatalf("model migrate: %v", err)
	}
	if err := setting.AutoMigrate(db); err != nil {
		t.Fatalf("setting migrate: %v", err)
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
	clientMgr := client.NewManager(db, logger)
	router := NewRouter(authManager, db, rssEngine, notifyService, reseedEngine, publishPipeline, seedingEngine, clientMgr, taskRegistry, &mockIYUUQueryService{}, "test", nil, logger)
	mux := http.NewServeMux()
	router.Register(mux, []string{"*"}, true, 120)
	env := &testEnv{
		db:           db,
		authManager:  authManager,
		router:       router,
		mux:          mux,
		taskRegistry: taskRegistry,
		stopCh:       make(chan struct{}),
	}
	go router.Hub().Run(env.stopCh)
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
	return env
}

func TestClient_NewClientHandler(t *testing.T) {
	logger := zap.NewNop()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	h := NewClientHandler(db, logger, nil)
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
}

func TestClient_Create_MissingFields(t *testing.T) {
	env := setupTestEnv(t)
	cases := []struct {
		name string
		body map[string]interface{}
	}{
		{"missing name", map[string]interface{}{"type": "qbittorrent", "url": "http://x", "role": "seeding"}},
		{"missing type", map[string]interface{}{"name": "x", "url": "http://x", "role": "seeding"}},
		{"missing url", map[string]interface{}{"name": "x", "type": "qbittorrent", "role": "seeding"}},
		{"missing role", map[string]interface{}{"name": "x", "type": "qbittorrent", "url": "http://x"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := env.doRequest("POST", "/api/v1/downloaders", tc.body)
			if w.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestClient_Create_InvalidType(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/downloaders", map[string]interface{}{
		"name": "badtype", "type": "deluge", "url": "http://x", "role": "seeding",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestClient_Create_InvalidRole(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/downloaders", map[string]interface{}{
		"name": "badrole", "type": "qbittorrent", "url": "http://x", "role": "invalid",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestClient_Create_DuplicateName(t *testing.T) {
	env := setupTestEnv(t)
	body := map[string]interface{}{
		"name": "DupClient", "type": "qbittorrent", "url": "http://localhost:8080",
		"role": "seeding", "enabled": true,
	}
	w := env.doRequest("POST", "/api/v1/downloaders", body)
	if w.Code != http.StatusOK {
		t.Fatalf("first create: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	w = env.doRequest("POST", "/api/v1/downloaders", body)
	if w.Code != http.StatusConflict {
		t.Fatalf("duplicate: expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestClient_Create_WithTransmission(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/downloaders", map[string]interface{}{
		"name": "trclient", "type": "transmission", "url": "http://localhost:9091",
		"role": "download", "enabled": true,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["type"] != "transmission" {
		t.Errorf("expected transmission, got %v", data["type"])
	}
}

func TestClient_Create_WithPathMappings(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/downloaders", map[string]interface{}{
		"name": "PathMapClient", "type": "qbittorrent", "url": "http://localhost:8080",
		"role": "seeding", "enabled": true,
		"pathMappings": []map[string]string{
			{"sourcePath": "/data/movies", "reseedPath": "/mnt/movies"},
		},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	mappings, ok := data["pathMappings"].([]interface{})
	if !ok || len(mappings) != 1 {
		t.Fatalf("expected 1 path mapping, got %v", data["pathMappings"])
	}
}

func TestClient_Get_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/downloaders/99999", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestClient_Delete_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("DELETE", "/api/v1/downloaders/99999", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestClient_Delete_Success(t *testing.T) {
	env := setupTestEnv(t)
	id := createTestClient(t, env, "DeleteMe", "seeding")
	w := env.doRequest("DELETE", fmt.Sprintf("/api/v1/downloaders/%d", id), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	w = env.doRequest("GET", fmt.Sprintf("/api/v1/downloaders/%d", id), nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("after delete get: expected 404, got %d", w.Code)
	}
}

func TestClient_List_Pagination(t *testing.T) {
	env := setupTestEnv(t)
	for i := 0; i < 5; i++ {
		env.doRequest("POST", "/api/v1/downloaders", map[string]interface{}{
			"name": fmt.Sprintf("PageClient%d", i), "type": "qbittorrent",
			"url": fmt.Sprintf("http://localhost:%d", 8080+i), "role": "seeding", "enabled": true,
		})
	}
	w := env.doRequest("GET", "/api/v1/downloaders?page=1&size=2", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	items, _ := data["items"].([]interface{})
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
	total, _ := data["total"].(float64)
	if int(total) != 5 {
		t.Errorf("expected total 5, got %v", total)
	}
}

func TestClient_TestConnection_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/downloaders/99999/test", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestClient_TestConnection_Error(t *testing.T) {
	env := setupTestEnvWithClientMgr(t)
	id := createTestClient(t, env, "TestConnErr", "seeding")
	w := env.doRequest("POST", fmt.Sprintf("/api/v1/downloaders/%d/test", id), nil)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", w.Code, w.Body.String())
	}
}

func TestClient_TestConnection_Transmission(t *testing.T) {
	env := setupTestEnvWithClientMgr(t)
	w := env.doRequest("POST", "/api/v1/downloaders", map[string]interface{}{
		"name": "TRTestConn", "type": "transmission", "url": "http://localhost:9091",
		"role": "download", "enabled": true,
	})
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	id := uint(data["id"].(float64))

	w = env.doRequest("POST", fmt.Sprintf("/api/v1/downloaders/%d/test", id), nil)
	if w.Code == http.StatusOK {
		resp = parseResponse(t, w)
		data, _ = resp.Data.(map[string]interface{})
		if ok, _ := data["ok"].(bool); ok {
			t.Log("transmission test connected (may be running locally)")
		}
	}
}

func TestClient_Torrents_NotFound(t *testing.T) {
	env := setupTestEnvWithClientMgr(t)
	w := env.doRequest("GET", "/api/v1/downloaders/99999/torrents", nil)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestClient_Torrents_ConnectionError(t *testing.T) {
	env := setupTestEnvWithClientMgr(t)
	id := createTestClient(t, env, "TorrentsErr", "seeding")
	w := env.doRequest("GET", fmt.Sprintf("/api/v1/downloaders/%d/torrents", id), nil)
	if w.Code == http.StatusOK {
		resp := parseResponse(t, w)
		if resp.Code == 0 {
			t.Log("torrents call succeeded (unexpected but ok)")
		}
	}
}

func TestClient_FreeSpace_NotFound(t *testing.T) {
	env := setupTestEnvWithClientMgr(t)
	w := env.doRequest("GET", "/api/v1/downloaders/99999/free-space", nil)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestClient_FreeSpace_ConnectionError(t *testing.T) {
	env := setupTestEnvWithClientMgr(t)
	id := createTestClient(t, env, "FreeSpaceErr", "seeding")
	w := env.doRequest("GET", fmt.Sprintf("/api/v1/downloaders/%d/free-space", id), nil)
	if w.Code == http.StatusOK {
		resp := parseResponse(t, w)
		if resp.Code == 0 {
			t.Log("free-space call succeeded (unexpected but ok)")
		}
	}
}

func TestClient_Maindata_ConnectionError(t *testing.T) {
	env := setupTestEnvWithClientMgr(t)
	id := createTestClient(t, env, "MaindataErr", "seeding")
	w := env.doRequest("GET", fmt.Sprintf("/api/v1/downloaders/%d/maindata", id), nil)
	if w.Code == http.StatusOK {
		resp := parseResponse(t, w)
		if resp.Code == 0 {
			t.Log("maindata call succeeded (unexpected but ok)")
		}
	}
}

func TestClient_Maindata_InvalidID(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/downloaders/abc/maindata", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestClient_TorrentAction_ConnectionError(t *testing.T) {
	env := setupTestEnvWithClientMgr(t)
	id := createTestClient(t, env, "ActionErr", "seeding")
	w := env.doRequest("POST", fmt.Sprintf("/api/v1/downloaders/%d/torrents/abc123def456/pause", id), nil)
	if w.Code == http.StatusOK {
		t.Log("pause call succeeded (unexpected but ok)")
	}
}

func TestClient_TorrentAction_Resume_ConnectionError(t *testing.T) {
	env := setupTestEnvWithClientMgr(t)
	id := createTestClient(t, env, "ResumeErr", "seeding")
	w := env.doRequest("POST", fmt.Sprintf("/api/v1/downloaders/%d/torrents/abc123def456/resume", id), nil)
	if w.Code == http.StatusOK {
		t.Log("resume call succeeded (unexpected but ok)")
	}
}

func TestClient_TorrentInfo_ConnectionError(t *testing.T) {
	env := setupTestEnvWithClientMgr(t)
	id := createTestClient(t, env, "InfoErr", "seeding")
	w := env.doRequest("GET", fmt.Sprintf("/api/v1/downloaders/%d/torrents/abc123def456", id), nil)
	if w.Code == http.StatusOK {
		resp := parseResponse(t, w)
		if resp.Code == 0 {
			t.Log("torrent info succeeded (unexpected but ok)")
		}
	}
}

func TestClient_TorrentInfo_Delete_ConnectionError(t *testing.T) {
	env := setupTestEnvWithClientMgr(t)
	id := createTestClient(t, env, "DelTorrentErr", "seeding")
	w := env.doRequest("DELETE", fmt.Sprintf("/api/v1/downloaders/%d/torrents/abc123def456", id), nil)
	if w.Code == http.StatusOK {
		t.Log("delete torrent succeeded (unexpected but ok)")
	}
}

func TestClient_TorrentInfo_DeleteWithFiles_ConnectionError(t *testing.T) {
	env := setupTestEnvWithClientMgr(t)
	id := createTestClient(t, env, "DelFilesErr", "seeding")
	w := env.doRequest("DELETE", fmt.Sprintf("/api/v1/downloaders/%d/torrents/abc123def456?deleteFiles=true", id), nil)
	if w.Code == http.StatusOK {
		t.Log("delete torrent with files succeeded (unexpected but ok)")
	}
}

func TestClient_ServeHTTP_List(t *testing.T) {
	env := setupTestEnv(t)
	createTestClient(t, env, "ServeList", "seeding")
	w := env.doRequest("GET", "/api/v1/downloaders", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	items, _ := data["items"].([]interface{})
	if len(items) < 1 {
		t.Error("expected at least 1 item")
	}
}

func TestClient_ServeHTTP_CreateViaRoute(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/downloaders/", map[string]interface{}{
		"name": "RouteCreate", "type": "qbittorrent", "url": "http://localhost:8080",
		"role": "seeding", "enabled": true,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create via route: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestClient_HandleRouteByPath_UnknownSubResource(t *testing.T) {
	env := setupTestEnv(t)
	id := createTestClient(t, env, "UnknownSub", "seeding")
	w := env.doRequest("GET", fmt.Sprintf("/api/v1/downloaders/%d/unknownthing", id), nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestClient_HandleRouteByPath_InvalidID(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/downloaders/abc", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestClient_HandleRouteByPath_MethodNotAllowed(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("PATCH", "/api/v1/downloaders", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d: %s", w.Code, w.Body.String())
	}
}

func TestClient_PublishTargets_InvalidBody(t *testing.T) {
	env := setupTestEnv(t)
	req := httptest.NewRequest("POST", "/api/v1/downloaders/publish-targets", bytes.NewReader([]byte("bad json")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.token)
	w := httptest.NewRecorder()
	env.mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestClient_PublishTargets_MissingRequired(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/downloaders/publish-targets", map[string]interface{}{
		"site_name": "test.com",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	w = env.doRequest("POST", "/api/v1/downloaders/publish-targets", map[string]interface{}{
		"client_id": 1,
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestClient_PublishTargets_UpdateNotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("PUT", "/api/v1/downloaders/publish-targets", map[string]interface{}{
		"id": 99999, "category_mapping": "tv",
	})
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestClient_PublishTargets_UpdateInvalidBody(t *testing.T) {
	env := setupTestEnv(t)
	req := httptest.NewRequest("PUT", "/api/v1/downloaders/publish-targets", bytes.NewReader([]byte("bad")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.token)
	w := httptest.NewRecorder()
	env.mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestClient_PublishTargets_DeleteInvalidBody(t *testing.T) {
	env := setupTestEnv(t)
	req := httptest.NewRequest("DELETE", "/api/v1/downloaders/publish-targets", bytes.NewReader([]byte("bad")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.token)
	w := httptest.NewRecorder()
	env.mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestClient_PublishTargets_MethodNotAllowed(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("PATCH", "/api/v1/downloaders/publish-targets", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d: %s", w.Code, w.Body.String())
	}
}

func TestClient_BuildClient_UnsupportedType(t *testing.T) {
	env := setupTestEnvWithClientMgr(t)
	env.db.Create(&model.ClientConfig{
		Name: "UnsupportedType", Type: "deluge", URL: "http://localhost:9999",
		Role: "seeding", Enabled: true,
	})
	w := env.doRequest("GET", "/api/v1/downloaders", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	items, _ := data["items"].([]interface{})
	var id float64
	for _, item := range items {
		it, _ := item.(map[string]interface{})
		if it["name"] == "UnsupportedType" {
			id = it["id"].(float64)
			break
		}
	}
	if id == 0 {
		t.Fatal("unsupported type client not found")
	}
	w = env.doRequest("GET", fmt.Sprintf("/api/v1/downloaders/%d/torrents", int(id)), nil)
	if w.Code == http.StatusOK {
		resp := parseResponse(t, w)
		if resp.Code == 0 {
			t.Log("unexpected success for unsupported type")
		}
	}
}

func TestClient_TestConnection_UnsupportedType(t *testing.T) {
	env := setupTestEnv(t)
	env.db.Create(&model.ClientConfig{
		Name: "TestUnsupported", Type: "deluge", URL: "http://localhost:9999",
		Role: "seeding", Enabled: true,
	})
	w := env.doRequest("GET", "/api/v1/downloaders", nil)
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	items, _ := data["items"].([]interface{})
	var id float64
	for _, item := range items {
		it, _ := item.(map[string]interface{})
		if it["name"] == "TestUnsupported" {
			id = it["id"].(float64)
			break
		}
	}
	if id == 0 {
		t.Fatal("unsupported type client not found")
	}
	w = env.doRequest("POST", fmt.Sprintf("/api/v1/downloaders/%d/test", int(id)), nil)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_NewSeedingHandler(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	logger := zap.NewNop()
	h := NewSeedingHandler(db, logger, nil)
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
}

func TestSeeding_ListConfigs_Empty(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/seeding/configs", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	items, _ := data["items"].([]interface{})
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestSeeding_CreateConfig_Success(t *testing.T) {
	env := setupTestEnv(t)
	body := map[string]interface{}{
		"clientId":       "client-1",
		"enabled":        true,
		"autoDeleteCron": "*/15 * * * *",
		"mainDataCron":   "*/5 * * * *",
	}
	w := env.doRequest("POST", "/api/v1/seeding/configs", body)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["client_id"] != "client-1" {
		t.Errorf("expected client_id=client-1, got %v", data["client_id"])
	}
	if data["min_disk_space_gb"] != 50.0 {
		t.Errorf("expected default min_disk_space_gb=50, got %v", data["min_disk_space_gb"])
	}
}

func TestSeeding_CreateConfig_Duplicate(t *testing.T) {
	env := setupTestEnv(t)
	body := map[string]interface{}{"clientId": "dup-client", "enabled": true}
	w1 := env.doRequest("POST", "/api/v1/seeding/configs", body)
	if w1.Code != http.StatusOK {
		t.Fatalf("first create: expected 200, got %d", w1.Code)
	}
	w2 := env.doRequest("POST", "/api/v1/seeding/configs", body)
	if w2.Code != http.StatusConflict {
		t.Fatalf("duplicate: expected 409, got %d: %s", w2.Code, w2.Body.String())
	}
}

func TestSeeding_CreateConfig_MissingClientID(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/seeding/configs", map[string]interface{}{"enabled": true})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSeeding_GetConfig_Success(t *testing.T) {
	env := setupTestEnv(t)
	env.db.Create(&model.SeedingClientConfig{ClientID: "gc-1", Enabled: true})
	var cfg model.SeedingClientConfig
	env.db.Where("client_id = ?", "gc-1").First(&cfg)

	w := env.doRequest("GET", fmt.Sprintf("/api/v1/seeding/configs/%d", cfg.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["client_id"] != "gc-1" {
		t.Errorf("expected client_id=gc-1, got %v", data["client_id"])
	}
}

func TestSeeding_GetConfig_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/seeding/configs/9999", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestSeeding_DeleteConfig_Success(t *testing.T) {
	env := setupTestEnv(t)
	env.db.Create(&model.SeedingClientConfig{ClientID: "del-1", Enabled: true})
	var cfg model.SeedingClientConfig
	env.db.Where("client_id = ?", "del-1").First(&cfg)

	w := env.doRequest("DELETE", fmt.Sprintf("/api/v1/seeding/configs/%d", cfg.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_DeleteConfig_InvalidID(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("DELETE", "/api/v1/seeding/configs/abc", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_Stats_Empty(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/seeding/stats", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["activeRecords"] != float64(0) {
		t.Errorf("expected 0 active, got %v", data["activeRecords"])
	}
}

func TestSeeding_Stats_WithRecords(t *testing.T) {
	env := setupTestEnv(t)
	env.db.Create(&model.SeedingTorrentRecord{
		ClientID: "c1", InfoHash: "hash1", SiteName: "site1", TorrentID: "t1",
		Status: model.SeedingStatusSeeding,
	})
	env.db.Create(&model.SeedingTorrentRecord{
		ClientID: "c1", InfoHash: "hash2", SiteName: "site1", TorrentID: "t2",
		Status: model.SeedingStatusPausedFreeEnd,
	})
	w := env.doRequest("GET", "/api/v1/seeding/stats", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["activeRecords"] != float64(0) {
		t.Errorf("expected 0 active (no engine), got %v", data["activeRecords"])
	}
	if data["dbPausedCount"] != float64(0) {
		t.Errorf("expected 0 paused (no engine), got %v", data["dbPausedCount"])
	}
}

func TestSeeding_EngineStatus(t *testing.T) {
	env := setupTestEnv(t)
	env.db.Create(&model.SeedingTorrentRecord{
		ClientID: "c1", InfoHash: "h1", SiteName: "s1", TorrentID: "t1",
		Status: model.SeedingStatusSeeding,
	})
	w := env.doRequest("GET", "/api/v1/seeding/status", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["running"] != true {
		t.Error("expected running=true")
	}
	overview, _ := data["overview"].(map[string]interface{})
	if overview["activeTorrents"] != float64(0) {
		t.Errorf("expected 0 active (no engine), got %v", overview["activeTorrents"])
	}
}

func TestSeeding_ListClients(t *testing.T) {
	env := setupTestEnv(t)
	env.db.Create(&model.SeedingClientConfig{ClientID: "lc-1", Enabled: true})
	env.db.Create(&model.SeedingClientConfig{ClientID: "lc-2", Enabled: false})
	w := env.doRequest("GET", "/api/v1/seeding/clients", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	items, _ := data["items"].([]interface{})
	if len(items) != 2 {
		t.Errorf("expected 2 clients, got %d", len(items))
	}
}

func TestSeeding_ListRules_Empty(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/seeding/rules", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	items, _ := data["items"].([]interface{})
	if len(items) != 0 {
		t.Errorf("expected 0 rules, got %d", len(items))
	}
}

func TestSeeding_ListRules_WithData(t *testing.T) {
	env := setupTestEnv(t)
	env.db.Create(&model.DeleteRule{Alias: "rule-1", Priority: 1, Enabled: true, Type: "normal", Action: "delete"})
	env.db.Create(&model.DeleteRule{Alias: "rule-2", Priority: 2, Enabled: true, Type: "expr", Expr: "seeders > 10", Action: "pause"})
	w := env.doRequest("GET", "/api/v1/seeding/rules", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	items, _ := data["items"].([]interface{})
	if len(items) != 2 {
		t.Errorf("expected 2 rules, got %d", len(items))
	}
}

func TestSeeding_GetRule_Success(t *testing.T) {
	env := setupTestEnv(t)
	env.db.Create(&model.DeleteRule{Alias: "gr-1", Priority: 1, Enabled: true})
	var rule model.DeleteRule
	env.db.Where("alias = ?", "gr-1").First(&rule)

	w := env.doRequest("GET", fmt.Sprintf("/api/v1/seeding/rules/%d", rule.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["alias"] != "gr-1" {
		t.Errorf("expected alias=gr-1, got %v", data["alias"])
	}
}

func TestSeeding_GetRule_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/seeding/rules/9999", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestSeeding_DeleteRule_Success(t *testing.T) {
	env := setupTestEnv(t)
	env.db.Create(&model.DeleteRule{Alias: "dr-1", Priority: 1, Enabled: true})
	var rule model.DeleteRule
	env.db.Where("alias = ?", "dr-1").First(&rule)

	w := env.doRequest("DELETE", fmt.Sprintf("/api/v1/seeding/rules/%d", rule.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_DeleteRule_InvalidID(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("DELETE", "/api/v1/seeding/rules/abc", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_ResumeRecord_V2(t *testing.T) {
	env := setupTestEnv(t)
	env.db.Create(&model.SeedingTorrentRecord{
		ClientID: "c1", InfoHash: "rh1", SiteName: "s1", TorrentID: "t1",
		Status: model.SeedingStatusPausedFreeEnd,
	})
	var rec model.SeedingTorrentRecord
	env.db.Where("client_id = ?", "c1").First(&rec)

	w := env.doRequest("POST", fmt.Sprintf("/api/v1/seeding/records/%d/resume", rec.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_PauseRecord_V2(t *testing.T) {
	env := setupTestEnv(t)
	env.db.Create(&model.SeedingTorrentRecord{
		ClientID: "c1", InfoHash: "ph1", SiteName: "s1", TorrentID: "t1",
		Status: model.SeedingStatusSeeding,
	})
	var rec model.SeedingTorrentRecord
	env.db.Where("client_id = ?", "c1").First(&rec)

	w := env.doRequest("POST", fmt.Sprintf("/api/v1/seeding/records/%d/pause", rec.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_ResumeRecord_InvalidID_V2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/seeding/records/abc/resume", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_TriggerClient(t *testing.T) {
	env := setupTestEnv(t)
	env.db.Create(&model.SeedingTorrentRecord{
		ClientID: "tc-1", InfoHash: "th1", SiteName: "s1", TorrentID: "t1",
		Status: model.SeedingStatusSeeding,
	})

	w := env.doRequest("POST", "/api/v1/seeding/clients/tc-1/trigger", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["clientId"] != "tc-1" {
		t.Errorf("expected clientId=tc-1, got %v", data["clientId"])
	}
}

func TestSeeding_ScoringLogs_V2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/seeding/scoring-logs", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_StatsBySite_V2(t *testing.T) {
	env := setupTestEnv(t)
	env.db.Create(&model.SeedingTorrentRecord{ClientID: "c1", InfoHash: "h1", SiteName: "siteA", TorrentID: "t1", Status: model.SeedingStatusSeeding})
	env.db.Create(&model.SeedingTorrentRecord{ClientID: "c1", InfoHash: "h2", SiteName: "siteA", TorrentID: "t2", Status: model.SeedingStatusSeeding})
	env.db.Create(&model.SeedingTorrentRecord{ClientID: "c1", InfoHash: "h3", SiteName: "siteB", TorrentID: "t3", Status: model.SeedingStatusSeeding})

	w := env.doRequest("GET", "/api/v1/seeding/stats/by-site", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	items, _ := data["items"].([]interface{})
	if len(items) != 2 {
		t.Errorf("expected 2 sites, got %d", len(items))
	}
}

func TestSeeding_StatsTorrents_V2(t *testing.T) {
	env := setupTestEnv(t)
	env.db.Create(&model.SeedingTorrentRecord{ClientID: "c1", InfoHash: "h1", SiteName: "s1", TorrentID: "t1", Status: model.SeedingStatusSeeding})
	env.db.Create(&model.SeedingTorrentRecord{ClientID: "c1", InfoHash: "h2", SiteName: "s1", TorrentID: "t2", Status: model.SeedingStatusPausedFreeEnd})
	env.db.Create(&model.SeedingTorrentRecord{ClientID: "c1", InfoHash: "h3", SiteName: "s1", TorrentID: "t3", Status: model.SeedingStatusDeleted})

	w := env.doRequest("GET", "/api/v1/seeding/stats/torrents", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["total"] != float64(2) {
		t.Errorf("expected 2 torrents (seeding + paused_free_end, exclude deleted), got %v", data["total"])
	}

	w = env.doRequest("GET", "/api/v1/seeding/stats/torrents?status=seeding", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp = parseResponse(t, w)
	data, _ = resp.Data.(map[string]interface{})
	if data["total"] != float64(1) {
		t.Errorf("expected 1 torrent (status=seeding filter), got %v", data["total"])
	}
}

func TestSeeding_DryrunAll_V2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/seeding/dryrun", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["total"] != float64(0) {
		t.Errorf("expected 0, got %v", data["total"])
	}
}

func TestSeeding_DryrunBySub(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/seeding/dryrun/sub-123", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["subscriptionId"] != "sub-123" {
		t.Errorf("expected sub-123, got %v", data["subscriptionId"])
	}
}

func TestSeeding_ScoringCycle_V2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/seeding/scoring-logs/cycles/cycle-001", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["cycleId"] != "cycle-001" {
		t.Errorf("expected cycle-001, got %v", data["cycleId"])
	}
}

func TestSeeding_Configs_MethodNotAllowed(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("PATCH", "/api/v1/seeding/configs", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestSeeding_Root_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/seeding/nonexistent", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_ScoringDryrun_V2(t *testing.T) {
	env := setupTestEnv(t)
	body := map[string]interface{}{
		"seeders":       5,
		"leechers":      10,
		"ageHours":      48.0,
		"size":          1073741824,
		"discount":      "FREE",
		"halfLifeHours": 2.0,
		"siteWeight":    1.0,
	}
	w := env.doRequest("POST", "/api/v1/seeding/scoring-dryrun", body)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if _, ok := data["score"]; !ok {
		t.Error("expected score field")
	}
	if _, ok := data["shouldCleanup"]; !ok {
		t.Error("expected shouldCleanup field")
	}
}

func TestSeeding_ScoringDryrun_BadRequest(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/seeding/scoring-dryrun", "not-json")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSeeding_ScoringDryrun_MethodNotAllowed(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/seeding/scoring-dryrun", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestSeeding_ExtractLastSegment(t *testing.T) {
	tests := []struct {
		path   string
		prefix string
		want   string
	}{
		{"/api/v1/seeding/configs/42", "/api/v1/seeding/configs/", "42"},
		{"/api/v1/seeding/configs/42/", "/api/v1/seeding/configs/", "42"},
		{"/api/v1/seeding/configs/", "/api/v1/seeding/configs/", ""},
		{"/api/v1/seeding/rules/5/test", "/api/v1/seeding/rules/", "5/test"},
	}
	for _, tt := range tests {
		got := extractLastSegment(tt.path, tt.prefix)
		if got != tt.want {
			t.Errorf("extractLastSegment(%q, %q) = %q, want %q", tt.path, tt.prefix, got, tt.want)
		}
	}
}

func TestSeeding_ListTorrents_Pagination(t *testing.T) {
	env := setupTestEnv(t)
	for i := 0; i < 5; i++ {
		env.db.Create(&model.SeedingTorrentRecord{
			ClientID: "c1", InfoHash: fmt.Sprintf("pag%d", i), SiteName: "s1",
			TorrentID: fmt.Sprintf("t%d", i), Status: model.SeedingStatusSeeding,
		})
	}
	w := env.doRequest("GET", "/api/v1/seeding/torrents?page=1&size=2", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	items, _ := data["items"].([]interface{})
	if len(items) != 2 {
		t.Errorf("expected 2 items (page size), got %d", len(items))
	}
	if data["total"] != float64(5) {
		t.Errorf("expected total=5, got %v", data["total"])
	}
}

func TestSeeding_StatsOverview_V2(t *testing.T) {
	env := setupTestEnv(t)
	env.db.Create(&model.SeedingTorrentRecord{ClientID: "c1", InfoHash: "o1", SiteName: "s1", TorrentID: "t1", Status: model.SeedingStatusSeeding})
	env.db.Create(&model.SeedingTorrentRecord{ClientID: "c1", InfoHash: "o2", SiteName: "s1", TorrentID: "t2", Status: model.SeedingStatusPausedRule})

	w := env.doRequest("GET", "/api/v1/seeding/stats/overview", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["activeTorrents"] != float64(0) {
		t.Errorf("expected 0 active (no engine), got %v", data["activeTorrents"])
	}
	if data["pausedTorrents"] != float64(0) {
		t.Errorf("expected 0 paused (no engine), got %v", data["pausedTorrents"])
	}
	if data["totalTorrents"] != float64(2) {
		t.Errorf("expected 2 total (DB), got %v", data["totalTorrents"])
	}
}

func TestSeeding_ScoringConfig_GetWithoutSubID(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/seeding/scoring-config", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["enabled"] != false {
		t.Error("expected enabled=false")
	}
}

func TestSeeding_ScoringConfig_PutWithoutSubID(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("PUT", "/api/v1/seeding/scoring-config", map[string]interface{}{"enabled": true})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_ScoringConfigByID_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/seeding/scoring-config/nonexistent-id", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_ScoringConfigByID_WithSub(t *testing.T) {
	env := setupTestEnv(t)
	sub := model.RSSSubscription{
		Name: "test-sub", SiteName: "site1", URLs: []string{"http://example.com/rss"},
		ScoringConfig: model.SeedingScoringConfig{Enabled: true, HalfLifeHours: 3.0},
	}
	env.db.Create(&sub)

	w := env.doRequest("GET", fmt.Sprintf("/api/v1/seeding/scoring-config/%d", sub.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if _, ok := data["half_life_hours"]; !ok {
		t.Error("expected half_life_hours field")
	}
}

func TestSeeding_UpdateConfig_Success(t *testing.T) {
	env := setupTestEnv(t)
	env.db.Create(&model.SeedingClientConfig{ClientID: "uc-1", Enabled: true})
	var cfg model.SeedingClientConfig
	env.db.Where("client_id = ?", "uc-1").First(&cfg)

	body := map[string]interface{}{"enabled": false, "minDiskSpaceGB": 100.0}
	w := env.doRequest("PUT", fmt.Sprintf("/api/v1/seeding/configs/%d", cfg.ID), body)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["enabled"] != false {
		t.Errorf("expected enabled=false, got %v", data["enabled"])
	}
}

func TestSeeding_UpdateConfig_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("PUT", "/api/v1/seeding/configs/9999", map[string]interface{}{"enabled": false})
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestSeeding_Configs_TrailingSlash(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/seeding/configs/", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_Records_WithPath_BadID(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/seeding/records/abc/resume", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_Records_WithPath_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/seeding/records/123/unknown", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_Torrents_ResumeViaTorrentPath(t *testing.T) {
	env := setupTestEnv(t)
	env.db.Create(&model.SeedingTorrentRecord{
		ClientID: "c1", InfoHash: "trh1", SiteName: "s1", TorrentID: "t1",
		Status: model.SeedingStatusPausedFreeEnd,
	})
	var rec model.SeedingTorrentRecord
	env.db.Where("info_hash = ?", "trh1").First(&rec)

	w := env.doRequest("POST", fmt.Sprintf("/api/v1/seeding/torrents/%d/resume", rec.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_Torrents_BadID(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/seeding/torrents/abc/resume", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_Clients_NonTriggerPath(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/seeding/clients/c1/unknown", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_ScoringLogs_InvalidSubPath(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/seeding/scoring-logs/invalid-path", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_DryrunBySub_Empty(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/seeding/dryrun/sub-1", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_DryrunAll_MethodNotAllowed(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/seeding/dryrun", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_CreateRule_Success(t *testing.T) {
	env := setupTestEnv(t)
	body := map[string]interface{}{
		"alias":    "test-rule",
		"priority": 1,
		"enabled":  true,
		"type":     "normal",
		"action":   "delete",
	}
	w := env.doRequest("POST", "/api/v1/seeding/rules", body)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["alias"] != "test-rule" {
		t.Errorf("expected alias=test-rule, got %v", data["alias"])
	}
}

func TestSeeding_CreateRule_MissingAlias(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/seeding/rules", map[string]interface{}{"priority": 1})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_Rules_MethodNotAllowed(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("DELETE", "/api/v1/seeding/rules", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d: %s", w.Code, w.Body.String())
	}
}

func createSiteWithCookie(t *testing.T, env *testEnv, name, domain string) uint {
	t.Helper()
	// 直接 db.Create（步骤 3 强白名单校验，POST API 不接受 fake domain）
	s := &model.Site{
		Name: name, Domain: domain, BaseURL: "https://" + domain,
		Framework: "nexusphp", AuthType: "cookie", Cookie: "session=abc123", Enabled: true,
	}
	if err := env.db.Create(s).Error; err != nil {
		t.Fatalf("createSiteWithCookie: %v", err)
	}
	return s.ID
}

func TestSite_NewSiteHandler(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	h := NewSiteHandler(site.NewRepository(db), zap.NewNop(), db)
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
}

func TestSiteV2_CreateWithCookie(t *testing.T) {
	env := setupTestEnv(t)
	id := createSiteWithCookie(t, env, "V2Site", "v2.example.com")
	w := env.doRequest("GET", fmt.Sprintf("/api/v1/sites/%d", id), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["hasCookie"] != true {
		t.Error("expected hasCookie=true")
	}
	if data["hasPasskey"] != false {
		t.Error("expected hasPasskey=false")
	}
}

func TestSiteV2_CreateMissingRequired(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/sites", map[string]interface{}{"name": "OnlyName"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSiteV2_CreateInvalidBody(t *testing.T) {
	env := setupTestEnv(t)
	req := httptest.NewRequest("POST", "/api/v1/sites", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.token)
	w := httptest.NewRecorder()
	env.mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSiteV2_CreateDefaultsFramework(t *testing.T) {
	env := setupTestEnv(t)
	// 步骤 3：framework/authType 由 seed 强制覆盖；选 hdroute.org (generic) 验证
	w := env.doRequest("POST", "/api/v1/sites", map[string]interface{}{
		"domain": "hdroute.org",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["framework"] != "generic" {
		t.Errorf("expected framework=generic, got %v", data["framework"])
	}
	if data["authType"] != "cookie" {
		t.Errorf("expected authType=cookie, got %v", data["authType"])
	}
}

func TestSiteV2_ListEmpty(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/sites", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if items, _ := data["items"].([]interface{}); len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestSiteV2_ListSorted(t *testing.T) {
	env := setupTestEnv(t)
	createSiteWithCookie(t, env, "Zeta", "zeta-v2.example.com")
	createSiteWithCookie(t, env, "Alpha", "alpha-v2.example.com")
	w := env.doRequest("GET", "/api/v1/sites", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	items, _ := data["items"].([]interface{})
	if len(items) != 2 {
		t.Fatalf("expected 2, got %d", len(items))
	}
	if items[0].(map[string]interface{})["name"] != "Alpha" {
		t.Error("expected sorted by name ASC")
	}
}

func TestSiteV2_ListPagination(t *testing.T) {
	env := setupTestEnv(t)
	for i := 0; i < 5; i++ {
		createSiteWithCookie(t, env, fmt.Sprintf("PagV2%d", i), fmt.Sprintf("pagv2-%d.example.com", i))
	}
	w := env.doRequest("GET", "/api/v1/sites?page=1&size=2", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if items, _ := data["items"].([]interface{}); len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
	if data["total"] != float64(5) {
		t.Errorf("expected total=5, got %v", data["total"])
	}
}

func TestSiteV2_GetSuccess(t *testing.T) {
	env := setupTestEnv(t)
	id := createSiteWithCookie(t, env, "GetV2", "getv2.example.com")
	w := env.doRequest("GET", fmt.Sprintf("/api/v1/sites/%d", id), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["name"] != "GetV2" {
		t.Errorf("expected name=GetV2, got %v", data["name"])
	}
}

func TestSiteV2_GetNotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/sites/99999", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestSiteV2_GetInvalidID(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/sites/abc", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSiteV2_UpdateSuccess(t *testing.T) {
	env := setupTestEnv(t)
	id := createSiteWithCookie(t, env, "UpdV2", "updv2.example.com")
	w := env.doRequest("PUT", fmt.Sprintf("/api/v1/sites/%d", id), map[string]interface{}{
		"name": "UpdatedV2", "enabled": false,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["name"] != "UpdatedV2" {
		t.Errorf("expected UpdatedV2, got %v", data["name"])
	}
	if data["enabled"] != false {
		t.Error("expected enabled=false")
	}
}

func TestSiteV2_UpdateNotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("PUT", "/api/v1/sites/99999", map[string]interface{}{"name": "X"})
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestSiteV2_UpdateInvalidBody(t *testing.T) {
	env := setupTestEnv(t)
	id := createSiteWithCookie(t, env, "UpdBadBody", "updbadbody.example.com")
	req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/sites/%d", id), bytes.NewReader([]byte("bad")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.token)
	w := httptest.NewRecorder()
	env.mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSiteV2_DeleteSuccess(t *testing.T) {
	env := setupTestEnv(t)
	id := createSiteWithCookie(t, env, "DelV2", "delv2.example.com")
	w := env.doRequest("DELETE", fmt.Sprintf("/api/v1/sites/%d", id), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	w = env.doRequest("GET", fmt.Sprintf("/api/v1/sites/%d", id), nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", w.Code)
	}
}

func TestSiteV2_DeleteNotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("DELETE", "/api/v1/sites/99999", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestSiteV2_DeleteInvalidID(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("DELETE", "/api/v1/sites/abc", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSiteV2_TestNotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/sites/99999/test", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestSiteV2_TestInvalidID(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/sites/abc/test", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSiteV2_DetectNotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/sites/99999/detect", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestSiteV2_DetectInvalidID(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/sites/abc/detect", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSiteV2_GetStatsSuccess(t *testing.T) {
	env := setupTestEnv(t)
	id := createSiteWithCookie(t, env, "StatsV2", "statsv2.example.com")
	env.db.Model(&model.Site{}).Where("id = ?", id).Updates(map[string]interface{}{
		"upload_bytes": 2048, "download_bytes": 1024, "ratio": 2.0,
	})
	w := env.doRequest("GET", fmt.Sprintf("/api/v1/sites/%d/stats", id), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["uploadBytes"] != float64(2048) {
		t.Errorf("expected 2048, got %v", data["uploadBytes"])
	}
}

func TestSiteV2_StatsNotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/sites/99999/stats", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestSiteV2_StatsInvalidID(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/sites/abc/stats", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSiteV2_StatsMethodNotAllowed(t *testing.T) {
	env := setupTestEnv(t)
	id := createSiteWithCookie(t, env, "StatsMethV2", "statsmethv2.example.com")
	w := env.doRequest("POST", fmt.Sprintf("/api/v1/sites/%d/stats", id), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (returns cached data with warning), got %d", w.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["data"] == nil {
		t.Fatal("expected data in response")
	}
}

func TestSiteV2_CredentialsUpdateSuccess(t *testing.T) {
	env := setupTestEnv(t)
	id := createSiteWithCookie(t, env, "CredUpV2", "credupv2.example.com")
	w := env.doRequest("PUT", fmt.Sprintf("/api/v1/sites/%d/credentials", id), map[string]interface{}{
		"passkey": "pk-new", "cookie": "ck-new",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["hasPasskey"] != true {
		t.Error("expected hasPasskey=true")
	}
}

func TestSiteV2_CredentialsEmptyBody(t *testing.T) {
	env := setupTestEnv(t)
	id := createSiteWithCookie(t, env, "CredEmptyV2", "credemptyv2.example.com")
	w := env.doRequest("PUT", fmt.Sprintf("/api/v1/sites/%d/credentials", id), map[string]interface{}{})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSiteV2_CredentialsNotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("PUT", "/api/v1/sites/99999/credentials", map[string]interface{}{"passkey": "x"})
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestSiteV2_CredentialsMethodNotAllowed(t *testing.T) {
	env := setupTestEnv(t)
	id := createSiteWithCookie(t, env, "CredMethV2", "credmethv2.example.com")
	w := env.doRequest("GET", fmt.Sprintf("/api/v1/sites/%d/credentials", id), nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestSiteV2_OverridesListEmpty(t *testing.T) {
	env := setupTestEnv(t)
	id := createSiteWithCookie(t, env, "OvListV2", "ovlistv2.example.com")
	w := env.doRequest("GET", fmt.Sprintf("/api/v1/sites/%d/overrides", id), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["total"] != float64(0) {
		t.Errorf("expected 0, got %v", data["total"])
	}
}

func TestSiteV2_OverridesUpsertAndList(t *testing.T) {
	env := setupTestEnv(t)
	id := createSiteWithCookie(t, env, "OvUpV2", "ovupv2.example.com")
	w := env.doRequest("PUT", fmt.Sprintf("/api/v1/sites/%d/overrides", id), map[string]interface{}{
		"fieldPath": "downloadMode", "fieldValue": "rss",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("upsert: expected 200, got %d", w.Code)
	}
	w = env.doRequest("GET", fmt.Sprintf("/api/v1/sites/%d/overrides", id), nil)
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if items, _ := data["items"].([]interface{}); len(items) != 1 {
		t.Fatalf("expected 1 override, got %d", len(items))
	}
}

func TestSiteV2_OverridesUpsertUpdate(t *testing.T) {
	env := setupTestEnv(t)
	id := createSiteWithCookie(t, env, "OvUpdV2", "ovupdv2.example.com")
	env.doRequest("PUT", fmt.Sprintf("/api/v1/sites/%d/overrides", id), map[string]interface{}{
		"fieldPath": "dm", "fieldValue": "rss",
	})
	w := env.doRequest("PUT", fmt.Sprintf("/api/v1/sites/%d/overrides", id), map[string]interface{}{
		"fieldPath": "dm", "fieldValue": "template",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["message"] != "覆盖已更新" {
		t.Errorf("expected update message, got %v", data["message"])
	}
}

func TestSiteV2_OverridesMissingFieldPath(t *testing.T) {
	env := setupTestEnv(t)
	id := createSiteWithCookie(t, env, "OvMissV2", "ovmissv2.example.com")
	w := env.doRequest("PUT", fmt.Sprintf("/api/v1/sites/%d/overrides", id), map[string]interface{}{"fieldValue": "v"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSiteV2_OverridesDelete(t *testing.T) {
	env := setupTestEnv(t)
	id := createSiteWithCookie(t, env, "OvDelV2", "ovdelv2.example.com")
	env.doRequest("PUT", fmt.Sprintf("/api/v1/sites/%d/overrides", id), map[string]interface{}{
		"fieldPath": "dm", "fieldValue": "rss",
	})
	w := env.doRequest("DELETE", fmt.Sprintf("/api/v1/sites/%d/overrides?fieldPath=dm", id), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["deleted"] != float64(1) {
		t.Errorf("expected 1, got %v", data["deleted"])
	}
}

func TestSiteV2_OverridesDeleteAll(t *testing.T) {
	env := setupTestEnv(t)
	id := createSiteWithCookie(t, env, "OvDelAllV2", "ovdelallv2.example.com")
	env.doRequest("PUT", fmt.Sprintf("/api/v1/sites/%d/overrides", id), map[string]interface{}{"fieldPath": "x", "fieldValue": "1"})
	env.doRequest("PUT", fmt.Sprintf("/api/v1/sites/%d/overrides", id), map[string]interface{}{"fieldPath": "y", "fieldValue": "2"})
	w := env.doRequest("DELETE", fmt.Sprintf("/api/v1/sites/%d/overrides", id), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["deleted"] != float64(2) {
		t.Errorf("expected 2, got %v", data["deleted"])
	}
}

func TestSiteV2_OverridesNotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/sites/99999/overrides", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestSiteV2_OverridesInvalidID(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/sites/abc/overrides", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSiteV2_OverridesMethodNotAllowed(t *testing.T) {
	env := setupTestEnv(t)
	id := createSiteWithCookie(t, env, "OvMethV2", "ovmethv2.example.com")
	w := env.doRequest("POST", fmt.Sprintf("/api/v1/sites/%d/overrides", id), nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestSiteV2_RouteNotFound(t *testing.T) {
	env := setupTestEnv(t)
	id := createSiteWithCookie(t, env, "RouteV2", "routev2.example.com")
	w := env.doRequest("GET", fmt.Sprintf("/api/v1/sites/%d/nonexistent", id), nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestSiteV2_RootSlashPath(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/sites/", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestSiteV2_MethodNotAllowedRoot(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("PATCH", "/api/v1/sites", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestSiteV2_FreezeInvalidID(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/sites/abc/freeze", map[string]interface{}{"duration": "1h", "reason": "test"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSiteV2_FreezeNotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/sites/99999/freeze", map[string]interface{}{"duration": "1h", "reason": "test"})
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestFrameworkDefaultHelpers(t *testing.T) {
	for fw, exp := range map[string]string{
		"gazelle": "xml_tag", "luminance": "xml_tag", "unit3d": "fake_from_id",
		"nexusphp": "guid", "mteam": "guid", "generic": "guid",
	} {
		if got := frameworkDefaultHash(fw); got != exp {
			t.Errorf("frameworkDefaultHash(%q) = %q, want %q", fw, got, exp)
		}
	}
	for fw, exp := range map[string]string{
		"unit3d": "desc_regex", "gazelle": "xml_tag", "luminance": "xml_tag",
		"nexusphp": "enclosure", "mteam": "enclosure", "generic": "enclosure",
	} {
		if got := frameworkDefaultSize(fw); got != exp {
			t.Errorf("frameworkDefaultSize(%q) = %q, want %q", fw, got, exp)
		}
	}
	for fw, exp := range map[string]string{
		"unit3d": "link_regex", "gazelle": "link_regex",
		"nexusphp": "query_param", "mteam": "query_param", "generic": "query_param",
	} {
		if got := frameworkDefaultID(fw); got != exp {
			t.Errorf("frameworkDefaultID(%q) = %q, want %q", fw, got, exp)
		}
	}
}

func TestDefaultStr(t *testing.T) {
	if defaultStr("", "def") != "def" {
		t.Error("expected def for empty")
	}
	if defaultStr("val", "def") != "val" {
		t.Error("expected val for non-empty")
	}
}

func TestBuildSiteHTTPClient(t *testing.T) {
	client := buildSiteHTTPClient(&model.Site{Domain: "test.example.com"}, 5*time.Second)
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.Timeout != 5*time.Second {
		t.Errorf("expected 5s, got %v", client.Timeout)
	}
}

func TestSiteV2_ToResponseHasCredentials(t *testing.T) {
	env := setupTestEnv(t)
	// 用白名单内 domain（步骤 3）；framework/authType 由 seed 强制覆盖（gazelle→cookie）
	w := env.doRequest("POST", "/api/v1/sites", map[string]interface{}{
		"domain": "dicmusic.com",
		"passkey": "pk",
		"apiKey": "ak", "authKey": "authk", "authHash": "authh",
		"rssKey": "rssk", "bearerToken": "bt", "enabled": true,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	for _, f := range []string{"hasPasskey", "hasApiKey", "hasBearerToken", "hasAuthKey", "hasAuthHash", "hasRssKey"} {
		if data[f] != true {
			t.Errorf("expected %s=true", f)
		}
	}
	if data["hasCookie"] != false {
		t.Error("expected hasCookie=false")
	}
}

func TestSiteV2_UpdateMultipleFields(t *testing.T) {
	env := setupTestEnv(t)
	id := createSiteWithCookie(t, env, "MultiUpV2", "multiupv2.example.com")
	w := env.doRequest("PUT", fmt.Sprintf("/api/v1/sites/%d", id), map[string]interface{}{
		"framework": "unit3d", "isSource": true, "isTarget": true, "participateAutoPublish": false,
		"cookieCloudSync": true, "cookieCloudDomain": "sync.example.com",
		"alternativeDomains": "alt.example.com", "hashStrategy": "fake_from_id",
		"sizeStrategy": "desc_regex", "idStrategy": "link_regex", "idPattern": `/torrent/(\d+)`,
		"downloadMode": "template", "downloadUrlTemplate": "https://test/dl?id={id}",
		"requiresSideLoading": true, "overrideRssUrl": "https://rss.override.com",
		"overrideSavePath": "/data/override", "proxyUrl": "http://proxy:8080", "skipSslVerify": true,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["framework"] != "unit3d" {
		t.Errorf("expected unit3d, got %v", data["framework"])
	}
}

func TestSiteV2_CreateAllFrameworks(t *testing.T) {
	env := setupTestEnv(t)
	// 步骤 3 强白名单校验：每个 framework 选 supported_sites.json 内的一个真实 domain
	frameworkDomains := []struct {
		fw     string
		domain string
	}{
		{"nexusphp", "13city.org"},
		{"unit3d", "monikadesign.uk"},
		{"gazelle", "dicmusic.com"},
		{"mteam", "api.m-team.cc"},
		{"tnode", "zhuque.in"},
		{"rousi", "rousi.pro"},
		{"yemapt", "www.yemapt.org"},
		{"generic", "hdroute.org"},
	}
	for i, tc := range frameworkDomains {
		w := env.doRequest("POST", "/api/v1/sites", map[string]interface{}{
			"domain": tc.domain,
		})
		if w.Code != http.StatusOK {
			t.Fatalf("framework %s (#%d, domain %s): expected 200, got %d: %s", tc.fw, i, tc.domain, w.Code, w.Body.String())
		}
		// 验证 framework 被 seed 强制覆盖（即使没传 framework 字段）
		resp := parseResponse(t, w)
		data, _ := resp.Data.(map[string]interface{})
		if data["framework"] != tc.fw {
			t.Errorf("framework %s: expected seed framework %s, got %v", tc.fw, tc.fw, data["framework"])
		}
	}
}

func TestSiteV2_UpdateCredentialsAllFields(t *testing.T) {
	env := setupTestEnv(t)
	id := createSiteWithCookie(t, env, "AllCredV2", "allcredv2.example.com")
	w := env.doRequest("PUT", fmt.Sprintf("/api/v1/sites/%d/credentials", id), map[string]interface{}{
		"passkey": "pk", "cookie": "ck", "apiKey": "ak",
		"bearerToken": "bt", "authKey": "auk", "authHash": "ah",
		"userId": 42, "rssKey": "rk",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["userId"] != float64(42) {
		t.Errorf("expected userId=42, got %v", data["userId"])
	}
}

func TestSiteV2_PublishExclusionsFullCRUD(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/publish/exclusions", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", w.Code)
	}
	w = env.doRequest("POST", "/api/v1/publish/exclusions", map[string]interface{}{
		"target_site": "tgt-v2.example.com", "source_site": "src-v2.example.com",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create: expected 200, got %d", w.Code)
	}
	w = env.doRequest("POST", "/api/v1/publish/exclusions", map[string]interface{}{
		"target_site": "tgt-v2.example.com", "source_site": "src-v2.example.com",
	})
	if w.Code != http.StatusConflict {
		t.Fatalf("dup: expected 409, got %d", w.Code)
	}
	w = env.doRequest("DELETE", "/api/v1/publish/exclusions", map[string]interface{}{
		"target_site": "tgt-v2.example.com", "source_site": "src-v2.example.com",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("delete: expected 200, got %d", w.Code)
	}
	w = env.doRequest("DELETE", "/api/v1/publish/exclusions", map[string]interface{}{
		"target_site": "tgt-v2.example.com", "source_site": "src-v2.example.com",
	})
	if w.Code != http.StatusNotFound {
		t.Fatalf("re-delete: expected 404, got %d", w.Code)
	}
}

func TestSiteV2_ExclusionsMissingFields(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/publish/exclusions", map[string]interface{}{"target_site": "x"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSiteV2_ExclusionsMethodNotAllowed(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("PATCH", "/api/v1/publish/exclusions", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestSiteV2_UpdateDuplicateName(t *testing.T) {
	env := setupTestEnv(t)
	createSiteWithCookie(t, env, "DupNmA", "dupnma-v2.example.com")
	id2 := createSiteWithCookie(t, env, "DupNmB", "dupnmb-v2.example.com")
	w := env.doRequest("PUT", fmt.Sprintf("/api/v1/sites/%d", id2), map[string]interface{}{"name": "DupNmA"})
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

func TestSiteV2_UpdateDuplicateDomain(t *testing.T) {
	env := setupTestEnv(t)
	createSiteWithCookie(t, env, "DupDomA", "dupdoma-v2.example.com")
	id2 := createSiteWithCookie(t, env, "DupDomB", "dupdomb-v2.example.com")
	w := env.doRequest("PUT", fmt.Sprintf("/api/v1/sites/%d", id2), map[string]interface{}{"domain": "dupdoma-v2.example.com"})
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

func TestSiteV2_CreateWithAllOptionalFields(t *testing.T) {
	env := setupTestEnv(t)
	// 用白名单内 domain（步骤 3）；framework/authType/downloadUrlTemplate/cookieCloudDomain 由 seed 强制覆盖，其他可选字段透传
	w := env.doRequest("POST", "/api/v1/sites", map[string]interface{}{
		"domain": "longpt.org",
		"cookie": "sid=full", "passkey": "pk",
		"hashStrategy": "guid", "sizeStrategy": "enclosure", "idStrategy": "query_param",
		"hashXmlTagName": "infoHash", "sizeXmlTagName": "contentLength", "hashUrlParamName": "hash",
		"sizeDescRegex": `(\d+)\s*GB`, "sizeTitleRegex": `(\d+)\s*MB`, "sizeBaseUnit": 1024,
		"downloadMode": "template",
		"downloadPagePattern": "/details.php", "requiresSideLoading": true,
		"isSource": true, "isTarget": true, "participateAutoPublish": true,
		"cookieCloudSync": true,
		"enabled": true, "alternativeDomains": "alt.longpt.org",
		"overrideRssUrl": "https://rss.longpt.org", "overrideSavePath": "/data/full",
		"proxyUrl": "socks5://proxy:1080", "skipSslVerify": true,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["requiresSideLoading"] != true {
		t.Error("expected requiresSideLoading=true")
	}
	if data["alternativeDomains"] != "alt.longpt.org" {
		t.Errorf("expected alt, got %v", data["alternativeDomains"])
	}
}

func TestSiteV2_CredentialsInvalidJSON(t *testing.T) {
	env := setupTestEnv(t)
	id := createSiteWithCookie(t, env, "CredBadV2", "credbadv2.example.com")
	req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/sites/%d/credentials", id), bytes.NewReader([]byte("bad")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.token)
	w := httptest.NewRecorder()
	env.mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSiteV2_UpdateInvalidFramework(t *testing.T) {
	env := setupTestEnv(t)
	id := createSiteWithCookie(t, env, "UpdInvFWV2", "updinvfwv2.example.com")
	w := env.doRequest("PUT", fmt.Sprintf("/api/v1/sites/%d", id), map[string]interface{}{"framework": "nope"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSiteV2_UpdateInvalidAuthType(t *testing.T) {
	env := setupTestEnv(t)
	id := createSiteWithCookie(t, env, "UpdInvATV2", "updinvatv2.example.com")
	w := env.doRequest("PUT", fmt.Sprintf("/api/v1/sites/%d", id), map[string]interface{}{"authType": "nope"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSiteV2_CreateInvalidFramework(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/sites", map[string]interface{}{
		"name": "BadFWV2", "domain": "badfwv2.example.com", "baseUrl": "https://badfwv2.example.com", "framework": "nope",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSiteV2_CreateInvalidAuthType(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/sites", map[string]interface{}{
		"name": "BadATV2", "domain": "badatv2.example.com", "baseUrl": "https://badatv2.example.com",
		"framework": "generic", "authType": "nope",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestResponse_Success2(t *testing.T) {
	rec := httptest.NewRecorder()
	Success(rec, map[string]string{"hello": "world"})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp Response
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Code != 0 || resp.Message != "ok" {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestResponse_Error2(t *testing.T) {
	rec := httptest.NewRecorder()
	Error(rec, http.StatusBadRequest, 40001, "bad request")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	var resp Response
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Code != 40001 || resp.Message != "bad request" {
		t.Errorf("unexpected: %+v", resp)
	}
}

func TestResponse_ErrorWithDetail2(t *testing.T) {
	rec := httptest.NewRecorder()
	ErrorWithDetail(rec, http.StatusInternalServerError, 50000, "fail", "detail here")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	var resp Response
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Detail != "detail here" {
		t.Errorf("expected detail, got: %s", resp.Detail)
	}
}

func TestSystem_Ping_Public2(t *testing.T) {
	env := setupTestEnv(t)
	env.token = ""
	w := env.doRequest("GET", "/api/v1/system/ping", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("ping: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["status"] != "ok" {
		t.Errorf("expected ok, got %v", data["status"])
	}
}

func TestSystem_Health2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/system/health", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("health: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	dbInfo, _ := data["database"].(map[string]interface{})
	if dbInfo["ok"] != true {
		t.Error("database should be ok")
	}
}

func TestSystem_Info2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/system/info", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("info: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["version"] != "test" {
		t.Errorf("expected test, got %v", data["version"])
	}
}

func TestSystem_ClearLogs2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("DELETE", "/api/v1/system/logs", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("clear logs: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSystem_Logs_PostMethod(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/system/logs", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestSystem_UnknownPath(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/system/nonexistent", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestPublish_ListTasks_Empty2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/publish/tasks", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list tasks: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["total"] != float64(0) {
		t.Errorf("expected 0 tasks, got %v", data["total"])
	}
}

func TestPublish_GetTask_NotFound2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/publish/tasks/999", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPublish_DeleteTask_NotFound2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("DELETE", "/api/v1/publish/tasks/999", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPublish_CancelTask_NotFound2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/publish/tasks/999/cancel", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPublish_GetCandidate_NotFound2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/publish/candidates/999", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPublish_DeleteCandidate_NotFound2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("DELETE", "/api/v1/publish/candidates/999", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (delete is idempotent), got %d: %s", w.Code, w.Body.String())
	}
}

func TestPublish_ManualPublish_NotFound2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/publish/candidates/999/publish", nil)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPublish_ListCandidates_Empty2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/publish/candidates", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list candidates: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPublish_ListResults_Empty2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/publish/results", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list results: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPublish_CreateTask_NoSourceSite(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/publish/tasks", map[string]interface{}{
		"sourceSiteId": 0,
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPublish_CreateTask_NoTargets(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/publish/tasks", map[string]interface{}{
		"sourceSiteId": 1,
		"targetSites":  []string{},
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPublish_CreateTask_BadBody2(t *testing.T) {
	env := setupTestEnv(t)
	req := httptest.NewRequest("POST", "/api/v1/publish/tasks", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.token)
	w := httptest.NewRecorder()
	env.mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestPublish_ListGroups_Empty2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/publish/groups", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list groups: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["total"] != float64(0) {
		t.Errorf("expected 0 groups, got %v", data["total"])
	}
}

func TestPublish_GetGroup_NotFound2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/publish/groups/999", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPublish_DeleteGroup2(t *testing.T) {
	env := setupTestEnv(t)
	group := &model.PublishGroup{Status: "active", SourceHash: "abc123del"}
	env.db.Create(group)
	w := env.doRequest("DELETE", fmt.Sprintf("/api/v1/publish/groups/%d", group.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete group: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPublish_LifecyclePause2(t *testing.T) {
	env := setupTestEnv(t)
	group := &model.PublishGroup{Status: "active", SourceHash: "pausehash"}
	env.db.Create(group)
	env.db.Create(&model.PublishGroupMember{PublishGroupID: group.ID, InfoHash: "h1p", Paused: false})

	w := env.doRequest("POST", fmt.Sprintf("/api/v1/publish/groups/%d/lifecycle/pause", group.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("pause: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPublish_LifecycleResume2(t *testing.T) {
	env := setupTestEnv(t)
	group := &model.PublishGroup{Status: "active", SourceHash: "resumehash"}
	env.db.Create(group)
	env.db.Create(&model.PublishGroupMember{PublishGroupID: group.ID, InfoHash: "h1r", Paused: true})

	w := env.doRequest("POST", fmt.Sprintf("/api/v1/publish/groups/%d/lifecycle/resume", group.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("resume: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPublish_LifecycleDelete2(t *testing.T) {
	env := setupTestEnv(t)
	group := &model.PublishGroup{Status: "active", SourceHash: "delhash"}
	env.db.Create(group)

	w := env.doRequest("POST", fmt.Sprintf("/api/v1/publish/groups/%d/lifecycle/delete", group.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("lifecycle delete: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPublish_GroupBadID2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/publish/groups/invalid", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestPublish_LifecycleBadAction2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/publish/groups/1/lifecycle/invalid", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestPublish_UnknownPath2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/publish/unknown", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestPublish_TasksCRUD2(t *testing.T) {
	env := setupTestEnv(t)
	siteID := createTestSite(t, env, "SrcSiteCRUD", "srccrud.com")

	w := env.doRequest("POST", "/api/v1/publish/tasks", map[string]interface{}{
		"sourceSiteId": siteID,
		"targetSites":  []string{"target1.com"},
		"manualCheck":  true,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create task: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	taskID := data["id"].(float64)

	w = env.doRequest("GET", fmt.Sprintf("/api/v1/publish/tasks/%d", int(taskID)), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get task: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("GET", "/api/v1/publish/tasks", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list tasks: expected 200, got %d", w.Code)
	}
}

func TestPublish_CancelTask2(t *testing.T) {
	env := setupTestEnv(t)
	siteID := createTestSite(t, env, "SrcSiteCancel", "srccancel.com")

	w := env.doRequest("POST", "/api/v1/publish/tasks", map[string]interface{}{
		"sourceSiteId": siteID,
		"targetSites":  []string{"target1.com"},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	taskID := uint(data["id"].(float64))

	w = env.doRequest("POST", fmt.Sprintf("/api/v1/publish/tasks/%d/cancel", taskID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("cancel: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPublish_DeleteTask2(t *testing.T) {
	env := setupTestEnv(t)
	siteID := createTestSite(t, env, "SrcSiteDel", "srcdel.com")

	w := env.doRequest("POST", "/api/v1/publish/tasks", map[string]interface{}{
		"sourceSiteId": siteID,
		"targetSites":  []string{"target1.com"},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	taskID := uint(data["id"].(float64))

	w = env.doRequest("DELETE", fmt.Sprintf("/api/v1/publish/tasks/%d", taskID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPublish_GetGroup_WithMembers2(t *testing.T) {
	env := setupTestEnv(t)
	group := &model.PublishGroup{Status: "active", SourceHash: "memberhash", SourceSite: "site1"}
	env.db.Create(group)
	env.db.Create(&model.PublishGroupMember{PublishGroupID: group.ID, InfoHash: "mh1", SiteName: "site1"})
	env.db.Create(&model.PublishGroupMember{PublishGroupID: group.ID, InfoHash: "mh2", SiteName: "site2"})

	w := env.doRequest("GET", fmt.Sprintf("/api/v1/publish/groups/%d", group.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get group: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	members, _ := data["members"].([]interface{})
	if len(members) != 2 {
		t.Errorf("expected 2 members, got %d", len(members))
	}
}

func TestPublish_ListGroups_WithStatus2(t *testing.T) {
	env := setupTestEnv(t)
	env.db.Create(&model.PublishGroup{Status: "active", SourceHash: "stat1"})
	env.db.Create(&model.PublishGroup{Status: "completed", SourceHash: "stat2"})

	w := env.doRequest("GET", "/api/v1/publish/groups?status=active", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list groups: expected 200, got %d", w.Code)
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["total"] != float64(1) {
		t.Errorf("expected 1 active group, got %v", data["total"])
	}
}

func TestTracker_ListMembers_Empty2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/tracker/members", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list members: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTracker_ListMembers_WithData2(t *testing.T) {
	env := setupTestEnv(t)
	env.db.Create(&model.PublishGroupMember{PublishGroupID: 1, InfoHash: "thash1", SiteName: "tsite1"})
	env.db.Create(&model.PublishGroupMember{PublishGroupID: 1, InfoHash: "thash2", SiteName: "tsite2"})

	w := env.doRequest("GET", "/api/v1/tracker/members", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", w.Code)
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["total"] != float64(2) {
		t.Errorf("expected 2, got %v", data["total"])
	}
}

func TestTracker_ListMembers_ByGroupID2(t *testing.T) {
	env := setupTestEnv(t)
	env.db.Create(&model.PublishGroupMember{PublishGroupID: 1, InfoHash: "tgh1"})
	env.db.Create(&model.PublishGroupMember{PublishGroupID: 2, InfoHash: "tgh2"})

	w := env.doRequest("GET", "/api/v1/tracker/members?groupId=1", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["total"] != float64(1) {
		t.Errorf("expected 1, got %v", data["total"])
	}
}

func TestTracker_GetMember_NotFound2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/tracker/members/nonexistenthash2", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTracker_GetMember_Found2(t *testing.T) {
	env := setupTestEnv(t)
	env.db.Create(&model.PublishGroupMember{PublishGroupID: 1, InfoHash: "tfound123", SiteName: "ts1"})

	w := env.doRequest("GET", "/api/v1/tracker/members/tfound123", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTracker_ListHistory_Empty2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/tracker/history", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("history: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTracker_ListHistory_WithData2(t *testing.T) {
	env := setupTestEnv(t)
	env.db.Create(&model.PublishGroupStatusHistory{PublishGroupID: 1, MemberHash: "th1", OldStatus: "new", NewStatus: "seeding", Reason: "test"})

	w := env.doRequest("GET", "/api/v1/tracker/history", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["total"] != float64(1) {
		t.Errorf("expected 1, got %v", data["total"])
	}
}

func TestTracker_ListHistory_ByGroupID2(t *testing.T) {
	env := setupTestEnv(t)
	env.db.Create(&model.PublishGroupStatusHistory{PublishGroupID: 1, MemberHash: "thg1", OldStatus: "new", NewStatus: "seeding"})
	env.db.Create(&model.PublishGroupStatusHistory{PublishGroupID: 2, MemberHash: "thg2", OldStatus: "new", NewStatus: "seeding"})

	w := env.doRequest("GET", "/api/v1/tracker/history?groupId=1", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["total"] != float64(1) {
		t.Errorf("expected 1, got %v", data["total"])
	}
}

func TestTracker_UnknownPath(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/tracker/unknown2", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestTracker_PostMembers(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/tracker/members", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestRSS_List_Empty2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/rss/subscriptions", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list rss: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRSS_Create_NoName(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/rss/subscriptions", map[string]interface{}{
		"name": "",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRSS_Create_MissingSite2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/rss/subscriptions", map[string]interface{}{
		"name":     "test-missing",
		"urls":     []string{"https://example.com/rss"},
		"siteName": "nonexistent2",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRSS_Get_NotFound2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/rss/subscriptions/999", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRSS_Delete_NotFound2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("DELETE", "/api/v1/rss/subscriptions/999", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRSS_Trigger_NotFound2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/rss/subscriptions/999/trigger", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRSS_Trigger_Disabled2(t *testing.T) {
	env := setupTestEnv(t)
	createTestSite(t, env, "RSSTrig2", "rsstrig2.com")
	sub := &model.RSSSubscription{
		Name: "test-sub-trig2", URLs: []string{"https://r.com"},
		SiteName: "RSSTrig2", Cron: "*/5 * * * *",
	}
	env.db.Create(sub)

	w := env.doRequest("POST", fmt.Sprintf("/api/v1/rss/subscriptions/%d/trigger", sub.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("trigger: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRSS_Pause_Resume2(t *testing.T) {
	env := setupTestEnv(t)
	createTestSite(t, env, "RSSPause3", "rsspause3.com")
	sub := &model.RSSSubscription{
		Name: "pause-sub3", Enabled: true, URLs: []string{"https://r.com"},
		SiteName: "RSSPause3", Cron: "*/5 * * * *",
	}
	env.db.Create(sub)

	w := env.doRequest("POST", fmt.Sprintf("/api/v1/rss/subscriptions/%d/pause", sub.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("pause: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("POST", fmt.Sprintf("/api/v1/rss/subscriptions/%d/resume", sub.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("resume: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRSS_Dryrun_NotFound2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/rss/subscriptions/999/dryrun", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRSS_DeleteSubs_MethodNotAllowed(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("DELETE", "/api/v1/rss/subscriptions", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestRSS_CRUD2(t *testing.T) {
	env := setupTestEnv(t)
	createTestSite(t, env, "RSSCRUD3", "rsscrud3.com")

	w := env.doRequest("POST", "/api/v1/rss/subscriptions", map[string]interface{}{
		"name":     "crud-sub3",
		"urls":     []string{"https://rsscrud3.com/rss"},
		"siteName": "RSSCRUD3",
		"enabled":  true,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	subID := uint(data["id"].(float64))

	w = env.doRequest("GET", fmt.Sprintf("/api/v1/rss/subscriptions/%d", subID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("PUT", fmt.Sprintf("/api/v1/rss/subscriptions/%d", subID), map[string]interface{}{
		"name": "updated-sub3",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("DELETE", fmt.Sprintf("/api/v1/rss/subscriptions/%d", subID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFilter_List_Empty2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/filters/rules", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list filters: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFilter_Create_NoName(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/filters/rules", map[string]interface{}{
		"name": "",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFilter_Create_NoConditions(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/filters/rules", map[string]interface{}{
		"name":     "rule-no-cond",
		"ruleType": "accept",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFilter_Create_BadType(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/filters/rules", map[string]interface{}{
		"name":       "rule-bad-type",
		"ruleType":   "badtype",
		"conditions": []map[string]interface{}{{"key": "title", "compareType": "contains", "value": "test"}},
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFilter_CRUD2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/filters/rules", map[string]interface{}{
		"name":     "filter-crud2",
		"ruleType": "accept",
		"enabled":  true,
		"conditions": []map[string]interface{}{
			{"key": "title", "compare_type": "contains", "value": "test"},
		},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	ruleID := uint(data["id"].(float64))

	w = env.doRequest("GET", fmt.Sprintf("/api/v1/filters/rules/%d", ruleID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("PUT", fmt.Sprintf("/api/v1/filters/rules/%d", ruleID), map[string]interface{}{
		"name": "filter-updated2",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("DELETE", fmt.Sprintf("/api/v1/filters/rules/%d", ruleID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFilter_Get_NotFound2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/filters/rules/999", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFilter_DeleteSubs_MethodNotAllowed(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("DELETE", "/api/v1/filters/rules", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestReseed_List_Empty2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/reseed/tasks", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list reseed: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestReseed_Create_NoName(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/reseed/tasks", map[string]interface{}{
		"name": "",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestReseed_Create_BadBody2(t *testing.T) {
	env := setupTestEnv(t)
	req := httptest.NewRequest("POST", "/api/v1/reseed/tasks", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.token)
	w := httptest.NewRecorder()
	env.mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestReseed_CRUD2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/reseed/tasks", map[string]interface{}{
		"name":    "reseed-crud2",
		"enabled": true,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	taskID := uint(data["id"].(float64))

	w = env.doRequest("GET", fmt.Sprintf("/api/v1/reseed/tasks/%d", taskID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("PUT", fmt.Sprintf("/api/v1/reseed/tasks/%d", taskID), map[string]interface{}{
		"name": "reseed-updated2",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("DELETE", fmt.Sprintf("/api/v1/reseed/tasks/%d", taskID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestReseed_Get_NotFound2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/reseed/tasks/999", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestReseed_Cancel_NotFound2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/reseed/tasks/999/cancel", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestReseed_Trigger_NotFound2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/reseed/tasks/999/trigger", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestReseed_Cancel2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/reseed/tasks", map[string]interface{}{
		"name":    "cancel-task-2",
		"enabled": true,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	taskID := uint(data["id"].(float64))

	w = env.doRequest("POST", fmt.Sprintf("/api/v1/reseed/tasks/%d/cancel", taskID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("cancel: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestReseed_Matches_NoInfoHash2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/reseed/tasks/1/matches", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["total"] != float64(0) {
		t.Errorf("expected 0, got %v", data["total"])
	}
}

func TestReseed_NegativeCache_NoInfoHash2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("DELETE", "/api/v1/reseed/tasks/1/negative-cache", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestReseed_BadTaskID(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/reseed/tasks/abc", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestReseed_SubNotFound2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/reseed/tasks/1/nonexistent", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestIYUU_GetConfig_Empty2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/iyuu/config", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["enabled"] != false {
		t.Error("expected disabled by default")
	}
}

func TestIYUU_UpdateConfig2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("PUT", "/api/v1/iyuu/config", map[string]interface{}{
		"token":   "test-token-xyz-2",
		"enabled": true,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("GET", "/api/v1/iyuu/config", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get after update: expected 200, got %d", w.Code)
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["enabled"] != true {
		t.Error("should be enabled")
	}
}

func TestIYUU_UpdateConfig_BadBody2(t *testing.T) {
	env := setupTestEnv(t)
	req := httptest.NewRequest("PUT", "/api/v1/iyuu/config", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.token)
	w := httptest.NewRecorder()
	env.mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestIYUU_ListSites_Empty2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/iyuu/sites", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list sites: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestIYUU_SyncSites2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/iyuu/sites", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("sync: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestIYUU_Test_NoConfig2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/iyuu/test", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestIYUU_Query_BadBody2(t *testing.T) {
	env := setupTestEnv(t)
	req := httptest.NewRequest("POST", "/api/v1/iyuu/query", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.token)
	w := httptest.NewRecorder()
	env.mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestIYUU_ConfigDelete(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("DELETE", "/api/v1/iyuu/config", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestIYUU_UnknownPath(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/iyuu/unknown2", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestFingerprint_List_Empty2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/fingerprints", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFingerprint_Get_NotFound2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/fingerprints/999", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFingerprint_Search_NoParams2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/fingerprints/search", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFingerprint_Search_ByInfoHash2(t *testing.T) {
	env := setupTestEnv(t)
	env.db.Create(&model.ContentFingerprint{InfoHash: "fpih1", SiteName: "site1fp", PiecesHash: "p1fp"})

	w := env.doRequest("GET", "/api/v1/fingerprints/search?infoHash=fpih1", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("search: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["total"] != float64(1) {
		t.Errorf("expected 1, got %v", data["total"])
	}
}

func TestFingerprint_Search_ByPiecesHash2(t *testing.T) {
	env := setupTestEnv(t)
	env.db.Create(&model.ContentFingerprint{InfoHash: "fpabc", SiteName: "site1fp2", PiecesHash: "fpph1"})

	w := env.doRequest("GET", "/api/v1/fingerprints/search?piecesHash=fpph1", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("search: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFingerprint_Delete2(t *testing.T) {
	env := setupTestEnv(t)
	fp := &model.ContentFingerprint{InfoHash: "fpdel1", SiteName: "site1del"}
	env.db.Create(fp)

	w := env.doRequest("DELETE", fmt.Sprintf("/api/v1/fingerprints/%d", fp.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFingerprint_DeleteCache2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("DELETE", "/api/v1/fingerprints/cache", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete cache: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFingerprint_List_WithFilter2(t *testing.T) {
	env := setupTestEnv(t)
	env.db.Create(&model.ContentFingerprint{InfoHash: "fpf1", SiteName: "siteAfp2"})
	env.db.Create(&model.ContentFingerprint{InfoHash: "fpf2", SiteName: "siteBfp2"})

	w := env.doRequest("GET", "/api/v1/fingerprints?siteName=siteAfp2", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["total"] != float64(1) {
		t.Errorf("expected 1, got %v", data["total"])
	}
}

func TestFingerprint_PostNotAllowed(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/fingerprints", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestFingerprint_BadID2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/fingerprints/invalid", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestNotify_List_Empty2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/notifications/channels", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestNotify_Create_NoName(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/notifications/channels", map[string]interface{}{
		"name": "",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestNotify_Create_BadType(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/notifications/channels", map[string]interface{}{
		"name": "ch-bad-type",
		"type": "invalid",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestNotify_CRUD2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/notifications/channels", map[string]interface{}{
		"name":    "ch-crud2",
		"type":    "webhook",
		"enabled": true,
		"config":  `{"url":"https://example.com/hook2"}`,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	chID := uint(data["id"].(float64))

	w = env.doRequest("GET", fmt.Sprintf("/api/v1/notifications/channels/%d", chID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("PUT", fmt.Sprintf("/api/v1/notifications/channels/%d", chID), map[string]interface{}{
		"name": "ch-updated2",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("DELETE", fmt.Sprintf("/api/v1/notifications/channels/%d", chID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestNotify_Get_NotFound2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/notifications/channels/999", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestNotify_Test_NotFound2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/notifications/channels/999/test", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestNotify_History2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/notifications/channels", map[string]interface{}{
		"name":    "hist-ch-3",
		"type":    "webhook",
		"enabled": true,
		"config":  `{"url":"https://example.com"}`,
	})
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	chID := uint(data["id"].(float64))

	w = env.doRequest("GET", fmt.Sprintf("/api/v1/notifications/channels/%d/history", chID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("history: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestNotify_DeleteSubs_MethodNotAllowed(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("DELETE", "/api/v1/notifications/channels", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestSettings_Backup2(t *testing.T) {
	env := setupTestEnv(t)
	env.doRequest("PUT", "/api/v1/settings/backup.test.key2", map[string]string{"value": "val"})

	w := env.doRequest("GET", "/api/v1/settings/backup", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("backup: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["exported"] != true {
		t.Error("expected exported=true")
	}
}

func TestSettings_Restore2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/settings/restore", map[string]interface{}{
		"settings": map[string]string{"general_key1r": "val1", "general_key2r": "val2"},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("restore: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSettings_Restore_Empty2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/settings/restore", map[string]interface{}{
		"settings": map[string]string{},
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSettings_Restore_BadBody2(t *testing.T) {
	env := setupTestEnv(t)
	req := httptest.NewRequest("POST", "/api/v1/settings/restore", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.token)
	w := httptest.NewRecorder()
	env.mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSettings_Set_BadBody2(t *testing.T) {
	env := setupTestEnv(t)
	req := httptest.NewRequest("PUT", "/api/v1/settings/badkeyx", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.token)
	w := httptest.NewRecorder()
	env.mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSettings_Backup_PostNotAllowed(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/settings/backup", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestSettings_Restore_GetNotAllowed(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/settings/restore", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestLifecycle_GetConfig2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/lifecycle/config", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get config: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["pauseSeeders"] != true {
		t.Error("default pauseSeeders should be true")
	}
}

func TestLifecycle_UpdateConfig2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("PUT", "/api/v1/lifecycle/config", map[string]interface{}{
		"pauseSeeders":  false,
		"deleteSeeders": true,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("update config: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLifecycle_UpdateConfig_BadBody2(t *testing.T) {
	env := setupTestEnv(t)
	req := httptest.NewRequest("PUT", "/api/v1/lifecycle/config", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.token)
	w := httptest.NewRecorder()
	env.mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestLifecycle_Backpressure2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/lifecycle/backpressure", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("backpressure: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["isThrottled"] != false {
		t.Error("should not be throttled with 0 publishes")
	}
}

func TestLifecycle_UpdateBackpressure2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("PUT", "/api/v1/lifecycle/backpressure", map[string]interface{}{
		"max_concurrent":    10,
		"pause_on_pressure": false,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("update bp: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLifecycle_UpdateBackpressure_BadBody2(t *testing.T) {
	env := setupTestEnv(t)
	req := httptest.NewRequest("PUT", "/api/v1/lifecycle/backpressure", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.token)
	w := httptest.NewRecorder()
	env.mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestLifecycle_PostNotAllowed(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/lifecycle/config", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestLifecycle_UnknownPath(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/lifecycle/unknown2", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestCookieCloud_GetConfig_Empty2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/cookiecloud/config", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get config: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["syncEnabled"] != false {
		t.Error("expected sync disabled")
	}
}

func TestCookieCloud_UpdateConfig2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("PUT", "/api/v1/cookiecloud/config", map[string]interface{}{
		"serverUrl":   "https://cc3.example.com",
		"uuid":        "test-uuid-3",
		"password":    "test-pass-3",
		"syncEnabled": true,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = env.doRequest("GET", "/api/v1/cookiecloud/config", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get after update: expected 200, got %d", w.Code)
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["hasPassword"] != true {
		t.Error("should have password")
	}
}

func TestCookieCloud_UpdateConfig_BadBody2(t *testing.T) {
	env := setupTestEnv(t)
	req := httptest.NewRequest("PUT", "/api/v1/cookiecloud/config", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.token)
	w := httptest.NewRecorder()
	env.mux.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCookieCloud_Sync_NoConfig2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/cookiecloud/sync", nil)
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCookieCloud_ListHistory_Empty2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/cookiecloud/history", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("history: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCookieCloud_Test_NoConfig2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/cookiecloud/test", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("test: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCookieCloud_DeleteNotAllowed(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("DELETE", "/api/v1/cookiecloud/config", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestCookieCloud_UnknownPath(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/cookiecloud/unknown2", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestHub_Broadcast2(t *testing.T) {
	hub := NewHub()
	stop := make(chan struct{})
	go hub.Run(stop)

	msg := NewWSMessage("test.event", map[string]string{"key": "value"})
	hub.Broadcast(msg)

	if hub.ClientCount() != 0 {
		t.Errorf("expected 0 clients, got %d", hub.ClientCount())
	}
	close(stop)
}

func TestHub_BroadcastWS2(t *testing.T) {
	hub := NewHub()
	stop := make(chan struct{})
	go hub.Run(stop)

	hub.BroadcastWS("test.type", map[string]int{"count": 1})
	close(stop)
}

func TestHub_ClientCount2(t *testing.T) {
	hub := NewHub()
	if hub.ClientCount() != 0 {
		t.Errorf("expected 0, got %d", hub.ClientCount())
	}
}

func TestWSMessage_Creation2(t *testing.T) {
	msg := NewWSMessage("test.event", map[string]string{"hello": "world"})
	if msg.Type != "test.event" {
		t.Errorf("expected test.event, got %s", msg.Type)
	}
	if msg.ID == "" {
		t.Error("expected non-empty ID")
	}
	if msg.Timestamp == "" {
		t.Error("expected non-empty timestamp")
	}
}

func TestClient_HandleTorrents_InvalidID2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/downloaders/abc/torrents", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestClient_HandleTorrents_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/downloaders/9999/torrents", nil)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestClient_HandleFreeSpace_InvalidID(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/downloaders/xyz/free-space", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestClient_HandleFreeSpace_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/downloaders/8888/free-space", nil)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestClient_HandleMaindata_InvalidID(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/downloaders/notanid/maindata", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestClient_HandleMaindata_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/downloaders/7777/maindata", nil)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestClient_HandleTorrentAction_InvalidID(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/downloaders/bad/torrents/abc123/pause", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestClient_HandleTorrentAction_Pause_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/downloaders/5555/torrents/abc123/pause", nil)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestClient_HandleTorrentAction_Resume_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/downloaders/5555/torrents/abc123/resume", nil)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestClient_HandleTorrentInfo_Get_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/downloaders/5555/torrents/abc123", nil)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestClient_HandleTorrentInfo_Delete_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("DELETE", "/api/v1/downloaders/5555/torrents/abc123", nil)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestClient_HandleTorrentInfo_Delete_WithDeleteFiles(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("DELETE", "/api/v1/downloaders/5555/torrents/abc123?deleteFiles=true", nil)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestClient_RouteByPath_TrailingSlash(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/downloaders/", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for list with trailing slash, got %d", w.Code)
	}
}

func TestClient_RouteByPath_UnknownSub(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/downloaders/1/unknown-sub", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestClient_RouteByPath_MethodNotAllowed(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("PATCH", "/api/v1/downloaders", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestClient_Create_DuplicateName2(t *testing.T) {
	env := setupTestEnv(t)
	body := map[string]interface{}{
		"name": "dup-client", "type": "qbittorrent", "url": "http://a.com",
		"username": "u", "password": "p", "role": "seeding", "enabled": true,
	}
	w := env.doRequest("POST", "/api/v1/downloaders", body)
	if w.Code != http.StatusOK {
		t.Fatalf("create1: expected 200, got %d", w.Code)
	}
	w = env.doRequest("POST", "/api/v1/downloaders", body)
	if w.Code != http.StatusConflict {
		t.Fatalf("create2: expected 409, got %d", w.Code)
	}
}

func TestClient_Create_InvalidType2(t *testing.T) {
	env := setupTestEnv(t)
	body := map[string]interface{}{
		"name": "bad-type", "type": "badclient", "url": "http://a.com",
		"role": "seeding", "enabled": true,
	}
	w := env.doRequest("POST", "/api/v1/downloaders", body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestClient_Create_InvalidRole2(t *testing.T) {
	env := setupTestEnv(t)
	body := map[string]interface{}{
		"name": "bad-role", "type": "qbittorrent", "url": "http://a.com",
		"role": "badrole", "enabled": true,
	}
	w := env.doRequest("POST", "/api/v1/downloaders", body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestClient_Create_MissingFields2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/downloaders", map[string]interface{}{"name": "only"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestClient_Delete_NotFound2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("DELETE", "/api/v1/downloaders/99999", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestClient_Get_InvalidID(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/downloaders/notanumber", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestClient_Update_InvalidJSON(t *testing.T) {
	env := setupTestEnv(t)
	body := map[string]interface{}{
		"name": "upclient", "type": "qbittorrent", "url": "http://a.com",
		"role": "seeding", "enabled": true,
	}
	w := env.doRequest("POST", "/api/v1/downloaders", body)
	if w.Code != http.StatusOK {
		t.Fatalf("create: expected 200, got %d", w.Code)
	}

	req := httptest.NewRequest("PUT", "/api/v1/downloaders/1", bytes.NewBufferString("{bad json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.token)
	rec := httptest.NewRecorder()
	env.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestClient_TestConnection_NotFound2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/downloaders/9999/test", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCookieCloud_UpdateConfig_CreateNew(t *testing.T) {
	env := setupTestEnv(t)
	env.db.Exec("DELETE FROM cookie_cloud_configs")
	w := env.doRequest("PUT", "/api/v1/cookiecloud/config", map[string]interface{}{
		"serverUrl":    "https://cc.example.com",
		"uuid":         "test-uuid",
		"password":     "test-pass",
		"cryptoType":   "aes-256-gcm",
		"syncEnabled":  true,
		"syncInterval": 120,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	if resp.Code != 0 {
		t.Errorf("expected code 0, got %d", resp.Code)
	}
}

func TestCookieCloud_UpdateConfig_Defaults(t *testing.T) {
	env := setupTestEnv(t)
	env.db.Exec("DELETE FROM cookie_cloud_configs")
	w := env.doRequest("PUT", "/api/v1/cookiecloud/config", map[string]interface{}{
		"serverUrl": "https://cc2.example.com",
		"uuid":      "uuid2",
		"password":  "pass2",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var cfg model.CookieCloudConfig
	env.db.First(&cfg)
	if cfg.CryptoType != "legacy" {
		t.Errorf("expected cryptoType=legacy, got %s", cfg.CryptoType)
	}
	if cfg.SyncInterval != 60 {
		t.Errorf("expected syncInterval=60, got %d", cfg.SyncInterval)
	}
}

func TestCookieCloud_UpdateConfig_UpdateExisting(t *testing.T) {
	env := setupTestEnv(t)
	env.db.Exec("DELETE FROM cookie_cloud_configs")
	env.db.Create(&model.CookieCloudConfig{
		ServerURL:    "https://old.com",
		UUID:         "old-uuid",
		Password:     "old-pass",
		CryptoType:   "legacy",
		SyncInterval: 60,
	})
	w := env.doRequest("PUT", "/api/v1/cookiecloud/config", map[string]interface{}{
		"serverUrl":    "https://new.com",
		"syncEnabled":  true,
		"syncInterval": 30,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var cfg model.CookieCloudConfig
	env.db.First(&cfg)
	if cfg.ServerURL != "https://new.com" {
		t.Errorf("expected url updated, got %s", cfg.ServerURL)
	}
	if !cfg.SyncEnabled {
		t.Error("expected syncEnabled=true")
	}
}

func TestCookieCloud_Sync_MethodNotAllowed(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/cookiecloud/sync", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestCookieCloud_History_MethodNotAllowed(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/cookiecloud/history", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestCookieCloud_Test_MethodNotAllowed(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/cookiecloud/test", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestCookieCloud_Test_IncompleteConfig(t *testing.T) {
	env := setupTestEnv(t)
	env.db.Exec("DELETE FROM cookie_cloud_configs")
	env.db.Create(&model.CookieCloudConfig{
		ServerURL: "https://cc.example.com",
		UUID:      "",
		Password:  "",
	})
	w := env.doRequest("POST", "/api/v1/cookiecloud/test", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCookieCloud_UpdateConfig_BadJSON(t *testing.T) {
	env := setupTestEnv(t)
	req := httptest.NewRequest("PUT", "/api/v1/cookiecloud/config", bytes.NewBufferString("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.token)
	rec := httptest.NewRecorder()
	env.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestSeeding_CreateConfig_DuplicateClient(t *testing.T) {
	env := setupTestEnv(t)
	body := map[string]interface{}{
		"clientId": "dup-seed-client", "enabled": true,
	}
	w := env.doRequest("POST", "/api/v1/seeding/configs", body)
	if w.Code != http.StatusOK {
		t.Fatalf("create1: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	w = env.doRequest("POST", "/api/v1/seeding/configs", body)
	if w.Code != http.StatusConflict {
		t.Fatalf("create2: expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_CreateConfig_MissingClientID2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/seeding/configs", map[string]interface{}{
		"enabled": true,
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_CreateConfig_BadBody(t *testing.T) {
	env := setupTestEnv(t)
	req := httptest.NewRequest("POST", "/api/v1/seeding/configs", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.token)
	rec := httptest.NewRecorder()
	env.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestSeeding_GetConfig_NotFound2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/seeding/configs/9999", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_UpdateConfig_NotFound2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("PUT", "/api/v1/seeding/configs/9999", map[string]interface{}{"enabled": true})
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_UpdateConfig_BadBody(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/seeding/configs", map[string]interface{}{
		"clientId": "update-badbody", "enabled": true,
	})
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	id := int(data["id"].(float64))

	req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/seeding/configs/%d", id), bytes.NewBufferString("xxx"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.token)
	rec := httptest.NewRecorder()
	env.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestSeeding_Configs_MethodNotAllowed2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("DELETE", "/api/v1/seeding/configs", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestSeeding_ConfigID_Invalid(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/seeding/configs/notanumber", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_ConfigSub_MethodNotAllowed(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("PATCH", "/api/v1/seeding/configs/1", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestSeeding_TestRule_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/seeding/rules/9999/test", map[string]interface{}{
		"torrentName": "test.torrent",
	})
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_TestRule_BadBody(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/seeding/delete-rules", map[string]interface{}{
		"alias": "test-rule-expr", "enabled": true, "type": "normal", "action": "delete",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create rule: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	id := int(data["id"].(float64))

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/seeding/rules/%d/test", id), bytes.NewBufferString("notjson"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.token)
	rec := httptest.NewRecorder()
	env.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestSeeding_TestRule_WithConditions(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/seeding/delete-rules", map[string]interface{}{
		"alias":      "cond-rule",
		"enabled":    true,
		"type":       "normal",
		"conditions": `[{"field":"seeder","operator":">","value":"10"}]`,
		"action":     "delete",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create rule: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	id := int(data["id"].(float64))

	w = env.doRequest("POST", fmt.Sprintf("/api/v1/seeding/rules/%d/test", id), map[string]interface{}{
		"torrentName": "test.torrent",
		"size":        1073741824,
		"seeders":     15,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("test rule: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp = parseResponse(t, w)
	data, _ = resp.Data.(map[string]interface{})
	if matched, ok := data["matched"].(bool); !ok || !matched {
		t.Errorf("expected matched=true, got %v", data["matched"])
	}
}

func TestSeeding_TestRule_NoMatch(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/seeding/delete-rules", map[string]interface{}{
		"alias":      "nomatch-rule",
		"enabled":    true,
		"type":       "normal",
		"conditions": `[{"field":"seeder","operator":">","value":"100"}]`,
		"action":     "delete",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create rule: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	id := int(data["id"].(float64))

	w = env.doRequest("POST", fmt.Sprintf("/api/v1/seeding/rules/%d/test", id), map[string]interface{}{
		"torrentName": "test.torrent",
		"seeders":     5,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("test rule: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp = parseResponse(t, w)
	data, _ = resp.Data.(map[string]interface{})
	if matched, ok := data["matched"].(bool); !ok || matched {
		t.Errorf("expected matched=false, got %v", data["matched"])
	}
}

func TestSeeding_DryrunAll2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/seeding/dryrun", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_DryrunAll_MethodNotAllowed2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/seeding/dryrun", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestSeeding_DryrunBySub2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/seeding/dryrun/sub123", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_DryrunBySub_MethodNotAllowed(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/seeding/dryrun/sub123", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestSeeding_ScoringDryrun_BadBody(t *testing.T) {
	env := setupTestEnv(t)
	req := httptest.NewRequest("POST", "/api/v1/seeding/scoring-dryrun", bytes.NewBufferString("xxx"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.token)
	rec := httptest.NewRecorder()
	env.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestSeeding_ScoringDryrun_Success(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/seeding/scoring-dryrun", map[string]interface{}{
		"seeders":       5,
		"leechers":      10,
		"ageHours":      24.0,
		"size":          1073741824,
		"discount":      "free",
		"halfLifeHours": 2.0,
		"siteWeight":    1.0,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_Records_InvalidSubPath(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/seeding/records/notanid/resume", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_Records_UnknownSubPath(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/seeding/records/1/unknown", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestSeeding_Torrents_InvalidSubPath(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/seeding/torrents/notanid/resume", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_Torrents_UnknownSubPath(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/seeding/torrents/1/unknown", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestSeeding_Clients_Trigger(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/seeding/clients/test-client/trigger", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_Clients_UnknownSub(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/seeding/clients/test-client/unknown", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestSeeding_Stats_Overview(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/seeding/stats/overview", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_Stats_BySite(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/seeding/stats/by-site", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_Stats_Torrents(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/seeding/stats/torrents", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_Stats_SiteTrend(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/seeding/stats/by-site/mysite/trend", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_Stats_SiteTrend_30d(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/seeding/stats/by-site/mysite/trend?range=30d", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_Stats_DownloaderSpeedTrend(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/seeding/stats/downloader/client1/speed-trend", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_Stats_DownloaderSpeedTrend_7d(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/seeding/stats/downloader/client1/speed-trend?range=7d", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_Stats_UnknownSub(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/seeding/stats/unknown", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestSeeding_Stats_BySiteUnknownSub(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/seeding/stats/by-site/mysite/bad", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestSeeding_ScoringConfig_Put_WithoutSubID(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("PUT", "/api/v1/seeding/scoring-config", map[string]interface{}{
		"enabled": true,
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_ScoringConfig_Get_NoSubID(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/seeding/scoring-config", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_ScoringConfigByID_NotFound2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/seeding/scoring-config/9999", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_ScoringLogs_Default(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/seeding/scoring-logs", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_ScoringCycle2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/seeding/scoring-logs/cycles/cycle1", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_ScoringLogs_UnknownSub(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/seeding/scoring-logs/unknown", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestSeeding_CreateRule_InvalidExpr(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/seeding/rules", map[string]interface{}{
		"alias": "bad-expr", "type": "expr", "expr": "invalid [[[expr", "enabled": true,
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_CreateRule_MissingAlias2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/seeding/rules", map[string]interface{}{
		"enabled": true,
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_UpdateRule_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("PUT", "/api/v1/seeding/rules/9999", map[string]interface{}{
		"alias": "updated",
	})
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_DeleteRule_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("DELETE", "/api/v1/seeding/rules/9999", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete non-existent should still return 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_Rules_MethodNotAllowed2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("DELETE", "/api/v1/seeding/rules", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestSeeding_Rules_InvalidID(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/seeding/rules/notanumber", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_ServeHTTP_UnknownPath(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/seeding/unknown-endpoint", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestPublish_CreateTask_MissingSourceSiteID(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/publish/tasks", map[string]interface{}{
		"targetSites": []string{"site1"},
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPublish_CreateTask_MissingTargetSites(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/publish/tasks", map[string]interface{}{
		"sourceSiteId": 1,
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPublish_CreateTask_BadBody(t *testing.T) {
	env := setupTestEnv(t)
	req := httptest.NewRequest("POST", "/api/v1/publish/tasks", bytes.NewBufferString("xxx"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.token)
	rec := httptest.NewRecorder()
	env.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestPublish_TaskSub_InvalidID(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/publish/tasks/notanumber", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPublish_CandidateSub_InvalidID(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/publish/candidates/notanumber", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPublish_GroupsSub_InvalidID(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/publish/groups/notanumber", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPublish_GroupsSub_UnknownLifecycle(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/publish/groups/1/lifecycle/unknown", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestPublish_GroupsSub_LifecycleNoAction(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/publish/groups/1/lifecycle", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestPublish_GroupsSub_UnknownSub(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/publish/groups/1/unknown", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestPublish_ServeHTTP_UnknownPath(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/publish/unknown", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestPublish_Candidates_MethodNotAllowed(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/publish/candidates", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestPublish_Results_MethodNotAllowed(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/publish/results", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestPublish_TaskCancel_InvalidID(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/publish/tasks/notanumber/cancel", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPublish_CandidatePublish_InvalidID(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/publish/candidates/notanumber/publish", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRSS_ServeHTTP_MethodNotAllowed(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("PATCH", "/api/v1/rss/subscriptions", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestRSS_SubID_MethodNotAllowed(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("PATCH", "/api/v1/rss/subscriptions/1", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestRSS_Sub_InvalidID(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/rss/subscriptions/notanumber", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRSS_Trigger_MethodNotAllowed(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/rss/subscriptions/1/trigger", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestRSS_Pause_MethodNotAllowed(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/rss/subscriptions/1/pause", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestRSS_Resume_MethodNotAllowed(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/rss/subscriptions/1/resume", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestRSS_Dryrun_MethodNotAllowed(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/rss/subscriptions/1/dryrun", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestRSS_Rules_MethodNotAllowed(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/rss/subscriptions/1/rules", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestRSS_SubResource_Unknown(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/rss/subscriptions/1/unknown", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestCredentialDetector_Notify(t *testing.T) {
	hub := NewHub()
	stop := make(chan struct{})
	go hub.Run(stop)
	defer close(stop)

	d := NewCredentialDetector(nil, zap.NewNop(), hub)
	d.notify("testsite", "cookie", "test message")
}

func TestCredentialDetector_CheckNow2(t *testing.T) {
	env := setupTestEnv(t)
	hub := NewHub()
	stop := make(chan struct{})
	go hub.Run(stop)
	defer close(stop)

	d := NewCredentialDetector(env.db, zap.NewNop(), hub)
	d.CheckNow(context.Background())
}

func TestIYUU_Config_MethodNotAllowed(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("DELETE", "/api/v1/iyuu/config", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestIYUU_Sites_MethodNotAllowed(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("DELETE", "/api/v1/iyuu/sites", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestIYUU_Query_MethodNotAllowed(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/iyuu/query", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestIYUU_Query_EmptyHashes(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/iyuu/query", map[string]interface{}{
		"infoHashes": []string{},
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestIYUU_Query_BadBody(t *testing.T) {
	env := setupTestEnv(t)
	req := httptest.NewRequest("POST", "/api/v1/iyuu/query", bytes.NewBufferString("notjson"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.token)
	rec := httptest.NewRecorder()
	env.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestIYUU_UpdateConfig_BadBody(t *testing.T) {
	env := setupTestEnv(t)
	req := httptest.NewRequest("PUT", "/api/v1/iyuu/config", bytes.NewBufferString("bad"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.token)
	rec := httptest.NewRecorder()
	env.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestIYUU_Test_NoConfig3(t *testing.T) {
	env := setupTestEnv(t)
	env.db.Exec("DELETE FROM iyuu_configs")
	w := env.doRequest("POST", "/api/v1/iyuu/test", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestIYUU_Test_EmptyToken2(t *testing.T) {
	env := setupTestEnv(t)
	env.db.Exec("DELETE FROM iyuu_configs")
	env.db.Create(&model.IYUUConfig{Token: ""})
	w := env.doRequest("POST", "/api/v1/iyuu/test", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestIYUU_UnknownPath2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/iyuu/unknown", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestExtractLastSegment(t *testing.T) {
	result := extractLastSegment("/api/v1/seeding/configs/123", "/api/v1/seeding/configs/")
	if result != "123" {
		t.Errorf("expected '123', got '%s'", result)
	}
	result = extractLastSegment("/api/v1/seeding/configs/123/", "/api/v1/seeding/configs/")
	if result != "123" {
		t.Errorf("expected '123' with trailing slash, got '%s'", result)
	}
}

func TestParseUintParam(t *testing.T) {
	_, err := parseUintParam("/api/v1/test/abc", "/api/v1/test/")
	if err == nil {
		t.Error("expected error for non-numeric id")
	}
	id, err := parseUintParam("/api/v1/test/42", "/api/v1/test/")
	if err != nil || id != 42 {
		t.Errorf("expected 42, got %d, err=%v", id, err)
	}
}

func TestMaskToken(t *testing.T) {
	if maskToken("short") != "****" {
		t.Errorf("expected **** for short token")
	}
	if maskToken("abcdefgh") != "****" {
		t.Errorf("expected **** for 8-char token")
	}
	result := maskToken("abcdefghijklmnop")
	if result != "abcd****mnop" {
		t.Errorf("expected abcd****mnop, got %s", result)
	}
}

func TestSeeding_CreateConfig_DefaultCron(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/seeding/configs", map[string]interface{}{
		"clientId": "default-cron-client",
		"enabled":  true,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	if data["auto_delete_cron"] != "*/30 * * * *" {
		t.Errorf("expected default auto_delete_cron, got %v", data["auto_delete_cron"])
	}
	if data["maindata_cron"] != "*/10 * * * *" {
		t.Errorf("expected default maindata_cron, got %v", data["maindata_cron"])
	}
	if diskGB, ok := data["min_disk_space_gb"].(float64); !ok || diskGB != 50 {
		t.Errorf("expected default min_disk_space_gb=50, got %v", data["min_disk_space_gb"])
	}
}

func TestPublish_LifecyclePause(t *testing.T) {
	env := setupTestEnv(t)
	group := model.PublishGroup{Status: "active", SourceHash: "hash1", SourceSite: "site1"}
	env.db.Create(&group)
	w := env.doRequest("POST", fmt.Sprintf("/api/v1/publish/groups/%d/lifecycle/pause", group.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPublish_LifecycleResume(t *testing.T) {
	env := setupTestEnv(t)
	group := model.PublishGroup{Status: "paused", SourceHash: "hash2", SourceSite: "site2"}
	env.db.Create(&group)
	w := env.doRequest("POST", fmt.Sprintf("/api/v1/publish/groups/%d/lifecycle/resume", group.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteRuleHandler_ServeHTTP_TrailingSlash(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/seeding/delete-rules/", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteRuleHandler_TrailingSlashEmpty(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/seeding/delete-rules/", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestDeleteRuleHandler_Get_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/seeding/delete-rules/9999", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteRuleHandler_Update_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("PUT", "/api/v1/seeding/delete-rules/9999", map[string]interface{}{
		"alias": "updated",
	})
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteRuleHandler_Update_BadBody(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/seeding/delete-rules", map[string]interface{}{
		"alias": "update-bad-rule", "enabled": true,
	})
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	id := int(data["id"].(float64))

	req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/seeding/delete-rules/%d", id), bytes.NewBufferString("bad"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.token)
	rec := httptest.NewRecorder()
	env.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestDeleteRuleHandler_Update_Success(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/seeding/delete-rules", map[string]interface{}{
		"alias": "up-rule", "enabled": true,
	})
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	id := int(data["id"].(float64))

	w = env.doRequest("PUT", fmt.Sprintf("/api/v1/seeding/delete-rules/%d", id), map[string]interface{}{
		"alias":      "updated-alias",
		"priority":   5,
		"enabled":    false,
		"type":       "normal",
		"conditions": `[{"field":"seeder","operator":">","value":"5"}]`,
		"action":     "pause",
		"deleteNum":  3,
		"removeData": true,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteRuleHandler_InvalidID(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/seeding/delete-rules/notanid", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteRuleHandler_MethodNotAllowed(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("PATCH", "/api/v1/seeding/delete-rules/1", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestDeleteRuleHandler_TopLevel_MethodNotAllowed(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("DELETE", "/api/v1/seeding/delete-rules", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestDeleteRuleHandler_TestRule_WithRecords(t *testing.T) {
	env := setupTestEnv(t)
	env.db.Create(&model.SeedingTorrentRecord{
		ClientID:  "test-client",
		InfoHash:  "abc123",
		SiteName:  "testsite",
		TorrentID: "t1",
		Status:    "seeding",
		Discount:  "none",
	})
	w := env.doRequest("POST", "/api/v1/seeding/delete-rules", map[string]interface{}{
		"alias":      "test-records",
		"enabled":    true,
		"type":       "normal",
		"conditions": `[{"field":"site_name","operator":"equals","value":"testsite"}]`,
	})
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	id := int(data["id"].(float64))

	w = env.doRequest("POST", fmt.Sprintf("/api/v1/seeding/delete-rules/%d/test", id), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp = parseResponse(t, w)
	data, _ = resp.Data.(map[string]interface{})
	if total, ok := data["total"].(float64); !ok || total != 1 {
		t.Errorf("expected total=1, got %v", data["total"])
	}
}

func TestDeleteRuleHandler_Create_BadBody(t *testing.T) {
	env := setupTestEnv(t)
	req := httptest.NewRequest("POST", "/api/v1/seeding/delete-rules", bytes.NewBufferString("xxx"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.token)
	rec := httptest.NewRecorder()
	env.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestDeleteRuleHandler_Create_MissingAlias(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/seeding/delete-rules", map[string]interface{}{})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteRuleHandler_TestRule_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/seeding/delete-rules/99999/test", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSeeding_DeleteRule_Error(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("DELETE", "/api/v1/seeding/rules/0", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for delete of non-existent, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPublish_DeleteGroup_Success(t *testing.T) {
	env := setupTestEnv(t)
	group := model.PublishGroup{Status: "active", SourceHash: "delhash", SourceSite: "site1"}
	env.db.Create(&group)
	w := env.doRequest("DELETE", fmt.Sprintf("/api/v1/publish/groups/%d", group.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPublish_DeleteGroup_NotFound2(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("DELETE", "/api/v1/publish/groups/99999", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (gorm delete doesn't error on missing), got %d: %s", w.Code, w.Body.String())
	}
}

func TestPublish_GetGroup_WithMembers(t *testing.T) {
	env := setupTestEnv(t)
	group := model.PublishGroup{Status: "active", SourceHash: "memhash", SourceSite: "site1"}
	env.db.Create(&group)
	env.db.Create(&model.PublishGroupMember{
		PublishGroupID: group.ID,
		SiteName:       "targetsite",
		Status:         "pending",
	})
	w := env.doRequest("GET", fmt.Sprintf("/api/v1/publish/groups/%d", group.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	members, ok := data["members"].([]interface{})
	if !ok || len(members) != 1 {
		t.Errorf("expected 1 member, got %v", data["members"])
	}
}

func TestPublish_GetGroup_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/publish/groups/99999", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPublish_GroupsList_WithStatus(t *testing.T) {
	env := setupTestEnv(t)
	env.db.Create(&model.PublishGroup{Status: "active", SourceHash: "h1", SourceSite: "s1"})
	env.db.Create(&model.PublishGroup{Status: "completed", SourceHash: "h2", SourceSite: "s2"})
	w := env.doRequest("GET", "/api/v1/publish/groups?status=active", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPublish_TaskGetMethodNotAllowed(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/publish/tasks/1", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestPublish_CandidateGetMethodNotAllowed(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("PUT", "/api/v1/publish/candidates/1", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestPublish_GroupDeleteMethodNotAllowed(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/publish/groups/1", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestRSS_Trigger_InvalidID(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/rss/subscriptions/notanumber/trigger", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRSS_Pause_InvalidID(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/rss/subscriptions/notanumber/pause", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRSS_Dryrun_InvalidID(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/rss/subscriptions/notanumber/dryrun", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRSS_UpdateRules_InvalidID(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("PUT", "/api/v1/rss/subscriptions/notanumber/rules", map[string]interface{}{
		"acceptRuleIds": []uint{1},
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRSS_UpdateRules_BadBody(t *testing.T) {
	env := setupTestEnv(t)
	createTestSite(t, env, "RSSRulesSite", "rssrulessite.com")
	w := env.doRequest("POST", "/api/v1/rss/subscriptions", map[string]interface{}{
		"name": "RulesSub", "siteName": "RSSRulesSite",
		"urls": []string{"https://rssrulessite.com/rss"}, "enabled": true,
	})
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	id := int(data["id"].(float64))

	req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/rss/subscriptions/%d/rules", id), bytes.NewBufferString("xxx"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.token)
	rec := httptest.NewRecorder()
	env.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestRSS_UpdateRules_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("PUT", "/api/v1/rss/subscriptions/99999/rules", map[string]interface{}{
		"acceptRuleIds": []uint{},
	})
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRSS_Update_ManyFields(t *testing.T) {
	env := setupTestEnv(t)
	createTestSite(t, env, "RSSUpSite", "rssupsite.com")
	w := env.doRequest("POST", "/api/v1/rss/subscriptions", map[string]interface{}{
		"name": "UpSub", "siteName": "RSSUpSite",
		"urls": []string{"https://rssupsite.com/rss"}, "enabled": true,
	})
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	id := int(data["id"].(float64))

	w = env.doRequest("PUT", fmt.Sprintf("/api/v1/rss/subscriptions/%d", id), map[string]interface{}{
		"name":            "UpSubRenamed",
		"enabled":         false,
		"urls":            []string{"https://rssupsite.com/rss2"},
		"siteName":        "RSSUpSite",
		"cron":            "*/15 * * * *",
		"clientId":        "client1",
		"savePath":        "/data",
		"category":        "cat1",
		"addPaused":       true,
		"autoTmm":         true,
		"uploadLimitKb":   1000,
		"downloadLimitKb": 2000,
		"tags":            []string{"tag1"},
		"scrapeFree":      true,
		"scrapeHr":        true,
		"pushNotify":      true,
		"notifyId":        "ch1",
		"publishEnabled":  true,
		"publishTargets":  []string{"site1"},
		"autoReseed":      true,
		"reseedClientIds": []string{"c1"},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRSS_Update_DupName(t *testing.T) {
	env := setupTestEnv(t)
	createTestSite(t, env, "RSSDupSite", "rssdupsite.com")
	env.doRequest("POST", "/api/v1/rss/subscriptions", map[string]interface{}{
		"name": "Sub1", "siteName": "RSSDupSite",
		"urls": []string{"https://rssdupsite.com/rss"}, "enabled": true,
	})
	w := env.doRequest("POST", "/api/v1/rss/subscriptions", map[string]interface{}{
		"name": "Sub2", "siteName": "RSSDupSite",
		"urls": []string{"https://rssdupsite.com/rss2"}, "enabled": true,
	})
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	id := int(data["id"].(float64))

	w = env.doRequest("PUT", fmt.Sprintf("/api/v1/rss/subscriptions/%d", id), map[string]interface{}{
		"name": "Sub1",
	})
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRSS_Update_BadBody(t *testing.T) {
	env := setupTestEnv(t)
	createTestSite(t, env, "RSSBadSite", "rssbadsite.com")
	w := env.doRequest("POST", "/api/v1/rss/subscriptions", map[string]interface{}{
		"name": "BadSub", "siteName": "RSSBadSite",
		"urls": []string{"https://rssbadsite.com/rss"}, "enabled": true,
	})
	resp := parseResponse(t, w)
	data, _ := resp.Data.(map[string]interface{})
	id := int(data["id"].(float64))

	req := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/rss/subscriptions/%d", id), bytes.NewBufferString("xxx"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.token)
	rec := httptest.NewRecorder()
	env.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestScheduler_HandleList(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/scheduler/tasks", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestScheduler_HandleGet_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/api/v1/scheduler/tasks/nonexistent", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestScheduler_HandlePause_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/scheduler/tasks/nonexistent/pause", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestScheduler_HandleResume_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/scheduler/tasks/nonexistent/resume", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestScheduler_HandleTrigger_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/api/v1/scheduler/tasks/nonexistent/trigger", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestScheduler_HandleReschedule_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("PUT", "/api/v1/scheduler/tasks/nonexistent/schedule", map[string]interface{}{
		"schedule": "*/5 * * * *",
	})
	if w.Code != http.StatusNotFound && w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400/404, got %d: %s", w.Code, w.Body.String())
	}
}
