package extract

import (
	"strings"
	"testing"

)

// TestExtractTitleTopContainerGuard §59.366: #top 在外层 header（容器）
// 而非 h1 自身时跳过——深度站 dstudio.me 案（Title 拼接促销+副题污染）。
func TestExtractTitleTopContainerGuard(t *testing.T) {
	html := `<html><body>
	<header class="details-hero" id="top"><div class="details-title-row">
	<h1 class="details-title">Clean Title 2024 1080p-GRP</h1>
	<div class="details-title-statuses"><span class="details-title-status--promotion"><b>[<font class='halfdown'>50%</font>]</b></span></div>
	</div><p class="details-subtitle">中文副题/中文</p></header>
	</body></html>`
	doc, err := ParseHTML(html)
	if err != nil {
		t.Fatal(err)
	}
	p := &PublicExtractor{}
	got := p.extractTitle(doc, "fallback")
	if got != "Clean Title 2024 1080p-GRP" {
		t.Fatalf("容器 #top 应跳过落 h1 干净值，got %q", got)
	}

	// 标准 NP：#top 是 h1 自身——照常命中
	html2 := `<html><body><h1 id="top">Standard NP Title</h1></body></html>`
	doc2, _ := ParseHTML(html2)
	if got := p.extractTitle(doc2, "fb"); got != "Standard NP Title" {
		t.Fatalf("标准 #top h1 应命中，got %q", got)
	}
}

// TestExtractSizeDetailsMeta §59.366: NP 新版 details-meta 结构
// （span label + strong value——深度站形态）。
func TestExtractSizeDetailsMeta(t *testing.T) {
	html := `<html><body><div class="details-meta">
	<div class="details-meta__item"><span class="details-meta__label">类型</span><strong class="details-meta__value">电影-Movies</strong></div>
	<div class="details-meta__item"><span class="details-meta__label">大小：</span><strong class="details-meta__value">46.26 GB</strong></div>
	</div></body></html>`
	doc, err := ParseHTML(html)
	if err != nil {
		t.Fatal(err)
	}
	// findSizeInRows 私有包内直测（主路径 dt/dd|th/td 不中 → details-meta 分支）
	got := findSizeInRows(doc)
	if !strings.Contains(got, "46.26") {
		t.Fatalf("details-meta 大小应命中，got %q", got)
	}
	if v := parseSizeToBytes(got); v != 49671296778 {
		t.Fatalf("46.26 GB 解析 = %d, want 49671296778", v)
	}
}
