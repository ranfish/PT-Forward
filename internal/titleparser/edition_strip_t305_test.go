package titleparser

import (
	"strings"
	"testing"
)

// §59.305 锚定主标题段尾版式词剥除——妖兽都市案（4K REMASTERED 主标题残留+
// Edition 槽 "4K Remaster" 相邻重复，幸运站拒绝）
func Test305_EditionStripFromLockedMain(t *testing.T) {
	cases := []struct {
		title, wantMain, wantEdition string
	}{
		// 妖兽都市案：Main 干净、Edition 单现
		{"Wicked.City.4K.REMASTERED.1987.JPN.BluRay.1080p.x264.FLAC-CMCT", "Wicked City", "4K Remaster"},
		// 版本词族（ReleaseVersion 通道——不在 editionPatterns）
		{"To.Live.REPACK.1994.1080p.BluRay.x264-CMCT", "To Live", "REPACK"},
		{"The.Movie.2019.PROPER.1080p.BluRay.x264-CMCT", "The Movie", "PROPER"},
		// 片名含版式短语形态（Final Cut 是片名一部分——不剥）
		{"Blade.Runner.The.Final.Cut.2007.BluRay.1080p.x264-CMCT", "Blade Runner The Final Cut", ""},
	}
	for _, c := range cases {
		tp := ParseTitleTech(c.title)
		if tp.MainTitle != c.wantMain {
			t.Errorf("Main=%q want %q（%s）", tp.MainTitle, c.wantMain, c.title[:20])
		}
		if tp.EditionInfo != c.wantEdition {
			t.Errorf("Edition=%q want %q（%s）", tp.EditionInfo, c.wantEdition, c.title[:20])
		}
		rt := ReassembleFromTechProfile(tp, V105TitleFormat())
		// 重组不含主标题残留版式词（剥除后主标题槽与版式槽语义单现）
		if c.wantMain != "" && !strings.HasPrefix(rt, c.wantMain) {
			t.Errorf("重组应以 %q 开头: %q", c.wantMain, rt)
		}
	}
	// 妖兽都市终验：无 4K REMASTERED×4K Remaster 相邻重复
	tp := ParseTitleTech("Wicked.City.4K.REMASTERED.1987.JPN.BluRay.1080p.x264.FLAC-CMCT")
	rt := ReassembleFromTechProfile(tp, V105TitleFormat())
	if strings.Contains(rt, "REMASTERED") {
		t.Errorf("主标题残留 REMASTERED: %q", rt)
	}
	if strings.Count(strings.ToLower(rt), "4k") > 1 {
		t.Errorf("4K 双现: %q", rt)
	}
}
