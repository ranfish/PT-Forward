package publish

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

// ===== tag_rules.go 测试 =====

func TestApplyTagRules_Empty(t *testing.T) {
	if result := ApplyTagRules(nil); result != nil {
		t.Errorf("empty input → nil, got %v", result)
	}
}

func TestApplyTagRules_CoverRule(t *testing.T) {
	// hdr10_plus 存在时移除 hdr10
	tags := []string{"hdr10", "hdr10_plus"}
	result := ApplyTagRules(tags)
	if containsTag(result, "hdr10") {
		t.Errorf("hdr10 should be removed when hdr10_plus present, got %v", result)
	}
	if !containsTag(result, "hdr10_plus") {
		t.Errorf("hdr10_plus should be kept")
	}
}

func TestApplyTagRules_MutexGroup(t *testing.T) {
	// 互斥组内仅保留首个命中的
	tags := []string{"hdr10", "hlg"}
	result := ApplyTagRules(tags)
	if len(result) != 1 {
		t.Errorf("mutex group: only 1 should remain, got %v", result)
	}
	if result[0] != "hdr10" {
		t.Errorf("first hit should be kept, got %q", result[0])
	}
}

func TestApplyTagRules_DVCoexist(t *testing.T) {
	// DV 与 HDR10 可共存
	tags := []string{"dolby_vision", "hdr10"}
	result := ApplyTagRules(tags)
	if !containsTag(result, "dolby_vision") || !containsTag(result, "hdr10") {
		t.Errorf("DV and HDR10 should coexist, got %v", result)
	}
}

func TestApplyTagRules_Dedup(t *testing.T) {
	tags := []string{"hdr10", "hdr10", "remux", "remux"}
	result := ApplyTagRules(tags)
	if len(result) != 2 {
		t.Errorf("should dedup to 2, got %d: %v", len(result), result)
	}
}

// ===== tag_inferer.go 测试 =====

func TestMediaTagInferer_HDR10(t *testing.T) {
	inferer := NewMediaTagInferer()
	tags := inferer.Infer("Video: HDR10", "电影 2024 HDR10")
	if !containsTag(tags, "hdr10") {
		t.Errorf("should infer hdr10, got %v", tags)
	}
}

func TestMediaTagInferer_HDR10Plus(t *testing.T) {
	inferer := NewMediaTagInferer()
	tags := inferer.Infer("HDR10+ stream", "电影.hdr10+")
	if !containsTag(tags, "hdr10_plus") {
		t.Errorf("should infer hdr10_plus, got %v", tags)
	}
	// hdr10 应被覆盖规则移除
	if containsTag(tags, "hdr10") {
		t.Errorf("hdr10 should be covered by hdr10_plus, got %v", tags)
	}
}

func TestMediaTagInferer_DolbyVision(t *testing.T) {
	inferer := NewMediaTagInferer()
	tags := inferer.Infer("Dolby Vision", "电影 DoVi")
	if !containsTag(tags, "dolby_vision") {
		t.Errorf("should infer dolby_vision, got %v", tags)
	}
}

func TestMediaTagInferer_ChineseSubtitle(t *testing.T) {
	inferer := NewMediaTagInferer()
	tags := inferer.Infer("", "电影 中字 2024")
	if !containsTag(tags, "chinese_subtitle") {
		t.Errorf("should infer chinese_subtitle, got %v", tags)
	}
}

func TestMediaTagInferer_ChineseAudio(t *testing.T) {
	inferer := NewMediaTagInferer()
	tags := inferer.Infer("", "电影 国语 2024")
	if !containsTag(tags, "chinese_audio") {
		t.Errorf("should infer chinese_audio, got %v", tags)
	}
}

func TestMediaTagInferer_CantoneseAudio(t *testing.T) {
	inferer := NewMediaTagInferer()
	tags := inferer.Infer("", "电影 粤语 2024")
	if !containsTag(tags, "cantonese_audio") {
		t.Errorf("should infer cantonese_audio, got %v", tags)
	}
}

func TestMediaTagInferer_Atmos(t *testing.T) {
	inferer := NewMediaTagInferer()
	tags := inferer.Infer("Dolby Atmos", "电影 atmos")
	if !containsTag(tags, "dolby_atmos") {
		t.Errorf("should infer dolby_atmos, got %v", tags)
	}
}

func TestMediaTagInferer_Remux(t *testing.T) {
	inferer := NewMediaTagInferer()
	tags := inferer.Infer("", "电影 2024 Remux")
	if !containsTag(tags, "remux") {
		t.Errorf("should infer remux, got %v", tags)
	}
}

func TestMediaTagInferer_10Bit(t *testing.T) {
	inferer := NewMediaTagInferer()
	tags := inferer.Infer("Bit depth: 10 bit", "电影 10bit")
	if !containsTag(tags, "10_bit") {
		t.Errorf("should infer 10_bit, got %v", tags)
	}
}

func TestMediaTagInferer_Empty(t *testing.T) {
	inferer := NewMediaTagInferer()
	tags := inferer.Infer("", "普通电影")
	if len(tags) != 0 {
		t.Errorf("empty: should infer nothing, got %v", tags)
	}
}

// ===== tag_applier.go 测试 =====

func TestTagApplier_TaglistMode(t *testing.T) {
	applier := NewTagApplier(&model.SiteTagConfig{Mode: model.TagModeTaglist})
	var fields []struct{ field, value string }
	applier.Apply([]string{"hdr10", "chinese_subtitle"}, func(f, v string) {
		fields = append(fields, struct{ field, value string }{f, v})
	})
	if len(fields) != 2 {
		t.Fatalf("taglist: should write 2 fields, got %d", len(fields))
	}
	if fields[0].field != "tagList" || fields[0].value != "hdr10" {
		t.Errorf("taglist field 0 mismatch: %+v", fields[0])
	}
}

func TestTagApplier_CheckboxIDMode(t *testing.T) {
	config := &model.SiteTagConfig{
		Mode: model.TagModeCheckboxID,
		Tags: map[string]string{
			"chinese_subtitle": "tag_chs_sub",
			"hdr10":            "tag_hdr10",
		},
	}
	applier := NewTagApplier(config)
	written := map[string]string{}
	applier.Apply([]string{"hdr10", "chinese_subtitle", "unknown"}, func(f, v string) {
		written[f] = v
	})
	if written["tag_hdr10"] != "on" {
		t.Errorf("checkbox_id: tag_hdr10 should be 'on', got %q", written["tag_hdr10"])
	}
	if written["tag_chs_sub"] != "on" {
		t.Errorf("checkbox_id: tag_chs_sub should be 'on'")
	}
	if _, ok := written["unknown"]; ok {
		t.Errorf("unknown tag should be skipped")
	}
}

func TestTagApplier_CheckboxSpanMode(t *testing.T) {
	config := &model.SiteTagConfig{
		Mode: model.TagModeCheckboxSpan,
		Tags: map[string]string{
			"chinese_audio": "5",
			"chinese_subtitle": "6",
		},
	}
	applier := NewTagApplier(config)
	var spanValues []string
	applier.Apply([]string{"chinese_audio"}, func(f, v string) {
		if f == "span[]" {
			spanValues = append(spanValues, v)
		}
	})
	if len(spanValues) != 1 || spanValues[0] != "5" {
		t.Errorf("checkbox_span mismatch: %v", spanValues)
	}
}

func TestTagApplier_IndependentFieldMode(t *testing.T) {
	config := &model.SiteTagConfig{
		Mode: model.TagModeIndependentField,
		Tags: map[string]string{
			"chinese_audio": "is_mandrain",
		},
	}
	applier := NewTagApplier(config)
	written := map[string]string{}
	applier.Apply([]string{"chinese_audio"}, func(f, v string) {
		written[f] = v
	})
	if written["is_mandrain"] != "1" {
		t.Errorf("independent_field: is_mandrain should be '1', got %q", written["is_mandrain"])
	}
}

func TestTagApplier_NilConfig(t *testing.T) {
	applier := NewTagApplier(nil)
	called := false
	applier.Apply([]string{"hdr10"}, func(f, v string) {
		called = true
		if f != "tagList" || v != "hdr10" {
			t.Errorf("nil config: should use taglist mode, got %s=%s", f, v)
		}
	})
	if !called {
		t.Error("nil config: should still apply tags")
	}
}

func TestTagApplier_IsSupported(t *testing.T) {
	config := &model.SiteTagConfig{
		Mode: model.TagModeCheckboxID,
		Tags: map[string]string{"hdr10": "tag_hdr10"},
	}
	applier := NewTagApplier(config)
	if !applier.IsSupported("hdr10") {
		t.Error("hdr10 should be supported")
	}
	if applier.IsSupported("unknown") {
		t.Error("unknown should not be supported")
	}
}

func TestTagApplier_TaglistAlwaysSupported(t *testing.T) {
	applier := NewTagApplier(&model.SiteTagConfig{Mode: model.TagModeTaglist})
	if !applier.IsSupported("anything") {
		t.Error("taglist mode: everything should be supported")
	}
}
