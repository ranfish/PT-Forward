package publish

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/titleparser"
)

// §59.541: 猪猪特化注入锚——分区组合判据 + 体裁数组生成。
// 实测锚样本（piggo.md §59.541 摸站 8 颗）：千与千寻/你的名字（日漫剧场
// →7）·蜘蛛侠平行宇宙（西方青少年向→7）·玩具总动员/小黄人（合家欢→5）。
func TestPiggoSectionOverride(t *testing.T) {
	cases := []struct {
		name, ptgenJSON, want string
	}{
		{
			"千与千寻-日本产地优先于Family", // genre 含家庭但产地日本→7（判据顺序：日本>Family）
			`{"genre":["动画","家庭","奇幻"],"region":["日本"]}`,
			"7",
		},
		{
			"玩具总动员-美产Family→少儿",
			`{"genre":["动画","家庭","喜剧"],"region":["美国"]}`,
			"5",
		},
		{
			"蜘蛛侠平行宇宙-美产无Family→动漫默认",
			`{"genre":["动画","动作","冒险"],"region":["美国"]}`,
			"7",
		},
		{
			"你的名字-日本",
			`{"genre":["动画","爱情"],"region":["日本"]}`,
			"7",
		},
		{
			"合拍首值-中国日本合拍首值中国大陆≠日本→默认7",
			`{"genre":["动画"],"region":["中国大陆 / 日本"]}`,
			"7",
		},
		{
			"PTGen缺失-默认动漫",
			``,
			"7",
		},
	}
	for _, c := range cases {
		meta := &model.TorrentMetadata{
			Category:         "category.animation",
			PTGenSourceJSON: c.ptgenJSON,
		}
		form := map[string]string{"section_id": "7"}
		piggoPublishInject(form, nil, meta, titleparser.TechProfile{})
		if form["section_id"] != c.want {
			t.Errorf("%s: want section %s, got %s", c.name, c.want, form["section_id"])
		}
	}
}

// 非动画类型不覆写（电影 1 保持——type 值映射结果不动）。
func TestPiggoSectionOverride_NonAnimation(t *testing.T) {
	meta := &model.TorrentMetadata{
		Category:         "category.movie",
		PTGenSourceJSON: `{"genre":["家庭"],"region":["美国"]}`,
	}
	form := map[string]string{"section_id": "1"}
	piggoPublishInject(form, nil, meta, titleparser.TechProfile{})
	if form["section_id"] != "1" {
		t.Errorf("movie 不应覆写: got %s", form["section_id"])
	}
}

// §59.541: 体裁数组——分区感知映射+多体裁惯例+少儿动画形态特判+兜底。
func TestPiggoCategoryIds(t *testing.T) {
	// Ronin 实测形态：电影区 动作+惊悚 → [3,7]
	if got := piggoCategoryIds(piggoSecMovie, []string{"动作", "惊悚"}, ""); len(got) != 2 || got[0] != 3 || got[1] != 7 {
		t.Errorf("Ronin 形态: got %v", got)
	}
	// Rocky III 形态：零命中 → [11]
	if got := piggoCategoryIds(piggoSecMovie, []string{"恐怖", "犯罪"}, ""); len(got) != 1 || got[0] != 11 {
		t.Errorf("零命中兜底: got %v", got)
	}
	// 奇幻科幻同 id 去重：[5]
	if got := piggoCategoryIds(piggoSecMovie, []string{"奇幻", "科幻"}, ""); len(got) != 1 || got[0] != 5 {
		t.Errorf("同 id 去重: got %v", got)
	}
	// 少儿区动画电影（无 S/E）→19 前置；剧集（S01）→6
	if got := piggoCategoryIds(piggoSecKids, []string{}, ""); got[0] != 19 {
		t.Errorf("动画电影: got %v", got)
	}
	if got := piggoCategoryIds(piggoSecKids, []string{"冒险"}, "S01"); got[0] != 6 {
		t.Errorf("动画剧集: got %v", got)
	}
	// 动漫区日漫词：热血+悬疑 → [35,8]
	if got := piggoCategoryIds(piggoSecAnime, []string{"热血", "悬疑"}, "S01"); len(got) != 2 || got[0] != 35 || got[1] != 8 {
		t.Errorf("日漫词: got %v", got)
	}
	// 未知分区（section_id 映射 miss）→ 兜 [11]
	if got := piggoCategoryIds(0, []string{"动作"}, ""); len(got) != 1 || got[0] != 11 {
		t.Errorf("未知分区: got %v", got)
	}
}

// §59.541: category_ids JSON 串形态注入 form（PiggoAdapter 消费形态）。
func TestPiggoPublishInject_CategoryJSON(t *testing.T) {
	meta := &model.TorrentMetadata{
		Category:         "category.movie",
		PTGenSourceJSON: `{"genre":["动作","惊悚"],"region":["美国"]}`,
	}
	form := map[string]string{"section_id": "1"}
	piggoPublishInject(form, nil, meta, titleparser.TechProfile{})
	if form["category_ids"] != "[3,7]" {
		t.Errorf("category_ids JSON: got %s", form["category_ids"])
	}
}

// §59.541 回归审核补锚: source/processing 域特化组装（jobs 通用段不消费
// 这两域——海豹同款特化先例·首版锚 cfg=nil 跳过未覆盖）。
func TestPiggoSourceProcessingInject(t *testing.T) {
	cfg := &model.PublishFormConfig{
		FormFields: map[string]string{
			"source": "source_id", "processing": "processing_id",
		},
		ValueMappings: map[string][]model.FormValueMapping{
			"source": {
				{Label: "WEB-DL", Value: "34", StandardKeys: []string{"medium.webdl", "medium.webrip"}},
			},
			"processing": {
				{Label: "SDR", Value: "6", StandardKeys: []string{"hdr.sdr"}},
				{Label: "DV", Value: "9", StandardKeys: []string{"hdr.dv"}},
			},
		},
	}
	meta := &model.TorrentMetadata{Category: "category.movie"}
	tp := titleparser.TechProfile{Specification: "WEB-DL", HDR: "DoVi"}
	form := map[string]string{"section_id": "1"}
	piggoPublishInject(form, cfg, meta, tp)
	if form["source_id"] != "34" {
		t.Errorf("source 特化: got %s", form["source_id"])
	}
	if form["processing_id"] != "9" {
		t.Errorf("processing 特化 (DoVi→hdr.dv→9): got %s", form["processing_id"])
	}
	// 素材 miss：两域留空（adapter 空维兜 Other/SDR——分层兜底不在本函数）
	tp2 := titleparser.TechProfile{}
	form2 := map[string]string{"section_id": "1"}
	piggoPublishInject(form2, cfg, meta, tp2)
	if form2["source_id"] != "" || form2["processing_id"] != "" {
		t.Errorf("miss 应留空交 adapter 兜底: %v", form2)
	}
}
