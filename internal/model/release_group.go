package model

import "time"

type ReleaseGroupMapping struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	GroupName string    `gorm:"size:100;not null;uniqueIndex:idx_group_name" json:"group_name"`
	SiteName  string    `gorm:"size:100;not null;index" json:"site_name"`
	IsOfficial bool     `gorm:"default:false" json:"is_official"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (ReleaseGroupMapping) TableName() string {
	return "release_group_mappings"
}
