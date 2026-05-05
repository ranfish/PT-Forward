package description

import "testing"

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
