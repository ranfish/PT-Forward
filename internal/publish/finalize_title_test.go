package publish

import (
	"strings"
	"context"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/titleparser"
)

// §59.514 附八: FinalizeTitle 共享单点锚——三/四消费方结构性同源。
func TestFinalizeTitleSinglePoint(t *testing.T) {
	e := &PublishExecutor{}
	// E01 坏标题+归一信号在场（副标全N集）——torrentData 空（仲裁降级）
	meta := &model.TorrentMetadata{
		Title:    "Quacks E01 2017 1080p AMZN WEBRip HEVC DDP 2.0-cXcY@FRDS",
		Subtitle: "江湖郎中 / 怪诞医生 | 全6集",
		Category: "category.tv_series",
		Tags:     `["complete"]`,
	}
	tp := titleparser.BuildTechProfile(meta.Title, "", "", "", "", "")
	title, notes := e.FinalizeTitle(context.Background(), meta, &tp, nil)
	// 归一生效：裸 E01+全6集信号 → S01（通道 C）
	if title == "" || !strings.Contains(title, "S01") {
		t.Errorf("FinalizeTitle=%q want 含 S01（归一通道 C）", title)
	}
	if strings.Contains(title, "E01") {
		t.Errorf("E01 未被归一: %q", title)
	}
	_ = notes
}

// preAuditTitle 读列锚：列在场优先列（§59.304 分叉防）·列空降级现算。
func TestPreAuditTitleReadsColumn(t *testing.T) {
	e := &PublishExecutor{}
	meta := &model.TorrentMetadata{
		Title:             "Some Raw Title",
		ReassembledTitle:  "Column Title S01 2020 1080p BluRay x264-GRP",
	}
	got, ok := e.preAuditTitle(meta)
	if !ok || got != "Column Title S01 2020 1080p BluRay x264-GRP" {
		t.Errorf("preAuditTitle 应优先列: got=%q ok=%v", got, ok)
	}
	// 列空降级
	meta2 := &model.TorrentMetadata{Title: "Show S01 2020 1080p BluRay x264-GRP"}
	got2, ok2 := e.preAuditTitle(meta2)
	if !ok2 || got2 == "" {
		t.Errorf("列空降级现算: got=%q", got2)
	}
}


// §59.514 附十 B: MI 证据层恒跑锚——torrentData 空（recompute 降级路径
// ExportTorrent 失败）时 WEB-DL∧MIEncoded 仍须纠正 WEBRip（原被
// arbitrateWithTorrent :46 门整体拦——243 全库筛查 35 行漏纠实证）。
func TestFinalizeTitleMIEvidenceWithoutTorrent(t *testing.T) {
	e := &PublishExecutor{}
	meta := &model.TorrentMetadata{
		Title:    "Movie 2025 1080p WEB-DL x264 AAC-UBWEB",
		Category: "category.movie",
	}
	// §59.517: MI 升级为引擎裁决消费——合成真实民压形态（x264+sc40+crf
	// 非零小数+开源 keyint 组合·Kanjoos 同族）。裸 x264 WL 零参数形态引擎
	// 按 B2 先验判官方不再无条件翻转（平台 x264 流语义——§59.517 交叉层）。
	tp := titleparser.BuildTechProfile(meta.Title, "General\nVideo\nFormat : AVC\nWriting library : x264 core 148\nEncoding settings : scenecut=40 / crf=20.5 / keyint=250 / keyint-min=25\nWidth : 1 920 pixels\nHeight : 816 pixels\nAudio #1\nFormat : AAC LC\nChannel(s) : 2 channels\n", "", "", "", "")
	if !tp.MIEncoded {
		t.Fatalf("前置失败：MIEncoded 应为 true（Writing library x264）")
	}
	if tp.Specification != "WEB-DL" {
		t.Fatalf("前置失败：Specification=%q want WEB-DL", tp.Specification)
	}
	if tp.MIWebVerdict != titleparser.WEBVerdictCivilian {
		t.Fatalf("前置失败：引擎裁决=%q want civilian", tp.MIWebVerdict)
	}
	title, notes := e.FinalizeTitle(context.Background(), meta, &tp, nil) // torrentData=nil——降级路径
	if tp.Specification != "WEBRip" {
		t.Errorf("降级路径漏纠：Specification=%q want WEBRip（MI 证据层应恒跑）", tp.Specification)
	}
	found := false
	for _, n := range notes {
		if strings.Contains(n, "规格仲裁(MI引擎)") {
			found = true
		}
	}
	if !found {
		t.Errorf("notes 缺 MI 引擎仲裁记录: %v", notes)
	}
	if !strings.Contains(title, "WEBRip") {
		t.Errorf("标题未含 WEBRip: %q", title)
	}
}
