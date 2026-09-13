package reseed

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/titleparser"
)

// §59.219 BFI 词典归位——霸王别姬案：候选 "1993.BFI.BluRay" 的 BFI 曾入
// platform 流媒体槽致 SourcePlatform 单侧反驳假杀（源 "BFI修复版" 粘连
// 提取失败）。移回品牌位（EditionInfo）+ §59.214 相对化。
func TestBFIDiscLabel(t *testing.T) {
	cand := "Farewell.My.Concubine.1993.BFI.BluRay.1080p.x264.FLAC.2Audios-CMCT"
	p := titleparser.ParseTitleTech(cand)
	if p.SourcePlatform != "" {
		t.Errorf("BFI 不应进 SourcePlatform: %q", p.SourcePlatform)
	}
	if p.EditionInfo != "BFI" {
		t.Errorf("BFI 应为品牌 EditionInfo: %q", p.EditionInfo)
	}
	src := "霸王别姬.BFI修复版.1993.1080p.国英双语￡CMCT风潇潇"
	m, _ := VerifyMatchWithTruncationCheckAndSource(
		[]*model.SeedingSearchResult{{TorrentID: "t", Title: cand, Size: 8000000000}},
		"CMCT", 8000000000, src)
	if m == nil {
		t.Error("BFI 碟标唯一候选应放行（无兄弟不反驳）")
	}
}
