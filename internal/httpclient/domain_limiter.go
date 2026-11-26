package httpclient

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"
)

const defaultMaxConcurrent = 2

// OriginKey 12 刀1（v0.0.1654）: 限流器域键唯一规范化入口——统一全库
// key 口径（V-01 根修）。输入任意形态（裸域名/带 scheme/带路径/带端口），
// 输出 scheme://host 形态（与请求侧 extractOrigin 同一语义）。
// 无 scheme 默认补 https://（PT 站主流；http 站需 Domain 带 scheme 传入）。
//
// 口径分裂史（三套并存·key 永不相等·站点 rps 配置失效）：
//   - factory.RegisterSiteRateLimit: TrimPrefix 去 scheme → 裸 domain；
//   - site_handler/main: 手拼 "https://"+domain；
//   - transport.RoundTrip: extractOrigin → https://host。
//
// 定案：调用方一律传原始 domain 字符串（DB 的 site.Domain 原样），
// DomainRateLimiter 所有入口内部经本函数收敛——禁止任何调用方手拼 key。
func OriginKey(domain string) string {
	d := strings.TrimSpace(domain)
	if d == "" {
		return d
	}
	if !strings.Contains(d, "://") {
		d = "https://" + d
	}
	u, err := url.Parse(d)
	if err != nil || u.Host == "" {
		return d // 解析失败保底原样（非规范但可预期——同输入恒同 key）
	}
	return u.Scheme + "://" + u.Host
}

// SelfCheckOriginKey 12 刀1: 启动期 key 一致性自检——OriginKey 与
// 请求侧 extractOrigin 必须同形态（fail-fast·Init 接线）。
// 关键形态：裸域/带 scheme/带路径/带端口/带 scheme+路径+尾斜杠。
func SelfCheckOriginKey() error {
	type probe struct {
		in   string
		want string
	}
	probes := []probe{
		{"example.com", "https://example.com"},
		{"https://example.com", "https://example.com"},
		{"example.com/path/to/page", "https://example.com"},
		{"https://example.com:8080", "https://example.com:8080"},
		{"https://example.com/path/", "https://example.com"},
		{"http://plain.example.com", "http://plain.example.com"},
	}
	for _, p := range probes {
		if got := OriginKey(p.in); got != p.want {
			return fmt.Errorf("originKey(%q) = %q, want %q", p.in, got, p.want)
		}
	}
	// 与请求侧 extractOrigin 同源断言（幂等性）
	u, err := url.Parse("https://selfcheck.invalid:8443/x")
	if err != nil {
		return err
	}
	if got := OriginKey(extractOrigin(u)); got != extractOrigin(u) {
		return fmt.Errorf("originKey not idempotent on extractOrigin output: %q", got)
	}
	return nil
}

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
	// 12 刀2（v0.0.1654）: 并发槽计数式信号量——热更新只改数值不换
	// 引用（原 concurrentCh 热替换与在途读写不同锁——-race DATA RACE
	// +取消路径从新空 channel 阻塞接收致 goroutine 永久卡死〔V-02·
	// 高危 CVSS 7.5〕）。等待者持有世代唤醒 channel 引用，Release 时
	// close 旧代+开新代 broadcast 唤醒重竞争。
	concurrent    int
	maxConcurrent int
	waitCh        chan struct{}
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
			waitCh:        make(chan struct{}),
		}
		l.domains[domain] = state
	}
	return state
}

// SetDomainConfig 12 刀1: 入口经 OriginKey 收敛（调用方传原始 domain）。
func (l *DomainRateLimiter) SetDomainConfig(domain string, cfg DomainLimitConfig) {
	l.mu.Lock()
	state := l.getOrCreate(OriginKey(domain))
	l.mu.Unlock()

	state.mu.Lock()
	if cfg.MaxReqs > 0 {
		state.maxReqs = cfg.MaxReqs
	}
	if cfg.WindowSecs > 0 {
		state.windowSecs = cfg.WindowSecs
	}
	if cfg.MaxConcurrent > 0 {
		// 12 刀2: 只改数值不换引用——缩容软生效（在途数自然回落），
		// 扩容立即 broadcast 唤醒等待者（原实现换新 channel 丢在途
		// token——新旧两代归还错位·DATA RACE〔V-02〕）。
		state.maxConcurrent = cfg.MaxConcurrent
		state.broadcastLocked()
	}
	state.mu.Unlock()
}

// acquireSlot 12 刀2: 锁内占坑·满则等世代唤醒 channel——所有读写
// （含取消路径）不接触可替换引用，无 channel 阻塞接收面。
func (s *domainState) acquireSlot(ctx context.Context) error {
	for {
		s.mu.Lock()
		if s.concurrent < s.maxConcurrent {
			s.concurrent++
			s.mu.Unlock()
			return nil
		}
		ch := s.waitCh
		s.mu.Unlock()
		select {
		case <-ch:
			// 被唤醒（Release/扩容 broadcast）——重竞争
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// releaseSlot 12 刀2: 锁内减计数+开新代唤醒——与旧实现的差异：
// 非阻塞（无 channel 接收）、归还永远落在当前计数上（无幽灵 token）。
func (s *domainState) releaseSlot() {
	s.mu.Lock()
	if s.concurrent > 0 {
		s.concurrent--
	}
	if s.concurrent < s.maxConcurrent {
		s.broadcastLocked()
	}
	s.mu.Unlock()
}

// broadcastLocked 12 刀2: close 当前世代唤醒所有等待者·开新世代。
// 调用方持有 s.mu。
func (s *domainState) broadcastLocked() {
	ch := s.waitCh
	s.waitCh = make(chan struct{})
	close(ch)
}

// Acquire 12 刀1: 入口经 OriginKey 收敛（V-01——配置桶与请求桶同 key）。
// 12 刀2: 并发槽计数式（V-02——取消路径锁内归还计数，无 channel 阻塞）。
func (l *DomainRateLimiter) Acquire(ctx context.Context, domain string) error {
	l.mu.Lock()
	state := l.getOrCreate(OriginKey(domain))
	l.mu.Unlock()

	if err := state.acquireSlot(ctx); err != nil {
		return err
	}

	state.mu.Lock()
	for {
		if time.Now().Before(state.frozenUntil) {
			waitDur := time.Until(state.frozenUntil)
			state.mu.Unlock()
			select {
			case <-time.After(waitDur):
			case <-ctx.Done():
				state.releaseSlot()
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
					state.releaseSlot()
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

// Release 12 刀1: 入口经 OriginKey 收敛（与 Acquire 同 key）。
// 12 刀2: 计数式归还——锁内减计数+broadcast 唤醒（原实现非阻塞收
// channel token·与热替换交叉产生幽灵/丢失归还〔V-02〕）。
func (l *DomainRateLimiter) Release(domain string) {
	l.mu.Lock()
	state := l.getOrCreate(OriginKey(domain))
	l.mu.Unlock()

	state.releaseSlot()
}

// Freeze 12 刀1: 入口经 OriginKey 收敛。
func (l *DomainRateLimiter) Freeze(domain string, duration time.Duration) {
	l.mu.Lock()
	state := l.getOrCreate(OriginKey(domain))
	l.mu.Unlock()

	state.mu.Lock()
	state.frozenUntil = time.Now().Add(state.streakDuration(duration))
	state.frozenBy = "auto"
	state.mu.Unlock()
}

// FreezeWithBackoff 12 刀1: 入口经 OriginKey 收敛。
func (l *DomainRateLimiter) FreezeWithBackoff(domain string, baseDuration time.Duration) {
	l.mu.Lock()
	state := l.getOrCreate(OriginKey(domain))
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

// ResetFreezeCounter 12 刀1: 入口经 OriginKey 收敛。
func (l *DomainRateLimiter) ResetFreezeCounter(domain string) {
	l.mu.Lock()
	state := l.getOrCreate(OriginKey(domain))
	l.mu.Unlock()

	state.mu.Lock()
	state.consecutiveFreezes = 0
	state.mu.Unlock()
}

// ManualFreeze 12 刀1: 入口经 OriginKey 收敛。
func (l *DomainRateLimiter) ManualFreeze(domain string, duration time.Duration, reason string) {
	l.mu.Lock()
	state := l.getOrCreate(OriginKey(domain))
	l.mu.Unlock()

	state.mu.Lock()
	state.frozenUntil = time.Now().Add(duration)
	state.frozenBy = "manual"
	state.freezeReason = reason
	state.mu.Unlock()
}

// ManualUnfreeze 12 刀1: 入口经 OriginKey 收敛。
func (l *DomainRateLimiter) ManualUnfreeze(domain string) {
	l.mu.Lock()
	state := l.getOrCreate(OriginKey(domain))
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

// GetFreezeStatus 12 刀1: 入口经 OriginKey 收敛。
func (l *DomainRateLimiter) GetFreezeStatus(domain string) FreezeStatus {
	l.mu.Lock()
	state := l.getOrCreate(OriginKey(domain))
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
