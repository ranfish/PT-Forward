package util

import "time"

// LocalDayStart 16 刀1（v0.0.1664）: 本地午夜口径单点——t 所在
// 本地日的 00:00:00（Location 感知）。
//
// 唯一正确写法=time.Date(..., t.Location())（dashboard_handler.go:282
// 同库正例）。**禁止 time.Time.Truncate(24*time.Hour) 取「今日」**
// （16 篇 V-01）：Truncate 按绝对零点对齐忽略 Location——UTC+8 下
// 对齐点落本地 08:00（前一日）·本地 00:00–08:00 被排除出今日+前一日
// 08:00–24:00 被计入（窗口错开 8 小时·epoch 整除 86400 实证）。
// 收编自 8 处生产点（dashboard×4/seeding_stats×3/engine_stats×1）。
// 部署形态依据：docker-compose.yml:23 TZ=Asia/Shanghai+Dockerfile:39
// tzdata（16 篇前置取证）。
func LocalDayStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}
