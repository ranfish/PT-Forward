package extract

import (
	"strings"
	"testing"
)

func TestIsToolSignatureQuote(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{"CSAUTO 压制", true},
		{"Encoder by FRDS", true},
		{"Ripped by xxx", true},
		{"NEXUS 发布器生成", true},
		{"感谢 ARDTU 工具压制", true}, // 含 ARDTU
		// 声明文本不再归工具签名（由 IsAcknowledgmentQuote 处理为 Statement）
		{"郑重声明：本资源仅供...", false},
		{"感谢原制作者", false},
		{"普通正文内容", false},
		{"这是一段简介", false},
		{"", false},
	}
	for _, c := range cases {
		got := IsToolSignatureQuote(c.text)
		if got != c.want {
			t.Errorf("IsToolSignatureQuote(%q) = %v, want %v", c.text, got, c.want)
		}
	}
}

func TestIsTechParamsQuote(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{"Format: Matroska\nBit rate: 8000\nWriting library: x264", true},
		{"Unique ID: 123\nComplete name: movie.mkv\nFile size: 8 GiB", true},
		{"Format profile: High\nCodec ID: V_MPEGH", true},
		{"普通文本不含技术参数", false},
		{"Format only", false}, // 只有一个关键字
		{"", false},
	}
	for _, c := range cases {
		got := IsTechParamsQuote(c.text)
		if got != c.want {
			t.Errorf("IsTechParamsQuote(%q) = %v, want %v", c.text, got, c.want)
		}
	}
}

func TestIsByARDTUWithGroupInfo(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{"By ARDTU@FRDS: 正文内容", true},
		{"by ARDTU@PTer", true},
		{"BY ARDTU 正文", true},
		{"By ARDTU", true},
		{"普通文本 By ARDTU", false}, // 不在开头
		{"", false},
	}
	for _, c := range cases {
		got := IsByARDTUWithGroupInfo(c.text)
		if got != c.want {
			t.Errorf("IsByARDTUWithGroupInfo(%q) = %v, want %v", c.text, got, c.want)
		}
	}
}

func TestStripByARDTUPrefix(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"By ARDTU@FRDS: 正文内容", "正文内容"},
		{"by ARDTU@PTer 正文", "正文"},
		{"By ARDTU 正文", "正文"},
		{"By ARDTU@FRDS", ""}, // 只有前缀
		{"无前缀的正文", "无前缀的正文"},
	}
	for _, c := range cases {
		got := StripByARDTUPrefix(c.in)
		if got != c.want {
			t.Errorf("StripByARDTUPrefix(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestIsAcknowledgmentQuote(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{"短文本", true}, // < 200 rune
		{"官组声明：仅供本站", true},
		{"感谢原制作者提供资源", true},
		{strings.Repeat("长文本", 150), false}, // > 200 rune 且不含声明关键词
		{"", false},
	}
	for _, c := range cases {
		got := IsAcknowledgmentQuote(c.text)
		if got != c.want {
			t.Errorf("IsAcknowledgmentQuote(text len=%d) = %v, want %v", len([]rune(c.text)), got, c.want)
		}
	}
}

func TestExtractQuoteBlocks(t *testing.T) {
	bbcode := `前言[quote]第一个 quote[/quote]中间[quote=有标签]第二个[/quote]结尾`
	blocks := extractQuoteBlocks(bbcode)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 quote blocks, got %d", len(blocks))
	}
	if strings.TrimSpace(blocks[0].Inner) != "第一个 quote" {
		t.Errorf("block 0 inner mismatch: %q", blocks[0].Inner)
	}
	if strings.TrimSpace(blocks[1].Inner) != "第二个" {
		t.Errorf("block 1 inner mismatch: %q", blocks[1].Inner)
	}
	if blocks[0].Start >= blocks[1].Start {
		t.Errorf("block positions wrong: 0=%d, 1=%d", blocks[0].Start, blocks[1].Start)
	}
}

func TestExtractQuoteBlocks_Empty(t *testing.T) {
	blocks := extractQuoteBlocks("无 quote 文本")
	if blocks != nil {
		t.Errorf("expected nil for no quotes, got %v", blocks)
	}
}

func TestSplitPosterAndScreenshots_FirstImage(t *testing.T) {
	urls := []string{"shot1.jpg", "shot2.jpg", "shot3.jpg"}
	poster, screenshots := splitPosterAndScreenshots(urls)
	if poster != "shot1.jpg" {
		t.Errorf("expected first image as poster, got %q", poster)
	}
	if len(screenshots) != 2 {
		t.Errorf("expected 2 screenshots, got %d", len(screenshots))
	}
}

func TestSplitPosterAndScreenshots_KeywordPriority(t *testing.T) {
	urls := []string{"shot1.jpg", "https://img9.doubanio.com/poster.jpg", "shot2.jpg"}
	poster, screenshots := splitPosterAndScreenshots(urls)
	if !strings.Contains(poster, "doubanio") {
		t.Errorf("expected doubanio URL as poster, got %q", poster)
	}
	if len(screenshots) != 2 {
		t.Errorf("expected 2 screenshots, got %d", len(screenshots))
	}
}

func TestSplitPosterAndScreenshots_Empty(t *testing.T) {
	poster, screenshots := splitPosterAndScreenshots(nil)
	if poster != "" {
		t.Errorf("expected empty poster, got %q", poster)
	}
	if screenshots != nil {
		t.Errorf("expected nil screenshots, got %v", screenshots)
	}
}

func TestFilterUnwantedImages(t *testing.T) {
	urls := []string{
		"https://example.com/poster.jpg",
		"https://example.com/logo.png",    // 黑名单
		"https://example.com/icon.png",    // 黑名单
		"https://example.com/shot1.jpg",
		"https://example.com/banner.gif",  // 黑名单
	}
	valid := filterUnwantedImages(urls)
	if len(valid) != 2 {
		t.Errorf("expected 2 valid images, got %d: %v", len(valid), valid)
	}
}

func TestSplitIntroSections_Statement(t *testing.T) {
	// 声明 quote 在首图前（实际 PT 站常见结构：声明 → 海报 → 正文）
	descrHTML := `<div>
<blockquote>官组声明：仅供本站</blockquote>
<img src="https://example.com/poster.jpg"/>
<p>正文内容</p>
</div>`
	descrBBCode := `[quote]官组声明：仅供本站[/quote]
[img]https://example.com/poster.jpg[/img]
正文内容`

	p := NewPublicExtractor("test", "测试")
	intro := p.splitIntroSections(descrHTML, descrBBCode)

	if intro.Poster != "https://example.com/poster.jpg" {
		t.Errorf("Poster mismatch: %q", intro.Poster)
	}
	if !strings.Contains(intro.Statement, "官组声明") {
		t.Errorf("Statement should contain '官组声明', got %q", intro.Statement)
	}
	if strings.Contains(intro.Body, "官组声明") {
		t.Errorf("Body should not contain Statement, got %q", intro.Body)
	}
}

func TestSplitIntroSections_ARDTUStripped(t *testing.T) {
	// ARDTU 签名在首图前
	descrHTML := `<div>
<blockquote>By ARDTU@FRDS: 压制声明</blockquote>
<img src="https://example.com/poster.jpg"/>
<p>正文</p>
</div>`
	descrBBCode := `[quote]By ARDTU@FRDS: 压制声明[/quote]
[img]https://example.com/poster.jpg[/img]
正文`

	p := NewPublicExtractor("test", "测试")
	intro := p.splitIntroSections(descrHTML, descrBBCode)

	if len(intro.RemovedARDTUDeclarations) == 0 {
		t.Error("should have ARDTU declarations removed")
	}
	if strings.Contains(intro.Body, "By ARDTU") {
		t.Errorf("Body should not contain ARDTU signature, got %q", intro.Body)
	}
}

func TestSplitIntroSections_NoPoster(t *testing.T) {
	descrHTML := `<div><p>无图片的简介</p></div>`
	descrBBCode := `无图片的简介`

	p := NewPublicExtractor("test", "测试")
	intro := p.splitIntroSections(descrHTML, descrBBCode)

	if intro.Poster != "" {
		t.Errorf("expected empty poster, got %q", intro.Poster)
	}
	if intro.Body != "无图片的简介" {
		t.Errorf("Body should be the full bbcode, got %q", intro.Body)
	}
}

func TestSplitIntroSections_LogoFiltered(t *testing.T) {
	descrHTML := `<div>
<img src="https://example.com/logo.png"/>
<img src="https://example.com/poster.jpg"/>
<img src="https://example.com/shot1.jpg"/>
</div>`
	p := NewPublicExtractor("test", "测试")
	intro := p.splitIntroSections(descrHTML, "")

	// logo 应被过滤，poster.jpg 成为首图
	if strings.Contains(intro.Poster, "logo") {
		t.Errorf("logo should be filtered, got poster=%q", intro.Poster)
	}
	if !strings.Contains(intro.Poster, "poster.jpg") {
		t.Errorf("expected poster.jpg as poster, got %q", intro.Poster)
	}
	urls := intro.ScreenshotURLs()
	if len(urls) != 1 || urls[0] != "https://example.com/shot1.jpg" {
		t.Errorf("expected 1 screenshot, got %v", urls)
	}
}
