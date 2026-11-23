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
