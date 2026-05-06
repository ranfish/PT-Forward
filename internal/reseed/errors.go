package reseed

import (
	"github.com/ranfish/pt-forward/internal/model"
)

const (
	ErrReseedGeneric = 36000
	ErrReseedDB      = 36001
	ErrReseedConfig  = 36002
)

func reseedError(code int, msg string, cause error) *model.AppError {
	return &model.AppError{
		Code:      code,
		Message:   msg,
		Cause:     cause,
		Retryable: false,
	}
}
