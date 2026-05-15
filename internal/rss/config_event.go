package rss

import (
	"sync"
	"time"
)

type ConfigChangedEvent struct {
	ChangedKeys []string
	Version     int64
}

type ConfigEventBus struct {
	mu          sync.Mutex
	subscribers []chan ConfigChangedEvent
	debounce    *time.Timer
	lastEvent   ConfigChangedEvent
	version     int64
}

func NewConfigEventBus() *ConfigEventBus {
	return &ConfigEventBus{}
}

func (b *ConfigEventBus) Subscribe() chan ConfigChangedEvent {
	ch := make(chan ConfigChangedEvent, 100)
	b.mu.Lock()
	b.subscribers = append(b.subscribers, ch)
	b.mu.Unlock()
	return ch
}

func (b *ConfigEventBus) Publish(event ConfigChangedEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if event.Version > 0 && event.Version <= b.version {
		return
	}
	b.version = event.Version
	b.lastEvent = event

	if b.debounce != nil {
		b.debounce.Reset(200 * time.Millisecond)
		return
	}
	b.debounce = time.AfterFunc(200*time.Millisecond, func() {
		b.mu.Lock()
		evt := b.lastEvent
		subs := make([]chan ConfigChangedEvent, len(b.subscribers))
		copy(subs, b.subscribers)
		b.mu.Unlock()

		for _, ch := range subs {
			select {
			case ch <- evt:
			default:
			}
		}
	})
}
