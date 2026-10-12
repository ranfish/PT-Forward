package httpclient

import (
	"net/http"
	"strings"
	"testing"
)

func TestBrowserUserAgent(t *testing.T) {
	ua := BrowserUserAgent()
	if !strings.Contains(ua, "Chrome/131.0.0.0") {
		t.Errorf("UA 缺 Chrome 版本: %s", ua)
	}
	if !strings.HasPrefix(ua, "Mozilla/5.0 (Windows NT 10.0; Win64; x64)") {
		t.Errorf("UA 平台前缀不符: %s", ua)
	}
}

func TestSetBrowserHeaders(t *testing.T) {
	h := http.Header{}
	SetBrowserHeaders(h, "uid=1;pass=abc")

	if ua := h.Get("User-Agent"); !strings.Contains(ua, "Chrome/131.0.0.0") {
		t.Errorf("UA 不符: %s", ua)
	}
	if sec := h.Get("Sec-Ch-Ua"); !strings.Contains(sec, `"131"`) {
		t.Errorf("Sec-Ch-Ua 版本与 UA 不一致: %s", sec)
	}
	for _, k := range []string{
		"Sec-Ch-Ua-Mobile", "Sec-Ch-Ua-Platform",
		"Sec-Fetch-Dest", "Sec-Fetch-Mode", "Sec-Fetch-Site", "Sec-Fetch-User",
		"Upgrade-Insecure-Requests", "Accept", "Accept-Language",
	} {
		if h.Get(k) == "" {
			t.Errorf("缺失头 %s", k)
		}
	}
	if h.Get("Cookie") != "uid=1;pass=abc" {
		t.Errorf("Cookie 不符: %s", h.Get("Cookie"))
	}
	if _, ok := h["Accept-Encoding"]; ok {
		t.Error("不应设置 Accept-Encoding（Go 自动 gzip 透明解压）")
	}
}

func TestSetBrowserHeadersEmptyCookie(t *testing.T) {
	h := http.Header{}
	SetBrowserHeaders(h, "")
	if _, ok := h["Cookie"]; ok {
		t.Error("空 cookie 不应设置 Cookie 头")
	}
}

func TestSetBrowserUserAgent(t *testing.T) {
	h := http.Header{}
	h.Set("Content-Type", "application/json")
	SetBrowserUserAgent(h, "sid=x")
	if ua := h.Get("User-Agent"); !strings.Contains(ua, "Chrome/131.0.0.0") {
		t.Errorf("UA 不符: %s", ua)
	}
	if h.Get("Cookie") != "sid=x" {
		t.Errorf("Cookie 不符: %s", h.Get("Cookie"))
	}
	if _, ok := h["Sec-Fetch-Dest"]; ok {
		t.Error("轻量版不应设置 Sec-Fetch 头")
	}
	if h.Get("Content-Type") != "application/json" {
		t.Error("不应覆盖已有头")
	}
}
