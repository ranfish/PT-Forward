package site

import "testing"

// §59.469: 音乐源站单点锚——三站在场+判定+影视站排除（天空案锚）。
func TestMusicSourceSites(t *testing.T) {
	list := MusicSourceSites()
	if len(list) != 3 {
		t.Fatalf("应有 3 站（皇后/海豚/乌托邦）: %v", list)
	}
	for _, s := range []string{"皇后", "海豚", "乌托邦"} {
		if !IsMusicSourceSite(s) {
			t.Errorf("%s 应为音乐源站", s)
		}
	}
	// 影视站结构性排除（天空案锚——非音乐源站不可进音乐链）
	for _, s := range []string{"天空", "织梦", "红豆饭", ""} {
		if IsMusicSourceSite(s) {
			t.Errorf("%q 不应为音乐源站", s)
		}
	}
}
