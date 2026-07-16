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

	if entry.count >= l.hourlyLimit {
		return fmt.Errorf("download quota exceeded for %s: %d/%d per hour", domain, entry.count, l.hourlyLimit)
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
