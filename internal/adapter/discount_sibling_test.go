package adapter

import (
	"strings"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.367: NP 新版 details-* 前端（深度站 dstudio.me 实证）促销标记在
// h1 兄弟节点（details-title-statuses div）——h1 内扫不到，须扩扫父容器。
// 标准 NP 促销在 h1 内——行为不变（回归锚）。

func TestDetectDiscountDetailsPageSiblingPromotion(t *testing.T) {
	// 深度站真实形态（tid 5059 实录）：h1 干净 + 兄弟 statuses 含
	// <font class='free'>免费</font> + 剩余时间 title 属性。
	html := `<div class="details-title-row">
	<h1 class="details-title">Merry Berry Love 2026 S01E04 1080p DSNP WEB-DL AAC2.0 H.264-DepWeb</h1>
	<div class="details-title-statuses"><span class="details-title-status details-title-status--promotion">
	<b>[<font class='free' >免费</font>]</b> <font color='#0000FF'>剩余时间：<span title="2026-11-01 01:03:03">2天16时</span></font>
	</span></div></div>`
	r := DetectDiscountFromDetailsPage(html, nil)
	if r.Level != model.DiscountFree {
		t.Fatalf("深度站兄弟节点 free 促销应判 FREE，got %q", r.Level)
	}
	if r.FreeEndAt == nil {
		t.Fatal("剩余时间在兄弟节点——父容器扩展后应解析到 FreeEndAt")
	}
	want := time.Date(2026, 11, 1, 1, 3, 3, 0, time.Local)
	if !r.FreeEndAt.Equal(want) {
		t.Fatalf("FreeEndAt=%v want %v", r.FreeEndAt, want)
	}
}

func TestDetectDiscountDetailsPageSiblingHalfdown(t *testing.T) {
	// tid 3374 实录形态：[50%] halfdown（下载半价）
	html := `<div class="details-title-row">
	<h1 class="details-title">Desert of Namibia 2024 1080p-DStudio</h1>
	<div class="details-title-statuses"><span class="details-title-status details-title-status--promotion">
	<b>[<font class='halfdown' >50%</font>]</b></span></div></div>`
	r := DetectDiscountFromDetailsPage(html, nil)
	if r.Level != model.DiscountPercent50 {
		t.Fatalf("兄弟节点 halfdown 应判 PERCENT_50，got %q", r.Level)
	}
}

func TestDetectDiscountDetailsPageH1InlineRegression(t *testing.T) {
	// 标准 NP：促销在 h1 内——首轮命中，行为不变（回归锚）
	html := `<h1>Title 2024 1080p <font class="pro_free">Free</font></h1>`
	r := DetectDiscountFromDetailsPage(html, nil)
	if r.Level != model.DiscountFree {
		t.Fatalf("h1 内 pro_free 应判 FREE（回归），got %q", r.Level)
	}
}

func TestDetectDiscountDetailsPageNoPromotion(t *testing.T) {
	// 无促销——NONE；父容器扩展不引入误报
	html := `<div class="details-title-row">
	<h1 class="details-title">Plain Title 2024 1080p-GRP</h1>
	<div class="details-title-statuses"><span>普通</span></div></div>`
	r := DetectDiscountFromDetailsPage(html, nil)
	if r.Level != model.DiscountNone {
		t.Fatalf("无促销应判 NONE，got %q", r.Level)
	}
	// h1 不存在（非详情页形态）——原逻辑全文扫，父容器扩展跳过
	r2 := DetectDiscountFromDetailsPage(`<div>no h1 here</div>`, nil)
	if r2.Level != model.DiscountNone {
		t.Fatalf("无 h1 无促销应判 NONE，got %q", r2.Level)
	}
}

func TestH1PromotionSiblingScope(t *testing.T) {
	// 兄弟形态（深度站）：返回紧邻兄弟 outer HTML（不含 h1）
	html := `<div class="details-title-row"><h1>T</h1><div class="details-title-statuses">X</div></div>`
	scope := h1PromotionSiblingScope(html)
	if !strings.Contains(scope, `<div class="details-title-statuses">`) || strings.Contains(scope, "<h1>") {
		t.Fatalf("应返回紧邻兄弟 outer HTML，got %q", scope)
	}
	if h1PromotionSiblingScope(`<div>no h1</div>`) != "" {
		t.Fatal("无 h1 应返回空串")
	}
	// 长度门：兄弟超 2000B（相似大表形态）返回空串
	big := `<h1>T</h1><div>` + strings.Repeat("x", 2500) + `</div>`
	if h1PromotionSiblingScope(big) != "" {
		t.Fatal("兄弟超 2000B 应返回空串（相似区防御）")
	}
	// 相似区防御锚（既有语义回归）：h1 next 是 h2（相似区标题）——无促销不误判
	html2 := `<html><body><h1>Target</h1><h2>Similar</h2><table><tr><td><img class="pro_free" src="x.gif" /></td></tr></table></body></html>`
	if r := DetectDiscountFromDetailsPage(html2, nil); r.Level != model.DiscountNone {
		t.Fatalf("相似区 pro_free 不应误判，got %q", r.Level)
	}
}
