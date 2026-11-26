package imagehost

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"go.uber.org/zap"
)

// 09 刀1 锚：弃全局 DefaultTransport 替换后的三断言（离线驱动——
// §59.338 真实组件法·同包直测 switcher）：
// ① http.DefaultTransport 全程未被改写（进程级零污染——旧方案真 bug）
// ② 走 switcher 的请求经代理（图床代理语义保持）
// ③ 普通请求（DefaultTransport 链）不经代理（无交叉污染）
// 并发形态：代理请求+普通请求+SetProxy 动态切换三方并发——
// switcher 锁保证 -race 干净。
func TestProxySwitcherNoGlobalTransportPollution(t *testing.T) {
	// 代理 server：记录命中数·返回 204（代理语义=转发路径已到）
	var proxyHits atomic.Int32
	proxySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyHits.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer proxySrv.Close()

	// 目标 server：记录「不经代理直连」命中
	var directHits atomic.Int32
	targetSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		directHits.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer targetSrv.Close()

	origDefault := http.DefaultTransport
	mgr := NewManager(zap.NewNop())
	if mgr.proxy == nil {
		t.Fatal("NewManager 应初始化 switcher")
	}

	// 模拟 host：Register 注入形态（一次性 SetTransport·此后只读）
	hostClient := &http.Client{}
	hostClient.Transport = mgr.proxy

	const rounds = 50
	var wg sync.WaitGroup
	wg.Add(3)

	// A: host 请求（应经代理——switcher 已 set）
	go func() {
		defer wg.Done()
		for i := 0; i < rounds; i++ {
			req, _ := http.NewRequestWithContext(context.Background(), "GET", targetSrv.URL, nil)
			resp, err := hostClient.Do(req)
			if err != nil {
				continue
			}
			resp.Body.Close()
		}
	}()

	// B: 普通请求（DefaultTransport 链——不得经代理）
	go func() {
		defer wg.Done()
		for i := 0; i < rounds; i++ {
			req, _ := http.NewRequestWithContext(context.Background(), "GET", targetSrv.URL, nil)
			resp, err := http.DefaultTransport.RoundTrip(req)
			if err != nil {
				continue
			}
			resp.Body.Close()
		}
		if http.DefaultTransport != origDefault {
			t.Errorf("② http.DefaultTransport 被改写（全局污染）")
		}
	}()

	// C: SetProxy 动态切换（开/关并发——switcher 锁的正确性）
	go func() {
		defer wg.Done()
		for i := 0; i < rounds; i++ {
			if i%2 == 0 {
				mgr.SetProxy(proxySrv.URL)
			} else {
				mgr.SetProxy("")
			}
		}
		// 结束态：开启（保证 A 的多数请求在代理态发出）
		mgr.SetProxy(proxySrv.URL)
	}()
	wg.Wait()

	// ① 全程+终态：DefaultTransport 指针不变
	if http.DefaultTransport != origDefault {
		t.Errorf("① http.DefaultTransport 被改写（旧方案的全局污染形态）")
	}

	// ② host 链经代理：代理命中>0（A 与 C 并发——C 结束前部分请求直连
	// 属预期·断言「有经过代理的」而非「全部」）
	if proxyHits.Load() == 0 {
		t.Errorf("③ switcher 链从未经过代理（代理语义丢失）")
	}
	// ③ 普通链不经代理：directHits 至少来自 B 的 rounds 次
	// （targetSrv 同时被 A 直连态命中·只断言下限）
	if directHits.Load() < rounds {
		t.Errorf("④ 普通请求链经代理污染（direct=%d < %d）", directHits.Load(), rounds)
	}
}
