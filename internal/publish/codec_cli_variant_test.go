package publish

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/titleparser"
)

// TestCodecCLIVariant §59.380: 压制形态联动（百川 §5.2 站方语义）。
// encode 媒介+站点有 X26x 选项 → CLI 形态；原盘/无选项站 → 标准形态不变。
func TestCodecCLIVariant(t *testing.T) {
	// 百川形态：codec 槽含 X264(13)/X265(14)
	baichuan := &model.PublishFormConfig{
		ValueMappings: map[string][]model.FormValueMapping{
			model.FieldDomainCodec: {
				{Label: "H.264", Value: "1", StandardKeys: []string{"video.h264"}},
				{Label: "H.265", Value: "10", StandardKeys: []string{"video.h265"}},
				{Label: "X264", Value: "13"},
				{Label: "X265", Value: "14"},
			},
		},
	}
	// 龙之家形态：无 CLI 选项
	noCLI := &model.PublishFormConfig{
		ValueMappings: map[string][]model.FormValueMapping{
			model.FieldDomainCodec: {
				{Label: "H.264(AVC)", Value: "1", StandardKeys: []string{"video.h264"}},
				{Label: "H.265(HEVC)", Value: "2", StandardKeys: []string{"video.h265"}},
			},
		},
	}
	e := &PublishExecutor{}

	// 压制种（MI Writing library 铁证）+ 百川 → X265(14)
	encTP := titleparser.TechProfile{
		VideoCodec: "x265",
		RawTitle:   "Movie 2020 1080p BluRay x265",
	}
	// titleparser.IsEncode(tp) 依赖 MI/碟源判定——构造可靠 encode 信号：
	encTP.MIEncoded = true
	if got := e.codecMappingOf(baichuan, encTP); got == nil || got.Value != "14" {
		t.Errorf("压制+百川: 期望 X265(14)，got %v", got)
	}
	// 原盘种 + 百川 → H.265(10) 不联动
	origTP := titleparser.TechProfile{VideoCodec: "HEVC", RawTitle: "Movie 2020 2160p UHD Blu-ray HEVC"}
	if got := e.codecMappingOf(baichuan, origTP); got == nil || got.Value != "10" {
		t.Errorf("原盘+百川: 期望 H.265(10) 不变，got %v", got)
	}
	// 压制种 + 无 CLI 选项站 → H.265 标准形态（行为零变化）
	if got := e.codecMappingOf(noCLI, encTP); got == nil || got.Value != "2" {
		t.Errorf("压制+无CLI站: 期望标准形态不变，got %v", got)
	}
}
