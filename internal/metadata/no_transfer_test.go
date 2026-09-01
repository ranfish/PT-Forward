package metadata

import (
	"strings"
	"testing"
	"time"
)

// §59.162: keepfrds 禁转两态判定锚定（用户站点经验权威）。
func TestParseRelativeAge(t *testing.T) {
	cases := map[string]time.Duration{
		"20时46分前":  20*time.Hour + 46*time.Minute,
		"3小时前":     3 * time.Hour,
		"5分钟前":     5 * time.Minute,
		"5天前":      5 * 24 * time.Hour,
		"1月2天前":    365 * 24 * time.Hour,
		"1年3月前":    365 * 24 * time.Hour,
		"":         0,
	}
	for in, want := range cases {
		if got := ParseRelativeAge(in); got != want {
			t.Errorf("ParseRelativeAge(%q)=%v want %v", in, got, want)
		}
	}
}

func TestClassifyNoTransfer(t *testing.T) {
	now := time.Now()
	// 限时禁转（带空格标记形态——列表页实测）+ 发布 20h46m 前 → until≈3h34m 后
	limited, permanent, until := ClassifyNoTransfer(
		"<td>阮玲玉...mUHD-FRDS</td><td>[ ] [ 限时禁转 ]</td>", "20时46分前", now)
	if !limited || permanent {
		t.Fatalf("应识别限时禁转, got limited=%v permanent=%v", limited, permanent)
	}
	remain := time.Until(until)
	if remain < 3*time.Hour || remain > 4*time.Hour {
		t.Errorf("窗口剩余应≈3.5h（24h-20h46m+30m 余量）, got %v", remain)
	}
	// 限时禁转+超龄（2天前发布）→ until 已过（now>until=放行语义由消费方判）
	_, _, until2 := ClassifyNoTransfer("<td>[ 限时禁转 ]</td>", "2天前", now)
	if until2.After(now) {
		t.Errorf("超龄窗口 until 应已过期, got %v", until2)
	}
	// 永久禁转（无"限时"）
	_, permanent2, _ := ClassifyNoTransfer("<td>[ 禁转 ]</td>", "5天前", now)
	if !permanent2 {
		t.Error("[禁转] 应识别永久禁转")
	}
	// 无标记
	limited3, permanent3, _ := ClassifyNoTransfer("<td>普通种子行</td>", "1小时前", now)
	if limited3 || permanent3 {
		t.Error("无标记行应两态皆 false")
	}
	// 空行
	limited4, permanent4, _ := ClassifyNoTransfer("", "1小时前", now)
	if limited4 || permanent4 {
		t.Error("空行应两态皆 false")
	}
	_ = strings.TrimSpace
}
