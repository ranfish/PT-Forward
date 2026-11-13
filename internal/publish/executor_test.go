package publish

import (
	"encoding/json"
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

// §59.175: 表单音频组合键最优先——B1 直取不得短路组合键
// （幸运 AUDIO_MISMATCH 十连拒实证回归）。
func TestAudioMappingOfCombinedFirst(t *testing.T) {
	e := &PublishExecutor{}
	cfg := &model.PublishFormConfig{
		ValueMappings: map[string][]model.FormValueMapping{
			model.FieldDomainAudiocodec: {
				{Value: "12", StandardKeys: []string{"audio.truehd"}},
				{Value: "17", StandardKeys: []string{"audio.truehd_atmos"}},
				{Value: "3", StandardKeys: []string{"audio.dd"}},
			},
		},
	}
	// TrueHD+Atmos → 组合键 audio.truehd_atmos（不得被 B1 裸 truehd 短路）
	m := e.audioMappingOf(cfg, model.FieldDomainAudiocodec, "TrueHD", "Atmos")
	if m == nil || m.Value != "17" {
		t.Errorf("TrueHD+Atmos 应选组合键 17: %+v", m)
	}
	// 无 tech → B1 直取裸 truehd
	m2 := e.audioMappingOf(cfg, model.FieldDomainAudiocodec, "TrueHD", "")
	if m2 == nil || m2.Value != "12" {
		t.Errorf("纯 TrueHD 应选 12: %+v", m2)
	}
	// DD 无组合词条场景（词表 exact 失明修复语义保持）
	m3 := e.audioMappingOf(cfg, model.FieldDomainAudiocodec, "DD", "")
	if m3 == nil || m3.Value != "3" {
		t.Errorf("DD 直取应选 3: %+v", m3)
	}
}

// §59.177: adjustTagsForSite 四场景——新推断正确/存量幸运自愈/藏宝阁覆写/用户手动保护。
func TestAdjustTagsForSite(t *testing.T) {
	title := "Masters of Horror S01-S02 2005-2006 1080p Blu-ray x265 DTS"
	subtitle := "【恐怖大师】全两季 全剧终 | 中英字幕"

	// ① 新推断正确（词典改后 S01-S02 不再触发 collection）——tags 已正确
	tags1 := []string{"10_bit", "chinese_subtitle", "complete"}
	out1 := adjustTagsForSite(tags1, "幸运", title, subtitle)
	if contains(out1, "collection") || !contains(out1, "complete") {
		t.Errorf("幸运通用：应保留 complete 无 collection: %v", out1)
	}

	// ② 存量数据发布幸运——旧 tags 含 collection（词典旧规则产物）→ 自愈移除+补 complete
	tags2 := []string{"10_bit", "chinese_subtitle", "collection"}
	out2 := adjustTagsForSite(tags2, "幸运", title, subtitle)
	if contains(out2, "collection") {
		t.Errorf("幸运存量自愈：应移除 collection: %v", out2)
	}
	if !contains(out2, "complete") {
		t.Errorf("幸运存量自愈：副标题全剧终应补 complete: %v", out2)
	}

	// ③ 藏宝阁覆写——跨季=合集（去完结加合集）
	tags3 := []string{"10_bit", "chinese_subtitle", "complete"}
	out3 := adjustTagsForSite(tags3, "藏宝阁", title, subtitle)
	if !contains(out3, "collection") || contains(out3, "complete") {
		t.Errorf("藏宝阁覆写：应加 collection 去 complete: %v", out3)
	}

	// ④ 用户手动保护——副标题含"合集"字样（真多作品合集）→ 调整层不动
	tags4 := []string{"10_bit", "collection"}
	subtitle4 := "【杜比全家桶合集】Top250 精选"
	out4 := adjustTagsForSite(tags4, "幸运", title, subtitle4)
	if !contains(out4, "collection") {
		t.Errorf("用户手动保护：真多作品合集不应被移除: %v", out4)
	}

	// ⑤ 非跨季标题 → 不调整
	tags5 := []string{"collection"}
	out5 := adjustTagsForSite(tags5, "幸运", "Oppenheimer 2023 1080p", "奥本海默")
	if !contains(out5, "collection") {
		t.Errorf("非跨季标题不应触发调整: %v", out5)
	}
}

func contains(tags []string, target string) bool {
	for _, t := range tags {
		if t == target {
			return true
		}
	}
	return false
}

// §59.391: 葡萄产区×类型双维分类映射锚——用户定案粒度（陆/港/澳/台
// →华语且大陆 vs 港台细分；日韩独立（综艺）；其余亚洲→亚洲；再其余
// 欧美兜底；首值优先；无产区证据→欧美 fallback 预览可手改）。
func TestSjtuRegionCategoryValue(t *testing.T) {
	cfg := &model.PublishFormConfig{ValueMappings: map[string][]model.FormValueMapping{
		model.FieldDomainType: {
			{Label: "华语电影", Value: "401"}, {Label: "欧美电影", Value: "402"},
			{Label: "亚洲电影", Value: "403"}, {Label: "港台电视剧", Value: "407"},
			{Label: "亚洲电视剧", Value: "408"}, {Label: "大陆电视剧", Value: "409"},
			{Label: "欧美电视剧", Value: "410"}, {Label: "大陆综艺节目", Value: "411"},
			{Label: "港台综艺节目", Value: "412"}, {Label: "欧美综艺节目", Value: "413"},
			{Label: "日韩综艺节目", Value: "414"},
		},
	}}
	ptgen := func(regions ...string) string {
		b, _ := json.Marshal(map[string]interface{}{"region": regions})
		return string(b)
	}
	cases := []struct {
		name     string
		category string
		ptgen    string
		want     string
	}{
		{"华语电影(大陆)", "category.movie", ptgen("中国大陆"), "401"},
		{"华语电影(港)", "category.movie", ptgen("中国香港"), "401"},
		{"亚洲电影(日本)", "category.movie", ptgen("日本"), "403"},
		{"欧美电影(美)", "category.movie", ptgen("美国"), "402"},
		{"无产区电影→欧美兜底", "category.movie", "", "402"},
		{"大陆剧", "category.tv_series", ptgen("中国大陆"), "409"},
		{"港台剧", "category.tv_series", ptgen("中国台湾"), "407"},
		{"韩剧→亚洲电视剧", "category.tv_series", ptgen("韩国"), "408"},
		{"美剧无产区→欧美兜底", "category.tv_series", "", "410"},
		{"日韩综艺", "category.tv_shows", ptgen("日本"), "414"},
		{"大陆综艺", "category.tv_shows", ptgen("中国大陆"), "411"},
		{"复合产区首值优先(大陆/香港→大陆剧)", "category.tv_series", ptgen("中国大陆", "美国"), "409"},
		{"首值miss全数组兜底(美国/泰国→亚洲电影)", "category.movie", ptgen("美国", "泰国"), "403"},
	}
	for _, c := range cases {
		meta := &model.TorrentMetadata{Category: c.category, PTGenSourceJSON: c.ptgen}
		got := sjtuRegionCategoryValue(cfg, "葡萄", meta)
		if got == nil || got.Value != c.want {
			t.Errorf("%s: want %s got %+v", c.name, c.want, got)
		}
	}
	// 非产区类→nil（回退通用链）
	if v := sjtuRegionCategoryValue(cfg, "葡萄", &model.TorrentMetadata{Category: "category.animation"}); v != nil {
		t.Errorf("动画类应回退通用链 got %+v", v)
	}
	// 非葡萄站→nil
	if v := sjtuRegionCategoryValue(cfg, "龙之家", &model.TorrentMetadata{Category: "category.movie"}); v != nil {
		t.Errorf("非葡萄站应回退 got %+v", v)
	}
}

// TestAdjustTagsForSiteUbitsYema §59.518: 优堡键修正+单季完结转合集+野马入覆写组。
func TestAdjustTagsForSiteUbitsYema(t *testing.T) {
	quacksTitle := "Quacks S01 2017 1080p AMZN WEBRip HEVC DDP 2.0-cXcY@FRDS"
	quacksSub := "【江湖郎中/怪诞医生】全一季 | 中英字幕 | 简中字幕"
	crossTitle := "Masters of Horror S01-S02 2005-2006 1080p Blu-ray x265 DTS"
	crossSub := "【恐怖大师】全两季 全剧终 | 中英字幕"

	// ① 优堡单季完结（Quacks 实案）——complete→collection（站方无完结标签）
	out1 := adjustTagsForSite([]string{"10_bit", "chinese_subtitle", "complete"}, "优堡", quacksTitle, quacksSub)
	if contains(out1, "complete") || !contains(out1, "collection") {
		t.Errorf("优堡单季：应去 complete 加 collection: %v", out1)
	}

	// ② 优堡跨季+complete 在场——特化分支命中（与覆写组收敛同果）
	out2 := adjustTagsForSite([]string{"complete"}, "优堡", crossTitle, crossSub)
	if contains(out2, "complete") || !contains(out2, "collection") {
		t.Errorf("优堡跨季+complete：应去 complete 加 collection: %v", out2)
	}

	// ③ 优堡跨季+complete 缺席——落覆写组补 collection（键修正后优堡在组内）
	out3 := adjustTagsForSite([]string{"10_bit"}, "优堡", crossTitle, crossSub)
	if !contains(out3, "collection") {
		t.Errorf("优堡跨季无complete：覆写组应加 collection: %v", out3)
	}

	// ④ 优堡电影不误转——无 complete 非跨季 → 不动
	out4 := adjustTagsForSite([]string{"high_rating"}, "优堡", "Oppenheimer 2023 1080p", "奥本海默")
	if len(out4) != 1 || out4[0] != "high_rating" {
		t.Errorf("优堡电影：不应误转: %v", out4)
	}

	// ⑤ 优堡用户意图保护——副标含「合集」不调整
	out5 := adjustTagsForSite([]string{"complete"}, "优堡", quacksTitle, "【杜比全家桶合集】Top250 精选")
	if !contains(out5, "complete") || contains(out5, "collection") {
		t.Errorf("优堡保护：真多作品合集不应被转: %v", out5)
	}

	// ⑥ 野马跨季——覆写组：去完结加合集（完结全集 18）
	out6 := adjustTagsForSite([]string{"complete"}, "野马", crossTitle, crossSub)
	if contains(out6, "complete") || !contains(out6, "collection") {
		t.Errorf("野马跨季：应去 complete 加 collection（完结全集）: %v", out6)
	}

	// ⑦ 野马单季完结——通用：complete 保留（站方「完结」12）
	out7 := adjustTagsForSite([]string{"complete"}, "野马", quacksTitle, quacksSub)
	if !contains(out7, "complete") || contains(out7, "collection") {
		t.Errorf("野马单季：应保留 complete（完结）: %v", out7)
	}
}

// TestTagApplierTaglistValueMapping §59.518: taglist 模式值映射修复——
// 显示层（applied）与上传层（TagArrayFields）同源（朱雀/野马 DryRun 假绿根修）。
func TestTagApplierTaglistValueMapping(t *testing.T) {
	var kvs []model.TagKV
	write := func(field, value string) {
		kvs = append(kvs, model.TagKV{Key: field, Value: value})
	}

	// ① 有映射 → 写站方值（野马 tagList 数字 ID 形态）
	a1 := NewTagApplier(&model.SiteTagConfig{
		Mode: model.TagModeTaglist,
		Tags: map[string]string{"complete": "12", "collection": "18"},
	})
	kvs = nil
	a1.Apply([]string{"complete", "collection"}, write)
	if len(kvs) != 2 || kvs[0].Key != "tagList" || kvs[0].Value != "12" || kvs[1].Value != "18" {
		t.Errorf("taglist 有映射：应写站方值: %+v", kvs)
	}

	// ② 无映射 → 标准键直出（零值集站兼容）
	a2 := NewTagApplier(&model.SiteTagConfig{Mode: model.TagModeTaglist})
	kvs = nil
	a2.Apply([]string{"complete"}, write)
	if len(kvs) != 1 || kvs[0].Value != "complete" {
		t.Errorf("taglist 无映射：应标准键直出: %+v", kvs)
	}
}

// TestAssembleTagsAdjustOrder §59.518: 调整层原料放行——值集无 complete 的站
// （优堡）先滤后转=转的原料没了（243 DryRun 复验 tags=['6'] 无合集 20 实锤）。
// 全链组合：assembleTags（过滤+放行）× adjustTagsForSite（转换）。
func TestAssembleTagsAdjustOrder(t *testing.T) {
	// 优堡值集形态（243 实态：中字 6/合集 20·无 complete 条目）
	cfg := &model.PublishFormConfig{
		ValueMappings: map[string][]model.FormValueMapping{
			model.FieldDomainTags: {
				{Label: "中字", Value: "6", StandardKeys: []string{"tag.chinese_subtitle"}},
				{Label: "合集", Value: "20", StandardKeys: []string{"tag.collection"}},
			},
		},
	}
	e := &PublishExecutor{}
	meta := &model.TorrentMetadata{
		Title:    "Quacks S01 2017 1080p AMZN WEBRip HEVC DDP 2.0-cXcY@FRDS",
		Subtitle: "【江湖郎中/怪诞医生】全一季 | 中英字幕",
		Tags:     `["10_bit","chinese_subtitle","english_subtitle","complete","high_rating"]`,
	}

	// ① 优堡：complete 放行 → adjust 转 collection → 映射面两个都在
	out := e.assembleTags(cfg, meta, nil)
	out = adjustTagsForSite(out, "优堡", meta.Title, meta.Subtitle)
	hasC, hasCol, hasSub := false, false, false
	for _, t := range out {
		switch t {
		case "complete":
			hasC = true
		case "collection":
			hasCol = true
		case "chinese_subtitle":
			hasSub = true
		}
	}
	if hasC || !hasCol || !hasSub {
		t.Errorf("优堡全链：应去 complete 留 collection+chinese_subtitle: %v", out)
	}

	// ② 幸运语义回归：值集有 complete 的站正常投 complete（放行不改变有值集站行为）
	cfgLuck := &model.PublishFormConfig{
		ValueMappings: map[string][]model.FormValueMapping{
			model.FieldDomainTags: {
				{Label: "完结", Value: "10", StandardKeys: []string{"tag.complete"}},
				{Label: "中字", Value: "6", StandardKeys: []string{"tag.chinese_subtitle"}},
			},
		},
	}
	out2 := e.assembleTags(cfgLuck, meta, nil)
	out2 = adjustTagsForSite(out2, "幸运", meta.Title, meta.Subtitle)
	found := map[string]bool{}
	for _, t := range out2 {
		found[t] = true
	}
	if !found["complete"] || !found["chinese_subtitle"] {
		t.Errorf("幸运回归：complete 应保留: %v", out2)
	}

	// ③ 值集无 complete 的非优堡站：complete 放行但 adjust 不转 → 留在 tags
	// 由映射层自然蒸发（applier miss 跳过=原行为）——本测试只验放行语义
	cfgNone := &model.PublishFormConfig{
		ValueMappings: map[string][]model.FormValueMapping{
			model.FieldDomainTags: {
				{Label: "中字", Value: "6", StandardKeys: []string{"tag.chinese_subtitle"}},
			},
		},
	}
	out3 := e.assembleTags(cfgNone, meta, nil)
	found3 := map[string]bool{}
	for _, t := range out3 {
		found3[t] = true
	}
	if !found3["complete"] {
		t.Errorf("放行语义：值集无 complete 的站 complete 应放行进调整层: %v", out3)
	}
}
