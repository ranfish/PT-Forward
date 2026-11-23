package api

import (
	"net/http"
	"net/url"
	"testing"
)

// §59.559 PR-D 契约锁定：三 helper 的缺省/合法/非法行为。
func TestParseBoolQuery(t *testing.T) {
	mk := func(q string) *http.Request {
		return &http.Request{URL: &url.URL{RawQuery: q}}
	}
	cases := []struct {
		query, key string
		def, want  bool
	}{
		{"x=1", "x", false, true},
		{"x=true", "x", false, true},
		{"x=0", "x", true, false},
		{"x=false", "x", true, false},
		{"", "x", false, false},
		{"", "x", true, true},
		{"x=", "x", true, true},     // 空值=缺省
		{"x=junk", "x", true, true}, // 非法=缺省
	}
	for _, c := range cases {
		if got := parseBoolQuery(mk(c.query), c.key, c.def); got != c.want {
			t.Errorf("?%s def=%v: got %v want %v", c.query, c.def, got, c.want)
		}
	}
}

func TestParseUintQuery(t *testing.T) {
	mk := func(q string) *http.Request {
		return &http.Request{URL: &url.URL{RawQuery: q}}
	}
	if got := parseUintQuery(mk("cid=42"), "cid", 0); got != 42 {
		t.Errorf("合法: got %d", got)
	}
	if got := parseUintQuery(mk(""), "cid", 7); got != 7 {
		t.Errorf("缺省: got %d", got)
	}
	if got := parseUintQuery(mk("cid=abc"), "cid", 7); got != 7 {
		t.Errorf("非法: got %d", got)
	}
	if got := parseUintQuery(mk("cid=-1"), "cid", 7); got != 7 {
		t.Errorf("负数: got %d", got)
	}
}

func TestParseIntQuery(t *testing.T) {
	mk := func(q string) *http.Request {
		return &http.Request{URL: &url.URL{RawQuery: q}}
	}
	if got := parseIntQuery(mk("limit=50"), "limit", 20); got != 50 {
		t.Errorf("合法: got %d", got)
	}
	if got := parseIntQuery(mk("limit=-3"), "limit", 20); got != -3 {
		t.Errorf("负数合法: got %d", got)
	}
	if got := parseIntQuery(mk(""), "limit", 20); got != 20 {
		t.Errorf("缺省: got %d", got)
	}
	if got := parseIntQuery(mk("limit=xyz"), "limit", 20); got != 20 {
		t.Errorf("非法: got %d", got)
	}
}
