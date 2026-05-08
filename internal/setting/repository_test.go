package setting

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&Setting{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestRepository_SetAndGet(t *testing.T) {
	repo := NewRepository(setupTestDB(t))
	ctx := context.Background()

	if err := repo.Set(ctx, "test_key", "test_value"); err != nil {
		t.Fatal(err)
	}

	val, err := repo.Get(ctx, "test_key")
	if err != nil {
		t.Fatal(err)
	}
	if val != "test_value" {
		t.Errorf("expected test_value, got %s", val)
	}
}

func TestRepository_Get_NotFound(t *testing.T) {
	repo := NewRepository(setupTestDB(t))
	ctx := context.Background()

	_, err := repo.Get(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent key")
	}
}

func TestRepository_Delete(t *testing.T) {
	repo := NewRepository(setupTestDB(t))
	ctx := context.Background()

	if err := repo.Set(ctx, "to_delete", "value"); err != nil {
		t.Fatal(err)
	}
	if err := repo.Delete(ctx, "to_delete"); err != nil {
		t.Fatal(err)
	}

	_, err := repo.Get(ctx, "to_delete")
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestRepository_Set_Overwrite(t *testing.T) {
	repo := NewRepository(setupTestDB(t))
	ctx := context.Background()

	if err := repo.Set(ctx, "key", "v1"); err != nil {
		t.Fatal(err)
	}
	if err := repo.Set(ctx, "key", "v2"); err != nil {
		t.Fatal(err)
	}

	val, _ := repo.Get(ctx, "key")
	if val != "v2" {
		t.Errorf("expected v2, got %s", val)
	}
}

func TestRepository_ListByPrefix(t *testing.T) {
	repo := NewRepository(setupTestDB(t))
	ctx := context.Background()

	if err := repo.Set(ctx, "app.name", "PT-Forward"); err != nil {
		t.Fatal(err)
	}
	if err := repo.Set(ctx, "app.version", "1.0"); err != nil {
		t.Fatal(err)
	}
	if err := repo.Set(ctx, "other.key", "value"); err != nil {
		t.Fatal(err)
	}

	result, err := repo.ListByPrefix(ctx, "app.")
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2, got %d", len(result))
	}
	if result["app.name"] != "PT-Forward" {
		t.Errorf("expected PT-Forward, got %s", result["app.name"])
	}
	if result["app.version"] != "1.0" {
		t.Errorf("expected 1.0, got %s", result["app.version"])
	}
}

func TestRepository_ListByPrefix_EmptyPrefix(t *testing.T) {
	repo := NewRepository(setupTestDB(t))
	ctx := context.Background()

	if err := repo.Set(ctx, "a", "1"); err != nil {
		t.Fatal(err)
	}
	if err := repo.Set(ctx, "b", "2"); err != nil {
		t.Fatal(err)
	}

	result, err := repo.ListByPrefix(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2, got %d", len(result))
	}
}

func TestRepository_ListAll(t *testing.T) {
	repo := NewRepository(setupTestDB(t))
	ctx := context.Background()

	if err := repo.Set(ctx, "k1", "v1"); err != nil {
		t.Fatal(err)
	}
	if err := repo.Set(ctx, "k2", "v2"); err != nil {
		t.Fatal(err)
	}

	result, err := repo.ListAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2, got %d", len(result))
	}
}

func TestRepository_RestoreAll(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	if err := repo.Set(ctx, "old_key", "old_value"); err != nil {
		t.Fatal(err)
	}

	restoreData := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}
	if err := repo.RestoreAll(ctx, restoreData); err != nil {
		t.Fatal(err)
	}

	got, err := repo.ListAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 after restore, got %d", len(got))
	}
	if got["key1"] != "value1" {
		t.Errorf("expected value1, got %s", got["key1"])
	}
	if _, ok := got["old_key"]; ok {
		t.Error("old_key should be deleted")
	}
}

func TestAutoMigrate(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	if !db.Migrator().HasTable("system_settings") {
		t.Error("expected system_settings table")
	}
}
