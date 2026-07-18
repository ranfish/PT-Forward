package metadata

import (
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/metadata/extract"
	"github.com/ranfish/pt-forward/internal/model"
)

func TestSeedToDetailSource_RoundTrip(t *testing.T) {
	now := time.Now()
	seed := extract.SeedData{
		Title:        "title",
		Subtitle:     "sub",
		MediaInfo:    "mi",
		Type:         "电影",
		VideoCodec:   "H.265",
		ReleaseGroup: "FRDS",
		Tags:         []string{"tag1"},
		Flags:        []string{"flag1"},
		IMDbLink:     "https://imdb.com/x",
		Intro:        extract.IntroData{Statement: "stmt", Body: "body"},
	}
	meta := extract.Meta{ExtractorName: "public", Duration: 10 * time.Millisecond}

	src := SeedToDetailSource(seed, now, meta)
	if src.Title != "title" || src.Type != "电影" || src.IMDbLink != "https://imdb.com/x" {
		t.Errorf("SeedToDetailSource field mismatch: %+v", src)
	}
	if !src.FetchedAt.Equal(now) {
		t.Errorf("FetchedAt mismatch")
	}
	if src.ExtractorInfo.ExtractorName != "public" {
		t.Errorf("ExtractorInfo mismatch")
	}

	back := DetailSourceToSeed(src)
	if back.Title != seed.Title || back.Type != seed.Type || back.IMDbLink != seed.IMDbLink {
		t.Errorf("DetailSourceToSeed mismatch: %+v", back)
	}
	if back.Intro.Statement != "stmt" || back.Intro.Body != "body" {
		t.Errorf("Intro mismatch: %+v", back.Intro)
	}
}

func TestPTGenToSource_RoundTrip(t *testing.T) {
	now := time.Now()
	result := model.PTGenResult{
		ChineseTitle: "中文标题",
		Genre:        []string{"动作", "喜剧"},
		IMDBURL:      "https://imdb.com/x",
	}

	src := PTGenToSource(result, now)
	if src.ChineseTitle != "中文标题" {
		t.Errorf("ChineseTitle mismatch")
	}
	if len(src.Genre) != 2 {
		t.Errorf("Genre mismatch")
	}
	if !src.FetchedAt.Equal(now) {
		t.Errorf("FetchedAt mismatch")
	}

	back := SourceToPTGen(src)
	if back.ChineseTitle != result.ChineseTitle {
		t.Errorf("SourceToPTGen ChineseTitle mismatch")
	}
	if len(back.Genre) != len(result.Genre) {
		t.Errorf("SourceToPTGen Genre mismatch")
	}
}

func TestArtifactToSource_RoundTrip(t *testing.T) {
	now := time.Now()
	shots := []string{"s1.jpg", "s2.jpg"}

	src := ArtifactToSource("mi", "bd", shots, now)
	if src.MediaInfo != "mi" || src.BDInfo != "bd" {
		t.Errorf("ArtifactToSource mismatch")
	}
	if len(src.Screenshots) != 2 {
		t.Errorf("Screenshots mismatch")
	}

	mi, bd, ss := SourceToArtifact(src)
	if mi != "mi" || bd != "bd" {
		t.Errorf("SourceToArtifact mi/bd mismatch")
	}
	if len(ss) != 2 {
		t.Errorf("SourceToArtifact screenshots mismatch")
	}
}
