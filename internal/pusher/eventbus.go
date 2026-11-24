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

// EventBusDrainTimeout §59.563 刀B: 停机排空 join 上限——srv.Shutdown 后
// Close→dispatch 排空缓冲的观测窗口（超时放行·进程退出兜底）。导出供
// main 停机序消费（跨包命名常量·附录A §3）。
const EventBusDrainTimeout = 10 * time.Second

type EventBus struct {
	mu              sync.RWMutex
	consumers       []Consumer
	eventCh         chan *PushedEvent
	logger          *zap.Logger
	baseCtx         context.Context
	consumerTimeout time.Duration
	closed          bool          // §59.563 刀B: Close 后 Publish 守卫（send-on-closed panic 消除）
	dispatchDone    chan struct{} // §59.563 刀B: dispatch 排空完成观测点
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
		dispatchDone:    make(chan struct{}),
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
	// §59.563 刀B: closed 守卫——RLock 与 Close 的 Lock 互斥保证：
	// Close 先等在途 Publish 的 RLock 释放再 close(channel)·置位后
	// 新 Publish 直接丢弃（消除 send-on-closed panic 陷阱·审计 02 §3.1）。
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		b.logger.Warn("event bus closed, dropping push event",
			zap.Uint("client_uid", event.ClientUID),
			zap.String("info_hash", event.InfoHash))
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

// Close §59.563 刀B: 停机排空入口——置位 closed（拒新事件）+close channel
// （dispatch range 排空缓冲后自然退出=排空完成）；幂等可重入。
// 停机序约定：srv.Shutdown 之后调用（HTTP 停·新 Publish 源头已枯竭）。
func (b *EventBus) Close() {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.closed = true
	b.mu.Unlock()
	close(b.eventCh)
}

// Done §59.563 刀B: dispatch 排空完成观测点（Close 后缓冲消费完退出时关闸）。
func (b *EventBus) Done() <-chan struct{} {
	return b.dispatchDone
}

func (b *EventBus) dispatch() {
	defer close(b.dispatchDone)
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
