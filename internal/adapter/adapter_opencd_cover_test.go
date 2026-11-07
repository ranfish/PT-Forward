package adapter

import "testing"

// §59.485: 皇后封面双版面提取（v0 正则双缺陷修复的回归锚）。
// v0 缺陷实证：新版面 img src='单引号' 不匹配（双引号正则）+ .*? 跨区
// 误捕 pic/trans.gif（12 库 9 簇污染）；老版面无 div.cover 零通道。

func TestOpenCDCover_NewLayoutSingleQuote(t *testing.T) {
	// 新版面（plugin_details·tid=204752 实证形态）：div.cover 直接子 img 单引号
	html := `<div class="cover" title="点击查看原图"><img src='https://img3.pixhost.to/images/6996/800453564_hermes-e34f84213d59bcee.jpg'/></div>`
	got := extractOpenCDCover(html)
	want := "https://img3.pixhost.to/images/6996/800453564_hermes-e34f84213d59bcee.jpg"
	if got != want {
		t.Errorf("新版面单引号封面: got=%q want=%q", got, want)
	}
}

func TestOpenCDCover_NewLayoutDoubleQuote(t *testing.T) {
	html := `<div class="cover"><img src="https://img2.pixhost.to/x.jpg"/></div>`
	if got := extractOpenCDCover(html); got != "https://img2.pixhost.to/x.jpg" {
		t.Errorf("新版面双引号: %q", got)
	}
}

func TestOpenCDCover_OldLayoutPreviewURL(t *testing.T) {
	// 老版面（details·tid=1065 实证形态）：kdescr 简介首图
	// （onclick Previewurl 指原图·src 为 .thumb.jpg 缩略——原图优先）
	html := `<div id='kdescr'><img id="attach136" alt="cover.jpg" src="attachments/201006/abc.jpg.thumb.jpg" onclick="Previewurl('attachments/201006/abc.jpg')" onmouseover="domTT_activate"></div>`
	got := extractOpenCDCover(html)
	if got != "attachments/201006/abc.jpg" {
		t.Errorf("老版面 Previewurl 原图优先: got=%q", got)
	}
}

func TestOpenCDCover_OldLayoutThumbFallback(t *testing.T) {
	// 老版面首图无 onclick（Previewurl 缺失）——thumb src 兜底
	html := `<div id="kdescr"><img src="attachments/201812/xyz.png.thumb.jpg"/></div>`
	if got := extractOpenCDCover(html); got != "attachments/201812/xyz.png.thumb.jpg" {
		t.Errorf("老版面 thumb 兜底: %q", got)
	}
}

func TestOpenCDCover_TransGifDefense(t *testing.T) {
	// trans.gif 站方占位图防御——新版面 div.cover 内占位（v0 跨区误捕形态）
	// +老版面首图占位：全跳过·返回空
	html := `<div class="cover"><img src='pic/trans.gif'/></div>` +
		`<div id='kdescr'><img src="pic/trans.gif"/></div>`
	if got := extractOpenCDCover(html); got != "" {
		t.Errorf("trans.gif 防御: got=%q want empty", got)
	}
}

func TestOpenCDCover_NoCover(t *testing.T) {
	// 双版面均无封面（大合集老种·div.cover 存在但空）
	html := `<div class="cover"></div><div id='kdescr'>纯文字简介</div>`
	if got := extractOpenCDCover(html); got != "" {
		t.Errorf("无封面: got=%q want empty", got)
	}
}
