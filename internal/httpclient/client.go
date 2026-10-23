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

func ResetDomainCircuit(domain string) {
	if GlobalCircuitBreaker == nil {
		return
	}
	GlobalCircuitBreaker.ResetCircuit(domain)
}

type SiteHTTPConfig struct {
	Domain              string
	Timeout             time.Duration
	ProxyURL            string
	SkipSSLVerify       bool
	WAFPatterns         []wafPattern
	MaxIdleConnsPerHost int
	IdleConnTimeout     time.Duration
	// §59.333: TLS/H2 指纹伪装开关（"chrome"=azuretls Chrome spec——
	// keepfrds 案 Ja3/H2 双层 Go 特征被 Cloudflare 识别；空=Go 原生）
	TLSPretend string
}

func NewSiteHTTPClient(cfg SiteHTTPConfig) *http.Client {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.MaxIdleConnsPerHost <= 0 {
		cfg.MaxIdleConnsPerHost = 10
	}
	if cfg.IdleConnTimeout <= 0 {
		// §59.338: 90s→45s——CF/keepfrds 空闲连接寿命 60-120s 随机掐（29
		// 实证空闲 120s 后复用死连接"写成功读挂起"）；45s 主动关闭空闲
		// 连接，永远不复用可能被服务端掐死的连接（重试 transport 兜底
		// 概率性失败）。
		cfg.IdleConnTimeout = 45 * time.Second
	}

	// §59.319 附二十四: IPv4 优先拨号——部分环境 IPv6 TCP 通但 HTTPS 数据
	// 不通（28/12 实测 pt.keepfrds.com Cloudflare：TLS 握手成功 0 字节），
	// Go net/http Happy Eyeballs 可能选中 IPv6 后挂死 → empty_response →
	// 域名冻结。自定义 resolver 过滤 AAAA，仅 A 记录可用时直用，无 A 时
	// 回落原始列表（不破坏纯 IPv6 站点）。
	transport := &http.Transport{
		DialContext: dialContextSmart((&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext),
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

	// §59.327: 层序对调——limiter 下沉 retry 内层（每物理请求 1 配额）。
	// 原层序 limiter{ breaker{ retry{} } } 一次配额被 retry 内部最多
	// 4 次物理请求放大（0.5 RPS 标称实际 2 RPS 峰值——12 环境批量 71%
	// 失败案），且 429 被判定 retryable 形成对抗循环。对调后：
	// breaker 最外（数逻辑失败——与原语义一致）→ retry → limiter
	// （每物理请求 Acquire + detector 首响应即冻结）→ 原始 transport。
	var inner http.RoundTripper = &domainLimiterTransport{
		limiter:  GlobalLimiter,
		detector: detector,
		emitter:  GlobalEmitter,
		domain:   cfg.Domain,
		base:     transport,
	}
	inner = NewRetryTransport(inner, DefaultRetryConfig(), nil)
	if cb := GlobalCircuitBreaker; cb != nil {
		inner = &circuitBreakerTransport{
			base:     inner,
			config:   cb.config,
			logger:   cb.logger,
			mu:       cb.mu,
			circuits: cb.circuits,
		}
	}
	if cfg.TLSPretend != "" {
		// §59.334 附二: Chrome 指纹传输层——拆解 azuretls 分层组合
		// （智能拨号 + Chrome TLS/H2 指纹完全叠加，替代 azuretlsTransport）
		return &http.Client{
			Timeout:   cfg.Timeout,
			Transport: &chromeTransport{session: getChromeSession(cfg.Domain, cfg.ProxyURL), domain: cfg.Domain},
		}
	}
	return &http.Client{
		Timeout:   cfg.Timeout,
		Transport: inner,
	}
}


// filterIPv4 从地址列表提取 IPv4（To4 非 nil）。
func filterIPv4(addrs []net.IPAddr) []net.IPAddr {
	var out []net.IPAddr
	for _, a := range addrs {
		if a.IP.To4() != nil {
			out = append(out, a)
		}
	}
	return out
}

// filterIPv6 从地址列表提取 IPv6（To4 为 nil）。
func filterIPv6(addrs []net.IPAddr) []net.IPAddr {
	var out []net.IPAddr
	for _, a := range addrs {
		if a.IP.To4() == nil {
			out = append(out, a)
		}
	}
	return out
}
