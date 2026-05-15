package adapter

import (
	"fmt"
	"io"
	"net/http"

	"github.com/ranfish/pt-forward/internal/model"
)

const (
	ErrAdapterNetwork   = 31001
	ErrAdapterHTTP      = 31002
	ErrAdapterParse     = 31003
	ErrAdapterUpload    = 31004
	ErrAdapterDownload  = 31005
	ErrAdapterNotFound  = 31006
	ErrAdapterConfig    = 31007
	ErrAdapterAuth      = 31008
	ErrAdapterRateLimit = 31009
	ErrAdapterSearch    = 31010
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

func fmtES(format string, args ...interface{}) string {
	return fmt.Sprintf(format, args...)
}

func readBody(resp *http.Response) ([]byte, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, networkError("读取响应失败", err)
	}
	return body, nil
}
