package description

import (
	"strings"
	"testing"
)

func TestBBCodeToMarkdown_Bold(t *testing.T) {
	got := BBCodeToMarkdown("[b]hello[/b]")
	want := "**hello**"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBBCodeToMarkdown_Italic(t *testing.T) {
	got := BBCodeToMarkdown("[i]world[/i]")
	want := "*world*"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBBCodeToMarkdown_Image(t *testing.T) {
	got := BBCodeToMarkdown("[img]https://example.com/img.png[/img]")
	want := "![](https://example.com/img.png)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBBCodeToMarkdown_URL(t *testing.T) {
	got := BBCodeToMarkdown("[url=https://example.com]Click[/url]")
	want := "[Click](https://example.com)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBBCodeToMarkdown_Code(t *testing.T) {
	got := BBCodeToMarkdown("[code]hello world[/code]")
	want := "```\nhello world\n```"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBBCodeToMarkdown_Color(t *testing.T) {
	got := BBCodeToMarkdown("[color=red]text[/color]")
	want := "text"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBBCodeToMarkdown_Size(t *testing.T) {
	got := BBCodeToMarkdown("[size=14px]text[/size]")
	want := "text"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBBCodeToMarkdown_Quote(t *testing.T) {
	got := BBCodeToMarkdown("[quote]some text[/quote]")
	want := "> some text\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBBCodeToMarkdown_Complex(t *testing.T) {
	input := `[b]Title[/b]
[img]https://example.com/poster.jpg[/img]
[color=blue][size=4]Info[/size][/color]
[url=https://example.com]Link[/url]`

	got := BBCodeToMarkdown(input)
	tests := []struct{ substr, missing string }{
		{"**Title**", "bold"},
		{"![](https://example.com/poster.jpg)", "image"},
		{"Info", "stripped color/size"},
		{"[Link](https://example.com)", "url"},
	}
	for _, tt := range tests {
		if !contains(got, tt.substr) {
			t.Errorf("missing %s in output: %q", tt.missing, got)
		}
	}
}

func TestBBCodeToMarkdown_Empty(t *testing.T) {
	got := BBCodeToMarkdown("")
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestBBCodeToMarkdown_NoBBCode(t *testing.T) {
	input := "plain text without any bbcode"
	got := BBCodeToMarkdown(input)
	if got != input {
		t.Errorf("expected unchanged, got %q", got)
	}
}

func TestBBCodeToMarkdown_Spoiler(t *testing.T) {
	got := BBCodeToMarkdown("[spoiler=hint]hidden text[/spoiler]")
	want := "<details>hidden text</details>"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBBCodeToMarkdown_Heading(t *testing.T) {
	got := BBCodeToMarkdown("[h3]Section[/h3]")
	want := "### Section"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && findSubstr(s, substr))
}

func findSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// §59.541 附三: 嵌套+多行 quote——Obsession 首发实战 422 形态（243 实拉
// statement 剥壳重包后的组装形态）。单遍非贪婪必残留内层 [quote]+落单
// [/quote]；且多行块在无 (?s) 模式下整个不转换——站方 MD 检测器 422。
func TestBBCodeToMarkdown_NestedMultilineQuote(t *testing.T) {
	in := "[quote]\nRemux来自 X 1964 2160p UHD\n[quote]\n[b]Source #1:[/b] Criterion Collection - [i]thanks TMT![/i]\n[/quote]\n双语特效字幕基于 fix字幕侠 版本制作\n[/quote]\n\n◎片名 痴迷"
	out := BBCodeToMarkdown(in)
	for _, bad := range []string{"[quote", "[/quote", "[b]", "[/b]", "[i]", "[/i]"} {
		if strings.Contains(out, bad) {
			t.Errorf("残留 BBCode %q: %q", bad, out)
		}
	}
	if !strings.Contains(out, "> ") {
		t.Errorf("缺 blockquote 前缀: %q", out)
	}
	if !strings.Contains(out, "fix字幕侠") {
		t.Errorf("内容丢失: %q", out)
	}
}

// §59.541 附三: 多行 b/color/size——原模式无 (?s) 多行块整个不转换（同族
// 第二漏点·声明区 [b][color][size] 组合跨行形态）。
func TestBBCodeToMarkdown_MultilineSpans(t *testing.T) {
	out := BBCodeToMarkdown("[b]line1\nline2[/b]")
	if strings.Contains(out, "[b]") || strings.Contains(out, "[/b]") {
		t.Errorf("多行 bold 残留: %q", out)
	}
	if !strings.Contains(out, "**line1") {
		t.Errorf("bold 未转换: %q", out)
	}
	out2 := BBCodeToMarkdown("[color=#ff0000][size=4]转载自HDChina\n感谢分享[/size][/color]")
	for _, bad := range []string{"[color", "[/color", "[size", "[/size"} {
		if strings.Contains(out2, bad) {
			t.Errorf("多行 color/size 残留 %q: %q", bad, out2)
		}
	}
}

// §59.541 附三: 落单/畸形 quote token 清扫兜底（MD 站任何残留=422）。
func TestBBCodeToMarkdown_OrphanQuoteSweep(t *testing.T) {
	out := BBCodeToMarkdown("前文\n[quote]没有闭合标签\n后文")
	if strings.Contains(out, "[quote") || strings.Contains(out, "[/quote") {
		t.Errorf("畸形 quote 残留: %q", out)
	}
	if !strings.Contains(out, "没有闭合标签") {
		t.Errorf("内容丢失: %q", out)
	}
}
