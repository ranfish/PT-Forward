package publish

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.560 锚：猫站 team 域产区化 + 标签 checkbox 投递。
// 数据源：pterclub.md §7/§8 + 发布页源码实证。
func TestCatRegionToTeamSel(t *testing.T) {
	cases := []struct{ region, want string }{
		{"中国大陆", "1"},
		{"中国香港", "2"},
		{"中国台湾", "3"},
		{"美国", "4"},
		{"韩国", "5"},
		{"日本", "6"},
		{"印度", "7"},
		{"法国", "4"},
		{"未知地区", "8"},
	}
	for _, c := range cases {
		if got := catRegionToTeamSel(c.region); got != c.want {
			t.Errorf("catRegionToTeamSel(%q) = %q, want %q", c.region, got, c.want)
		}
	}
}

func TestCatTeamRegionMapping(t *testing.T) {
	cfg := &model.PublishFormConfig{
		ValueMappings: map[string][]model.FormValueMapping{
			model.FieldDomainTeam: {
				{Label: "大陆 (Mainland,CHN)", Value: "1"},
				{Label: "香港 (HKG,CHN)", Value: "2"},
				{Label: "其它 (Other)", Value: "8"},
			},
		},
	}
	// PTGen region 中国大陆 → team_sel=1
	meta := &model.TorrentMetadata{
		PTGenSourceJSON: `{"region":["中国大陆"]}`,
		Title:           "Movie 2020 1080p-GRP", // 无关组名
	}
	m := catTeamRegionMapping(cfg, meta)
	if m == nil || m.Value != "1" {
		t.Fatalf("PTGen region 中国大陆应映射 team_sel=1: %+v", m)
	}
	// 副标词探（无 PTGen）
	meta2 := &model.TorrentMetadata{Subtitle: "国产 喜剧"}
	m2 := catTeamRegionMapping(cfg, meta2)
	if m2 == nil || m2.Value != "1" {
		t.Fatalf("副标「国产」应映射 team_sel=1: %+v", m2)
	}
	// 兜底 Other
	meta3 := &model.TorrentMetadata{}
	m3 := catTeamRegionMapping(cfg, meta3)
	if m3 == nil || m3.Value != "8" {
		t.Fatalf("无信号应兜底 Other=8: %+v", m3)
	}
}

func TestCatTagCheckboxFields(t *testing.T) {
	// 副标含「国语 简繁英字幕」→ guoyu+zhongzi+ensub
	meta := &model.TorrentMetadata{
		Subtitle: "【站直啰，别趴下】10bit HEVC版本 国语 简繁英字幕",
	}
	applied := []string{} // assembleTags canonical 键（本场景无 tags 域→空）
	kvs := catTagCheckboxFields(meta, applied)
	got := map[string]string{}
	for _, kv := range kvs {
		got[kv.Key] = kv.Value
	}
	if got["guoyu"] != "yes" {
		t.Errorf("副标「国语」应勾 guoyu: %v", got)
	}
	if got["zhongzi"] != "yes" {
		t.Errorf("副标「简繁英字幕」应勾 zhongzi: %v", got)
	}
	if got["ensub"] != "yes" {
		t.Errorf("副标「简繁英字幕」含英字信号应勾 ensub: %v", got)
	}

	// §59.561 圣克莱尔案：中德双语字幕≠英字——「双语」宽词探撤除
	meta1 := &model.TorrentMetadata{
		Subtitle: "【圣克莱尔】10bit HEVC版本 英语 简繁德双语字幕",
	}
	kvs1 := catTagCheckboxFields(meta1, nil)
	for _, kv := range kvs1 {
		if kv.Key == "ensub" {
			t.Errorf("中德双语字幕不应勾 ensub（双语≠英字）: %v", kvs1)
		}
	}

	// canonical 通道（§59.561 ensub 重构主通道）：english_subtitle → ensub
	kvs2 := catTagCheckboxFields(&model.TorrentMetadata{Subtitle: "英语原声"}, []string{"english_subtitle"})
	foundEn := false
	for _, kv := range kvs2 {
		if kv.Key == "ensub" {
			foundEn = true
		}
	}
	if !foundEn {
		t.Errorf("canonical english_subtitle 应勾 ensub: %v", kvs2)
	}

	// canonical 键通道（中字）
	meta2 := &model.TorrentMetadata{Subtitle: ""}
	kvs3 := catTagCheckboxFields(meta2, []string{"chinese_audio", "chinese_subtitle"})
	got2 := map[string]string{}
	for _, kv := range kvs3 {
		got2[kv.Key] = kv.Value
	}
	if got2["guoyu"] != "yes" || got2["zhongzi"] != "yes" {
		t.Errorf("canonical 键应映射 checkbox: %v", got2)
	}

	// 粤语
	meta3 := &model.TorrentMetadata{Subtitle: "粤语版"}
	kvs4 := catTagCheckboxFields(meta3, nil)
	found := false
	for _, kv := range kvs4 {
		if kv.Key == "yueyu" {
			found = true
		}
	}
	if !found {
		t.Errorf("副标「粤语」应勾 yueyu: %v", kvs4)
	}
}

// §59.560 追记：diy 三态锚——资源属性标签（双门：关键词+碟形态源）。
func TestCatTagDIY(t *testing.T) {
	// ① 命中：副标「DIY原盘」+ BluRay 源 → diy=yes
	meta := &model.TorrentMetadata{
		Subtitle:   "国配DIY原盘 简繁英字幕",
		SourceType: "BluRay",
	}
	kvs := catTagCheckboxFields(meta, nil)
	got := false
	for _, kv := range kvs {
		if kv.Key == "diy" && kv.Value == "yes" {
			got = true
		}
	}
	if !got {
		t.Errorf("副标「DIY原盘」+BluRay 应勾 diy: %v", kvs)
	}

	// ② 非碟拦截：标题含 DIY 但 WEB 源 → 不投（组名/DIY 字样碰撞防护）
	meta2 := &model.TorrentMetadata{
		Title:      "Movie 2020 1080p WEB-DL DDP5.1 H265-DIYFans",
		SourceType: "WEB",
	}
	kvs2 := catTagCheckboxFields(meta2, nil)
	for _, kv := range kvs2 {
		if kv.Key == "diy" {
			t.Errorf("WEB 源含 DIY 词形不应投 diy（非碟门拦截）: %v", kvs2)
		}
	}

	// ③ 无信号：碟形态但无 DIY 关键词 → 不投
	meta3 := &model.TorrentMetadata{
		Subtitle:   "国语 简繁英字幕",
		SourceType: "UHD BluRay",
	}
	kvs3 := catTagCheckboxFields(meta3, nil)
	for _, kv := range kvs3 {
		if kv.Key == "diy" {
			t.Errorf("无 DIY 信号不应投 diy: %v", kvs3)
		}
	}

	// ④ 标题通道+自制词形：标题 DIY 词 + 「自制」副标均命中（UHD Blu-ray 词形）
	meta4 := &model.TorrentMetadata{
		Title:      "Movie 2160p UHD BluRay DIY Atmos-GRP",
		SourceType: "UHD Blu-ray",
	}
	kvs4 := catTagCheckboxFields(meta4, nil)
	got4 := false
	for _, kv := range kvs4 {
		if kv.Key == "diy" {
			got4 = true
		}
	}
	if !got4 {
		t.Errorf("标题 DIY+UHD Blu-ray 应勾 diy: %v", kvs4)
	}

	// ⑤ 「自制」词形：副标含「自制」+ DVD 源 → diy=yes
	meta5 := &model.TorrentMetadata{
		Subtitle:   "自制原盘收藏版",
		SourceType: "DVD",
	}
	kvs5 := catTagCheckboxFields(meta5, nil)
	got5 := false
	for _, kv := range kvs5 {
		if kv.Key == "diy" {
			got5 = true
		}
	}
	if !got5 {
		t.Errorf("副标「自制」+DVD 应勾 diy: %v", kvs5)
	}
}

func TestCatIsTeamRegionField(t *testing.T) {
	cfg := &model.PublishFormConfig{
		ValueMappings: map[string][]model.FormValueMapping{
			model.FieldDomainTeam: {
				{Label: "大陆 (Mainland,CHN)", Value: "1"},
			},
		},
	}
	if !catIsTeamRegionField(cfg) {
		t.Error("值集含「大陆」应 true")
	}
	// 普通制作组站
	cfg2 := &model.PublishFormConfig{
		ValueMappings: map[string][]model.FormValueMapping{
			model.FieldDomainTeam: {
				{Label: "FRDS", Value: "1"},
				{Label: "Other", Value: "99"},
			},
		},
	}
	if catIsTeamRegionField(cfg2) {
		t.Error("制作组站应 false")
	}
}
