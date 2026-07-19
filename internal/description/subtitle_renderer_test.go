package description

import (
	"strings"
	"testing"
)

func TestRenderSubtitle_EmptyTemplate(t *testing.T) {
	data := SubtitleData{SourceSubtitle: "源站副标题"}
	if result := RenderSubtitle("", data); result != "源站副标题" {
		t.Errorf("empty template should return source subtitle, got %q", result)
	}
}

func TestRenderSubtitle_AllVariables(t *testing.T) {
	template := "{ptgen.chinese_title} / {ptgen.foreign_title} ({ptgen.year})"
	data := SubtitleData{
		PTGenChineseTitle: "测试电影",
		PTGenForeignTitle: "Test Movie",
		PTGenYear:         "2024",
	}
	result := RenderSubtitle(template, data)
	if result != "测试电影 / Test Movie (2024)" {
		t.Errorf("all variables mismatch, got %q", result)
	}
}

func TestRenderSubtitle_PartialVariables(t *testing.T) {
	template := "{ptgen.foreign_title} {ptgen.year}"
	data := SubtitleData{
		PTGenForeignTitle: "Test Movie",
		PTGenYear:         "2024",
	}
	result := RenderSubtitle(template, data)
	if result != "Test Movie 2024" {
		t.Errorf("partial variables mismatch, got %q", result)
	}
}

func TestRenderSubtitle_EmptyVariable(t *testing.T) {
	template := "{ptgen.chinese_title} {ptgen.foreign_title}"
	data := SubtitleData{
		PTGenForeignTitle: "Test Movie",
		// ChineseTitle 为空
	}
	result := RenderSubtitle(template, data)
	// 空变量替换为空字符串，可能有多余空格
	if !strings.Contains(result, "Test Movie") {
		t.Errorf("should contain foreign title, got %q", result)
	}
}

func TestRenderSubtitle_SourceSubtitle(t *testing.T) {
	template := "{source_subtitle}"
	data := SubtitleData{SourceSubtitle: "源站原始副标题"}
	if result := RenderSubtitle(template, data); result != "源站原始副标题" {
		t.Errorf("source subtitle mismatch, got %q", result)
	}
}

func TestRenderSubtitle_TitleMain(t *testing.T) {
	template := "{title.main} [{ptgen.year}]"
	data := SubtitleData{
		TitleMain: "Main.Title",
		PTGenYear: "2024",
	}
	result := RenderSubtitle(template, data)
	if result != "Main.Title [2024]" {
		t.Errorf("title main mismatch, got %q", result)
	}
}

func TestRenderSubtitle_DefaultTemplate(t *testing.T) {
	data := SubtitleData{
		PTGenChineseTitle: "中文",
		PTGenForeignTitle: "English",
		PTGenYear:         "2024",
	}
	result := RenderSubtitle(DefaultSubtitleTemplate, data)
	if result != "中文 English 2024" {
		t.Errorf("default template mismatch, got %q", result)
	}
}

func TestRenderSubtitle_TrimSpace(t *testing.T) {
	template := "  {ptgen.year}  "
	data := SubtitleData{PTGenYear: "2024"}
	if result := RenderSubtitle(template, data); result != "2024" {
		t.Errorf("should trim spaces, got %q", result)
	}
}
