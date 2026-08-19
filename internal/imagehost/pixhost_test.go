package imagehost

import (
	"context"
	"testing"
)

func TestPixhostRegex(t *testing.T) {
	tests := []struct {
		name    string
		html    string
		wantURL string
	}{
		{
			"to domain jpg",
			`<img src="https://img2.pixhost.to/images/7047/713252627_test.jpg">`,
			"https://img2.pixhost.to/images/7047/713252627_test.jpg",
		},
		{
			"cc domain png",
			`<img src="https://img2.pixhost.cc/images/9452/750578020_test.png">`,
			"https://img2.pixhost.cc/images/9452/750578020_test.png",
		},
		{
			"cc domain jpg",
			`<img src="https://img3.pixhost.cc/images/100/123_test.jpg">`,
			"https://img3.pixhost.cc/images/100/123_test.jpg",
		},
		{
			"to domain jpeg",
			`<img src="https://img1.pixhost.to/images/50/test.jpeg">`,
			"https://img1.pixhost.to/images/50/test.jpeg",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match := rePixhostDirect.FindString(tt.html)
			if match != tt.wantURL {
				t.Errorf("regex match = %q, want %q", match, tt.wantURL)
			}
		})
	}
}

func TestPixhostRegex_NoMatch(t *testing.T) {
	nonMatching := []string{
		`<img src="https://example.com/image.jpg">`,
		`<img src="https://img2.pixhost.org/images/test.jpg">`, // .org not matched
		`<img src="https://pixhost.to/images/test.gif">`,       // .gif not matched (only jpg/jpeg/png)
		`<img src="https://img2.pixhost.to/icons/test.png">`,  // /icons/ not /images/
	}
	for _, html := range nonMatching {
		if match := rePixhostDirect.FindString(html); match != "" {
			t.Errorf("regex should not match %q, got %q", html, match)
		}
	}
}

func TestPixhostDomainList(t *testing.T) {
	host := NewPixhostHost(nil)
	if len(host.domains) != 3 {
		t.Fatalf("expected 3 domains, got %d", len(host.domains))
	}
	if host.domains[0] != "pixhost.cc" {
		t.Errorf("first domain should be pixhost.cc (priority), got %q", host.domains[0])
	}
	if host.domains[1] != "pixhost.to" {
		t.Errorf("second domain should be pixhost.to (fallback), got %q", host.domains[1])
	}
	if host.domains[2] != "pixho.st" {
		t.Errorf("third domain should be pixho.st (§59.42), got %q", host.domains[2])
	}
}

func TestPixhostHealthCheck(t *testing.T) {
	host := NewPixhostHost(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 15_000_000_000)
	defer cancel()
	err := host.HealthCheck(ctx)
	// pixhost.cc should be reachable
	if err != nil {
		t.Logf("HealthCheck failed (may be network issue): %v", err)
	}
}
