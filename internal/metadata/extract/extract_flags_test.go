// §32 keepfrds.md 禁转标记两层检测测试。
package extract

import (
	"reflect"
	"testing"
)

func TestExtractFlags(t *testing.T) {
	p := NewPublicExtractor("test", "测试")

	cases := []struct {
		name     string
		title    string
		subtitle string
		descr    string
		want     []string
	}{
		// 站点标记形态（权威源）
		{"marker [禁转]", "Movie 2024", "", "desc [禁转]", []string{"禁转"}},
		{"marker [限时禁转]", "Movie 2024", "[限时禁转] name", "", []string{"限时禁转", "禁转"}},
		{"marker 限转资源", "Movie", "【限转资源】", "", []string{"限转"}},

		// 关键词长词
		{"kw 禁止转载", "Movie", "", "本资源禁止转载", []string{"禁止转载"}},
		{"kw 严禁转载", "Movie", "", "严禁转载！", []string{"严禁转载"}},

		// 跨词伪命中（v0.0.606 修复的三种子场景）
		{"crossword 严禁转发BT", "How to Make a Killing 2026", "",
			"本资源 发布3天后可转发 注意转发礼节 严禁转发BT感谢理解和支持！", nil},
		{"crossword 禁止转发", "Movie", "", "禁止转发到其它站点", nil},
		{"crossword 谢绝转发", "Movie", "", "谢绝转发，谢谢", nil},

		// 定向禁转（我们 v0.0.597 的致谢模板）
		{"directed 禁转PTT", "Movie", "",
			"[quote]请遵守PT互相遵重共识，禁转PTT[/quote]", nil},
		{"directed 禁转BT", "Movie", "", "禁转BT站", nil},

		// 真禁转
		{"genuine 禁转", "Movie", "", "本资源禁转", []string{"禁转"}},
		{"genuine 限转", "Movie", "", "本资源限转", []string{"限转"}},
		{"genuine 独占", "Movie", "独占资源", "", []string{"独占"}},

		// 无标记
		{"clean", "Movie 2024 2160p", "中字", "普通简介", nil},
	}
	for _, c := range cases {
		got := p.extractFlags(c.title, c.subtitle, c.descr)
		want := c.want
		if want == nil {
			want = []string{}
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %v, want %v", c.name, got, want)
		}
	}
}
