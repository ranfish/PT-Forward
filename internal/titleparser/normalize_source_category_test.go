package titleparser

import "testing"

// TestNormalizeSourceCategoryFullFamily §59.317: dict canonical 全键族 + 站方中文词
// + 旧映射遗留值兼容（category.cartoon→animation 存量窗口期）+ 特定先于泛化排序。
func TestNormalizeSourceCategoryFullFamily(t *testing.T) {
	cases := []struct{ raw, want string }{
		// dict canonical 键直通
		{"category.movie", "category.movie"},
		{"category.tv_series", "category.tv_series"},
		{"category.tv_shows", "category.tv_shows"},
		{"category.animation", "category.animation"},
		{"category.documentary", "category.documentary"},
		{"category.music", "category.music"},
		{"category.sports", "category.sports"},
		// §59.317 新键族
		{"category.concert", "category.concert"},
		{"category.lossless_music", "category.lossless_music"},
		{"category.education", "category.education"},
		{"category.game", "category.game"},
		{"category.ebook", "category.ebook"},
		{"category.audiobook", "category.audiobook"},
		{"category.software", "category.software"},
		{"category.short_drama", "category.short_drama"},
		{"category.comic", "category.comic"},
		{"category.adult", "category.adult"},
		// 排序防碰撞：lossless 不被 music 吞；audiobook 不被 ebook 吞
		// 站方中文词
		{"演唱会", "category.concert"},
		{"有声书", "category.audiobook"},
		{"電子書", "category.ebook"},
		{"无损音乐", "category.lossless_music"},
		{"短剧", "category.short_drama"},
		{"漫画", "category.comic"},
		// 旧映射遗留值兼容（存量 P1 刷正前）
		{"category.cartoon", "category.animation"},
		{"category.documentaries", "category.documentary"},
		{"category.study", "category.education"},
		{"category.playlet", "category.short_drama"},
		// 未知/空
		{"421", ""},
		{"", ""},
		{"奇怪分类", ""},
	}
	for _, c := range cases {
		if got := NormalizeSourceCategory(c.raw); got != c.want {
			t.Errorf("NormalizeSourceCategory(%q) = %q, want %q", c.raw, got, c.want)
		}
	}
}
