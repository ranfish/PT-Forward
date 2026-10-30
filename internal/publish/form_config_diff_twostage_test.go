package publish

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

// TestMergeDraftTwoStageAlign §59.378: 两级对齐（label→value）+分叉族校验。
// 五二改版实录形态回归：label 文案微调（value 同）→ renamed+keys 继承
// 不再误报 removed+added 对；真增删才 removed/added；松查 extended 侧
// 单键自动补 dict 侧双键。
func TestMergeDraftTwoStageAlign(t *testing.T) {
	current := &model.PublishFormConfig{
		FormFields:   map[string]string{"type": "type"},
		ValueMappings: map[string][]model.FormValueMapping{
			"type": {
				{Label: "Animations/动漫", Value: "405", StandardKeys: []string{"category.animation"}},
				{Label: "Music Videos/音乐MV", Value: "406", StandardKeys: []string{"category.mv"}},
				{Label: "Opera/戏曲", Value: "410"}, // label-only 特化（无键）
				{Label: "PTHOME", Value: "15"},      // 将被站方删除
			},
		},
	}
	draft := &model.PublishFormConfig{
		FormFields:   map[string]string{"type": "type"},
		ValueMappings: map[string][]model.FormValueMapping{
			"type": {
				// label 微调 value 同 → renamed + keys 继承
				{Label: "Animations/动漫(画)", Value: "405", StandardKeys: []string{"category.animation"}},
				// label 微调 value 同 + 松查产出 extended 侧单键 → renamed 继承原键（不走松查）
				{Label: "Music Videos/音乐MV(演唱会)", Value: "406", StandardKeys: []string{"category.music_video"}},
				// label 微调 value 同 + label-only → 继承无键（特化保护）
				{Label: "Opera/戏曲 曲艺", Value: "410"},
				// 真新增 → added + 分叉族校验（music_video 单键补 mv 双键）
				{Label: "Concert/演唱会", Value: "412", StandardKeys: []string{"category.music_video"}},
				// PTHOME 不在（站方删除）→ removed
			},
		},
	}
	merged, diffs := MergeDraftWithCurrent(current, draft)

	byKind := map[string][]FormConfigDiffItem{}
	for _, d := range diffs {
		byKind[d.Kind] = append(byKind[d.Kind], d)
	}
	if len(byKind["renamed"]) != 3 {
		t.Errorf("renamed 应 3 条（文案微调），got %d: %+v", len(byKind["renamed"]), byKind["renamed"])
	}
	if len(byKind["removed"]) != 1 || byKind["removed"][0].Label != "PTHOME" {
		t.Errorf("removed 应仅 PTHOME（两级收紧），got %+v", byKind["removed"])
	}
	if len(byKind["added"]) != 1 || byKind["added"][0].Label != "Concert/演唱会" {
		t.Errorf("added 应仅新增项，got %+v", byKind["added"])
	}

	// merged keys 继承断言
	for _, m := range merged.ValueMappings["type"] {
		switch m.Value {
		case "405":
			if len(m.StandardKeys) != 1 || m.StandardKeys[0] != "category.animation" {
				t.Errorf("405 keys 应继承 animation: %v", m.StandardKeys)
			}
		case "406":
			if len(m.StandardKeys) != 1 || m.StandardKeys[0] != "category.mv" {
				t.Errorf("406 keys 应继承 mv（不被松查 music_video 侵蚀）: %v", m.StandardKeys)
			}
		case "410":
			if len(m.StandardKeys) != 0 {
				t.Errorf("410 应保持 label-only（特化保护）: %v", m.StandardKeys)
			}
		case "412":
			if len(m.StandardKeys) != 2 || m.StandardKeys[0] != "category.mv" || m.StandardKeys[1] != "category.music_video" {
				t.Errorf("412 added 应分叉双键 [mv, music_video]: %v", m.StandardKeys)
			}
		}
	}
}

// TestPairDivergentKeys §59.378: 分叉族校验单测。
func TestPairDivergentKeys(t *testing.T) {
	if got := pairDivergentKeys([]string{"category.music_video"}); len(got) != 2 || got[0] != "category.mv" {
		t.Errorf("extended 单键应补 dict 双键: %v", got)
	}
	if got := pairDivergentKeys([]string{"category.mv"}); len(got) != 1 {
		t.Errorf("dict 键单向不扩: %v", got)
	}
	if got := pairDivergentKeys([]string{"category.movie"}); len(got) != 1 {
		t.Errorf("非分叉族不动: %v", got)
	}
}
