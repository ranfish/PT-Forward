package auth

import (
	"github.com/ranfish/pt-forward/internal/model"
)

const (
	ErrAuthLogin    = 40000
	ErrAuthJWT      = 40001
	ErrAuthToken    = 40002
	ErrAuthPassword = 40003
	ErrAuthInit     = 40004
)

func authError(code int, msg string, cause error) *model.AppError {
	return &model.AppError{
		Code:    code,
		Message: msg,
		Cause:   cause,
	}
}
