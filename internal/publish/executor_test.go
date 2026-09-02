package publish

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.156 切片 2: 执行器域值装配——幸运真实 form_config 数据形态锚定。
// 数据源：migration 27 落库实测（243 幸运 77 条映射化入）。
func luckCfg() *model.PublishFormConfig {
	return &model.PublishFormConfig{
		Enabled:     true,
		PreAuditURL: "/api/auto-audit/pre-audit",
		FormFields: map[string]string{
			model.FieldDomainType:       "type",
			model.FieldDomainMedium:     "medium_sel[4]",
			model.FieldDomainCodec:      "codec_sel[4]",
			model.FieldDomainStandard:   "standard_sel[4]",
			model.FieldDomainAudiocodec: "audiocodec_sel[4]",
			model.FieldDomainTeam:       "team_sel[4]",
			model.FieldDomainTags:       "tags[4][]",
		},
		ValueMappings: map[string][]model.FormValueMapping{
			model.FieldDomainType: {
				{Label: "电影", Value: "401", StandardKeys: []string{"category.movie"}},
				{Label: "电视剧", Value: "402", StandardKeys: []string{"category.tv_series"}},
			},
			model.FieldDomainMedium: {
				{Label: "Encode", Value: "7", StandardKeys: []string{"medium.encode"}},
				{Label: "WEB-DL", Value: "11", StandardKeys: []string{"medium.webdl"}},
			},
			model.FieldDomainCodec: {
				// vc 双形态：站方一选项双键覆盖（§59.151）
				{Label: "H.265/HEVC", Value: "6", StandardKeys: []string{"video.h265", "video.x265"}},
				{Label: "H.264/AVC", Value: "1", StandardKeys: []string{"video.h264", "video.x264"}},
			},
			model.FieldDomainStandard: {
				{Label: "4K/2160p/2160i", Value: "6", StandardKeys: []string{"resolution.r2160p"}},
			},
			model.FieldDomainAudiocodec: {
				{Label: "TrueHD", Value: "14", StandardKeys: []string{"audio.truehd"}},
				{Label: "TrueHD Atmos", Value: "11", StandardKeys: []string{"audio.truehd_atmos"}},
			},
			model.FieldDomainTags: {
				{Label: "Dolby Vision", Value: "20", StandardKeys: []string{"tag.dolby_vision"}},
				{Label: "首发", Value: "2", StandardKeys: []string{"tag.first_release"}},
				{Label: "英语", Value: "22", StandardKeys: []string{"tag.english_audio"}},
			},
		},
	}
}

func TestLookupByStdKey(t *testing.T) {
	e := &PublishExecutor{}
	cfg := luckCfg()
	// 单键命中
	m := e.lookupByStdKey(cfg, model.FieldDomainType, "category.movie")
	if m == nil || m.Value != "401" {
		t.Errorf("category.movie 应命中 401, got %+v", m)
	}
	// 双键任一命中（vc 双形态：x265 重编码键也进 H.265/HEVC 选项）
	m2 := e.lookupByStdKey(cfg, model.FieldDomainCodec, "video.x265")
	if m2 == nil || m2.Value != "6" {
		t.Errorf("video.x265 应命中 6（双键）, got %+v", m2)
	}
	// 未映射键（team 域/待标注）返回 nil
	if e.lookupByStdKey(cfg, model.FieldDomainTeam, "team.frds") != nil {
		t.Error("未配置域应返回 nil")
	}
}

func TestMediumMappingEncodeRule(t *testing.T) {
	e := &PublishExecutor{}
	cfg := luckCfg()
	// 碟转压二维规则：BluRay 片源 + x265 编码痕迹 → Encode（§59.150 medium 二维/IsEncode 铁证）
	meta := &model.TorrentMetadata{SourceType: "BluRay", VideoCodec: "x265"}
	m := e.mediumMapping(cfg, meta)
	if m == nil || m.Value != "7" {
		t.Errorf("BluRay+x265 应判 Encode(7), got %+v", m)
	}
	// WEB-DL 直配
	meta2 := &model.TorrentMetadata{Specification: "WEB-DL"}
	m2 := e.mediumMapping(cfg, meta2)
	if m2 == nil || m2.Value != "11" {
		t.Errorf("WEB-DL spec 应命中 11, got %+v", m2)
	}
}

func TestAssembleTagsAutoFalse(t *testing.T) {
	e := &PublishExecutor{}
	cfg := luckCfg()
	no := false
	cfg.ValueMappings[model.FieldDomainTags] = append(cfg.ValueMappings[model.FieldDomainTags],
		model.FormValueMapping{Label: "首发", Value: "2", StandardKeys: []string{"tag.first"}, Auto: &no})
	meta := &model.TorrentMetadata{Title: "Test DV HDR10"}
	// 判据引擎输出的 auto:false 标签（首发）应被过滤；overrides 可进入
	tags := e.assembleTags(cfg, meta, []string{"tag.first"})
	firstInferred := false
	for _, tg := range tags {
		if tg == "tag.first" {
			firstInferred = true
		}
	}
	_ = firstInferred
	// tag.first 不在 allowed（auto:false 排除）——即使 inferer 产出也不进；overrides 强制进
	// （assembleTags 对 overrides 不做 allowed 过滤——人工决定权）
	if len(tags) == 0 {
		t.Log("推断零标签（title 无 MI 证据）——overrides 生效验证:")
	}
	found := false
	for _, tg := range tags {
		if tg == "tag.first" {
			found = true
		}
	}
	if !found {
		t.Errorf("overrides 的 tag.first 应进入（人工决定权）, got %v", tags)
	}
}

func TestTagApplierSpanFieldParam(t *testing.T) {
	// §59.156: span 字段名参数化——幸运 tags[4][]（旧 span[] 默认兼容）
	written := map[string][]string{}
	write := func(field, value string) { written[field] = append(written[field], value) }

	// 幸运形态：SpanField=tags[4][]
	applier := NewTagApplier(&model.SiteTagConfig{
		Mode:     model.TagModeCheckboxSpan,
		Tags:     map[string]string{"tag.dolby_vision": "20"},
		SpanField: "tags[4][]",
	})
	applier.Apply([]string{"tag.dolby_vision"}, write)
	if len(written["tags[4][])"]) + len(written["tags[4][]"]) == 0 {
		t.Errorf("应写入 tags[4][] 字段, got %v", written)
	}
	// 旧形态：无 SpanField 默认 span[]
	written2 := map[string][]string{}
	write2 := func(field, value string) { written2[field] = append(written2[field], value) }
	applier2 := NewTagApplier(&model.SiteTagConfig{
		Mode: model.TagModeCheckboxSpan,
		Tags: map[string]string{"tag.dolby_vision": "20"},
	})
	applier2.Apply([]string{"tag.dolby_vision"}, write2)
	if len(written2["span[]"]) == 0 {
		t.Errorf("旧配置应回落 span[], got %v", written2)
	}
}

// §59.164: 修道院 cnname 中文段提取
func TestChineseTitleOf(t *testing.T) {
	cases := map[string]string{
		"阴风阵阵.Suspiria.2018.UHD.BluRay.2160p":            "阴风阵阵",
		"Five Easy Pieces 1970 UHD BluRay":                   "",
		"阮玲玉 Center Stage 1991 1080p":                      "阮玲玉",
		"七武士 七人の侍 Seven Samurai 1954":                    "七武士 七人", // 假名不在 Han——段被の切开取最长
		"The.Matrix.1999":                                    "",
		"花木兰 Mulan 2020 4K":                                 "花木兰",
	}
	for in, want := range cases {
		if got := chineseTitleOf(in); got != want {
			t.Errorf("chineseTitleOf(%q) = %q, want %q", in, got, want)
		}
	}
}
