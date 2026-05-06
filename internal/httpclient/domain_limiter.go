package httpclient

import (
	"context"
	"sync"
	"time"
)

type DomainRateLimiter struct {
	mu         sync.Mutex
	domains    map[string]*domainState
	defaultRPS float64
}

type domainState struct {
	mu                 sync.Mutex
	timestamps         []time.Time
	maxReqs            int
	windowSecs         int
	frozenUntil        time.Time
	consecutiveFreezes int
	frozenBy           string
	freezeReason       string
}

type DomainLimitConfig struct {
	MaxReqs    int
	WindowSecs int
}

func NewDomainRateLimiter(defaultRPS float64) *DomainRateLimiter {
	return &DomainRateLimiter{
		domains:    make(map[string]*domainState),
		defaultRPS: defaultRPS,
	}
}

func (l *DomainRateLimiter) getOrCreate(domain string) *domainState {
	state, ok := l.domains[domain]
	if !ok {
		maxReqs := 30
		windowSecs := 60
		if l.defaultRPS > 0 {
			maxReqs = int(l.defaultRPS * 60)
			windowSecs = 60
		}
		state = &domainState{
			maxReqs:    maxReqs,
			windowSecs: windowSecs,
		}
		l.domains[domain] = state
	}
	return state
}

func (l *DomainRateLimiter) SetDomainConfig(domain string, cfg DomainLimitConfig) {
	l.mu.Lock()
	state := l.getOrCreate(domain)
	l.mu.Unlock()

	state.mu.Lock()
	if cfg.MaxReqs > 0 {
		state.maxReqs = cfg.MaxReqs
	}
	if cfg.WindowSecs > 0 {
		state.windowSecs = cfg.WindowSecs
	}
	state.mu.Unlock()
}

func (l *DomainRateLimiter) Acquire(ctx context.Context, domain string) error {
	l.mu.Lock()
	state := l.getOrCreate(domain)
	l.mu.Unlock()

	state.mu.Lock()

	now := time.Now()

	if now.Before(state.frozenUntil) {
		waitDur := time.Until(state.frozenUntil)
		state.mu.Unlock()
		select {
		case <-time.After(waitDur):
		case <-ctx.Done():
			return ctx.Err()
		}
		state.mu.Lock()
	}

	cutoff := time.Now().Add(-time.Duration(state.windowSecs) * time.Second)
	valid := state.timestamps[:0]
	for _, ts := range state.timestamps {
		if ts.After(cutoff) {
			valid = append(valid, ts)
		}
	}
	state.timestamps = valid

	if len(state.timestamps) >= state.maxReqs {
		waitDur := state.timestamps[0].Sub(cutoff)
		if waitDur > 0 {
			state.mu.Unlock()
			select {
			case <-time.After(waitDur):
			case <-ctx.Done():
				return ctx.Err()
			}
			state.mu.Lock()
		}
	}

	state.timestamps = append(state.timestamps, time.Now())
	state.mu.Unlock()
	return nil
}

func (l *DomainRateLimiter) Freeze(domain string, duration time.Duration) {
	l.mu.Lock()
	state := l.getOrCreate(domain)
	l.mu.Unlock()

	state.mu.Lock()
	state.frozenUntil = time.Now().Add(duration)
	state.frozenBy = "auto"
	state.mu.Unlock()
}

func (l *DomainRateLimiter) FreezeWithBackoff(domain string, baseDuration time.Duration) {
	l.mu.Lock()
	state := l.getOrCreate(domain)
	l.mu.Unlock()

	state.mu.Lock()
	state.consecutiveFreezes++
	multiplier := 1 << min(state.consecutiveFreezes-1, 5)
	actualDuration := min(baseDuration*time.Duration(multiplier), 10*time.Minute)
	state.frozenUntil = time.Now().Add(actualDuration)
	state.frozenBy = "auto_backoff"
	state.mu.Unlock()
}

func (l *DomainRateLimiter) ResetFreezeCounter(domain string) {
	l.mu.Lock()
	state := l.getOrCreate(domain)
	l.mu.Unlock()

	state.mu.Lock()
	state.consecutiveFreezes = 0
	state.mu.Unlock()
}

func (l *DomainRateLimiter) ManualFreeze(domain string, duration time.Duration, reason string) {
	l.mu.Lock()
	state := l.getOrCreate(domain)
	l.mu.Unlock()

	state.mu.Lock()
	state.frozenUntil = time.Now().Add(duration)
	state.frozenBy = "manual"
	state.freezeReason = reason
	state.mu.Unlock()
}

func (l *DomainRateLimiter) ManualUnfreeze(domain string) {
	l.mu.Lock()
	state := l.getOrCreate(domain)
	l.mu.Unlock()

	state.mu.Lock()
	state.frozenUntil = time.Time{}
	state.frozenBy = ""
	state.freezeReason = ""
	state.consecutiveFreezes = 0
	state.mu.Unlock()
}

type FreezeStatus struct {
	Domain      string     `json:"domain"`
	Frozen      bool       `json:"frozen"`
	FrozenUntil *time.Time `json:"frozen_until,omitempty"`
	FrozenBy    string     `json:"frozen_by,omitempty"`
	Reason      string     `json:"reason,omitempty"`
}

func (l *DomainRateLimiter) GetFreezeStatus(domain string) FreezeStatus {
	l.mu.Lock()
	state := l.getOrCreate(domain)
	l.mu.Unlock()

	state.mu.Lock()
	defer state.mu.Unlock()

	status := FreezeStatus{Domain: domain}
	if time.Now().Before(state.frozenUntil) {
		status.Frozen = true
		fu := state.frozenUntil
		status.FrozenUntil = &fu
		status.FrozenBy = state.frozenBy
		status.Reason = state.freezeReason
	}
	return status
}

func (l *DomainRateLimiter) GetDomainStatuses() map[string]FreezeStatus {
	l.mu.Lock()
	defer l.mu.Unlock()

	result := make(map[string]FreezeStatus, len(l.domains))
	for domain, state := range l.domains {
		state.mu.Lock()
		s := FreezeStatus{Domain: domain}
		if time.Now().Before(state.frozenUntil) {
			s.Frozen = true
			fu := state.frozenUntil
			s.FrozenUntil = &fu
			s.FrozenBy = state.frozenBy
			s.Reason = state.freezeReason
		}
		state.mu.Unlock()
		result[domain] = s
	}
	return result
}
