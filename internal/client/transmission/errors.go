package transmission

import (
	"github.com/ranfish/pt-forward/internal/model"
)

const (
	ErrTRParse = 32002
)

func trError(code int, msg string, cause error) *model.AppError {
	return &model.AppError{
		Code:      code,
		Message:   msg,
		Cause:     cause,
		Retryable: false,
	}
}
