package publish

import "testing"

// §59.451: 馒头 labels 位掩码翻译锚（站方 API 定义 bit0=DIY/bit1=中配/
// bit2=中字——DryRun 不透出 ExtraFields，纯函数锚兜底）。
func TestMteamLabelsBits(t *testing.T) {
	cases := []struct {
		name string
		tags []string
		want int
	}{
		{"空标签", nil, 0},
		{"仅 DIY", []string{"diy"}, 1},
		{"仅中配", []string{"chinese_audio"}, 2},
		{"仅中字", []string{"chinese_subtitle"}, 4},
		{"中字+国语（形态一实证）", []string{"chinese_subtitle", "chinese_audio"}, 6},
		{"三标全中", []string{"diy", "chinese_audio", "chinese_subtitle"}, 7},
		{"非白名单键不翻（HDR/DV 无 bit）", []string{"hdr10", "dolby_vision"}, 0},
		{"混合白名单与其它", []string{"hdr10", "chinese_subtitle", "english_subtitle"}, 4},
	}
	for _, c := range cases {
		if got := mteamLabelsBits(c.tags); got != c.want {
			t.Errorf("%s: mteamLabelsBits(%v)=%d want %d", c.name, c.tags, got, c.want)
		}
	}
}
