package api

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"

)

// §59.318 D3+D7: isTarget PUT 三重门槛——未适配拒绝/适配未配置拒绝(引导)/
// 适配已配置开启=form_config.enabled 翻转+is_target 同步。
func TestSiteIsTargetGate(t *testing.T) {
	env := setupTestEnv(t)
	mk := func(name, domain, cfg string) *model.Site {
		s := &model.Site{Name: name, Domain: domain, BaseURL: "https://" + domain,
			Framework: "nexusphp", AuthType: "cookie", Enabled: true, PublishFormConfig: cfg}
		if err := env.db.Create(s).Error; err != nil {
			t.Fatal(err)
		}
		return s
	}
	unsupported := mk("未适配站", "longpt.org", "")                                     // 白名单内但未适配
	unconfigured := mk("幸运", "pt.luckpt.de", "")                                      // 适配但未配置
	configured := mk("修道院", "xdypt.vip", `{"enabled":false,"form_fields":{"a":"b"}}`) // 适配已配置(暂停)

	// ①未适配站开启 → 403
	w := env.doRequest("PUT", fmt.Sprintf("/api/v1/sites/%d", unsupported.ID), map[string]interface{}{"isTarget": true})
	if w.Code != http.StatusForbidden {
		t.Fatalf("未适配站应 403, got %d: %s", w.Code, w.Body.String())
	}
	// ②适配未配置开启 → 400（引导完成配置）
	w = env.doRequest("PUT", fmt.Sprintf("/api/v1/sites/%d", unconfigured.ID), map[string]interface{}{"isTarget": true})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("适配未配置应 400, got %d: %s", w.Code, w.Body.String())
	}
	// ③适配已配置开启 → 200；form_config.enabled 翻转 + is_target 同步
	w = env.doRequest("PUT", fmt.Sprintf("/api/v1/sites/%d", configured.ID), map[string]interface{}{"isTarget": true})
	if w.Code != http.StatusOK {
		t.Fatalf("适配已配置应 200, got %d: %s", w.Code, w.Body.String())
	}
	w = env.doRequest("GET", fmt.Sprintf("/api/v1/sites/%d", configured.ID), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("回读失败: %d", w.Code)
	}
	rd, _ := parseResponse(t, w).Data.(map[string]interface{})
	if rd["isTarget"] != true {
		t.Error("is_target 应同步为 true")
	}
	// ④关闭 → 双双复位（form_fields 保真）
	w = env.doRequest("PUT", fmt.Sprintf("/api/v1/sites/%d", configured.ID), map[string]interface{}{"isTarget": false})
	if w.Code != http.StatusOK {
		t.Fatalf("关闭应 200, got %d", w.Code)
	}
	w = env.doRequest("GET", fmt.Sprintf("/api/v1/sites/%d", configured.ID), nil)
	rd, _ = parseResponse(t, w).Data.(map[string]interface{})
	if rd["isTarget"] != false {
		t.Error("is_target 应回 false")
	}
	// ⑤isReseedTarget 自由开关（无门槛）
	w = env.doRequest("PUT", fmt.Sprintf("/api/v1/sites/%d", unsupported.ID), map[string]interface{}{"isReseedTarget": true})
	if w.Code != http.StatusOK {
		t.Fatalf("辅种探测开关应自由, got %d: %s", w.Code, w.Body.String())
	}
	w = env.doRequest("GET", fmt.Sprintf("/api/v1/sites/%d", unsupported.ID), nil)
	rd, _ = parseResponse(t, w).Data.(map[string]interface{})
	if rd["isReseedTarget"] != true {
		t.Error("is_reseed_target 应已置 true")
	}
}

// §59.318 附四: 导入降级归 warnings——未适配发布站携带 is_target=true 降为 false
// 并出警告（非 errors）；适配站 is_target=true 原样通过。
func TestSiteImportDegradeWarning(t *testing.T) {
	env := setupTestEnv(t)
	mk := func(name, domain, cfg string) *model.Site {
		s := &model.Site{Name: name, Domain: domain, BaseURL: "https://" + domain,
			Framework: "nexusphp", AuthType: "cookie", Enabled: true, PublishFormConfig: cfg}
		if err := env.db.Create(s).Error; err != nil {
			t.Fatal(err)
		}
		return s
	}
	mk("未适配站", "longpt.org", "")                                     // 白名单内但未适配
	mk("修道院", "xdypt.vip", `{"enabled":false,"form_fields":{"a":"b"}}`) // 适配

	w := env.doRequest("POST", "/api/v1/sites/import", map[string]interface{}{
		"sites": []map[string]interface{}{
			{"domain": "longpt.org", "is_target": true},
			{"domain": "xdypt.vip", "is_target": true},
		},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("导入应 200, got %d: %s", w.Code, w.Body.String())
	}
	rd, _ := parseResponse(t, w).Data.(map[string]interface{})
	if rd["updated"] != float64(2) {
		t.Errorf("应更新 2 站, got %v", rd["updated"])
	}
	warns, _ := rd["warnings"].([]interface{})
	if len(warns) != 1 {
		t.Errorf("应恰 1 条降级警告, got %v", warns)
	}
	errs, _ := rd["errors"].([]interface{})
	if len(errs) != 0 {
		t.Errorf("降级不应计入 errors, got %v", errs)
	}
	var unsupported2, configured2 model.Site
	if err := env.db.Where("domain = ?", "longpt.org").First(&unsupported2).Error; err != nil {
		t.Fatal(err)
	}
	if unsupported2.IsTarget {
		t.Error("未适配站导入后 is_target 应降级为 false")
	}
	if err := env.db.Where("domain = ?", "xdypt.vip").First(&configured2).Error; err != nil {
		t.Fatal(err)
	}
	if !configured2.IsTarget {
		t.Error("适配站导入后 is_target 应保持 true")
	}
}
