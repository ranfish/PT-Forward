package model

import (
	"time"

	"github.com/robfig/cron/v3"
)

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
	ID                uint   `gorm:"primarykey"`
	Token             string `gorm:"size:64;not null" encrypted:"true"`
	BaseURL           string `gorm:"size:128;default:'https://2025.iyuu.cn'"`
	IsVIP             bool   `gorm:"default:false"`
	Enabled           bool   `gorm:"default:false"`
	Version           string `gorm:"size:32;default:'1.0.0'"`
	RequestTimeoutSec int    `gorm:"default:60"`
	SyncIntervalHours int    `gorm:"default:24"`
	// §59.535 刀3: IYUU 覆盖定时任务——开关（默认停·无发布需求环境零开销）
	// +调度计划（cron·默认每 12h）。窗口=调度周期自动推导（「多久查一轮缓存
	// 保鲜多久」）。全量轮询（非增量——转发动态性：A 种今天 a 站明天 b/c/d 站）。
	CoverageEnabled  bool   `gorm:"default:false" json:"coverage_enabled"`
	CoverageSchedule string `gorm:"size:64;default:''" json:"coverage_schedule"`
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

// CoverageHit 已迁移 model/coverage.go（§59.535 扩 TargetInfoHash）

// CoverageWindow §59.535 刀3: 窗口=调度周期自动推导（「多久查一轮，缓存就保
// 鲜多久」）。cron 两次触发间隔（Next 链差值）；解析失败/异常退化为 24h
// （偏小安全——窗口>周期会任务空转、窗口<周期仅少量重复查询）。纯函数·
// coverage/iyuu/reseed 三方消费中立位。
func CoverageWindow(schedule string) time.Duration {
	const fallback = 24 * time.Hour
	if schedule == "" {
		return fallback
	}
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	spec, err := parser.Parse(schedule)
	if err != nil {
		return fallback
	}
	now := time.Now()
	next1 := spec.Next(now)
	if next1.IsZero() {
		return fallback
	}
	next2 := spec.Next(next1)
	if next2.IsZero() {
		return fallback
	}
	d := next2.Sub(next1)
	if d <= 0 || d > 30*24*time.Hour {
		return fallback
	}
	return d
}
