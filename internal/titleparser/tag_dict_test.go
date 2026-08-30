package titleparser

import "testing"

// §59.35 P4: tag 域查询表行为锚定（迁移自 tag_inferer if 链的关键语义）
func TestTagInferAnchors(t *testing.T) {
	cases := []struct {
		name string
		in   TagInferInput
		want []string // 含 ApplyTagRules 前的原始命中（顺序 = dict 序）
	}{
		{"空输入零命中", TagInferInput{Title: "Normal Movie"}, nil},
		// §59.151: HDR 族文本 regex 废除（幽灵 hdr10 根因——MI 尾段/标题字样误命中）——
		// 判据移 tag_inferer.inferHDRTagsFromMI（MI Video 层 HDR format 含 Profile 语义）
		{"HDR 族标题字样不命中(§59.151)", TagInferInput{Title: "Movie DV HDR HDR10+", MediaInfo: "no dv here"}, nil},
		{"HDR 族 MI 文本字样不命中(§59.151)", TagInferInput{Title: "Movie", MediaInfo: "Video\nHDR format : Dolby Vision Profile 5"}, nil},
		{"中字 MI Text 段", TagInferInput{Title: "Movie", MediaInfo: "Text #1: Chinese"}, []string{"chinese_subtitle"}},
		{"国语排除国家", TagInferInput{Title: "中国国家地理纪录片"}, nil},
		{"国语 PTGen 行", TagInferInput{Description: "◎语　言　汉语普通话"}, []string{"chinese_audio"}},
		{"complete S01 无 E", TagInferInput{Title: "Show S01 Complete"}, []string{"complete"}},
		{"complete 全N集副标题", TagInferInput{Title: "Show", Subtitle: "全12集"}, []string{"complete"}},
		{"S01E01 不判 complete(分集命中)", TagInferInput{Title: "Show S01E01"}, []string{"episode_split"}},
	}
	for _, c := range cases {
		got := TagInferMatches(c.in)
		if len(got) != len(c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%s: got %v, want %v", c.name, got, c.want)
				break
			}
		}
	}
}

// §59.35 P4: TagGroups 分组结构派生（TagSelector 数据源）
func TestTagGroupsStructure(t *testing.T) {
	groups := TagGroups()
	if len(groups) != 9 {
		t.Errorf("分组数 %d, want 57（-dubbed §59.111）", len(groups))
	}
	total := 0
	seen := map[string]bool{}
	for _, g := range groups {
		for _, tok := range g.Tags {
			total++
			if seen[tok.Canonical] {
				t.Errorf("tag 词条重复: %q", tok.Canonical)
			}
			seen[tok.Canonical] = true
			if tok.tagLabel() == "" {
				t.Errorf("tag %q label 为空", tok.Canonical)
			}
		}
	}
	if total != 57 {
		t.Errorf("tag 词条 %d, want 56（+§59.85 A类缺口）", total)
	}
	// 关键词条锚定
	for _, k := range []string{"dolby_vision", "chinese_subtitle", "chinese_audio", "complete", "10_bit", "hdr10_plus", "auro_3d"} {
		if !seen[k] {
			t.Errorf("tag 缺关键词条 %q", k)
		}
	}
}

// §59.73: 直采标签归一——DOM 显示名(源站 torrent_tag) → canonical。
func TestNormalizeTagDisplay(t *testing.T) {
	cases := []struct{ in, want string }{
		{"中字", "chinese_subtitle"},
		{"特效", "special_effects_subs"},
		{"特效字幕", "special_effects_subs"},
		{"国语", "chinese_audio"},
		{"杜比", "dolby_vision"},
		{"HDR", "hdr10"},
		{"高码率", "high_bitrate"},
		{"完结", "complete"},
		{"自定义标签", "自定义标签"}, // miss 保留原文
		{"", ""},
	}
	for _, c := range cases {
		if got := NormalizeTagDisplay(c.in); got != c.want {
			t.Errorf("NormalizeTagDisplay(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
