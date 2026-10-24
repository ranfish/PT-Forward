package httpclient

import (
	"context"
	"sync"
	"time"
)

const defaultMaxConcurrent = 2

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
	concurrentCh       chan struct{}
	maxConcurrent      int
	// §59.340 E: 时间窗连续冻结计数——成功穿插不清零（探究实验实证：
	// 黄牌后站方放行几个请求（consecutiveFreezes 清零）再冻结——
	// 指数退避永远爬不起来，25s 反复出现；10min 内复冻=同一波）。
	freezeStreak int
	lastFreezeAt time.Time
}

type DomainLimitConfig struct {
	MaxReqs       int
	WindowSecs    int
	MaxConcurrent int
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
		mc := defaultMaxConcurrent
		state = &domainState{
			maxReqs:       maxReqs,
			windowSecs:    windowSecs,
			maxConcurrent: mc,
			concurrentCh:  make(chan struct{}, mc),
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
	if cfg.MaxConcurrent > 0 {
		state.maxConcurrent = cfg.MaxConcurrent
		state.concurrentCh = make(chan struct{}, cfg.MaxConcurrent)
	}
	state.mu.Unlock()
}

func (l *DomainRateLimiter) Acquire(ctx context.Context, domain string) error {
	l.mu.Lock()
	state := l.getOrCreate(domain)
	l.mu.Unlock()

	select {
	case state.concurrentCh <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}

	state.mu.Lock()
	for {
		if time.Now().Before(state.frozenUntil) {
			waitDur := time.Until(state.frozenUntil)
			state.mu.Unlock()
			select {
			case <-time.After(waitDur):
			case <-ctx.Done():
				<-state.concurrentCh
				return ctx.Err()
			}
			state.mu.Lock()
			continue
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
					<-state.concurrentCh
					return ctx.Err()
				}
				state.mu.Lock()
				continue
			}
		}

		break
	}

	state.timestamps = append(state.timestamps, time.Now())
	state.mu.Unlock()
	return nil
}

func (l *DomainRateLimiter) Release(domain string) {
	l.mu.Lock()
	state := l.getOrCreate(domain)
	l.mu.Unlock()

	select {
	case <-state.concurrentCh:
	default:
	}
}

func (l *DomainRateLimiter) Freeze(domain string, duration time.Duration) {
	l.mu.Lock()
	state := l.getOrCreate(domain)
	l.mu.Unlock()

	state.mu.Lock()
	state.frozenUntil = time.Now().Add(state.streakDuration(duration))
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
	state.frozenUntil = time.Now().Add(state.streakDuration(actualDuration))
	state.frozenBy = "auto_backoff"
	state.mu.Unlock()
}

// streakDuration §59.340 E: 10min 内重复冻结（含成功穿插）→ 静默升级 3min。
// 探究实验（s3）：黄牌（首冻）先于全面封锁 ~7.5min——25s 基数杯水车薪；
// 成功穿插清零 consecutiveFreezes 致退避爬不起来——时间窗 streak 修复。
// 调用方持有 state.mu。
func (s *domainState) streakDuration(base time.Duration) time.Duration {
	now := time.Now()
	if now.Sub(s.lastFreezeAt) < 10*time.Minute {
		s.freezeStreak++
	} else {
		s.freezeStreak = 1
	}
	s.lastFreezeAt = now
	if s.freezeStreak >= 2 && base < 3*time.Minute {
		return 3 * time.Minute
	}
	return base
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

// MaxFreeRemaining §59.330 P4: 全域冻结状态探测——返回剩余冻结的最大毫秒数
// （0=全部解冻）。批量二轮的探测式等待判据（L2 定案）。
func (l *DomainRateLimiter) MaxFreeRemaining() time.Duration {
	l.mu.Lock()
	states := make([]*domainState, 0, len(l.domains))
	for _, s := range l.domains {
		states = append(states, s)
	}
	l.mu.Unlock()

	maxRemain := time.Duration(0)
	now := time.Now()
	for _, s := range states {
		s.mu.Lock()
		if now.Before(s.frozenUntil) {
			if d := s.frozenUntil.Sub(now); d > maxRemain {
				maxRemain = d
			}
		}
		s.mu.Unlock()
	}
	return maxRemain
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
