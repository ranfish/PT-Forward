package pusher

import (
	"context"
	"sync"

	"go.uber.org/zap"
)

type Consumer interface {
	OnPushed(ctx context.Context, event *PushedEvent)
}

type EventBus struct {
	mu         sync.RWMutex
	consumers  []Consumer
	eventCh    chan *PushedEvent
	logger     *zap.Logger
	bufferSize int
}

func NewEventBus(logger *zap.Logger, bufferSize int) *EventBus {
	if bufferSize <= 0 {
		bufferSize = 256
	}
	bus := &EventBus{
		eventCh:    make(chan *PushedEvent, bufferSize),
		logger:     logger,
		bufferSize: bufferSize,
	}
	go bus.dispatch()
	return bus
}

func (b *EventBus) Register(c Consumer) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.consumers = append(b.consumers, c)
}

func (b *EventBus) Publish(event *PushedEvent) {
	if event == nil {
		return
	}
	select {
	case b.eventCh <- event:
	default:
		b.logger.Warn("event bus buffer full, dropping push event",
			zap.String("client_id", event.ClientID),
			zap.String("info_hash", event.InfoHash))
	}
}

func (b *EventBus) Close() {
	close(b.eventCh)
}

func (b *EventBus) dispatch() {
	for event := range b.eventCh {
		b.mu.RLock()
		consumers := b.consumers
		b.mu.RUnlock()

		for _, c := range consumers {
			func(consumer Consumer, ev *PushedEvent) {
				defer func() {
					if r := recover(); r != nil {
						b.logger.Error("event bus consumer panic recovered",
							zap.Any("panic", r))
					}
				}()
				consumer.OnPushed(context.Background(), ev)
			}(c, event)
		}
	}
}

func (b *EventBus) ConsumerCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.consumers)
}
