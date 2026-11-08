package extract

import (
	"testing"

	"github.com/PuerkitoBio/goquery"
	"strings"
)

func docOf(t *testing.T, html string) *goquery.Document {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return doc
}

// §59.487: 修道院横幅防御——全站固定横幅 h1（slogan）与 <title> 真标题
// 零词重叠 → 裸 h1 跳过 → 终达 <title> 清理值（正确种子标题）。
func TestExtractTitle_BannerDefense_Xdypt(t *testing.T) {
	// 修道院 4042 真实形态（2026-11-08 抓取实证）
	html := `<html><head><title>修道院 :: 种子详情 "A Penny 2020 2160p WEB-DL DoVi H.265 DDP 5.1 Atmos-CMCTV"</title></head>
	<body><h1>渡劫，炼炁，修真而登仙</h1><h2>未登录!</h2>
	<div>正文区</div></body></html>`
	p := &PublicExtractor{}
	got := p.extractTitle(docOf(t, html), "fallback")
	want := `A Penny 2020 2160p WEB-DL DoVi H.265 DDP 5.1 Atmos-CMCTV`
	if got != want {
		t.Errorf("修道院横幅: got=%q want=%q", got, want)
	}
}

// 正常站：裸 h1=种子名 与 <title> 高度重叠 → h1 命中保持（不变）。
func TestExtractTitle_BannerDefense_NormalH1(t *testing.T) {
	html := `<html><head><title>某站 :: 种子详情 "Movie Name 2020 1080p"</title></head>
	<body><h1>Movie Name 2020 1080p BluRay</h1></body></html>`
	p := &PublicExtractor{}
	got := p.extractTitle(docOf(t, html), "")
	if got != "Movie Name 2020 1080p BluRay" {
		t.Errorf("正常 h1 应保持: %q", got)
	}
}

// h1 中文/title 英文（零词重叠）→ 跳 h1 落 title（英文官方标题）。
func TestExtractTitle_BannerDefense_CjkH1LatinTitle(t *testing.T) {
	html := `<html><head><title>Site :: 种子详情 "English Title 2021"</title></head>
	<body><h1>中文电影名蓝光原盘</h1></body></html>`
	p := &PublicExtractor{}
	got := p.extractTitle(docOf(t, html), "")
	if got != "English Title 2021" {
		t.Errorf("零重叠应落 title: %q", got)
	}
}

// <title> 清理后空 → 不校验（h1 保持——防御条件非空）。
func TestExtractTitle_BannerDefense_EmptyTitleTag(t *testing.T) {
	html := `<html><head><title></title></head>
	<body><h1>Just The H1 Title</h1></body></html>`
	p := &PublicExtractor{}
	got := p.extractTitle(docOf(t, html), "")
	if got != "Just The H1 Title" {
		t.Errorf("title 空时 h1 保持: %q", got)
	}
}

// 特异选择器 #torrent-name 不校验（语义明确——SSD 形态优先）。
func TestExtractTitle_BannerDefense_SpecificSelector(t *testing.T) {
	html := `<html><head><title>SSD :: 种子详情 "Whatever Else"</title></head>
	<body><span id="torrent-name">Specific Name Torrent</span><h1>Site Banner</h1></body></html>`
	p := &PublicExtractor{}
	got := p.extractTitle(docOf(t, html), "")
	if got != "Specific Name Torrent" {
		t.Errorf("特异选择器优先: %q", got)
	}
}
