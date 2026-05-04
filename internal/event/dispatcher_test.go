package event

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

type mockHandler struct {
	called int
	err    error
}

func (h *mockHandler) OnTorrents(_ context.Context, _ []model.TorrentEvent) error {
	h.called++
	return h.err
}

type mockWSBroadcaster struct {
	count atomic.Int32
	last  atomic.Value
}

func (m *mockWSBroadcaster) BroadcastWS(eventType string, payload interface{}) {
	m.count.Add(1)
	m.last.Store(map[string]interface{}{"type": eventType, "payload": payload})
}

func TestDispatcher_RegisterAndDispatch(t *testing.T) {
	d := NewDispatcher(zap.NewNop())

	h1 := &mockHandler{}
	h2 := &mockHandler{}
	d.Register("rss", h1)
	d.Register("rss", h2)

	events := []model.TorrentEvent{{TorrentID: "1"}, {TorrentID: "2"}}
	d.Dispatch(context.Background(), "rss", events)

	if h1.called != 1 {
		t.Errorf("h1 called %d times, want 1", h1.called)
	}
	if h2.called != 1 {
		t.Errorf("h2 called %d times, want 1", h2.called)
	}
}

func TestDispatcher_NoHandlers(t *testing.T) {
	d := NewDispatcher(zap.NewNop())
	err := d.Dispatch(context.Background(), "unknown", nil)
	if err != nil {
		t.Errorf("expected nil for no handlers, got %v", err)
	}
}

func TestDispatcher_HandlerError(t *testing.T) {
	d := NewDispatcher(zap.NewNop())

	h1 := &mockHandler{err: context.DeadlineExceeded}
	h2 := &mockHandler{}
	d.Register("test", h1)
	d.Register("test", h2)

	err := d.Dispatch(context.Background(), "test", nil)
	if err == nil {
		t.Error("expected error when handler fails")
	}
	if h2.called != 1 {
		t.Error("second handler should still be called")
	}
}

func TestDispatcher_HandlerCount(t *testing.T) {
	d := NewDispatcher(zap.NewNop())
	if d.HandlerCount("rss") != 0 {
		t.Error("should be 0")
	}

	d.Register("rss", &mockHandler{})
	d.Register("rss", &mockHandler{})
	if d.HandlerCount("rss") != 2 {
		t.Errorf("expected 2, got %d", d.HandlerCount("rss"))
	}
}

func TestDispatcher_WSBroadcast(t *testing.T) {
	d := NewDispatcher(zap.NewNop())
	ws := &mockWSBroadcaster{}
	d.SetWSBroadcaster(ws)

	events := []model.TorrentEvent{
		{ID: 1, SiteName: "site1", TorrentID: "t1", Title: "Test", Size: 1024},
		{ID: 2, SiteName: "site2", TorrentID: "t2", Title: "Test2", Size: 2048},
	}
	d.Dispatch(context.Background(), "rss", events)

	if ws.count.Load() != 2 {
		t.Errorf("expected 2 WS broadcasts, got %d", ws.count.Load())
	}
}

func TestDispatcher_WSBroadcast_NilEvents(t *testing.T) {
	d := NewDispatcher(zap.NewNop())
	ws := &mockWSBroadcaster{}
	d.SetWSBroadcaster(ws)

	d.Dispatch(context.Background(), "rss", nil)
	if ws.count.Load() != 0 {
		t.Errorf("expected 0 WS broadcasts for nil events, got %d", ws.count.Load())
	}
}

func TestDispatcher_WSBroadcast_EventType(t *testing.T) {
	d := NewDispatcher(zap.NewNop())
	ws := &mockWSBroadcaster{}
	d.SetWSBroadcaster(ws)

	events := []model.TorrentEvent{{ID: 1, SiteName: "s", TorrentID: "t"}}
	d.Dispatch(context.Background(), "rss", events)

	last := ws.last.Load().(map[string]interface{})
	if last["type"] != "torrent.added" {
		t.Errorf("expected torrent.added, got %v", last["type"])
	}
}
