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
		// §59.333: 伪装域整链替换——azuretls 自带连接池/超时；
		// domainLimiter 冻结语义由外层 fetch 重试链承接（简化：
		// 伪装站点极少，先豁免限流链——观察期后按需补 limiter 桥接）
		s := getAzureSession(cfg.Domain, cfg.ProxyURL) // §59.333 附：代理跟随站点配置
		return &http.Client{
			Timeout:   cfg.Timeout,
			Transport: &azuretlsTransport{session: s},
		}
	}
	return &http.Client{
		Timeout:   cfg.Timeout,
		Transport: inner,
	}
}

// dialContextIPv4First §59.319 附二十四: 包装 DialContext——拨号前将
// 地址列表过滤为 IPv4 优先（有 A 记录时去掉 AAAA，无 A 时保持原样）。
// 根因：部分环境 IPv6 路径 TCP/TLS 通但应用层数据不通（Cloudflare
// 边缘节点半开），Go Happy Eyeballs 竞速可能选中 IPv6 后挂死。
// 不破坏纯 IPv6 站点（无 A 记录时不过滤）。
// dialContextIPv4First §59.319 附二十四修订: IPv4 优先+IPv6 回落——
// 部分环境 IPv6 TCP 通但 HTTPS 数据不通（keepfrds），另一部分 IPv4 不通
// （cspt.top 财神）——单向强制任一族都会破坏另一族。
// 修订：IPv4 先试（3s 短超时），失败回落 IPv6。
func dialContextIPv4First(dial func(ctx context.Context, network, addr string) (net.Conn, error)) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return dial(ctx, network, addr)
		}
		// IP 字面量直通
		if ip := net.ParseIP(host); ip != nil {
			return dial(ctx, network, addr)
		}

		// 域名——解析
		addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return dial(ctx, network, addr)
		}
		v4 := filterIPv4(addrs)
		v6 := filterIPv6(addrs)

		// 纯 IPv6 站点——不过滤
		if len(v4) == 0 {
			return dial(ctx, network, addr)
		}
		// 纯 IPv4——直接用
		if len(v6) == 0 {
			return dial(ctx, "tcp4", net.JoinHostPort(v4[0].String(), port))
		}

		// 双栈——IPv4 先试（3s 短超时），失败回落 IPv6
		v4Ctx, v4Cancel := context.WithTimeout(ctx, 3*time.Second)
		defer v4Cancel()
		conn, v4Err := dial(v4Ctx, "tcp4", net.JoinHostPort(v4[0].String(), port))
		if v4Err == nil {
			return conn, nil
		}
		// IPv4 失败——回落 IPv6
		return dial(ctx, "tcp6", net.JoinHostPort(v6[0].String(), port))
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
