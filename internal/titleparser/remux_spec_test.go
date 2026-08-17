package titleparser

import "testing"

// §59.34: 片源写法忠实原文（v1.05 qingwapt.md:114-118）
//   - 原盘类写法带连字符：Blu-ray / UHD Blu-ray / 3D Blu-ray
//   - 压制类写法无连字符：BluRay / UHD BluRay / 3D BluRay
//   - Encode 规格为空，由片源写法/编码族区分（IsEncode）
func TestVerifyRemux(t *testing.T) {
	cases := []struct{ title, wantSrc, wantSpec, wantRelVer string }{
		{"Movie 2160p UHD BluRay Remux DTS GRP", "UHD BluRay", "Remux", ""},
		{"Movie 1080p BluRay Remux GRP", "BluRay", "Remux", ""},
		{"Movie 1080p WEB-DL x264 GRP", "", "WEB-DL", ""},
		{"Movie 1080p BluRay x264 PROPER GRP", "BluRay", "", "PROPER"},
		{"Movie 1080p BluRay x264 REPACK GRP", "BluRay", "", "REPACK"},
		{"Movie 2160p UHD BluRay x265 GRP", "UHD BluRay", "", ""},
		// 原盘写法（带连字符/点分隔）
		{"Movie 2160p UHD Blu-ray AVC GRP", "UHD Blu-ray", "", ""},
		{"Movie 1080p Blu-ray AVC GRP", "Blu-ray", "", ""},
		{"Movie 2160p UHD.Blu-ray AVC GRP", "UHD Blu-ray", "", ""},
		// 用户案例：Encode 资源（压制写法 + 压制编码族）
		{"Stand by Me 1986 UHD BluRay 2160p x265 DV HDR TrueHD7.1 Atmos mUHD-FRDS", "UHD BluRay", "", ""},
	}
	for _, c := range cases {
		p := ParseTitleTech(c.title)
		if p.SourceType != c.wantSrc || p.Specification != c.wantSpec {
			t.Errorf("%q: src=%q spec=%q, want (%q,%q)", c.title, p.SourceType, p.Specification, c.wantSrc, c.wantSpec)
		}
		if p.ReleaseVersion != c.wantRelVer {
			t.Errorf("%q: relVer=%q want %q", c.title, p.ReleaseVersion, c.wantRelVer)
		}
	}
}

// §59.34: Encode 判定（v1.05：Encode 规格为空，由片源写法/编码族区分）
func TestIsEncode(t *testing.T) {
	cases := []struct {
		title string
		want  bool
	}{
		// Encode：压制写法片源 或 压制编码族
		{"Stand by Me 1986 UHD BluRay 2160p x265 DV HDR TrueHD7.1 Atmos mUHD-FRDS", true},
		{"Movie 1080p BluRay x264 DD5.1 GRP", true},
		{"Movie 2160p UHD BluRay x265 GRP", true},
		// 原盘：片源带连字符
		{"Movie 2160p UHD Blu-ray HEVC TrueHD GRP", false},
		{"Movie 1080p Blu-ray AVC DTS GRP", false},
		// Remux：规格排除
		{"Movie 2160p UHD BluRay Remux HEVC GRP", false},
		// WEB/HDTV：规格排除
		{"Movie 1080p WEB-DL x264 DDP5.1 GRP", false},
		{"Movie 1080p HDTV x264 GRP", false},
		// 原盘写法 + 压制编码族矛盾：以片源写法为准
		{"Movie 1080p Blu-ray x264 GRP", false},
	}
	for _, c := range cases {
		p := ParseTitleTech(c.title)
		if got := IsEncode(p); got != c.want {
			t.Errorf("%q: IsEncode=%v want %v (src=%q spec=%q codec=%q)", c.title, got, c.want, p.SourceType, p.Specification, p.VideoCodec)
		}
	}
}
