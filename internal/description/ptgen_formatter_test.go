package description

import (
	"strings"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

func makeTestPTGen() *model.PTGenResult {
	return &model.PTGenResult{
		ChineseTitle: "中文片名",
		ForeignTitle: "English Title",
		Year:         "2024",
		Region:       []string{"中国大陆"},
		Genre:        []string{"动作", "喜剧"},
		Language:     []string{"普通话"},
		Playdate:     "2024-01-01",
		Duration:     "120分钟",
		Director:     []string{"张三"},
		Writer:       []string{"李四"},
		Cast: []model.PersonInfo{
			{Name: "王五"},
			{Name: "赵六"},
		},
		Introduction: "简介内容",
		PosterURL:    "https://example.com/poster.jpg",
		IMDBRating:   "8.5",
		IMDBVotes:    "100000",
		DoubanRating: "8.0",
		DoubanVotes:  "50000",
		IMDBURL:      "https://www.imdb.com/title/tt1234567/",
		DoubanURL:    "https://movie.douban.com/subject/12345/",
		Awards:       []string{"最佳影片"},
		AKA:          []string{"别名一"},
	}
}

func TestFormatPTGen_DoubanStyle(t *testing.T) {
	result := FormatPTGen(makeTestPTGen(), PTGenTemplateDouban)
	if result == "" {
		t.Fatal("result should not be empty")
	}
	if !strings.Contains(result, "◎译　名") {
		t.Errorf("douban style should contain '◎译　名', got: %q", result)
	}
	if !strings.Contains(result, "中文片名") {
		t.Errorf("should contain ChineseTitle")
	}
	if !strings.Contains(result, "◎IMDb评分") {
		t.Errorf("should contain '◎IMDb评分'")
	}
	if !strings.Contains(result, "8.5/10 from 100000 users") {
		t.Errorf("rating format mismatch, got: %q", result)
	}
	if !strings.Contains(result, "[img]https://example.com/poster.jpg[/img]") {
		t.Errorf("should contain poster img tag")
	}
	if !strings.Contains(result, "[url=https://movie.douban.com/subject/12345/]豆瓣链接[/url]") {
		t.Errorf("should contain douban url")
	}
}

func TestFormatPTGen_IMDbStyle(t *testing.T) {
	result := FormatPTGen(makeTestPTGen(), PTGenTemplateIMDb)
	if result == "" {
		t.Fatal("result should not be empty")
	}
	if !strings.Contains(result, "Title:") {
		t.Errorf("imdb style should contain 'Title:', got: %q", result)
	}
	if !strings.Contains(result, "English Title") {
		t.Errorf("should contain ForeignTitle")
	}
	if !strings.Contains(result, "Director:") {
		t.Errorf("should contain 'Director:'")
	}
	if !strings.Contains(result, "IMDb Rating:") {
		t.Errorf("should contain 'IMDb Rating:'")
	}
}

func TestFormatPTGen_NilResult(t *testing.T) {
	if result := FormatPTGen(nil, PTGenTemplateDouban); result != "" {
		t.Errorf("nil result should return empty string, got %q", result)
	}
}

func TestFormatPTGen_EmptyResult(t *testing.T) {
	empty := &model.PTGenResult{}
	if result := FormatPTGen(empty, PTGenTemplateDouban); result != "" {
		t.Errorf("empty result should return empty string, got %q", result)
	}
}

func TestFormatPTGen_PartialResult(t *testing.T) {
	partial := &model.PTGenResult{
		ForeignTitle: "Only Title",
		Year:         "2024",
	}
	result := FormatPTGen(partial, PTGenTemplateDouban)
	if result == "" {
		t.Fatal("partial result should not be empty")
	}
	if !strings.Contains(result, "Only Title") {
		t.Errorf("should contain ForeignTitle")
	}
	if strings.Contains(result, "◎译　名") {
		t.Errorf("should not contain empty ChineseTitle field")
	}
}

func TestFormatPTGen_CastLimit(t *testing.T) {
	r := &model.PTGenResult{
		ForeignTitle: "Test",
		Cast: []model.PersonInfo{
			{Name: "演员1"}, {Name: "演员2"}, {Name: "演员3"},
			{Name: "演员4"}, {Name: "演员5"}, {Name: "演员6"},
			{Name: "演员7"}, {Name: "演员8"}, {Name: "演员9"},
			{Name: "演员10"}, {Name: "演员11"}, {Name: "演员12"},
		},
	}
	result := FormatPTGen(r, PTGenTemplateDouban)
	if strings.Contains(result, "演员11") || strings.Contains(result, "演员12") {
		t.Errorf("cast should be limited to 10, got: %q", result)
	}
}

func TestFormatRating(t *testing.T) {
	cases := []struct {
		rating, votes, want string
	}{
		{"8.5", "100000", "8.5/10 from 100000 users"},
		{"7.0", "", "7.0"},
		{"", "100", ""},
		{"", "", ""},
	}
	for _, c := range cases {
		got := formatRating(c.rating, c.votes)
		if got != c.want {
			t.Errorf("formatRating(%q, %q) = %q, want %q", c.rating, c.votes, got, c.want)
		}
	}
}

func TestFormatPeople(t *testing.T) {
	people := []model.PersonInfo{
		{Name: "A"}, {Name: "B"}, {Name: ""},
	}
	if got := formatPeople(people, 0); !strings.Contains(got, "A") || !strings.Contains(got, "B") {
		t.Errorf("formatPeople should contain A and B, got %q", got)
	}
	// limit=2 → 只保留前 2 个
	got := formatPeople([]model.PersonInfo{
		{Name: "A"}, {Name: "B"}, {Name: "C"},
	}, 2)
	if strings.Contains(got, "C") {
		t.Errorf("formatPeople limit=2 should not contain C, got %q", got)
	}
	// 空
	if got := formatPeople(nil, 0); got != "" {
		t.Errorf("formatPeople(nil) should be empty, got %q", got)
	}
}

func TestJoinTitles(t *testing.T) {
	cases := []struct {
		main string
		aka  []string
		want string
	}{
		{"主", []string{"别名1", "别名2"}, "主 / 别名1 / 别名2"},
		{"", []string{"别名1"}, "别名1"},
		{"主", nil, "主"},
		{"", nil, ""},
	}
	for _, c := range cases {
		got := joinTitles(c.main, c.aka)
		if got != c.want {
			t.Errorf("joinTitles(%q, %v) = %q, want %q", c.main, c.aka, got, c.want)
		}
	}
}

func TestFormatField_PublicHelper(t *testing.T) {
	if got := FormatField("Label", "value"); got != "Label value\n" {
		t.Errorf("FormatField mismatch: %q", got)
	}
	if got := FormatField("Label", ""); got != "" {
		t.Errorf("empty value should return empty, got %q", got)
	}
}
