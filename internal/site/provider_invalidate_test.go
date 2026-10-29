package site

import (
	"context"
	"testing"

	"github.com/ranfish/pt-forward/internal/adapter"
	"go.uber.org/zap"
)

// §59.367 附二: adapter 缓存失效——配置变更（proxy 等 doer 构造参数）
// 后必须失效缓存，否则旧直连 doer 持续服务直到重启（§59.333 附同款）。
// 双键语义：GetAdapter 缓存键可能是站名（RSS detect 链）或域名（其它链）
// ——同站双副本，InvalidateAdapter(name, domain) 全删。
func TestProvider_InvalidateAdapterDualKeys(t *testing.T) {
	db := setupProviderDB(t)
	createTestSite(t, db)
	p := NewProvider(db, adapter.NewFactory(zap.NewNop(), nil), zap.NewNop())
	ctx := context.Background()

	// 按站名与域名各取一次——建立双键缓存
	if _, err := p.GetAdapter(ctx, "test-site"); err != nil {
		t.Fatalf("GetAdapter by name: %v", err)
	}
	if _, err := p.GetAdapter(ctx, "test.example.com"); err != nil {
		t.Fatalf("GetAdapter by domain: %v", err)
	}
	if len(p.adapters) != 2 {
		t.Fatalf("同站双键应产生 2 个缓存副本，got %d", len(p.adapters))
	}

	// 双键失效——全删
	p.InvalidateAdapter("test-site", "test.example.com")
	if len(p.adapters) != 0 {
		t.Fatalf("双键失效后缓存应为空，got %d", len(p.adapters))
	}
}

// ResolveSiteProxy 的 use_global_proxy 分支（§59.156 单点已存在，
// §59.367 附二 RSS fetch 链接线的语义锚）。
func TestResolveSiteProxyGlobalFallback(t *testing.T) {
	db := setupProviderDB(t)
	s := createTestSite(t, db)

	// 无代理配置——直连
	if got := ResolveSiteProxy(db, context.Background(), s); got != "" {
		t.Fatalf("无配置应直连，got %q", got)
	}
	// proxy_url 优先
	s.ProxyURL = "http://site-proxy:1"
	if got := ResolveSiteProxy(db, context.Background(), s); got != "http://site-proxy:1" {
		t.Fatalf("proxy_url 应优先，got %q", got)
	}
	// use_global_proxy 回退全局
	s.ProxyURL = ""
	s.UseGlobalProxy = true
	db.Exec("CREATE TABLE IF NOT EXISTS system_settings (key TEXT PRIMARY KEY, value TEXT)")
	db.Exec("INSERT OR REPLACE INTO system_settings (key, value) VALUES ('httpProxy', 'http://global:2')")
	if got := ResolveSiteProxy(db, context.Background(), s); got != "http://global:2" {
		t.Fatalf("use_global_proxy 应回退全局代理，got %q", got)
	}
}
