package reseed

import "testing"

// §59.234 ②: 频道词剥离（CCTV 族——剧名区频道词非片名）
func TestChannelWordStrip(t *testing.T) {
	cases := []struct{ in, want string }{
		{"[刘老庄八十二壮士].CCTV6.82.Warriors.2013.1080p.HDTV.H264.AAC-CMCTV.mp4", "82 Warriors 2013 1080p HDTV"},
		{"Movie.2023.1080p.BluRay.x264-CMCT", "Movie 2023 1080p BluRay"},
	}
	for _, c := range cases {
		if got := ExtractSearchKeyword(c.in); got != c.want {
			t.Errorf("kw(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
