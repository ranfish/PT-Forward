package extract

import (
	"errors"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

const sampleHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head><meta charset="UTF-8"><title>测试种子页</title></head>
<body>
<div id="main">
  <table class="torrent-info">
    <tr><td>标题</td><td>测试电影 2024 4K UHD Blu-ray HDR10</td></tr>
  </table>
  <div id="kdescr" class="torrent-description">
    <p>简介正文 <a href="https://www.imdb.com/title/tt1234567/">IMDb</a></p>
    <img src="https://example.com/poster.jpg" alt="海报"/>
    <div class="codemain"><pre><code>General
Complete name                : test.mkv
Format                       : Matroska</code></pre></div>
    <img src="https://example.com/shot1.jpg"/>
    <img src="https://example.com/shot2.jpg"/>
  </div>
</div>
</body>
</html>`

func TestParseHTML_Success(t *testing.T) {
	doc, err := ParseHTML(sampleHTML)
	if err != nil {
		t.Fatalf("ParseHTML failed: %v", err)
	}
	if doc == nil {
		t.Fatal("doc is nil")
	}
	// 验证 CSS selector 可用
	title := doc.Find("table.torrent-info td").Last().Text()
	if !strings.Contains(title, "测试电影") {
		t.Errorf("title not matched, got: %q", title)
	}
}

func TestParseHTML_DescriptionContainer(t *testing.T) {
	doc, err := ParseHTML(sampleHTML)
	if err != nil {
		t.Fatalf("ParseHTML failed: %v", err)
	}
	desc := doc.Find("#kdescr")
	if desc.Length() == 0 {
		t.Fatal("#kdescr not found")
	}

	// 验证 IMDb 链接提取（goquery selector 主路径）
	imdbURL, exists := doc.Find(`a[href*="imdb.com"]`).First().Attr("href")
	if !exists {
		t.Fatal("IMDb link not found")
	}
	if !strings.Contains(imdbURL, "tt1234567") {
		t.Errorf("IMDb ID not matched, got: %q", imdbURL)
	}

	// 验证图片遍历
	var imgSrcs []string
	doc.Find("#kdescr img").Each(func(_ int, s *goquery.Selection) {
		if src, ok := s.Attr("src"); ok {
			imgSrcs = append(imgSrcs, src)
		}
	})
	if len(imgSrcs) != 3 {
		t.Errorf("expected 3 images, got %d: %v", len(imgSrcs), imgSrcs)
	}

	// 验证 MediaInfo 容器
	miText := doc.Find("div.codemain pre code").Text()
	if !strings.Contains(miText, "Matroska") {
		t.Errorf("MediaInfo not extracted, got: %q", miText)
	}
}

func TestParseHTML_EmptyInput(t *testing.T) {
	cases := []string{"", "   ", "\n\n\t"}
	for _, c := range cases {
		doc, err := ParseHTML(c)
		if !errors.Is(err, ErrEmptyHTML) {
			t.Errorf("input %q expected ErrEmptyHTML, got err=%v doc=%v", c, err, doc)
		}
	}
}

func TestParseHTMLFromReader_Nil(t *testing.T) {
	doc, err := ParseHTMLFromReader(nil)
	if err == nil {
		t.Errorf("expected error for nil reader, got doc=%v", doc)
	}
}

func TestParseHTMLFromReader_Success(t *testing.T) {
	doc, err := ParseHTMLFromReader(strings.NewReader(sampleHTML))
	if err != nil {
		t.Fatalf("ParseHTMLFromReader failed: %v", err)
	}
	if doc.Find("body").Length() == 0 {
		t.Error("body not found from reader")
	}
}

func TestMustParseHTML_Success(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("should not panic, got: %v", r)
		}
	}()
	doc := MustParseHTML(sampleHTML)
	if doc == nil {
		t.Fatal("doc is nil")
	}
}

func TestMustParseHTML_PanicOnEmpty(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on empty input")
		}
	}()
	_ = MustParseHTML("")
}

func TestParseHTML_MalformedHTML(t *testing.T) {
	// goquery/golang.org/x/net/html 容错性强，残缺 HTML 不应返回错误
	malformed := `<div><p>未闭合段落<h1>标题`
	doc, err := ParseHTML(malformed)
	if err != nil {
		t.Fatalf("malformed HTML should not error: %v", err)
	}
	if !strings.Contains(doc.Text(), "标题") {
		t.Errorf("expected to find '标题', got: %q", doc.Text())
	}
}
