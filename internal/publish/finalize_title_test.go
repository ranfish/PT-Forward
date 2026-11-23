package publish

import (
	"context"
	"strings"
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
		Title:            "Some Raw Title",
		ReassembledTitle: "Column Title S01 2020 1080p BluRay x264-GRP",
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
func TestFinalizeTitleFollowSourceSpec(t *testing.T) {
	// §59.536: WEB 族 MI 仲裁撤销——跟随源站标注。原 §59.514 附十 B
	// 「降级路径恒跑纠 WEB-DL→WEBRip」锚反转：MI civilian 裁决在场
	// （真压制形态）规格仍保真 WEB-DL（财神强制改回/优堡「与文件名
	// 不符」拒——站方 ground truth=源站标注）。
	e := &PublishExecutor{}
	meta := &model.TorrentMetadata{
		Title:    "Movie 2025 1080p WEB-DL x264 AAC-UBWEB",
		Category: "category.movie",
	}
	tp := titleparser.BuildTechProfile(meta.Title, "General\nVideo\nFormat : AVC\nWriting library : x264 core 148\nEncoding settings : scenecut=40 / crf=20.5 / keyint=250 / keyint-min=25\nWidth : 1 920 pixels\nHeight : 816 pixels\nAudio #1\nFormat : AAC LC\nChannel(s) : 2 channels\n", "", "", "", "")
	if !tp.MIEncoded {
		t.Fatalf("前置失败：MIEncoded 应为 true")
	}
	if tp.Specification != "WEB-DL" {
		t.Fatalf("前置失败：Specification=%q want WEB-DL", tp.Specification)
	}
	if tp.MIWebVerdict != titleparser.WEBVerdictCivilian {
		t.Fatalf("前置失败：引擎裁决=%q want civilian（计算保留·IsEncode 消费）", tp.MIWebVerdict)
	}
	_, notes := e.FinalizeTitle(context.Background(), meta, &tp, nil)
	if tp.Specification != "WEB-DL" {
		t.Errorf("§59.536 跟随源站：Specification=%q want WEB-DL（MI 不得改写）", tp.Specification)
	}
	for _, n := range notes {
		if strings.Contains(n, "规格仲裁") && strings.Contains(n, "WEB") {
			t.Errorf("WEB 族仲裁 note 应零出现: %v", n)
		}
	}
}
