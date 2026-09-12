package reseed

import (
	"testing"
)

// §59.204 关键词结构标记三修——BBC王朝/色即是空挂账兑现。
func TestKeywordStructuralMarkers(t *testing.T) {
	cases := []struct{ in, want string }{
		// F1: 中文数字集数剥除（BBC王朝案：全五集残留 AND 0 结果）
		{"BBC王朝.全五集.2018.720p.中英字幕￡CMCT小鱼", "BBC王朝 2018 720p"},
		// F2+F3: 年份区间剥除 + 罗马编号/合集结构词不构成标题 → 中文片名回退
		{"色即是空I.II合集.2002-2007.韩语中字￡CMCT如烟", "色即是空I II合集"},
		// F1 回归：阿拉伯数字集数（原行为保持）
		{"萧十一郎.全40集.2002.国语无字￡CMCT风潇潇", "萧十一郎 2002"},
	}
	for _, c := range cases {
		if got := ExtractSearchKeyword(c.in); got != c.want {
			t.Errorf("ExtractSearchKeyword(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	// F3 单元：结构词判定（含粘着形态）
	for _, w := range []string{"合集", "全集", "II合集", "2合集"} {
		if !isCollectionMarkerToken(w) {
			t.Errorf("isCollectionMarkerToken(%q) = false, want true", w)
		}
	}
	for _, w := range []string{"集结号", "合集杀人事件"} {
		if isCollectionMarkerToken(w) {
			t.Errorf("isCollectionMarkerToken(%q) = true, want false", w)
		}
	}
	// F3 单元：罗马数字
	for _, w := range []string{"I", "ii", "VII"} {
		if !isRomanNumeralToken(w) {
			t.Errorf("isRomanNumeralToken(%q) = false, want true", w)
		}
	}
}
