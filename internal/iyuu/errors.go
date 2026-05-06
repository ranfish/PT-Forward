package iyuu

import (
	"github.com/ranfish/pt-forward/internal/model"
)

const (
	ErrIYUUConfig = 41000
	ErrIYUUAPI    = 41001
	ErrIYUUHTTP   = 41002
)

func iyuuError(code int, msg string, cause error) *model.AppError {
	return &model.AppError{
		Code:    code,
		Message: msg,
		Cause:   cause,
	}
}
