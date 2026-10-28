package adapter

import (
	"strings"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

// TestFillMediaFromRawBlock §59.366: 结构化字段区 MI/BDInfo 补取——
// 深度站 nexus-media-info-raw div 的 <pre>（DISC INFO: 报告头）。
func TestFillMediaFromRawBlock(t *testing.T) {
	a := &NexusPHPAdapter{}

	// ① BDInfo 形态（DISC INFO: 头）
	html := `<td class="rowfollow"><div class="nexus-media-info-raw" style="margin-top: 15px;"><details><summary>显示/隐藏原始 BDInfo</summary><pre>DISC INFO:  Disc Label:     Test.Disc.2024
Disc Size:      49,672,827,462 bytes
Protection:     AACS
PLAYLIST REPORT:  Name: 00010.MPLS</pre></details></div></td>`
	d := &model.TorrentDetail{}
	a.fillMediaFromRawBlock(html, d)
	if d.BDInfo == "" {
		t.Fatal("BDInfo 形态应填 detail.BDInfo")
	}
	if !strings.Contains(d.BDInfo, "DISC INFO") {
		t.Fatalf("BDInfo 内容异常: %q", d.BDInfo[:40])
	}
	if d.MediaInfo != "" {
		t.Fatal("BDInfo 形态不应填 MediaInfo")
	}

	// ② 已有值不覆盖（简介区优先）
	d2 := &model.TorrentDetail{MediaInfo: "existing MI"}
	a.fillMediaFromRawBlock(html, d2)
	if d2.BDInfo != "" || d2.MediaInfo != "existing MI" {
		t.Fatal("双非空才补——已有值不覆盖")
	}

	// ③ 无容器（标准 NP 站简介区自带 MI）——零动作
	d3 := &model.TorrentDetail{}
	a.fillMediaFromRawBlock(`<div>no raw block here</div>`, d3)
	if d3.BDInfo != "" || d3.MediaInfo != "" {
		t.Fatal("无 nexus-media-info-raw 容器应零动作")
	}
}
