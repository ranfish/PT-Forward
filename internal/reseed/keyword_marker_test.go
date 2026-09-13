package reseed

import (
	"strings"
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

// §59.215 全角冒号副题段——柯南剧场版16 案。
func TestColonSubSegment(t *testing.T) {
	cases := []struct{ in, want string }{
		{"名侦探柯南剧场版16：第11位前锋 2012", "第11位前锋 2012"},
		{"无冒号 标题 2020", ""},
		{"主题：", ""},
	}
	for _, c := range cases {
		if got := ColonSubSegment(c.in); got != c.want {
			t.Errorf("ColonSubSegment(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// §59.217 CJK 后挂括号注记剥离——雅尼案。
func TestStripCJKParenNote(t *testing.T) {
	kw := ExtractSearchKeyword("雅尼雅典卫城音乐会(25周年纪念版).1993.720p.中英字幕￡CMCT小鱼")
	if !strings.HasPrefix(kw, "雅尼雅典卫城音乐会 ") {
		t.Errorf("雅尼 keyword = %q, want 前缀 雅尼雅典卫城音乐会", kw)
	}
	// 回归：方括号标题/年份括号/根(国英) 形态
	if kw := ExtractSearchKeyword("[杀人回忆].Memories.of.Murder.2003.BluRay.720p.x264.AC3-CMCT.mkv"); !strings.Contains(kw, "Memories of Murder") {
		t.Errorf("方括号标题形态回归: %q", kw)
	}
	if kw := ExtractSearchKeyword("根(国英)S01.Roots.1977.1080p.Blu-ray.x265.DTS￡cXcY@FRDS"); kw == "" {
		t.Errorf("根(国英) 形态不应产生空关键词")
	}
}

// §59.218 注记噪声词+版式后缀循环剥净——勇敢的心/死亡诗社/角斗士三案。
func TestAnnotNoiseAndEditionPeel(t *testing.T) {
	cases := []struct{ in, want string }{
		{"勇敢的心.国英双语.1995.中英字幕￡CMCT暮雨潇潇", "勇敢的心 1995"},
		{"死亡诗社.蓝光版.1989.中英字幕￡CMCT暮雨潇潇", "死亡诗社 1989"},
	}
	for _, c := range cases {
		if got := ExtractSearchKeyword(c.in); got != c.want {
			t.Errorf("kw(%q) = %q, want %q", c.in[:12], got, c.want)
		}
	}
	if pk := PurifyChineseKeyword(ExtractSearchKeyword("角斗士十周年加长版.国英双语.2000.720p￡CMCT九洲客")); pk != "角斗士 2000" {
		t.Errorf("角斗士 purify = %q, want 角斗士 2000（两连后缀循环剥净）", pk)
	}
}

// §59.220 规格词拼写变体归一——Inception MINBD 案。
func TestSpecVariantNormalize(t *testing.T) {
	kw := ExtractSearchKeyword("Inception.2010.BluRay.x264.DTS.MINBD1080P-CMCT")
	if !strings.Contains(kw, "MiniBD1080P") {
		t.Errorf("MINBD 应归一 MiniBD: %q", kw)
	}
}
