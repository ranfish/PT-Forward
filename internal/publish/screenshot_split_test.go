package publish

import (
	"strings"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

// TestAssembleDescription_ScreenshotFieldSplit §59.541 附十一 回归锚：
// 配置 screenshots 域 → 截图不内嵌 descr（走独立框）；未配域 → 内嵌
// 行为不变（NP 主流零回归）。
func TestAssembleDescription_ScreenshotFieldSplit(t *testing.T) {
	meta := func() *model.TorrentMetadata {
		return &model.TorrentMetadata{
			Description: "◎片　　名　Test\n◎简　　介　剧情",
			Screenshots: "[\"https://img.example/1.jpg\",\"https://img.example/2.jpg\"]",
		}
	}

	// ① 未配 screenshots 域：内嵌行为不变（NP 主流）
	cfgPlain := &model.PublishFormConfig{FormFields: map[string]string{"description": "descr"}}
	out := assembleDescription(meta(), cfgPlain)
	if !strings.Contains(out, "[img]https://img.example/1.jpg[/img]") {
		t.Fatalf("① 未配域应内嵌截图，got: %q", out)
	}

	// ② 配 screenshots 域：descr 不内嵌
	cfgSplit := &model.PublishFormConfig{FormFields: map[string]string{
		"description": "descr", "screenshots": "screenshots"}}
	out = assembleDescription(meta(), cfgSplit)
	if strings.Contains(out, "[img]") {
		t.Fatalf("② 配域后 descr 不应含 [img]，got: %q", out)
	}
	if !strings.Contains(out, "◎片　　名") {
		t.Fatalf("② 简介正文应保留，got: %q", out)
	}
}

// TestAssembleDescription_CoverFieldPosterStrip §59.541 附十一 回归锚：
// 配置 cover 域 → 简介首行海报 [img] 剥离（走独立框）；未配域 → 保留。
func TestAssembleDescription_CoverFieldPosterStrip(t *testing.T) {
	meta := func() *model.TorrentMetadata {
		return &model.TorrentMetadata{
			Description: "[img]https://db.example/poster.jpg[/img]\n◎片　　名　Test\n◎简　　介　剧情",
		}
	}

	// ① 未配 cover 域：海报原样保留（瓷器以外的现有行为）
	cfgPlain := &model.PublishFormConfig{FormFields: map[string]string{"description": "descr"}}
	out := assembleDescription(meta(), cfgPlain)
	if !strings.Contains(out, "[img]https://db.example/poster.jpg[/img]") {
		t.Fatalf("① 未配域海报应保留，got: %q", out)
	}

	// ② 配 cover 域：首行海报剥除·正文保留
	cfgCover := &model.PublishFormConfig{FormFields: map[string]string{
		"description": "descr", "cover": "cover", "screenshots": "screenshots"}}
	out = assembleDescription(meta(), cfgCover)
	if strings.Contains(out, "[img]https://db.example/poster.jpg[/img]") {
		t.Fatalf("② 配域后海报应剥除，got: %q", out)
	}
	if !strings.Contains(out, "◎片　　名　Test") {
		t.Fatalf("② 正文应保留，got: %q", out)
	}
}

// TestStripLeadingPosterImage 剥壳函数单点锚（五态）。
func TestStripLeadingPosterImage(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"[img]https://a/1.jpg[/img]\n正文", "正文"},                                 // 标准形态
		{"\n[img]https://a/1.jpg[/img]\n\n正文", "正文"},                             // 前导空白
		{"正文开头\n[img]https://a/1.jpg[/img]", "正文开头\n[img]https://a/1.jpg[/img]"}, // 非首行不剥
		{"没有图的开头", "没有图的开头"},                                                     // 无图
		{"[img]未闭合", "[img]未闭合"},                                                 // 未闭合形态原样
	}
	for i, c := range cases {
		if got := stripLeadingPosterImage(c.in); got != c.want {
			t.Fatalf("态%d: got %q want %q", i+1, got, c.want)
		}
	}
}
