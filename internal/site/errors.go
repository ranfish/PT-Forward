package site

import (
	"github.com/ranfish/pt-forward/internal/model"
)

const (
	ErrSiteSeed = 33000
)

func siteError(code int, msg string, cause error) *model.AppError {
	return &model.AppError{
		Code:    code,
		Message: msg,
		Cause:   cause,
	}
}
