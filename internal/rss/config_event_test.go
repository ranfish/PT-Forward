package rss

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestConfigEventBus_PublishReceive(t *testing.T) {
	bus := NewConfigEventBus()
	ch := bus.Subscribe()

	bus.Publish(ConfigChangedEvent{
		ChangedKeys: []string{"rss_interval"},
		Version:     1,
	})

	select {
	case evt := <-ch:
		require.Equal(t, []string{"rss_interval"}, evt.ChangedKeys)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out")
	}
}

func TestConfigEventBus_VersionDedup(t *testing.T) {
	bus := NewConfigEventBus()
	ch := bus.Subscribe()

	bus.Publish(ConfigChangedEvent{Version: 2, ChangedKeys: []string{"a"}})
	bus.Publish(ConfigChangedEvent{Version: 1, ChangedKeys: []string{"b"}})
	bus.Publish(ConfigChangedEvent{Version: 2, ChangedKeys: []string{"c"}})

	var received []ConfigChangedEvent
	timeout := time.After(500 * time.Millisecond)
	for len(received) < 2 {
		select {
		case evt := <-ch:
			received = append(received, evt)
		case <-timeout:
			goto done
		}
	}
done:

	require.Equal(t, 1, len(received), "version 1 should be deduped")
	require.Equal(t, []string{"a"}, received[0].ChangedKeys)
}

func TestConfigEventBus_Debounce(t *testing.T) {
	bus := NewConfigEventBus()
	ch := bus.Subscribe()

	bus.Publish(ConfigChangedEvent{Version: 1, ChangedKeys: []string{"a"}})
	bus.Publish(ConfigChangedEvent{Version: 2, ChangedKeys: []string{"b"}})

	select {
	case evt := <-ch:
		require.Equal(t, int64(2), evt.Version, "should get last debounced version")
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out")
	}

	select {
	case <-ch:
		t.Fatal("should only get one debounced event")
	case <-time.After(300 * time.Millisecond):
	}
}

func TestConfigEventBus_ZeroVersionAlwaysPasses(t *testing.T) {
	bus := NewConfigEventBus()
	ch := bus.Subscribe()

	bus.Publish(ConfigChangedEvent{Version: 0, ChangedKeys: []string{"a"}})

	select {
	case evt := <-ch:
		require.Equal(t, []string{"a"}, evt.ChangedKeys)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out")
	}
}
