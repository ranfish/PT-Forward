package qbittorrent

import (
	"github.com/ranfish/pt-forward/internal/model"
)

const (
	ErrQBNetwork = 32001
	ErrQBParse   = 32002
)

func qbError(code int, msg string, cause error) *model.AppError {
	return &model.AppError{
		Code:      code,
		Message:   msg,
		Cause:     cause,
		Retryable: code == ErrQBNetwork,
	}
}
