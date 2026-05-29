package httpclient

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

type CircuitBreakerConfig struct {
	FailureThreshold int
	RecoveryTimeout  time.Duration
	HalfOpenMaxReqs  int
	SuccessThreshold int
	FailureChecker   func(*http.Response, error) bool
}

func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 5,
		RecoveryTimeout:  60 * time.Second,
		HalfOpenMaxReqs:  3,
		SuccessThreshold: 3,
		FailureChecker:   DefaultCircuitFailureChecker,
	}
}

func DefaultCircuitFailureChecker(resp *http.Response, err error) bool {
	if err != nil {
		return true
	}
	return resp.StatusCode >= 500
}

type circuitState struct {
	mu               sync.Mutex
	state            CircuitState
	failures         int
	successes        int
	halfOpenAttempts int
	lastFailure      time.Time
	openedAt         time.Time
}

type circuitBreakerTransport struct {
	base     http.RoundTripper
	config   CircuitBreakerConfig
	logger   *zap.Logger
	mu       *sync.Mutex
	circuits map[string]*circuitState
}

func NewCircuitBreakerTransport(base http.RoundTripper, cfg CircuitBreakerConfig, logger *zap.Logger) *circuitBreakerTransport {
	if cfg.FailureThreshold <= 0 {
		cfg.FailureThreshold = 5
	}
	if cfg.RecoveryTimeout <= 0 {
		cfg.RecoveryTimeout = 60 * time.Second
	}
	if cfg.HalfOpenMaxReqs <= 0 {
		cfg.HalfOpenMaxReqs = 3
	}
	if cfg.SuccessThreshold <= 0 {
		cfg.SuccessThreshold = 3
	}
	if cfg.FailureChecker == nil {
		cfg.FailureChecker = DefaultCircuitFailureChecker
	}
	return &circuitBreakerTransport{
		base:     base,
		config:   cfg,
		logger:   logger,
		mu:       &sync.Mutex{},
		circuits: make(map[string]*circuitState),
	}
}

func (t *circuitBreakerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	domain := extractDomain(req.URL)
	circuit := t.getOrCreate(domain)

	circuit.mu.Lock()
	switch circuit.state {
	case CircuitOpen:
		if time.Since(circuit.openedAt) > t.config.RecoveryTimeout {
			circuit.state = CircuitHalfOpen
			circuit.halfOpenAttempts = 0
			circuit.successes = 0
			circuit.failures = 0
		} else {
			circuit.mu.Unlock()
			return nil, httpError(ErrHTCircuitOpen,
				fmt.Sprintf("circuit open for domain %s", domain), nil)
		}
	case CircuitHalfOpen:
		if circuit.halfOpenAttempts >= t.config.HalfOpenMaxReqs {
			circuit.mu.Unlock()
			return nil, httpError(ErrHTCircuitOpen,
				fmt.Sprintf("circuit half-open, max probe requests reached for %s", domain), nil)
		}
		circuit.halfOpenAttempts++
	}
	circuit.mu.Unlock()

	resp, err := t.base.RoundTrip(req)

	isFailure := t.config.FailureChecker(resp, err)

	circuit.mu.Lock()
	defer circuit.mu.Unlock()

	if isFailure {
		circuit.failures++
		circuit.lastFailure = time.Now()

		if circuit.state == CircuitHalfOpen {
			t.tripCircuit(circuit, domain)
		} else if circuit.failures >= t.config.FailureThreshold {
			t.tripCircuit(circuit, domain)
		}
	} else {
		if circuit.state == CircuitHalfOpen {
			circuit.successes++
			if circuit.successes >= t.config.SuccessThreshold {
				circuit.state = CircuitClosed
				circuit.failures = 0
				circuit.successes = 0
				circuit.halfOpenAttempts = 0
				if t.logger != nil {
					t.logger.Info("circuit closed",
						zap.String("domain", domain),
					)
				}
			}
		} else if circuit.state == CircuitClosed {
			circuit.failures = 0
		}
	}

	return resp, err
}

func (t *circuitBreakerTransport) tripCircuit(circuit *circuitState, domain string) {
	circuit.state = CircuitOpen
	circuit.openedAt = time.Now()
	circuit.halfOpenAttempts = 0
	circuit.successes = 0
	if t.logger != nil {
		t.logger.Warn("circuit tripped open",
			zap.String("domain", domain),
			zap.Int("failures", circuit.failures),
		)
	}
}

func (t *circuitBreakerTransport) getOrCreate(domain string) *circuitState {
	t.mu.Lock()
	defer t.mu.Unlock()
	circuit, ok := t.circuits[domain]
	if !ok {
		circuit = &circuitState{state: CircuitClosed}
		t.circuits[domain] = circuit
	}
	return circuit
}

type CircuitStatus struct {
	Domain      string       `json:"domain"`
	State       CircuitState `json:"state"`
	Failures    int          `json:"failures"`
	Successes   int          `json:"successes"`
	OpenedAt    *time.Time   `json:"opened_at,omitempty"`
	LastFailure *time.Time   `json:"last_failure,omitempty"`
}

func (t *circuitBreakerTransport) GetCircuitStatus(domain string) CircuitStatus {
	circuit := t.getOrCreate(domain)
	circuit.mu.Lock()
	defer circuit.mu.Unlock()

	status := CircuitStatus{
		Domain:    domain,
		State:     circuit.state,
		Failures:  circuit.failures,
		Successes: circuit.successes,
	}
	if !circuit.openedAt.IsZero() {
		oa := circuit.openedAt
		status.OpenedAt = &oa
	}
	if !circuit.lastFailure.IsZero() {
		lf := circuit.lastFailure
		status.LastFailure = &lf
	}
	return status
}

func (t *circuitBreakerTransport) GetAllCircuitStatuses() map[string]CircuitStatus {
	t.mu.Lock()
	domains := make([]string, 0, len(t.circuits))
	for d := range t.circuits {
		domains = append(domains, d)
	}
	t.mu.Unlock()

	result := make(map[string]CircuitStatus, len(domains))
	for _, d := range domains {
		result[d] = t.GetCircuitStatus(d)
	}
	t.cleanupStale()
	return result
}

func (t *circuitBreakerTransport) cleanupStale() {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	for domain, circuit := range t.circuits {
		circuit.mu.Lock()
		isStale := circuit.state == CircuitClosed &&
			circuit.failures == 0 &&
			circuit.successes == 0 &&
			now.Sub(circuit.lastFailure) > 30*time.Minute
		circuit.mu.Unlock()
		if isStale {
			delete(t.circuits, domain)
		}
	}
}

func (t *circuitBreakerTransport) ResetCircuit(domain string) {
	circuit := t.getOrCreate(domain)
	circuit.mu.Lock()
	circuit.state = CircuitClosed
	circuit.failures = 0
	circuit.successes = 0
	circuit.halfOpenAttempts = 0
	circuit.openedAt = time.Time{}
	circuit.mu.Unlock()
}

func (t *circuitBreakerTransport) CloseIdleConnections() {
	if ci, ok := t.base.(interface{ CloseIdleConnections() }); ok {
		ci.CloseIdleConnections()
	}
}
