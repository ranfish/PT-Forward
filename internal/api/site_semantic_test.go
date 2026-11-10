package api

import (
	"net/url"
	"net/http/httptest"
	"encoding/json"
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/ranfish/pt-forward/internal/crypto"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/publish"
	"go.uber.org/zap"
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
	unsupported := mk("未适配站", "piggo.me", "")                                      // 真未适配站（二师兄——longpt.org 已适配 §59.444 原占位失效）
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
// §59.318 附六: 导入的明文凭证必须加密落库（enc2:）——Updates(in) 绕过
// GORM 加密 callback 的回归锚。
func TestSiteImportDegradeWarning(t *testing.T) {
	env := setupTestEnv(t)
	enc, err := crypto.NewCredentialEncryptor("test-encryption-key-0123456789")
	if err != nil {
		t.Fatal(err)
	}
	if err := crypto.RegisterCallbacks(env.db, enc, zap.NewNop()); err != nil {
		t.Fatal(err)
	}
	mk := func(name, domain, cfg string) *model.Site {
		s := &model.Site{Name: name, Domain: domain, BaseURL: "https://" + domain,
			Framework: "nexusphp", AuthType: "cookie", Enabled: true, PublishFormConfig: cfg}
		if err := env.db.Create(s).Error; err != nil {
			t.Fatal(err)
		}
		return s
	}
	mk("未适配站", "piggo.me", "")                                      // 真未适配站（§59.444 原占位 longpt.org 已适配失效）
	mk("修道院", "xdypt.vip", `{"enabled":false,"form_fields":{"a":"b"}}`) // 适配

	const plainPasskey = "0123456789abcdef0123456789abcdef"
	w := env.doRequest("POST", "/api/v1/sites/import", map[string]interface{}{
		"sites": []map[string]interface{}{
			{"domain": "piggo.me", "is_target": true, "passkey": plainPasskey},
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
	if err := env.db.Where("domain = ?", "piggo.me").First(&unsupported2).Error; err != nil {
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
	// 附六：passkey 落库形态必须是 enc2: 密文，且可解密回明文
	var rawPasskey string
	if err := env.db.Raw("SELECT passkey FROM sites WHERE domain = 'piggo.me'").Scan(&rawPasskey).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(rawPasskey, "enc2:") {
		t.Errorf("passkey 应加密落库, got %q", rawPasskey)
	}
	if got, err := enc.Decrypt(rawPasskey); err != nil || got != plainPasskey {
		t.Errorf("passkey 密文应解密回明文, err=%v match=%v", err, got == plainPasskey)
	}
}

// §59.512: 幽灵站点导入复活防线——DB 潜伏幽灵行（supported miss）+
// 旧版本配置文件（制作时幽灵还在支持集·条目 enabled=true）→ 导入过滤
// 拒入（当前版本支持集为唯一权威·文件声称无效）+DB 行不被续命。
func TestSiteImportGhostFilter(t *testing.T) {
	env := setupTestEnv(t)
	enc, err := crypto.NewCredentialEncryptor("test-encryption-key-0123456789")
	if err != nil {
		t.Fatal(err)
	}
	if err := crypto.RegisterCallbacks(env.db, enc, zap.NewNop()); err != nil {
		t.Fatal(err)
	}
	// DB 潜伏幽灵行（历史窗口期 seed 重建形态·enabled=0 不可见潜伏）
	ghost := &model.Site{Name: "传道院", Domain: "pt.cdy.skin", BaseURL: "https://pt.cdy.skin",
		Framework: "nexusphp", AuthType: "cookie"}
	if err := env.db.Create(ghost).Error; err != nil {
		t.Fatal(err)
	}
	// GORM bool 零值坑：Create 跳过 false → 列默认 true（SeedSites 同坑
	// 有对策·测试直 Create 需显式置 false——243 幽灵行 enabled=1 的可能来源）
	if err := env.db.Model(ghost).Update("enabled", false).Error; err != nil {
		t.Fatal(err)
	}
	// 在支持集的对照行（正常站不受过滤影响）
	okRow := &model.Site{Name: "时光", Domain: "hdtime.org", BaseURL: "https://hdtime.org",
		Framework: "nexusphp", AuthType: "cookie", Enabled: false}
	if err := env.db.Create(okRow).Error; err != nil {
		t.Fatal(err)
	}
	// 旧版本文件：含幽灵条目（enabled=true 复活尝试）+正常条目
	w := env.doRequest("POST", "/api/v1/sites/import", map[string]interface{}{
		"sites": []map[string]interface{}{
			{"domain": "pt.cdy.skin", "enabled": true, "passkey": "0123456789abcdef0123456789abcdef"},
			{"domain": "hdtime.org", "enabled": true},
		},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("导入应 200, got %d: %s", w.Code, w.Body.String())
	}
	rd, _ := parseResponse(t, w).Data.(map[string]interface{})
	// 幽灵被过滤（skip+警告）·正常站 updated
	if rd["updated"] != float64(1) {
		t.Errorf("仅正常站应 updated=1, got %v", rd["updated"])
	}
	skipped, _ := rd["skipped"].([]interface{})
	if len(skipped) != 1 || skipped[0] != "pt.cdy.skin" {
		t.Errorf("幽灵应 skip, got %v", skipped)
	}
	warns, _ := rd["warnings"].([]interface{})
	if len(warns) != 1 || !strings.Contains(fmt.Sprint(warns[0]), "已下线") {
		t.Errorf("应 1 条下线警告, got %v", warns)
	}
	// DB 幽灵行不被续命：enabled 保持 0 + passkey 未被写入
	var ghost2 model.Site
	if err := env.db.Where("domain = ?", "pt.cdy.skin").First(&ghost2).Error; err != nil {
		t.Fatal("幽灵行应仍在（清理另行走删除·导入不删行）", err)
	}
	if ghost2.Enabled {
		t.Error("幽灵行 enabled 不应被导入刷为 true（复活拦截）")
	}
	if ghost2.Passkey != "" {
		t.Error("幽灵行 passkey 不应被写入（拒入含拒绝更新）")
	}
	// 正常站被更新
	var ok2 model.Site
	if err := env.db.Where("domain = ?", "hdtime.org").First(&ok2).Error; err != nil {
		t.Fatal(err)
	}
	if !ok2.Enabled {
		t.Error("正常站应被更新 enabled=true")
	}
}

// §59.509 附六: 批量补推后端判据锚——WHERE 与前端 canRepush 事实判据
// 单点一致（tid 在手且未做种）。历史案：前端已改事实判据、后端漏跟
// 仍 seed_error 旧判据 → duplicate 714 行全滤误拒（用户实测报 bug）。
func TestBatchRepushCriteriaAligned(t *testing.T) {
	env := setupTestEnv(t)
	if err := env.db.AutoMigrate(&model.PublishResultRecord{}); err != nil {
		t.Fatal(err)
	}
	mk := func(status, tid string, seeded bool, dlID uint, savePath string) *model.PublishResultRecord {
		return &model.PublishResultRecord{
			Status: model.PublishResultStatus(status), TorrentID: tid, Seeded: seeded,
			DownloaderID: dlID, SavePath: savePath,
			TargetSite: "瓷器", Title: "T-" + status + "-" + tid,
		}
	}
	rows := []*model.PublishResultRecord{
		mk("duplicate", "49104", false, 2, "/PT"),   // ✓ 应入（用户案主形态）
		mk("duplicate", "", false, 2, "/PT"),        // ✗ 无 tid
		mk("duplicate", "0", false, 2, "/PT"),       // ✗ tid='0'
		mk("duplicate", "49105", true, 2, "/PT"),    // ✗ 已做种
		mk("duplicate", "49106", false, 0, "/PT"),   // ✗ 无下载器
		mk("duplicate", "49107", false, 2, ""),      // ✗ 无保存路径
		mk("failed", "99001", false, 2, "/PT"),      // ✓ 应入（failed-but-tid 边缘行）
		mk("existing", "", false, 2, "/PT"),         // ✗ 无 tid（辅种范畴）
	}
	for _, r := range rows {
		if err := env.db.Create(r).Error; err != nil {
			t.Fatal(err)
		}
	}
	// executor 需在场（nil 在 WHERE 前被拦）——最小构造（Execute 会因
	// 素材缺失失败，但本锚验证 WHERE 过滤集，outcome failed 不影响）
	h := &PublishTorrentsHandler{
		db: env.db, logger: zap.NewNop(),
		executor: publish.NewPublishExecutor(publish.NewPipeline(env.db, zap.NewNop())),
	}
	ids := make([]uint, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	body, _ := json.Marshal(map[string]interface{}{"ids": ids})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/publish/seeds/batch-repush", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.handleBatchRepush(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("应 200（duplicate tid 在手行可入）, got %d: %s", w.Code, w.Body.String()[:200])
	}
	var resp struct {
		Data struct {
			Outcomes []struct {
				ID uint `json:"id"`
			} `json:"outcomes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Data.Outcomes) != 2 {
		t.Errorf("WHERE 应恰放行 2 行（duplicate 49104 + failed 99001），got %d", len(resp.Data.Outcomes))
	}
	got := map[uint]bool{}
	for _, o := range resp.Data.Outcomes {
		got[o.ID] = true
	}
	if !got[rows[0].ID] || !got[rows[6].ID] {
		t.Errorf("应含 duplicate 行 %d 与 failed-but-tid 行 %d，got %v", rows[0].ID, rows[6].ID, got)
	}
}

// §59.513: 批量发布 graceful 停止——cancel 端点幂等三态（§59.495 批量
// 获取同款语义：取消不再开始下一颗·当前颗完整执行完）。
func TestSiteBatchCancelIdempotent(t *testing.T) {
	env := setupTestEnv(t)
	h := &PublishTorrentsHandler{db: env.db, logger: zap.NewNop()}
	h.siteBatch.tasks = map[string]*siteBatchTask{}
	h.siteBatch.active = map[string]string{}
	doCancel := func(site string) (int, string) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/publish/seeds/site-batch-cancel?site="+url.QueryEscape(site), nil)
		w := httptest.NewRecorder()
		h.handleSiteBatchCancel(w, req)
		return w.Code, w.Body.String()
	}
	// ① 无任务：no-op 200
	code, body := doCancel("瓷器")
	if code != http.StatusOK || !strings.Contains(body, "无运行中任务") {
		t.Errorf("无任务应 no-op 200, got %d %s", code, body[:min(80, len(body))])
	}
	// ② 运行中：请求接受+stopRequested 置位
	task := &siteBatchTask{ID: "t1", TargetSite: "瓷器", Total: 10}
	h.siteBatch.tasks["t1"] = task
	h.siteBatch.active["瓷器"] = "t1"
	code, body = doCancel("瓷器")
	if code != http.StatusOK || !strings.Contains(body, "停止请求已接受") {
		t.Errorf("运行中应接受, got %d %s", code, body[:min(80, len(body))])
	}
	if !task.StopRequested {
		t.Error("stopRequested 未置位")
	}
	// ③ 幂等：再取消同一任务仍 200（已请求）
	code, _ = doCancel("瓷器")
	if code != http.StatusOK {
		t.Errorf("重复取消应幂等 200, got %d", code)
	}
	// ④ 已完成：告知终态（不复活）
	task.Finished = true
	delete(h.siteBatch.active, "瓷器")
	code, body = doCancel("瓷器")
	if code != http.StatusOK || !strings.Contains(body, "无运行中任务") {
		t.Errorf("已完成应 no-op, got %d", code)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
