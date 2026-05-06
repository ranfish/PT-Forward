package ptgen

import (
	"github.com/ranfish/pt-forward/internal/model"
)

const (
	ErrPTGenRemote   = 45000
	ErrPTGenResponse = 45001
)

func ptgenError(code int, msg string, cause error) *model.AppError {
	return &model.AppError{
		Code:    code,
		Message: msg,
		Cause:   cause,
	}
}
