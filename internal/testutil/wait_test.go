package testutil

import (
	"sync/atomic"
	"testing"
	"time"
)

// §59.567 刀B 锚：WaitFor 三态。
func TestWaitFor(t *testing.T) {
	// ① 立即为真 → 快速返回
	if !WaitFor(t, time.Second, func() bool { return true }) {
		t.Fatal("立即为真应返回 true")
	}
	// ② 超时仍假 → 返回 false（且实际耗时≥timeout）
	start := time.Now()
	if WaitFor(t, 50*time.Millisecond, func() bool { return false }) {
		t.Fatal("恒假应超时返回 false")
	}
	if time.Since(start) < 45*time.Millisecond {
		t.Fatalf("超时应等待约 timeout, took %v", time.Since(start))
	}
	// ③ 期间变真 → 返回 true（模拟异步副作用就绪）
	var n atomic.Int64
	go func() {
		time.Sleep(20 * time.Millisecond)
		n.Store(1)
	}()
	if !WaitFor(t, 2*time.Second, func() bool { return n.Load() == 1 }) {
		t.Fatal("20ms 后变真应在超时前返回 true")
	}
}
