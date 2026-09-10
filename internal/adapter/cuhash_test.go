package adapter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

// §59.185 ②: cuhash 依赖判定——mode 显式或模板含 cuhash=（城市案：mode=template
// 但模板用 cuhash={passkey}，旧门只判 mode 导致同步链路空转）。
func TestCuhashEnabled(t *testing.T) {
	cases := []struct {
		name   string
		mode   string
		tpl    string
		want   bool
	}{
		{"城市形态：template + cuhash 模板", "template", "download?id={id}&cuhash={passkey}", true},
		{"显式 cuhash mode", "cuhash", "", true},
		{"常规 template 无 cuhash", "template", "download.php?id={id}&passkey={passkey}", false},
		{"signed 模式", "signed", "", false},
	}
	for _, c := range cases {
		cfg := &model.SiteConfig{DownloadMode: c.mode, DownloadURLTemplate: c.tpl}
		if got := cuhashEnabled(cfg); got != c.want {
			t.Errorf("%s: cuhashEnabled=%v want %v", c.name, got, c.want)
		}
	}
}

// §59.185 ②: ScrapeCuhash 从首页任意下载链接提取当前 cuhash。
func TestScrapeCuhash(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<a href="download?id=123&cuhash=abc123def456abc123def456abc12345">dl</a>`))
	}))
	defer srv.Close()

	adapter := NewNexusPHPAdapter(&HTTPDoer{Client: srv.Client()}, zap.NewNop())
	cfg := &model.SiteConfig{Domain: srv.URL, Cookie: "c_secure_uid=x"}
	got := adapter.ScrapeCuhash(context.Background(), cfg)
	if got != "abc123def456abc123def456abc12345" {
		t.Fatalf("ScrapeCuhash=%q want 32hex", got)
	}

	// 无 cuhash 链接 → 空串
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html>no links</html>`))
	}))
	defer srv2.Close()
	cfg2 := &model.SiteConfig{Domain: srv2.URL, Cookie: "c_secure_uid=x"}
	if got := adapter.ScrapeCuhash(context.Background(), cfg2); got != "" {
		t.Fatalf("无链接应返回空串, got %q", got)
	}
}
