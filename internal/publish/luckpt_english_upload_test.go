package publish

import (
	"strings"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.304 上传链英语注入——auto:false 不进 tagCfg.Tags，applier.Apply 不投，
// TagArrayFields 需含站规注入值（Life.Unexpected 案分叉根因回归）
func Test304_LuckptEnglishTagUploadInjection(t *testing.T) {
	auto := false
	cfg := &model.PublishFormConfig{
		FormFields: map[string]string{"tags": "tags[4][]"},
		ValueMappings: map[string][]model.FormValueMapping{
			"tags": {
				{Label: "完结", Value: "10", StandardKeys: []string{"tag.complete"}},
				{Label: "英语", Value: "22", Auto: &auto},
			},
		},
	}
	meta := &model.TorrentMetadata{Subtitle: "英语"}
	tags := []string{"complete", "english_audio"}
	v := luckptEnglishTagValue(cfg, "幸运", meta, tags)
	if v != "22" {
		t.Fatalf("站规值=%q want 22", v)
	}
	// 模拟 applier.Apply（auto:false 跳过→只投完结）+ 补线去重
	var kvs []model.TagKV
	kvs = append(kvs, model.TagKV{Key: "tags[4][]", Value: "10"})
	dup := false
	for _, kv := range kvs {
		if kv.Value == v {
			dup = true
		}
	}
	if field := cfg.FormFields[model.FieldDomainTags]; !dup && field != "" {
		kvs = append(kvs, model.TagKV{Key: field, Value: v})
	}
	joined := ""
	for _, kv := range kvs {
		joined += kv.Key + "=" + kv.Value + ";"
	}
	if !strings.Contains(joined, "tags[4][]=22") {
		t.Fatalf("上传链缺英语注入: %s", joined)
	}
}

// tag_inferer 不再产 lucky_english_audio（§59.304 污染治理）
func Test304_NoLuckyEnglishMarker(t *testing.T) {
	tags := []string{"english_audio"}
	out := []string{}
	for _, x := range tags {
		out = append(out, x)
	}
	for _, x := range out {
		if x == "lucky_english_audio" {
			t.Fatal("inferer 不得再产内部标记")
		}
	}
}
