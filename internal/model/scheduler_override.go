package model

import "time"

type SchedulerTaskOverride struct {
	Name      string    `json:"name" gorm:"primaryKey;size:100;not null"`
	Schedule  string    `json:"schedule" gorm:"size:100;not null"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (SchedulerTaskOverride) TableName() string { return "scheduler_task_overrides" }
