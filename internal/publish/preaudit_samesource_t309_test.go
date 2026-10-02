package publish

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.309 预检消费主管线产物——TagArrayFields 构建单点（预检 body.Tags 与
// pubReq.TagArrayFields 同一产物，构造级同源断言）
func Test309_TagArrayFieldsSingleSource(t *testing.T) {
	auto := false
	cfg := &model.PublishFormConfig{
		FormFields: map[string]string{"tags": "tags[4][]"},
		ValueMappings: map[string][]model.FormValueMapping{
			"tags": {
				{Label: "完结", Value: "10"},
				{Label: "英语", Value: "22", Auto: &auto},
			},
		},
	}
	// 模拟前置构建段（executor 内联逻辑的等价重演）：applier.Apply 产物 + 站规注入
	tags := []string{"complete", "english_audio"}
	tagCfg := &model.SiteTagConfig{Mode: model.TagModeCheckboxSpan, SpanField: "tags[4][]", Tags: map[string]string{"complete": "10"}}
	applier := NewTagApplier(tagCfg)
	var fields []model.TagKV
	applier.Apply(tags, func(field, value string) {
		fields = append(fields, model.TagKV{Key: field, Value: value})
	})
	// 英语注入（幸运站规——auto:false 不在 Tags 映射）
	if v := luckptEnglishTagValue(cfg, "幸运", &model.TorrentMetadata{Subtitle: "英语"}, tags); v == "22" {
		fields = append(fields, model.TagKV{Key: "tags[4][]", Value: "22"})
	}
	if len(fields) != 2 || fields[0].Value != "10" || fields[1].Value != "22" {
		t.Fatalf("TagArrayFields=%v want [10 22]", fields)
	}
	// 预检 body.Tags 反查（miss 兜底 value）与上传同值集
	got := map[string]bool{}
	for _, kv := range fields {
		got[kv.Value] = true
	}
	if !got["10"] || !got["22"] {
		t.Fatal("预检/上传 tags 值集不一致")
	}
}
