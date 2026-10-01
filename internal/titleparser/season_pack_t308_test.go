package titleparser

import "testing"

// §59.308 完结季包季集归一——海的开始案（S01E01-S01E10 区间形态幸运种审判单集拒）
func Test308_NormalizeSeasonPack(t *testing.T) {
	cases := []struct {
		se           string
		complete     bool
		ptgenEps     int
		want         string
	}{
		// A 通道：tags complete → 全季区间归一 S01
		{"S01E01-S01E10", true, 0, "S01"},
		{"S01E01-E10", true, 0, "S01"},
		// B 通道：PTGen episodes=末集 → 归一（无 complete tag）
		{"S01E01-S01E10", false, 10, "S01"},
		// B 不匹配（PTGen=12 末集=10 非全集）→ 不动
		{"S01E01-S01E10", false, 12, "S01E01-S01E10"},
		// 双通道皆无 → 不动
		{"S01E01-S01E10", false, 0, "S01E01-S01E10"},
		// 真分集（非 E01 起）→ 永不动
		{"S01E03-E05", true, 0, "S01E03-E05"},
		// 单集/纯季号 → 不动
		{"S01E03", true, 0, "S01E03"},
		{"S01", false, 0, "S01"},
		{"", true, 10, ""},
	}
	for _, c := range cases {
		if got := NormalizeSeasonPack(c.se, c.complete, c.ptgenEps); got != c.want {
			t.Errorf("NormalizeSeasonPack(%q, complete=%v, eps=%d)=%q want %q", c.se, c.complete, c.ptgenEps, got, c.want)
		}
	}
}
