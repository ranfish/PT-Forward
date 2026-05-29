package httpclient

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/ranfish/pt-forward/internal/model"
)

func TestCircuitState_String(t *testing.T) {
	cases := map[CircuitState]string{
		CircuitClosed:    "closed",
		CircuitOpen:      "open",
		CircuitHalfOpen:  "half-open",
		CircuitState(99): "unknown",
	}
	for state, want := range cases {
		if got := state.String(); got != want {
			t.Errorf("CircuitState(%d).String() = %q, want %q", state, got, want)
		}
	}
}

func TestDefaultCircuitBreakerConfig_Values(t *testing.T) {
	cfg := DefaultCircuitBreakerConfig()
	if cfg.FailureThreshold != 5 {
		t.Errorf("FailureThreshold = %d, want 5", cfg.FailureThreshold)
	}
	if cfg.RecoveryTimeout != 60*time.Second {
		t.Errorf("RecoveryTimeout = %v, want 60s", cfg.RecoveryTimeout)
	}
	if cfg.HalfOpenMaxReqs != 3 {
		t.Errorf("HalfOpenMaxReqs = %d, want 3", cfg.HalfOpenMaxReqs)
	}
	if cfg.SuccessThreshold != 3 {
		t.Errorf("SuccessThreshold = %d, want 3", cfg.SuccessThreshold)
	}
	if cfg.FailureChecker == nil {
		t.Error("FailureChecker should not be nil")
	}
}

func TestDefaultCircuitFailureChecker(t *testing.T) {
	if !DefaultCircuitFailureChecker(nil, errors.New("err")) {
		t.Error("should return true on error")
	}
	if !DefaultCircuitFailureChecker(&http.Response{StatusCode: 500}, nil) {
		t.Error("should return true on 500")
	}
	if DefaultCircuitFailureChecker(&http.Response{StatusCode: 200}, nil) {
		t.Error("should return false on 200")
	}
}

func TestNewCircuitBreakerTransport_Defaults(t *testing.T) {
	cb := NewCircuitBreakerTransport(http.DefaultTransport, CircuitBreakerConfig{
		FailureThreshold: -1,
		RecoveryTimeout:  -1,
		HalfOpenMaxReqs:  -1,
		SuccessThreshold: -1,
		FailureChecker:   nil,
	}, nil)
	if cb.config.FailureThreshold != 5 {
		t.Errorf("FailureThreshold = %d, want 5", cb.config.FailureThreshold)
	}
	if cb.config.RecoveryTimeout != 60*time.Second {
		t.Errorf("RecoveryTimeout = %v, want 60s", cb.config.RecoveryTimeout)
	}
	if cb.config.HalfOpenMaxReqs != 3 {
		t.Errorf("HalfOpenMaxReqs = %d, want 3", cb.config.HalfOpenMaxReqs)
	}
	if cb.config.SuccessThreshold != 3 {
		t.Errorf("SuccessThreshold = %d, want 3", cb.config.SuccessThreshold)
	}
	if cb.config.FailureChecker == nil {
		t.Error("FailureChecker should not be nil")
	}
}

func TestCircuitBreakerTransport_CloseIdleConnections(t *testing.T) {
	called := false
	base := &mockCloseableTransport{closeCalled: &called}
	cb := NewCircuitBreakerTransport(base, DefaultCircuitBreakerConfig(), nil)
	cb.CloseIdleConnections()
	if !called {
		t.Error("CloseIdleConnections was not called on base")
	}
}

type mockCloseableTransport struct {
	closeCalled *bool
}

func (m *mockCloseableTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: 200, Body: http.NoBody}, nil
}

func (m *mockCloseableTransport) CloseIdleConnections() {
	*m.closeCalled = true
}

func TestRetryTransport_CloseIdleConnections(t *testing.T) {
	called := false
	base := &mockCloseableTransport{closeCalled: &called}
	rt := NewRetryTransport(base, DefaultRetryConfig(), nil)
	rt.CloseIdleConnections()
	if !called {
		t.Error("CloseIdleConnections was not called on base")
	}
}

func TestDomainLimiterTransport_CloseIdleConnections(t *testing.T) {
	called := false
	base := &mockCloseableTransport{closeCalled: &called}
	transport := &domainLimiterTransport{
		limiter:  NewDomainRateLimiter(0),
		detector: nil,
		emitter:  nil,
		domain:   "test.local",
		base:     base,
	}
	transport.CloseIdleConnections()
	if !called {
		t.Error("CloseIdleConnections was not called on base")
	}
}

func TestNewRetryTransport_Defaults(t *testing.T) {
	rt := NewRetryTransport(http.DefaultTransport, RetryConfig{
		MaxRetries:     -1,
		BaseDelay:      -1,
		MaxDelay:       -1,
		RetryableCheck: nil,
	}, nil)
	if rt.config.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", rt.config.MaxRetries)
	}
	if rt.config.BaseDelay != 500*time.Millisecond {
		t.Errorf("BaseDelay = %v, want 500ms", rt.config.BaseDelay)
	}
	if rt.config.MaxDelay != 30*time.Second {
		t.Errorf("MaxDelay = %v, want 30s", rt.config.MaxDelay)
	}
	if rt.config.RetryableCheck == nil {
		t.Error("RetryableCheck should not be nil")
	}
}

func TestDefaultRetryableCheck_EdgeCases(t *testing.T) {
	if !DefaultRetryableCheck(nil, nil) {
		t.Error("nil resp should be retryable")
	}
	if !DefaultRetryableCheck(nil, errors.New("err")) {
		t.Error("error should be retryable")
	}
	if !DefaultRetryableCheck(&http.Response{StatusCode: 429}, nil) {
		t.Error("429 should be retryable")
	}
	if !DefaultRetryableCheck(&http.Response{StatusCode: 502}, nil) {
		t.Error("502 should be retryable")
	}
	if !DefaultRetryableCheck(&http.Response{StatusCode: 599}, nil) {
		t.Error("599 should be retryable")
	}
	if DefaultRetryableCheck(&http.Response{StatusCode: 200}, nil) {
		t.Error("200 should not be retryable")
	}
	if DefaultRetryableCheck(&http.Response{StatusCode: 404}, nil) {
		t.Error("404 should not be retryable")
	}
}

func TestRetryTransport_OnRetryCallback(t *testing.T) {
	var onRetryCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
	}))
	defer server.Close()

	transport := NewRetryTransport(http.DefaultTransport, RetryConfig{
		MaxRetries:     2,
		BaseDelay:      10 * time.Millisecond,
		MaxDelay:       100 * time.Millisecond,
		RetryableCheck: DefaultRetryableCheck,
		OnRetry: func(attempt int, resp *http.Response, err error) {
			atomic.AddInt32(&onRetryCalls, 1)
		},
	}, nil)
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()
	if atomic.LoadInt32(&onRetryCalls) != 3 {
		t.Errorf("expected 3 OnRetry calls, got %d", atomic.LoadInt32(&onRetryCalls))
	}
}

func TestRetryTransport_ContextCancel(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(503)
	}))
	defer server.Close()

	transport := NewRetryTransport(http.DefaultTransport, RetryConfig{
		MaxRetries:     10,
		BaseDelay:      5 * time.Second,
		MaxDelay:       10 * time.Second,
		RetryableCheck: DefaultRetryableCheck,
	}, nil)
	client := &http.Client{Transport: transport, Timeout: 30 * time.Second}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	_, err := client.Do(req)
	if err == nil {
		t.Error("expected context deadline error")
	}
}

func TestRetryTransport_BodyRestore(t *testing.T) {
	var lastBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		lastBody = string(body)
		w.WriteHeader(503)
	}))
	defer server.Close()

	var reqAttempts int32
	transport2 := NewRetryTransport(http.DefaultTransport, RetryConfig{
		MaxRetries: 2,
		BaseDelay:  10 * time.Millisecond,
		MaxDelay:   100 * time.Millisecond,
		RetryableCheck: func(resp *http.Response, err error) bool {
			n := atomic.AddInt32(&reqAttempts, 1)
			return n <= 2
		},
	}, nil)

	client := &http.Client{Transport: transport2, Timeout: 5 * time.Second}
	body := bytes.NewReader([]byte("test-payload"))
	req, _ := http.NewRequest("POST", server.URL, io.NopCloser(body))

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()

	if lastBody != "test-payload" {
		t.Errorf("body not restored on retry, got %q", lastBody)
	}
}

func TestCachedBody_Reader(t *testing.T) {
	cb := &cachedBody{data: []byte("hello")}
	cr := cb.Reader()
	buf := make([]byte, 10)
	n, err := cr.Read(buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(buf[:n]) != "hello" {
		t.Errorf("got %q, want %q", string(buf[:n]), "hello")
	}

	n2, err2 := cr.Read(buf)
	if n2 != 0 || err2 != io.EOF {
		t.Errorf("expected (0, EOF), got (%d, %v)", n2, err2)
	}
}

func TestReadAndCacheBody_NilBody(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://test.local", nil)
	cb, err := readAndCacheBody(req)
	if cb != nil || err != nil {
		t.Errorf("expected (nil, nil), got (%v, %v)", cb, err)
	}
}

func TestReadAndCacheBody_WithGetBody(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://test.local", nil)
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("body")), nil
	}
	cb, err := readAndCacheBody(req)
	if cb != nil || err != nil {
		t.Errorf("expected (nil, nil) when GetBody is set, got (%v, %v)", cb, err)
	}
}

func TestReadAndCacheBody_WithBody(t *testing.T) {
	req, _ := http.NewRequest("POST", "http://test.local", io.NopCloser(strings.NewReader("payload")))
	cb, err := readAndCacheBody(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cb == nil {
		t.Fatal("expected non-nil cachedBody")
	}
	cr := cb.Reader()
	data, _ := io.ReadAll(cr)
	if string(data) != "payload" {
		t.Errorf("cached body = %q, want %q", string(data), "payload")
	}
}

func TestReadAndCacheBody_ReadError(t *testing.T) {
	req, _ := http.NewRequest("POST", "http://test.local", &errorReader{})
	cb, err := readAndCacheBody(req)
	if cb != nil {
		t.Error("expected nil cachedBody on error")
	}
	if err == nil {
		t.Error("expected error")
	}
}

type errorReader struct{}

func (e *errorReader) Read(p []byte) (int, error) { return 0, errors.New("read error") }
func (e *errorReader) Close() error               { return nil }

func TestFreezeEventEmitter_SetDB(t *testing.T) {
	emitter := NewFreezeEventEmitter(nil)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite not available: %v", err)
	}
	err = db.AutoMigrate(&model.FreezeEventRecord{})
	if err != nil {
		t.Skipf("migration failed: %v", err)
	}
	emitter.SetDB(db)
	if emitter.db != db {
		t.Error("SetDB did not set the db")
	}
}

func TestFreezeEventEmitter_EmitWithDB(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite not available: %v", err)
	}
	err = db.AutoMigrate(&model.FreezeEventRecord{})
	if err != nil {
		t.Skipf("migration failed: %v", err)
	}
	logger := zap.NewNop()
	emitter := NewFreezeEventEmitter(logger)
	emitter.SetDB(db)
	ch := make(chan FreezeEvent, 10)
	emitter.Subscribe(ch)

	emitter.Emit(FreezeEvent{
		Domain:   "emit-db-test.local",
		Reason:   "test",
		Duration: 30 * time.Second,
		URL:      "http://emit-db-test.local/page",
		At:       time.Now(),
	})

	var record model.FreezeEventRecord
	result := db.Where("domain = ?", "emit-db-test.local").First(&record)
	if result.Error != nil {
		t.Errorf("expected record in db: %v", result.Error)
	}
	if record.Domain != "emit-db-test.local" {
		t.Errorf("domain = %q, want %q", record.Domain, "emit-db-test.local")
	}
}

func TestFreezeEventEmitter_EmitWithLogger(t *testing.T) {
	logger := zap.NewNop()
	emitter := NewFreezeEventEmitter(logger)
	emitter.Emit(FreezeEvent{
		Domain:   "log-test.local",
		Reason:   "test",
		Duration: 10 * time.Second,
		URL:      "http://log-test.local/test",
		At:       time.Now(),
	})
}

func TestFreezeEventEmitter_EmitSubscriberFull(t *testing.T) {
	emitter := NewFreezeEventEmitter(nil)
	ch := make(chan FreezeEvent)
	emitter.Subscribe(ch)

	emitter.Emit(FreezeEvent{
		Domain:   "full.local",
		Reason:   "test",
		Duration: 10 * time.Second,
		At:       time.Now(),
	})
}

func TestDomainLimiterTransport_EmptyDomain(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	limiter := NewDomainRateLimiter(100)
	transport := &domainLimiterTransport{
		limiter:  limiter,
		detector: nil,
		emitter:  nil,
		domain:   "",
		base:     http.DefaultTransport,
	}
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

func TestDomainLimiterTransport_RateLimitError(t *testing.T) {
	limiter := NewDomainRateLimiter(0)
	limiter.ManualFreeze("test.local", 10*time.Second, "test")

	transport := &domainLimiterTransport{
		limiter:  limiter,
		detector: nil,
		emitter:  nil,
		domain:   "test.local",
		base:     http.DefaultTransport,
	}
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", "http://test.local/page", nil)
	_, err := client.Do(req)
	if err == nil {
		t.Error("expected error due to rate limit freeze")
	}
}

func TestDomainLimiterTransport_WAFFreezeWithEmitter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
	}))
	defer server.Close()

	limiter := NewDomainRateLimiter(100)
	emitter := NewFreezeEventEmitter(nil)
	ch := make(chan FreezeEvent, 10)
	emitter.Subscribe(ch)

	transport := &domainLimiterTransport{
		limiter:  limiter,
		detector: NewWAFResponseDetector(nil),
		emitter:  emitter,
		domain:   "test.local",
		base:     http.DefaultTransport,
	}
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()

	select {
	case evt := <-ch:
		if evt.Domain != "test.local" {
			t.Errorf("event domain = %q, want %q", evt.Domain, "test.local")
		}
	case <-time.After(2 * time.Second):
		t.Error("expected freeze event")
	}
}

func TestDomainLimiterTransport_BaseError(t *testing.T) {
	limiter := NewDomainRateLimiter(100)
	transport := &domainLimiterTransport{
		limiter:  limiter,
		detector: nil,
		emitter:  nil,
		domain:   "test.local",
		base: &mockTransport{
			roundTrip: func(req *http.Request) (*http.Response, error) {
				return nil, errors.New("connection refused")
			},
		},
	}
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}

	req, _ := http.NewRequest("GET", "http://test.local/page", nil)
	_, err := client.Do(req)
	if err == nil {
		t.Error("expected error from base transport")
	}
}

func TestInit(t *testing.T) {
	logger := zap.NewNop()
	Init(logger)
	if GlobalEmitter == nil {
		t.Error("GlobalEmitter should be initialized")
	}
	if GlobalCircuitBreaker == nil {
		t.Error("GlobalCircuitBreaker should be initialized")
	}
}

func TestNewSiteHTTPClient(t *testing.T) {
	logger := zap.NewNop()
	Init(logger)

	t.Run("defaults", func(t *testing.T) {
		client := NewSiteHTTPClient(SiteHTTPConfig{
			Domain: "example.com",
		})
		if client == nil {
			t.Fatal("client should not be nil")
		}
		if client.Timeout != 30*time.Second {
			t.Errorf("Timeout = %v, want 30s", client.Timeout)
		}
	})

	t.Run("custom timeout", func(t *testing.T) {
		client := NewSiteHTTPClient(SiteHTTPConfig{
			Domain:  "example.com",
			Timeout: 10 * time.Second,
		})
		if client.Timeout != 10*time.Second {
			t.Errorf("Timeout = %v, want 10s", client.Timeout)
		}
	})

	t.Run("with proxy", func(t *testing.T) {
		client := NewSiteHTTPClient(SiteHTTPConfig{
			Domain:   "example.com",
			ProxyURL: "http://127.0.0.1:9999",
		})
		if client == nil {
			t.Fatal("client should not be nil")
		}
	})

	t.Run("with custom WAF patterns", func(t *testing.T) {
		client := NewSiteHTTPClient(SiteHTTPConfig{
			Domain: "example.com",
			WAFPatterns: []wafPattern{
				{
					Name: "custom_waf",
					MatchFunc: func(resp *http.Response) bool {
						return resp.StatusCode == 999
					},
					FreezeDur: 10 * time.Second,
				},
			},
		})
		if client == nil {
			t.Fatal("client should not be nil")
		}
	})

	t.Run("invalid proxy URL", func(t *testing.T) {
		client := NewSiteHTTPClient(SiteHTTPConfig{
			Domain:   "example.com",
			ProxyURL: "://invalid",
		})
		if client == nil {
			t.Fatal("client should not be nil even with invalid proxy")
		}
	})
}

func TestBuildRetryChain_WithGlobalCB(t *testing.T) {
	logger := zap.NewNop()
	Init(logger)

	chain := buildRetryChain(http.DefaultTransport, "chain-test.local")
	if chain == nil {
		t.Error("buildRetryChain should return non-nil")
	}
}

func TestBuildRetryChain_WithoutGlobalCB(t *testing.T) {
	orig := GlobalCircuitBreaker
	GlobalCircuitBreaker = nil
	defer func() { GlobalCircuitBreaker = orig }()

	chain := buildRetryChain(http.DefaultTransport, "no-cb.local")
	if chain == nil {
		t.Error("buildRetryChain should return non-nil without CB")
	}
}

func TestWAFDetector_CustomPatterns(t *testing.T) {
	detector := NewWAFResponseDetector([]wafPattern{
		{
			Name: "custom_999",
			MatchFunc: func(resp *http.Response) bool {
				return resp.StatusCode == 999
			},
			FreezeDur: 42 * time.Second,
		},
	})
	resp := &http.Response{StatusCode: 999, Body: http.NoBody}
	dur := detector.Detect(resp)
	if dur != 42*time.Second {
		t.Errorf("expected 42s for custom pattern, got %v", dur)
	}
	if detector.LastReason() != "custom_999" {
		t.Errorf("expected reason custom_999, got %s", detector.LastReason())
	}
}

func TestWAFDetector_CloudflareChallenge_403(t *testing.T) {
	detector := NewWAFResponseDetector(nil)
	body := `<html><head><title>cf-browser-verification</title></head></html>`
	resp := &http.Response{
		StatusCode: 403,
		Body:       newStringBody(body),
	}
	dur := detector.Detect(resp)
	if dur != 5*time.Minute {
		t.Errorf("expected 5min for CF challenge 403, got %v", dur)
	}
}

func TestWAFDetector_Cloudflare5sShield(t *testing.T) {
	detector := NewWAFResponseDetector(nil)
	body := `<html><body><script src="trk_jschal_js"></script></body></html>`
	resp := &http.Response{
		StatusCode: 503,
		Body:       newStringBody(body),
	}
	dur := detector.Detect(resp)
	if dur != 5*time.Minute {
		t.Errorf("expected 5min for CF 5s shield, got %v", dur)
	}
}

func TestWAFDetector_LoginRedirect_Variants(t *testing.T) {
	cases := []struct {
		name   string
		body   string
		expect time.Duration
	}{
		{
			"action=login",
			`<html><form action="login" method="post"><input name="user"/></form></html>`,
			0,
		},
		{
			"action=/auth/login",
			`<html><form action="/auth/login" method="post"><input name="user"/></form></html>`,
			0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			detector := NewWAFResponseDetector(nil)
			resp := &http.Response{
				StatusCode: 200,
				Body:       newStringBody(tc.body),
			}
			dur := detector.Detect(resp)
			if dur != tc.expect {
				t.Errorf("expected %v, got %v", tc.expect, dur)
			}
		})
	}
}

func TestWAFDetector_RateLimitText_English(t *testing.T) {
	detector := NewWAFResponseDetector(nil)
	body := `<html><body>rate limit exceeded, please slow down</body></html>`
	resp := &http.Response{
		StatusCode: 200,
		Body:       newStringBody(body),
	}
	dur := detector.Detect(resp)
	if dur != 60*time.Second {
		t.Errorf("expected 60s for rate limit text, got %v", dur)
	}
}

func TestWAFDetector_PeekBodyLimit(t *testing.T) {
	detector := NewWAFResponseDetector(nil)
	longBody := strings.Repeat("x", 8192) + "Just a moment"
	resp := &http.Response{
		StatusCode: 503,
		Body:       newStringBody(longBody),
	}
	dur := detector.Detect(resp)
	if dur != 0 {
		t.Errorf("pattern beyond peek limit should not match, got %v", dur)
	}
}

func TestAcquire_RateLimitWait(t *testing.T) {
	limiter := NewDomainRateLimiter(0)
	limiter.SetDomainConfig("wait.local", DomainLimitConfig{MaxReqs: 2, WindowSecs: 2, MaxConcurrent: 5})

	for i := 0; i < 2; i++ {
		if err := limiter.Acquire(context.Background(), "wait.local"); err != nil {
			t.Fatalf("acquire %d failed: %v", i, err)
		}
		limiter.Release("wait.local")
	}

	start := time.Now()
	if err := limiter.Acquire(context.Background(), "wait.local"); err != nil {
		t.Fatalf("acquire after wait failed: %v", err)
	}
	limiter.Release("wait.local")
	elapsed := time.Since(start)
	if elapsed < 500*time.Millisecond {
		t.Errorf("expected rate limit wait, elapsed=%v", elapsed)
	}
}

func TestAcquire_RateLimitContextCancel(t *testing.T) {
	limiter := NewDomainRateLimiter(0)
	limiter.SetDomainConfig("cancel.local", DomainLimitConfig{MaxReqs: 1, WindowSecs: 10, MaxConcurrent: 5})
	limiter.Acquire(context.Background(), "cancel.local")
	limiter.Release("cancel.local")

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := limiter.Acquire(ctx, "cancel.local")
	if err == nil {
		t.Error("expected context cancelled error")
		limiter.Release("cancel.local")
	}
}

func TestNewDomainRateLimiter_DefaultRPS(t *testing.T) {
	limiter := NewDomainRateLimiter(1.0)
	state := limiter.getOrCreate("rps-test.local")
	if state.maxReqs != 60 {
		t.Errorf("maxReqs = %d, want 60", state.maxReqs)
	}
}

func TestRetryTransport_WithLogger(t *testing.T) {
	logger := zap.NewNop()
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n <= 1 {
			w.WriteHeader(500)
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
	}, logger)
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

func TestCircuitBreaker_HalfOpenMaxReqsExceeded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	cb := NewCircuitBreakerTransport(http.DefaultTransport, CircuitBreakerConfig{
		FailureThreshold: 2,
		RecoveryTimeout:  100 * time.Millisecond,
		HalfOpenMaxReqs:  1,
		SuccessThreshold: 3,
		FailureChecker:   DefaultCircuitFailureChecker,
	}, zap.NewNop())

	domain := "http://" + server.Listener.Addr().String()

	circuit := cb.getOrCreate(domain)
	circuit.state = CircuitHalfOpen
	circuit.halfOpenAttempts = 1

	client := &http.Client{Transport: cb, Timeout: 5 * time.Second}
	_, err := client.Get(server.URL)
	if err == nil {
		t.Error("expected error when half-open max requests exceeded")
	}
}

func TestCircuitBreaker_ClosedResetsFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	cb := NewCircuitBreakerTransport(http.DefaultTransport, CircuitBreakerConfig{
		FailureThreshold: 5,
		RecoveryTimeout:  1 * time.Second,
		HalfOpenMaxReqs:  3,
		SuccessThreshold: 2,
		FailureChecker:   DefaultCircuitFailureChecker,
	}, zap.NewNop())

	client := &http.Client{Transport: cb, Timeout: 5 * time.Second}

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()

	domain := "http://" + server.Listener.Addr().String()
	status := cb.GetCircuitStatus(domain)
	if status.Failures != 0 {
		t.Errorf("expected 0 failures after success, got %d", status.Failures)
	}
}

func TestDomainLimiterTransport_ResetFreezeCounterOnSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	limiter := NewDomainRateLimiter(100)
	limiter.FreezeWithBackoff("test.local", 10*time.Millisecond)
	limiter.ResetFreezeCounter("test.local")

	transport := &domainLimiterTransport{
		limiter:  limiter,
		detector: NewWAFResponseDetector(nil),
		emitter:  nil,
		domain:   "test.local",
		base:     http.DefaultTransport,
	}
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

func TestHttpError(t *testing.T) {
	cause := errors.New("root cause")
	appErr := httpError(ErrHTRateLimit, "test msg", cause)
	if appErr.Code != ErrHTRateLimit {
		t.Errorf("code = %d, want %d", appErr.Code, ErrHTRateLimit)
	}
	if appErr.Message != "test msg" {
		t.Errorf("message = %q, want %q", appErr.Message, "test msg")
	}
	if appErr.Cause != cause {
		t.Error("cause mismatch")
	}
}

func TestNewSiteHTTPClient_FullChain(t *testing.T) {
	logger := zap.NewNop()
	Init(logger)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	client := NewSiteHTTPClient(SiteHTTPConfig{
		Domain:              "chain.local",
		Timeout:             5 * time.Second,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     30 * time.Second,
	})

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestBackoffDelay_MaxDelay(t *testing.T) {
	rt := NewRetryTransport(http.DefaultTransport, RetryConfig{
		MaxRetries:     3,
		BaseDelay:      1 * time.Nanosecond,
		MaxDelay:       2 * time.Nanosecond,
		RetryableCheck: DefaultRetryableCheck,
	}, nil)

	delay := rt.backoffDelay(10)
	if delay != 2*time.Nanosecond {
		t.Errorf("expected max delay cap, got %v", delay)
	}
}

func TestFreezeEventEmitter_EmitDBError(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite not available: %v", err)
	}

	logger := zap.NewNop()
	emitter := NewFreezeEventEmitter(logger)
	emitter.SetDB(db)

	ch := make(chan FreezeEvent, 10)
	emitter.Subscribe(ch)

	emitter.Emit(FreezeEvent{
		Domain:   "db-error.local",
		Reason:   "test",
		Duration: 30 * time.Second,
		URL:      "http://db-error.local/test",
		At:       time.Now(),
	})

	select {
	case <-ch:
	default:
		t.Error("expected event even with db error")
	}
}

func TestRetryTransport_GetBodyRestore(t *testing.T) {
	var bodies []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(body))
		w.WriteHeader(503)
	}))
	defer server.Close()

	var attempts int32
	transport := NewRetryTransport(http.DefaultTransport, RetryConfig{
		MaxRetries: 2,
		BaseDelay:  10 * time.Millisecond,
		MaxDelay:   100 * time.Millisecond,
		RetryableCheck: func(resp *http.Response, err error) bool {
			n := atomic.LoadInt32(&attempts)
			return n <= 2
		},
	}, nil)
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}

	body := []byte("restore-test")
	req, _ := http.NewRequest("POST", server.URL, bytes.NewReader(body))
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()
}

func TestCircuitBreaker_CircuitStatusFields(t *testing.T) {
	cb := NewCircuitBreakerTransport(http.DefaultTransport, DefaultCircuitBreakerConfig(), nil)

	circuit := cb.getOrCreate("status-test.local")
	circuit.state = CircuitOpen
	circuit.failures = 5
	circuit.openedAt = time.Now().Add(-30 * time.Second)
	circuit.lastFailure = time.Now().Add(-10 * time.Second)

	status := cb.GetCircuitStatus("status-test.local")
	if status.State != CircuitOpen {
		t.Errorf("state = %v, want open", status.State)
	}
	if status.Failures != 5 {
		t.Errorf("failures = %d, want 5", status.Failures)
	}
	if status.OpenedAt == nil {
		t.Error("OpenedAt should not be nil")
	}
	if status.LastFailure == nil {
		t.Error("LastFailure should not be nil")
	}
}
