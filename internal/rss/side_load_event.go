package rss

import (
	"context"
	"sync"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

type SideLoadEvent struct {
	TorrentID    string
	SiteName     string
	Status       model.SideLoadStatus
	TorrentEvent *model.TorrentEvent
	FailedReason string
}

type SideLoadEventEmitter struct {
	subscribers sync.Map
}

func NewSideLoadEventEmitter() *SideLoadEventEmitter {
	return &SideLoadEventEmitter{}
}

type subHandle struct {
	ch chan SideLoadEvent
}

func (e *SideLoadEventEmitter) Subscribe() chan SideLoadEvent {
	ch := make(chan SideLoadEvent, 100)
	handle := &subHandle{ch: ch}
	e.subscribers.Store(handle, true)
	return ch
}

func (e *SideLoadEventEmitter) Unsubscribe(ch chan SideLoadEvent) {
	e.subscribers.Range(func(key, _ interface{}) bool {
		if h, ok := key.(*subHandle); ok && h.ch == ch {
			e.subscribers.Delete(key)
			return false
		}
		return true
	})
}

func (e *SideLoadEventEmitter) Emit(event SideLoadEvent, logger *zap.Logger) {
	e.subscribers.Range(func(key, _ interface{}) bool {
		h, ok := key.(*subHandle)
		if !ok {
			return true
		}
		select {
		case h.ch <- event:
		default:
			if logger != nil {
				logger.Warn("side load event dropped: subscriber channel full",
					zap.String("site", event.SiteName),
					zap.String("torrent_id", event.TorrentID),
					zap.String("status", string(event.Status)),
				)
			}
		}
		return true
	})
}

func StartSideLoadRepeater(ctx context.Context, eventCh <-chan SideLoadEvent, dispatchFn func(context.Context, []model.TorrentEvent) error, logger *zap.Logger) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-eventCh:
				if !ok {
					return
				}
				if event.Status == model.SideLoadCompleted && event.TorrentEvent != nil {
					ev := event.TorrentEvent
					if ev.Metadata == nil {
						ev.Metadata = make(map[string]any)
					}
					ev.Metadata["source"] = "side_load_replay"
					if err := dispatchFn(ctx, []model.TorrentEvent{*ev}); err != nil {
						if logger != nil {
							logger.Error("side load replay dispatch failed",
								zap.String("site", event.SiteName),
								zap.String("torrent_id", event.TorrentID),
								zap.Error(err),
							)
						}
					}
				}
			}
		}
	}()
}
