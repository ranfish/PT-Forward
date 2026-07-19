package metadata

import (
	"context"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

func TestPosterResolver_PTGenFirst_PTGenWins(t *testing.T) {
	r := NewPosterResolver()
	ptgen := &model.PTGenResult{PosterURL: "https://ptgen.com/poster.jpg"}
	detailPoster := "https://detail.com/poster.jpg"

	src := r.Resolve(context.Background(), ptgen, detailPoster, MergeModePTGenFirst)
	if src.URL != "https://ptgen.com/poster.jpg" {
		t.Errorf("PTGen first should pick PTGen poster, got %q", src.URL)
	}
	if src.Source != SourcePTGen {
		t.Errorf("source should be ptgen, got %q", src.Source)
	}
}

func TestPosterResolver_DetailFirst_DetailWins(t *testing.T) {
	r := NewPosterResolver()
	ptgen := &model.PTGenResult{PosterURL: "https://ptgen.com/poster.jpg"}
	detailPoster := "https://detail.com/poster.jpg"

	src := r.Resolve(context.Background(), ptgen, detailPoster, MergeModeDetailFirst)
	if src.URL != "https://detail.com/poster.jpg" {
		t.Errorf("detail first should pick detail poster, got %q", src.URL)
	}
	if src.Source != SourceDetail {
		t.Errorf("source should be detail, got %q", src.Source)
	}
}

func TestPosterResolver_PTGenOnly(t *testing.T) {
	r := NewPosterResolver()
	ptgen := &model.PTGenResult{PosterURL: "https://ptgen.com/poster.jpg"}

	src := r.Resolve(context.Background(), ptgen, "", MergeModePTGenFirst)
	if src.URL != "https://ptgen.com/poster.jpg" {
		t.Errorf("should pick PTGen, got %q", src.URL)
	}
	if src.Source != SourcePTGen {
		t.Errorf("source should be ptgen")
	}
}

func TestPosterResolver_DetailOnly(t *testing.T) {
	r := NewPosterResolver()

	src := r.Resolve(context.Background(), nil, "https://detail.com/poster.jpg", MergeModePTGenFirst)
	if src.URL != "https://detail.com/poster.jpg" {
		t.Errorf("should pick detail, got %q", src.URL)
	}
	if src.Source != SourceDetail {
		t.Errorf("source should be detail")
	}
}

func TestPosterResolver_BothEmpty(t *testing.T) {
	r := NewPosterResolver()
	src := r.Resolve(context.Background(), nil, "", MergeModePTGenFirst)
	if src.URL != "" {
		t.Errorf("should be empty URL, got %q", src.URL)
	}
	if src.Source != "" {
		t.Errorf("should be empty source, got %q", src.Source)
	}
}

func TestPosterResolver_PTGenEmpty_UseDetail(t *testing.T) {
	r := NewPosterResolver()
	ptgen := &model.PTGenResult{PosterURL: ""}

	src := r.Resolve(context.Background(), ptgen, "https://detail.com/poster.jpg", MergeModePTGenFirst)
	if src.URL != "https://detail.com/poster.jpg" {
		t.Errorf("PTGen empty should fallback to detail, got %q", src.URL)
	}
	if src.Source != SourceDetail {
		t.Errorf("source should be detail")
	}
}

func TestPosterResolver_NilPTGen(t *testing.T) {
	r := NewPosterResolver()
	src := r.Resolve(context.Background(), nil, "https://detail.com/poster.jpg", MergeModePTGenFirst)
	if src.URL != "https://detail.com/poster.jpg" {
		t.Errorf("nil PTGen should fallback to detail, got %q", src.URL)
	}
}

func TestPosterResolver_FallbackChain_PTGenFirst(t *testing.T) {
	// ptgen_first 模式：PTGen 有 → 用 PTGen（不 fallback）
	r := NewPosterResolver()
	ptgen := &model.PTGenResult{PosterURL: "https://ptgen.com/poster.jpg"}
	detailPoster := "https://detail.com/poster.jpg"

	src := r.Resolve(context.Background(), ptgen, detailPoster, MergeModePTGenFirst)
	if src.Source != SourcePTGen {
		t.Errorf("ptgen_first + PTGen has poster → should use PTGen")
	}
}

func TestPosterResolver_FallbackChain_DetailFirst(t *testing.T) {
	// detail_first 模式：detail 有 → 用 detail（不 fallback）
	r := NewPosterResolver()
	ptgen := &model.PTGenResult{PosterURL: "https://ptgen.com/poster.jpg"}
	detailPoster := "https://detail.com/poster.jpg"

	src := r.Resolve(context.Background(), ptgen, detailPoster, MergeModeDetailFirst)
	if src.Source != SourceDetail {
		t.Errorf("detail_first + detail has poster → should use detail")
	}
}
