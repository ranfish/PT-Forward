package extract

import (
	"strings"
	"testing"
)

func TestNormalizeImageURL(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"https://Example.com/Poster.JPG", "https://example.com/poster.jpg"},
		{"https://example.com/img.jpg?q=123", "https://example.com/img.jpg"},
		{"https://example.com/img.jpg#frag", "https://example.com/img.jpg"},
		{"https://example.com/path/", "https://example.com/path"},
		{"https://example.com/path//", "https://example.com/path"},
		{"  https://example.com/x.jpg  ", "https://example.com/x.jpg"},
		{"", ""},
	}
	for _, c := range cases {
		got := NormalizeImageURL(c.in)
		if got != c.want {
			t.Errorf("NormalizeImageURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestIsUnwantedImage(t *testing.T) {
	cases := []struct {
		url  string
		want bool
	}{
		// substring 匹配
		{"https://example.com/logo.png", true},
		{"https://example.com/icon.png", true},
		{"https://example.com/banner.gif", true},
		{"https://example.com/rank_1.png", true},
		{"https://example.com/emoji_smile.png", true},
		{"https://example.com/poster.jpg", false},
		{"https://example.com/shot1.jpg", false},
		// host 匹配
		{"https://twemoji.com/abc.png", true},
		{"https://cdn.twemoji.com/xyz.png", true},
		{"https://emoji-cdn.com/abc.png", true},
		// 大小写不敏感
		{"https://example.com/LOGO.PNG", true},
		{"https://example.com/Poster.JPG", false},
		// 空
		{"", false},
	}
	for _, c := range cases {
		got := IsUnwantedImage(c.url)
		if got != c.want {
			t.Errorf("IsUnwantedImage(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestIsWhitelistedImage(t *testing.T) {
	cases := []struct {
		url  string
		want bool
	}{
		{"https://pixhost.to/show/abc.jpg", true},
		{"https://pixhost.cc/show/abc.jpg", true},
		{"https://img2.pixhost.cc/images/123/test.png", true},
		{"https://img9.doubanio.com/poster.jpg", true},
		{"https://image.tmdb.org/poster.jpg", true},
		{"https://m.media-amazon.com/poster.jpg", true},
		{"https://example.com/img.jpg", false},
		{"", false},
	}
	for _, c := range cases {
		got := IsWhitelistedImage(c.url)
		if got != c.want {
			t.Errorf("IsWhitelistedImage(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestIsWhitelistedImage_OverridesBlacklist(t *testing.T) {
	// 白名单优先于黑名单
	// pixhost.to 在白名单，即使 URL 含 "logo" 也保留
	url := "https://pixhost.to/logo-test.jpg"
	if !IsWhitelistedImage(url) {
		t.Errorf("should be whitelisted: %q", url)
	}
	// IsUnwantedImage 也会命中（"logo"），但调用方应先检查白名单
	if !IsUnwantedImage(url) {
		t.Errorf("should also hit blacklist: %q", url)
	}
}

func TestExtractImages_FromHTML(t *testing.T) {
	descrHTML := `<div>
<img src="https://example.com/logo.png"/>
<img src="https://example.com/poster.jpg"/>
<img src="https://example.com/shot1.jpg"/>
<img src="https://example.com/shot2.jpg"/>
</div>`
	urls := ExtractImages(descrHTML, "")
	// logo 被黑名单过滤
	if len(urls) != 3 {
		t.Errorf("expected 3 images (logo filtered), got %d: %v", len(urls), urls)
	}
}

func TestExtractImages_FromBBCode(t *testing.T) {
	descrBBCode := `[img]https://example.com/poster.jpg[/img]
[img]https://example.com/shot1.jpg[/img]`
	urls := ExtractImages("", descrBBCode)
	if len(urls) != 2 {
		t.Errorf("expected 2 images, got %d: %v", len(urls), urls)
	}
}

func TestExtractImages_Dedup(t *testing.T) {
	descrHTML := `<img src="https://example.com/x.jpg"/>
<img src="https://example.com/x.jpg"/>
<img src="https://example.com/x.jpg?v=1"/>
<img src="https://example.com/y.jpg"/>`
	urls := ExtractImages(descrHTML, "")
	// 归一化后 x.jpg 和 x.jpg?v=1 相同 → 去重
	if len(urls) != 2 {
		t.Errorf("expected 2 unique images, got %d: %v", len(urls), urls)
	}
}

func TestExtractImages_DataSrcLazyLoad(t *testing.T) {
	descrHTML := `<img data-src="https://example.com/lazy.jpg" src="placeholder.gif"/>
<img data-original="https://example.com/orig.jpg"/>`
	urls := ExtractImages(descrHTML, "")
	// data-src 优先于 src（placeholder.gif 被忽略）
	found := false
	for _, u := range urls {
		if strings.Contains(u, "lazy.jpg") {
			found = true
		}
	}
	if !found {
		t.Errorf("data-src lazy load URL not extracted: %v", urls)
	}
}

func TestExtractImages_EmojiSkipped(t *testing.T) {
	descrHTML := `<img alt=":smile:" src="https://example.com/emoji.png"/>
<img src="https://example.com/real.jpg"/>`
	urls := ExtractImages(descrHTML, "")
	// emoji 图片（alt 含 :xxx:）被跳过
	if len(urls) != 1 {
		t.Errorf("expected 1 image (emoji skipped), got %d: %v", len(urls), urls)
	}
	if !strings.Contains(urls[0], "real.jpg") {
		t.Errorf("expected real.jpg, got %v", urls)
	}
}

func TestExtractImages_Empty(t *testing.T) {
	urls := ExtractImages("", "")
	if len(urls) != 0 {
		t.Errorf("expected empty, got %v", urls)
	}
}

func TestExtractImages_NormalizationApplied(t *testing.T) {
	descrHTML := `<img src="HTTPS://Example.COM/Poster.JPG"/>`
	urls := ExtractImages(descrHTML, "")
	if len(urls) != 1 {
		t.Fatalf("expected 1 image, got %d", len(urls))
	}
	if urls[0] != "https://example.com/poster.jpg" {
		t.Errorf("URL not normalized, got %q", urls[0])
	}
}
