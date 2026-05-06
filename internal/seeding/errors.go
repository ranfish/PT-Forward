package seeding

import (
	"github.com/ranfish/pt-forward/internal/model"
)

const (
	ErrSeedingDB    = 37001
	ErrSeedingParse = 37002
)

func seedingError(code int, msg string, cause error) *model.AppError {
	return &model.AppError{
		Code:      code,
		Message:   msg,
		Cause:     cause,
		Retryable: false,
	}
}
