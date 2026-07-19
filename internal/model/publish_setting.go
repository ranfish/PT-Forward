package model

import "time"

// §56.14 决策 5 — PublishSetting: 通用 key-value 发布配置表。
// 当前 key: metadata_priority (ptgen_first | detail_first)
// 后续可扩展其他全局发布配置。
type PublishSetting struct {
	ID          uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Key         string    `json:"key" gorm:"uniqueIndex;size:100;not null"`
	Value       string    `json:"value" gorm:"size:500;not null"`
	Description string    `json:"description" gorm:"size:500"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (PublishSetting) TableName() string { return "publish_settings" }

// MetadataPriorityDefault 默认元数据优先级（ptgen_first）。
const MetadataPriorityDefault = "ptgen_first"
