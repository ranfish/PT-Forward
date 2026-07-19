package metadata

import (
	"testing"
)

func TestPreviewBuilder_Complete(t *testing.T) {
	merged := &MergedMetadata{
		Title:        "测试电影 2024",
		Subtitle:     "副标题",
		Type:         "电影",
		Medium:       "Blu-ray",
		VideoCodec:   "H.265",
		AudioCodec:   "DTS-HD MA",
		Resolution:   "2160p",
		ReleaseGroup: "FRDS",
		IMDbURL:      "https://imdb.com/x",
		DoubanURL:    "https://douban.com/x",
		Tags:         []string{"动作", "喜剧"},
		SourceOf: map[string]string{
			"title":      "ptgen",
			"subtitle":   "detail",
			"resolution": "detail",
			"imdb_url":   "ptgen",
		},
	}
	merged.Intro.Body = "简介正文"
	merged.Intro.SetScreenshotURLs([]string{"shot1.jpg", "shot2.jpg", "shot3.jpg"})

	builder := NewPreviewBuilder()
	resp := builder.Build(merged, nil, "PTer", "ptgen_first")

	if resp.TargetSite != "PTer" {
		t.Errorf("target site mismatch: %q", resp.TargetSite)
	}
	if len(resp.Fields) == 0 {
		t.Fatal("should have fields")
	}

	// 检查标题字段
	var titleField *PreviewFieldDummy
	for i := range resp.Fields {
		if resp.Fields[i].StandardKey == "title" {
			titleField = &PreviewFieldDummy{
				Value:    resp.Fields[i].TargetValue,
				Source:   resp.Fields[i].Source,
				Required: resp.Fields[i].Required,
			}
			break
		}
	}
	if titleField == nil {
		t.Fatal("missing title field")
	}
	if titleField.Value != "测试电影 2024" {
		t.Errorf("title value mismatch: %q", titleField.Value)
	}
	if titleField.Source != "ptgen" {
		t.Errorf("title source mismatch: %q", titleField.Source)
	}
	if !titleField.Required {
		t.Error("title should be required")
	}

	// 检查完整度
	if resp.Completeness.Total == 0 {
		t.Error("completeness total should not be 0")
	}
	if resp.Completeness.Percent == 0 {
		t.Error("completeness percent should not be 0")
	}
}

// PreviewFieldDummy 测试辅助结构。
type PreviewFieldDummy struct {
	Value    string
	Source   string
	Required bool
}

func TestPreviewBuilder_Nil(t *testing.T) {
	builder := NewPreviewBuilder()
	resp := builder.Build(nil, nil, "test", "ptgen_first")
	if resp.TargetSite != "test" {
		t.Errorf("target site mismatch")
	}
	if len(resp.Fields) != 0 {
		t.Errorf("nil merged: should have no fields")
	}
}

func TestPreviewBuilder_UserOverride(t *testing.T) {
	merged := &MergedMetadata{
		Title:  "自动标题",
		SourceOf: map[string]string{"title": "ptgen"},
	}
	builder := NewPreviewBuilder()
	resp := builder.Build(merged, map[string]string{
		"title": "用户修改标题",
	}, "test", "ptgen_first")

	for _, f := range resp.Fields {
		if f.StandardKey == "title" {
			if f.TargetValue != "用户修改标题" {
				t.Errorf("user override mismatch: %q", f.TargetValue)
			}
			if f.SourceValue != "自动标题" {
				t.Errorf("source value mismatch: %q", f.SourceValue)
			}
			if !f.IsUserEdited {
				t.Error("should be marked as user edited")
			}
			if f.Source != "user" {
				t.Errorf("source should be 'user', got %q", f.Source)
			}
			return
		}
	}
	t.Fatal("title field not found")
}

func TestPreviewBuilder_MissingRequired(t *testing.T) {
	merged := &MergedMetadata{
		Title: "有标题",
		// 分辨率为空（必填）
		SourceOf: map[string]string{"title": "ptgen"},
	}
	builder := NewPreviewBuilder()
	resp := builder.Build(merged, nil, "test", "ptgen_first")

	for _, f := range resp.Fields {
		if f.StandardKey == "resolution" {
			if !f.Required {
				t.Error("resolution should be required")
			}
			if !f.Missing {
				t.Error("empty resolution should be missing")
			}
			return
		}
	}
	t.Fatal("resolution field not found")
}

func TestPreviewBuilder_Completeness(t *testing.T) {
	merged := &MergedMetadata{
		Title:      "有值",
		Resolution: "2160p",
		SourceOf:   map[string]string{},
	}
	builder := NewPreviewBuilder()
	resp := builder.Build(merged, nil, "test", "ptgen_first")

	if resp.Completeness.Filled == 0 {
		t.Error("should have filled fields")
	}
	if resp.Completeness.Missing == 0 {
		t.Error("should have missing required fields")
	}
}

func TestPreviewBuilder_ScreenshotCount(t *testing.T) {
	merged := &MergedMetadata{
		SourceOf: map[string]string{},
	}
	merged.Intro.SetScreenshotURLs([]string{"a", "b"})
	builder := NewPreviewBuilder()
	resp := builder.Build(merged, nil, "test", "ptgen_first")

	for _, f := range resp.Fields {
		if f.StandardKey == "screenshots" {
			if f.TargetValue == "" {
				t.Error("screenshot count should not be empty")
			}
			return
		}
	}
}

func TestTruncateForPreview(t *testing.T) {
	short := "短文本"
	if got := truncateForPreview(short, 200); got != short {
		t.Errorf("short text should not be truncated, got %q", got)
	}

	long := string(make([]rune, 300))
	for i := range long {
		long = long[:i] + "x" + long[i+1:]
	}
	got := truncateForPreview(long, 100)
	if !endsWith(got, "...") {
		t.Errorf("long text should end with '...', got %q", got[len(got)-10:])
	}
}

func endsWith(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}
