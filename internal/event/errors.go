package event

import (
	"github.com/ranfish/pt-forward/internal/model"
)

const (
	ErrEventHandler = 47010
)

func eventError(code int, msg string, cause error) *model.AppError {
	return &model.AppError{
		Code:    code,
		Message: msg,
		Cause:   cause,
	}
}
