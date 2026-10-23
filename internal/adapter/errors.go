package adapter

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ranfish/pt-forward/internal/httpclient"
	"github.com/ranfish/pt-forward/internal/model"
)

const (
	ErrAdapterNetwork         = 31001
	ErrAdapterHTTP            = 31002
	ErrAdapterParse           = 31003
	ErrAdapterUpload          = 31004
	ErrAdapterDownload        = 31005
	ErrAdapterNotFound        = 31006
	ErrAdapterConfig          = 31007
	ErrAdapterAuth            = 31008
	ErrAdapterRateLimit       = 31009
	ErrAdapterSearch          = 31010
	ErrAdapterWAFBlocked      = 31011
	ErrAdapterCredentialExpired = 31012
)

func adapterError(code int, msg string, cause error) *model.AppError {
	return &model.AppError{
		Code:      code,
		Message:   msg,
		Cause:     cause,
		Retryable: code == ErrAdapterNetwork || code == ErrAdapterHTTP || code == ErrAdapterRateLimit,
	}
}

func networkError(msg string, err error) *model.AppError {
	return adapterError(ErrAdapterNetwork, msg, err)
}

func httpError(msg string, err error) *model.AppError {
	return adapterError(ErrAdapterHTTP, msg, err)
}

func parseError(msg string, err error) *model.AppError {
	return adapterError(ErrAdapterParse, msg, err)
}

func uploadError(msg string, err error) *model.AppError {
	return adapterError(ErrAdapterUpload, msg, err)
}

func downloadError(msg string, err error) *model.AppError {
	return adapterError(ErrAdapterDownload, msg, err)
}

func notFoundError(msg string) *model.AppError {
	return adapterError(ErrAdapterNotFound, msg, nil)
}

func configError(msg string) *model.AppError {
	return adapterError(ErrAdapterConfig, msg, nil)
}

func authError(msg string, err error) *model.AppError {
	return adapterError(ErrAdapterAuth, msg, err)
}

func searchError(msg string, err error) *model.AppError {
	return adapterError(ErrAdapterSearch, msg, err)
}

func classifySiteError(resp *http.Response, op string) *model.AppError {
	reason := resp.Header.Get(httpclient.WAFReasonHeader)

	if reason == "cloudflare_challenge" || reason == "cloudflare_5s_shield" {
		return adapterError(ErrAdapterWAFBlocked,
			fmtES("site blocked by WAF (%s) during %s", reason, op), nil)
	}

	if reason == "login_redirect" || resp.StatusCode == 401 || resp.StatusCode == 403 {
		return adapterError(ErrAdapterCredentialExpired,
			fmtES("authentication failed (HTTP %d) during %s", resp.StatusCode, op), nil)
	}

	if resp.StatusCode == 419 {
		return adapterError(ErrAdapterCredentialExpired,
			fmtES("CSRF token expired during %s", op), nil)
	}

	return httpError(fmtES("HTTP %d during %s", resp.StatusCode, op), nil)
}

func isWAFBlocked(resp *http.Response) bool {
	reason := resp.Header.Get(httpclient.WAFReasonHeader)
	return reason == "cloudflare_challenge" || reason == "cloudflare_5s_shield"
}

func fmtES(format string, args ...interface{}) string {
	return fmt.Sprintf(format, args...)
}

const maxResponseBodySize = 10 * 1024 * 1024

func readBody(resp *http.Response) ([]byte, error) {
	// §59.338: 读停滞保护——死连接 body 阶段挂起（RoundTrip 已返回但
	// 断流；29/243 实证"写成功读挂起"烧满外层预算 25s）。15s 无进展
	// 即快速失败（正常 body 毫秒级到达；慢站瓶颈在响应头等待，属
	// RoundTrip 阶段不受此影响）。错误串带 timeout 关键词——可被
	// isTransientFetchErr 分类为瞬态（deferred 二轮兜底）。
	type readResult struct {
		b   []byte
		err error
	}
	ch := make(chan readResult, 1)
	go func() {
		b, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodySize))
		ch <- readResult{b, err}
	}()
	select {
	case r := <-ch:
		if r.err != nil {
			return nil, networkError("读取响应失败", r.err)
		}
		return r.b, nil
	case <-time.After(15 * time.Second):
		resp.Body.Close() // 中断挂起的 Read（goroutine 随之退出）
		return nil, networkError("读取响应停滞 stalled timeout 15s", nil)
	}
}

func drainBody(resp *http.Response) {
	httpclient.DrainBody(resp)
}
