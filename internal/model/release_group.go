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

// ExternalReleaseGroup §59.577 刀1: 外站组注册表——非本系统内置支持站点
// 官组的发布组（国际站/外站 encoder 组·如 PTP/HDB/BHD 的组）。
// 与 release_group_mappings 语义分离：mappings=「支持站官组→支持站」
// （LookupGroup/源站优先级/词表三消费方语义纯净）；本表=「外站组身份
// 注册」——窄门 A 的发布资格（收录才可发布·未收录落禁转/无映射）。
// home_site 信息性（外站名/域名·不参与任何运行时路由）；变体=显式行
// （§59.550 显式收编纪律·非正则推导）。
type ExternalReleaseGroup struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	GroupName string    `gorm:"size:100;not null;uniqueIndex:idx_ext_group_name" json:"group_name"`
	HomeSite  string    `gorm:"size:100" json:"home_site"` // 信息性（外站名·空=未知）
	Source    string    `gorm:"size:50" json:"source"`     // 词条来源：manual/census/learned
	Notes     string    `gorm:"size:500" json:"notes"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (ExternalReleaseGroup) TableName() string {
	return "external_release_groups"
}
