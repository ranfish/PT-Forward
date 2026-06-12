package httpclient

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"time"

	"go.uber.org/zap"
)

var (
	GlobalLimiter        = NewDomainRateLimiter(0.5)
	GlobalEmitter        *FreezeEventEmitter
	GlobalDetector       = NewWAFResponseDetector(nil)
	GlobalCircuitBreaker *circuitBreakerTransport
)

func Init(logger *zap.Logger) {
	GlobalEmitter = NewFreezeEventEmitter(logger)
	GlobalCircuitBreaker = NewCircuitBreakerTransport(nil, DefaultCircuitBreakerConfig(), logger)
}

func IsDomainCircuitOpen(domain string) bool {
	if GlobalCircuitBreaker == nil {
		return false
	}
	return GlobalCircuitBreaker.GetCircuitStatus(domain).State == CircuitOpen
}

func TripDomainCircuit(domain string) {
	if GlobalCircuitBreaker == nil {
		return
	}
	GlobalCircuitBreaker.TripCircuit(domain)
}

type SiteHTTPConfig struct {
	Domain              string
	Timeout             time.Duration
	ProxyURL            string
	SkipSSLVerify       bool
	WAFPatterns         []wafPattern
	MaxIdleConnsPerHost int
	IdleConnTimeout     time.Duration
}

func NewSiteHTTPClient(cfg SiteHTTPConfig) *http.Client {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.MaxIdleConnsPerHost <= 0 {
		cfg.MaxIdleConnsPerHost = 10
	}
	if cfg.IdleConnTimeout <= 0 {
		cfg.IdleConnTimeout = 90 * time.Second
	}

	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:        cfg.MaxIdleConnsPerHost,
		MaxIdleConnsPerHost: cfg.MaxIdleConnsPerHost,
		MaxConnsPerHost:     cfg.MaxIdleConnsPerHost + 5,
		IdleConnTimeout:     cfg.IdleConnTimeout,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.SkipSSLVerify, //nolint:gosec // configurable by user for self-signed certs
		},
		ForceAttemptHTTP2: true,
	}

	if cfg.ProxyURL != "" {
		if pu, err := url.Parse(cfg.ProxyURL); err == nil {
			transport.Proxy = http.ProxyURL(pu)
		}
	}

	detector := GlobalDetector
	if len(cfg.WAFPatterns) > 0 {
		detector = NewWAFResponseDetector(cfg.WAFPatterns)
	}

	return &http.Client{
		Timeout: cfg.Timeout,
		Transport: &domainLimiterTransport{
			limiter:  GlobalLimiter,
			detector: detector,
			emitter:  GlobalEmitter,
			domain:   cfg.Domain,
			base:     buildRetryChain(transport, cfg.Domain),
		},
	}
}

func buildRetryChain(base http.RoundTripper, domain string) http.RoundTripper {
	retry := NewRetryTransport(base, DefaultRetryConfig(), nil)

	cb := GlobalCircuitBreaker
	if cb == nil {
		return retry
	}
	return &circuitBreakerTransport{
		base:     retry,
		config:   cb.config,
		logger:   cb.logger,
		mu:       cb.mu,
		circuits: cb.circuits,
	}
}
