package model

import "time"

type ReleaseGroupMapping struct {
	ID         uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	GroupName  string    `gorm:"size:100;not null;uniqueIndex:idx_group_name" json:"group_name"`
	Domain     string    `gorm:"size:200;not null" json:"domain"`
	SiteName   string    `gorm:"size:100" json:"site_name"`
	IsOfficial bool      `gorm:"default:false" json:"is_official"`
	IsBuiltin  bool      `gorm:"default:false" json:"is_builtin"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (ReleaseGroupMapping) TableName() string {
	return "release_group_mappings"
}
