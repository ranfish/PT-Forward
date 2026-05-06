package httpclient

import (
	"net/http"
	"net/url"
	"sync"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/ranfish/pt-forward/internal/model"
)

type FreezeEvent struct {
	Domain   string        `json:"domain"`
	Reason   string        `json:"reason"`
	Duration time.Duration `json:"duration"`
	URL      string        `json:"url"`
	At       time.Time     `json:"at"`
}

type FreezeEventEmitter struct {
	logger       *zap.Logger
	subscribers  []chan FreezeEvent
	db           *gorm.DB
	mu           sync.Mutex
	lastNotified map[string]time.Time
	dedupWindow  time.Duration
}

func NewFreezeEventEmitter(logger *zap.Logger) *FreezeEventEmitter {
	return &FreezeEventEmitter{
		logger:       logger,
		lastNotified: make(map[string]time.Time),
		dedupWindow:  10 * time.Minute,
	}
}

func (e *FreezeEventEmitter) SetDB(db *gorm.DB) {
	e.db = db
}

func (e *FreezeEventEmitter) Subscribe(ch chan FreezeEvent) {
	e.subscribers = append(e.subscribers, ch)
}

func (e *FreezeEventEmitter) Emit(event FreezeEvent) {
	if e.logger != nil {
		e.logger.Warn("domain frozen",
			zap.String("domain", event.Domain),
			zap.String("reason", event.Reason),
			zap.Duration("duration", event.Duration),
			zap.String("url", event.URL),
		)
	}

	if e.db != nil {
		record := &model.FreezeEventRecord{
			Domain:   event.Domain,
			Reason:   event.Reason,
			Duration: int64(event.Duration / time.Millisecond),
			URL:      event.URL,
			At:       event.At,
		}
		if err := e.db.Create(record).Error; err != nil {
			if e.logger != nil {
				e.logger.Error("failed to persist freeze event", zap.Error(err))
			}
		}
	}

	e.mu.Lock()
	last, exists := e.lastNotified[event.Domain]
	shouldNotify := !exists || time.Since(last) > e.dedupWindow
	if shouldNotify {
		e.lastNotified[event.Domain] = time.Now()
	}
	e.mu.Unlock()

	if !shouldNotify {
		return
	}

	for _, ch := range e.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}

func (e *FreezeEventEmitter) ClearDedup(domain string) {
	e.mu.Lock()
	delete(e.lastNotified, domain)
	e.mu.Unlock()
}

type domainLimiterTransport struct {
	limiter  *DomainRateLimiter
	detector *WAFResponseDetector
	emitter  *FreezeEventEmitter
	domain   string
	base     http.RoundTripper
}

func (t *domainLimiterTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	domain := t.domain
	if domain == "" {
		domain = extractDomain(req.URL)
	}

	if err := t.limiter.Acquire(req.Context(), domain); err != nil {
		return nil, httpError(ErrHTRateLimit, "domain rate limit acquire failed", err)
	}

	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	if t.detector != nil {
		freezeDuration := t.detector.Detect(resp)
		if freezeDuration > 0 {
			t.limiter.FreezeWithBackoff(domain, freezeDuration)
			if t.emitter != nil {
				t.emitter.Emit(FreezeEvent{
					Domain:   domain,
					Reason:   t.detector.LastReason(),
					Duration: freezeDuration,
					URL:      req.URL.String(),
					At:       time.Now(),
				})
			}
		} else if t.detector.LastReason() == "" && resp.StatusCode < 400 {
			t.limiter.ResetFreezeCounter(domain)
		}
	}

	return resp, nil
}

func (t *domainLimiterTransport) CloseIdleConnections() {
	if ci, ok := t.base.(interface{ CloseIdleConnections() }); ok {
		ci.CloseIdleConnections()
	}
}

func extractDomain(u *url.URL) string {
	return u.Scheme + "://" + u.Host
}
