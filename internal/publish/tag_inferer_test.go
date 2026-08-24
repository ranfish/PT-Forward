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
