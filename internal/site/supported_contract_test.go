package site

import (
	"encoding/json"
	"strings"
	"testing"
)

// 14 刀2（v0.0.1659）: PublishSupported 零值序列化契约锚（V-02）——
// false 是有意义值（未适配站是常态）·键恒存在。
// 原实现 bool+omitempty：false 时键从 JSON 消失——前端 types.ts
// 声明 boolean（非 optional）契约撒谎（实测 {"show_cookie":false}
// 而无 publish_supported 键）。
//
// 阴性对照：恢复 omitempty → 锚1 FAIL（键消失）。
func TestSupportedSite_PublishSupportedZeroValueSerialized(t *testing.T) {
	// 锚1: 零值 false → 键存在且值为 false
	s := &SupportedSite{Domain: "x.example.com"}
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	out := string(b)
	if !strings.Contains(out, `"publish_supported":false`) {
		t.Errorf("零值 false 应序列化为键存在：实际 %s——omitempty 回归（V-02）", out)
	}

	// 锚2: true → 键存在且值为 true（既有行为不破坏）
	s.PublishSupported = true
	b2, _ := json.Marshal(s)
	if !strings.Contains(string(b2), `"publish_supported":true`) {
		t.Errorf("true 应序列化存在：实际 %s", b2)
	}

	// 锚3: 反序列化往返（supported_sites.json 读方向不受影响——
	// 带/不带键均合法·不带时 Go 零值 false）
	for _, in := range []string{
		`{"domain":"a.com","publish_supported":true}`,
		`{"domain":"b.com","publish_supported":false}`,
		`{"domain":"c.com"}`,
	} {
		var v SupportedSite
		if err := json.Unmarshal([]byte(in), &v); err != nil {
			t.Errorf("unmarshal %s: %v", in, err)
		}
	}
}
