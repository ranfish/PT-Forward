package mocks

import (
	"context"

	"github.com/ranfish/pt-forward/internal/model"
)

type EventHandler struct {
	Fn func(ctx context.Context, events []model.TorrentEvent) error
}

func (h *EventHandler) OnTorrents(ctx context.Context, events []model.TorrentEvent) error {
	if h.Fn != nil {
		return h.Fn(ctx, events)
	}
	return nil
}
