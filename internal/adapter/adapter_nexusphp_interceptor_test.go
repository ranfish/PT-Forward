package adapter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.266: NexusPHP 适配器三防线——不可杜（hddolby）走 NexusPHP 框架，
// 守卫只写 Generic 不生效的复发案（243 15:52 异地登录提醒落库实证）
func newNexusForTest() *NexusPHPAdapter {
	return NewNexusPHPAdapter(NewHTTPDoer(), nil)
}

func nexusCfg(srvURL string) *model.SiteConfig {
	return &model.SiteConfig{
		Domain: srvURL,
		Cookie: "c=1",
		SiteDefault: model.SiteDefault{
			Framework: "nexusphp",
			Paths:     model.SitePathsConfig{Detail: "details.php?id={id}"},
		},
	}
}

func TestNexusDetail_2FARedirect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "take2fa") {
			w.WriteHeader(200)
			_, _ = w.Write([]byte("<html><title>HDDolby :: 2fa - Powered by NexusPHP</title></html>"))
			return
		}
		http.Redirect(w, r, "/take2fa.php?returnto=details.php%3Fid%3D1", 302)
	}))
	defer srv.Close()

	_, err := newNexusForTest().GetTorrentDetail(context.Background(), nexusCfg(srv.URL), "1")
	if err == nil {
		t.Fatal("NexusPHP 路径 2FA 重定向应报 authError（修复落错适配器复发案）")
	}
}

func TestNexusDetail_InterstitialTitleOnly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html><head><title>异地登录提醒！</title></head><body>您的账号在异地登录</body></html>"))
	}))
	defer srv.Close()

	_, err := newNexusForTest().GetTorrentDetail(context.Background(), nexusCfg(srv.URL), "1")
	if err == nil {
		t.Fatal("NexusPHP 路径 title-only 拦截页应报 authError")
	}
	if !strings.Contains(err.Error(), "凭证拦截") {
		t.Errorf("错误应指向凭证拦截: %v", err)
	}
}
