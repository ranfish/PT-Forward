package publish

import "testing"


// §59.69: 高码/高帧推导（用户五判据定案）
// 高码: ①文案 标题+副标题(高码率|高码|高比特率) ②MI ≥4K && Overall ≥15Mb/s ③MI <4K && Overall ≥9Mb/s
// 高帧: ④文案 标题+副标题(高帧率|高帧|60FPS|120FPS) ⑤MI Frame rate ≥60（59.940 不算，NTSC 不算）
func TestInferHighBitrate(t *testing.T) {
	mi4k := "General\nComplete name : a.mkv\nOverall bit rate : 15.2 Mb/s\nVideo\nWidth : 3 840 pixels\nFrame rate : 23.976 FPS\n"
	mi4kLow := "General\nOverall bit rate : 14.9 Mb/s\nVideo\nWidth : 3 840 pixels\n"
	mi1080 := "General\nOverall bit rate : 9.1 Mb/s\nVideo\nWidth : 1 920 pixels\nFrame rate : 23.976 FPS\n"
	mi1080Low := "General\nOverall bit rate : 8.9 Mb/s\nVideo\nWidth : 1 920 pixels\n"
	musicMI := "General\nOverall bit rate : 987 kb/s\nAudio\nFormat : FLAC\n"
	miNoWidth := "General\nOverall bit rate : 40 Mb/s\nVideo\nFormat : HEVC\n"

	cases := []struct {
		name  string
		in    TagInput
		hasHB bool // high_bitrate
		hasHF bool // high_frame_rate
	}{
		{"文案高码率", TagInput{Title: "电影.2024.2160p 高码率"}, true, false},
		{"文案高码", TagInput{Title: "电影.2024 高码"}, true, false},
		{"文案高比特率", TagInput{Subtitle: "高比特率版本"}, true, false},
		{"MI 4K 15.2Mb/s", TagInput{Title: "M.2024", MediaInfo: mi4k}, true, false},
		{"MI 4K 14.9Mb/s 不达标", TagInput{Title: "M.2024", MediaInfo: mi4kLow}, false, false},
		{"MI 1080p 9.1Mb/s", TagInput{Title: "M.2024", MediaInfo: mi1080}, true, false},
		{"MI 1080p 8.9Mb/s 不达标", TagInput{Title: "M.2024", MediaInfo: mi1080Low}, false, false},
		{"音乐 MI 不推导", TagInput{Title: "Album", MediaInfo: musicMI}, false, false},
		{"MI 无宽度不数值判定", TagInput{Title: "M.2024", MediaInfo: miNoWidth}, false, false},
		{"8K 宽 7680 ≥4K 阈值15", TagInput{Title: "M", MediaInfo: "General\nOverall bit rate : 20 Mb/s\nVideo\nWidth : 7 680 pixels\n"}, true, false},
		{"文案60FPS", TagInput{Title: "剧集 60FPS"}, false, true},
		{"文案高帧率", TagInput{Subtitle: "高帧率"}, false, true},
		{"MI 60FPS", TagInput{Title: "M", MediaInfo: "General\nOverall bit rate : 5 Mb/s\nVideo\nWidth : 1 920 pixels\nFrame rate : 60.000 FPS\n"}, false, true},
		{"MI 59.940 不算", TagInput{Title: "M", MediaInfo: "General\nOverall bit rate : 5 Mb/s\nVideo\nWidth : 1 920 pixels\nFrame rate : 59.940 FPS\n"}, false, false},
		{"kb/s 单位解析不误判", TagInput{Title: "M", MediaInfo: "General\nOverall bit rate : 8 500 kb/s\nVideo\nWidth : 1 920 pixels\nFrame rate : 24 FPS\n"}, false, false},
	}
	for _, c := range cases {
		got := NewMediaTagInferer().InferFull(c.in)
		has := func(k string) bool { for _, g := range got { if g == k { return true } }; return false }
		if has("high_bitrate") != c.hasHB || has("high_frame_rate") != c.hasHF {
			t.Errorf("%s: high_bitrate=%v(want %v) high_frame_rate=%v(want %v) got=%v",
				c.name, has("high_bitrate"), c.hasHB, has("high_frame_rate"), c.hasHF, got)
		}
	}
}

// §59.70: 高分标签——豆瓣评分 ≥8.0（8.0 算 7.9 不算；无评分/暂无评分不命中）。
// 评分行来自 PTGen 简介落库后（"◎豆瓣评分　8.2/10 (N 人评价)"，全角空格），
// t0 推断时 PTGen 未落库 → applyPosterFallback t2 后重推补上（api 层锚）。
func TestInferHighRating(t *testing.T) {
	cases := []struct {
		name string
		desc string
		want bool
	}{
		{"8.2 命中", "◎豆瓣评分　8.2/10 (19214 人评价)", true},
		{"8.0 边界含", "◎豆瓣评分　8.0/10 (500 人评价)", true},
		{"7.9 不算", "◎豆瓣评分　7.9/10 (99999 人评价)", false},
		{"无评分行", "◎豆瓣评分　暂无评分", false},
		{"完全没有", "剧情简介正文", false},
		{"IMDb 行不消费", "◎IMDb评分  8.5/10 (61992 人评价)", false},
		{"全角空格+普通空格变体", "◎豆瓣评分 8.6/10", true},
	}
	for _, c := range cases {
		got := NewMediaTagInferer().InferFull(TagInput{Title: "M.2024", Description: c.desc})
		has := false
		for _, g := range got {
			if g == "high_rating" {
				has = true
			}
		}
		if has != c.want {
			t.Errorf("%s: high_rating=%v want %v (desc=%q)", c.name, has, c.want, c.desc)
		}
	}
}

// §59.71 B1: 分集/合集/完结 三标签（藏宝阁 7.6 判定优先级权威）。
func TestInferEpisodeSplitCollectionComplete(t *testing.T) {
	cases := []struct {
		name         string
		title, sub   string
		wantSplit    bool // episode_split
		wantColl     bool // collection
		wantComplete bool // complete
	}{
		{"S01E01 分集", "Show.S01E01.1080p", "", true, false, false},
		{"EP01 分集", "Show.EP01.1080p", "", true, false, false},
		{"第1集 分集", "剧.第1集", "", true, false, false},
		{"E13-E21 范围分集", "Show.E13-E21.1080p", "", true, false, false},
		{"S01-S02 合集", "Show.S01-S02.1080p", "", false, true, false},
		{"副标题合集", "Show.S01", "第1-2季合集", false, true, false},
		{"副标题集全", "Show.S01", "全集", false, false, true},
		{"已完结", "Show.2024", "已完结", false, false, true},
		{"全34集 完结", "剧.全34集", "", false, false, true},
		{"电影无标识", "Movie.2024.1080p", "", false, false, false},
		{"分集在场压制完结(藏宝阁7.6第一句)", "Show.S01E01.全34集", "", true, false, false},
	}
	for _, c := range cases {
		got := NewMediaTagInferer().InferFull(TagInput{Title: c.title, Subtitle: c.sub})
		has := func(k string) bool { for _, g := range got { if g == k { return true } }; return false }
		if has("episode_split") != c.wantSplit || has("collection") != c.wantColl || has("complete") != c.wantComplete {
			t.Errorf("%s: split=%v coll=%v complete=%v got=%v", c.name, has("episode_split"), has("collection"), has("complete"), got)
		}
	}
}

// §59.72 B2: 连载(episode_split && !complete && !collection 组合) + 大包(size>1TB, AGSV 硬规则)。
func TestInferOngoingAndBigPack(t *testing.T) {
	const tb = 1024 * 1024 * 1024 * 1024
	cases := []struct {
		name         string
		title        string
		size         int64
		wantOngoing  bool
		wantBigPack  bool
		wantComplete bool
	}{
		{"分集→连载", "Show.S01E05.1080p", 0, true, false, false},
		{"完结抑制连载", "Show.S01.1080p", 0, false, false, true},
		{"合集抑制连载", "Show.S01-S02.1080p", 0, false, false, false},
		{"1TB+1B 大包", "Movie.2024.2160p", tb + 1, false, true, false},
		{"恰 1TB 不算(>1T 字面)", "Movie.2024.2160p", tb, false, false, false},
		{"999G 不算", "Movie.2024.2160p", tb - 5*1024*1024*1024, false, false, false},
		{"分集+大包 独立", "Show.S01E05.UHD", 2 * tb, true, true, false},
		{"电影无形态", "Movie.2024.1080p", 20 * 1024 * 1024 * 1024, false, false, false},
	}
	for _, c := range cases {
		got := NewMediaTagInferer().InferFull(TagInput{Title: c.title, Size: c.size})
		has := func(k string) bool { for _, g := range got { if g == k { return true } }; return false }
		if has("ongoing") != c.wantOngoing || has("big_pack") != c.wantBigPack || has("complete") != c.wantComplete {
			t.Errorf("%s: ongoing=%v big_pack=%v complete=%v got=%v", c.name, has("ongoing"), has("big_pack"), has("complete"), got)
		}
	}
}

// §59.73 B1 补遗: 特效字幕(17 站消费, B1 清单漏做)——auto_feed 全站判据 特效字幕/特效中字,
// 副标题+标题(title_raw) + 简介第二源(desc_raw, HDRoute 行为)。
func TestInferSpecialEffectsSubs(t *testing.T) {
	cases := []struct {
		name, title, sub, desc, stmt string
		want                         bool
	}{
		{"副标题特效字幕", "Show.S01", "特效字幕", "", "", true},
		{"副标题特效中字", "Show.S01", "特效中字", "", "", true},
		{"标题特效字幕", "Movie.2024.特效字幕", "", "", "", true},
		{"引用声明特效字幕(第二源)", "Movie.2024", "", "", "[quote]内封特效字幕[/quote]", true},
		{"PTGen 剧情简介特效不命中", "Movie.2024", "", "影片以特效见长", "", false},
		{"裸特效不命中(防误判)", "Movie.2024.特效大片", "", "", "", false},
		{"无关", "Movie.2024", "简体中文", "", "", false},
	}
	for _, c := range cases {
		got := NewMediaTagInferer().InferFull(TagInput{Title: c.title, Subtitle: c.sub, Description: c.desc, Statement: c.stmt})
		has := false
		for _, g := range got {
			if g == "special_effects_subs" {
				has = true
			}
		}
		if has != c.want {
			t.Errorf("%s: special_effects_subs=%v want %v", c.name, has, c.want)
		}
	}
}

// §59.74: 直采+推断合并单点——跨源冲突在合并结果上仲裁(发布时不再出现互斥并存)。
func TestMergeTags(t *testing.T) {
	cases := []struct {
		name             string
		existing, in     []string
		want             []string
	}{
		{"同语义去重(直采显示名归一)", []string{"特效", "国语"}, []string{"special_effects_subs", "chinese_audio"}, []string{"special_effects_subs", "chinese_audio"}},
		{"跨源互斥直采赢(完结vs分集)", []string{"complete"}, []string{"episode_split"}, []string{"complete"}},
		{"跨源互斥直采赢(分集vs完结)", []string{"episode_split"}, []string{"complete"}, []string{"episode_split"}},
		{"覆盖规则作用于合并结果", []string{"hdr10"}, []string{"hdr10_plus"}, []string{"hdr10_plus"}},
		{"miss 显示名保留", []string{"自定义标签"}, nil, []string{"自定义标签"}},
		{"空并集", nil, nil, nil},
	}
	for _, c := range cases {
		got := MergeTags(c.existing, c.in)
		if len(got) != len(c.want) {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%s: got %v want %v", c.name, got, c.want)
				break
			}
		}
	}
}

// §59.85: A类缺口 9 词条——文案判据 + 4K/8K 数值判据。
func TestInferGapTags(t *testing.T) {
	cases := []struct {
		name string
		in   TagInput
		wants []string
	}{
		{"原盘文案", TagInput{Title: "Movie.2024.BD原盘"}, []string{"original_disc"}},
		{"3D token", TagInput{Title: "Movie.3D.BluRay.1080p"}, []string{"3d"}},
		{"4K 数值", TagInput{Title: "M", MediaInfo: "General\nVideo\nWidth : 3 840 pixels\n"}, []string{"resolution_4k"}},
		{"8K 数值", TagInput{Title: "M", MediaInfo: "General\nVideo\nWidth : 7 680 pixels\n"}, []string{"resolution_8k"}},
		{"1080p 非4K", TagInput{Title: "M", MediaInfo: "General\nVideo\nWidth : 1 920 pixels\n"}, nil},
		{"超分", TagInput{Subtitle: "AI放大"}, []string{"upscale"}},
		{"补帧", TagInput{Subtitle: "补帧"}, []string{"frame_interp"}},
		{"短剧副标题", TagInput{Title: "剧.2024", Subtitle: "短剧 全集"}, []string{"short_play"}},
	}
	for _, c := range cases {
		got := NewMediaTagInferer().InferFull(c.in)
		for _, w := range c.wants {
			found := false
			for _, g := range got { if g == w { found = true } }
			if !found { t.Errorf("%s: 缺 %q got=%v", c.name, w, got) }
		}
		if c.wants == nil {
			for _, g := range got {
				if g == "resolution_4k" || g == "resolution_8k" { t.Errorf("%s: 不应命中 %q", c.name, g) }
			}
		}
	}
}

// §59.105: 语言族补缺——english_audio/bilingual_audio。
func TestInferLanguageGapTags(t *testing.T) {
	cases := []struct {
		name string
		in   TagInput
		want string
	}{
		{"副标题英语", TagInput{Subtitle: "英语 简繁字幕"}, "english_audio"},
		{"国英双语", TagInput{Subtitle: "国英双语 简繁字幕"}, "bilingual_audio"},
		{"中英双语", TagInput{Subtitle: "中英双语"}, "bilingual_audio"},
		{"英语字幕不算音轨", TagInput{Subtitle: "简英字幕"}, ""},
	}
	for _, c := range cases {
		got := NewMediaTagInferer().InferFull(c.in)
		found := false
		for _, g := range got { if g == c.want { found = true } }
		if found != (c.want != "") {
			t.Errorf("%s: %q found=%v", c.name, c.want, found)
		}
	}
}

// §59.109: 语言组判据补齐——日语/韩语/配音（原空 pattern）。
func TestInferLangAudioCompletion(t *testing.T) {
	cases := []struct {
		name string
		in   TagInput
		want string
	}{
		{"副标题日语", TagInput{Subtitle: "日语 中字"}, "japanese_audio"},
		{"MI Japanese", TagInput{MediaInfo: "Audio #1\nLanguage : Japanese"}, "japanese_audio"},
		{"副标题韩语", TagInput{Subtitle: "韩语"}, "korean_audio"},
		// §59.111: dubbed 已删（0 站消费+语言标签完全吸收: 国配/台配→国语、粤配→粤语）
		{"MI Title Mandarin 国配注释→国语", TagInput{MediaInfo: "Audio #2\nTitle : Mandarin (央视国配)"}, "chinese_audio"},
		{"英语字幕不算配音", TagInput{Subtitle: "英字"}, ""},
	}
	for _, c := range cases {
		got := NewMediaTagInferer().InferFull(c.in)
		found := false
		for _, g := range got { if g == c.want { found = true } }
		if found != (c.want != "") {
			t.Errorf("%s: want %q found=%v got=%v", c.name, c.want, found, got)
		}
	}
}
