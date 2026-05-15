package model

import (
	"time"

	"gorm.io/gorm"
)

// §8.8.4 — FilterRule: 过滤规则
type FilterRule struct {
	ID         uint            `json:"id" gorm:"primaryKey;autoIncrement"`
	Name       string          `json:"name" gorm:"size:128;not null"`
	RuleType   string          `json:"rule_type" gorm:"size:16;not null"`
	Priority   int             `json:"priority" gorm:"default:100"`
	Conditions []RuleCondition `json:"conditions" gorm:"type:text;serializer:json"`
	SavePath   string          `json:"save_path" gorm:"size:512"`
	Category   string          `json:"category" gorm:"size:128"`
	Tags       string          `json:"tags" gorm:"size:512"`
	Enabled    bool            `json:"enabled" gorm:"default:true"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

func (FilterRule) TableName() string { return "filter_rules" }

// RuleCondition — 规则条件
type RuleCondition struct {
	Key         string      `json:"key"`
	CompareType CompareType `json:"compare_type"`
	Value       string      `json:"value"`
}
