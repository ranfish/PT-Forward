package cookiecloud

import (
	"github.com/ranfish/pt-forward/internal/model"
)

const (
	ErrCCCrypto = 39000
	ErrCCHTTP   = 39001
	ErrCCParse  = 39002
	ErrCCConfig = 39003
	ErrCCSync   = 39004
)

func ccError(code int, msg string, cause error) *model.AppError {
	return &model.AppError{
		Code:    code,
		Message: msg,
		Cause:   cause,
	}
}
