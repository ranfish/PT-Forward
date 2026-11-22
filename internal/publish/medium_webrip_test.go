package publish

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/titleparser"
)

// §59.549 锚: WEB 源压制媒介源保真——站方有 WEBRip/TVRip 独立槽（劳改所
// WEBRiP(7)=[webrip]/TVRip(2)=[tvrip] 精确分立式）时优先精确键；无该槽的
// 站回退 §59.166 通用 Encode 翻译（零行为变化）。
//
// 起案：Quacks S01 AMZN WEBRip 发布劳改所——站方标题检测 WEBRiP·媒介槽
// 投了 BDRip（TYPE_MISMATCH）：翻译层 webrip→medium.encode（§59.166 通用
// 翻译·语境=站方无 WEBRip 槽）+ 劳改所 encode 键绑 BDRip(5)。
func TestMediumMappingWebRipSourceFidelity(t *testing.T) {
	e := &PublishExecutor{}
	tp := titleparser.TechProfile{Specification: "WEBRip", RawTitle: "Quacks S01 2017 1080p AMZN WEBRip HEVC"}

	// ① 劳改所式值集（有 WEBRiP 独立槽）→ 命中 webrip 键（value=7）
	// 值集 StandardKeys=全键形态（medium.xxx·243 劳改所实态——锚首跑
	// 抓出裸键误解：lookupByStdKey 是 k==stdKey 精确等值）
	ptlgs := &model.PublishFormConfig{ValueMappings: map[string][]model.FormValueMapping{
		model.FieldDomainMedium: {
			{Label: "BDRip", Value: "5", StandardKeys: []string{"medium.bdrip", "medium.encode"}},
			{Label: "WEBRiP", Value: "7", StandardKeys: []string{"medium.webrip"}},
			{Label: "TVRip", Value: "2", StandardKeys: []string{"medium.tvrip"}},
		},
	}}
	m := e.mediumMappingOf(ptlgs, "劳改所", nil, tp)
	if m == nil || m.Value != "7" {
		t.Fatalf("① 劳改所式应命中 WEBRiP(7)·got %v", m)
	}

	// ② 无 WEBRip 槽的站（普通 Encode 项）→ 回退 §59.166 encode 翻译
	plain := &model.PublishFormConfig{ValueMappings: map[string][]model.FormValueMapping{
		model.FieldDomainMedium: {
			{Label: "Encode", Value: "3", StandardKeys: []string{"medium.encode"}},
		},
	}}
	m = e.mediumMappingOf(plain, "普通站", nil, tp)
	if m == nil || m.Value != "3" {
		t.Fatalf("② 无槽站应回退 Encode(3)·got %v", m)
	}

	// ③ TVRip 同理：有槽命中·无槽回退 Encode
	tpTV := titleparser.TechProfile{Specification: "TVRip", RawTitle: "Show S01 HDTV TVRip x264"}
	m = e.mediumMappingOf(ptlgs, "劳改所", nil, tpTV)
	if m == nil || m.Value != "2" {
		t.Fatalf("③ TVRip 有槽应命中 TVRip(2)·got %v", m)
	}
	m = e.mediumMappingOf(plain, "普通站", nil, tpTV)
	if m == nil || m.Value != "3" {
		t.Fatalf("③ TVRip 无槽应回退 Encode(3)·got %v", m)
	}

	// ④ BDRip 素材（BD 源压制）在劳改所式值集仍 BDRip——源保真不误伤
	tpBD := titleparser.TechProfile{Specification: "BDRip", RawTitle: "Film 2016 1080p BDRip x264"}
	m = e.mediumMappingOf(ptlgs, "劳改所", nil, tpBD)
	if m == nil || m.Value != "5" {
		t.Fatalf("④ BDRip 素材应保持 BDRip(5)·got %v", m)
	}

	// ⑤ 血统隐式通道：spec=encode+SourceType=WEB（标题无 webrip 词但血统
	// WEB——AMZN x265 式标注）→ 源保真 WEBRiP；无槽站回退 Encode
	tpImplicit := titleparser.TechProfile{Specification: "encode", SourceType: "WEB",
		RawTitle: "Show S01 2019 1080p AMZN x264-GRP"}
	m = e.mediumMappingOf(ptlgs, "劳改所", nil, tpImplicit)
	if m == nil || m.Value != "7" {
		t.Fatalf("⑤ 血统隐式 WEB+encode 应源保真 WEBRiP(7)·got %v", m)
	}
	m = e.mediumMappingOf(plain, "普通站", nil, tpImplicit)
	if m == nil || m.Value != "3" {
		t.Fatalf("⑤ 血统隐式无槽站应回退 Encode(3)·got %v", m)
	}

	// ⑥ BD 血统 encode 不改写——「蓝光压制要求 BDRip」适配期定案保持
	tpBDBlood := titleparser.TechProfile{Specification: "", SourceType: "Blu-ray",
		RawTitle: "Movie 2020 1080p Blu-ray x265", VideoCodec: "x265"}
	m = e.mediumMappingOf(ptlgs, "劳改所", nil, tpBDBlood)
	if m == nil || m.Value != "5" {
		t.Fatalf("⑥ BD 血统 encode 应保持 BDRip(5)〔适配期定案〕·got %v", m)
	}
}
