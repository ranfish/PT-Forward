// Package metadata keepfrds 禁转两态判定（§59.162 用户站点经验权威定义）。
//
// 标记语义（列表页种子名后标记区）：
//   [禁转]（无"限时"）   = 站点设置长期禁转——永久拦
//   [ 限时禁转 ]（带空格）= 新种 24h 保护窗——no_transfer_until=added+24h+30m 余量，
//                          now<until 才拦，过期自动放行（无需复查采集）
//   24h 后站方文案自动消失；老种列表页首屏无行=无标记语义
//
// 发布时间（详情页"由 xx 发布于 X时X分前"）：相对时间仅分/时档需换算；
// 天/月/年档出现即必然超 24h 窗（用户实测：形态恒为相对，不切绝对）。
package metadata

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	reLimitedNoTransfer = regexp.MustCompile(`限\s*时\s*禁\s*转`)
	rePermanentNoXfer   = regexp.MustCompile(`禁\s*转`)
)

// ParseRelativeAge NP 相对时间 → 距今时长（"20时46分前"/"3小时前"/"5天前"/
// "1月2天前"/"1年3月前"）。天档以上给大值（超 24h 即够用——精度要求低）。
func ParseRelativeAge(s string) time.Duration {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, " ", "")
	// 档位顺序：月/年（必然超 24h 窗）→ 天 → 时（X时X分/X小时）→ 分
	if strings.Contains(s, "月") || strings.Contains(s, "年") {
		return 365 * 24 * time.Hour
	}
	if d := regexp.MustCompile(`(\d+)天`).FindStringSubmatch(s); d != nil {
		days, _ := strconv.Atoi(d[1])
		return time.Duration(days) * 24 * time.Hour
	}
	if h := regexp.MustCompile(`(\d+)[时小]时?(?:(\d+)分)?`).FindStringSubmatch(s); h != nil {
		hours, _ := strconv.Atoi(h[1])
		mins := 0
		if len(h) > 2 && h[2] != "" {
			mins, _ = strconv.Atoi(h[2])
		}
		return time.Duration(hours)*time.Hour + time.Duration(mins)*time.Minute
	}
	if m := regexp.MustCompile(`(\d+)分`).FindStringSubmatch(s); m != nil {
		mins, _ := strconv.Atoi(m[1])
		return time.Duration(mins) * time.Minute
	}
	return 0
}

// ClassifyNoTransfer 列表行文本 → 禁转两态。
// limited=true 时返回窗口截止时间（added=发布时间推算基准 now-age）。
func ClassifyNoTransfer(rawListRow, addedAtText string, now time.Time) (limited bool, permanent bool, until time.Time) {
	if rawListRow == "" {
		return false, false, time.Time{}
	}
	// 归一化（剥 HTML 标签后整体扫描——标记可能在名后/副标题区任意位置）
	text := regexp.MustCompile(`<[^>]+>`).ReplaceAllString(rawListRow, " ")
	if reLimitedNoTransfer.MatchString(text) {
		age := ParseRelativeAge(addedAtText)
		return true, false, now.Add(24*time.Hour - age + 30*time.Minute)
	}
	// 永久禁转：含"禁转"但非限时形态（"限时"优先已排除）
	if rePermanentNoXfer.MatchString(text) {
		return false, true, time.Time{}
	}
	return false, false, time.Time{}
}
