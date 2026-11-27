package api

import (
	"testing"
	"time"
)

// 18 刀3（v0.0.1671）: WS channel 生命周期端到端锚（V-01 防回归）——
// stale 驱逐（生产 Hub.Run 全链）后再发 ping 不得 panic。
// 原案：stale 路径 close(send) 不关 conn → readPump 存活 ping →
// send on closed channel panic（无 recover 杀进程）。
//
// 阴性对照：恢复 stale 分支 close(c.send) → 本锚 panic FAIL。
func TestWSSendOnClosedStaleEviction(t *testing.T) {
	h := NewHub()
	stop := make(chan struct{})
	go h.Run(stop)
	defer close(stop)

	c := &Client{
		hub:  h,
		send: make(chan *WSMessage, 64),
		done: make(chan struct{}),
	}
	h.register <- c
	time.Sleep(50 * time.Millisecond)

	// 填满 send 缓冲 → Broadcast default → stale → 驱逐
	for i := 0; i < 70; i++ {
		select {
		case c.send <- NewWSMessage("fill", nil):
		default:
		}
	}
	h.Broadcast(NewWSMessage("trigger", nil))
	time.Sleep(150 * time.Millisecond)

	// 驱逐完成：done 已关·send 未关（V-01 根修形态）
	h.mu.RLock()
	n := len(h.clients)
	h.mu.RUnlock()
	if n != 0 {
		t.Fatalf("stale 驱逐后 clients=%d want 0", n)
	}

	// readPump ping 形态（trySend）——不得 panic
	if !c.trySend(NewWSMessage("pong", nil)) {
		// done 已关——trySend 返回 false（发送方退出信号）✓ 无 panic
		t.Log("✓ done 已关——trySend 返回 false（退出信号·无 panic）")
	}
}

// 18 刀3: stop 全客户端关闭路径锚——Run stop 后 trySend 不 panic。
func TestWSStopAllClientsShutdown(t *testing.T) {
	h := NewHub()
	stop := make(chan struct{})
	go h.Run(stop)

	c := &Client{
		hub:  h,
		send: make(chan *WSMessage, 64),
		done: make(chan struct{}),
	}
	h.register <- c
	time.Sleep(50 * time.Millisecond)
	close(stop)
	time.Sleep(100 * time.Millisecond)

	// stop 分支 shutdown+conn.Close 后——trySend 走 done 分支退出
	if c.trySend(NewWSMessage("test", nil)) {
		t.Error("stop 后 trySend 不应成功发送（done 应已关）")
	}
	select {
	case <-c.done:
		// ✓ done 已关
	default:
		t.Error("stop 后 done 应已关闭")
	}
}

// 18 刀3: trySend 语义锚——正常态发送成功/满态丢弃（default 语义保留）。
func TestWSTrySendSemantics(t *testing.T) {
	c := &Client{
		send: make(chan *WSMessage, 2),
		done: make(chan struct{}),
	}
	// 正常态
	if !c.trySend(NewWSMessage("a", nil)) {
		t.Error("正常态应发送成功")
	}
	// 满态（容量 2 已满）——default 丢弃
	if !c.trySend(NewWSMessage("b", nil)) {
		t.Error("满态应返回 true（丢弃语义保留）")
	}
	if len(c.send) != 2 {
		t.Errorf("send 长度=%d want 2（丢弃不增加）", len(c.send))
	}
	// done 关闭后——返回 false（退出信号）
	c.shutdown()
	if c.trySend(NewWSMessage("c", nil)) {
		t.Error("done 关闭后应返回 false（退出信号）")
	}
}
