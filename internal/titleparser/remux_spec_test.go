package titleparser

import "testing"

func TestVerifyRemux(t *testing.T) {
	cases := []struct{ title, wantSrc, wantSpec, wantRelVer string }{
		{"Movie 2160p UHD BluRay Remux DTS GRP", "UHD Blu-ray", "Remux", ""},
		{"Movie 1080p BluRay Remux GRP", "Blu-ray", "Remux", ""},
		{"Movie 1080p WEB-DL x264 GRP", "", "WEB-DL", ""},
		{"Movie 1080p BluRay x264 PROPER GRP", "Blu-ray", "", "PROPER"},
		{"Movie 1080p BluRay x264 REPACK GRP", "Blu-ray", "", "REPACK"},
		{"Movie 2160p UHD BluRay x265 GRP", "UHD Blu-ray", "", ""},
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
