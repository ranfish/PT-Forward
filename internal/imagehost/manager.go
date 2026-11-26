package imagehost

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"go.uber.org/zap"
)

type Manager struct {
	mu          sync.RWMutex
	hosts       map[string]ImageHost
	defaultHost string
	logger      *zap.Logger
	// 代理设置（09 刀1·v0.0.1644 重构）：switcher 显式注入各 host client
	// （Register 时一次性 Transport 注入）——SetProxy 动态切换 switcher 内
	// 部 rt（锁保护·每请求经 RoundTrip 读当前值）。替代 v0.0.256 的
	// 「临时替换 http.DefaultTransport」全局污染方案（并发交叉污染真 bug：
	// 锁外改进程级共享状态·多 withProxy 并发可把全局永久留在图床代理——
	// 09 §2.1）。
	proxy *transportSwitcher
}

// transportSwitcher 09 刀1: 动态代理切换 RoundTripper——一次注入 host
// client（Transport 字段此后只读·零 race）·SetProxy 只改内部 rt（写锁）·
// 每请求 RoundTrip 读当前 rt（读锁）。nil rt 回落 http.DefaultTransport。
type transportSwitcher struct {
	mu sync.RWMutex
	rt http.RoundTripper
}

func (s *transportSwitcher) RoundTrip(req *http.Request) (*http.Response, error) {
	s.mu.RLock()
	rt := s.rt
	s.mu.RUnlock()
	if rt == nil {
		return http.DefaultTransport.RoundTrip(req)
	}
	return rt.RoundTrip(req)
}

func (s *transportSwitcher) set(rt http.RoundTripper) {
	s.mu.Lock()
	s.rt = rt
	s.mu.Unlock()
}

func NewManager(logger *zap.Logger) *Manager {
	return &Manager{
		hosts:  make(map[string]ImageHost),
		logger: logger,
		// 09 刀1: switcher 常驻（rt 空=回落 DefaultTransport）——
		// Register 注入/SetProxy 动态切换·弃全局替换。
		proxy: &transportSwitcher{},
	}
}

// SetProxy 设置图床代理（v0.0.256）。
// proxyURL 为空时关闭代理（直连）。
// 开启后所有 host 的 HTTP 请求自动走代理（通过临时替换 http.DefaultTransport 实现）。
func (m *Manager) SetProxy(proxyURL string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// 09 刀1: 切/关代理=只改 switcher 内部 rt（写锁）+ 旧 rt
	// CloseIdleConnections（换 transport 时回收旧连接池——09 §3 刀1
	// 要求·v0.0.256 的每次 new Transport 无回收=连接资源悬挂）。
	// hosts 的 client 已在 Register 时注入 switcher·零重注入。
	old := m.currentProxyRT()
	if proxyURL == "" {
		m.proxy.set(nil)
		m.logger.Info("image host proxy disabled")
	} else {
		m.proxy.set(&http.Transport{Proxy: http.ProxyURL(toURL(proxyURL))})
		m.logger.Info("image host proxy enabled", zap.String("proxy", proxyURL))
	}
	if t, ok := old.(*http.Transport); ok {
		t.CloseIdleConnections()
	}
}

// currentProxyRT 09 刀1: 读取 switcher 当前 rt（SetProxy 回收旧值用）。
func (m *Manager) currentProxyRT() http.RoundTripper {
	m.proxy.mu.RLock()
	defer m.proxy.mu.RUnlock()
	return m.proxy.rt
}

// Register 09 刀1: 注册时一次性注入 switcher（Transport 字段此后只读——
// 零并发写 race）·后续 SetProxy 只改 switcher 内部 rt。
func (m *Manager) Register(host ImageHost) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hosts[host.Name()] = host
	if m.proxy != nil {
		host.SetTransport(m.proxy)
	}
}

func (m *Manager) SetDefault(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.hosts[name]; !ok && name != "" {
		return fmt.Errorf("image host %q not registered", name)
	}
	m.defaultHost = name
	return nil
}

func (m *Manager) GetHost(name string) (ImageHost, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if name == "" {
		name = m.defaultHost
	}
	host, ok := m.hosts[name]
	if !ok {
		return nil, fmt.Errorf("image host %q not found", name)
	}
	return host, nil
}

func (m *Manager) DefaultHost() ImageHost {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.defaultHost == "" {
		return nil
	}
	return m.hosts[m.defaultHost]
}

func (m *Manager) Upload(ctx context.Context, data []byte, filename string) (*UploadResult, error) {
	host := m.DefaultHost()
	if host == nil {
		return nil, fmt.Errorf("no default image host configured")
	}
	return host.Upload(ctx, data, filename)
}

func (m *Manager) Rehost(ctx context.Context, sourceURL string) (*UploadResult, error) {
	host := m.DefaultHost()
	if host == nil {
		return nil, fmt.Errorf("no default image host configured")
	}
	return host.Rehost(ctx, sourceURL)
}

func (m *Manager) UploadWithHost(ctx context.Context, hostName string, data []byte, filename string) (*UploadResult, error) {
	host, err := m.GetHost(hostName)
	if err != nil {
		host = m.DefaultHost()
		if host == nil {
			return nil, fmt.Errorf("no image host available")
		}
	}
	return host.Upload(ctx, data, filename)
}

func (m *Manager) RehostWithFallback(ctx context.Context, sourceURL string) (*UploadResult, error) {
	host := m.DefaultHost()
	if host == nil {
		return nil, fmt.Errorf("no image host configured")
	}
	result, firstErr := host.Rehost(ctx, sourceURL)
	if firstErr == nil {
		return result, nil
	}

	m.logger.Warn("default image host rehost failed, trying fallback",
		zap.String("host", host.Name()),
		zap.Error(firstErr))

	m.mu.RLock()
	for name, fallback := range m.hosts {
		if name == host.Name() {
			continue
		}
		m.mu.RUnlock()
		fbResult, fbErr := fallback.Rehost(ctx, sourceURL)
		if fbErr == nil {
			return fbResult, nil
		}
		m.logger.Warn("fallback image host rehost failed",
			zap.String("host", name),
			zap.Error(fbErr))
		m.mu.RLock()
	}
	m.mu.RUnlock()

	return nil, fmt.Errorf("all image hosts failed: %w", firstErr)
}

func (m *Manager) ListHosts() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var names []string
	for name := range m.hosts {
		names = append(names, name)
	}
	return names
}

func (m *Manager) HealthCheckAll(ctx context.Context) map[string]error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	results := make(map[string]error)
	for name, host := range m.hosts {
		results[name] = host.HealthCheck(ctx)
	}
	return results
}
