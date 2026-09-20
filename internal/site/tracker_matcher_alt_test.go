package site

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

func TestProbe_AltDomainMatch(t *testing.T) {
	m := NewTrackerMatcherFromSites([]model.Site{
		{Name: "不可说", Domain: "springsunday.net", AlternativeDomains: "hdcmct.org"},
		{Name: "猫", Domain: "pter.cat", AlternativeDomains: `["pterclub.com"]`},
	})
	if got := m.Match("http://hdcmct.org/"); got != "不可说" {
		t.Fatalf("裸串 alt 域未命中: %q", got)
	}
	if got := m.Match("http://pterclub.com/"); got != "猫" {
		t.Fatalf("JSON alt 域未命中: %q", got)
	}
	if got := m.Match("http://springsunday.net/"); got != "不可说" {
		t.Fatalf("主域失效: %q", got)
	}
}
