package fingerprint

import (
	"github.com/ranfish/pt-forward/internal/model"
)

const (
	ErrFPBencode = 38000
	ErrFPEncode  = 38001
	ErrFPCompute = 38002
	ErrFPRepo    = 38003
)

func fpError(code int, msg string, cause error) *model.AppError {
	return &model.AppError{
		Code:    code,
		Message: msg,
		Cause:   cause,
	}
}
