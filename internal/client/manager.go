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
	clients map[string]model.DownloaderClient
}

func NewManager(db *gorm.DB, logger *zap.Logger) *Manager {
	return &Manager{
		db:      db,
		logger:  logger,
		clients: make(map[string]model.DownloaderClient),
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

	activeIDs := make(map[string]bool)
	for _, cfg := range configs {
		activeIDs[cfg.Name] = true

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
			delete(m.clients, cfg.Name)
			continue
		}
		cancel()

		m.clients[cfg.Name] = client
		m.logger.Info("client connected", zap.String("name", cfg.Name), zap.String("type", cfg.Type))
	}

	for name := range m.clients {
		if !activeIDs[name] {
			delete(m.clients, name)
		}
	}

	return nil
}

func (m *Manager) Get(clientID string) (model.DownloaderClient, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	c, ok := m.clients[clientID]
	if !ok {
		return nil, clientError(ErrClientConnection, fmt.Sprintf("client %q not found or not connected", clientID), nil)
	}
	return c, nil
}

func (m *Manager) ListClients() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.clients))
	for name := range m.clients {
		names = append(names, name)
	}
	return names
}

func (m *Manager) GetByDBID(ctx context.Context, id uint) (model.DownloaderClient, *model.ClientConfig, error) {
	var cfg model.ClientConfig
	if err := m.db.WithContext(ctx).
		Where("id = ? AND enabled = ?", id, true).
		First(&cfg).Error; err != nil {
		return nil, nil, clientError(ErrClientConnection, "client config not found", err)
	}

	client, err := m.Get(cfg.Name)
	if err != nil {
		return nil, &cfg, err
	}
	return client, &cfg, nil
}

func (m *Manager) Reload(ctx context.Context) error {
	return m.LoadClients(ctx)
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

func (m *Manager) PingAll(ctx context.Context) {
	m.mu.RLock()
	clients := make(map[string]model.DownloaderClient, len(m.clients))
	for k, v := range m.clients {
		clients[k] = v
	}
	m.mu.RUnlock()

	for id, c := range clients {
		if ctx.Err() != nil {
			return
		}

		if checker, ok := c.(ipBannedChecker); ok && checker.IsIPBanned() {
			m.logger.Warn("client IP banned, attempting reconnect",
				zap.String("clientID", id),
			)
			if connector, ok := c.(connecter); ok {
				if err := connector.Connect(ctx); err != nil {
					m.logger.Warn("client reconnect failed, IP still banned",
						zap.String("clientID", id),
						zap.Error(err),
					)
					continue
				}
				m.logger.Info("client reconnect succeeded after IP ban",
					zap.String("clientID", id),
				)
			}
			continue
		}

		if _, err := c.GetMainData(ctx); err != nil {
			m.logger.Warn("client ping failed", zap.String("clientID", id), zap.Error(err))
		}
	}
}

func (m *Manager) GetTorrentInfo(ctx context.Context, clientID uint, infoHash string) (*model.TorrentInfo, error) {
	dl, _, err := m.GetByDBID(ctx, clientID)
	if err != nil {
		return nil, err
	}
	return dl.GetTorrentByHash(ctx, infoHash)
}
