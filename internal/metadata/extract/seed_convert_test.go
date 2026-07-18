package extract

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

func TestSeedToDetail_Success(t *testing.T) {
	seed := SeedData{
		Title:        "电影 2024",
		Subtitle:     "副标题",
		MediaInfo:    "General info",
		BDInfo:       "BD info",
		Type:         "电影",
		VideoCodec:   "H.265",
		AudioCodec:   "DTS-HD MA",
		Resolution:   "2160p",
		Source:       "中国大陆",
		ReleaseGroup: "FRDS",
		Tags:         []string{"喜剧"},
		Flags:        []string{"禁转"},
		IMDbLink:     "https://www.imdb.com/title/tt1234/",
		DoubanLink:   "https://movie.douban.com/subject/1234/",
		TMDbLink:     "https://www.themoviedb.org/movie/1234",
		InfoHash:     "abc123",
		Size:         1024 * 1024 * 1024,
		Intro: IntroData{
			Statement: "声明",
			Poster:    "https://example.com/poster.jpg",
			Body:      "正文 BBCode",
		},
	}
	seed.Intro.SetScreenshotURLs([]string{"https://example.com/shot1.jpg"})

	d := SeedToDetail(seed)
	if d == nil {
		t.Fatal("expected non-nil detail")
	}
	if d.Title != seed.Title {
		t.Errorf("Title mismatch")
	}
	if d.Category != seed.Type {
		t.Errorf("Category/Type mismatch: %q vs %q", d.Category, seed.Type)
	}
	if d.Codec != seed.VideoCodec {
		t.Errorf("Codec/VideoCodec mismatch: %q vs %q", d.Codec, seed.VideoCodec)
	}
	if d.IMDbURL != seed.IMDbLink {
		t.Errorf("IMDbURL/IMDbLink mismatch")
	}
	if d.Description != seed.Intro.Body {
		t.Errorf("Description/Intro.Body mismatch")
	}
	if d.PosterURL != seed.Intro.Poster {
		t.Errorf("PosterURL/Intro.Poster mismatch")
	}
	if len(d.Screenshots) != 1 || d.Screenshots[0] != "https://example.com/shot1.jpg" {
		t.Errorf("Screenshots mismatch: %v", d.Screenshots)
	}
}

func TestSeedToDetail_NonMeaningful(t *testing.T) {
	if d := SeedToDetail(SeedData{}); d != nil {
		t.Error("expected nil for non-meaningful seed")
	}
}

func TestSeedToDetail_NilSlices(t *testing.T) {
	// IsMeaningful=true 但 Tags/Flags/Screenshots 为 nil
	seed := SeedData{Title: "x"}
	d := SeedToDetail(seed)
	if d.Tags == nil {
		t.Error("Tags should be initialized")
	}
	if d.Flags == nil {
		t.Error("Flags should be initialized")
	}
	if d.Screenshots == nil {
		t.Error("Screenshots should be initialized")
	}
}

func TestDetailToSeed_Success(t *testing.T) {
	d := &model.TorrentDetail{
		Title:        "测试",
		Subtitle:     "副标题",
		Description:  "正文",
		Category:     "电影",
		Codec:        "H.264",
		AudioCodec:   "TrueHD",
		Resolution:   "1080p",
		Source:       "日本",
		ReleaseGroup: "PTer",
		Tags:         []string{"动漫"},
		Flags:        []string{"限转"},
		IMDbURL:      "https://imdb.com/x",
		DoubanURL:    "https://douban.com/x",
		TMDbURL:      "https://tmdb.org/x",
		PosterURL:    "https://example.com/p.jpg",
		Screenshots:  []string{"https://example.com/s1.jpg"},
		MediaInfo:    "MI",
		BDInfo:       "BD",
		InfoHash:     "hash",
		Size:         100,
	}
	seed := DetailToSeed(d)
	if seed.Title != d.Title {
		t.Errorf("Title mismatch")
	}
	if seed.Type != d.Category {
		t.Errorf("Type/Category mismatch")
	}
	if seed.VideoCodec != d.Codec {
		t.Errorf("VideoCodec/Codec mismatch")
	}
	if seed.IMDbLink != d.IMDbURL {
		t.Errorf("IMDbLink/IMDbURL mismatch")
	}
	if seed.Intro.Body != d.Description {
		t.Errorf("Intro.Body/Description mismatch")
	}
	if seed.Intro.Poster != d.PosterURL {
		t.Errorf("Intro.Poster/PosterURL mismatch")
	}
	urls := seed.Intro.ScreenshotURLs()
	if len(urls) != 1 || urls[0] != "https://example.com/s1.jpg" {
		t.Errorf("Screenshots mismatch: %v", urls)
	}
}

func TestDetailToSeed_NilSafe(t *testing.T) {
	seed := DetailToSeed(nil)
	if seed.IsMeaningful() {
		t.Error("nil detail → seed should not be meaningful")
	}
}

func TestSeedToDetail_RoundTrip(t *testing.T) {
	original := SeedData{
		Title:        "往返测试",
		Type:         "电视剧",
		VideoCodec:   "H.265",
		Resolution:   "2160p",
		ReleaseGroup: "FRDS",
		Tags:         []string{"tag1", "tag2"},
		Flags:        []string{"flag1"},
		Intro:        IntroData{Body: "正文", Poster: "p.jpg"},
	}
	original.Intro.SetScreenshotURLs([]string{"s1.jpg", "s2.jpg"})

	detail := SeedToDetail(original)
	if detail == nil {
		t.Fatal("SeedToDetail returned nil")
	}
	roundTrip := DetailToSeed(detail)

	if roundTrip.Title != original.Title {
		t.Errorf("Title round-trip failed: %q vs %q", roundTrip.Title, original.Title)
	}
	if roundTrip.Type != original.Type {
		t.Errorf("Type round-trip failed")
	}
	if roundTrip.VideoCodec != original.VideoCodec {
		t.Errorf("VideoCodec round-trip failed")
	}
	if roundTrip.ReleaseGroup != original.ReleaseGroup {
		t.Errorf("ReleaseGroup round-trip failed")
	}
	if len(roundTrip.Tags) != len(original.Tags) {
		t.Errorf("Tags round-trip failed: %v vs %v", roundTrip.Tags, original.Tags)
	}
}
