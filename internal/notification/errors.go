package notification

import (
	"github.com/ranfish/pt-forward/internal/model"
)

const (
	ErrNotifyChannel = 46000
)

func notifyError(code int, msg string, cause error) *model.AppError {
	return &model.AppError{
		Code:    code,
		Message: msg,
		Cause:   cause,
	}
}
