package rss

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestSideLoadEventEmitter_SubscribeAndEmit(t *testing.T) {
	emitter := NewSideLoadEventEmitter()
	ch := emitter.Subscribe()

	emitter.Emit(SideLoadEvent{
		TorrentID: "123",
		SiteName:  "test",
		Status:    model.SideLoadCompleted,
	}, nil)

	select {
	case evt := <-ch:
		require.Equal(t, "123", evt.TorrentID)
		require.Equal(t, model.SideLoadCompleted, evt.Status)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestSideLoadEventEmitter_MultipleSubscribers(t *testing.T) {
	emitter := NewSideLoadEventEmitter()
	ch1 := emitter.Subscribe()
	ch2 := emitter.Subscribe()

	emitter.Emit(SideLoadEvent{TorrentID: "456", Status: model.SideLoadFailed}, nil)

	requireReceived := func(ch <-chan SideLoadEvent) {
		select {
		case evt := <-ch:
			require.Equal(t, "456", evt.TorrentID)
		case <-time.After(time.Second):
			t.Fatal("subscriber should receive event")
		}
	}
	requireReceived(ch1)
	requireReceived(ch2)
}

func TestSideLoadEventEmitter_NonBlocking(t *testing.T) {
	emitter := NewSideLoadEventEmitter()
	ch := make(chan SideLoadEvent)
	emitter.subscribers.Store(new(int), ch)

	done := make(chan struct{})
	go func() {
		emitter.Emit(SideLoadEvent{TorrentID: "789"}, zap.NewNop())
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Emit should not block on full channel")
	}
}

func TestStartSideLoadRepeater_DispatchesCompleted(t *testing.T) {
	ch := make(chan SideLoadEvent, 1)
	var dispatched int32

	dispatchFn := func(ctx context.Context, events []model.TorrentEvent) error {
		atomic.AddInt32(&dispatched, 1)
		require.Len(t, events, 1)
		require.Equal(t, "replayed_hash", events[0].InfoHash)
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	StartSideLoadRepeater(ctx, ch, dispatchFn, zap.NewNop())

	ev := &model.TorrentEvent{
		TorrentID: "999",
		InfoHash:  "replayed_hash",
	}
	ev.Metadata = map[string]any{}

	ch <- SideLoadEvent{
		TorrentID:    "999",
		Status:       model.SideLoadCompleted,
		TorrentEvent: ev,
	}

	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&dispatched) == 1
	}, 2*time.Second, 10*time.Millisecond)
}

func TestStartSideLoadRepeater_IgnoresFailed(t *testing.T) {
	ch := make(chan SideLoadEvent, 1)
	var dispatched int32

	dispatchFn := func(ctx context.Context, events []model.TorrentEvent) error {
		atomic.AddInt32(&dispatched, 1)
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	StartSideLoadRepeater(ctx, ch, dispatchFn, nil)

	ch <- SideLoadEvent{
		TorrentID:    "999",
		Status:       model.SideLoadFailed,
		TorrentEvent: &model.TorrentEvent{TorrentID: "999"},
	}

	time.Sleep(100 * time.Millisecond)
	require.Equal(t, int32(0), atomic.LoadInt32(&dispatched), "failed events should not be dispatched")
}
