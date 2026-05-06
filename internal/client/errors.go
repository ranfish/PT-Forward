package client

import (
	"github.com/ranfish/pt-forward/internal/model"
)

const (
	ErrClientGeneric     = 32000
	ErrClientConnection  = 32001
	ErrClientConfigParse = 32002
)

func clientError(code int, msg string, cause error) *model.AppError {
	return &model.AppError{
		Code:      code,
		Message:   msg,
		Cause:     cause,
		Retryable: code == ErrClientConnection,
	}
}
