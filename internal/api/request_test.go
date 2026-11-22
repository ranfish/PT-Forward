package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// §59.553 契约锁定：Decode 失败统一 40001「请求格式错误」·成功透传解码。
func TestDecodeJSONBody(t *testing.T) {
	var got struct {
		Name string `json:"name"`
	}

	// ① 正常 JSON
	r := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"name":"pt"}`))
	w := httptest.NewRecorder()
	if !decodeJSONBody(w, r, &got) {
		t.Fatal("① 正常 JSON 应返回 true")
	}
	if got.Name != "pt" {
		t.Fatalf("① 解码透传失败: %q", got.Name)
	}

	// ② 非法 JSON
	r = httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(`{invalid`))
	w = httptest.NewRecorder()
	if decodeJSONBody(w, r, &got) {
		t.Fatal("② 非法 JSON 应返回 false")
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("② 应 400·got %d", w.Code)
	}

	// ③ 空 body（EOF）
	r = httptest.NewRequest(http.MethodPost, "/test", http.NoBody)
	w = httptest.NewRecorder()
	if decodeJSONBody(w, r, &got) {
		t.Fatal("③ 空 body 应返回 false")
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("③ 应 400·got %d", w.Code)
	}
}
