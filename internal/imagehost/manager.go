package imagehost

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/zap"
)

type Manager struct {
	mu          sync.RWMutex
	hosts       map[string]ImageHost
	defaultHost string
	logger      *zap.Logger
}

func NewManager(logger *zap.Logger) *Manager {
	return &Manager{
		hosts:  make(map[string]ImageHost),
		logger: logger,
	}
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
	result, err := host.Rehost(ctx, sourceURL)
	if err == nil {
		return result, nil
	}

	m.logger.Warn("default image host rehost failed, trying fallback",
		zap.String("host", host.Name()),
		zap.Error(err))

	m.mu.RLock()
	for name, fallback := range m.hosts {
		if name == host.Name() {
			continue
		}
		m.mu.RUnlock()
		result, fbErr := fallback.Rehost(ctx, sourceURL)
		if fbErr == nil {
			return result, nil
		}
		m.logger.Warn("fallback image host rehost failed",
			zap.String("host", name),
			zap.Error(fbErr))
		m.mu.RLock()
	}
	m.mu.RUnlock()

	return nil, fmt.Errorf("all image hosts failed: %w", err)
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
