package dispatcher

import (
	"sync"
	"time"

	"github.com/ranfish/pt-forward/internal/notification"
	"go.uber.org/zap"
)

type CircuitBreaker struct {
	logger        *zap.Logger
	notifyService *notification.Service
	mu            sync.RWMutex
	failureCounts map[string]int
	openUntil     map[string]time.Time
	threshold     int
	resetTimeout  time.Duration
}

func NewCircuitBreaker(logger *zap.Logger, notifyService *notification.Service) *CircuitBreaker {
	return &CircuitBreaker{
		logger:        logger,
		notifyService: notifyService,
		failureCounts: make(map[string]int),
		openUntil:     make(map[string]time.Time),
		threshold:     5,
		resetTimeout:  5 * time.Minute,
	}
}

func (cb *CircuitBreaker) Allow(key string) bool {
	cb.mu.Lock()
	now := time.Now()
	if len(cb.openUntil) > 50 {
		for k, until := range cb.openUntil {
			if now.After(until) {
				delete(cb.openUntil, k)
				delete(cb.failureCounts, k)
			}
		}
	}
	until, ok := cb.openUntil[key]
	cb.mu.Unlock()
	if ok {
		return now.After(until)
	}
	return true
}

func (cb *CircuitBreaker) RecordSuccess(key string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	delete(cb.failureCounts, key)
	delete(cb.openUntil, key)
}

func (cb *CircuitBreaker) RecordFailure(key string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failureCounts[key]++
	if cb.failureCounts[key] >= cb.threshold {
		cb.openUntil[key] = time.Now().Add(cb.resetTimeout)
		cb.logger.Warn("circuit breaker opened",
			zap.String("key", key),
			zap.Int("failures", cb.failureCounts[key]),
		)
	}
}
