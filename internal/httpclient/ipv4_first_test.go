package httpclient

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// §59.319 附二十四: IPv4 优先拨号——DNS 有 A 记录时过滤 AAAA
func TestDialContextIPv4First(t *testing.T) {
	// 测试服务器（始终 IPv4）
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	// 用 NewSiteHTTPClient 构造的客户端访问
	client := NewSiteHTTPClient(SiteHTTPConfig{Timeout: 10 * time.Second})
	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("IPv4 服务器应可达: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("status = %d", resp.StatusCode)
	}
}

// filterIPv4 单测
func TestFilterIPv4(t *testing.T) {
	v4a := net.IPAddr{IP: net.ParseIP("192.168.1.1")}
	v4b := net.IPAddr{IP: net.ParseIP("172.16.0.1")}
	v6a := net.IPAddr{IP: net.ParseIP("2606:4700::1")}

	all := []net.IPAddr{v6a, v4a, v4b}
	got := filterIPv4(all)
	if len(got) != 2 {
		t.Errorf("应过滤出 2 个 IPv4, got %d", len(got))
	}

	// 纯 IPv6 列表——返回空（不过滤语义由调用方处理）
	onlyV6 := []net.IPAddr{v6a}
	if got := filterIPv4(onlyV6); len(got) != 0 {
		t.Errorf("纯 IPv6 应返回 0, got %d", len(got))
	}
}

// dialContextIPv4First 对 IP 字面量的处理
func TestDialContextIPv4First_IPLiteral(t *testing.T) {
	ctx := context.Background()
	called := false
	mockDial := func(ctx context.Context, network, addr string) (net.Conn, error) {
		called = true
		// IPv4 字面量——network 应保持 tcp 或 tcp4
		if network != "tcp" && network != "tcp4" {
			t.Errorf("IPv4 字面量 network = %q", network)
		}
		return nil, nil
	}
	fn := dialContextIPv4First(mockDial)
	_, _ = fn(ctx, "tcp", "127.0.0.1:80")
	if !called {
		t.Error("IPv4 字面量应直接走原始 dial")
	}
}
