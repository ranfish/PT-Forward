package health

import (
	"encoding/json"
	"errors"
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

type mockPinger struct {
	err error
}

func (m *mockPinger) Ping() error { return m.err }

func TestHealthChecker_WithDBPinger_Healthy(t *testing.T) {
	h := NewHealthChecker("test-v2")
	h.SetDBPinger(&mockPinger{err: nil})

	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	h.Handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("expected status=ok, got %s", resp.Status)
	}
	if resp.Components["database"].Status != "healthy" {
		t.Errorf("expected database=healthy, got %s", resp.Components["database"].Status)
	}
}

func TestHealthChecker_WithDBPinger_Unhealthy(t *testing.T) {
	h := NewHealthChecker("test-v2")
	h.SetDBPinger(&mockPinger{err: errors.New("connection refused")})

	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	h.Handler(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}

	var resp HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Status != "degraded" {
		t.Errorf("expected status=degraded, got %s", resp.Status)
	}
	db := resp.Components["database"]
	if db.Status != "unhealthy" {
		t.Errorf("expected database status=unhealthy, got %s", db.Status)
	}
	if db.Error != "connection refused" {
		t.Errorf("expected database error='connection refused', got %s", db.Error)
	}
}

func TestHealthChecker_NoDBPinger_OmitsComponents(t *testing.T) {
	h := NewHealthChecker("test-v3")

	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	h.Handler(w, req)

	var resp HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Components != nil {
		t.Errorf("expected no components when no DB pinger set, got %v", resp.Components)
	}
}
