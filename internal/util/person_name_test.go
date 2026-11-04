package util

import "testing"

// §59.462: 人名中英拆分测试锚（PTGen 形态实锚——director/cast 实测词形）。
func TestSplitPersonName(t *testing.T) {
	cases := []struct {
		in    string
		name  string
		sub   string
		desc  string
	}{
		{"克里斯·帕拉特 Chris Pratt", "Chris Pratt", "克里斯·帕拉特", "中英混排（PTGen 主形态）"},
		{"提莫·贝克曼贝托夫 Timur Bekmambetov", "Timur Bekmambetov", "提莫·贝克曼贝托夫", "长中文名"},
		{"Quentin Tarantino", "Quentin Tarantino", "", "纯英文"},
		{"张艺谋", "", "张艺谋", "纯中文"},
		{"", "", "", "空"},
		{"  克里斯·帕拉特   Chris Pratt  ", "Chris Pratt", "克里斯·帕拉特", "前后空白容错"},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			got := SplitPersonName(c.in)
			if got.Name != c.name || got.SubName != c.sub {
				t.Errorf("in=%q got Name=%q SubName=%q want %q/%q", c.in, got.Name, got.SubName, c.name, c.sub)
			}
		})
	}
}
