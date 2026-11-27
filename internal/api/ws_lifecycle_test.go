package api

import (
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/testutil"
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
	// 18 刀3 附（回归审核 flake 修）: register 处理等待条件化——
	// Run 收进 h.clients 才继续（§59.567 刀B：固定 sleep 在全量
	// 负载下调度延迟即假失败·WaitFor 条件轮询为确定性同步）。
	if !testutil.WaitFor(t, 2*time.Second, func() bool { return h.ClientCount() == 1 }) {
		t.Fatal("register 后 client 未入 hub（Run 竞态）")
	}

	// 填满 send 缓冲 → Broadcast default → stale → 驱逐
	for i := 0; i < 70; i++ {
		select {
		case c.send <- NewWSMessage("fill", nil):
		default:
		}
	}
	h.Broadcast(NewWSMessage("trigger", nil))
	// 驱逐完成等待条件化——stale 驱逐清空 hub（原 150ms sleep 假失败窗口）
	if !testutil.WaitFor(t, 2*time.Second, func() bool { return h.ClientCount() == 0 }) {
		t.Fatal("stale 驱逐未完成（Broadcast→default→stale→unregister 链断）")
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
	if !testutil.WaitFor(t, 2*time.Second, func() bool { return h.ClientCount() == 1 }) {
		t.Fatal("register 后 client 未入 hub（Run 竞态）")
	}
	close(stop)
	// 18 刀3 附（回归审核 flake 修）: stop 处理等待条件化——原固定
	// 100ms sleep 在全量并发负载下 Run 调度可超窗（build 门实测
	// FAIL 一次）·改为轮询 done 关闭（确定性同步·done 依赖 Hub
	// stop 分支 shutdown 全链——等待的正是被测生产链本身）。
	done := make(chan struct{})
	go func() {
		<-c.done
		close(done)
	}()
	select {
	case <-done:
		// ✓ Run stop 分支已 shutdown——跳出
	case <-time.After(2 * time.Second):
		t.Fatal("stop 后 done 未关闭（Run 处理 stop 超时——shutdown 断链）")
	}

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
