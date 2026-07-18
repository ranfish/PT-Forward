package model

import "time"

// §56.1 — StandardKey: 标准化键体系
// 对齐 PTNexus global_mappings.yaml + auto_feed_js 中文 label 路线
// key（中文）是主标识，standard_code（英文有前缀）是代码引用
type StandardKey struct {
	ID           uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Category     string    `json:"category" gorm:"size:50;not null;uniqueIndex:idx_stdkey_cat_key,priority:1"`
	Key          string    `json:"key" gorm:"size:200;not null;uniqueIndex:idx_stdkey_cat_key,priority:2;index:idx_stdkey_category"`
	StandardCode string    `json:"standard_code" gorm:"size:100"`
	AliasesJSON  string    `json:"aliases_json" gorm:"type:text"`
	IsProtected  bool      `json:"is_protected" gorm:"default:false"`
	SortOrder    int       `json:"sort_order" gorm:"default:0"`
	Source       string    `json:"source" gorm:"size:20;not null;default:'builtin'"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (StandardKey) TableName() string { return "standard_keys" }
