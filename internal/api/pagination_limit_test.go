package api

import (
	"net/http/httptest"
	"testing"
)

// §59.565 D2 锚：parseLimitQuery 六态（top-N limit 契约锁定）。
func TestParseLimitQuery(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		want    int
		clamped bool
	}{
		{"缺省回落 20", "", 20, false},
		{"零回落 20", "0", 20, false},
		{"负数回落 20", "-5", 20, false},
		{"正常值透传", "50", 50, false},
		{"边界 200 不 clamp", "200", 200, false},
		{"超限 clamp 200", "10000", 200, true},
	}
	for _, c := range cases {
		r := httptest.NewRequest("GET", "/?limit="+c.raw, nil)
		got, clamped := parseLimitQuery(r)
		if got != c.want || clamped != c.clamped {
			t.Errorf("%s: limit=%q got (%d,%v), want (%d,%v)",
				c.name, c.raw, got, clamped, c.want, c.clamped)
		}
	}
}
