package setting

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupRuntimeDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&Setting{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestNewRuntimeConfig(t *testing.T) {
	repo := NewRepository(setupRuntimeDB(t))
	rc := NewRuntimeConfig(repo, zap.NewNop())
	if rc == nil {
		t.Fatal("expected non-nil RuntimeConfig")
	}
	if rc.ttl != 30*time.Second {
		t.Errorf("default TTL should be 30s, got %v", rc.ttl)
	}
}

func TestRuntimeConfig_Reload(t *testing.T) {
	db := setupRuntimeDB(t)
	repo := NewRepository(db)
	rc := NewRuntimeConfig(repo, zap.NewNop())
	ctx := context.Background()

	repo.Set(ctx, "test_key", "test_value")

	if err := rc.Reload(ctx); err != nil {
		t.Fatalf("reload: %v", err)
	}

	got := rc.GetString(ctx, "test_key")
	if got != "test_value" {
		t.Errorf("expected test_value, got %q", got)
	}
}

func TestRuntimeConfig_GetString(t *testing.T) {
	db := setupRuntimeDB(t)
	repo := NewRepository(db)
	rc := NewRuntimeConfig(repo, zap.NewNop())
	ctx := context.Background()

	repo.Set(ctx, "str_key", "hello")
	rc.Reload(ctx)

	if v := rc.GetString(ctx, "str_key"); v != "hello" {
		t.Errorf("GetString: got %q, want %q", v, "hello")
	}
	if v := rc.GetString(ctx, "nonexistent"); v != "" {
		t.Errorf("nonexistent: got %q, want empty", v)
	}
}

func TestRuntimeConfig_GetInt(t *testing.T) {
	db := setupRuntimeDB(t)
	repo := NewRepository(db)
	rc := NewRuntimeConfig(repo, zap.NewNop())
	ctx := context.Background()

	repo.Set(ctx, "int_key", "42")
	repo.Set(ctx, "bad_int", "not-a-number")
	rc.Reload(ctx)

	if v := rc.GetInt(ctx, "int_key"); v != 42 {
		t.Errorf("GetInt: got %d, want 42", v)
	}
	if v := rc.GetInt(ctx, "bad_int"); v != 0 {
		t.Errorf("bad int: got %d, want 0", v)
	}
	if v := rc.GetInt(ctx, "nonexistent"); v != 0 {
		t.Errorf("nonexistent: got %d, want 0", v)
	}
}

func TestRuntimeConfig_GetBool(t *testing.T) {
	db := setupRuntimeDB(t)
	repo := NewRepository(db)
	rc := NewRuntimeConfig(repo, zap.NewNop())
	ctx := context.Background()

	repo.Set(ctx, "bool_true", "true")
	repo.Set(ctx, "bool_one", "1")
	repo.Set(ctx, "bool_false", "false")
	repo.Set(ctx, "bool_zero", "0")
	repo.Set(ctx, "bool_random", "yes")
	rc.Reload(ctx)

	if !rc.GetBool(ctx, "bool_true") {
		t.Error("true should be true")
	}
	if !rc.GetBool(ctx, "bool_one") {
		t.Error("1 should be true")
	}
	if rc.GetBool(ctx, "bool_false") {
		t.Error("false should be false")
	}
	if rc.GetBool(ctx, "bool_zero") {
		t.Error("0 should be false")
	}
	if rc.GetBool(ctx, "bool_random") {
		t.Error("yes should be false")
	}
}

func TestRuntimeConfig_CacheAutoRefresh(t *testing.T) {
	db := setupRuntimeDB(t)
	repo := NewRepository(db)
	rc := NewRuntimeConfig(repo, zap.NewNop())
	rc.ttl = 10 * time.Millisecond
	ctx := context.Background()

	repo.Set(ctx, "cached_key", "v1")
	rc.Reload(ctx)

	if v := rc.GetString(ctx, "cached_key"); v != "v1" {
		t.Errorf("initial: got %q", v)
	}

	repo.Set(ctx, "cached_key", "v2")

	time.Sleep(15 * time.Millisecond)

	if v := rc.GetString(ctx, "cached_key"); v != "v2" {
		t.Errorf("after TTL: got %q, want v2", v)
	}
}

func TestRuntimeConfig_GetBeforeReload(t *testing.T) {
	db := setupRuntimeDB(t)
	repo := NewRepository(db)
	rc := NewRuntimeConfig(repo, zap.NewNop())
	ctx := context.Background()

	repo.Set(ctx, "lazy_key", "lazy_value")

	got := rc.GetString(ctx, "lazy_key")
	if got != "lazy_value" {
		t.Errorf("auto-load on miss: got %q, want lazy_value", got)
	}
}

func TestSeedDefaults(t *testing.T) {
	db := setupRuntimeDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	seeds := map[string]string{
		"seed_a": "value_a",
		"seed_b": "value_b",
	}

	SeedDefaults(ctx, repo, seeds, zap.NewNop())

	v, err := repo.Get(ctx, "seed_a")
	if err != nil || v != "value_a" {
		t.Errorf("seed_a: got %q, err %v", v, err)
	}

	repo.Set(ctx, "seed_a", "overwritten")
	SeedDefaults(ctx, repo, seeds, zap.NewNop())

	v, _ = repo.Get(ctx, "seed_a")
	if v != "overwritten" {
		t.Errorf("existing value should not be overwritten, got %q", v)
	}
}

func TestDefaultSeeds_Values(t *testing.T) {
	if DefaultSeeds[KeyLoginLockoutEnabled] != "false" {
		t.Errorf("KeyLoginLockoutEnabled default should be false")
	}
	if DefaultSeeds[KeyRateLimitGlobal] != "600" {
		t.Errorf("KeyRateLimitGlobal default should be 600")
	}
	if DefaultSeeds[KeyScreenshotCount] != "6" {
		t.Errorf("KeyScreenshotCount default should be 6")
	}
	if DefaultSeeds[KeyDataCleanupPTGenCacheDays] != "90" {
		t.Errorf("KeyDataCleanupPTGenCacheDays default should be 90")
	}
}

func TestRuntimeConfig_ConcurrentAccess(t *testing.T) {
	db := setupRuntimeDB(t)
	repo := NewRepository(db)
	rc := NewRuntimeConfig(repo, zap.NewNop())
	ctx := context.Background()

	repo.Set(ctx, "concurrent_key", "initial")
	rc.Reload(ctx)

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				rc.GetString(ctx, "concurrent_key")
				rc.GetInt(ctx, "concurrent_key")
				rc.GetBool(ctx, "concurrent_key")
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
