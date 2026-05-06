package httpclient

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestDomainRateLimiter_BasicAcquire(t *testing.T) {
	limiter := NewDomainRateLimiter(0)
	limiter.SetDomainConfig("example.com", DomainLimitConfig{MaxReqs: 3, WindowSecs: 1})

	for i := 0; i < 3; i++ {
		if err := limiter.Acquire(context.Background(), "example.com"); err != nil {
			t.Fatalf("acquire %d failed: %v", i, err)
		}
	}
}

func TestDomainRateLimiter_Freeze(t *testing.T) {
	limiter := NewDomainRateLimiter(0)
	limiter.Freeze("example.com", 100*time.Millisecond)

	start := time.Now()
	err := limiter.Acquire(context.Background(), "example.com")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("acquire after freeze failed: %v", err)
	}
	if elapsed < 80*time.Millisecond {
		t.Errorf("expected to wait for freeze, elapsed=%v", elapsed)
	}
}

func TestDomainRateLimiter_FreezeWithBackoff(t *testing.T) {
	limiter := NewDomainRateLimiter(0)

	limiter.FreezeWithBackoff("example.com", 10*time.Millisecond)
	status := limiter.GetFreezeStatus("example.com")
	if !status.Frozen {
		t.Error("should be frozen after first backoff")
	}

	limiter.ResetFreezeCounter("example.com")
	limiter.FreezeWithBackoff("example.com", 10*time.Millisecond)
	status2 := limiter.GetFreezeStatus("example.com")
	if !status2.Frozen {
		t.Error("should be frozen after reset+backoff")
	}
}

func TestDomainRateLimiter_ManualFreezeUnfreeze(t *testing.T) {
	limiter := NewDomainRateLimiter(0)

	limiter.ManualFreeze("example.com", 1*time.Hour, "test")
	status := limiter.GetFreezeStatus("example.com")
	if !status.Frozen || status.FrozenBy != "manual" || status.Reason != "test" {
		t.Errorf("unexpected status: %+v", status)
	}

	limiter.ManualUnfreeze("example.com")
	status = limiter.GetFreezeStatus("example.com")
	if status.Frozen {
		t.Error("should not be frozen after unfreeze")
	}
}

func TestDomainRateLimiter_CancelContext(t *testing.T) {
	limiter := NewDomainRateLimiter(0)
	limiter.Freeze("example.com", 5*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := limiter.Acquire(ctx, "example.com")
	if err == nil {
		t.Error("expected context cancellation error")
	}
}

func TestWAFDetector_HTTP429(t *testing.T) {
	detector := NewWAFResponseDetector(nil)
	resp := &http.Response{StatusCode: 429, Body: http.NoBody}
	dur := detector.Detect(resp)
	if dur != 30*time.Second {
		t.Errorf("expected 30s freeze for 429, got %v", dur)
	}
	if detector.LastReason() != "http_429" {
		t.Errorf("expected reason http_429, got %s", detector.LastReason())
	}
}

func TestWAFDetector_CloudflareChallenge(t *testing.T) {
	detector := NewWAFResponseDetector(nil)
	body := `<!DOCTYPE html><html><head><title>Just a moment...</title></head></html>`
	resp := &http.Response{
		StatusCode: 503,
		Body:       http.NoBody,
	}
	resp.Body = newStringBody(body)
	dur := detector.Detect(resp)
	if dur != 5*time.Minute {
		t.Errorf("expected 5min freeze for CF challenge, got %v", dur)
	}
}

func TestWAFDetector_RateLimitText(t *testing.T) {
	detector := NewWAFResponseDetector(nil)
	body := `<html><body>请求过于频繁，请稍后再试</body></html>`
	resp := &http.Response{
		StatusCode: 200,
		Body:       http.NoBody,
	}
	resp.Body = newStringBody(body)
	dur := detector.Detect(resp)
	if dur != 60*time.Second {
		t.Errorf("expected 60s freeze for rate limit text, got %v", dur)
	}
}

func TestWAFDetector_LoginRedirect_NoFreeze(t *testing.T) {
	detector := NewWAFResponseDetector(nil)
	body := `<html><form action="/login" method="post"><input name="username"/></form></html>`
	resp := &http.Response{
		StatusCode: 200,
		Body:       http.NoBody,
	}
	resp.Body = newStringBody(body)
	dur := detector.Detect(resp)
	if dur != 0 {
		t.Errorf("expected 0 freeze for login redirect, got %v", dur)
	}
}

func TestWAFDetector_EmptyResponse(t *testing.T) {
	detector := NewWAFResponseDetector(nil)
	resp := &http.Response{
		StatusCode: 200,
		Body:       newStringBody("   "),
	}
	dur := detector.Detect(resp)
	if dur != 60*time.Second {
		t.Errorf("expected 60s for empty response, got %v", dur)
	}
}

func TestWAFDetector_NormalResponse(t *testing.T) {
	detector := NewWAFResponseDetector(nil)
	body := `<html><body>Normal page content here</body></html>`
	resp := &http.Response{
		StatusCode: 200,
		Body:       http.NoBody,
	}
	resp.Body = newStringBody(body)
	dur := detector.Detect(resp)
	if dur != 0 {
		t.Errorf("expected 0 for normal response, got %v", dur)
	}
}

func TestTransport_Integration(t *testing.T) {
	limiter := NewDomainRateLimiter(0)
	limiter.SetDomainConfig("test.local", DomainLimitConfig{MaxReqs: 100, WindowSecs: 60})

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount <= 1 {
			w.WriteHeader(200)
			w.Write([]byte("ok"))
		} else {
			w.WriteHeader(429)
		}
	}))
	defer server.Close()

	detector := NewWAFResponseDetector(nil)
	transport := &domainLimiterTransport{
		limiter:  limiter,
		detector: detector,
		emitter:  nil,
		domain:   "test.local",
		base:     http.DefaultTransport,
	}

	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("first request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("first request: expected 200, got %d", resp.StatusCode)
	}

	status := limiter.GetFreezeStatus("test.local")
	if status.Frozen {
		t.Error("should not be frozen after 200 response")
	}

	resp2, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("second request failed: %v", err)
	}
	resp2.Body.Close()

	status = limiter.GetFreezeStatus("test.local")
	if !status.Frozen {
		t.Error("should be frozen after 429 response")
	}
}

func TestTransport_ConcurrentRequests(t *testing.T) {
	limiter := NewDomainRateLimiter(0)
	limiter.SetDomainConfig("concurrent.local", DomainLimitConfig{MaxReqs: 50, WindowSecs: 1})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	transport := &domainLimiterTransport{
		limiter:  limiter,
		detector: NewWAFResponseDetector(nil),
		domain:   "concurrent.local",
		base:     http.DefaultTransport,
	}
	client := &http.Client{Transport: transport, Timeout: 10 * time.Second}

	var wg sync.WaitGroup
	errors := make(chan error, 20)

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := client.Get(server.URL)
			if err != nil {
				errors <- err
				return
			}
			resp.Body.Close()
		}()
	}
	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("concurrent request failed: %v", err)
	}
}

func TestGetDomainStatuses(t *testing.T) {
	limiter := NewDomainRateLimiter(0)
	limiter.Freeze("a.com", 1*time.Hour)
	limiter.Acquire(context.Background(), "b.com")

	statuses := limiter.GetDomainStatuses()
	if !statuses["a.com"].Frozen {
		t.Error("a.com should be frozen")
	}
	if statuses["b.com"].Frozen {
		t.Error("b.com should not be frozen")
	}
}

func TestFreezeEventEmitter_Dedup(t *testing.T) {
	emitter := NewFreezeEventEmitter(nil)
	ch := make(chan FreezeEvent, 10)
	emitter.Subscribe(ch)

	emitter.Emit(FreezeEvent{Domain: "a.com", Reason: "test", Duration: 30 * time.Second, At: time.Now()})
	emitter.Emit(FreezeEvent{Domain: "a.com", Reason: "test2", Duration: 30 * time.Second, At: time.Now()})

	count := 0
	for {
		select {
		case <-ch:
			count++
		default:
			goto done
		}
	}
done:
	if count != 1 {
		t.Errorf("expected 1 notification (dedup), got %d", count)
	}
}

func TestFreezeEventEmitter_DedupExpiry(t *testing.T) {
	emitter := NewFreezeEventEmitter(nil)
	emitter.dedupWindow = 50 * time.Millisecond
	ch := make(chan FreezeEvent, 10)
	emitter.Subscribe(ch)

	emitter.Emit(FreezeEvent{Domain: "a.com", Reason: "test", Duration: 30 * time.Second, At: time.Now()})

	time.Sleep(60 * time.Millisecond)

	emitter.Emit(FreezeEvent{Domain: "a.com", Reason: "test2", Duration: 30 * time.Second, At: time.Now()})

	count := 0
	for {
		select {
		case <-ch:
			count++
		default:
			goto done2
		}
	}
done2:
	if count != 2 {
		t.Errorf("expected 2 notifications (dedup expired), got %d", count)
	}
}

func TestFreezeEventEmitter_ClearDedup(t *testing.T) {
	emitter := NewFreezeEventEmitter(nil)
	ch := make(chan FreezeEvent, 10)
	emitter.Subscribe(ch)

	emitter.Emit(FreezeEvent{Domain: "a.com", Reason: "test", Duration: 30 * time.Second, At: time.Now()})
	emitter.ClearDedup("a.com")
	emitter.Emit(FreezeEvent{Domain: "a.com", Reason: "test2", Duration: 30 * time.Second, At: time.Now()})

	count := 0
	for {
		select {
		case <-ch:
			count++
		default:
			goto done3
		}
	}
done3:
	if count != 2 {
		t.Errorf("expected 2 notifications (dedup cleared), got %d", count)
	}
}

type stringBody struct {
	data []byte
	pos  int
}

func newStringBody(s string) *stringBody {
	return &stringBody{data: []byte(s)}
}

func (sb *stringBody) Read(p []byte) (int, error) {
	if sb.pos >= len(sb.data) {
		return 0, io.EOF
	}
	n := copy(p, sb.data[sb.pos:])
	sb.pos += n
	return n, nil
}

func (sb *stringBody) Close() error { return nil }
