package setting

import (
	"context"
	"testing"

	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// §59.568 刀4 锚：GetIntDefault 六态。
func newRuntimeTestDB(t *testing.T) (*RuntimeConfig, *Repository) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:settingtest_intdefault?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&Setting{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := NewRepository(db)
	rc := NewRuntimeConfig(repo, zap.NewNop())
	return rc, repo
}

func TestGetIntDefault(t *testing.T) {
	rc, repo := newRuntimeTestDB(t)
	ctx := context.Background()

	// ① 缺省 → def
	if got := rc.GetIntDefault(ctx, "test_missing", 42, 1, 100); got != 42 {
		t.Errorf("missing key: got %d, want 42", got)
	}
	// ② 合法值透传（Set 后主动失效缓存——30s TTL 等不起）
	_ = repo.Set(ctx, "test_valid", "77")
	InvalidateAll()
	if got := rc.GetIntDefault(ctx, "test_valid", 42, 1, 100); got != 77 {
		t.Errorf("valid value: got %d, want 77", got)
	}
	// ③ 解析失败 → def（GetInt 会静默 0）
	_ = repo.Set(ctx, "test_bad", "not_a_number")
	InvalidateAll()
	if got := rc.GetIntDefault(ctx, "test_bad", 42, 1, 100); got != 42 {
		t.Errorf("parse failure: got %d, want 42", got)
	}
	// ④ 下界越界 → def
	_ = repo.Set(ctx, "test_low", "0")
	InvalidateAll()
	if got := rc.GetIntDefault(ctx, "test_low", 42, 1, 100); got != 42 {
		t.Errorf("below min: got %d, want 42", got)
	}
	// ⑤ 上界越界 → def
	_ = repo.Set(ctx, "test_high", "9999")
	InvalidateAll()
	if got := rc.GetIntDefault(ctx, "test_high", 42, 1, 100); got != 42 {
		t.Errorf("above max: got %d, want 42", got)
	}
	// ⑥ max=0 表示无上界（哨兵语义——仅下界约束）
	_ = repo.Set(ctx, "test_nomax", "999999")
	InvalidateAll()
	if got := rc.GetIntDefault(ctx, "test_nomax", 42, 1, 0); got != 999999 {
		t.Errorf("no max: got %d, want 999999", got)
	}
}
