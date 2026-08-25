package titleparser

import "testing"

func TestRegionGenreLookup(t *testing.T) {
	cases := []struct{ domain, raw, want string }{
		{"region", "美国", "region.us"},
		{"region", "英国", "region.gb"},
		{"region", "中国香港", "region.hk"},
		{"genre", "剧情", "genre.drama"},
		{"genre", "科幻", "genre.scifi"},
		{"genre", "纪录片", "genre.documentary"},
	}
	for _, c := range cases {
		if got := LookupDictKey(c.domain, c.raw); got != c.want {
			t.Errorf("LookupDictKey(%s, %q) = %q, want %q", c.domain, c.raw, got, c.want)
		}
	}
}
