package notification

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.NotificationChannel{}, &model.NotificationHistory{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func newTestService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db := setupTestDB(t)
	logger := zap.NewNop()
	return NewService(db, logger), db
}

func TestRepository_CRUD(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	ch := &model.NotificationChannel{
		Type:    "telegram",
		Name:    "test-bot",
		Enabled: true,
		Config:  `{"token":"abc","chat_id":"123"}`,
		Events:  "rss,publish",
		Healthy: true,
	}
	if err := repo.Create(ctx, ch); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if ch.ID == 0 {
		t.Fatal("ID should be set after create")
	}

	got, err := repo.GetByID(ctx, ch.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "test-bot" {
		t.Errorf("expected name test-bot, got %s", got.Name)
	}

	channels, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(channels) != 1 {
		t.Errorf("expected 1 channel, got %d", len(channels))
	}

	exists, err := repo.ExistsByName(ctx, "test-bot", 0)
	if err != nil {
		t.Fatalf("ExistsByName: %v", err)
	}
	if !exists {
		t.Error("should exist")
	}
	exists, _ = repo.ExistsByName(ctx, "test-bot", ch.ID)
	if exists {
		t.Error("should not exist when excluding own ID")
	}

	got.Name = "updated"
	if err := repo.Update(ctx, got); err != nil {
		t.Fatalf("Update: %v", err)
	}
	updated, _ := repo.GetByID(ctx, ch.ID)
	if updated.Name != "updated" {
		t.Errorf("expected updated, got %s", updated.Name)
	}

	if err := repo.Delete(ctx, ch.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err = repo.GetByID(ctx, ch.ID)
	if err == nil {
		t.Error("should be deleted")
	}
}

func TestRepository_ListHistory(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	ch := &model.NotificationChannel{Type: "webhook", Name: "wh", Enabled: true, Healthy: true}
	if err := repo.Create(ctx, ch); err != nil {
		t.Fatalf("Create: %v", err)
	}

	for i := 0; i < 5; i++ {
		db.Create(&model.NotificationHistory{
			ChannelID: ch.ID,
			Event:     "rss",
			Level:     "info",
			Title:     "test",
			Success:   true,
		})
	}

	history, err := repo.ListHistory(ctx, ch.ID, 3)
	if err != nil {
		t.Fatalf("ListHistory: %v", err)
	}
	if len(history) != 3 {
		t.Errorf("expected 3, got %d", len(history))
	}

	all, _ := repo.ListHistory(ctx, 0, 0)
	if len(all) != 5 {
		t.Errorf("expected 5 for all channels, got %d", len(all))
	}
}

func TestRepository_CleanupOldHistory(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	old := model.NotificationHistory{
		ChannelID: 1, Event: "rss", Level: "info", Title: "old",
		CreatedAt: time.Now().AddDate(0, 0, -60),
	}
	db.Create(&old)

	recent := model.NotificationHistory{
		ChannelID: 1, Event: "rss", Level: "info", Title: "recent",
		CreatedAt: time.Now(),
	}
	db.Create(&recent)

	if err := repo.CleanupOldHistory(ctx, 30); err != nil {
		t.Fatalf("CleanupOldHistory: %v", err)
	}

	var count int64
	db.Model(&model.NotificationHistory{}).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 remaining, got %d", count)
	}
}

func TestService_Send_NoChannels(t *testing.T) {
	svc, _ := newTestService(t)
	err := svc.Send(context.Background(), model.FormattedMessage{
		Title: "test", Message: "hello", Level: "info",
	})
	if err != nil {
		t.Errorf("Send with no channels should return nil, got %v", err)
	}
}

func TestService_Send_WithMockServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	svc, db := newTestService(t)
	ctx := context.Background()

	ch := &model.NotificationChannel{
		Type:    "webhook",
		Name:    "test-wh",
		Enabled: true,
		Healthy: true,
		Config:  `{"url":"` + server.URL + `"}`,
		Events:  "*",
	}
	db.Create(ch)

	err := svc.Send(ctx, model.FormattedMessage{
		Title: "test", Message: "hello", Level: "info",
	})
	if err != nil {
		t.Errorf("Send failed: %v", err)
	}

	var history []model.NotificationHistory
	db.Find(&history)
	if len(history) != 1 {
		t.Errorf("expected 1 history record, got %d", len(history))
	}
	if !history[0].Success {
		t.Error("history should be success")
	}
}

func TestService_Send_FailureIncrements(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()

	ch := &model.NotificationChannel{
		Type:             "webhook",
		Name:             "failing",
		Enabled:          true,
		Healthy:          true,
		Config:           `{"url":"http://127.0.0.1:1/fail"}`,
		Events:           "*",
		MaxErrorsPerHour: 3,
	}
	db.Create(ch)

	err := svc.Send(ctx, model.FormattedMessage{Title: "t", Message: "m", Level: "info"})
	if err == nil {
		t.Error("expected error from failed send")
	}

	var updated model.NotificationChannel
	db.First(&updated, ch.ID)
	if updated.ConsecutiveFailures != 1 {
		t.Errorf("expected 1 failure, got %d", updated.ConsecutiveFailures)
	}
}

func TestService_Dispatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	svc, db := newTestService(t)
	ctx := context.Background()

	ch := &model.NotificationChannel{
		Type:    "webhook",
		Name:    "disp",
		Enabled: true,
		Healthy: true,
		Config:  `{"url":"` + server.URL + `"}`,
		Events:  "rss",
	}
	db.Create(ch)

	err := svc.Dispatch(ctx, "rss", model.FormattedMessage{Title: "t", Message: "m", Level: "info"})
	if err != nil {
		t.Errorf("Dispatch failed: %v", err)
	}

	err = svc.Dispatch(ctx, "publish", model.FormattedMessage{Title: "t", Message: "m", Level: "info"})
	if err != nil {
		t.Error("Dispatch with non-matching event should not error, just skip")
	}
}

func TestService_Dispatch_UnhealthySkipped(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()

	ch := &model.NotificationChannel{
		Type:    "webhook",
		Name:    "unhealthy",
		Enabled: true,
		Healthy: true,
		Config:  `{"url":"http://127.0.0.1:1"}`,
		Events:  "*",
	}
	db.Create(ch)
	db.Model(ch).Update("healthy", false)

	err := svc.Dispatch(ctx, "rss", model.FormattedMessage{Title: "t", Message: "m", Level: "info"})
	if err != nil {
		t.Errorf("should skip unhealthy without error, got %v", err)
	}
}

func TestService_ResetFailures(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()

	ch := &model.NotificationChannel{
		Type:                "webhook",
		Name:                "reset",
		Enabled:             true,
		Healthy:             false,
		ConsecutiveFailures: 5,
		Config:              `{}`,
	}
	db.Create(ch)

	svc.resetFailures(ctx, ch)

	var updated model.NotificationChannel
	db.First(&updated, ch.ID)
	if updated.ConsecutiveFailures != 0 {
		t.Errorf("expected 0 failures, got %d", updated.ConsecutiveFailures)
	}
	if !updated.Healthy {
		t.Error("should be healthy again")
	}
}

func TestService_IncrementFailures_MarksUnhealthy(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()

	ch := &model.NotificationChannel{
		Type:             "webhook",
		Name:             "inc",
		Enabled:          true,
		Healthy:          true,
		MaxErrorsPerHour: 2,
		Config:           `{}`,
	}
	db.Create(ch)

	svc.incrementFailures(ctx, ch)
	svc.incrementFailures(ctx, ch)

	var updated model.NotificationChannel
	db.First(&updated, ch.ID)
	if updated.ConsecutiveFailures != 2 {
		t.Errorf("expected 2 failures, got %d", updated.ConsecutiveFailures)
	}
	if updated.Healthy {
		t.Error("should be unhealthy after reaching max errors")
	}
}

func TestService_SendTelegram_MissingConfig(t *testing.T) {
	svc := &Service{}
	ch := &model.NotificationChannel{Type: "telegram", Config: `{}`}
	ok, msg := svc.sendTelegram(context.Background(), ch, model.FormattedMessage{Title: "t", Message: "m"})
	if ok {
		t.Error("should fail without token/chat_id")
	}
	if msg != "缺少 token 或 chat_id" {
		t.Errorf("unexpected error: %s", msg)
	}
}

func TestService_SendBark_MissingConfig(t *testing.T) {
	svc := &Service{}
	ch := &model.NotificationChannel{Type: "bark", Config: `{}`}
	ok, _ := svc.sendBark(context.Background(), ch, model.FormattedMessage{Title: "t", Message: "m"})
	if ok {
		t.Error("should fail without url/device_key")
	}
}

func TestService_SendServerChan_MissingConfig(t *testing.T) {
	svc := &Service{}
	ch := &model.NotificationChannel{Type: "serverchan", Config: `{}`}
	ok, _ := svc.sendServerChan(context.Background(), ch, model.FormattedMessage{})
	if ok {
		t.Error("should fail without sendkey")
	}
}

func TestService_SendDingTalk_MissingConfig(t *testing.T) {
	svc := &Service{}
	ch := &model.NotificationChannel{Type: "dingtalk", Config: `{}`}
	ok, _ := svc.sendDingTalk(context.Background(), ch, model.FormattedMessage{})
	if ok {
		t.Error("should fail without webhook")
	}
}

func TestService_SendWebhook_MissingConfig(t *testing.T) {
	svc := &Service{}
	ch := &model.NotificationChannel{Type: "webhook", Config: `{}`}
	ok, _ := svc.sendWebhook(context.Background(), ch, model.FormattedMessage{})
	if ok {
		t.Error("should fail without url")
	}
}

func TestService_SendToChannel_UnsupportedType(t *testing.T) {
	svc := &Service{}
	ch := &model.NotificationChannel{Type: "unknown", Config: `{}`}
	ok, msg := svc.sendToChannel(context.Background(), ch, model.FormattedMessage{})
	if ok {
		t.Error("should fail for unsupported type")
	}
	if msg == "" {
		t.Error("should have error message")
	}
}

func TestService_InQuietHours(t *testing.T) {
	svc := &Service{}
	ch := &model.NotificationChannel{QuietHoursStart: "", QuietHoursEnd: ""}
	if svc.inQuietHours(ch) {
		t.Error("no quiet hours set should return false")
	}
}

func TestService_RecordHistory(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()

	svc.recordHistory(ctx, 1, model.FormattedMessage{
		Title: "test", Message: "body", Level: "info",
	}, true, "")

	var h model.NotificationHistory
	db.First(&h)
	if h.Title != "test" || !h.Success {
		t.Errorf("unexpected history: %+v", h)
	}
}

func TestTestService_SendTest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ts := NewTestService(&model.NotificationChannel{
		Type:   "webhook",
		Config: `{"url":"` + server.URL + `"}`,
	}, zap.NewNop())

	ok, msg := ts.SendTest(context.Background(), model.FormattedMessage{Title: "t", Message: "m"})
	if !ok {
		t.Errorf("expected success, got: %s", msg)
	}
}

func TestCalcDingTalkSign(t *testing.T) {
	sign := calcDingTalkSign(1234567890000, "test-secret")
	if sign == "" {
		t.Error("sign should not be empty")
	}
}

func TestParseConfig_Empty(t *testing.T) {
	cfg := parseConfig("")
	if len(cfg) != 0 {
		t.Errorf("expected empty map, got %v", cfg)
	}
}

func TestParseConfig_ValidJSON(t *testing.T) {
	cfg := parseConfig(`{"token":"abc","chat_id":"123"}`)
	if cfg["token"] != "abc" {
		t.Errorf("expected token=abc, got %s", cfg["token"])
	}
	if cfg["chat_id"] != "123" {
		t.Errorf("expected chat_id=123, got %s", cfg["chat_id"])
	}
}

func TestParseConfig_InvalidJSON(t *testing.T) {
	cfg := parseConfig("not json")
	if len(cfg) != 0 {
		t.Errorf("expected empty map for invalid JSON, got %v", cfg)
	}
}

func TestEscapeHTML(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{"hello", "hello"},
		{"<b>bold</b>", "&lt;b&gt;bold&lt;/b&gt;"},
		{"a&b", "a&amp;b"},
		{"a>b<c&d", "a&gt;b&lt;c&amp;d"},
	}

	for _, tt := range tests {
		got := escapeHTML(tt.input)
		if got != tt.expect {
			t.Errorf("escapeHTML(%q) = %q, want %q", tt.input, got, tt.expect)
		}
	}
}

func TestMatchEvent(t *testing.T) {
	svc := &Service{}
	ch := &model.NotificationChannel{Events: "rss,publish"}

	if !svc.matchEvent(ch, "rss") {
		t.Error("should match rss")
	}
	if !svc.matchEvent(ch, "publish") {
		t.Error("should match publish")
	}
	if svc.matchEvent(ch, "system") {
		t.Error("should not match system")
	}
}

func TestMatchEvent_Wildcard(t *testing.T) {
	svc := &Service{}
	ch := &model.NotificationChannel{Events: "*"}
	if !svc.matchEvent(ch, "anything") {
		t.Error("wildcard should match everything")
	}
}

func TestMatchEvent_Empty(t *testing.T) {
	svc := &Service{}
	ch := &model.NotificationChannel{Events: ""}
	if !svc.matchEvent(ch, "anything") {
		t.Error("empty events should match everything")
	}
}

func TestService_Test(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, zap.NewNop())

	ch := &model.NotificationChannel{
		Name:    "test-ch",
		Type:    "webhook",
		Events:  "*",
		Config:  `{"url":"http://127.0.0.1:1/nonexistent"}`,
		Enabled: true,
	}
	db.Create(ch)

	err := svc.Test(context.Background())
	if err == nil {
		t.Error("expected error with no real endpoint")
	}
}

func TestService_DoHTTPGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	svc := &Service{client: srv.Client()}
	ok, errMsg := svc.doHTTPGet(context.Background(), srv.URL+"/test")
	if !ok {
		t.Errorf("expected success, got error: %s", errMsg)
	}
}

func TestService_DoHTTPGet_Error(t *testing.T) {
	svc := &Service{client: &http.Client{Timeout: time.Millisecond}}
	ok, _ := svc.doHTTPGet(context.Background(), "http://127.0.0.1:1/fail")
	if ok {
		t.Error("expected failure for unreachable URL")
	}
}

func TestService_DoHTTPGet_Status500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	svc := &Service{client: srv.Client()}
	ok, errMsg := svc.doHTTPGet(context.Background(), srv.URL+"/fail")
	if ok {
		t.Error("expected failure for 500")
	}
	if errMsg == "" {
		t.Error("expected error message")
	}
}

func TestRepository_CleanupOldHistory_DefaultDays(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	old := model.NotificationHistory{
		ChannelID: 1, Event: "rss", Level: "info", Title: "old",
		CreatedAt: time.Now().AddDate(0, 0, -60),
	}
	db.Create(&old)

	recent := model.NotificationHistory{
		ChannelID: 1, Event: "rss", Level: "info", Title: "recent",
		CreatedAt: time.Now(),
	}
	db.Create(&recent)

	if err := repo.CleanupOldHistory(ctx, 0); err != nil {
		t.Fatalf("CleanupOldHistory: %v", err)
	}

	var count int64
	db.Model(&model.NotificationHistory{}).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 remaining, got %d", count)
	}
}

func TestService_SendTelegram_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	svc := &Service{client: &http.Client{
		Timeout:   15 * time.Second,
		Transport: &redirectTransport{url: srv.URL + "/telegram"},
	}}
	ch := &model.NotificationChannel{
		Type:   "telegram",
		Config: `{"token":"fake-token","chat_id":"123"}`,
	}
	ok, errMsg := svc.sendTelegram(context.Background(), ch, model.FormattedMessage{Title: "<hello>", Message: "a&b"})
	if !ok {
		t.Errorf("expected success, got: %s", errMsg)
	}
}

func TestService_SendBark_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	svc := &Service{client: &http.Client{
		Timeout:   15 * time.Second,
		Transport: &redirectTransport{url: srv.URL + "/bark"},
	}}
	ch := &model.NotificationChannel{
		Type:   "bark",
		Config: fmt.Sprintf(`{"url":"%s","device_key":"abc123"}`, srv.URL),
	}
	ok, errMsg := svc.sendBark(context.Background(), ch, model.FormattedMessage{Title: "test", Message: "msg"})
	if !ok {
		t.Errorf("expected success, got: %s", errMsg)
	}
}

func TestService_SendServerChan_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	svc := &Service{client: &http.Client{
		Timeout:   15 * time.Second,
		Transport: &redirectTransport{url: srv.URL + "/schan"},
	}}
	ch := &model.NotificationChannel{
		Type:   "serverchan",
		Config: `{"sendkey":"testkey"}`,
	}
	ok, errMsg := svc.sendServerChan(context.Background(), ch, model.FormattedMessage{Title: "t", Message: "m"})
	if !ok {
		t.Errorf("expected success, got: %s", errMsg)
	}
}

func TestService_SendDingTalk_WithSecret(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	svc := &Service{client: &http.Client{
		Timeout:   15 * time.Second,
		Transport: &redirectTransport{url: srv.URL + "/ding"},
	}}
	ch := &model.NotificationChannel{
		Type:   "dingtalk",
		Config: `{"webhook":"http://hook.example.com/api/send?","secret":"my-secret"}`,
	}
	ok, errMsg := svc.sendDingTalk(context.Background(), ch, model.FormattedMessage{Title: "t", Message: "m"})
	if !ok {
		t.Errorf("expected success, got: %s", errMsg)
	}
}

func TestService_SendDingTalk_NoSecret(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	svc := &Service{client: &http.Client{
		Timeout:   15 * time.Second,
		Transport: &redirectTransport{url: srv.URL + "/ding"},
	}}
	ch := &model.NotificationChannel{
		Type:   "dingtalk",
		Config: `{"webhook":"http://hook.example.com"}`,
	}
	ok, errMsg := svc.sendDingTalk(context.Background(), ch, model.FormattedMessage{Title: "t", Message: "m"})
	if !ok {
		t.Errorf("expected success, got: %s", errMsg)
	}
}

func TestService_Send_Failover(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userAgent := r.Header.Get("User-Agent")
		if userAgent == "" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if r.URL.Path == "/primary" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	svc, db := newTestService(t)
	ctx := context.Background()

	primary := &model.NotificationChannel{
		Type:             "webhook",
		Name:             "primary",
		Enabled:          true,
		Healthy:          true,
		Config:           fmt.Sprintf(`{"url":"%s/primary"}`, srv.URL),
		Events:           "*",
		FailoverGroupID:  "group1",
		MaxErrorsPerHour: 10,
	}
	db.Create(primary)

	fallback := &model.NotificationChannel{
		Type:            "webhook",
		Name:            "fallback",
		Enabled:         true,
		Healthy:         true,
		Config:          fmt.Sprintf(`{"url":"%s/fallback"}`, srv.URL),
		Events:          "*",
		FailoverGroupID: "group1",
	}
	db.Create(fallback)

	svc.client = srv.Client()

	err := svc.Send(ctx, model.FormattedMessage{Title: "t", Message: "m", Level: "info"})
	if err == nil {
		t.Error("expected error because primary fails")
	}

	var history []model.NotificationHistory
	db.Find(&history)
	if len(history) < 2 {
		t.Errorf("expected at least 2 history records, got %d", len(history))
	}
}

func TestService_DoHTTPPost_StatusError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad request"))
	}))
	defer srv.Close()

	svc := &Service{client: srv.Client()}
	ok, errMsg := svc.doHTTPPost(context.Background(), srv.URL+"/test", "application/json", "{}")
	if ok {
		t.Error("expected failure for 400")
	}
	if errMsg == "" {
		t.Error("expected error message")
	}
}

func TestService_DoHTTPPost_InvalidURL(t *testing.T) {
	svc := &Service{client: &http.Client{}}
	ok, _ := svc.doHTTPPost(context.Background(), "http://127.0.0.1:1/fail", "application/json", "{}")
	if ok {
		t.Error("expected failure for unreachable URL")
	}
}

func TestService_RecordHistory_DBError(t *testing.T) {
	db := setupTestDB(t)
	svc := &Service{db: db, logger: zap.NewNop()}
	svc.recordHistory(context.Background(), 999, model.FormattedMessage{Title: "t", Message: "m", Level: "info"}, false, "some error")

	var history []model.NotificationHistory
	db.Find(&history)
	if len(history) != 1 {
		t.Fatalf("expected 1 history record, got %d", len(history))
	}
	if history[0].Success {
		t.Error("expected success=false for failed notification")
	}
	if history[0].ChannelID != 999 {
		t.Errorf("expected channel_id=999, got %d", history[0].ChannelID)
	}
	if history[0].ErrorMsg != "some error" {
		t.Errorf("expected error_msg='some error', got %q", history[0].ErrorMsg)
	}
}

func TestService_InQuietHours_Active(t *testing.T) {
	svc := &Service{}
	ch := &model.NotificationChannel{
		QuietHoursStart: "00:00",
		QuietHoursEnd:   "23:59",
	}
	if !svc.inQuietHours(ch) {
		t.Error("should be in quiet hours")
	}
}

func TestService_InQuietHours_NotActive(t *testing.T) {
	svc := &Service{}
	ch := &model.NotificationChannel{
		QuietHoursStart: "03:00",
		QuietHoursEnd:   "04:00",
	}
	if svc.inQuietHours(ch) {
		t.Error("should not be in quiet hours when start > end of same-day range")
	}
}

func TestService_Dispatch_Failure(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()

	ch := &model.NotificationChannel{
		Type:    "webhook",
		Name:    "disp-fail",
		Enabled: true,
		Healthy: true,
		Config:  `{"url":"http://127.0.0.1:1/fail"}`,
		Events:  "*",
	}
	db.Create(ch)

	err := svc.Dispatch(ctx, "rss", model.FormattedMessage{Title: "t", Message: "m", Level: "info"})
	if err == nil {
		t.Error("expected error from failed dispatch")
	}
}

func TestService_Dispatch_QuietHours(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()

	ch := &model.NotificationChannel{
		Type:            "webhook",
		Name:            "quiet",
		Enabled:         true,
		Healthy:         true,
		Config:          `{"url":"http://127.0.0.1:1"}`,
		Events:          "*",
		QuietHoursStart: "00:00",
		QuietHoursEnd:   "23:59",
	}
	db.Create(ch)

	err := svc.Dispatch(ctx, "rss", model.FormattedMessage{Title: "t", Message: "m", Level: "info"})
	if err != nil {
		t.Errorf("quiet hours should skip without error, got %v", err)
	}
}

func TestService_Send_QuietHoursSkips(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()

	ch := &model.NotificationChannel{
		Type:            "webhook",
		Name:            "quiet-ch",
		Enabled:         true,
		Healthy:         true,
		Config:          `{"url":"http://127.0.0.1:1"}`,
		Events:          "*",
		QuietHoursStart: "00:00",
		QuietHoursEnd:   "23:59",
	}
	db.Create(ch)

	err := svc.Send(ctx, model.FormattedMessage{Title: "t", Message: "m", Level: "info"})
	if err != nil {
		t.Errorf("quiet hours should skip without error, got %v", err)
	}
}

func TestService_Send_EventNotMatch(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()

	ch := &model.NotificationChannel{
		Type:    "webhook",
		Name:    "event-filter",
		Enabled: true,
		Healthy: true,
		Config:  `{"url":"http://127.0.0.1:1"}`,
		Events:  "rss",
	}
	db.Create(ch)

	err := svc.Send(ctx, model.FormattedMessage{Title: "t", Message: "m", Level: "warning"})
	if err != nil {
		t.Errorf("non-matching event should skip without error, got %v", err)
	}
}

func TestTestService_SendTest_Failure(t *testing.T) {
	ts := NewTestService(&model.NotificationChannel{
		Type:   "webhook",
		Config: `{}`,
	}, zap.NewNop())

	ok, msg := ts.SendTest(context.Background(), model.FormattedMessage{Title: "t", Message: "m"})
	if ok {
		t.Error("expected failure for missing url")
	}
	if msg == "" {
		t.Error("expected error message")
	}
}

func TestService_SendToChannel_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	svc := &Service{client: srv.Client()}
	ch := &model.NotificationChannel{
		Type:      "webhook",
		Config:    fmt.Sprintf(`{"url":"%s"}`, srv.URL),
		TimeoutMs: 5000,
	}
	ok, errMsg := svc.sendToChannel(context.Background(), ch, model.FormattedMessage{Title: "t", Message: "m"})
	if !ok {
		t.Errorf("expected success with custom timeout, got: %s", errMsg)
	}
}

func TestService_SendToChannel_Telegram(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	svc := &Service{client: &http.Client{
		Timeout:   15 * time.Second,
		Transport: &redirectTransport{url: srv.URL + "/tg"},
	}}
	ch := &model.NotificationChannel{
		Type:   "telegram",
		Config: `{"token":"fake","chat_id":"123"}`,
	}
	ok, _ := svc.sendToChannel(context.Background(), ch, model.FormattedMessage{Title: "t", Message: "m"})
	if !ok {
		t.Error("expected success for telegram")
	}
}

func TestService_SendToChannel_Bark(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	svc := &Service{client: &http.Client{
		Timeout:   15 * time.Second,
		Transport: &redirectTransport{url: srv.URL + "/bark"},
	}}
	ch := &model.NotificationChannel{
		Type:   "bark",
		Config: fmt.Sprintf(`{"url":"%s","device_key":"abc"}`, srv.URL),
	}
	ok, _ := svc.sendToChannel(context.Background(), ch, model.FormattedMessage{Title: "t", Message: "m"})
	if !ok {
		t.Error("expected success for bark")
	}
}

func TestService_SendToChannel_ServerChan(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	svc := &Service{client: &http.Client{
		Timeout:   15 * time.Second,
		Transport: &redirectTransport{url: srv.URL + "/sc"},
	}}
	ch := &model.NotificationChannel{
		Type:   "serverchan",
		Config: `{"sendkey":"testkey"}`,
	}
	ok, _ := svc.sendToChannel(context.Background(), ch, model.FormattedMessage{Title: "t", Message: "m"})
	if !ok {
		t.Error("expected success for serverchan")
	}
}

func TestService_SendToChannel_DingTalk(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	svc := &Service{client: &http.Client{
		Timeout:   15 * time.Second,
		Transport: &redirectTransport{url: srv.URL + "/dt"},
	}}
	ch := &model.NotificationChannel{
		Type:   "dingtalk",
		Config: `{"webhook":"http://hook.example.com/api/send?","secret":"s"}`,
	}
	ok, _ := svc.sendToChannel(context.Background(), ch, model.FormattedMessage{Title: "t", Message: "m"})
	if !ok {
		t.Error("expected success for dingtalk")
	}
}

type redirectTransport struct {
	url string
}

func (rt *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	newReq := req.Clone(req.Context())
	newReq.URL, _ = url.Parse(rt.url)
	return http.DefaultTransport.RoundTrip(newReq)
}
