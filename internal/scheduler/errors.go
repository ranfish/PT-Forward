package scheduler

import (
	"github.com/ranfish/pt-forward/internal/model"
)

const (
	ErrSchedulerDuplicate = 43000
	ErrSchedulerNotFound  = 43001
	ErrSchedulerPaused    = 43002
	ErrSchedulerSchedule  = 43003
)

func schedulerError(code int, msg string, cause error) *model.AppError {
	return &model.AppError{
		Code:    code,
		Message: msg,
		Cause:   cause,
	}
}
