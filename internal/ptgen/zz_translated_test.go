package ptgen

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.237: TranslatedTitles 独立行提取（doubaninfo 正置语义）+陷阱隔离
func TestTranslatedTitlesExtraction(t *testing.T) {
	r := &model.PTGenResult{
		ChineseTitle: "火车梦", // 端点结构化（正置语义）
		ForeignTitle: "火车梦", // 端点陷阱字段（恒=中文）
		RawBBCode:    "◎片　　名　火车梦\n◎译　　名　Train Dreams / 铁路梦影(港) / 火車大夢(台)\n◎年　　代　2025",
	}
	enrichFromBBCode(r)
	if r.TranslatedTitles != "Train Dreams / 铁路梦影(港) / 火車大夢(台)" {
		t.Errorf("TranslatedTitles = %q, want 完整译名串", r.TranslatedTitles)
	}
	if r.ChineseTitle != "火车梦" {
		t.Errorf("ChineseTitle = %q（端点正置语义应保持）", r.ChineseTitle)
	}
	// 意语样本（§59.235 挂账场景——完整串天然包含）
	r2 := &model.PTGenResult{RawBBCode: "◎译　　名　Il Profumo della signora in nero / The Perfume of the Lady in Black"}
	enrichFromBBCode(r2)
	if r2.TranslatedTitles == "" || r2.TranslatedTitles[:2] != "Il" {
		t.Errorf("意语完整串: %q", r2.TranslatedTitles)
	}
}
