package adapter

import (
	"os"
	"strings"
	"testing"
)

func loadTestHTML(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Logf("skip: cannot read testdata/%s: %v", name, err)
		return ""
	}
	return string(data)
}

func TestExtractOpenCDDetail(t *testing.T) {
	html := loadTestHTML(t, "open.cd.html")
	if html == "" {
		t.Skip("no test data")
	}

	detail := extractOpenCDDetail(html)
	if detail == nil {
		t.Fatal("detail is nil")
	}

	// 标题
	if !strings.Contains(detail.Title, "Paper Towns") {
		t.Errorf("Title = %q, should contain 'Paper Towns'", detail.Title)
	}
	if strings.Contains(detail.Title, "<img") {
		t.Errorf("Title = %q, should not contain HTML tags", detail.Title)
	}

	// 副标题
	if !strings.Contains(detail.Subtitle, "纸镇") {
		t.Errorf("Subtitle = %q, should contain '纸镇'", detail.Subtitle)
	}

	// 简介含封面 + 介绍 + 曲目 + LOG
	if !strings.Contains(detail.Description, "[img]") {
		t.Error("Description should contain cover [img]")
	}
	if !strings.Contains(detail.Description, "Quentin Jacobsen") {
		t.Error("Description should contain album intro text")
	}
	if !strings.Contains(detail.Description, "Santigold") {
		t.Error("Description should contain tracklist (Santigold)")
	}
	if !strings.Contains(detail.Description, "[hide=Log]") {
		t.Error("Description should contain EAC log in [hide=Log]")
	}

	// 字段提取
	if detail.Category == "" {
		t.Error("Category should not be empty")
	}
	if detail.Source == "" {
		t.Error("Source should not be empty")
	}

	// 封面/海报
	if len(detail.Screenshots) == 0 {
		t.Error("Screenshots should not be empty")
	}
}

func TestExtractOpenCDFields(t *testing.T) {
	htmlStr := `<table>
		<td class="rowtitle" style="width:30%;">专辑名称：</td><td>Test Album</td>
		<td class="rowtitle">艺术家名：</td><td><a>Test Artist</a></td>
		<td class="rowtitle">媒介：</td><td>CD</td>
		<td class="rowtitle">类型：</td><td>音乐(Music)</td>
	</table>`

	fields := extractOpenCDFields(htmlStr)
	if fields["专辑名称"] != "Test Album" {
		t.Errorf("专辑名称 = %q, want 'Test Album'", fields["专辑名称"])
	}
	if fields["媒介"] != "CD" {
		t.Errorf("媒介 = %q, want 'CD'", fields["媒介"])
	}
	if fields["类型"] != "音乐(Music)" {
		t.Errorf("类型 = %q, want '音乐(Music)'", fields["类型"])
	}
}

func TestCleanOpenCDText(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Title <img class=\"pro_free\" src=\"pic/trans.gif\" alt=\"Free\" />", "Title"},
		{"Line1<br />Line2", "Line1\nLine2"},
		{"<b>Bold</b> text", "Bold text"},
		{"&amp; &lt; &gt;", "& < >"},
	}
	for _, tt := range tests {
		got := cleanOpenCDText(tt.input)
		if got != tt.want {
			t.Errorf("cleanOpenCDText(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
