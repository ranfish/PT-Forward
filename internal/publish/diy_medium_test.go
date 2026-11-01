package publish

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/titleparser"
)

// §59.407: DIY 原盘媒介联动锚（六站受益+三重零触发面+Remux 不触发）。
func TestDiyMediumOption(t *testing.T) {
	// 海的宝形态（原盘×DIY 四分）
	cfg := &model.PublishFormConfig{
		ValueMappings: map[string][]model.FormValueMapping{
			model.FieldDomainMedium: {
				{Label: "UHD Blu-ray 原盘", Value: "1", StandardKeys: []string{"medium.uhd_bluray"}},
				{Label: "UHD Blu-ray DIY", Value: "2"},
				{Label: "Blu-ray 原盘", Value: "3", StandardKeys: []string{"medium.bluray"}},
				{Label: "Blu-ray DIY", Value: "4"},
				{Label: "REMUX", Value: "5", StandardKeys: []string{"medium.remux"}},
			},
		},
	}
	// ① UHD 原盘+DIY 信号 → UHD DIY(2)
	got := diyMediumOption(cfg, titleparser.TechProfile{RawTitle: "Movie 2020 2160p DIY Blu-ray AVC"}, "medium.uhd_bluray")
	if got == nil || got.Value != "2" {
		t.Errorf("UHD+DIY: got %v, want value 2", got)
	}
	// ② BD 原盘+DIY 信号 → BD DIY(4)
	got = diyMediumOption(cfg, titleparser.TechProfile{RawTitle: "Movie 2010 1080p DIY Blu-ray AVC"}, "medium.bluray")
	if got == nil || got.Value != "4" {
		t.Errorf("BD+DIY: got %v, want value 4", got)
	}
	// ③ 标题小写 diy 也命中（大小写不敏感）
	got = diyMediumOption(cfg, titleparser.TechProfile{RawTitle: "Movie diy 2020 Blu-ray"}, "medium.uhd_bluray")
	if got == nil || got.Value != "2" {
		t.Errorf("小写 diy 应命中: got %v", got)
	}
	// ④ 无 DIY 信号 → nil（原盘照旧）
	if v := diyMediumOption(cfg, titleparser.TechProfile{RawTitle: "Movie 2020 2160p Blu-ray"}, "medium.uhd_bluray"); v != nil {
		t.Errorf("无 DIY 信号应 nil, got %v", v)
	}
	// ⑤ Remux+DIY 信号 → nil（用户定案：DIY Remux 不触发——六站均无 Remux DIY 项）
	if v := diyMediumOption(cfg, titleparser.TechProfile{RawTitle: "Movie 2020 DIY Remux"}, "medium.remux"); v != nil {
		t.Errorf("DIY Remux 应不触发 nil, got %v", v)
	}
	// ⑥ Encode/WEB-DL 等非原盘族 → nil
	if v := diyMediumOption(cfg, titleparser.TechProfile{RawTitle: "Movie 2020 DIY Encode"}, "medium.encode"); v != nil {
		t.Errorf("Encode 应 nil, got %v", v)
	}
	// ⑦ 无 DIY 项的站（普通站）→ nil 零变化
	cfgNoDiy := &model.PublishFormConfig{
		ValueMappings: map[string][]model.FormValueMapping{
			model.FieldDomainMedium: {
				{Label: "Blu-ray", Value: "1", StandardKeys: []string{"medium.bluray"}},
				{Label: "Encode", Value: "7", StandardKeys: []string{"medium.encode"}},
			},
		},
	}
	if v := diyMediumOption(cfgNoDiy, titleparser.TechProfile{RawTitle: "Movie 2020 DIY Blu-ray"}, "medium.bluray"); v != nil {
		t.Errorf("无 DIY 项站应 nil, got %v", v)
	}
	// ⑧ 冬樱/壹吧形态（label 含 "/"：Blu-ray/DIY、4K UHD DIY 分族）
	cfgSlash := &model.PublishFormConfig{
		ValueMappings: map[string][]model.FormValueMapping{
			model.FieldDomainMedium: {
				{Label: "UHD Blu-ray", Value: "10", StandardKeys: []string{"medium.uhd_bluray"}},
				{Label: "UHD Blu-ray/DIY", Value: "11"},
				{Label: "Blu-ray", Value: "12", StandardKeys: []string{"medium.bluray"}},
				{Label: "Blu-ray/DIY", Value: "13"},
				{Label: "4K UHD DIY", Value: "14"}, // 五二 4K 词形 → UHD 族
			},
		},
	}
	got = diyMediumOption(cfgSlash, titleparser.TechProfile{RawTitle: "Movie DIY UHD"}, "medium.uhd_bluray")
	if got == nil || got.Value != "11" {
		t.Errorf("斜杠形态 UHD/DIY: got %v, want 11", got)
	}
	got = diyMediumOption(cfgSlash, titleparser.TechProfile{RawTitle: "Movie DIY BD"}, "medium.bluray")
	if got == nil || got.Value != "13" {
		t.Errorf("斜杠形态 BD/DIY: got %v, want 13", got)
	}
}
