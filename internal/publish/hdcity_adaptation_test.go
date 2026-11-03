package publish

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/titleparser"
)

// §59.454 附: 城市站特化函数测试锚（回归审核补——首版零锚）。

func TestHdcityTag1ingValue(t *testing.T) {
	cfg := &model.PublishFormConfig{}
	cases := []struct {
		name string
		desc string
		want string
	}{
		{"单题材", "◎类　　别　科幻\n◎年　　代　2019", "科幻/Sci-Fi"},
		{"多题材按序首中", "◎类　　别　剧情 / 喜剧 / 动作\n", "剧情/Drama"},
		{"首词不在表取行内次词", "◎类　　别　恐怖 / 悬疑 / 犯罪", "悬疑/Mystery"},
		{"奇幻→魔幻语义对齐", "◎类　　别　奇幻", "魔幻/Fantasy"},
		{"纪录→记录", "◎类　　别　纪录", "记录/Documentary"},
		{"无类别行", "◎年　　代　2019", ""},
		{"空描述", "", ""},
		{"全词不在表", "◎类　　别　同性 / 音乐", ""},
	}
	for _, c := range cases {
		if got := hdcityTag1ingValue(cfg, &model.TorrentMetadata{Description: c.desc}); got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
	// nil meta 安全
	if got := hdcityTag1ingValue(cfg, nil); got != "" {
		t.Errorf("nil meta: got %q", got)
	}
}

func TestHdcityProcessingOption(t *testing.T) {
	cfg := &model.PublishFormConfig{ValueMappings: map[string][]model.FormValueMapping{
		"processing": {
			{Label: "3D H-OU/上下半宽", Value: "1"},
			{Label: "3D H-SBS/左右半宽", Value: "2"},
			{Label: "3D Interleaved/交织", Value: "3"},
			{Label: "3D Red-blue/红蓝", Value: "4"},
			{Label: "3D Alt/其他3D", Value: "5"},
		},
	}}
	cases := []struct {
		stereo string
		want   string
	}{
		{"HOU", "1"}, {"HSBS", "2"}, {"SBS", "5"}, {"OU", "5"},
		{"", ""}, {"XXX", ""},
	}
	for _, c := range cases {
		m := hdcityProcessingOption(cfg, titleparser.TechProfile{Stereo3D: c.stereo})
		if c.want == "" {
			if m != nil {
				t.Errorf("stereo=%q: 应不投递 got %+v", c.stereo, m)
			}
			continue
		}
		if m == nil || m.Value != c.want {
			t.Errorf("stereo=%q: got %+v want value %q", c.stereo, m, c.want)
		}
	}
	// nil cfg 安全
	if m := hdcityProcessingOption(nil, titleparser.TechProfile{Stereo3D: "HOU"}); m != nil {
		t.Errorf("nil cfg: got %+v", m)
	}
}

func TestHdcityTagDependentFill(t *testing.T) {
	// meta.Tags 直读（不经 assembleTags allowed 过滤——城市无 tags 域全滤教训）
	form := map[string]string{}
	hdcityTagDependentFill(form, `["chinese_subtitle","hdr"]`)
	if form["chnize"] != "1" {
		t.Errorf("中字标签应产 chnize=1: %v", form)
	}
	// 无中字标签
	form2 := map[string]string{}
	hdcityTagDependentFill(form2, `["hdr"]`)
	if _, ok := form2["chnize"]; ok {
		t.Errorf("无中字不应写 chnize: %v", form2)
	}
	// 坏 JSON / 空安全
	hdcityTagDependentFill(map[string]string{}, `not-json`)
	hdcityTagDependentFill(map[string]string{}, "")
	hdcityTagDependentFill(nil, `["chinese_subtitle"]`)
}
