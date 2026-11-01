package publish

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/titleparser"
)

// §59.405: 好大 type 槽媒介细分路由锚（用户五定案矩阵）。
// 输入形态：tp.SourceType（归一媒介词——mediumStdKeyOf 单源）+Resolution+Stereo3D。
func TestHdareaCategoryValue(t *testing.T) {
	cfg := &model.PublishFormConfig{
		ValueMappings: map[string][]model.FormValueMapping{
			model.FieldDomainType: {
				{Label: "Movie UHD-4K", Value: "300"}, {Label: "Movies Blu-ray", Value: "401"},
				{Label: "Movies REMUX", Value: "415"}, {Label: "Movies 3D", Value: "416"},
				{Label: "Movies 1080p", Value: "410"}, {Label: "Movies 720p", Value: "411"},
				{Label: "Movies DVD", Value: "414"}, {Label: "Movies WEB-DL", Value: "412"},
				{Label: "Movies HDTV", Value: "413"}, {Label: "Movies iPad", Value: "417"},
				{Label: "Documentaries", Value: "404", StandardKeys: []string{"category.documentary"}},
			},
		},
	}
	movie := &model.TorrentMetadata{Category: "category.movie"}

	cases := []struct {
		name string
		tp   titleparser.TechProfile
		want string
	}{
		// ① 定案1：3D 最优先（覆盖媒介判定）
		{"3D 优先于 UHD 原盘", titleparser.TechProfile{SourceType: "UHD Blu-ray", Stereo3D: "HSBS", Resolution: "1080p"}, "416"},
		{"3D 优先于 WEB-DL", titleparser.TechProfile{SourceType: "WEB-DL", Stereo3D: "SBS", Resolution: "1080p"}, "416"},
		// ②-⑦ 媒介路由
		{"UHD 原盘", titleparser.TechProfile{SourceType: "UHD Blu-ray", Resolution: "2160p"}, "300"},
		{"BD 原盘", titleparser.TechProfile{SourceType: "Blu-ray", Resolution: "1080p"}, "401"},
		{"REMUX", titleparser.TechProfile{SourceType: "Remux", Resolution: "1080p"}, "415"},
		{"UHD REMUX 归并", titleparser.TechProfile{SourceType: "UHD Remux", Resolution: "2160p"}, "415"},
		{"DVD 媒介", titleparser.TechProfile{SourceType: "DVDR"}, "414"},
		{"WEB-DL 4K 不看分辨率", titleparser.TechProfile{SourceType: "WEB-DL", Resolution: "2160p"}, "412"},
		{"WEB-DL 1080p", titleparser.TechProfile{SourceType: "WEB-DL", Resolution: "1080p"}, "412"},
		{"HDTV", titleparser.TechProfile{SourceType: "HDTV"}, "413"},
		// ⑧ 兜底按分辨率（定案2/3：Encode+4K→300/SD→414/MiniBD/iPad 走分辨率）
		{"Encode 4K→300", titleparser.TechProfile{SourceType: "Encode", Resolution: "2160p"}, "300"},
		{"Encode 1080p", titleparser.TechProfile{SourceType: "Encode", Resolution: "1080p"}, "410"},
		{"Encode 1080i 归 410", titleparser.TechProfile{SourceType: "Encode", Resolution: "1080i"}, "410"},
		{"Encode 720p", titleparser.TechProfile{SourceType: "Encode", Resolution: "720p"}, "411"},
		{"Encode SD→414（DVD 分辨率族）", titleparser.TechProfile{SourceType: "Encode", Resolution: "480p"}, "414"},
		{"MiniBD 按分辨率兜底", titleparser.TechProfile{SourceType: "MiniBD", Resolution: "720p"}, "411"},
		{"iPad 形态按分辨率兜底", titleparser.TechProfile{SourceType: "", Resolution: "1080p"}, "410"},
	}
	for _, c := range cases {
		got := hdareaCategoryValue(cfg, "好大", movie, c.tp)
		if got == nil || got.Value != c.want {
			t.Errorf("%s: got %v, want %s", c.name, got, c.want)
		}
	}
	// 非电影类回退（B 组直配——定案5：动画 WEB-DL 归 405 不走 412）
	if v := hdareaCategoryValue(cfg, "好大", &model.TorrentMetadata{Category: "category.animation"}, titleparser.TechProfile{SourceType: "WEB-DL"}); v != nil {
		t.Errorf("非电影类应回退通用链 nil, got %v", v)
	}
	// 非好大站零变化
	if v := hdareaCategoryValue(cfg, "幸运", movie, titleparser.TechProfile{SourceType: "WEB-DL"}); v != nil {
		t.Errorf("非好大站应 nil, got %v", v)
	}
}
