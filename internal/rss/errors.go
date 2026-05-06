package rss

import (
	"github.com/ranfish/pt-forward/internal/model"
)

const (
	ErrRSSNetwork = 35001
	ErrRSSParse   = 35002
	ErrRSSDisk    = 35003
)

func rssError(code int, msg string, cause error) *model.AppError {
	return &model.AppError{
		Code:      code,
		Message:   msg,
		Cause:     cause,
		Retryable: code == ErrRSSNetwork,
	}
}
