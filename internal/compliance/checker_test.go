package compliance

import (
	"context"
	"testing"
)

func TestCheck_Adult(t *testing.T) {
	c := NewChecker(nil, nil)
	ctx := context.Background()
	tests := []struct {
		title string
		want  string
	}{
		{"Movie.Name.9KG.xxx", "adult"},
		{"Some.Show.NSFW", "adult"},
		{"Adult.Movie.2024", "adult"},
		{"Normal.Movie.2024", ""},
		// qui detector cases (structural patterns, not keywords)
		{"ABP-100.Some.Title", "adult"},         // JAV code
		{"[2024.01.01].Some.Title", "adult"},     // bracket date
		{"010124_001.Some.Title", "adult"},       // adult date pattern
		// false positive checks
		{"Movie.2024.1080p.BluRay.x264.AC3-FGT", ""},    // AC3 codec, not JAV
		{"AAEJ-123.Some.Show.S01E02.1080p", ""},          // JAV-like + video markers
	}
	for _, tt := range tests {
		r := c.Check(ctx, tt.title)
		if tt.want == "" {
			if !r.Passed {
				t.Errorf("Check(%q) = %s, want pass", tt.title, r.Category)
			}
		} else {
			if r.Passed || r.Category != tt.want {
				t.Errorf("Check(%q) = {Passed:%v, Category:%s}, want Category:%s", tt.title, r.Passed, r.Category, tt.want)
			}
		}
	}
}

func TestCheck_ForbiddenTransfer(t *testing.T) {
	c := NewChecker(nil, nil)
	r := c.Check(context.Background(), "禁转.Movie.2024")
	if r.Passed || r.Category != "forbidden_transfer" {
		t.Errorf("expected forbidden_transfer, got %v %s", r.Passed, r.Category)
	}
}

func TestCheck_ForbiddenGroup(t *testing.T) {
	c := NewChecker(nil, nil)
	r := c.Check(context.Background(), "CatEDU.Some.Course")
	if r.Passed || r.Category != "forbidden_group" {
		t.Errorf("expected forbidden_group, got %v %s", r.Passed, r.Category)
	}
}

func TestCheck_Empty(t *testing.T) {
	c := NewChecker(nil, nil)
	r := c.Check(context.Background(), "")
	if !r.Passed {
		t.Errorf("empty title should pass")
	}
}

func TestExtractReleaseGroup(t *testing.T) {
	tests := []struct {
		title string
		want  string
	}{
		{"Movie.Name.2024-GROUP", "GROUP"},
		{"Show.S01.1080p.WEB-DL.x264-UBWEB", "UBWEB"},
		{"NoGroup", ""},
	}
	for _, tt := range tests {
		got := extractReleaseGroup(tt.title)
		if got != tt.want {
			t.Errorf("extractReleaseGroup(%q) = %q, want %q", tt.title, got, tt.want)
		}
	}
}
