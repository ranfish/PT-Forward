package config

import (
	"github.com/ranfish/pt-forward/internal/model"
)

const (
	ErrConfigLoad     = 42000
	ErrConfigValidate = 42001
)

func configError(code int, msg string, cause error) *model.AppError {
	return &model.AppError{
		Code:    code,
		Message: msg,
		Cause:   cause,
	}
}
