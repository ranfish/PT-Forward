package description

import (
	"strings"
	"testing"
)

// ===== thanks_quote.go 测试 =====

func TestGenerateThanksQuote_DefaultChinese(t *testing.T) {
	result := GenerateThanksQuote("猫站", "FRDS", false, nil)
	if strings.Contains(result, "猫站") || strings.Contains(result, "转自") {
		t.Errorf("§59.68 中文模板不应披露转载来源, got %q", result)
	}
	if !strings.Contains(result, "FRDS") {
		t.Errorf("should contain group name, got %q", result)
	}
	if !strings.Contains(result, "感谢原制作者发布") {
		t.Errorf("should contain default chinese template, got %q", result)
	}
}

func TestGenerateThanksQuote_DefaultEnglish(t *testing.T) {
	result := GenerateThanksQuote("HDHome", "FRDS", true, nil)
	if strings.Contains(result, "HDHome") || strings.Contains(result, "from [b]") {
		t.Errorf("§59.68 英文模板不应披露转载来源, got %q", result)
	}
	if !strings.Contains(result, "FRDS") {
		t.Errorf("should contain group name, got %q", result)
	}
	if !strings.Contains(result, "original uploader") {
		t.Errorf("should contain default english template, got %q", result)
	}
}

func TestGenerateThanksQuote_Disabled(t *testing.T) {
	config := &ThanksQuoteConfig{Enabled: false}
	if result := GenerateThanksQuote("猫站", "FRDS", false, config); result != "" {
		t.Errorf("disabled should return empty, got %q", result)
	}
}

func TestGenerateThanksQuote_CustomTemplate(t *testing.T) {
	config := &ThanksQuoteConfig{
		Enabled:         true,
		ChineseTemplate: "来自 {source_site}",
	}
	result := GenerateThanksQuote("猫站", "FRDS", false, config)
	if result != "来自 猫站" {
		t.Errorf("custom template mismatch, got %q", result)
	}
}

func TestGenerateThanksQuote_SiteOverride(t *testing.T) {
	config := &ThanksQuoteConfig{
		Enabled:      true,
		SiteTemplate: "站点专属: {source_site}",
	}
	result := GenerateThanksQuote("猫站", "FRDS", false, config)
	if result != "站点专属: 猫站" {
		t.Errorf("site override mismatch, got %q", result)
	}
}

func TestGenerateThanksQuote_EmptySourceSite(t *testing.T) {
	result := GenerateThanksQuote("", "FRDS", false, nil)
	// 占位符被替换为空，但仍含模板其他内容
	if !strings.Contains(result, "感谢原制作者发布") {
		t.Errorf("should still contain template, got %q", result)
	}
}

// ===== bbcode_to_html.go 测试 =====

func TestBBCodeToHTML_Empty(t *testing.T) {
	if result := BBCodeToHTML(""); result != "" {
		t.Errorf("empty input → empty output, got %q", result)
	}
}

func TestBBCodeToHTML_Bold(t *testing.T) {
	result := BBCodeToHTML("[b]bold[/b]")
	if !strings.Contains(result, "<strong>bold</strong>") {
		t.Errorf("bold mismatch: %q", result)
	}
}

func TestBBCodeToHTML_Italic(t *testing.T) {
	result := BBCodeToHTML("[i]italic[/i]")
	if !strings.Contains(result, "<em>italic</em>") {
		t.Errorf("italic mismatch: %q", result)
	}
}

func TestBBCodeToHTML_Image(t *testing.T) {
	result := BBCodeToHTML("[img]https://example.com/x.jpg[/img]")
	if !strings.Contains(result, `<img src="https://example.com/x.jpg"/>`) {
		t.Errorf("image mismatch: %q", result)
	}
}

func TestBBCodeToHTML_URL(t *testing.T) {
	result := BBCodeToHTML("[url=https://example.com]链接[/url]")
	if !strings.Contains(result, `<a href="https://example.com">链接</a>`) {
		t.Errorf("url mismatch: %q", result)
	}
}

func TestBBCodeToHTML_Quote(t *testing.T) {
	result := BBCodeToHTML("[quote]引用内容[/quote]")
	if !strings.Contains(result, "<blockquote>引用内容</blockquote>") {
		t.Errorf("quote mismatch: %q", result)
	}
}

func TestBBCodeToHTML_List(t *testing.T) {
	result := BBCodeToHTML("[list][*]项目一[*]项目二[/list]")
	if !strings.Contains(result, "<ul>") || !strings.Contains(result, "<li>项目一</li>") {
		t.Errorf("list mismatch: %q", result)
	}
}

func TestBBCodeToHTML_Code(t *testing.T) {
	result := BBCodeToHTML("[code]code block[/code]")
	if !strings.Contains(result, "<pre><code>code block</code></pre>") {
		t.Errorf("code mismatch: %q", result)
	}
}

func TestBBCodeToHTML_HR(t *testing.T) {
	result := BBCodeToHTML("before[hr]after")
	if !strings.Contains(result, "<hr/>") {
		t.Errorf("hr mismatch: %q", result)
	}
}

func TestBBCodeToHTML_ColorStripped(t *testing.T) {
	result := BBCodeToHTML("[color=red]红色文字[/color]")
	if strings.Contains(result, "[color=") {
		t.Errorf("color tag should be stripped: %q", result)
	}
	if !strings.Contains(result, "红色文字") {
		t.Errorf("content should be kept: %q", result)
	}
}

func TestBBCodeToHTML_Center(t *testing.T) {
	result := BBCodeToHTML("[center]居中[/center]")
	if !strings.Contains(result, `text-align:center`) {
		t.Errorf("center mismatch: %q", result)
	}
}

func TestBBCodeToHTML_Newline(t *testing.T) {
	result := BBCodeToHTML("line1\nline2")
	if !strings.Contains(result, "<br/>") {
		t.Errorf("newline should convert to <br/>: %q", result)
	}
}

func TestBBCodeToHTML_Spoiler(t *testing.T) {
	result := BBCodeToHTML("[spoiler]隐藏内容[/spoiler]")
	if !strings.Contains(result, "<details>") {
		t.Errorf("spoiler mismatch: %q", result)
	}
}

func TestBBCodeToHTML_Table(t *testing.T) {
	result := BBCodeToHTML("[table][tr][td]cell[/td][/tr][/table]")
	if !strings.Contains(result, "<table>") || !strings.Contains(result, "<td>cell</td>") {
		t.Errorf("table mismatch: %q", result)
	}
}
