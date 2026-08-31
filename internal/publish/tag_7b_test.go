package publish

import (
	"github.com/ranfish/pt-forward/internal/titleparser"
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
	// §59.151: HDR 族 MI 层结构化（Video 段 HDR format 唯一通道；文本字样废除）
	tags := inferer.Infer("Video\nHDR format : SMPTE ST 2086, HDR10 compatible", "电影 2024")
	if !containsTag(tags, "hdr10") {
		t.Errorf("should infer hdr10, got %v", tags)
	}
	// 纯文本字样（旧 regex 通道）不再命中
	tags2 := inferer.Infer("Video: HDR10", "电影 2024 HDR10")
	if containsTag(tags2, "hdr10") {
		t.Errorf("text-only hdr10 should not infer (§59.151), got %v", tags2)
	}
}

func TestMediaTagInferer_HDR10Plus(t *testing.T) {
	inferer := NewMediaTagInferer()
	tags := inferer.Infer("Video\nHDR format : SMPTE ST 2094-40, HDR10+", "电影.hdr10+")
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
	// dvhe.08 = DV+HDR10 双层（§59.151 Profile 语义——两 tag 同产）
	tags := inferer.Infer("Video\nHDR format : Dolby Vision, Version 1.0, Profile 8.1, dvhe.08.06, BL+RPU, no metadata compression, HDR10", "电影")
	if !containsTag(tags, "dolby_vision") || !containsTag(tags, "hdr10") {
		t.Errorf("dvhe.08 should infer dv+hdr10 (dual layer), got %v", tags)
	}
	// §59.154：dvhe.05/dvav.09 = 仅 DV（P5 无兼容层 IPTPQc2 / P9 8-bit AVC+SDR 兼容）
	for _, hdr := range []string{
		"Dolby Vision, Version 1.0, Profile 5, dvhe.05.06, BL+RPU", // SNYLV 2160p 实证
		"Dolby Vision, Version 1.0, Profile 9, dvav.09.06, BL+RPU", // 理论构造（P9=AVC）
	} {
		tags2 := inferer.Infer("Video\nHDR format : "+hdr, "电影")
		if !containsTag(tags2, "dolby_vision") || containsTag(tags2, "hdr10") {
			t.Errorf("P5/P9 应仅 dv 不勾 hdr10（%s）, got %v", hdr, tags2)
		}
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
	// §59.151: audio 语言族 MI 单点（MI Audio 段 Language/Title——标题声明废除）
	tags := inferer.Infer("Audio #1\nLanguage : Chinese", "电影 2024")
	if !containsTag(tags, "chinese_audio") {
		t.Errorf("should infer chinese_audio, got %v", tags)
	}
	tags2 := inferer.Infer("", "电影 国语 2024")
	if containsTag(tags2, "chinese_audio") {
		t.Errorf("title-only chinese_audio should not infer (§59.151), got %v", tags2)
	}
}

func TestMediaTagInferer_CantoneseAudio(t *testing.T) {
	inferer := NewMediaTagInferer()
	tags := inferer.Infer("Audio #1\nTitle : Cantonese", "电影 2024")
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

// §59.146: 灰度构造——flag 命中站从 sites_source_keys 静态构造 SiteTagConfig

func TestHasHKOriginalTrack(t *testing.T) {
	mk := func(lang, title string) titleparser.MISections {
		return titleparser.MISections{Audios: []map[string]string{{"language": lang, "title": title}}}
	}
	// 霹雳火正例：香港 + Chinese|原声
	if !hasHKOriginalTrack(mk("Chinese", "原声"), "中国香港") {
		t.Error("香港+Chinese原声 应判粤语(霹雳火)")
	}
	// 一个好人反例：English|原声（英语原声）
	if hasHKOriginalTrack(mk("English", "原声"), "中国香港") {
		t.Error("香港+English原声 不应判粤语(一个好人反例)")
	}
	// 非香港
	if hasHKOriginalTrack(mk("Chinese", "原声"), "日本") {
		t.Error("非香港 不应判")
	}
	// 香港但 Title 非精确"原声"
	if hasHKOriginalTrack(mk("Chinese", "国语"), "中国香港") {
		t.Error("香港+国语Title 非原声 不应判")
	}
}

func TestHasMandarinTitledTrack(t *testing.T) {
	sec := titleparser.MISections{Audios: []map[string]string{
		{"title": "Mandarin (粤配)"}, // 迫降航班：粤配轨标记 Mandarin——反证生效
	}}
	if !hasMandarinTitledTrack(sec) {
		t.Error("Mandarin Title 应为反证(迫降航班)")
	}
	sec2 := titleparser.MISections{Audios: []map[string]string{
		{"language": "Chinese"}, {"title": "粤语"},
	}}
	if hasMandarinTitledTrack(sec2) {
		t.Error("无 Mandarin/国语 标记不应反证")
	}
	sec3 := titleparser.MISections{Audios: []map[string]string{{"title": "上译国配"}}}
	if !hasMandarinTitledTrack(sec3) {
		t.Error("国配 Title 应反证")
	}
}

func TestInferSubtitleFromMITexts(t *testing.T) {
	sec := titleparser.MISections{Texts: []map[string]string{
		{"language": "Chinese", "title": "CHS&ENG"}, // 挽救计划形态
	}}
	out := inferSubtitleFromMITexts(sec)
	if !out["chinese_subtitle"] {
		t.Error("Text Language=Chinese 应产中字(挽救计划)")
	}
	// Title-only 形态（无 Language 行——CHS/CHT 压制组标记）
	sec2 := titleparser.MISections{Texts: []map[string]string{
		{"title": "CHT"}, {"title": "eng"},
	}}
	out2 := inferSubtitleFromMITexts(sec2)
	if !out2["chinese_subtitle"] || !out2["english_subtitle"] {
		t.Error("Title-only CHS/CHT+eng 应产中字+英字(四通道防御)")
	}
	// Commentary 字幕轨排除
	sec3 := titleparser.MISections{Texts: []map[string]string{
		{"language": "English", "title": "Commentary subtitles"},
	}}
	out3 := inferSubtitleFromMITexts(sec3)
	if out3["english_subtitle"] {
		t.Error("Commentary 字幕轨应排除")
	}
}

// InferFull 集成：粤语复合三场景（霹雳火/天空之城/迫降航班）
func TestCantoneseCompositeScenarios(t *testing.T) {
	inf := NewMediaTagInferer()
	// 场景1 霹雳火：region 香港 + Chinese|原声 → cantonese
	mi1 := "Audio #1\nLanguage : Chinese\nTitle : 原声\n"
	tags1 := inf.InferFull(TagInput{MediaInfo: mi1, Title: "Thunderbolt", Region: "中国香港"})
	found := false
	for _, tg := range tags1 {
		if tg == "cantonese_audio" { found = true }
	}
	if !found {
		t.Errorf("霹雳火场景应产 cantonese, got %v", tags1)
	}
	// 场景2 迫降航班：副标题含粤 + MI Mandarin 标记 → 不产
	mi2 := "Audio #1\nLanguage : Chinese\nTitle : Mandarin (粤配)\n"
	tags2 := inf.InferFull(TagInput{MediaInfo: mi2, Title: "Flight", Subtitle: "国粤台英音轨", Region: "美国"})
	found2 := false
	for _, tg := range tags2 {
		if tg == "cantonese_audio" { found2 = true }
	}
	if found2 {
		t.Errorf("迫降航班场景(Mandarin反证)不应产 cantonese, got %v", tags2)
	}
	// 场景3 天空之城：副标题粤 + MI 无 Mandarin 标记 → 产
	mi3 := "Audio #1\nLanguage : Japanese\nAudio #2\nLanguage : Chinese\n"
	tags3 := inf.InferFull(TagInput{MediaInfo: mi3, Title: "Laputa", Subtitle: "国粤英日四语", Region: "日本"})
	found3 := false
	for _, tg := range tags3 {
		if tg == "cantonese_audio" { found3 = true }
	}
	if !found3 {
		t.Errorf("天空之城场景(副标题声明无反证)应产 cantonese, got %v", tags3)
	}
}

// inferHDRTagsFromMI 直测（dvhe.08 双层/ST2086/空）
func TestInferHDRTagsFromMI(t *testing.T) {
	mkV := func(hdr string) titleparser.MISections {
		return titleparser.MISections{Videos: []map[string]string{{"hdr format": hdr}}}
	}
	// dvhe.08 = DV+HDR10 双层
	out := inferHDRTagsFromMI(mkV("Dolby Vision, Version 1.0, Profile 8.1, dvhe.08.06, BL+RPU, no metadata compression, HDR10"), nil)
	if !containsStr(out, "dolby_vision") || !containsStr(out, "hdr10") {
		t.Errorf("dvhe.08 应双产 dv+hdr10, got %v", out)
	}
	// ST2086 = HDR10
	out2 := inferHDRTagsFromMI(mkV("SMPTE ST 2086, HDR10 compatible"), nil)
	if !containsStr(out2, "hdr10") || containsStr(out2, "dolby_vision") {
		t.Errorf("ST2086 应仅 hdr10, got %v", out2)
	}
	// 空 = 全不产
	out3 := inferHDRTagsFromMI(mkV(""), nil)
	if len(out3) != 0 {
		t.Errorf("空 HDR format 应零产出, got %v", out3)
	}
	// 无 Video 段 = 剔除后返回（纯音频种子）
	out4 := inferHDRTagsFromMI(titleparser.MISections{}, []string{"hdr10", "other"})
	if containsStr(out4, "hdr10") || !containsStr(out4, "other") {
		t.Errorf("无 Video 段应剔除 HDR 族保留其它, got %v", out4)
	}
	// ST2094 = HDR10+ 不叠 HDR10
	out5 := inferHDRTagsFromMI(mkV("SMPTE ST 2094-40, HDR10+"), nil)
	if !containsStr(out5, "hdr10_plus") || containsStr(out5, "hdr10") {
		t.Errorf("HDR10+ 应仅 hdr10_plus, got %v", out5)
	}
}

// §59.151 附7: 英语标签条件产出（lucky TAGS_MISSING_ENGLISH_AUDIO 实证）
// Buried.Alive.1990（纯英语 FLAC/PGS-EN）探针：勾→100 / 不勾→WARN 扣分
func TestLuckyEnglishAudioConditional(t *testing.T) {
	inf := NewMediaTagInferer()
	// 场景1 纯英语（Buried Alive 形态）：Audio English + Text English → lucky_english_audio
	mi1 := "Audio #1\nFormat : FLAC\nLanguage : English\nText #1\nLanguage : English\n"
	tags1 := inf.InferFull(TagInput{MediaInfo: mi1, Title: "Buried Alive 1990"})
	if !containsStr(tags1, "lucky_english_audio") {
		t.Errorf("纯英语种应产 lucky_english_audio, got %v", tags1)
	}
	// 场景2 英语音轨+中文字幕（爱情抓马形态）→ 不产
	mi2 := mi1 + "Text #2\nLanguage : Chinese\n"
	tags2 := inf.InferFull(TagInput{MediaInfo: mi2, Title: "The Drama 2026"})
	if containsStr(tags2, "lucky_english_audio") {
		t.Errorf("有中字不应产 lucky_english_audio, got %v", tags2)
	}
	// 场景3 国语音轨+英语音轨 → 不产
	mi3 := "Audio #1\nFormat : TrueHD\nLanguage : English\nAudio #2\nLanguage : Chinese\nTitle : Mandarin\n"
	tags3 := inf.InferFull(TagInput{MediaInfo: mi3, Title: "Movie"})
	if containsStr(tags3, "lucky_english_audio") {
		t.Errorf("有国语不应产 lucky_english_audio, got %v", tags3)
	}
}
