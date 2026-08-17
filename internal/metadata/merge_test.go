package metadata

import (
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/metadata/extract"
	"github.com/ranfish/pt-forward/internal/model"
)

func makeDetail(title, body string) *DetailSourceJSON {
	return &DetailSourceJSON{
		Title:     title,
		Subtitle:  "detail-sub",
		Intro:     extract.IntroData{Statement: "detail-stmt", Body: body, Poster: "detail-poster"},
		MediaInfo: "detail-mi",
		BDInfo:    "detail-bd",
		Type:      "电影",
		Tags:      []string{"detail-tag"},
		IMDbLink:  "https://imdb.com/detail",
		FetchedAt: time.Now(),
	}
}

func makePTGen(title, body string) *PTGenSourceJSON {
	return &PTGenSourceJSON{
		PTGenResult: model.PTGenResult{
			ChineseTitle:  title,
			ForeignTitle:  "ptgen-foreign",
			Introduction:  body,
			PosterURL:     "ptgen-poster",
			DoubanURL:     "https://douban.com/ptgen",
			IMDBURL:       "https://imdb.com/ptgen",
			Genre:         []string{"动作"},
			Cast:          []model.PersonInfo{{Name: "演员"}},
			RawBBCode:     "[ptgen]",
		},
		FetchedAt: time.Now(),
	}
}

func makeLocal() *LocalSourceJSON {
	return &LocalSourceJSON{
		MediaInfo:   "local-mi",
		BDInfo:      "local-bd",
		Screenshots: []string{"local-shot1", "local-shot2"},
		GeneratedAt: time.Now(),
	}
}

func TestMerge_PTGenFirst_PTGenWinsCommonFields(t *testing.T) {
	d := makeDetail("detail-title", "detail-body")
	p := makePTGen("ptgen-title", "ptgen-body")
	m := Merge(d, p, nil, MergeModePTGenFirst)

	if m.Title != "ptgen-title" {
		t.Errorf("Title should come from PTGen, got %q", m.Title)
	}
	if m.SourceOf["title"] != SourcePTGen {
		t.Errorf("title source should be ptgen, got %q", m.SourceOf["title"])
	}
	if m.Intro.Body != "ptgen-body" {
		t.Errorf("Body should come from PTGen, got %q", m.Intro.Body)
	}
	if m.Intro.Poster != "ptgen-poster" {
		t.Errorf("Poster should come from PTGen, got %q", m.Intro.Poster)
	}
	if m.IMDbURL != "https://imdb.com/ptgen" {
		t.Errorf("IMDbURL should come from PTGen, got %q", m.IMDbURL)
	}
}

func TestMerge_DetailFirst_DetailWinsCommonFields(t *testing.T) {
	d := makeDetail("detail-title", "detail-body")
	p := makePTGen("ptgen-title", "ptgen-body")
	m := Merge(d, p, nil, MergeModeDetailFirst)

	if m.Title != "detail-title" {
		t.Errorf("Title should come from Detail, got %q", m.Title)
	}
	if m.SourceOf["title"] != SourceDetail {
		t.Errorf("title source should be detail, got %q", m.SourceOf["title"])
	}
	if m.Intro.Body != "detail-body" {
		t.Errorf("Body should come from Detail, got %q", m.Intro.Body)
	}
	if m.Intro.Poster != "detail-poster" {
		t.Errorf("Poster should come from Detail, got %q", m.Intro.Poster)
	}
	if m.IMDbURL != "https://imdb.com/detail" {
		t.Errorf("IMDbURL should come from Detail, got %q", m.IMDbURL)
	}
}

func TestMerge_AlwaysPreserved_Statement(t *testing.T) {
	d := makeDetail("title", "body")
	d.Intro.Statement = "官组声明（始终保留）"

	for _, mode := range []MergeMode{MergeModePTGenFirst, MergeModeDetailFirst} {
		m := Merge(d, makePTGen("ptgen-title", "ptgen-body"), nil, mode)
		if m.Intro.Statement != "官组声明（始终保留）" {
			t.Errorf("mode=%v: Statement should be preserved, got %q", mode, m.Intro.Statement)
		}
		if m.SourceOf["statement"] != SourceDetail {
			t.Errorf("mode=%v: statement source should be detail", mode)
		}
	}
}

func TestMerge_MediaInfo_LocalOverDetail(t *testing.T) {
	d := makeDetail("title", "body")
	l := makeLocal()
	m := Merge(d, nil, l, MergeModePTGenFirst)

	if m.MediaInfo != "local-mi" {
		t.Errorf("MediaInfo should come from Local, got %q", m.MediaInfo)
	}
	if m.SourceOf["mediainfo"] != SourceLocal {
		t.Errorf("mediainfo source should be local, got %q", m.SourceOf["mediainfo"])
	}
	if m.BDInfo != "local-bd" {
		t.Errorf("BDInfo should come from Local, got %q", m.BDInfo)
	}
}

func TestMerge_Screenshots_LocalOverDetail(t *testing.T) {
	d := makeDetail("title", "body")
	d.Intro.SetScreenshotURLs([]string{"detail-shot"})
	l := makeLocal()
	m := Merge(d, nil, l, MergeModePTGenFirst)

	urls := m.Intro.ScreenshotURLs()
	if len(urls) != 2 || urls[0] != "local-shot1" {
		t.Errorf("Screenshots should come from Local, got %v", urls)
	}
	if m.SourceOf["screenshots"] != SourceLocal {
		t.Errorf("screenshots source should be local", )
	}
}

func TestMerge_PTGenUniqueFields_AlwaysKept(t *testing.T) {
	p := makePTGen("title", "body")
	for _, mode := range []MergeMode{MergeModePTGenFirst, MergeModeDetailFirst} {
		m := Merge(nil, p, nil, mode)
		if len(m.PTGen.Genre) != 1 || m.PTGen.Genre[0] != "动作" {
			t.Errorf("mode=%v: PTGen.Genre should be preserved", mode)
		}
		if len(m.PTGen.Cast) != 1 {
			t.Errorf("mode=%v: PTGen.Cast should be preserved", mode)
		}
	}
}

func TestMerge_AllNil(t *testing.T) {
	m := Merge(nil, nil, nil, MergeModePTGenFirst)
	if m.Title != "" {
		t.Errorf("expected empty Title, got %q", m.Title)
	}
	if len(m.Tags) != 0 {
		t.Errorf("expected empty Tags, got %v", m.Tags)
	}
	if m.SourceOf == nil {
		t.Error("SourceOf should be initialized")
	}
}

func TestMerge_PTGenOnly_FallsBackToDetail(t *testing.T) {
	// ptgen_first 模式下 PTGen 无 Title → fallback 到 Detail
	p := &PTGenSourceJSON{PTGenResult: model.PTGenResult{ChineseTitle: ""}, FetchedAt: time.Now()}
	d := makeDetail("detail-fallback", "body")
	m := Merge(d, p, nil, MergeModePTGenFirst)

	if m.Title != "detail-fallback" {
		t.Errorf("Title should fallback to detail, got %q", m.Title)
	}
	if m.SourceOf["title"] != SourceDetail {
		t.Errorf("title source should be detail", )
	}
}

func TestMerge_StructuredFields_FromDetail(t *testing.T) {
	d := makeDetail("title", "body")
	d.Type = "电视剧"
	// §59.34: detail 提取器存 standard key（生产形态），Merge 出口 ReverseLookup 归一化
	d.Resolution = "resolution.r2160p"
	d.VideoCodec = "video.h265"
	d.ReleaseGroup = "FRDS"
	m := Merge(d, nil, nil, MergeModePTGenFirst)

	if m.Type != "电视剧" {
		t.Errorf("Type mismatch: %q", m.Type)
	}
	if m.Resolution != "2160p" {
		t.Errorf("Resolution mismatch: %q", m.Resolution)
	}
	if m.VideoCodec != "HEVC" {
		t.Errorf("VideoCodec mismatch: %q", m.VideoCodec)
	}
	if m.ReleaseGroup != "FRDS" {
		t.Errorf("ReleaseGroup mismatch: %q", m.ReleaseGroup)
	}
}

func TestMerge_TagsDedup(t *testing.T) {
	// detail.Tags 没有重复，但验证 appendNonEmpty 去重
	d := makeDetail("title", "body")
	d.Tags = []string{"喜剧", "喜剧", "动作"}
	m := Merge(d, nil, nil, MergeModePTGenFirst)

	if len(m.Tags) != 2 {
		t.Errorf("Tags should be deduped, got %v", m.Tags)
	}
}
