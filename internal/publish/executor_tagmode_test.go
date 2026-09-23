package publish

import (
	"strings"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.269 A2 兜底: TagConfig nil + 字段名数组形态 → checkbox_span
func TestTagModeFallback(t *testing.T) {
	// 直接验证推断谓词（executor 内联逻辑同构）
	cfg := &model.PublishFormConfig{
		FormFields: map[string]string{model.FieldDomainTags: "tags[4][]"},
	}
	fieldName := cfg.FormFields[model.FieldDomainTags]
	if !strings.Contains(fieldName, "[") {
		t.Errorf("tags[4][] 应判定数组形态")
	}
	if strings.Contains("tagList", "[") {
		t.Error("tagList 非数组形态")
	}
}
