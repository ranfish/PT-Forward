package titleparser

import (
	"testing"
)

// §59.196: 显式像素形态优先于裸 4K/8K（"4K.REMASTER...1080p" 侠女案）。
func TestResolutionPixelFormPriority(t *testing.T) {
	cases := []struct{ in, want string }{
		{"A.Touch.of.Zen.1970.4K.REMASTER.HKG.BluRay.1080p.x264.FLAC-CMCT", "1080p"},
		{"Movie.2020.4K.HDR.2160p.BluRay-x264", "2160p"},
		{"Movie.2020.2160p.BluRay-x264", "2160p"},
		{"Movie.2020.4K.BluRay-x264", "4K"},
		{"Movie.2020.1080p.BluRay-x264", "1080p"},
		{"Movie.2020.8K.REMASTER.4320p-x264", "4320p"},
		{"Movie.2020.8K-x264", "8K"},
		{"Movie.2020.720p.WEBRip-x264", "720p"},
	}
	for _, c := range cases {
		if got := ParseTitleTech(c.in).Resolution; got != c.want {
			t.Errorf("ParseTitleTech(%q).Resolution = %q, want %q", c.in, got, c.want)
		}
	}
}

// §59.190 挂账兑现: 4K修复版版式短语的 4K 不进 Resolution（编码分辨率未标）。
func TestResolutionEdition4KPhrase(t *testing.T) {
	cases := []struct{ in, want string }{
		{"午夜凶铃.4K修复版.1998.简繁中字￡CMCT梦幻", ""},
		{"浪人.4K修复版.1998", ""},
		{"Movie.1998.4K.REMASTERED.BluRay.x264-CMCT", ""},
		{"Movie.2023.4K.BluRay.x264-GRP", "4K"},                 // 裸 4K 无版式语境
		{"Movie.1985.4K修复版.720p.x264-CMCT", "720p"},          // 像素形态优先（§59.196）
		{"Movie.1998.8K修复版", ""},
	}
	for _, c := range cases {
		if got := ParseTitleTech(c.in).Resolution; got != c.want {
			t.Errorf("ParseTitleTech(%q).Resolution = %q, want %q", c.in, got, c.want)
		}
	}
}
