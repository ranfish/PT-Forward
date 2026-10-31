package publish

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.400: 财神题材标签注入锚（喜剧——◎类别行同源消费）。
func TestCsptGenreTagValue(t *testing.T) {
	cfg := &model.PublishFormConfig{
		ValueMappings: map[string][]model.FormValueMapping{
			model.FieldDomainTags: {
				{Label: "HDR", Value: "7"},
				{Label: "喜剧", Value: "8"},
			},
		},
	}
	meta := &model.TorrentMetadata{Description: "◎片　　名　缩小人生\n◎类　　别　剧情 / 喜剧 / 科幻\n◎语　　言　英语"}
	// ① 财神+含喜剧 → 8
	if v := csptGenreTagValue(cfg, "财神", meta); v != "8" {
		t.Errorf("财神含喜剧 = %q, want 8", v)
	}
	// ② 非喜剧题材 → 空
	meta2 := &model.TorrentMetadata{Description: "◎类　　别　剧情 / 科幻"}
	if v := csptGenreTagValue(cfg, "财神", meta2); v != "" {
		t.Errorf("无喜剧 = %q, want 空", v)
	}
	// ③ 非财神站零变化
	if v := csptGenreTagValue(cfg, "幸运", meta); v != "" {
		t.Errorf("非财神站 = %q, want 空", v)
	}
	// ④ 无◎类别行（description 空）→ 空
	if v := csptGenreTagValue(cfg, "财神", &model.TorrentMetadata{}); v != "" {
		t.Errorf("空 description = %q, want 空", v)
	}
	// ⑤ 站方无喜剧选项 → 空（跨站安全）
	cfgNoComedy := &model.PublishFormConfig{
		ValueMappings: map[string][]model.FormValueMapping{
			model.FieldDomainTags: {{Label: "HDR", Value: "7"}},
		},
	}
	if v := csptGenreTagValue(cfgNoComedy, "财神", meta); v != "" {
		t.Errorf("无喜剧选项 = %q, want 空", v)
	}
}
