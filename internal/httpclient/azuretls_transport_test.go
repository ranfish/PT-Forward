package httpclient

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

// §59.333: azuretls 伪装传输——指纹脱离 Go 特征 + 标准接口兼容。
func TestAzuretlsTransport_Fingerprint(t *testing.T) {
	if testing.Short() {
		t.Skip("网络测试")
	}
	tr := &azuretlsTransport{session: getAzureSession("tls.peet.ws", "")}
	req, _ := http.NewRequest("GET", "https://tls.peet.ws/api/all", nil)
	SetBrowserHeaders(req.Header, "")
	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("roundtrip: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	s := string(body)
	// H2 伪头序应为浏览器序（m 在前——Go 特征是 a,m,p,s）
	if !strings.Contains(s, `"ja4"`) {
		t.Fatal("指纹服务未返回")
	}
	// JA4 第二段不应是 Go 的 d13（cipher 13 个）
	if strings.Contains(s, "t13d1312h2_") {
		t.Fatal("仍是 Go 原生指纹（d13 cipher）——伪装未生效")
	}
	// H2 伪头序不应是 a,m,p,s（Go）
	if strings.Contains(s, `"a,m,p,s"`) {
		t.Fatal("H2 伪头序仍是 Go 特征 a,m,p,s")
	}
}

func TestAzuretlsTransport_POST(t *testing.T) {
	if testing.Short() {
		t.Skip("网络测试")
	}
	// 标准 POST 语义（body 传递）
	tr := &azuretlsTransport{session: getAzureSession("httpbin.org", "")}
	req, _ := http.NewRequest("POST", "https://httpbin.org/post", strings.NewReader("hello=azuretls"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "azuretls") {
		t.Fatal("POST body 未到达")
	}
}
