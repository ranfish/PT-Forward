package publish

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
)

func TestCheckImageCompatibility_NoRestriction(t *testing.T) {
	urls := []string{"https://img.example.com/1.jpg", "https://img.example.com/2.jpg"}
	site := &model.Site{}
	compatible, incompatible := CheckImageCompatibility(urls, site)
	if len(incompatible) != 0 {
		t.Errorf("no restriction: all should be compatible, got %d incompatible", len(incompatible))
	}
	if len(compatible) != 2 {
		t.Errorf("expected 2 compatible, got %d", len(compatible))
	}
}

func TestCheckImageCompatibility_Whitelist(t *testing.T) {
	urls := []string{
		"https://pixhost.to/1.jpg",     // 白名单内
		"https://blacklisted.com/2.jpg", // 不在白名单
	}
	site := &model.Site{ImageWhitelist: "pixhost.to,doubanio.com"}
	compatible, incompatible := CheckImageCompatibility(urls, site)
	if len(compatible) != 1 {
		t.Errorf("expected 1 compatible, got %d", len(compatible))
	}
	if len(incompatible) != 1 {
		t.Errorf("expected 1 incompatible, got %d", len(incompatible))
	}
}

func TestCheckImageCompatibility_Blacklist(t *testing.T) {
	urls := []string{
		"https://bad.com/1.jpg",   // 黑名单内
		"https://good.com/2.jpg",  // 不在黑名单
	}
	site := &model.Site{ImageBlacklist: "bad.com,evil.com"}
	compatible, incompatible := CheckImageCompatibility(urls, site)
	if len(compatible) != 1 {
		t.Errorf("expected 1 compatible, got %d", len(compatible))
	}
	if len(incompatible) != 1 {
		t.Errorf("expected 1 incompatible, got %d", len(incompatible))
	}
}

func TestCheckImageCompatibility_SubdomainMatch(t *testing.T) {
	urls := []string{
		"https://cdn.pixhost.to/1.jpg",  // 子域名应匹配
		"https://other.com/2.jpg",
	}
	site := &model.Site{ImageWhitelist: "pixhost.to"}
	compatible, _ := CheckImageCompatibility(urls, site)
	if len(compatible) != 1 {
		t.Errorf("subdomain should match whitelist, got %d compatible", len(compatible))
	}
}

func TestCheckImageCompatibility_NilSite(t *testing.T) {
	urls := []string{"https://example.com/1.jpg"}
	compatible, incompatible := CheckImageCompatibility(urls, nil)
	if len(compatible) != 1 || len(incompatible) != 0 {
		t.Errorf("nil site should allow all")
	}
}

func TestCheckImageCompatibility_EmptyURLs(t *testing.T) {
	site := &model.Site{ImageBlacklist: "bad.com"}
	compatible, incompatible := CheckImageCompatibility(nil, site)
	if len(compatible) != 0 || len(incompatible) != 0 {
		t.Errorf("empty urls should return empty results")
	}
}

func TestResolveScreenshots_Fresh(t *testing.T) {
	cached := []string{"https://example.com/1.jpg"}
	generatedAt := time.Now().Add(-2 * time.Minute) // 2 分钟前
	if !ResolveScreenshots(context.Background(), cached, generatedAt, nil) {
		t.Error("≤5min should reuse")
	}
}

func TestResolveScreenshots_RecentReachable(t *testing.T) {
	// 启动测试服务器
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	cached := []string{ts.URL + "/1.jpg"}
	generatedAt := time.Now().Add(-1 * time.Hour) // 1 小时前
	if !ResolveScreenshots(context.Background(), cached, generatedAt, nil) {
		t.Error("recent + reachable should reuse")
	}
}

func TestResolveScreenshots_RecentUnreachable(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	cached := []string{ts.URL + "/1.jpg"}
	generatedAt := time.Now().Add(-1 * time.Hour) // 1 小时前
	if ResolveScreenshots(context.Background(), cached, generatedAt, nil) {
		t.Error("recent + unreachable should reupload")
	}
}

func TestResolveScreenshots_Stale(t *testing.T) {
	cached := []string{"https://example.com/1.jpg"}
	generatedAt := time.Now().Add(-48 * time.Hour) // 48 小时前
	if ResolveScreenshots(context.Background(), cached, generatedAt, nil) {
		t.Error(">24h should reupload")
	}
}

func TestResolveScreenshots_EmptyCache(t *testing.T) {
	if ResolveScreenshots(context.Background(), nil, time.Now(), nil) {
		t.Error("empty cache should reupload")
	}
}

func TestParseHostList(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"a.com,b.com,c.com", 3},
		{"  a.com  ,  b.com  ", 2},
		{"", 0},
		{"single.com", 1},
	}
	for _, c := range cases {
		got := parseHostList(c.in)
		if len(got) != c.want {
			t.Errorf("parseHostList(%q) = %v, want %d items", c.in, got, c.want)
		}
	}
}

func TestIsHostInList(t *testing.T) {
	list := []string{"pixhost.to", "doubanio.com"}
	cases := []struct {
		host string
		want bool
	}{
		{"pixhost.to", true},
		{"cdn.pixhost.to", true},   // 子域名
		{"PIXHOST.TO", false},       // 大小写敏感（调用前应 lower）
		{"other.com", false},
		{"", false},
	}
	for _, c := range cases {
		if got := isHostInList(c.host, list); got != c.want {
			t.Errorf("isHostInList(%q) = %v, want %v", c.host, got, c.want)
		}
	}
}

func TestExtractHost(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"https://Example.COM/path", "example.com"},
		{"https://cdn.pixhost.to/img.jpg", "cdn.pixhost.to"},
		{"not a url", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := extractHost(c.in); got != c.want {
			t.Errorf("extractHost(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
