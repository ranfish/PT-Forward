package api

import (
	"github.com/ranfish/pt-forward/internal/model"
)

const (
	ErrAPIInvalidID = 44000
)

func apiError(code int, msg string, cause error) *model.AppError {
	return &model.AppError{
		Code:    code,
		Message: msg,
		Cause:   cause,
	}
}
