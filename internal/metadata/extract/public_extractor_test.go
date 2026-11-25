package extract

import (
	"strings"
	"testing"
)

const samplePTPage = `<!DOCTYPE html>
<html><head><title>测试电影 2024 2160p UHD Blu-ray FRDS</title></head>
<body>
<h1 id="top">测试电影 2024 2160p UHD Blu-ray FRDS |pter</h1>
<table class="torrent_info">
  <tr><th>大小</th><td>8.5 GB</td></tr>
  <tr><th>类型</th><td>电影</td></tr>
  <tr><th>媒介</th><td>UHD Blu-ray</td></tr>
  <tr><th>编码</th><td>H.265/HEVC</td></tr>
  <tr><th>分辨率</th><td>2160p</td></tr>
  <tr><th>制作组</th><td>FRDS</td></tr>
  <tr><th>Hash码</th><td>abc123def4567890abc123def4567890abc123de</td></tr>
</table>
<div class="tags">
  <a class="torrent_tag" href="#">喜剧</a>
  <a class="torrent_tag" href="#">动作</a>
</div>
<div id="kdescr">
  <p>官组声明（仅限本站）</p>
  <img src="https://img.example.com/poster.jpg"/>
  <p>简介正文：<a href="https://www.imdb.com/title/tt1234567/">IMDb</a></p>
  <blockquote>引用片段</blockquote>
  <img src="https://img.example.com/shot1.jpg"/>
  <img src="https://img.example.com/shot2.jpg"/>
</div>
</body>
</html>`

func TestPublicExtractor_EndToEnd(t *testing.T) {
	p := NewPublicExtractor("test-site", "测试站")
	seed, err := p.Extract(Input{
		SiteCode:      "test-site",
		SiteNickname:  "测试站",
		PageHTML:      samplePTPage,
		FallbackTitle: "fallback",
	})
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// 标题（应去除 |pter 工具 ID 后缀）
	if seed.Title == "" {
		t.Error("Title is empty")
	}
	if strings.Contains(seed.Title, "|pter") {
		t.Errorf("Title should strip tool ID suffix, got %q", seed.Title)
	}
	if !strings.Contains(seed.Title, "测试电影") {
		t.Errorf("Title should contain '测试电影', got %q", seed.Title)
	}

	// 大小
	if seed.Size <= 0 {
		t.Errorf("Size should be > 0, got %d", seed.Size)
	}
	// 8.5 GB ≈ 9 * 1024^3
	expectedBytes := int64(8.5 * 1024 * 1024 * 1024)
	if seed.Size != expectedBytes {
		t.Logf("Size = %d (expected ~%d)", seed.Size, expectedBytes)
	}

	// 类型（v0.0.238: 标准化为 code）
	if seed.Type != "category.movie" {
		t.Errorf("Type mismatch: %q", seed.Type)
	}
	// 媒介
	// v0.0.238: 字段值经 LookupStandardKey 标准化为 code
	if seed.Medium != "medium.uhd_bluray" {
		t.Errorf("Medium mismatch: %q", seed.Medium)
	}
	// 视频编码
	if seed.VideoCodec != "video.h265" {
		t.Errorf("VideoCodec mismatch: %q", seed.VideoCodec)
	}
	// 分辨率
	if seed.Resolution != "resolution.r2160p" {
		t.Errorf("Resolution mismatch: %q", seed.Resolution)
	}
	// 制作组
	if seed.ReleaseGroup != "team.frds" {
		t.Errorf("ReleaseGroup mismatch: %q", seed.ReleaseGroup)
	}

	// 标签
	if len(seed.Tags) < 2 {
		t.Errorf("Tags should have 2 items, got %v", seed.Tags)
	}
	found := false
	for _, tag := range seed.Tags {
		if tag == "喜剧" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Tags should contain '喜剧', got %v", seed.Tags)
	}

	// InfoHash（40 位 hex）
	if len(seed.InfoHash) != 40 {
		t.Errorf("InfoHash should be 40 chars, got %q", seed.InfoHash)
	}

	// IMDb
	if !strings.Contains(seed.IMDbLink, "tt1234567") {
		t.Errorf("IMDbLink mismatch: %q", seed.IMDbLink)
	}

	// 海报（首图）
	if !strings.Contains(seed.Intro.Poster, "poster.jpg") {
		t.Errorf("Poster mismatch: %q", seed.Intro.Poster)
	}

	// 截图（去掉首图后剩余）
	urls := seed.Intro.ScreenshotURLs()
	if len(urls) != 2 {
		t.Errorf("expected 2 screenshots, got %d: %v", len(urls), urls)
	}
}

func TestPublicExtractor_EmptyHTML(t *testing.T) {
	p := NewPublicExtractor("test", "测试")
	_, err := p.Extract(Input{PageHTML: ""})
	if err == nil {
		t.Error("expected error for empty HTML")
	}
}

func TestPublicExtractor_ImplementsExtractor(t *testing.T) {
	// 验证 PublicExtractor 实现 Extractor 接口
	var _ Extractor = (*PublicExtractor)(nil)
}

func TestExtractSize_ParseSizeToBytes(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"1 KB", 1024},
		{"1 MB", 1024 * 1024},
		{"1 GB", 1024 * 1024 * 1024},
		{"1 TB", 1024 * 1024 * 1024 * 1024},
		{"1.5 GB", int64(1.5 * 1024 * 1024 * 1024)},
		{"800 MB", 800 * 1024 * 1024},
		{"2,500 MB", 2500 * 1024 * 1024}, // 含逗号
		{"invalid", 0},
		{"", 0},
	}
	for _, c := range cases {
		got := parseSizeToBytes(c.in)
		if got != c.want {
			t.Errorf("parseSizeToBytes(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestExtractFlags_Default(t *testing.T) {
	p := NewPublicExtractor("test", "测试")
	flags := p.extractFlags("电影标题", "副标题 禁转", "", "正文")
	if len(flags) == 0 {
		t.Error("should detect '禁转' flag")
	}
	found := false
	for _, f := range flags {
		if f == "禁转" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("flags should contain '禁转', got %v", flags)
	}
}

func TestMatchBasicInfoLabel(t *testing.T) {
	cases := map[string]string{
		"类型":      "type",
		"類型":      "type",
		"类型：":     "type",
		"大小":      "", // 不在默认 7 类
		"媒介":      "medium",
		"分辨率":     "resolution",
		"制作组：":    "team",
		"unknown": "",
		"":        "",
	}
	for in, want := range cases {
		got := matchBasicInfoLabel(in)
		if got != want {
			t.Errorf("matchBasicInfoLabel(%q) = %q, want %q", in, got, want)
		}
	}
}

// §59.577 附（1633·猫 tid=802365 Send Help SPHD 案）: 编码器日志 quote
// （压制信息）不入 Statement·保留 Body（压制参数属简介正文·信息不丢）；
// 同区真官组声明仍入 Statement（分类互不干扰）。
const sampleEncoderLogPage = `<!DOCTYPE html>
<html><head><title>Send Help 2026 2160p UHD BluRay DoVi HDR10+ x265 TrueHD 7.1 Atmos 2Audios-SPHD</title></head>
<body>
<h1 id="top">Send Help 2026 2160p UHD BluRay DoVi HDR10+ x265 TrueHD 7.1 Atmos 2Audios-SPHD</h1>
<table class="torrent_info">
  <tr><th>类型</th><td>电影</td></tr>
  <tr><th>媒介</th><td>UHD Blu-ray</td></tr>
</table>
<div id="kdescr">
  <blockquote>MTeam官组作品，感谢原制作者发布。</blockquote>
  <blockquote>x265 [INFO]: frame I:  1252, Avg QP:11.54  kb/s: 85071.08
x265 [INFO]: frame P:  32378, Avg QP:12.22  kb/s: 54477.34
x265 [INFO]: frame B: 128971, Avg QP:14.00  kb/s: 38788.84</blockquote>
  <img src="https://img.example.com/poster.jpg"/>
  <p>简介正文</p>
</div>
</body>
</html>`

func TestPublicExtractor_EncoderLogExcludedFromStatement(t *testing.T) {
	p := NewPublicExtractor("test-site", "测试站")
	seed, err := p.Extract(Input{
		SiteCode:      "test-site",
		SiteNickname:  "测试站",
		PageHTML:      sampleEncoderLogPage,
		FallbackTitle: "fallback",
	})
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}
	// ① 编码日志不入 Statement（猫案修复本体）
	if strings.Contains(seed.Intro.Statement, "x265 [INFO]") {
		t.Errorf("Statement 不应含编码日志, got %q", seed.Intro.Statement)
	}
	// ② 同区真声明仍入 Statement（互不干扰）
	if !strings.Contains(seed.Intro.Statement, "官组作品") {
		t.Errorf("Statement 应含真官组声明, got %q", seed.Intro.Statement)
	}
	// ③ 编码日志保留 Body（压制参数属简介正文·信息不丢）
	if !strings.Contains(seed.Intro.Body, "x265 [INFO]") {
		t.Errorf("Body 应保留编码日志（压制参数）, got %q", seed.Intro.Body)
	}
}
