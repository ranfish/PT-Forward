package api

import (
	"context"
	"strings"
	"testing"

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
