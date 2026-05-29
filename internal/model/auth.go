package model

import "time"

// User — 单用户模型（§34 认证）
type User struct {
	ID           uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	Username     string `gorm:"size:50;uniqueIndex;not null" json:"username"`
	DisplayName  string `gorm:"size:100" json:"displayName"`
	PasswordHash string `gorm:"size:60;not null" json:"-"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

func (User) TableName() string { return "users" }
