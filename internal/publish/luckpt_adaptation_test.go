package publish

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.273: Other 兜底三域（幸运 Other 无 standard_key 形态）
func otherCfg() *model.PublishFormConfig {
	return &model.PublishFormConfig{
		ValueMappings: map[string][]model.FormValueMapping{
			model.FieldDomainCodec: {
				{Label: "H.264", Value: "1", StandardKeys: []string{"video.h264"}},
				{Label: "Other", Value: "5"},
			},
			model.FieldDomainAudiocodec: {
				{Label: "AAC", Value: "1", StandardKeys: []string{"audio.aac"}},
				{Label: "Other", Value: "7"},
			},
			model.FieldDomainTeam: {
				{Label: "FRDS", Value: "1", StandardKeys: []string{"team.frds"}},
				{Label: "Other", Value: "5"},
			},
		},
	}
}

func TestOtherFallback_ThreeDomains(t *testing.T) {
	e := &PublishExecutor{}
	cfg := otherCfg()
	// Opus → audio Other（幸运 Robot Dreams 种审案）
	if m := e.audioMappingOf(cfg, "Opus", ""); m == nil || m.Label != "Other" {
		t.Errorf("[audio Opus] got %v", m)
	}
	// 原命中不回归
	if m := e.audioMappingOf(cfg, "AAC", ""); m == nil || m.Label != "AAC" {
		t.Errorf("[audio AAC] got %v", m)
	}
	// codec AV1 → Other（无键形态）
	if m := e.codecMappingOf(cfg, "AV1"); m == nil || m.Label != "Other" {
		t.Errorf("[codec AV1] got %v", m)
	}
	// team 无匹配组 → Other
	if m := e.teamMapping(cfg, &model.TorrentMetadata{Title: "Some.Title.2020-CMCT"}); m == nil || m.Label != "Other" {
		t.Errorf("[team CMCT] got %v", m)
	}
}

// §59.273: 幸运中字启发
func luckptMeta(subtitle, ptgenRegion string) *model.TorrentMetadata {
	src := "{\"region\": [\"" + ptgenRegion + "\"]}"
	if ptgenRegion == "" {
		src = "{}"
	}
	return &model.TorrentMetadata{Subtitle: subtitle, PTGenSourceJSON: src}
}

func TestLuckptChineseSubtitleHeuristic(t *testing.T) {
	// §59.278 终版：产地忽略——三关键词任一即补
	for _, sub := range []string{"国语硬字幕 内嵌中字", "中英字幕", "硬字幕", "内嵌中字", "中字"} {
		tags := luckptChineseSubtitleHeuristic("幸运", luckptMeta(sub, "美国"), nil)
		if len(tags) != 1 || tags[0] != "chinese_subtitle" {
			t.Errorf("[%s+美国] got %v", sub, tags)
		}
	}
	// 无中字否定式不误配（§59.277 Deep Water 案）
	tags := luckptChineseSubtitleHeuristic("幸运", luckptMeta("[英语/无中字]", "美国"), nil)
	if len(tags) != 0 {
		t.Errorf("[无中字] got %v", tags)
	}
	// 无关键词不补
	tags = luckptChineseSubtitleHeuristic("幸运", luckptMeta("4K HDR", "中国大陆"), nil)
	if len(tags) != 0 {
		t.Errorf("[无说明] got %v", tags)
	}
	// 已有中字不重复
	tags = luckptChineseSubtitleHeuristic("幸运", luckptMeta("硬字幕", "美国"), []string{"chinese_subtitle"})
	if len(tags) != 1 {
		t.Errorf("[已有] got %v", tags)
	}
	// 非幸运站不动
	tags = luckptChineseSubtitleHeuristic("修道院", luckptMeta("硬字幕", "中国大陆"), nil)
	if len(tags) != 0 {
		t.Errorf("[非幸运] got %v", tags)
	}
}

// §59.277: 幸运英语标签站规（Deep Water 种审案）
func luckptTagCfg() *model.PublishFormConfig {
	return &model.PublishFormConfig{
		ValueMappings: map[string][]model.FormValueMapping{
			model.FieldDomainTags: {
				{Label: "中字", Value: "6", StandardKeys: []string{"tag.chinese_subtitle"}},
				{Label: "国语", Value: "5", StandardKeys: []string{"tag.chinese_audio"}},
				{Label: "粤语", Value: "14", StandardKeys: []string{"tag.cantonese_audio"}},
				{Label: "英语", Value: "22", Auto: boolPtr(false)},
			},
		},
	}
}

func boolPtr(b bool) *bool { return &b }

func TestLuckptEnglishTagValue(t *testing.T) {
	cfg := luckptTagCfg()
	meta := &model.TorrentMetadata{Subtitle: "[英语/无中字] 2160p AMZN"}
	// ① Deep Water 案：无中文标签 + 副标题英语/无中字 → 22
	if v := luckptEnglishTagValue(cfg, "幸运", meta, nil); v != "22" {
		t.Errorf("[Deep Water] got %q want 22", v)
	}
	// ② english_audio 推断路径（无副标题声明）
	meta2 := &model.TorrentMetadata{}
	if v := luckptEnglishTagValue(cfg, "幸运", meta2, []string{"english_audio"}); v != "22" {
		t.Errorf("[english_audio] got %q", v)
	}
	// ③ lucky_english_audio 路径（§59.151 附7 产出键）
	if v := luckptEnglishTagValue(cfg, "幸运", meta2, []string{"lucky_english_audio"}); v != "22" {
		t.Errorf("[lucky_english_audio] got %q", v)
	}
	// ④ 有中字（§59.273 启发补的也算）→ 不要求英语
	if v := luckptEnglishTagValue(cfg, "幸运", meta, []string{"chinese_subtitle"}); v != "" {
		t.Errorf("[有中字] got %q", v)
	}
	// ⑤ 无英语证据 → 不注入
	meta3 := &model.TorrentMetadata{Subtitle: "4K HDR"}
	if v := luckptEnglishTagValue(cfg, "幸运", meta3, nil); v != "" {
		t.Errorf("[无证据] got %q", v)
	}
	// ⑥ 非幸运站不动
	if v := luckptEnglishTagValue(cfg, "修道院", meta, nil); v != "" {
		t.Errorf("[非幸运] got %q", v)
	}
	// ⑦ 站表无英语选项 → 空
	cfgNoEn := luckptTagCfg()
	cfgNoEn.ValueMappings[model.FieldDomainTags] = cfgNoEn.ValueMappings[model.FieldDomainTags][:3]
	if v := luckptEnglishTagValue(cfgNoEn, "幸运", meta, nil); v != "" {
		t.Errorf("[无英语选项] got %q", v)
	}
}

// §59.278: 硬字幕无条件中字 + HDR 标签标题对齐

func TestSyncHDRTagFromProfile(t *testing.T) {
	// 十三猎杀案形态：HDR10 无 MI 标签 → 补 hdr10
	if got := syncHDRTagFromProfile("HDR10", nil); len(got) != 1 || got[0] != "hdr10" {
		t.Errorf("[HDR10] got %v", got)
	}
	if got := syncHDRTagFromProfile("HDR Vivid", nil); len(got) != 1 || got[0] != "hdr_vivid" {
		t.Errorf("[Vivid] got %v", got)
	}
	if got := syncHDRTagFromProfile("DoVi HDR", nil); len(got) != 1 || got[0] != "dolby_vision" {
		t.Errorf("[DoVi] got %v", got)
	}
	if got := syncHDRTagFromProfile("HDR10+", nil); len(got) != 1 || got[0] != "hdr10_plus" {
		t.Errorf("[HDR10+] got %v", got)
	}
	// MI 铁证优先不覆盖
	if got := syncHDRTagFromProfile("HDR10", []string{"hdr_vivid"}); len(got) != 1 {
		t.Errorf("[已有族] got %v", got)
	}
	// 空/SDR 不动
	if got := syncHDRTagFromProfile("SDR", nil); len(got) != 0 {
		t.Errorf("[SDR] got %v", got)
	}
	if got := syncHDRTagFromProfile("", nil); len(got) != 0 {
		t.Errorf("[空] got %v", got)
	}
}
