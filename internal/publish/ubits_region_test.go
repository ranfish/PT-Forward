package publish

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.494: 地区兜底站自适应锚——优堡 mappings 无 "28"（HDVideo 值）·
// fallbackByOtherLabel 按 label 词形命中"其它(Other)"桶。
func TestFallbackByOtherLabel(t *testing.T) {
	ubits := []model.FormValueMapping{
		{Label: "中国大陆", Value: "1"},
		{Label: "欧美", Value: "4"},
		{Label: "其它(Other)", Value: "11"},
	}
	if m := fallbackByValue(ubits, "28"); m != nil {
		t.Fatal("28 不在优堡值集·应 nil")
	}
	m := fallbackByOtherLabel(ubits)
	if m == nil || m.Value != "11" {
		t.Fatalf("其它桶: got %v want 11", m)
	}
	hd := []model.FormValueMapping{
		{Label: "其他地区", Value: "28"},
	}
	if m := fallbackByOtherLabel(hd); m == nil || m.Value != "28" {
		t.Fatalf("HDVideo 其他地区: got %v", m)
	}
	if fallbackByOtherLabel(nil) != nil {
		t.Fatal("nil mappings 应 nil")
	}
}
