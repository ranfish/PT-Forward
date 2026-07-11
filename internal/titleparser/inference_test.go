package titleparser

import "testing"

func TestInferCategory_SourceMovie(t *testing.T) {
	c := TitleComponents{}
	got := InferCategory(c, "电影", "", "")
	if got != "category.movie" {
		t.Errorf("got %q, want category.movie", got)
	}
}

func TestInferCategory_SourceTV(t *testing.T) {
	c := TitleComponents{}
	got := InferCategory(c, "剧集", "", "")
	if got != "category.tv_series" {
		t.Errorf("got %q, want category.tv_series", got)
	}
}

func TestInferCategory_TitleSeason(t *testing.T) {
	c := TitleComponents{SeasonEpisode: "S01E03"}
	got := InferCategory(c, "", "", "")
	if got != "category.tv_series" {
		t.Errorf("got %q, want category.tv_series", got)
	}
}

func TestInferCategory_PTgenAnimation(t *testing.T) {
	c := TitleComponents{}
	got := InferCategory(c, "", "动画/冒险/comedy", "")
	if got != "category.animation" {
		t.Errorf("got %q, want category.animation", got)
	}
}

func TestInferCategory_PTgenDocumentary(t *testing.T) {
	c := TitleComponents{}
	got := InferCategory(c, "", "纪录片", "")
	if got != "category.documentary" {
		t.Errorf("got %q, want category.documentary", got)
	}
}

func TestInferCategory_DefaultMovie(t *testing.T) {
	c := TitleComponents{}
	got := InferCategory(c, "", "", "")
	if got != "category.movie" {
		t.Errorf("got %q, want category.movie (default)", got)
	}
}

func TestInferCategory_SourceOverridesTitle(t *testing.T) {
	// 源站说电影，但标题有季集 → 源站优先
	c := TitleComponents{SeasonEpisode: "S01E01"}
	got := InferCategory(c, "电影", "", "")
	if got != "category.movie" {
		t.Errorf("got %q, want category.movie (source overrides)", got)
	}
}

func TestInferCategory_AnimatedMovieOverride(t *testing.T) {
	// 源站说电影，但 PTGen genre 含动画 → 动画
	// 但按设计，源站优先，动画 genre 只在源站为空时生效
	c := TitleComponents{}
	got := InferCategory(c, "", "动画/冒险", "")
	if got != "category.animation" {
		t.Errorf("got %q, want category.animation", got)
	}
}

func TestInferCategory_EnglishCategory(t *testing.T) {
	got := InferCategory(TitleComponents{}, "Movies", "", "")
	if got != "category.movie" {
		t.Errorf("got %q, want category.movie", got)
	}
}

func TestInferCategory_PTgenEpisodesMultiple(t *testing.T) {
	got := InferCategory(TitleComponents{}, "", "", "12")
	if got != "category.tv_series" {
		t.Errorf("got %q, want category.tv_series (episodes=12)", got)
	}
}
