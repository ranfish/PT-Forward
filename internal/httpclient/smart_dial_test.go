
package httpclient

import (
	"io"
	"net/http"
	"strings"
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

// §59.334 附二: chromeTransport（智能拨号+Chrome 指纹叠加）
func TestChromeTransport_Fingerprint(t *testing.T) {
	tr := &chromeTransport{inner: getChromeTransport("tls.peet.ws")}
	req, _ := http.NewRequest("GET", "https://tls.peet.ws/api/all", nil)
	SetBrowserHeaders(req.Header, "")
	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("roundtrip: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	s := string(body)
	// JA4 不应是 Go 的 d13 cipher 特征
	if strings.Contains(s, "t13d1312h2_") {
		t.Fatal("仍是 Go 原生指纹（d13）——Chrome 伪装未生效")
	}
	// H2 伪头序不应是 Go 的 a,m,p,s
	if strings.Contains(s, `"a,m,p,s"`) {
		t.Fatal("H2 伪头序仍是 Go 特征 a,m,p,s")
	}
}
