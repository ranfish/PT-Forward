package httpclient

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Noooste/azuretls-client"
)

// §59.334 附二修订: Chrome 指纹传输层——azuretls 基座 + RoundTrip 层重试
//
// v1（分层组合 fhttp/utls/fhttp2）因 fhttp.Transport 对 *utls.UConn 的
// HTTP/2 协商依赖 *utls.Conn 类型断言（返回 *Conn 后 400 Bad Request
// Cloudflare 拒收），调试成本过高。回退到 azuretls 基座（验证过能工作），
// 智能拨号通过 RoundTrip 层重试实现（首次拨到坏 IP → 重试时 azuretls
// 的 Happy Eyeballs 会尝试其他 IP）。
//
// azuretls 的 ModifyDialer 可设置 net.Dialer.Resolver，但 net.Resolver
// 是具体类型非接口——自定义解析不可行。智能拨号退化为：
//   首次请求 → azuretls 内置 Happy Eyeballs
//   失败重试 → 再次请求（azuretls 可能选不同 IP）
//   健康缓存 → 在 RoundTrip 层记住成功的 session（跳过重试）

type chromeTransport struct {
	session *azuretls.Session
	domain  string
}

var (
	chromeSessions   = map[string]*azuretls.Session{}
	chromeSessionsMu sync.Mutex
)

// getChromeSession §59.334 附二: 站点级 Session 复用
func getChromeSession(domain string) *azuretls.Session {
	chromeSessionsMu.Lock()
	defer chromeSessionsMu.Unlock()
	if s, ok := chromeSessions[domain]; ok {
		return s
	}
	s := azuretls.NewSession()
	s.ApplyJa3("", azuretls.Chrome)
	s.ApplyHTTP2("")
	s.MaxRedirects = 5
	chromeSessions[domain] = s
	return s
}

// RoundTrip 实现 http.RoundTripper——标准→azuretls 转换 + 失败重试
func (t *chromeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var bodyBytes []byte
	if req.Body != nil {
		b, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("chrome read body: %w", err)
		}
		req.Body.Close()
		bodyBytes = b
	}

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
	if req.Host != "" {
		ordered = append(ordered, []string{"host", req.Host})
	} else if req.URL.Host != "" {
		ordered = append(ordered, []string{"host", req.URL.Host})
	}

	// 最多 2 次尝试（首次 + 重试——azuretls Happy Eyeballs 可能选不同 IP）
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			// 重试前短暂等待（避免立即撞同一坏 IP）
			select {
			case <-req.Context().Done():
				return nil, req.Context().Err()
			case <-time.After(500 * time.Millisecond):
			}
		}

		aReq := &azuretls.Request{
			Method:           req.Method,
			Url:              req.URL.String(),
			NoCookie:         true,
			OrderedHeaders:   ordered,
			DisableRedirects: true,
		}
		if len(bodyBytes) > 0 {
			aReq.Body = bodyBytes
		}
		if req.ContentLength > 0 {
			aReq.ContentLength = req.ContentLength
		}
		aReq.SetContext(req.Context())

		aResp, err := t.session.Do(aReq)
		if err != nil {
			if req.Context().Err() != nil {
				return nil, req.Context().Err()
			}
			lastErr = fmt.Errorf("chrome attempt %d: %w", attempt, err)
			continue // 重试
		}

		// 成功——转换为标准 Response
		resp := &http.Response{
			StatusCode:    aResp.StatusCode,
			Status:        fmt.Sprintf("%d %s", aResp.StatusCode, http.StatusText(aResp.StatusCode)),
			Proto:         "HTTP/2.0",
			ProtoMajor:    2,
			Header:        make(http.Header),
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
	return nil, lastErr
}

func (t *chromeTransport) CloseIdleConnections() {}
