package dispatcher

import (
	"github.com/ranfish/pt-forward/internal/model"
)

const (
	ErrDispatcherRoute = 47000
)

func dispatcherError(code int, msg string, cause error) *model.AppError {
	return &model.AppError{
		Code:    code,
		Message: msg,
		Cause:   cause,
	}
}
