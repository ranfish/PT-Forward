package metadata

import (
	"context"
	"testing"
)

// §59.542 刀3 补: 获取链声明过滤回调——回流声明剥除（Pope 案路径）
func TestFetcherFilterDecl(t *testing.T) {
	f := &Fetcher{}
	siteNorm := "[quote]\n[b][color=#0000ff][size=5]FRDS torrent官组作品，感谢原制作者发布。[/size][/color][/b][/quote]\n\n[quote]\n[b][color=#ff0000][size=5]互珍互重，禁转PTT[/size][/color][/b][/quote]"

	// ① 未装配（nil）——原样返回（测试/离线驱动零行为）
	if got := f.filterDecl(context.Background(), siteNorm); got != siteNorm {
		t.Errorf("[未装配] 应原样: got=%q", got)
	}
	// ② 装配后——回流声明剥净（词表命中整块删——模拟 declFilter 行为）
	f.DeclFilter = func(_ context.Context, s string) string {
		// 模拟 Filter：两块都含词表词（互珍互重/官组作品感谢原制作者）→ 剥净
		return ""
	}
	if got := f.filterDecl(context.Background(), siteNorm); got != "" {
		t.Errorf("[装配] 应剥净: got=%q", got)
	}
	// ③ 空 statement 短路
	if got := f.filterDecl(context.Background(), ""); got != "" {
		t.Errorf("[空] got=%q", got)
	}
}
