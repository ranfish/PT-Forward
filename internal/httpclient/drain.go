package httpclient

import (
	"io"
	"net/http"
	"time"
)

const maxDrainSize = 1 << 20

const MaxBodySize = 10 << 20

// DrainBody 排空响应 body 以利连接复用。
// §59.339: 限时排空——HTTP/2 半死流（响应头到达后断流）上 io.Copy 挂死
// 在 h2pipe 的 Cond.Wait 无超时，且挂死时永远到不了下一行 Close →
// pipe 永不关闭 → Cond.Wait 永等（自锁）。243 实证：两个 goroutine 卡
// 28-32 分钟锁死批量 active（409 永锁）。排空是复用优化非必需——
// 5s 超时即 Close 放弃复用（Close 的 pipe.CloseWithError 广播唤醒
// 挂起的 Copy goroutine 退出，无泄漏）。
func DrainBody(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxDrainSize))
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
	}
	_ = resp.Body.Close()
}

func ReadBody(resp *http.Response) ([]byte, error) {
	return io.ReadAll(io.LimitReader(resp.Body, MaxBodySize))
}
