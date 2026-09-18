package client

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ranfish/pt-forward/internal/client/qbittorrent"
	"github.com/ranfish/pt-forward/internal/client/transmission"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Manager struct {
	db      *gorm.DB
	logger  *zap.Logger
	mu      sync.RWMutex
	clients map[uint]model.DownloaderClient // §59.251: 键=恒定 DB ID（名字仅显示）
}

func NewManager(db *gorm.DB, logger *zap.Logger) *Manager {
	return &Manager{
		db:      db,
		logger:  logger,
		clients: make(map[uint]model.DownloaderClient),
	}
}

func (m *Manager) LoadClients(ctx context.Context) error {
	var configs []model.ClientConfig
	if err := m.db.WithContext(ctx).
		Where("enabled = ?", true).
		Find(&configs).Error; err != nil {
		return clientError(ErrClientConfigParse, "load client configs", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	activeIDs := make(map[uint]bool)
	for _, cfg := range configs {
		activeIDs[cfg.ID] = true

		paths := m.loadPaths(cfg.ID)
		client, err := m.createClient(&cfg, paths)
		if err != nil {
			m.logger.Warn("failed to create client",
				zap.String("name", cfg.Name),
				zap.String("type", cfg.Type),
				zap.Error(err),
			)
			continue
		}

		connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		if connected, err := m.connectClient(connectCtx, client); !connected {
			m.logger.Warn("failed to connect client, skipping",
				zap.String("name", cfg.Name),
				zap.Error(err),
			)
			cancel()
			delete(m.clients, cfg.ID)
			continue
		}
		cancel()

		m.clients[cfg.ID] = client
		m.logger.Info("client connected", zap.String("name", cfg.Name), zap.String("type", cfg.Type))
	}

	for id := range m.clients {
		if !activeIDs[id] {
			if old, ok := m.clients[id]; ok {
				old.Close()
			}
			delete(m.clients, id)
		}
	}

	return nil
}

// Get §59.251: 按恒定 DB ID 获取（名字键废弃）
func (m *Manager) Get(clientUID uint) (model.DownloaderClient, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	c, ok := m.clients[clientUID]
	if !ok {
		return nil, clientError(ErrClientConnection, fmt.Sprintf("client %d not found or not connected", clientUID), nil)
	}
	return c, nil
}

func (m *Manager) IsConnected(id uint) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.clients[id]
	return ok
}

func (m *Manager) ListClients() []uint {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]uint, 0, len(m.clients))
	for id := range m.clients {
		ids = append(ids, id)
	}
	return ids
}

func (m *Manager) GetByDBID(ctx context.Context, id uint) (model.DownloaderClient, *model.ClientConfig, error) {
	var cfg model.ClientConfig
	if err := m.db.WithContext(ctx).
		Where("id = ? AND enabled = ?", id, true).
		First(&cfg).Error; err != nil {
		return nil, nil, clientError(ErrClientConnection, "client config not found", err)
	}

	client, err := m.Get(cfg.ID)
	if err != nil {
		return nil, &cfg, err
	}
	return client, &cfg, nil
}

func (m *Manager) Reload(ctx context.Context) error {
	return m.LoadClients(ctx)
}

// ReloadClient 重连单个客户端（编辑下载器时用，避免全量 Reload 阻塞其他客户端）
func (m *Manager) ReloadClient(ctx context.Context, id uint) error {
	var cfg model.ClientConfig
	if err := m.db.WithContext(ctx).Where("id = ? AND enabled = ?", id, true).First(&cfg).Error; err != nil {
		// 客户端被禁用或删除 → 从池中移除
		m.mu.Lock()
		if old, ok := m.clients[id]; ok {
			old.Close()
			delete(m.clients, id)
		}
		m.mu.Unlock()
		return nil
	}

	paths := m.loadPaths(cfg.ID)
	client, err := m.createClient(&cfg, paths)
	if err != nil {
		m.logger.Warn("reloadClient: failed to create client", zap.Uint("id", id), zap.String("name", cfg.Name), zap.Error(err))
		return err
	}

	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if connected, err := m.connectClient(connectCtx, client); !connected {
		m.logger.Warn("reloadClient: connect failed", zap.Uint("id", id), zap.String("name", cfg.Name), zap.Error(err))
		return err
	}

	m.mu.Lock()
	if old, ok := m.clients[id]; ok {
		old.Close()
	}
	m.clients[id] = client
	m.mu.Unlock()

	m.logger.Info("client reloaded", zap.Uint("id", id), zap.String("name", cfg.Name), zap.String("type", cfg.Type))
	return nil
}

func (m *Manager) ConnectedCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.clients)
}

type connecter interface {
	Connect(ctx context.Context) error
}

func (m *Manager) connectClient(ctx context.Context, client model.DownloaderClient) (bool, error) {
	if c, ok := client.(connecter); ok {
		if err := c.Connect(ctx); err != nil {
			return false, err
		}
	}
	return true, nil
}

func (m *Manager) createClient(cfg *model.ClientConfig, paths []model.SharedPathMapping) (model.DownloaderClient, error) {
	switch cfg.Type {
	case "qbittorrent":
		return qbittorrent.NewQBClient(cfg, paths, m.logger)
	case "transmission":
		return transmission.NewTRClient(cfg, paths, m.logger)
	default:
		return nil, clientError(ErrClientConfigParse, fmt.Sprintf("unsupported client type: %s", cfg.Type), nil)
	}
}

func (m *Manager) loadPaths(clientID uint) []model.SharedPathMapping {
	var mappings []model.ClientPathMapping
	if err := m.db.Where("source_client_id = ?", clientID).Find(&mappings).Error; err != nil {
		m.logger.Warn("failed to load path mappings", zap.Uint("clientID", clientID), zap.Error(err))
		return nil
	}

	result := make([]model.SharedPathMapping, 0, len(mappings))
	for _, mp := range mappings {
		result = append(result, model.SharedPathMapping{
			SourcePath: mp.SourcePath,
			ReseedPath: mp.ReseedPath,
		})
	}
	return result
}

type ipBannedChecker interface {
	IsIPBanned() bool
}

func (m *Manager) HealthCheck(ctx context.Context) {
	var configs []model.ClientConfig
	if err := m.db.WithContext(ctx).Where("enabled = ?", true).Find(&configs).Error; err != nil {
		m.logger.Error("health check: failed to load configs", zap.Error(err))
		return
	}

	activeIDs := make(map[uint]bool, len(configs))

	for i := range configs {
		if ctx.Err() != nil {
			return
		}
		cfg := &configs[i]
		activeIDs[cfg.ID] = true

		m.mu.RLock()
		existing, isConnected := m.clients[cfg.ID]
		m.mu.RUnlock()

		if isConnected {
			m.healthCheckConnected(ctx, cfg.ID, existing)
		} else {
			m.tryReconnect(ctx, cfg)
		}
	}

	m.mu.Lock()
	for id := range m.clients {
		if !activeIDs[id] {
			delete(m.clients, id)
			m.logger.Info("client removed (disabled or deleted)", zap.Uint("id", id))
		}
	}
	m.mu.Unlock()
}

func (m *Manager) healthCheckConnected(ctx context.Context, id uint, c model.DownloaderClient) {
	if checker, ok := c.(ipBannedChecker); ok && checker.IsIPBanned() {
		m.logger.Warn("client IP banned, attempting reconnect", zap.Uint("client_uid", id))
		if connector, ok := c.(connecter); ok {
			connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			if err := connector.Connect(connectCtx); err != nil {
				m.logger.Warn("client reconnect failed, IP still banned",
					zap.Uint("client_uid", id), zap.Error(err))
				cancel()
				return
			}
			m.logger.Info("client reconnect succeeded after IP ban", zap.Uint("client_uid", id))
			cancel()
		}
		return
	}

	if _, err := c.GetMainData(ctx); err != nil {
		m.logger.Warn("client ping failed, attempting reconnect",
			zap.Uint("client_uid", id), zap.Error(err))
		if connector, ok := c.(connecter); ok {
			connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			if err := connector.Connect(connectCtx); err != nil {
				m.logger.Warn("client reconnect failed, removing from pool",
					zap.Uint("client_uid", id), zap.Error(err))
				m.mu.Lock()
				delete(m.clients, id)
				m.mu.Unlock()
			} else {
				m.logger.Info("client reconnected successfully", zap.Uint("client_uid", id))
			}
			cancel()
		}
	}
}

func (m *Manager) tryReconnect(ctx context.Context, cfg *model.ClientConfig) {
	paths := m.loadPaths(cfg.ID)
	client, err := m.createClient(cfg, paths)
	if err != nil {
		m.logger.Warn("health check: failed to create client",
			zap.String("name", cfg.Name), zap.Error(err))
		return
	}

	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	if connected, err := m.connectClient(connectCtx, client); !connected {
		m.logger.Warn("health check: client still unreachable",
			zap.String("name", cfg.Name), zap.Error(err))
		cancel()
		return
	}
	cancel()

	m.mu.Lock()
	m.clients[cfg.ID] = client
	m.mu.Unlock()

	m.logger.Info("client connected (recovered by health check)",
		zap.String("name", cfg.Name), zap.String("type", cfg.Type))
}

func (m *Manager) GetTorrentInfo(ctx context.Context, clientID uint, infoHash string) (*model.TorrentInfo, error) {
	dl, _, err := m.GetByDBID(ctx, clientID)
	if err != nil {
		return nil, err
	}
	return dl.GetTorrentByHash(ctx, infoHash)
}
