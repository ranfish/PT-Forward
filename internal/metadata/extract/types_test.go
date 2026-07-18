package extract

import (
	"testing"
)

func TestIntroData_ScreenshotURLs_RoundTrip(t *testing.T) {
	urls := []string{
		"https://example.com/shot1.jpg",
		"https://example.com/shot2.jpg",
		"https://example.com/shot3.jpg",
	}
	var i IntroData
	i.SetScreenshotURLs(urls)

	got := i.ScreenshotURLs()
	if len(got) != len(urls) {
		t.Fatalf("expected %d urls, got %d: %v", len(urls), len(got), got)
	}
	for k, v := range got {
		if v != urls[k] {
			t.Errorf("[%d] expected %q, got %q", k, urls[k], v)
		}
	}
}

func TestIntroData_ScreenshotURLs_Empty(t *testing.T) {
	var i IntroData
	if got := i.ScreenshotURLs(); got != nil {
		t.Errorf("expected nil for empty screenshots, got %v", got)
	}
	i.SetScreenshotURLs(nil)
	if i.Screenshots != "" {
		t.Errorf("expected empty string, got %q", i.Screenshots)
	}
}

func TestIntroData_SetScreenshotURLs_FiltersEmpty(t *testing.T) {
	var i IntroData
	i.SetScreenshotURLs([]string{"", "  ", "https://valid.com/x.jpg", ""})
	if i.Screenshots != "[img]https://valid.com/x.jpg[/img]" {
		t.Errorf("expected single valid img, got: %q", i.Screenshots)
	}
}

func TestSeedData_IsMeaningful(t *testing.T) {
	cases := []struct {
		name string
		seed SeedData
		want bool
	}{
		{"empty", SeedData{}, false},
		{"whitespace only", SeedData{Title: "   ", Intro: IntroData{Body: "  "}}, false},
		{"title", SeedData{Title: "电影"}, true},
		{"body", SeedData{Intro: IntroData{Body: "正文"}}, true},
		{"mediainfo", SeedData{MediaInfo: "General"}, true},
		{"bdinfo", SeedData{BDInfo: "BDSummary"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.seed.IsMeaningful(); got != c.want {
				t.Errorf("IsMeaningful() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestSeedData_NormalizeWithFallback(t *testing.T) {
	seed := SeedData{
		Title:   "  电影  ",
		Subtitle: "  ",
		Intro:   IntroData{Body: "  正文  "},
		Tags:    nil,
		Flags:   nil,
	}
	got := seed.NormalizeWithFallback("fallback title")

	if got.Title != "电影" {
		t.Errorf("Title should be trimmed, got %q", got.Title)
	}
	if got.Intro.Body != "正文" {
		t.Errorf("Body should be trimmed, got %q", got.Intro.Body)
	}
	if got.Tags == nil {
		t.Error("Tags should be initialized")
	}
	if got.Flags == nil {
		t.Error("Flags should be initialized")
	}

	// 空 Title → fallback
	empty := SeedData{}.NormalizeWithFallback("fallback")
	if empty.Title != "fallback" {
		t.Errorf("empty Title should use fallback, got %q", empty.Title)
	}
}
