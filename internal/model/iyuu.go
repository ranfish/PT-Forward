package model

// §33.1.15 — IYUUReseedResult / IYUUTarget: IYUU 辅种查询结果
type IYUUReseedResult struct {
	SourceInfoHash string       `json:"source_info_hash"`
	Targets        []IYUUTarget `json:"targets"`
}

type IYUUTarget struct {
	Sid       int    `json:"sid"`
	TorrentID int    `json:"torrent_id"`
	InfoHash  string `json:"info_hash"`
	Group     int    `json:"group"`
}

// §33.1.16 — IYUUSite: IYUU 站点信息
type IYUUSite struct {
	Sid      int    `json:"sid"`
	Nickname string `json:"nickname"`
	BaseURL  string `json:"base_url"`
	Site     string `json:"site"`
}

// IYUUConfig — IYUU 服务配置（DB 存储）
type IYUUConfig struct {
	ID                 uint   `gorm:"primarykey"`
	Token              string `gorm:"size:64;not null" encrypted:"true"`
	BaseURL            string `gorm:"size:128;default:'https://2025.iyuu.cn'"`
	IsVIP              bool   `gorm:"default:false"`
	Enabled            bool   `gorm:"default:false"`
	Version            string `gorm:"size:32;default:'1.0.0'"`
	RequestTimeoutSec  int    `gorm:"default:60"`
	SyncIntervalHours  int    `gorm:"default:24"`
}

func (IYUUConfig) TableName() string { return "iyuu_configs" }

// IYUUSiteMapping — sid ↔ domain 映射（DB 存储）
type IYUUSiteMapping struct {
	ID         uint   `gorm:"primarykey"`
	IYUUSid    int    `gorm:"uniqueIndex;not null"`
	SiteDomain string `gorm:"size:128;uniqueIndex;not null"`
	SiteName   string `gorm:"size:64"`
	Enabled    bool   `gorm:"default:true"`
}

func (IYUUSiteMapping) TableName() string { return "iyuu_site_mappings" }
