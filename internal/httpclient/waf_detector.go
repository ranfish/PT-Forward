package httpclient

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

type WAFResponseDetector struct {
	patterns   []wafPattern
	lastReason atomic.Value
}

type wafPattern struct {
	Name      string
	MatchFunc func(*http.Response) bool
	FreezeDur time.Duration
}

func NewWAFResponseDetector(customPatterns []wafPattern) *WAFResponseDetector {
	return &WAFResponseDetector{
		patterns: append(builtinWAFPatterns, customPatterns...),
	}
}

var builtinWAFPatterns = []wafPattern{
	{
		Name: "http_429",
		MatchFunc: func(resp *http.Response) bool {
			return resp.StatusCode == 429
		},
		FreezeDur: 30 * time.Second,
	},
	{
		Name: "cloudflare_challenge",
		MatchFunc: func(resp *http.Response) bool {
			if resp.StatusCode != 403 && resp.StatusCode != 503 {
				return false
			}
			body := peekBody(resp, 2048)
			return strings.Contains(body, "cf-browser-verification") ||
				strings.Contains(body, "jschl-answer") ||
				strings.Contains(body, "Just a moment") ||
				strings.Contains(body, "checking your browser")
		},
		FreezeDur: 5 * time.Minute,
	},
	{
		Name: "cloudflare_5s_shield",
		MatchFunc: func(resp *http.Response) bool {
			if resp.StatusCode != 503 {
				return false
			}
			body := peekBody(resp, 2048)
			return strings.Contains(body, "trk_jschal_js")
		},
		FreezeDur: 5 * time.Minute,
	},
	{
		Name: "login_redirect",
		MatchFunc: func(resp *http.Response) bool {
			if resp.StatusCode != 200 {
				return false
			}
			body := peekBody(resp, 4096)
			return strings.Contains(body, `<form`) &&
				(strings.Contains(body, `action="login"`) ||
					strings.Contains(body, `action="/login"`) ||
					strings.Contains(body, `action="/auth/login"`))
		},
		FreezeDur: 0,
	},
	{
		Name: "empty_response",
		MatchFunc: func(resp *http.Response) bool {
			if resp.StatusCode != 200 {
				return false
			}
			body := peekBody(resp, 512)
			return len(strings.TrimSpace(body)) == 0
		},
		FreezeDur: 60 * time.Second,
	},
	{
		Name: "rate_limit_text",
		MatchFunc: func(resp *http.Response) bool {
			if resp.StatusCode != 200 {
				return false
			}
			body := peekBody(resp, 4096)
			lower := strings.ToLower(body)
			return strings.Contains(lower, "rate limit") ||
				strings.Contains(lower, "too many requests") ||
				strings.Contains(lower, "请求过于频繁") ||
				strings.Contains(lower, "访问频率过高")
		},
		FreezeDur: 60 * time.Second,
	},
}

func (d *WAFResponseDetector) Detect(resp *http.Response) time.Duration {
	for _, p := range d.patterns {
		if p.MatchFunc(resp) {
			d.lastReason.Store(p.Name)
			return p.FreezeDur
		}
	}
	d.lastReason.Store("")
	return 0
}

func (d *WAFResponseDetector) LastReason() string {
	v := d.lastReason.Load()
	if v == nil {
		return ""
	}
	return v.(string)
}

const peekBodyMaxBytes int64 = 4096

func peekBody(resp *http.Response, n int64) string {
	if n > peekBodyMaxBytes {
		n = peekBodyMaxBytes
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, n))
	resp.Body = io.NopCloser(io.MultiReader(bytes.NewReader(body), resp.Body))
	return string(body)
}
