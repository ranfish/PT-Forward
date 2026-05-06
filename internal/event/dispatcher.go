package event

import (
	"context"
	"sync"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

type WSBroadcaster interface {
	BroadcastWS(eventType string, payload interface{})
}

type Dispatcher struct {
	mu          sync.RWMutex
	handlers    map[string][]model.EventHandler
	logger      *zap.Logger
	wsBroadcast WSBroadcaster
}

func NewDispatcher(logger *zap.Logger) *Dispatcher {
	return &Dispatcher{
		handlers: make(map[string][]model.EventHandler),
		logger:   logger,
	}
}

func (d *Dispatcher) SetWSBroadcaster(b WSBroadcaster) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.wsBroadcast = b
}

func (d *Dispatcher) Register(eventType string, handler model.EventHandler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handlers[eventType] = append(d.handlers[eventType], handler)
}

func (d *Dispatcher) Dispatch(ctx context.Context, eventType string, events []model.TorrentEvent) error {
	d.mu.RLock()
	handlers := d.handlers[eventType]
	wsB := d.wsBroadcast
	d.mu.RUnlock()

	if len(handlers) == 0 && wsB == nil {
		return nil
	}

	var lastErr error
	for _, handler := range handlers {
		if err := handler.OnTorrents(ctx, events); err != nil {
			d.logger.Warn("event handler failed",
				zap.String("event_type", eventType),
				zap.Error(err),
			)
			lastErr = eventError(ErrEventHandler, "handler failed", err)
		}
	}

	if wsB != nil {
		for i := range events {
			wsB.BroadcastWS("torrent.added", map[string]interface{}{
				"eventId":     events[i].ID,
				"siteName":    events[i].SiteName,
				"torrentId":   events[i].TorrentID,
				"title":       events[i].Title,
				"size":        events[i].Size,
				"discount":    events[i].Discount,
				"matchedRule": events[i].MatchedRuleName,
			})
		}
	}

	return lastErr
}

func (d *Dispatcher) HandlerCount(eventType string) int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.handlers[eventType])
}
