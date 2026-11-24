package pusher

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
)

// §59.562 刀2 锚：consumer ctx 取消链与超时边界。
type timeoutTestConsumer struct {
	wg        sync.WaitGroup
	gotCtxErr atomic.Value // consumer 内观察到的 ctx.Err()
}

func (c *timeoutTestConsumer) OnPushed(ctx context.Context, event *PushedEvent) {
	defer c.wg.Done()
	// 模拟慢消费者：睡过 deadline 后检查 ctx 是否已取消
	time.Sleep(80 * time.Millisecond)
	c.gotCtxErr.Store(ctx.Err())
}

// TestEventBusConsumerTimeoutCtx: 超过 consumerTimeout 的消费者——其收到的
// ctx 必须已取消（deadline 到点）——Task ctx 派生链生效的机械证明。
func TestEventBusConsumerTimeoutCtx(t *testing.T) {
	bus := &EventBus{
		eventCh:         make(chan *PushedEvent, 4),
		logger:          zap.NewNop(),
		baseCtx:         context.Background(),
		consumerTimeout: 30 * time.Millisecond, // 测试缩窗（生产=60s 常量）
		dispatchDone:    make(chan struct{}),
	}
	c := &timeoutTestConsumer{}
	bus.Register(c)
	go bus.dispatch()

	c.wg.Add(1)
	bus.Publish(&PushedEvent{ClientUID: 1, InfoHash: "abc"})
	c.wg.Wait()

	err, _ := c.gotCtxErr.Load().(error)
	if err == nil {
		t.Fatal("超过 consumerTimeout 的消费者 ctx 应已取消（deadline）")
	}
}

// TestEventBusBaseCtxCancelPropagation: baseCtx（service ctx）cancel 必须
// 传播到消费者派生 ctx——停机取消链接通的机械证明。
func TestEventBusBaseCtxCancelPropagation(t *testing.T) {
	baseCtx, baseCancel := context.WithCancel(context.Background())
	bus := &EventBus{
		eventCh:         make(chan *PushedEvent, 4),
		logger:          zap.NewNop(),
		baseCtx:         baseCtx,
		consumerTimeout: 10 * time.Second,
		dispatchDone:    make(chan struct{}),
	}

	var gotErr atomic.Value
	var wg sync.WaitGroup
	wg.Add(1)
	bus.Register(consumerFunc(func(ctx context.Context, event *PushedEvent) {
		defer wg.Done()
		baseCancel() // 模拟停机信号
		time.Sleep(10 * time.Millisecond)
		gotErr.Store(ctx.Err()) // 派生 ctx 应随 baseCtx 取消
	}))
	go bus.dispatch()

	bus.Publish(&PushedEvent{ClientUID: 2, InfoHash: "def"})
	wg.Wait()

	err, _ := gotErr.Load().(error)
	if err == nil {
		t.Fatal("baseCtx cancel 应传播到消费者派生 ctx（停机取消链）")
	}
}

// TestEventBusNilCtxFallback: nil ctx 兜底 Background（附录A §2.5）。
func TestEventBusNilCtxFallback(t *testing.T) {
	bus := NewEventBus(nil, zap.NewNop(), 1)
	if bus.baseCtx == nil {
		t.Fatal("nil ctx 应兜底 Background")
	}
}

type consumerFunc func(ctx context.Context, event *PushedEvent)

func (f consumerFunc) OnPushed(ctx context.Context, event *PushedEvent) { f(ctx, event) }

// §59.563 刀B 锚：Close 语义三态。

// TestEventBusPublishAfterCloseNoPanic: Close 后 Publish 丢弃不 panic
// （send-on-closed 陷阱消除——closed 守卫机械证明）。
func TestEventBusPublishAfterCloseNoPanic(t *testing.T) {
	bus := NewEventBus(context.Background(), zap.NewNop(), 4)
	bus.Close()
	bus.Publish(&PushedEvent{ClientUID: 1, InfoHash: "abc"}) // 修复前=panic
	bus.Publish(&PushedEvent{ClientUID: 2, InfoHash: "def"})
}

// TestEventBusCloseDrainsBuffer: Close 排空语义——缓冲内事件全部消费完
// dispatch 才退出（Done 关闸=无丢失）。
func TestEventBusCloseDrainsBuffer(t *testing.T) {
	bus := NewEventBus(context.Background(), zap.NewNop(), 16)
	var consumed atomic.Int64
	bus.Register(consumerFunc(func(ctx context.Context, event *PushedEvent) {
		consumed.Add(1)
	})) // NewEventBus 已自带 dispatch goroutine——勿重复起（双 close 竞态）

	const n = 8
	for i := 0; i < n; i++ {
		bus.Publish(&PushedEvent{ClientUID: uint(i), InfoHash: "h"})
	}
	bus.Close()

	select {
	case <-bus.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("Close 后 dispatch 应排空退出")
	}
	if got := consumed.Load(); got != n {
		t.Fatalf("排空语义：应消费全部 %d 事件, got %d", n, got)
	}
}

// TestEventBusCloseIdempotent: Close 幂等（重复 Close 不 panic）。
func TestEventBusCloseIdempotent(t *testing.T) {
	bus := NewEventBus(context.Background(), zap.NewNop(), 2)
	bus.Close()
	bus.Close() // 幂等
}

// §59.567 刀C 锚：重复投递 at-least-once 语义——同一事件投两次应送达两次
// （消费者幂等是消费方契约·总线不去重不丢失）。
func TestEventBusDuplicatePublishDelivered(t *testing.T) {
	bus := NewEventBus(context.Background(), zap.NewNop(), 16)
	var count atomic.Int64
	bus.Register(consumerFunc(func(ctx context.Context, event *PushedEvent) {
		count.Add(1)
	}))
	ev := &PushedEvent{ClientUID: 7, InfoHash: "dup"}
	bus.Publish(ev)
	bus.Publish(ev)
	bus.Close()
	select {
	case <-bus.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("Close 后 dispatch 应排空退出")
	}
	if got := count.Load(); got != 2 {
		t.Fatalf("重复投递应送达两次（at-least-once）, got %d", got)
	}
}
