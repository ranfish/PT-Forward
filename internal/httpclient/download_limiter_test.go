package httpclient

import (
	"strings"
	"testing"
	"time"
)

// §59.183: 站点级下载限流覆盖——hourlyLimit<=0 回落全局默认，>0 按站点值。
func TestDownloadLimiter_PerSiteLimit(t *testing.T) {
	l := NewDownloadRateLimiter(0, 95)

	// 站点覆盖 3/hour：第 4 次拒绝，报错文案含站点值
	for i := 0; i < 3; i++ {
		if err := l.AcquireWithLimit("ubits.club", 3); err != nil {
			t.Fatalf("第 %d 次不应拒绝: %v", i+1, err)
		}
	}
	err := l.AcquireWithLimit("ubits.club", 3)
	if err == nil || !strings.Contains(err.Error(), "3/3 per hour") {
		t.Fatalf("第 4 次应按站点限流 3/3 拒绝, got %v", err)
	}

	// 全局默认域：95 次仍放行（0 回落默认）
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
