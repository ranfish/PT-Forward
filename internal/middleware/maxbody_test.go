package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMaxBodySize_AllowsNormal(t *testing.T) {
	handler := MaxBodySize(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Q string `json:"query"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	body := `{"query":"test"}`
	req := httptest.NewRequest("POST", "/api/v1/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for normal body, got %d", w.Code)
	}
}

func TestMaxBodySize_RejectsOversized(t *testing.T) {
	handler := MaxBodySize(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	oversized := `{"data": "` + strings.Repeat("x", 2*1024*1024) + `"}`
	req := httptest.NewRequest("POST", "/api/v1/test", strings.NewReader(oversized))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		t.Errorf("expected non-200 for oversized body, got %d", w.Code)
	}
}

func TestMaxBodySize_NilBody(t *testing.T) {
	handler := MaxBodySize(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for nil body, got %d", w.Code)
	}
}

func TestMaxBodySize_ExactlyAtLimit(t *testing.T) {
	handler := MaxBodySize(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1)
		_, err := r.Body.Read(buf)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	body := strings.Repeat("x", 1024*1024)
	req := httptest.NewRequest("POST", "/api/v1/test", strings.NewReader(body))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for body at limit, got %d", w.Code)
	}
}
