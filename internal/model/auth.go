package model

import "time"

// User — 单用户模型（§34 认证）
type User struct {
	ID           uint   `gorm:"primaryKey;autoIncrement"`
	Username     string `gorm:"size:50;uniqueIndex;not null"`
	DisplayName  string `gorm:"size:100"`
	PasswordHash string `gorm:"size:60;not null"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (User) TableName() string { return "users" }
