package compliance

import (
	"context"
	"testing"
)

// §59.489: 音乐/影视分流测试锚——School Rumble 案（皇后音乐种文件夹名
// [2005.04.27] 日期前缀命中 DetectAdult bracket_date 规则误判系统禁转）。
// 定案：音乐不存在成人内容——isMusic 跳过 adult 段；禁转词/组保留。
func TestCheckWithSite_MusicBypass(t *testing.T) {
	c := NewChecker(nil, nil)
	ctx := context.Background()
	name := "[2005.04.27] School Rumble School Running"

	// 影视口径：bracket_date 拦截（回归锚——影视链不放松）
	r := c.CheckWithSite(ctx, name, "修道院")
	if r.Passed {
		t.Fatal("video-side: bracket_date should block [YYYY.MM.DD] name")
	}
	if r.Category != "adult" || r.Reason != "bracket_date" {
		t.Fatalf("video-side: want adult/bracket_date, got %s/%s", r.Category, r.Reason)
	}

	// 音乐口径：Pass（音乐不存在成人内容）
	r2 := c.CheckWithSite(ctx, name, "皇后", true)
	if !r2.Passed {
		t.Fatalf("music-side: should pass, got %s/%s", r2.Category, r2.Reason)
	}

	// 音乐禁转保留：音乐种标题含禁转词照样拦
	r3 := c.CheckWithSite(ctx, "[2020.01.01] 禁转 Album FLAC", "皇后", true)
	if r3.Passed {
		t.Fatal("music-side: forbidden keyword should still block")
	}
	if r3.Category != "forbidden_transfer" {
		t.Fatalf("music-side: want forbidden_transfer, got %s", r3.Category)
	}

	// 音乐口径不豁免其它成人形态以外的 DB 规则（变参缺省=影视——
	// 现有调用零破坏验证：无变参等同 false）
	r4 := c.CheckWithSite(ctx, name, "修道院") // 与第一次相同·防变参默认值漂移
	if r4.Passed {
		t.Fatal("default (no opt): should behave as video-side")
	}
}
