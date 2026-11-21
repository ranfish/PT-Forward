package httpclient

import (
	"net/http"
	"testing"
)

// unwrapLayers 剥层序：兼容 circuitBreakerTransport 有无两种形态
// （GlobalCircuitBreaker 由 InitHTTPInfrastructure 建·单测环境可能 nil），
// 返回 domainLimiterTransport 与最终 base。
func unwrapLayers(t *testing.T, tr interface{}) (*domainLimiterTransport, interface{}) {
	t.Helper()
	var inner interface{}
	if cb, ok := tr.(*circuitBreakerTransport); ok {
		inner = cb.base
	} else {
		inner = tr
	}
	rt, ok := inner.(*retryTransport)
	if !ok {
		t.Fatalf("期望 retryTransport 层，实际 %T", inner)
	}
	dl, ok := rt.base.(*domainLimiterTransport)
	if !ok {
		t.Fatalf("期望 domainLimiterTransport 层，实际 %T", rt.base)
	}
	return dl, dl.base
}

// TestSiteHTTPClient_ChromeGoesThroughLayers §59.544 修2 回归锚：
// TLSPretend=chrome 的栈必须走完整层序（chrome 栈裸奔=好大 L2 825 颗全速
// 轰炸触发站方 hang 案根因②）。原实现裸 return newChromeClient——
// chromeTransport 直接作 Transport·limiter/retry/breaker 全部失效。
func TestSiteHTTPClient_ChromeGoesThroughLayers(t *testing.T) {
	c := NewSiteHTTPClient(SiteHTTPConfig{Domain: "chromelayer.example.com", TLSPretend: "chrome"})

	// 断言①：Transport 不得直接是 chromeTransport（裸奔形态）
	if _, isChrome := c.Transport.(*chromeTransport); isChrome {
		t.Fatal("chrome 栈裸奔：Transport 直接是 chromeTransport（§59.544 根因②回归）")
	}

	// 断言②：剥层后 base 应为 chromeTransport（chrome 未绕过 limiter/retry）
	_, base := unwrapLayers(t, c.Transport)
	if _, ok := base.(*chromeTransport); !ok {
		t.Fatalf("期望 base 为 chromeTransport，实际 %T（chrome 未走层序）", base)
	}
}

// TestSiteHTTPClient_ChromeSessionShared §59.544 修2 附：chrome session
// 站点级复用语义不变（同 domain+proxy 复用同一 session·与原 newChromeClient
// 同源）——reseed 并发 worker 共享 session 的既有形态保持。
func TestSiteHTTPClient_ChromeSessionShared(t *testing.T) {
	c1 := NewSiteHTTPClient(SiteHTTPConfig{Domain: "shared.example.com", TLSPretend: "chrome"})
	c2 := NewSiteHTTPClient(SiteHTTPConfig{Domain: "shared.example.com", TLSPretend: "chrome"})

	_, base1 := unwrapLayers(t, c1.Transport)
	_, base2 := unwrapLayers(t, c2.Transport)
	ct1, ok := base1.(*chromeTransport)
	if !ok {
		t.Fatalf("期望 chromeTransport base，实际 %T", base1)
	}
	ct2, ok := base2.(*chromeTransport)
	if !ok {
		t.Fatalf("期望 chromeTransport base，实际 %T", base2)
	}
	if ct1.session != ct2.session {
		t.Fatal("同 domain 的 chrome session 未复用（并发形态回归）")
	}
}

// TestSiteHTTPClient_ChromeLimiterEnforced §59.544 修2：chrome 栈的 limiter
// 通道连通性——domainLimiterTransport 的 base 为 chrome 时 limiter 接线
// 仍然在位（请求配额受站点 RPS 约束·真实限流语义由 domain_limiter_test
// 覆盖，此处锁结构防回归）。
func TestSiteHTTPClient_ChromeLimiterEnforced(t *testing.T) {
	c := NewSiteHTTPClient(SiteHTTPConfig{Domain: "limiter-check.example.com", TLSPretend: "chrome"})
	dl, _ := unwrapLayers(t, c.Transport)
	if dl.limiter == nil {
		t.Fatal("chrome 栈 limiter 未接线（Acquire 通道断裂=限流失效）")
	}
}

// TestSiteHTTPClient_PlainTransportUnchanged §59.544 修2：非 chrome 栈
// 行为不变——base 仍为原生 http.Transport（普通站点零回归）。
func TestSiteHTTPClient_PlainTransportUnchanged(t *testing.T) {
	c := NewSiteHTTPClient(SiteHTTPConfig{Domain: "plain.example.com"})
	_, base := unwrapLayers(t, c.Transport)
	if _, ok := base.(*http.Transport); !ok {
		t.Fatalf("普通栈 base 应为 *http.Transport，实际 %T（普通站点回归）", base)
	}
}
