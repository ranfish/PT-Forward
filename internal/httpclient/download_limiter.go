package httpclient

import (
	"fmt"
	"sync"
	"time"
)

type DownloadRateLimiter struct {
	mu           sync.Mutex
	entries      map[string]*downloadRateEntry
	minInterval  time.Duration
	hourlyLimit  int
}

type downloadRateEntry struct {
	lastDownload time.Time
	count        int
	windowStart  time.Time
}

func NewDownloadRateLimiter(minInterval time.Duration, hourlyLimit int) *DownloadRateLimiter {
	return &DownloadRateLimiter{
		entries:     make(map[string]*downloadRateEntry),
		minInterval: minInterval,
		hourlyLimit: hourlyLimit,
	}
}

var GlobalDownloadLimiter = NewDownloadRateLimiter(2*time.Second, 95)

func (l *DownloadRateLimiter) Acquire(domain string) error {
	// 无参路径=全局默认（构造值，GlobalDownloadLimiter 为 95）；0=不限是 AcquireWithLimit 的显式语义
	return l.AcquireWithLimit(domain, l.hourlyLimit)
}

// AcquireWithLimit §52.4.3/§59.183: hourlyLimit<=0 不限（仍有 minInterval 兜底，
// ≈2s/次）；站点级值来自 sites.download_hourly_limit（站点管理-详情-网络，默认 95）。
func (l *DownloadRateLimiter) AcquireWithLimit(domain string, hourlyLimit int) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	entry, ok := l.entries[domain]
	if !ok {
		entry = &downloadRateEntry{windowStart: now}
		l.entries[domain] = entry
	}

	if now.Sub(entry.windowStart) >= time.Hour {
		entry.count = 0
		entry.windowStart = now
	}

	if hourlyLimit > 0 && entry.count >= hourlyLimit {
		return fmt.Errorf("download quota exceeded for %s: %d/%d per hour", domain, entry.count, hourlyLimit)
	}

	if !entry.lastDownload.IsZero() {
		elapsed := now.Sub(entry.lastDownload)
		if elapsed < l.minInterval {
			wait := l.minInterval - elapsed
			l.mu.Unlock()
			time.Sleep(wait)
			l.mu.Lock()
		}
	}

	entry.count++
	entry.lastDownload = time.Now()
	return nil
}

func (l *DownloadRateLimiter) GetStatus(domain string) (count int, limit int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry, ok := l.entries[domain]
	if !ok {
		return 0, l.hourlyLimit
	}
	now := time.Now()
	if now.Sub(entry.windowStart) >= time.Hour {
		return 0, l.hourlyLimit
	}
	return entry.count, l.hourlyLimit
}

func (l *DownloadRateLimiter) Reset(domain string) {
	l.mu.Lock()
	delete(l.entries, domain)
	l.mu.Unlock()
}
