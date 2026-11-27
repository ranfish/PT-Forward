package util

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// 16 刀3（v0.0.1664）: 本地日界锚+Truncate(24h) 静态禁用锚。
//
// 时区确定性：锚点全部用 ParseInLocation 显式构造（时间值
// Location 归属明确·不依赖运行环境 TZ——15 篇 §59.292「CI UTC
// 与本地 CST 双环境飘移」教训）。LocalDayStart 是 Location 感知
// 函数——在固定 Location 的输入上断言输出·任何环境跑结果一致。
//
// 阴性对照（08 篇方法论——锚必须可触发）：
//   - LocalDayStart 退回 Truncate(24h) 实现 → 日界锚 FAIL；
//   - 趋势图 dayStart 改回 Truncate / 标签改回 dayStart.Format →
//     窗口/标签锚 FAIL（api 包）；
//   - 删静态锚或新增 Truncate(24h) → 静态锚 FAIL。

func mustLocalTime(t *testing.T, layout, value string) time.Time {
	t.Helper()
	tm, err := time.ParseInLocation(layout, value, time.Local)
	if err != nil {
		t.Fatalf("ParseInLocation(%q): %v", value, err)
	}
	return tm
}

// 锚1: 本地午夜口径四态——凌晨/早晨/上午/次日边界全部归当日 00:00。
func TestLocalDayStartBoundary(t *testing.T) {
	cases := []struct {
		name  string
		clock string
	}{
		{"凌晨（V-01 原案·原归前一日 08:00）", "2026-11-27 00:30:00"},
		{"早晨（原归前一日 08:00）", "2026-11-27 07:30:00"},
		{"上午（原对齐点 08:00）", "2026-11-27 08:30:00"},
		{"午夜前一刻（次日边界）", "2026-11-27 23:59:59"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tm := mustLocalTime(t, "2006-01-02 15:04:05", c.clock)
			got := LocalDayStart(tm)
			want := mustLocalTime(t, "2006-01-02 15:04:05", "2026-11-27 00:00:00")
			if !got.Equal(want) {
				t.Errorf("LocalDayStart(%s) = %s, want 2026-11-27 00:00（当日本地午夜）",
					c.clock, got.Format("2006-01-02 15:04:05"))
			}
		})
	}

	// 跨日：次日凌晨归次日
	next := mustLocalTime(t, "2006-01-02 15:04:05", "2026-11-28 00:30:00")
	wantNext := mustLocalTime(t, "2006-01-02 15:04:05", "2026-11-28 00:00:00")
	if got := LocalDayStart(next); !got.Equal(wantNext) {
		t.Errorf("LocalDayStart(11-28 00:30) = %s, want 2026-11-28 00:00", got.Format("2006-01-02 15:04:05"))
	}
}

// 锚2: 对照断言——本地午夜口径与 Truncate(24h) 在 UTC+8 形态下
// 分叉（差异窗口 8 小时·黑车防「两口径等价」的退化）。
// 注：此锚在 TZ=UTC 环境两口径恰等价——用固定 Location=UTC+8
// 构造（FixedZone）保证任何环境都分叉。
func TestLocalDayStartDivergesFromTruncate(t *testing.T) {
	// 固定 UTC+8（不依赖运行环境——CI/本地皆同）
	utcPlus8 := time.FixedZone("UTC+8", 8*3600)
	tm := time.Date(2026, 11, 27, 7, 30, 0, 0, utcPlus8)
	got := LocalDayStart(tm)
	tr := tm.Truncate(24 * time.Hour)
	if got.Equal(tr) {
		t.Errorf("UTC+8 下 LocalDayStart 与 Truncate(24h) 不应等价（等价=退回 V-01 回归）: both=%s",
			got.Format("2006-01-02 15:04:05"))
	}
	// LocalDayStart 归当日 00:00·Truncate 归前一日 08:00
	if got.Hour() != 0 || got.Day() != 27 {
		t.Errorf("LocalDayStart(UTC+8 11-27 07:30) = %s, want 11-27 00:00", got.Format("2006-01-02 15:04:05"))
	}
	if tr.Hour() != 8 || tr.Day() != 26 {
		t.Errorf("Truncate(24h) 参照 = %s, want 11-26 08:00（原缺陷形态）", tr.Format("2006-01-02 15:04:05"))
	}
}

// 锚3: 静态禁用锚——全库生产源码禁止 Truncate(24*time.Hour)
// （16 篇 V-01/V-02 形态指纹·收编后残留应为零。测试文件豁免
// ——本锚文件自身的对照实现用字符串拼接规避自匹配）。
func TestNoBareTruncate24h(t *testing.T) {
	// 17 篇用户复审提示：扫描根扩展到仓库根（internal/+cmd/+tools/
	// ——cmd/pt-forward 是生产主程序·tools/siteprobe 有 DB 读写栈）。
	// 目录白名单形态（internal/cmd/tools 明确列举）：examples/ 是
	// 第三方参考代码（autobrr/pt-tools/qui vendored——非本项目维护
	// 面·扩根首跑实证 3 处形态全在 examples/）不扫。
	root := filepath.Join("..", "..")
	var violations []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// 一级目录白名单（17 篇扩根）：root 的直接子目录限定
			// internal/cmd/tools——examples/ 是第三方参考代码
			// （autobrr/pt-tools/qui vendored 非维护面·扩根首跑实证
			// 3 处形态全在其中）；web/frontend/.git 等跳过。二级及
			// 以下（internal/util、cmd/pt-forward 等包目录）全放行
			// ——目录名匹配形态会误跳包子目录（debug 实证 ENTER cmd
			// 后 pt-forward 子目录被 Skip）。
			if path == root {
				return nil
			}
			if filepath.Dir(path) == root {
				switch info.Name() {
				case "internal", "cmd", "tools":
					return nil
				default:
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		// 指纹：Truncate(24 后跟 * time.Hour) 或 24*time.Hour（两种空格形态）
		for i, line := range strings.Split(string(data), "\n") {
			// 注释截断（day.go 文档说明含禁用词——非代码·13 篇 like 锚同款处理）
			if idx := strings.Index(line, "//"); idx >= 0 {
				line = line[:idx]
			}
			if strings.Contains(line, "Truncate(24") && strings.Contains(line, "time.Hour") {
				rel, _ := filepath.Rel(root, path)
				violations = append(violations, rel+":"+itoa16(i+1)+" "+strings.TrimSpace(line))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(violations) > 0 {
		t.Errorf("发现 Truncate(24h) 禁用形态（取「今日」一律 util.LocalDayStart——16 篇 V-01 日界偏移 8 小时）:\n%s",
			strings.Join(violations, "\n"))
	}
}

func itoa16(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa16(n/10) + string(rune('0'+n%10))
}
