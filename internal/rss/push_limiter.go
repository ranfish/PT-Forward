package rss

import (
	"sync"
	"time"
)

type PushLimiter struct {
	mu       sync.Mutex
	counters map[string]*pushCounter
	maxPerHour int
}

type pushCounter struct {
	count   int
	resetAt time.Time
}

func NewPushLimiter(maxPerHour int) *PushLimiter {
	return &PushLimiter{
		counters:   make(map[string]*pushCounter),
		maxPerHour: maxPerHour,
	}
}

func (l *PushLimiter) Allow(subscriptionID string) bool {
	if l.maxPerHour <= 0 {
		return true
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	if len(l.counters) > 100 {
		for k, c := range l.counters {
			if now.After(c.resetAt) {
				delete(l.counters, k)
			}
		}
	}

	c, ok := l.counters[subscriptionID]
	if !ok || now.After(c.resetAt) {
		l.counters[subscriptionID] = &pushCounter{
			count:   1,
			resetAt: now.Add(time.Hour),
		}
		return true
	}

	if c.count >= l.maxPerHour {
		return false
	}
	c.count++
	return true
}

func (l *PushLimiter) Reset(subscriptionID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.counters, subscriptionID)
}
