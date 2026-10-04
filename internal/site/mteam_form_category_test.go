package site

import (
	"encoding/json"
	"testing"
)

// TestMTeamFormCategoryAuthoritative §59.317 P2: sites.json 馒头 form.category
// 必须对齐 categoryList 权威树叶子快照（2026-10-03 实测；成人区排除——§59.222
// 定案成人不可发布）。历史教训：旧表误抄发布表单快照，馒头 2024-03 重建两级树
// 后 23 项全错位（421=电影/BluRay 被标"漫画"、434=Music(无损) 被标"微卫星"）。
func TestMTeamFormCategoryAuthoritative(t *testing.T) {
	// 权威快照：value → nameChs（categoryList 叶子，成人区 410-413/424-426/
	// 429-433/436-437/440/425 排除）
	want := map[string]string{
		"401": "电影/SD", "419": "电影/HD", "420": "电影/DVDiSo", "421": "电影/BluRay", "439": "电影/Remux",
		"402": "影剧/综艺/HD", "403": "影剧/综艺/SD", "435": "影剧/综艺/DVDiSo", "438": "影剧/综艺/BluRay",
		"404": "纪录", "405": "动画", "453": "动画/BluRay",
		"406": "演唱", "434": "Music(无损)",
		"423": "PC游戏", "448": "TV遊戲",
		"407": "运动", "409": "Misc(其他)", "422": "软件", "427": "電子書", "442": "有聲書", "451": "教育影片",
	}
	for _, s := range seedSites() {
		if s.Name != "馒头" {
			continue
		}
		if len(s.Form.Category) != len(want) {
			t.Errorf("馒头 form.category 项数 = %d, want %d", len(s.Form.Category), len(want))
		}
		for _, opt := range s.Form.Category {
			w, ok := want[opt.Value]
			if !ok {
				t.Errorf("馒头 category 出现非权威叶子: %s=%q", opt.Value, opt.Label)
				continue
			}
			if opt.Label != w {
				t.Errorf("馒头 category[%s] label = %q, want %q", opt.Value, opt.Label, w)
			}
		}
		return
	}
	t.Fatal("sites.json 未找到馒头")
}

// TestMTeamFormCategoryJSONShape: embed sites.json 可解析且馒头 form 无旧错值残留
func TestMTeamFormCategoryJSONShape(t *testing.T) {
	var data struct {
		Sites []json.RawMessage `json:"sites"`
	}
	if err := json.Unmarshal(sitesJSON, &data); err != nil {
		t.Fatal(err)
	}
	for _, raw := range data.Sites {
		var s struct {
			Name string `json:"name"`
			Form struct {
				Category []struct {
					Value string `json:"value"`
					Label string `json:"label"`
				} `json:"category"`
			} `json:"form"`
		}
		if json.Unmarshal(raw, &s) != nil || s.Name != "馒头" {
			continue
		}
		for _, opt := range s.Form.Category {
			// 旧错位残留哨兵：label 与 value 权威语义矛盾（404 权威=纪录，旧表=动漫）
			if opt.Value == "404" && opt.Label == "动漫" {
				t.Error("404 仍是旧错值（权威语义=纪录）")
			}
			if opt.Value == "421" && opt.Label == "漫画" {
				t.Error("421 仍是旧错值（权威语义=电影/BluRay）")
			}
		}
		return
	}
}
