package httpclient

import (
	"context"
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

	// §59.319 附二十四: IPv4 优先拨号——部分环境 IPv6 TCP 通但 HTTPS 数据
	// 不通（28/12 实测 pt.keepfrds.com Cloudflare：TLS 握手成功 0 字节），
	// Go net/http Happy Eyeballs 可能选中 IPv6 后挂死 → empty_response →
	// 域名冻结。自定义 resolver 过滤 AAAA，仅 A 记录可用时直用，无 A 时
	// 回落原始列表（不破坏纯 IPv6 站点）。
	transport := &http.Transport{
		DialContext: dialContextIPv4First((&net.Dialer{
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

// dialContextIPv4First §59.319 附二十四: 包装 DialContext——拨号前将
// 地址列表过滤为 IPv4 优先（有 A 记录时去掉 AAAA，无 A 时保持原样）。
// 根因：部分环境 IPv6 路径 TCP/TLS 通但应用层数据不通（Cloudflare
// 边缘节点半开），Go Happy Eyeballs 竞速可能选中 IPv6 后挂死。
// 不破坏纯 IPv6 站点（无 A 记录时不过滤）。
func dialContextIPv4First(dial func(ctx context.Context, network, addr string) (net.Conn, error)) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return dial(ctx, network, addr)
		}
		// 已经是 IP 直连（非域名）——检查是否 IPv6
		if ip := net.ParseIP(host); ip != nil {
			if ip.To4() != nil || network == "tcp4" {
				return dial(ctx, network, addr)
			}
			// IPv6 字面量——查 A 记录回落
			addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err == nil {
				if v4 := filterIPv4(addrs); len(v4) > 0 {
					return dial(ctx, "tcp4", net.JoinHostPort(v4[0].String(), port))
				}
			}
			return dial(ctx, network, addr) // 无 IPv4——原样
		}

		// 域名——解析并过滤
		addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return dial(ctx, network, addr) // DNS 失败——走原始 dial（错误一致）
		}
		v4 := filterIPv4(addrs)
		if len(v4) == 0 {
			return dial(ctx, network, addr) // 纯 IPv6 站点——不过滤
		}
		// 有 IPv4——强制 tcp4 直连首个 A 记录
		return dial(ctx, "tcp4", net.JoinHostPort(v4[0].String(), port))
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
