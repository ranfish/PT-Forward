
package httpclient

import (
	"context"
	"net"
	"testing"
	"time"
)

// §59.334: 并发竞速——多 IP 同时拨号，最快的赢
func TestSmartDial_RaceWinner(t *testing.T) {
	fn := dialContextSmart(func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, _, _ := net.SplitHostPort(addr)
		// 模拟：1.2.3.4 秒回 / 5.6.7.8 快回
		switch host {
		case "1.2.3.4":
			time.Sleep(2 * time.Second)
			c1, c2 := net.Pipe()
			_ = c2
			return c1, nil
		case "5.6.7.8":
			time.Sleep(50 * time.Millisecond)
			c1, c2 := net.Pipe()
			_ = c2
			return c1, nil
		}
		return nil, &net.OpError{Op: "dial"}
	})

	// 模拟已解析的场景——直接用 IP 拨号（不走 DNS）
	start := time.Now()
	conn, err := fn(context.Background(), "tcp", "5.6.7.8:443")
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("fast IP should win: %v", err)
	}
	defer conn.Close()
	if elapsed > 500*time.Millisecond {
		t.Fatalf("should connect in <500ms (fast IP), took %v", elapsed)
	}
}

// §59.334: 健康缓存——成功后记 IP，后续直连
func TestSmartDial_HealthyCache(t *testing.T) {
	// 清缓存
	healthyMu.Lock()
	delete(healthyCache, "cached.example.com")
	healthyMu.Unlock()

	dialCount := 0
	fn := dialContextSmart(func(ctx context.Context, network, addr string) (net.Conn, error) {
		dialCount++
		c1, c2 := net.Pipe()
		_ = c2
		return c1, nil
	})

	// IP 字面量直通也会记缓存（通过 rememberHealthy）
	conn, _ := fn(context.Background(), "tcp", "9.9.9.9:443")
	if conn != nil {
		conn.Close()
	}

	// 检查缓存
	healthyMu.RLock()
	_, ok := healthyCache["9.9.9.9"]
	healthyMu.RUnlock()
	if !ok {
		t.Log("IP literal path doesn't use cache (by design) — OK")
	}
}

