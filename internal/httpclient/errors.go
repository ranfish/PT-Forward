package httpclient

import (
	"github.com/ranfish/pt-forward/internal/model"
)

const (
	ErrHTRateLimit   = 47020
	ErrHTCircuitOpen = 47021
	ErrHTRetryFailed = 47022
)

func httpError(code int, msg string, cause error) *model.AppError {
	return &model.AppError{
		Code:    code,
		Message: msg,
		Cause:   cause,
	}
}
