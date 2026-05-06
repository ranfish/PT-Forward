package publish

import (
	"github.com/ranfish/pt-forward/internal/model"
)

const (
	ErrPublishGeneric = 34000
	ErrPublishDB      = 34001
	ErrPublishConfig  = 34002
)

func publishError(code int, msg string, cause error) *model.AppError {
	return &model.AppError{
		Code:      code,
		Message:   msg,
		Cause:     cause,
		Retryable: false,
	}
}
