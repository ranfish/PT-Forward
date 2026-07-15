package adapter

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
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
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodySize))
	if err != nil {
		return nil, networkError("读取响应失败", err)
	}
	return body, nil
}

func drainBody(resp *http.Response) {
	httpclient.DrainBody(resp)
}

var http1FallbackClient = &http.Client{
	Timeout: 20 * time.Second,
	Transport: &http.Transport{
		TLSNextProto: make(map[string]func(string, *tls.Conn) http.RoundTripper),
	},
}

func isHTTP2Error(err error) bool {
	for err != nil {
		if strings.Contains(err.Error(), "http2") {
			return true
		}
		err = errors.Unwrap(err)
	}
	return false
}

func retryWithHTTP1(originalURL, cookie string) ([]byte, int, error) {
	req, err := http.NewRequest("GET", originalURL, nil)
	if err != nil {
		return nil, 0, err
	}
	setCommonHeaders(req, cookie)
	resp, err := http1FallbackClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer func() { httpclient.DrainBody(resp) }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodySize))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}
