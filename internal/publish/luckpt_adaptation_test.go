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
	// 产地华语区 + 硬字幕 → 补
	tags := luckptChineseSubtitleHeuristic("幸运", luckptMeta("国语硬字幕 内嵌中字", "中国香港"), nil)
	if len(tags) != 1 || tags[0] != "chinese_subtitle" {
		t.Errorf("[硬字幕+香港] got %v", tags)
	}
	// 中英字幕 + 大陆 → 补
	tags = luckptChineseSubtitleHeuristic("幸运", luckptMeta("中英字幕", "中国大陆"), nil)
	if len(tags) != 1 || tags[0] != "chinese_subtitle" {
		t.Errorf("[中英字幕+大陆] got %v", tags)
	}
	// 非华语区不补
	tags = luckptChineseSubtitleHeuristic("幸运", luckptMeta("硬字幕", "美国"), nil)
	if len(tags) != 0 {
		t.Errorf("[美国] got %v", tags)
	}
	// 华语区但无字幕说明不补
	tags = luckptChineseSubtitleHeuristic("幸运", luckptMeta("4K HDR", "中国大陆"), nil)
	if len(tags) != 0 {
		t.Errorf("[无说明] got %v", tags)
	}
	// 已有中字不重复
	tags = luckptChineseSubtitleHeuristic("幸运", luckptMeta("硬字幕", "中国大陆"), []string{"chinese_subtitle"})
	if len(tags) != 1 {
		t.Errorf("[已有] got %v", tags)
	}
	// 非幸运站不动
	tags = luckptChineseSubtitleHeuristic("修道院", luckptMeta("硬字幕", "中国大陆"), nil)
	if len(tags) != 0 {
		t.Errorf("[非幸运] got %v", tags)
	}
}
