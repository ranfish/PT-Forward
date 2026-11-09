package cookiecloud

import (
	"testing"
	"time"
)

// §59.498: 同步循环等待计算锚——"每 interval 分钟一次"语义（开始到开始=
// interval·耗时扣除）+ off-by-epsilon 根治回归锚（旧杂交模式：LastSyncAt
// 记完成时刻+3s 耗时 → 严格小于判断把"正好 interval"的 tick 永远 skip →
// 10 分钟配置跑 15 分钟周期·29 实证）。
func TestCookiecloudLoopWait(t *testing.T) {
	cases := []struct {
		name        string
		enabled     bool
		mode        string
		intervalMin int
		elapsed     time.Duration
		want        time.Duration
	}{
		// 未启用 → 短轮询感知配置变化
		{"disabled", false, "remote", 10, 0, 30 * time.Second},
		// builtin（插件推送·无需拉取）→ 短轮询
		{"builtin", true, "builtin", 10, 0, 30 * time.Second},
		// 配置缺失（cfg 零值）→ 短轮询
		{"zero config", false, "", 0, 0, 30 * time.Second},
		// interval 下限钳 5min（旧调度任务同款钳制保留）
		{"clamp min", true, "remote", 3, 0, 5 * time.Minute},
		// 核心：10min 配置 + 3s 耗时 → 等 9m57s（旧模式此处差 3 秒被
		// skip 顺延 15 分钟——开始时刻基准+耗时扣除根治）
		{"epsilon fixed", true, "remote", 10, 3 * time.Second, 9*time.Minute + 57*time.Second},
		// 正常周期：15min 配置零耗时 → 等整 15min
		{"exact interval", true, "remote", 15, 0, 15 * time.Minute},
		// 耗时≥interval-30s（慢网络）→ 30s 兜底防热循环
		{"slow overrun", true, "remote", 10, 12 * time.Minute, 30 * time.Second},
		// 剩余 <30s → 取兜底 30s（等待略长于剩余间隔·无害）
		{"near boundary", true, "remote", 10, 9*time.Minute + 40*time.Second, 30 * time.Second},
	}
	for _, c := range cases {
		if got := cookiecloudLoopWait(c.enabled, c.mode, c.intervalMin, c.elapsed); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}
