package httpclient

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/Noooste/azuretls-client"
)

// §59.333: azuretls 伪装传输层——标准 http.RoundTripper ↔ azuretls.Session
// 适配（keepfrds 案：JA4 cipher 13/31 + H2 SETTINGS + 伪头序 a,m,p,s 三层
// Go 特征被 Cloudflare 指纹库识别 → empty_response 267 次/天；UA 修复
// 应用层后残留实证=TLS/H2 层）。
//
// 适配策略：请求侧（标准 Request→azuretls OrderedHeaders 保序）+ 响应侧
// （azuretls.Response→标准 Response 流式包装）。Session 全站复用（连接
// 池内建）+ 并发安全（azuretls Session 文档标注 goroutine-safe）。
type azuretlsTransport struct {
	session *azuretls.Session
}

var (
	azureSessions   = map[string]*azuretls.Session{}
	azureSessionsMu sync.Mutex
)

// getAzureSession §59.333: 站点级 Session 复用（连接池）——
// 缓存 key=domain+proxy（同域不同代理独立 session）。
// proxyURL 跟随站点配置（ResolveSiteProxy 已解析——站点显式 > 全局开关 >
// 空直连；azuretls 层不引入额外代理判断）。
func getAzureSession(domain, proxyURL string) *azuretls.Session {
	azureSessionsMu.Lock()
	defer azureSessionsMu.Unlock()
	key := domain + "|" + proxyURL
	if s, ok := azureSessions[key]; ok {
		return s
	}
	s := azuretls.NewSession()
	s.ApplyJa3("", azuretls.Chrome) // 内置 Chrome spec（Ja3+H2+伪头序一体）
	s.ApplyHTTP2("")
	if proxyURL != "" {
		if err := s.SetProxy(proxyURL); err != nil {
			// 代理设置失败——降级直连（不阻断——外层错误分类承接）
			_ = err
		}
	}

	s.MaxRedirects = 5
	azureSessions[key] = s
	return s
}

func (t *azuretlsTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	s := t.session

	// 请求体（POST 等）
	var bodyBytes []byte
	if req.Body != nil {
		b, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("azuretls read body: %w", err)
		}
		req.Body.Close()
		bodyBytes = b
	}

	// 标准头→OrderedHeaders（保序——azuretls 伪头序由 session 控，
	// 普通头按标准 Header 遍历序传）
	ordered := make([][]string, 0, len(req.Header)+1)
	if ua := req.Header.Get("User-Agent"); ua != "" {
		ordered = append(ordered, []string{"user-agent", ua})
	}
	for k, vs := range req.Header {
		if strings.EqualFold(k, "User-Agent") {
			continue
		}
		for _, v := range vs {
			ordered = append(ordered, []string{strings.ToLower(k), v})
		}
	}
	if host := req.Host; host != "" {
		ordered = append(ordered, []string{"host", host})
	} else if req.URL.Host != "" {
		ordered = append(ordered, []string{"host", req.URL.Host})
	}

	aReq := &azuretls.Request{
		Method:         req.Method,
		Url:            req.URL.String(),
		NoCookie:       true,
		OrderedHeaders: ordered,
		DisableRedirects: true,
	}
	if len(bodyBytes) > 0 {
		aReq.Body = bodyBytes
	}
	if req.ContentLength > 0 {
		aReq.ContentLength = req.ContentLength
	}
	aReq.SetContext(req.Context())

	aResp, err := s.Do(aReq)
	if err != nil {
		// ctx 取消透传（竞速/超时场景——非网络错误）
		if req.Context().Err() != nil {
			return nil, req.Context().Err()
		}
		return nil, fmt.Errorf("azuretls: %w", err)
	}

	// 响应转换：azuretls.Response → 标准 http.Response（body 流式）
	resp := &http.Response{
		StatusCode:    aResp.StatusCode,
		Status:        fmt.Sprintf("%d %s", aResp.StatusCode, http.StatusText(aResp.StatusCode)),
		Proto:         "HTTP/2.0",
		ProtoMajor:    2,
		Header:        http.Header{},
		ContentLength: int64(len(aResp.Body)),
		Request:       req,
	}
	for k, vs := range aResp.Header {
		for _, v := range vs {
			resp.Header.Add(k, v)
		}
	}
	resp.Body = io.NopCloser(bytes.NewReader(aResp.Body))
	return resp, nil
}

func (t *azuretlsTransport) CloseIdleConnections() {}
