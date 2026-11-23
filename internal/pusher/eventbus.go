package pusher

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// eventBusConsumerTimeout §59.562 刀2: 单消费者超时上限——消费面=推种后处理
// （DB 写为主·60s 足够宽松不误杀）；命名常量·凭观测调值不预拍紧
// （附录A-Context使用规范 §3 超时值约定）。
const eventBusConsumerTimeout = 60 * time.Second

type Consumer interface {
	OnPushed(ctx context.Context, event *PushedEvent)
}

type EventBus struct {
	mu              sync.RWMutex
	consumers       []Consumer
	eventCh         chan *PushedEvent
	logger          *zap.Logger
	baseCtx         context.Context
	consumerTimeout time.Duration
}

// NewEventBus §59.562 刀2: baseCtx=service ctx（main 根 ctx）——停机 cancel
// 自然传播到所有消费者派生 ctx；nil ctx 兜底 Background（附录A §2.5 边界防御）。
func NewEventBus(ctx context.Context, logger *zap.Logger, bufferSize int) *EventBus {
	if bufferSize <= 0 {
		bufferSize = 256
	}
	if ctx == nil {
		ctx = context.Background()
	}
	bus := &EventBus{
		eventCh:         make(chan *PushedEvent, bufferSize),
		logger:          logger,
		baseCtx:         ctx,
		consumerTimeout: eventBusConsumerTimeout,
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
			zap.Uint("client_uid", event.ClientUID),
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
				// §59.562 刀2: Task ctx——service ctx 派生+单消费者超时
				// （审计 01 §3.2：裸 Background 无取消链·单消费者阻塞占用
				// 处理 goroutine 且停机不可感知）；Consumer 契约=尊重 ctx·
				// 超时/取消快速返回（附录A §2.3）。
				ctx, cancel := context.WithTimeout(b.baseCtx, b.consumerTimeout)
				defer cancel()
				start := time.Now()
				consumer.OnPushed(ctx, ev)
				if took := time.Since(start); took >= b.consumerTimeout {
					// 消费者不尊重 ctx 或真超时——结构化证据供观测调值
					b.logger.Warn("event bus consumer exceeded timeout",
						zap.String("consumer", fmt.Sprintf("%T", consumer)),
						zap.Uint("client_uid", ev.ClientUID),
						zap.String("info_hash", ev.InfoHash),
						zap.Duration("took", took),
						zap.Duration("timeout", b.consumerTimeout))
				}
			}(c, event)
		}
	}
}

func (b *EventBus) ConsumerCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.consumers)
}
