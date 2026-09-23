package publish

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.268: codec 域 miss 折叠——修道院真实映射形态（仅标准名选项）
func xdyCodecCfg() *model.PublishFormConfig {
	return &model.PublishFormConfig{
		ValueMappings: map[string][]model.FormValueMapping{
			model.FieldDomainCodec: {
				{Label: "H.264", Value: "1", StandardKeys: []string{"video.h264"}},
				{Label: "VC-1", Value: "2", StandardKeys: []string{"video.vc1"}},
				{Label: "Xvid", Value: "3", StandardKeys: []string{"video.xvid"}},
				{Label: "MPEG-2", Value: "4", StandardKeys: []string{"video.mpeg2"}},
				{Label: "Other", Value: "5", StandardKeys: []string{"video.other"}},
				{Label: "H.265", Value: "6", StandardKeys: []string{"video.h265"}},
				{Label: "VP8/9", Value: "7", StandardKeys: []string{"video.vp9"}},
			},
		},
	}
}

func TestCodecMappingOf_Fallback(t *testing.T) {
	e := &PublishExecutor{}
	cfg := xdyCodecCfg()
	cases := []struct{ codec, wantLabel string }{
		{"x264", "H.264"},   // 编码器实现归标准（×216 案）
		{"x265", "H.265"},   // ×6 案
		{"AV1", "Other"},    // 新编码无选项 → Other（×2 案）
		{"H264", "H.264"},   // 原命中不回归
		{"HEVC", "H.265"},   // 原命中不回归
		{"VC-1", "VC-1"},    // 原命中不回归
	}
	for _, tc := range cases {
		m := e.codecMappingOf(cfg, tc.codec)
		if m == nil {
			t.Errorf("[%s] 映射 nil，want %s", tc.codec, tc.wantLabel)
			continue
		}
		if m.Label != tc.wantLabel {
			t.Errorf("[%s] got %s want %s", tc.codec, m.Label, tc.wantLabel)
		}
	}
	// 站方无 Other/H.264 选项时不强填（折叠链全 miss 维持 nil）
	cfgLean := xdyCodecCfg()
	var lean []model.FormValueMapping
	for _, m := range cfgLean.ValueMappings[model.FieldDomainCodec] {
		if m.Label == "Other" || m.Label == "H.264" {
			continue
		}
		lean = append(lean, m)
	}
	cfgLean.ValueMappings[model.FieldDomainCodec] = lean
	if m := e.codecMappingOf(cfgLean, "AV1"); m != nil {
		t.Errorf("[AV1 无 Other 选项] 应维持 nil，got %s", m.Label)
	}
	if m := e.codecMappingOf(cfgLean, "x264"); m != nil {
		t.Errorf("[x264 无 H.264 选项] 应维持 nil，got %s", m.Label)
	}
}
