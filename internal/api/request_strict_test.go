package api

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// §59.565 D3a 锚：decodeJSONBodyStrict 三态。
// （新接口启用·存量不一刀切——04 篇 D3 定案·本锚锁定变体契约）
func TestDecodeJSONBodyStrict(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}

	// ① 合法 body：单对象+已知字段 → true
	r := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"a"}`))
	w := httptest.NewRecorder()
	var p payload
	if !decodeJSONBodyStrict(w, r, &p) || p.Name != "a" {
		t.Fatalf("合法 body 应通过: code=%d body=%v", w.Code, p)
	}

	// ② 未知字段 → 400 拒绝（拼写错不再静默成功）
	r2 := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"a","typo_field":1}`))
	w2 := httptest.NewRecorder()
	var p2 payload
	if decodeJSONBodyStrict(w2, r2, &p2) {
		t.Fatal("未知字段应被拒绝（DisallowUnknownFields）")
	}
	if w2.Code != 400 {
		t.Fatalf("未知字段应 400, got %d", w2.Code)
	}

	// ③ 尾随多余 JSON 值 → 400 拒绝
	r3 := httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"a"} {"name":"b"}`))
	w3 := httptest.NewRecorder()
	var p3 payload
	if decodeJSONBodyStrict(w3, r3, &p3) {
		t.Fatal("尾随多余值应被拒绝（单值检查）")
	}
	if w3.Code != 400 {
		t.Fatalf("尾随值应 400, got %d", w3.Code)
	}
}
