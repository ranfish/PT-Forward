package testutil

import (
	"testing"
	"time"
)

// WaitFor §59.567 刀B: 条件轮询等待——替代固定 time.Sleep 的同步等待形态
// （审计 06 §2.2：固定 sleep 在慢 CI/CPU 抢占下偶发失败且难复现）。
// 语义：cond 为真立即返回 true；超时仍假返回 false（调用方断言）。
// 轮询间隔 5ms（本地 goroutine/DB 副作用可观察粒度足够·总耗时由 timeout 主导）。
func WaitFor(t *testing.T, timeout time.Duration, cond func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return cond()
}
