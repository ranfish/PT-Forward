package health

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthChecker_Handler(t *testing.T) {
	h := NewHealthChecker("test-v1")

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	h.Handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected application/json, got %s", ct)
	}

	var resp HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("expected status=ok, got %s", resp.Status)
	}
	if resp.Version != "test-v1" {
		t.Errorf("expected version=test-v1, got %s", resp.Version)
	}
	if resp.Uptime == "" {
		t.Error("expected non-empty uptime")
	}
}

func TestNewHealthChecker(t *testing.T) {
	h := NewHealthChecker("1.0.0")
	if h == nil {
		t.Fatal("expected non-nil HealthChecker")
	}
	if h.version != "1.0.0" {
		t.Errorf("expected version=1.0.0, got %s", h.version)
	}
}
