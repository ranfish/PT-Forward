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
	// 代理设置（v0.0.256 图床代理开关）
	// proxyTransport 非 nil 时，Upload/Rehost/HealthCheck 临时替换 http.DefaultTransport
	// 使所有 host 内部的 &http.Client{} 自动走代理，无需改 host 代码
	proxyTransport http.RoundTripper
}

func NewManager(logger *zap.Logger) *Manager {
	return &Manager{
		hosts:  make(map[string]ImageHost),
		logger: logger,
	}
}

// SetProxy 设置图床代理（v0.0.256）。
// proxyURL 为空时关闭代理（直连）。
// 开启后所有 host 的 HTTP 请求自动走代理（通过临时替换 http.DefaultTransport 实现）。
func (m *Manager) SetProxy(proxyURL string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if proxyURL == "" {
		m.proxyTransport = nil
		m.logger.Info("image host proxy disabled")
		return
	}
	m.proxyTransport = &http.Transport{
		Proxy: http.ProxyURL(toURL(proxyURL)),
	}
	m.logger.Info("image host proxy enabled", zap.String("proxy", proxyURL))
}

// withProxy 临时替换 DefaultTransport，调用后恢复。
// 图床上传/转存是串行的（for 循环逐张），不存在并发冲突。
func (m *Manager) withProxy(fn func() error) error {
	m.mu.RLock()
	transport := m.proxyTransport
	m.mu.RUnlock()
	if transport == nil {
		return fn()
	}
	original := http.DefaultTransport
	http.DefaultTransport = transport
	defer func() { http.DefaultTransport = original }()
	return fn()
}

func (m *Manager) Register(host ImageHost) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hosts[host.Name()] = host
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
	var result *UploadResult
	var err error
	fnErr := m.withProxy(func() error {
		result, err = host.Upload(ctx, data, filename)
		return err
	})
	if fnErr != nil {
		return nil, fnErr
	}
	return result, nil
}

func (m *Manager) Rehost(ctx context.Context, sourceURL string) (*UploadResult, error) {
	host := m.DefaultHost()
	if host == nil {
		return nil, fmt.Errorf("no default image host configured")
	}
	var result *UploadResult
	var err error
	fnErr := m.withProxy(func() error {
		result, err = host.Rehost(ctx, sourceURL)
		return err
	})
	if fnErr != nil {
		return nil, fnErr
	}
	return result, nil
}

func (m *Manager) UploadWithHost(ctx context.Context, hostName string, data []byte, filename string) (*UploadResult, error) {
	host, err := m.GetHost(hostName)
	if err != nil {
		host = m.DefaultHost()
		if host == nil {
			return nil, fmt.Errorf("no image host available")
		}
	}
	var result *UploadResult
	var upErr error
	fnErr := m.withProxy(func() error {
		result, upErr = host.Upload(ctx, data, filename)
		return upErr
	})
	if fnErr != nil {
		return nil, fnErr
	}
	return result, nil
}

func (m *Manager) RehostWithFallback(ctx context.Context, sourceURL string) (*UploadResult, error) {
	host := m.DefaultHost()
	if host == nil {
		return nil, fmt.Errorf("no image host configured")
	}
	var result *UploadResult
	var firstErr error
	fnErr := m.withProxy(func() error {
		result, firstErr = host.Rehost(ctx, sourceURL)
		return firstErr
	})
	if fnErr == nil {
		return result, nil
	}

	m.logger.Warn("default image host rehost failed, trying fallback",
		zap.String("host", host.Name()),
		zap.Error(fnErr))

	m.mu.RLock()
	for name, fallback := range m.hosts {
		if name == host.Name() {
			continue
		}
		m.mu.RUnlock()
		var fbResult *UploadResult
		var fbErr error
		callErr := m.withProxy(func() error {
			fbResult, fbErr = fallback.Rehost(ctx, sourceURL)
			return fbErr
		})
		if callErr == nil {
			return fbResult, nil
		}
		m.logger.Warn("fallback image host rehost failed",
			zap.String("host", name),
			zap.Error(callErr))
		m.mu.RLock()
	}
	m.mu.RUnlock()

	return nil, fmt.Errorf("all image hosts failed: %w", fnErr)
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
		h := host
		results[name] = m.withProxy(func() error {
			return h.HealthCheck(ctx)
		})
	}
	return results
}
