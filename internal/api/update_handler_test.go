package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/setting"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func newUpdateTestHandler(t *testing.T) (*SystemHandler, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&setting.Setting{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return NewSystemHandler("v0.0.0-test", db, nil, zap.NewNop()), db
}

// §59.367 附四: GitHub API 请求构造单点行为锚——token 注入语义。
// 背景：§59.354 曾在 handleCheckUpdate 单点注入、handleUpdate 副本漏注
// （"修一漏一"双副本漂移致全环境 OTA 403）——收敛单点后此锚保证
// 注入行为本身不再回归。
func TestNewGitHubAPIRequestTokenInjection(t *testing.T) {
	h, db := newUpdateTestHandler(t)
	ctx := context.Background()

	// ① 未配置 token——无 Authorization 头
	req, err := h.newGitHubAPIRequest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := req.Header.Get("Authorization"); got != "" {
		t.Fatalf("未配置 token 不应带 Authorization，got %q", got)
	}
	if got := req.Header.Get("Accept"); got != "application/vnd.github+json" {
		t.Fatalf("Accept 头异常: %q", got)
	}

	// ② 配置 token——Bearer 注入（checkUpdate/update 两链同源唯一注入点）
	if err := db.Exec("INSERT INTO system_settings (`key`, value) VALUES ('github_token', 'ghp_test123')").Error; err != nil {
		t.Fatal(err)
	}
	req2, err := h.newGitHubAPIRequest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got := req2.Header.Get("Authorization")
	if !strings.HasPrefix(got, "Bearer ghp_test123") {
		t.Fatalf("token 应以 Bearer 注入，got %q", got)
	}

	// ③ 空串 token——不注入（"留空=保持已配置"是前端语义，这里测后端
	// 对显式空串的防御）
	db.Exec("UPDATE system_settings SET value = '' WHERE `key` = 'github_token'")
	req3, _ := h.newGitHubAPIRequest(ctx)
	if got := req3.Header.Get("Authorization"); got != "" {
		t.Fatalf("空 token 不应注入，got %q", got)
	}
}

// §59.386: fresh=1 绕过检查缓存行为锚。背景：v0.0.1234 发布后 6 分钟内
// 全环境"发现不了更新"——发布前 ~10 分钟的成功检查缓存了旧 latest，
// ota.sh 以 current==latest 短路跳过更新。三态锚：
// ① 缓存有效时普通请求命中缓存（不打网络）② fresh=1 绕缓存拿实时值
// ③ fresh 成功后回写缓存（后续普通请求吃到新值）。
func TestCheckUpdateFreshBypassesCache(t *testing.T) {
	h, _ := newUpdateTestHandler(t)

	// 假 GitHub：返回固定 release（v0.0.2000 + amd64 资产）
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v0.0.2000","prerelease":false,` +
			`"assets":[{"name":"pt-forward-linux-amd64","browser_download_url":"http://example.com/b"}]}`))
	}))
	defer srv.Close()
	orig := githubAPI
	githubAPI = srv.URL
	defer func() { githubAPI = orig }()

	// 预填缓存（假旧值 v9.9.9-陈旧——模拟发布前写入）
	h.checkCacheMu.Lock()
	h.checkCache = map[string]interface{}{
		"has_update": false, "current_version": h.version,
		"latest_version": "v9.9.9-stale", "download_url": "http://example.com/old",
	}
	h.checkCacheAt = time.Now()
	h.checkCacheMu.Unlock()

	// ① 普通请求 → 命中缓存旧值
	w1 := httptest.NewRecorder()
	h.handleCheckUpdate(w1, httptest.NewRequest("GET", "/check-update", nil))
	var r1 map[string]interface{}
	if err := json.Unmarshal(w1.Body.Bytes(), &r1); err != nil {
		t.Fatal(err)
	}
	if got := r1["data"].(map[string]interface{})["latest_version"]; got != "v9.9.9-stale" {
		t.Fatalf("① 普通请求应命中缓存旧值，got %v", got)
	}

	// ② fresh=1 → 绕缓存拿实时值
	w2 := httptest.NewRecorder()
	h.handleCheckUpdate(w2, httptest.NewRequest("GET", "/check-update?fresh=1", nil))
	var r2 map[string]interface{}
	if err := json.Unmarshal(w2.Body.Bytes(), &r2); err != nil {
		t.Fatal(err)
	}
	if got := r2["data"].(map[string]interface{})["latest_version"]; got != "v0.0.2000" {
		t.Fatalf("② fresh 请求应返回实时值，got %v", got)
	}
	if got := r2["data"].(map[string]interface{})["has_update"]; got != true {
		t.Fatalf("② 实时值应判 has_update=true，got %v", got)
	}

	// ③ fresh 后缓存已回写 → 普通请求返回新值
	w3 := httptest.NewRecorder()
	h.handleCheckUpdate(w3, httptest.NewRequest("GET", "/check-update", nil))
	var r3 map[string]interface{}
	if err := json.Unmarshal(w3.Body.Bytes(), &r3); err != nil {
		t.Fatal(err)
	}
	if got := r3["data"].(map[string]interface{})["latest_version"]; got != "v0.0.2000" {
		t.Fatalf("③ fresh 后缓存应回写新值，got %v", got)
	}
}
