package model

import "time"

// ClusterScreenshotCache §59.63: 簇截图链接缓存（观察期）。
// 自动链截图上传成功后写穿；清簇重取时命中且在观察期内直接复用链接，
// 跳过探活/转存/mpv/上传全链（省 mpv CPU + pixhost 上传配额 + 重复图副本）。
// 键 = 簇键（client_id, save_path, name），与 §59.61 cluster_key 一致。
// 手动截图捕获行为不变（每次全新截传），但结果同样写穿（Q4 定案——
// 缓存语义 = 簇最新已知好链接）。观察期过期 = miss（惰性判定，无后台扫描）。
type ClusterScreenshotCache struct {
	ID          uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	ClientID    string    `json:"client_id" gorm:"size:50;not null;uniqueIndex:idx_shot_cache_cluster"`
	SavePath    string    `json:"save_path" gorm:"size:500;not null;uniqueIndex:idx_shot_cache_cluster"`
	Name        string    `json:"name" gorm:"size:500;not null;uniqueIndex:idx_shot_cache_cluster"`
	Screenshots string    `json:"screenshots" gorm:"type:text;not null"` // JSON 数组
	UpdatedAt   time.Time `json:"updated_at" gorm:"not null"`            // 观察期锚点（最近一次成功写穿）
}

func (ClusterScreenshotCache) TableName() string { return "cluster_screenshot_cache" }
