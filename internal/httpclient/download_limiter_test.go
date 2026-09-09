package httpclient

import (
	"strings"
	"testing"
	"time"
)

// §59.183: 站点级下载限流——0=不限（§59.183 v2 语义修订），>0=站点限额，回落全局默认 95。
func TestDownloadLimiter_PerSiteLimit(t *testing.T) {
	l := NewDownloadRateLimiter(0, 95)

	// 站点覆盖 3/hour：第 4 次拒绝，报错文案含站点值
	for i := 0; i < 3; i++ {
		if err := l.AcquireWithLimit("ubits.club", 3); err != nil {
			t.Fatalf("第 %d 次不应拒绝: %v", i+1, err)
		}
	}
	if err := l.AcquireWithLimit("ubits.club", 3); err == nil || !strings.Contains(err.Error(), "3/3 per hour") {
		t.Fatalf("第 4 次应按站点限流 3/3 拒绝, got %v", err)
	}

	// 0=不限：100 次全部放行（minInterval=0 前提下）
	for i := 0; i < 100; i++ {
		if err := l.AcquireWithLimit("free.example", 0); err != nil {
			t.Fatalf("不限域第 %d 次不应拒绝: %v", i+1, err)
		}
	}

	// 全局默认：无参 Acquire 95 次放行，第 96 次拒绝
	l2 := NewDownloadRateLimiter(0, 95)
	for i := 0; i < 95; i++ {
		if err := l2.Acquire("default.example"); err != nil {
			t.Fatalf("默认域第 %d 次不应拒绝: %v", i+1, err)
		}
	}
	if err := l2.Acquire("default.example"); err == nil {
		t.Fatal("默认域第 96 次应拒绝")
	}

	// 窗口滚动：1 小时后清零
	e := l2.entries["default.example"]
	e.windowStart = time.Now().Add(-time.Hour - time.Minute)
	if err := l2.AcquireWithLimit("default.example", 10); err != nil {
		t.Fatalf("窗口滚动后应放行: %v", err)
	}
}
