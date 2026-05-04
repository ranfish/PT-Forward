package site

import (
	"context"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&model.Site{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestRepository_CreateAndGet(t *testing.T) {
	repo := NewRepository(setupTestDB(t))
	ctx := context.Background()

	site := &model.Site{
		Domain:    "example.com",
		Name:      "Example",
		BaseURL:   "https://example.com",
		Framework: "nexusphp",
		Enabled:   true,
	}
	if err := repo.Create(ctx, site); err != nil {
		t.Fatal(err)
	}
	if site.ID == 0 {
		t.Fatal("ID should be set after create")
	}

	got, err := repo.GetByID(ctx, site.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Example" {
		t.Errorf("expected Example, got %s", got.Name)
	}
}

func TestRepository_GetByDomain(t *testing.T) {
	repo := NewRepository(setupTestDB(t))
	ctx := context.Background()

	repo.Create(ctx, &model.Site{Domain: "test.com", Name: "Test", BaseURL: "https://test.com", Framework: "generic"})

	got, err := repo.GetByDomain(ctx, "test.com")
	if err != nil {
		t.Fatal(err)
	}
	if got.Domain != "test.com" {
		t.Errorf("expected test.com, got %s", got.Domain)
	}
}

func TestRepository_GetByName(t *testing.T) {
	repo := NewRepository(setupTestDB(t))
	ctx := context.Background()

	repo.Create(ctx, &model.Site{Domain: "site.net", Name: "MySite", BaseURL: "https://site.net", Framework: "nexusphp"})

	got, err := repo.GetByName(ctx, "MySite")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "MySite" {
		t.Errorf("expected MySite, got %s", got.Name)
	}
}

func TestRepository_List(t *testing.T) {
	repo := NewRepository(setupTestDB(t))
	ctx := context.Background()

	repo.Create(ctx, &model.Site{Domain: "a.com", Name: "Alpha", BaseURL: "https://a.com", Framework: "generic"})
	repo.Create(ctx, &model.Site{Domain: "b.com", Name: "Bravo", BaseURL: "https://b.com", Framework: "nexusphp"})

	sites, err := repo.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 2 {
		t.Fatalf("expected 2 sites, got %d", len(sites))
	}
}

func TestRepository_Update(t *testing.T) {
	repo := NewRepository(setupTestDB(t))
	ctx := context.Background()

	site := &model.Site{Domain: "u.com", Name: "Up", BaseURL: "https://u.com", Framework: "generic"}
	repo.Create(ctx, site)

	site.Framework = "unit3d"
	repo.Update(ctx, site)

	got, _ := repo.GetByID(ctx, site.ID)
	if got.Framework != "unit3d" {
		t.Errorf("expected unit3d, got %s", got.Framework)
	}
}

func TestRepository_Delete(t *testing.T) {
	repo := NewRepository(setupTestDB(t))
	ctx := context.Background()

	site := &model.Site{Domain: "d.com", Name: "Del", BaseURL: "https://d.com", Framework: "generic"}
	repo.Create(ctx, site)

	repo.Delete(ctx, site.ID)

	_, err := repo.GetByID(ctx, site.ID)
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestRepository_ExistsByDomain(t *testing.T) {
	repo := NewRepository(setupTestDB(t))
	ctx := context.Background()

	repo.Create(ctx, &model.Site{Domain: "exists.com", Name: "Exists", BaseURL: "https://exists.com", Framework: "generic"})

	exists, _ := repo.ExistsByDomain(ctx, "exists.com", 0)
	if !exists {
		t.Error("should exist")
	}

	exists2, _ := repo.ExistsByDomain(ctx, "nope.com", 0)
	if exists2 {
		t.Error("should not exist")
	}
}

func TestRepository_ExistsByDomain_ExcludeID(t *testing.T) {
	repo := NewRepository(setupTestDB(t))
	ctx := context.Background()

	site := &model.Site{Domain: "exc.com", Name: "Exc", BaseURL: "https://exc.com", Framework: "generic"}
	repo.Create(ctx, site)

	exists, _ := repo.ExistsByDomain(ctx, "exc.com", site.ID)
	if exists {
		t.Error("should not exist when excluded by own ID")
	}
}

func TestRepository_ExistsByName(t *testing.T) {
	repo := NewRepository(setupTestDB(t))
	ctx := context.Background()

	repo.Create(ctx, &model.Site{Domain: "n.com", Name: "ByName", BaseURL: "https://n.com", Framework: "generic"})

	exists, _ := repo.ExistsByName(ctx, "ByName", 0)
	if !exists {
		t.Error("should exist")
	}

	exists2, _ := repo.ExistsByName(ctx, "Missing", 0)
	if exists2 {
		t.Error("should not exist")
	}
}

func TestRepository_UpdateCredentials(t *testing.T) {
	repo := NewRepository(setupTestDB(t))
	ctx := context.Background()

	site := &model.Site{Domain: "c.com", Name: "Creds", BaseURL: "https://c.com", Framework: "nexusphp"}
	repo.Create(ctx, site)

	err := repo.UpdateCredentials(ctx, site.ID, map[string]interface{}{
		"passkey": "new_pk_123",
	})
	if err != nil {
		t.Fatal(err)
	}

	got, _ := repo.GetByID(ctx, site.ID)
	if got.Passkey != "new_pk_123" {
		t.Errorf("expected new_pk_123, got %s", got.Passkey)
	}
}

func TestRepository_UpdateStats(t *testing.T) {
	repo := NewRepository(setupTestDB(t))
	ctx := context.Background()

	site := &model.Site{Domain: "s.com", Name: "Stats", BaseURL: "https://s.com", Framework: "generic"}
	repo.Create(ctx, site)

	err := repo.UpdateStats(ctx, site.ID, map[string]interface{}{
		"upload_bytes":  int64(123456),
		"seeding_count": 42,
	})
	if err != nil {
		t.Fatal(err)
	}

	got, _ := repo.GetByID(ctx, site.ID)
	if got.UploadBytes != 123456 {
		t.Errorf("expected 123456, got %d", got.UploadBytes)
	}
	if got.SeedingCount != 42 {
		t.Errorf("expected 42, got %d", got.SeedingCount)
	}
}
