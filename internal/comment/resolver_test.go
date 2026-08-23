package comment

import "testing"

// hostResolver 测试桩: 已知域映射（真实解析器在 site 包，层选层接线）
func stubResolver(host string) string {
	table := map[string]string{
		"pt.keepfrds.com":    "朋友",
		"cspt.top":           "财神",
		"hdcmct.org":         "不可说",
		"pt.agsvpt.cn":       "末日",
		"tracker.m-team.cc":  "馒头",
		"pt.tjupt.org":       "不可羊",
		"ourbits.club":       "我堡",
	}
	if v, ok := table[host]; ok {
		return v
	}
	return ""
}

// 五方言锚定——样本全部来自 §59.60 实测（243/29 真实种子 comment）
func TestResolve_FiveDialects(t *testing.T) {
	cases := []struct {
		name, comment, trackerHost, wantSite, wantTid string
	}{
		{"标准URL-朋友", "https://pt.keepfrds.com/details.php?id=2781947", "", "朋友", "2781947"},
		{"标准URL-财神", "https://cspt.top/details.php?id=170568", "", "财神", "170568"},
		{"双域-不可说", "https://hdcmct.org/details.php?id=61082", "", "不可说", "61082"},
		{"ob_tid仅tid", "ob_tid=337906", "ourbits.club", "我堡", "337906"},
		{"URL+ob_tid复合", "https://pt.agsvpt.cn/details.php?id=99894 ,ob_tid=281595", "ourbits.club", "我堡", "281595"},
		{"HDHx家园", "HDHx277116x1769321369x3c9d2ccc", "", "家园", "277116"},
		{"馒头纯数字", "1115950", "tracker.m-team.cc", "馒头", "1115950"},
		{"相对路径-不可羊", "/details.php?id=512640", "pt.tjupt.org", "不可羊", "512640"},
	}
	for _, c := range cases {
		got := Resolve(c.comment, stubResolver, c.trackerHost)
		found := false
		for _, tgt := range got {
			if tgt.SiteName == c.wantSite && tgt.TorrentID == c.wantTid {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: want (%s,%s), got %v", c.name, c.wantSite, c.wantTid, got)
		}
	}
}

// 复合形态双命中: 主 URL（末日）+ ob_tid（我堡）都要返回
func TestResolve_CompositeBothTargets(t *testing.T) {
	got := Resolve("https://pt.agsvpt.cn/details.php?id=99894 ,ob_tid=281595", stubResolver, "ourbits.club")
	if len(got) != 2 {
		t.Fatalf("复合形态应双命中, got %v", got)
	}
	if got[0].SiteName != "末日" || got[1].SiteName != "我堡" {
		t.Errorf("顺序或站点错: %v", got)
	}
}

// 负样本（§59.60 ⑥签名型/纯文本——不得产生直达）
func TestResolve_NegativeSamples(t *testing.T) {
	neg := []string{
		"南风知我意，吹梦到西洲。",                    // 朱雀诗句
		"TorrenTGui.ORG",                        // 套套哥签名
		"Never spread this torrent in public",   // 在脚下声明
		"HDHx266295x1764420325x04f02edc",        // HDHx 合法形态（正样本, 排除串行干扰）
		"1142992",                               // 纯数字但 tracker 非馒头（不命中）
	}
	got := Resolve(neg[0], stubResolver, "")
	got2 := Resolve(neg[1], stubResolver, "")
	got3 := Resolve(neg[2], stubResolver, "")
	got5 := Resolve(neg[4], stubResolver, "tracker.qingwapt.org")
	if len(got)+len(got2)+len(got3)+len(got5) != 0 {
		t.Errorf("负样本不应命中: %v %v %v %v", got, got2, got3, got5)
	}
}

// 空与边界
func TestResolve_Empty(t *testing.T) {
	if Resolve("", stubResolver, "") != nil {
		t.Error("空 comment 应返回 nil")
	}
	if Resolve("   ", stubResolver, "") != nil {
		t.Error("空白 comment 应返回 nil")
	}
}
