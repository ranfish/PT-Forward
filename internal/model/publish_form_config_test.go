package model

import (
	"encoding/json"
	"strings"
	"testing"
)

// §59.147 切片 1: PublishFormConfig 序列化 roundtrip + 条件标签语义锚定。
func TestPublishFormConfigRoundtrip(t *testing.T) {
	no := false
	cfg := &PublishFormConfig{
		Enabled:     true,
		Framework:   "np",
		PreAuditURL: "/api/auto-audit/pre-audit",
		FormFields:  map[string]string{FieldDomainMedium: "medium_sel[4]", FieldDomainTags: "tags[4][]"},
		ValueMappings: map[string][]FormValueMapping{
			FieldDomainTags: {
				{Label: "英语", Value: "22", Auto: &no},              // 条件标签 §59.150
				{Label: "Dolby Vision", Value: "20", StandardKeys: []string{"tag.dolby_vision"}},
			},
		},
	}
	s := cfg.Serialize()
	if s == "" {
		t.Fatal("Serialize 不应返回空串")
	}
	got := ParseFormConfig(s)
	if got == nil {
		t.Fatal("ParseFormConfig 不应返回 nil")
	}
	if !got.Enabled || got.PreAuditURL != cfg.PreAuditURL {
		t.Errorf("基础字段 roundtrip 失败: %+v", got)
	}
	if got.FormFields[FieldDomainMedium] != "medium_sel[4]" {
		t.Errorf("FormFields roundtrip 失败: %+v", got.FormFields)
	}
	tags := got.ValueMappings[FieldDomainTags]
	if len(tags) != 2 {
		t.Fatalf("ValueMappings roundtrip 条数错: %d", len(tags))
	}
	if tags[0].Auto == nil || *tags[0].Auto != false {
		t.Errorf("英语标签 Auto 应为 *false（条件标签）: %+v", tags[0].Auto)
	}
	if tags[1].Auto != nil {
		t.Errorf("DV 标签 Auto 应为 nil（可自动）: %+v", tags[1].Auto)
	}
	if len(tags[1].StandardKeys) != 1 || tags[1].StandardKeys[0] != "tag.dolby_vision" {
		t.Errorf("StandardKeys roundtrip 失败: %+v", tags[1].StandardKeys)
	}
	// JSON 形态：auto 字段省略（omitempty）+ enabled 在
	if strings.Contains(s, `"auto":true`) {
		t.Errorf("可自动标签不应序列化 auto 字段: %s", s)
	}
	if !strings.Contains(s, `"enabled":true`) {
		t.Errorf("enabled 位应在 JSON: %s", s)
	}
}

func TestParseFormConfigEmpty(t *testing.T) {
	if ParseFormConfig("") != nil {
		t.Error("空串应返回 nil（未配置）")
	}
	if ParseFormConfig("not json") != nil {
		t.Error("非法 JSON 应返回 nil")
	}
	// 结构完整性：域常量与 JSON key 对齐（防手写漂移；零值 map 被 omitempty 省略——赋值后验）
	cfg2 := &PublishFormConfig{
		FormFields:    map[string]string{FieldDomainType: "type"},
		ValueMappings: map[string][]FormValueMapping{FieldDomainType: {{Label: "电影", Value: "401"}}},
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(cfg2.Serialize()), &raw); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"enabled", "form_fields", "value_mappings"} {
		if _, ok := raw[k]; !ok {
			t.Errorf("JSON 缺 key: %s", k)
		}
	}
}
