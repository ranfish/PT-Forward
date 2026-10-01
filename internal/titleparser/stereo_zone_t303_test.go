package titleparser

import (
	"strings"
	"testing"
)

// §59.303 3D 封装技术段约束——主标题区拼音词不得误提取
func Test303_Stereo3D_MainTitleZone(t *testing.T) {
	cases := []struct{ title string; want string }{
		// 丁丁战猴王案："Zhan Hou Wang" 的 Hou 在年份前=主标题区 → 不提取
		{"Ding Ding Zhan Hou Wang 1980 BluRay 1080p x265 10bit FLAC MNHD-FRDS", ""},
		// 技术段内的 HOU/HSBS 正常提取
		{"Movie.2011.3D.BluRay.1080p.HSBS.x264-CMCT", "HSBS"},
		{"Movie.2011.3D.BluRay.1080p.H-OU.x264-CMCT", "HOU"},
		// 无技术 token → 不提取
		{"Just A Name Hou", ""},
	}
	for _, c := range cases {
		if got := extractStereo3D(c.title); got != c.want {
			t.Errorf("extractStereo3D(%q)=%q want %q", c.title, got, c.want)
		}
	}
}

// §59.303 组名源限定全段——重组忠实 + Tab1 纯组名
func Test303_GroupQualifierFull(t *testing.T) {
	cases := []struct{ title, wantGroup, wantFull string }{
		{"Ding.Ding.Zhan.Hou.Wang.1980.BluRay.1080p.x265.FLAC.MNHD-FRDS", "FRDS", "MNHD-FRDS"},
		{"Some.Movie.2020.2160p.BluRay.x265.mUHD-FRDS", "FRDS", "mUHD-FRDS"},
		{"A.Movie.2024.1080p.WEB-DL-cXcY@FRDS", "cXcY@FRDS", "cXcY@FRDS"},
		{"Plain.2019.1080p.BluRay.x264-CMCT", "CMCT", "CMCT"},
	}
	for _, c := range cases {
		tp := BuildTechProfile(c.title, "", "", "", "", "")
		if tp.ReleaseGroup != c.wantGroup {
			t.Errorf("Group=%q want %q（Tab1 须纯组名）", tp.ReleaseGroup, c.wantGroup)
		}
		if tp.ReleaseGroupFull != c.wantFull {
			t.Errorf("Full=%q want %q（标题须忠实全段）", tp.ReleaseGroupFull, c.wantFull)
		}
		rt := ReassembleFromTechProfile(tp, V105TitleFormat())
		if !strings.HasSuffix(rt, c.wantFull) {
			t.Errorf("重组标题组段: %q 应以 %q 结尾", rt, c.wantFull)
		}
	}
}
