package extract

import (
	"errors"
	"testing"
)

// stubExtractor 测试用桩提取器。
type stubExtractor struct {
	name   string
	seed   SeedData
	err    error
	called int
}

func (s *stubExtractor) Name() string { return s.name }
func (s *stubExtractor) Extract(_ Input) (SeedData, error) {
	s.called++
	return s.seed, s.err
}

func TestEngine_SpecialHit(t *testing.T) {
	special := &stubExtractor{name: "pterclub-special", seed: SeedData{Title: "PTer 专用"}}
	pub := &stubExtractor{name: "public", seed: SeedData{Title: "public fallback"}}

	e := NewEngine(pub, map[string]Extractor{"pterclub": special})
	seed, meta := e.Extract(Input{SiteCode: "pterclub", FallbackTitle: "fallback"})

	if seed.Title != "PTer 专用" {
		t.Errorf("expected special output, got %q", seed.Title)
	}
	if meta.UsedFallback {
		t.Error("should not use fallback")
	}
	if meta.ExtractorName != "pterclub-special" {
		t.Errorf("expected extractor=pterclub-special, got %q", meta.ExtractorName)
	}
	if pub.called != 0 {
		t.Error("public extractor should not be called")
	}
}

func TestEngine_FallbackOnNonMeaningful(t *testing.T) {
	// special 返回空 SeedData（IsMeaningful=false）→ fallback public
	special := &stubExtractor{name: "pterclub-special", seed: SeedData{}}
	pub := &stubExtractor{name: "public", seed: SeedData{Title: "public"}}

	e := NewEngine(pub, map[string]Extractor{"pterclub": special})
	seed, meta := e.Extract(Input{SiteCode: "pterclub", FallbackTitle: "fallback"})

	if seed.Title != "public" {
		t.Errorf("expected fallback public output, got %q", seed.Title)
	}
	if !meta.UsedFallback {
		t.Error("UsedFallback should be true")
	}
	if meta.ExtractorName != "public" {
		t.Errorf("expected extractor=public, got %q", meta.ExtractorName)
	}
	if meta.FallbackReason == "" {
		t.Error("FallbackReason should be set")
	}
}

func TestEngine_FallbackOnError(t *testing.T) {
	special := &stubExtractor{name: "pterclub-special", err: errors.New("network timeout")}
	pub := &stubExtractor{name: "public", seed: SeedData{Title: "public"}}

	e := NewEngine(pub, map[string]Extractor{"pterclub": special})
	seed, meta := e.Extract(Input{SiteCode: "pterclub", FallbackTitle: "fallback"})

	if seed.Title != "public" {
		t.Errorf("expected fallback public output on special error, got %q", seed.Title)
	}
	if !meta.UsedFallback {
		t.Error("UsedFallback should be true")
	}
}

func TestEngine_NoSpecial_UsePublic(t *testing.T) {
	pub := &stubExtractor{name: "public", seed: SeedData{Title: "public"}}
	e := NewEngine(pub, nil)
	seed, meta := e.Extract(Input{SiteCode: "unknown-site", FallbackTitle: "fallback"})

	if seed.Title != "public" {
		t.Errorf("expected public output, got %q", seed.Title)
	}
	// 没找到 special 直接用 public，UsedFallback 应为 false（没有发生回退）
	if meta.UsedFallback {
		t.Error("UsedFallback should be false when no special found")
	}
}

func TestEngine_SpecialByNickname(t *testing.T) {
	special := &stubExtractor{name: "猫站-special", seed: SeedData{Title: "猫站"}}
	pub := &stubExtractor{name: "public", seed: SeedData{}}

	// "猫站" 含中文 → specialByNick
	e := NewEngine(pub, map[string]Extractor{"猫站": special})
	seed, meta := e.Extract(Input{SiteNickname: "猫站", FallbackTitle: "fallback"})

	if seed.Title != "猫站" {
		t.Errorf("expected nickname-based lookup, got %q", seed.Title)
	}
	if meta.ExtractorName != "猫站-special" {
		t.Errorf("expected extractor=猫站-special, got %q", meta.ExtractorName)
	}
}

func TestEngine_NoExtractor(t *testing.T) {
	e := NewEngine(nil, nil)
	seed, meta := e.Extract(Input{SiteCode: "any", FallbackTitle: "fallback"})

	if seed.IsMeaningful() {
		t.Error("expected empty seed")
	}
	if meta.FallbackReason == "" {
		t.Error("FallbackReason should be set")
	}
}

func TestEngine_FallbackTitleFill(t *testing.T) {
	// public 返回空 Title → NormalizeWithFallback 用 FallbackTitle 填充
	pub := &stubExtractor{name: "public", seed: SeedData{MediaInfo: "General"}}
	e := NewEngine(pub, nil)
	seed, _ := e.Extract(Input{SiteCode: "any", FallbackTitle: "fallback-title"})

	if seed.Title != "fallback-title" {
		t.Errorf("expected fallback title, got %q", seed.Title)
	}
}

func TestIsLikelySiteCode(t *testing.T) {
	cases := []struct {
		s    string
		want bool
	}{
		{"pterclub", true},
		{"hhanclub", true},
		{"ssd", true},
		{"hd_home", true},
		{"hdsky-2", true},
		{"猫站", false},
		{"HDHome", false}, // 含大写
		{"", false},
		{"不可说", false},
	}
	for _, c := range cases {
		if got := isLikelySiteCode(c.s); got != c.want {
			t.Errorf("isLikelySiteCode(%q) = %v, want %v", c.s, got, c.want)
		}
	}
}
