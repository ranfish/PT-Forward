package description

import (
	"strings"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

func TestRenderer_NilData(t *testing.T) {
	r := NewRenderer("bbcode")
	result, err := r.Render(nil, model.SiteDescConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestRenderer_BBCode(t *testing.T) {
	r := NewRenderer("bbcode")
	data := &model.DescriptionData{
		Statement:     "禁止转载",
		PosterURL:     "https://example.com/poster.jpg",
		PTGenBody:     "[b]豆瓣信息[/b]",
		MediaInfoText: "General\nTitle: test",
		Screenshots:   []string{"https://a.com/1.jpg", "https://a.com/2.jpg"},
		SourceSite:    "site1",
	}
	config := model.SiteDescConfig{Format: "bbcode"}

	result, err := r.Render(data, config)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result, "[b]声明[/b]") {
		t.Error("missing statement section")
	}
	if !strings.Contains(result, "[img]https://example.com/poster.jpg[/img]") {
		t.Error("missing poster")
	}
	if !strings.Contains(result, "[b]MediaInfo[/b]") {
		t.Error("missing mediainfo section")
	}
	if !strings.Contains(result, "[code]") {
		t.Error("missing code wrapper for mediainfo")
	}
	if !strings.Contains(result, "[img]https://a.com/1.jpg[/img]") {
		t.Error("missing screenshot 1")
	}
	if !strings.Contains(result, "转载自 site1") {
		t.Error("missing source site")
	}
}

func TestRenderer_Markdown(t *testing.T) {
	r := NewRenderer("markdown")
	data := &model.DescriptionData{
		Statement:     "test",
		MediaInfoText: "info",
	}
	config := model.SiteDescConfig{Format: "markdown"}

	result, err := r.Render(data, config)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result, "## 声明") {
		t.Error("missing markdown heading")
	}
	if !strings.Contains(result, "```") {
		t.Error("missing markdown code block")
	}
}

func TestRenderer_HTML(t *testing.T) {
	r := NewRenderer("html")
	data := &model.DescriptionData{
		Statement: "test",
	}
	config := model.SiteDescConfig{Format: "html"}

	result, err := r.Render(data, config)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result, "<h3>声明</h3>") {
		t.Error("missing HTML heading")
	}
}

func TestRenderer_DefaultFormat(t *testing.T) {
	r := NewRenderer("")
	data := &model.DescriptionData{Statement: "test"}
	config := model.SiteDescConfig{}

	result, err := r.Render(data, config)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "[b]声明[/b]") {
		t.Error("expected bbcode as default")
	}
}

func TestRenderer_ConfigOverridesRenderer(t *testing.T) {
	r := NewRenderer("html")
	data := &model.DescriptionData{Statement: "test"}
	config := model.SiteDescConfig{Format: "markdown"}

	result, err := r.Render(data, config)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "## 声明") {
		t.Error("config format should override renderer default")
	}
}

func TestRenderer_TemplateOverride(t *testing.T) {
	r := NewRenderer("bbcode")
	data := &model.DescriptionData{
		Statement:     "no repost",
		PosterURL:     "https://example.com/p.jpg",
		PTGenBody:     "ptgen content",
		MediaInfoText: "mediainfo",
		SourceSite:    "site1",
	}
	config := model.SiteDescConfig{
		Format:           "bbcode",
		TemplateOverride: "Custom: {{statement}}\nPoster: {{poster}}\nFrom: {{source_site}}",
	}

	result, err := r.Render(data, config)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Custom: no repost") {
		t.Errorf("template not applied: %s", result)
	}
	if !strings.Contains(result, "From: site1") {
		t.Errorf("template source_site not replaced: %s", result)
	}
}

func TestFormatMediaInfo(t *testing.T) {
	r := NewRenderer("bbcode")

	result := r.FormatMediaInfo("  General\nTitle  ", model.MediaInfoFormatBBCode)
	if !strings.Contains(result, "[code]General") {
		t.Errorf("expected bbcode code wrapper, got %s", result)
	}

	result = r.FormatMediaInfo("", model.MediaInfoFormatBBCode)
	if result != "" {
		t.Errorf("expected empty for empty input, got %s", result)
	}

	result = r.FormatMediaInfo("info", model.MediaInfoFormatMarkdown)
	if !strings.Contains(result, "```") {
		t.Errorf("expected markdown code block, got %s", result)
	}
}

func TestFormatScreenshots(t *testing.T) {
	r := NewRenderer("bbcode")

	result := r.FormatScreenshots([]string{"https://a.com/1.jpg", "https://a.com/2.jpg"})
	if !strings.Contains(result, "[img]https://a.com/1.jpg[/img]") {
		t.Errorf("expected screenshot images, got %s", result)
	}
	if !strings.Contains(result, "[img]https://a.com/2.jpg[/img]") {
		t.Errorf("expected second screenshot, got %s", result)
	}

	result = r.FormatScreenshots(nil)
	if result != "" {
		t.Errorf("expected empty for nil, got %s", result)
	}

	result = r.FormatScreenshots([]string{"", "https://a.com/1.jpg"})
	if strings.Contains(result, "[img][/img]") {
		t.Error("should skip empty URLs")
	}
}

func TestRenderer_Poster(t *testing.T) {
	r := NewRenderer("bbcode")
	data := &model.DescriptionData{PosterURL: "https://example.com/p.jpg"}
	config := model.SiteDescConfig{Format: "bbcode"}

	result, err := r.Render(data, config)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "[img]https://example.com/p.jpg[/img]") {
		t.Error("missing bbcode poster")
	}
}

func TestRenderer_BDInfo(t *testing.T) {
	r := NewRenderer("bbcode")
	data := &model.DescriptionData{BDInfoText: "disc info here"}
	config := model.SiteDescConfig{Format: "bbcode"}

	result, err := r.Render(data, config)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "[b]BDInfo[/b]") {
		t.Error("missing BDInfo section")
	}
	if !strings.Contains(result, "[code]disc info here[/code]") {
		t.Error("missing BDInfo code wrapper")
	}
}

func TestRenderer_EmptySections(t *testing.T) {
	r := NewRenderer("bbcode")
	data := &model.DescriptionData{Statement: "only statement"}
	config := model.SiteDescConfig{Format: "bbcode"}

	result, err := r.Render(data, config)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result, "MediaInfo") {
		t.Error("should not have MediaInfo section when empty")
	}
	if strings.Contains(result, "截图") {
		t.Error("should not have screenshots section when empty")
	}
}

func TestFormat(t *testing.T) {
	r := NewRenderer("markdown")
	if r.Format() != "markdown" {
		t.Errorf("expected markdown, got %s", r.Format())
	}
}
