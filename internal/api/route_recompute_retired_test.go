package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

// §59.556 锚: 路由分发修正双态锁定——
// ① recompute-profiles 同步版路由退役（11-11 §59.539 异步化改造的残骸·
//    空壳 case 曾致「命中但空响应」）→ 现应 404（调用方立即发现用错端点·
//    正主是 recompute-profiles-task）
// ② fix-stale-dolby 单响应（双 handler 串联曾致 superfluous WriteHeader
//    风险+全量 recompute 负载）→ 响应体应为 Dolby 清洗结构且唯一。
func TestRoute_RecomputeProfilesRetired(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest(http.MethodPost, "/api/v1/publish/seeds/recompute-profiles", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("退役路由应 404·got %d（空响应壳复活？）", w.Code)
	}
	// 正主 task 版仍健康
	w2 := env.doRequest(http.MethodPost, "/api/v1/publish/seeds/recompute-profiles-task", nil)
	if w2.Code == http.StatusNotFound {
		t.Fatal("task 版路由不应 404")
	}
}

func TestRoute_FixStaleDolbySingleResponse(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest(http.MethodPost, "/api/v1/publish/seeds/fix-stale-dolby", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("应 200·got %d·body=%s", w.Code, w.Body.String()[:min(200, w.Body.Len())])
	}
	var resp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Scanned   int `json:"scanned"`
			Cleaned   int `json:"cleaned"`
			Reinfer   int `json:"reinfer_scheduled"`
			Skipped   int `json:"skipped_incomplete"`
			KeptByInf int `json:"kept_by_infer"`
		} `json:"data"`
	}
	body := w.Body.Bytes()
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("响应应可解析为单 JSON 对象（双写污染?）: %v", err)
	}
	if resp.Data.Scanned == 0 && resp.Data.Cleaned == 0 && resp.Message == "" {
		t.Fatalf("应为 Dolby 清洗结构: %s", body[:min(200, len(body))])
	}
	// 单 JSON 断言：decoder 第二次解码应 EOF（无第二响应体片段）
	dec := json.NewDecoder(w.Body)
	var first json.RawMessage
	if err := dec.Decode(&first); err != nil {
		t.Fatalf("首次解码失败: %v", err)
	}
	if err := dec.Decode(new(json.RawMessage)); err == nil {
		t.Fatal("检测到第二个 JSON 对象（双 handler 双写）")
	}
}
