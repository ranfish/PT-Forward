package httpclient

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestRetryTransport_SuccessOnFirstAttempt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	transport := NewRetryTransport(http.DefaultTransport, DefaultRetryConfig(), nil)
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestRetryTransport_RetriesOn500(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n <= 2 {
			w.WriteHeader(503)
			return
		}
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	transport := NewRetryTransport(http.DefaultTransport, RetryConfig{
		MaxRetries:     3,
		BaseDelay:      10 * time.Millisecond,
		MaxDelay:       100 * time.Millisecond,
		RetryableCheck: DefaultRetryableCheck,
	}, nil)
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("expected 3 attempts, got %d", atomic.LoadInt32(&attempts))
	}
}

func TestRetryTransport_MaxRetriesExhausted(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(503)
	}))
	defer server.Close()

	transport := NewRetryTransport(http.DefaultTransport, RetryConfig{
		MaxRetries:     2,
		BaseDelay:      10 * time.Millisecond,
		MaxDelay:       100 * time.Millisecond,
		RetryableCheck: DefaultRetryableCheck,
	}, nil)
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 503 {
		t.Errorf("expected 503, got %d", resp.StatusCode)
	}
	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("expected 3 attempts (1 + 2 retries), got %d", atomic.LoadInt32(&attempts))
	}
}

func TestRetryTransport_NoRetryOn4xx(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(403)
	}))
	defer server.Close()

	transport := NewRetryTransport(http.DefaultTransport, RetryConfig{
		MaxRetries:     3,
		BaseDelay:      10 * time.Millisecond,
		MaxDelay:       100 * time.Millisecond,
		RetryableCheck: DefaultRetryableCheck,
	}, nil)
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
	if atomic.LoadInt32(&attempts) != 1 {
		t.Errorf("expected 1 attempt (no retry on 4xx), got %d", atomic.LoadInt32(&attempts))
	}
}

func TestRetryTransport_RetryOnNetworkError(t *testing.T) {
	var callCount int32
	transport := NewRetryTransport(&mockTransport{
		roundTrip: func(req *http.Request) (*http.Response, error) {
			n := atomic.AddInt32(&callCount, 1)
			if n <= 1 {
				return nil, errors.New("connection refused")
			}
			return &http.Response{StatusCode: 200, Body: http.NoBody}, nil
		},
	}, RetryConfig{
		MaxRetries:     3,
		BaseDelay:      10 * time.Millisecond,
		MaxDelay:       100 * time.Millisecond,
		RetryableCheck: DefaultRetryableCheck,
	}, nil)
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}

	req, _ := http.NewRequest("GET", "http://test.local/fake", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestCircuitBreaker_ClosedToOpen(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(500)
	}))
	defer server.Close()

	cb := NewCircuitBreakerTransport(http.DefaultTransport, CircuitBreakerConfig{
		FailureThreshold: 3,
		RecoveryTimeout:  1 * time.Second,
		HalfOpenMaxReqs:  1,
		SuccessThreshold: 2,
		FailureChecker:   DefaultCircuitFailureChecker,
	}, zap.NewNop())

	client := &http.Client{Transport: cb, Timeout: 5 * time.Second}

	for i := 0; i < 3; i++ {
		resp, _ := client.Get(server.URL)
		if resp != nil {
			resp.Body.Close()
		}
	}

	status := cb.GetCircuitStatus("http://" + server.Listener.Addr().String())
	if status.State != CircuitOpen {
		t.Errorf("expected circuit open, got %s", status.State)
	}

	resp, err := client.Get(server.URL)
	if err == nil {
		resp.Body.Close()
		t.Error("expected error when circuit is open")
	}
}

func TestCircuitBreaker_OpenToHalfOpenToClosed(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n <= 3 {
			w.WriteHeader(500)
			return
		}
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	cb := NewCircuitBreakerTransport(http.DefaultTransport, CircuitBreakerConfig{
		FailureThreshold: 3,
		RecoveryTimeout:  100 * time.Millisecond,
		HalfOpenMaxReqs:  3,
		SuccessThreshold: 2,
		FailureChecker:   DefaultCircuitFailureChecker,
	}, zap.NewNop())

	client := &http.Client{Transport: cb, Timeout: 5 * time.Second}

	for i := 0; i < 3; i++ {
		resp, _ := client.Get(server.URL)
		if resp != nil {
			resp.Body.Close()
		}
	}

	time.Sleep(150 * time.Millisecond)

	for i := 0; i < 2; i++ {
		resp, err := client.Get(server.URL)
		if err != nil {
			t.Fatalf("unexpected error in half-open: %v", err)
		}
		resp.Body.Close()
	}

	status := cb.GetCircuitStatus("http://" + server.Listener.Addr().String())
	if status.State != CircuitClosed {
		t.Errorf("expected circuit closed, got %s", status.State)
	}
}

func TestCircuitBreaker_HalfOpenTripBackToOpen(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(500)
	}))
	defer server.Close()

	cb := NewCircuitBreakerTransport(http.DefaultTransport, CircuitBreakerConfig{
		FailureThreshold: 2,
		RecoveryTimeout:  100 * time.Millisecond,
		HalfOpenMaxReqs:  3,
		SuccessThreshold: 2,
		FailureChecker:   DefaultCircuitFailureChecker,
	}, zap.NewNop())

	client := &http.Client{Transport: cb, Timeout: 5 * time.Second}

	for i := 0; i < 2; i++ {
		resp, _ := client.Get(server.URL)
		if resp != nil {
			resp.Body.Close()
		}
	}

	time.Sleep(150 * time.Millisecond)

	resp, _ := client.Get(server.URL)
	if resp != nil {
		resp.Body.Close()
	}

	status := cb.GetCircuitStatus("http://" + server.Listener.Addr().String())
	if status.State != CircuitOpen {
		t.Errorf("expected circuit open after half-open failure, got %s", status.State)
	}
}

func TestCircuitBreaker_Reset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer server.Close()

	domain := "http://" + server.Listener.Addr().String()
	cb := NewCircuitBreakerTransport(http.DefaultTransport, CircuitBreakerConfig{
		FailureThreshold: 2,
		RecoveryTimeout:  1 * time.Second,
		HalfOpenMaxReqs:  1,
		SuccessThreshold: 1,
		FailureChecker:   DefaultCircuitFailureChecker,
	}, zap.NewNop())

	client := &http.Client{Transport: cb, Timeout: 5 * time.Second}

	for i := 0; i < 2; i++ {
		resp, _ := client.Get(server.URL)
		if resp != nil {
			resp.Body.Close()
		}
	}

	status := cb.GetCircuitStatus(domain)
	if status.State != CircuitOpen {
		t.Errorf("expected open, got %s", status.State)
	}

	cb.ResetCircuit(domain)

	status = cb.GetCircuitStatus(domain)
	if status.State != CircuitClosed {
		t.Errorf("expected closed after reset, got %s", status.State)
	}
}

func TestCircuitBreaker_GetAllStatuses(t *testing.T) {
	cb := NewCircuitBreakerTransport(http.DefaultTransport, CircuitBreakerConfig{
		FailureThreshold: 2,
		RecoveryTimeout:  1 * time.Second,
		HalfOpenMaxReqs:  1,
		SuccessThreshold: 1,
		FailureChecker:   DefaultCircuitFailureChecker,
	}, zap.NewNop())

	cb.getOrCreate("a.com").state = CircuitOpen
	cb.getOrCreate("b.com").state = CircuitClosed

	statuses := cb.GetAllCircuitStatuses()
	if statuses["a.com"].State != CircuitOpen {
		t.Error("a.com should be open")
	}
	if statuses["b.com"].State != CircuitClosed {
		t.Error("b.com should be closed")
	}
}

func TestFullChain_RetryCircuitAndRateLimit(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n <= 2 {
			w.WriteHeader(503)
			return
		}
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	limiter := NewDomainRateLimiter(0)
	limiter.SetDomainConfig("test.local", DomainLimitConfig{MaxReqs: 100, WindowSecs: 60})

	detector := NewWAFResponseDetector(nil)

	base := http.DefaultTransport
	retryLayer := NewRetryTransport(base, RetryConfig{
		MaxRetries:     3,
		BaseDelay:      10 * time.Millisecond,
		MaxDelay:       100 * time.Millisecond,
		RetryableCheck: DefaultRetryableCheck,
	}, nil)
	cbLayer := NewCircuitBreakerTransport(retryLayer, CircuitBreakerConfig{
		FailureThreshold: 10,
		RecoveryTimeout:  1 * time.Second,
		HalfOpenMaxReqs:  3,
		SuccessThreshold: 3,
		FailureChecker:   DefaultCircuitFailureChecker,
	}, zap.NewNop())
	rateLimitLayer := &domainLimiterTransport{
		limiter:  limiter,
		detector: detector,
		emitter:  nil,
		domain:   "test.local",
		base:     cbLayer,
	}

	client := &http.Client{Transport: rateLimitLayer, Timeout: 5 * time.Second}

	req, _ := http.NewRequest("GET", server.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

type mockTransport struct {
	roundTrip func(*http.Request) (*http.Response, error)
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.roundTrip(req)
}

var _ io.Reader = (*cachedReader)(nil)
