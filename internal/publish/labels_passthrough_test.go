package publish

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.576（v0.0.1623·馒头 1298316 labels=0 案）：副标中字通道锚。
// Zhizn 副标「俄语 | 简中字幕」形态——窄词表（中字/中文字幕/简中/简繁/
// 简体/繁体）命中 chinese_subtitle；特效字幕/英文字幕不收（窄词表纪律）。
func TestSubtitleFromSubtitleKeywords(t *testing.T) {
	inf := NewMediaTagInferer()
	contains := func(tags []string, k string) bool {
		for _, x := range tags {
			if x == k {
				return true
			}
		}
		return false
	}
	cases := []struct {
		name, subtitle string
		want           bool
	}{
		{"Zhizn 简中字幕", "【生活与命运】全一季 | 俄语 | 简中字幕 | HEVC版本", true},
		{"中字裸词", "全一季 | 国语中字", true},
		{"中文字幕", "4K | 国语 | 中文字幕", true},
		{"简繁", "简繁双语字幕", true},
		{"英文字幕不收", "英语 | 英文字幕", false},
		{"特效字幕不收", "国语 | 特效字幕", false},
		{"无字幕词", "全一季 | 俄语", false},
	}
	for _, c := range cases {
		tags := inf.Infer("", "Some Title 2020 1080p BluRay x265", c.subtitle)
		if got := contains(tags, "chinese_subtitle"); got != c.want {
			t.Errorf("%s: chinese_subtitle=%v, want %v（tags=%v）", c.name, got, c.want, tags)
		}
	}
}

// §59.576（v0.0.1623）：labels 位掩码消费键放行锚——无 tags 表单域映射的
// 站（馒头形态·标记走 labels 位掩码）chinese_audio/chinese_subtitle/diy
// 不被 allowed 过滤杀掉；无消费方的键（hdr10）仍被过滤。
func TestAssembleTagsBitmaskConsumerPassthrough(t *testing.T) {
	e := &PublishExecutor{}
	// 馒头形态：无 tags 值映射（标记走 labels 位掩码·无 tags 表单域）
	cfg := &model.PublishFormConfig{ValueMappings: map[string][]model.FormValueMapping{}}
	meta := &model.TorrentMetadata{
		Title: "Zhizn I Sudba S01 2012 1080p BluRay x265",
		Tags:  `["chinese_subtitle","chinese_audio","diy","hdr10"]`,
	}
	tags := e.assembleTags(cfg, meta, nil)
	has := func(k string) bool {
		for _, x := range tags {
			if x == k {
				return true
			}
		}
		return false
	}
	if !has("chinese_subtitle") || !has("chinese_audio") || !has("diy") {
		t.Errorf("位掩码消费键应放行: got %v", tags)
	}
	if has("hdr10") {
		t.Errorf("无消费方键仍应被过滤: got %v", tags)
	}
}
