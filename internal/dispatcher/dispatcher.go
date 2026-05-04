package dispatcher

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/ranfish/pt-forward/internal/client"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Role string

const (
	RoleSeeding  Role = "seeding"
	RoleDownload Role = "download"
	RoleSource   Role = "source"
	RoleReseed   Role = "reseed"
)

type TorrentDispatcher struct {
	db        *gorm.DB
	clientMgr *client.Manager
	logger    *zap.Logger
	handlers  map[Role]model.EventHandler
	mu        sync.RWMutex
}

func NewTorrentDispatcher(db *gorm.DB, clientMgr *client.Manager, logger *zap.Logger) *TorrentDispatcher {
	return &TorrentDispatcher{
		db:        db,
		clientMgr: clientMgr,
		logger:    logger,
		handlers:  make(map[Role]model.EventHandler),
	}
}

func (d *TorrentDispatcher) RegisterHandler(role Role, handler model.EventHandler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handlers[role] = handler
}

func (d *TorrentDispatcher) OnTorrents(ctx context.Context, events []model.TorrentEvent) error {
	if len(events) == 0 {
		return nil
	}

	eventsByRole, err := d.enrichAndRoute(ctx, events)
	if err != nil {
		return fmt.Errorf("enrich and route events: %w", err)
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	var firstErr error
	for role, evts := range eventsByRole {
		handler, ok := d.handlers[role]
		if !ok {
			d.logger.Debug("no handler registered for role", zap.String("role", string(role)))
			continue
		}
		if err := handler.OnTorrents(ctx, evts); err != nil {
			d.logger.Error("role handler failed", zap.String("role", string(role)), zap.Error(err))
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	return firstErr
}

func (d *TorrentDispatcher) enrichAndRoute(ctx context.Context, events []model.TorrentEvent) (map[Role][]model.TorrentEvent, error) {
	result := make(map[Role][]model.TorrentEvent)

	for i := range events {
		ev := &events[i]

		subID, err := strconv.ParseUint(ev.SourceID, 10, 64)
		if err != nil {
			d.logger.Debug("event has invalid subscription source_id",
				zap.String("source_id", ev.SourceID),
				zap.String("site", ev.SiteName),
				zap.String("torrent_id", ev.TorrentID),
			)
			continue
		}

		sub, err := d.getSubscription(ctx, uint(subID))
		if err != nil {
			d.logger.Debug("subscription not found, skipping event",
				zap.Uint("subscription_id", uint(subID)),
				zap.String("site", ev.SiteName),
				zap.String("torrent_id", ev.TorrentID),
				zap.Error(err),
			)
			continue
		}

		clientCfg, err := d.getClientConfig(ctx, sub.ClientID)
		if err != nil {
			d.logger.Debug("client config not found, skipping event",
				zap.String("client_id", sub.ClientID),
				zap.String("site", ev.SiteName),
				zap.Error(err),
			)
			continue
		}

		role := Role(clientCfg.Role)

		if ev.Metadata == nil {
			ev.Metadata = make(map[string]any)
		}
		ev.Metadata["client_name"] = sub.ClientID
		ev.Metadata["client_role"] = string(role)
		ev.Metadata["subscription_id"] = ev.SourceID

		result[role] = append(result[role], *ev)
	}

	return result, nil
}

func (d *TorrentDispatcher) getSubscription(ctx context.Context, id uint) (*model.RSSSubscription, error) {
	var sub model.RSSSubscription
	err := d.db.WithContext(ctx).
		Where("id = ? AND deleted_at = ? AND enabled = ?", id, time.Time{}, true).
		First(&sub).Error
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

func (d *TorrentDispatcher) getClientConfig(ctx context.Context, clientName string) (*model.ClientConfig, error) {
	var cfg model.ClientConfig
	err := d.db.WithContext(ctx).
		Where("name = ? AND deleted_at = ? AND enabled = ?", clientName, time.Time{}, true).
		First(&cfg).Error
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

func GetClientName(ev *model.TorrentEvent) string {
	if ev.Metadata != nil {
		if name, ok := ev.Metadata["client_name"].(string); ok {
			return name
		}
	}
	return ev.SourceID
}

func GetClientRole(ev *model.TorrentEvent) string {
	if ev.Metadata != nil {
		if role, ok := ev.Metadata["client_role"].(string); ok {
			return role
		}
	}
	return ""
}

func GetSubscriptionID(ev *model.TorrentEvent) string {
	if ev.Metadata != nil {
		if id, ok := ev.Metadata["subscription_id"].(string); ok {
			return id
		}
	}
	return ev.SourceID
}
