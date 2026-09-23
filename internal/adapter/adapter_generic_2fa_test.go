package adapter

import (
	"context"
	"net/http"
	"strings"
	"net/http/httptest"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.263: take2fa 重定向与空解析守卫——hddolby 二验拦截案
func newGenericFor2FA(t *testing.T) *GenericAdapter {
	t.Helper()
	return NewGenericAdapter("nexusphp", NewHTTPDoer(), nil)
}

func TestGetTorrentDetail_2FARedirect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "take2fa") {
			w.WriteHeader(200)
			_, _ = w.Write([]byte("<html><title>HDDolby :: 2fa - Powered by NexusPHP</title></html>"))
			return
		}
		http.Redirect(w, r, "/take2fa.php?returnto=details.php%3Fid%3D1", 302)
	}))
	defer srv.Close()

	a := newGenericFor2FA(t)
	cfg := &model.SiteConfig{Domain: srv.URL, Cookie: "c=1",
		SiteDefault: model.SiteDefault{Paths: model.SitePathsConfig{Detail: "details.php?id={id}"}}}
	_, err := a.GetTorrentDetail(context.Background(), cfg, "1")
	if err == nil {
		t.Fatal("2FA 重定向应报 authError，实际 nil（静默成功——降级链失效根因）")
	}
	if !strings.Contains(err.Error(), "二次验证") {
		t.Errorf("错误应指向凭证拦截: %v", err)
	}
}

func TestGetTorrentDetail_EmptyParseGuard(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html><body>some unknown interstitial page</body></html>"))
	}))
	defer srv.Close()

	a := newGenericFor2FA(t)
	cfg := &model.SiteConfig{Domain: srv.URL, Cookie: "c=1",
		SiteDefault: model.SiteDefault{Paths: model.SitePathsConfig{Detail: "details.php?id={id}"}}}
	_, err := a.GetTorrentDetail(context.Background(), cfg, "1")
	if err == nil {
		t.Fatal("空解析应报 parseError，实际 nil（静默成功）")
	}
}

func contains(s, sub string) bool { return len(s) >= len(sub) && (func() bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
})() }
