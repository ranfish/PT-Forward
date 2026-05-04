package rss

import (
	"context"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupRepoTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&model.RSSSubscription{}, &model.RSSTorrentSeen{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestRepository_CreateAndGet(t *testing.T) {
	repo := NewRepository(setupRepoTestDB(t))
	ctx := context.Background()

	sub := &model.RSSSubscription{
		Name:     "test-sub",
		SiteName: "example",
		URLs:     []string{"https://example.com/rss"},
		Cron:     "*/5 * * * *",
		Enabled:  true,
	}
	if err := repo.Create(ctx, sub); err != nil {
		t.Fatal(err)
	}
	if sub.ID == 0 {
		t.Fatal("ID should be set")
	}

	got, err := repo.GetByID(ctx, sub.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "test-sub" {
		t.Errorf("expected test-sub, got %s", got.Name)
	}
}

func TestRepository_SoftDelete(t *testing.T) {
	repo := NewRepository(setupRepoTestDB(t))
	ctx := context.Background()

	sub := &model.RSSSubscription{Name: "del-sub", SiteName: "s", URLs: []string{"https://x.com/rss"}}
	repo.Create(ctx, sub)

	repo.Delete(ctx, sub.ID)

	_, err := repo.GetByID(ctx, sub.ID)
	if err == nil {
		t.Fatal("expected error after soft delete")
	}
}

func TestRepository_List(t *testing.T) {
	repo := NewRepository(setupRepoTestDB(t))
	ctx := context.Background()

	repo.Create(ctx, &model.RSSSubscription{Name: "b-sub", SiteName: "s1", URLs: []string{"https://x.com/rss"}})
	repo.Create(ctx, &model.RSSSubscription{Name: "a-sub", SiteName: "s2", URLs: []string{"https://y.com/rss"}})

	subs, err := repo.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 2 {
		t.Fatalf("expected 2, got %d", len(subs))
	}
	if subs[0].Name != "a-sub" {
		t.Errorf("expected alphabetical order, first is %s", subs[0].Name)
	}
}

func TestRepository_ListActive(t *testing.T) {
	repo := NewRepository(setupRepoTestDB(t))
	ctx := context.Background()

	repo.Create(ctx, &model.RSSSubscription{Name: "active", SiteName: "s", URLs: []string{"https://x.com/rss"}, Enabled: true, Paused: false})

	subs, err := repo.ListActive(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 1 {
		t.Fatalf("expected 1 active sub, got %d", len(subs))
	}
	if subs[0].Name != "active" {
		t.Errorf("expected active, got %s", subs[0].Name)
	}
}

func TestRepository_ExistsByName(t *testing.T) {
	repo := NewRepository(setupRepoTestDB(t))
	ctx := context.Background()

	repo.Create(ctx, &model.RSSSubscription{Name: "unique", SiteName: "s", URLs: []string{"https://x.com/rss"}})

	exists, _ := repo.ExistsByName(ctx, "unique", 0)
	if !exists {
		t.Error("should exist")
	}

	exists2, _ := repo.ExistsByName(ctx, "missing", 0)
	if exists2 {
		t.Error("should not exist")
	}
}

func TestRepository_MarkSeen_AndIsSeen(t *testing.T) {
	repo := NewRepository(setupRepoTestDB(t))
	ctx := context.Background()

	seen := &model.RSSTorrentSeen{
		SiteName:       "example",
		TorrentID:      "42",
		SubscriptionID: "1",
		Title:          "Test",
		Status:         "seen",
	}
	if err := repo.MarkSeen(ctx, seen); err != nil {
		t.Fatal(err)
	}

	isSeen, err := repo.IsSeen(ctx, "example", "42")
	if err != nil {
		t.Fatal(err)
	}
	if !isSeen {
		t.Error("should be seen")
	}

	isSeen2, _ := repo.IsSeen(ctx, "example", "99")
	if isSeen2 {
		t.Error("should not be seen")
	}
}

func TestRepository_ListSeenBySite(t *testing.T) {
	repo := NewRepository(setupRepoTestDB(t))
	ctx := context.Background()

	since := time.Now().Add(-time.Hour)
	repo.MarkSeen(ctx, &model.RSSTorrentSeen{SiteName: "s1", TorrentID: "1", SubscriptionID: "1", Status: "seen"})
	repo.MarkSeen(ctx, &model.RSSTorrentSeen{SiteName: "s1", TorrentID: "2", SubscriptionID: "1", Status: "seen"})
	repo.MarkSeen(ctx, &model.RSSTorrentSeen{SiteName: "s2", TorrentID: "3", SubscriptionID: "1", Status: "seen"})

	seen, err := repo.ListSeenBySite(ctx, "s1", since)
	if err != nil {
		t.Fatal(err)
	}
	if len(seen) != 2 {
		t.Fatalf("expected 2, got %d", len(seen))
	}
}

func TestParseCronInterval(t *testing.T) {
	tests := []struct {
		cron   string
		expect time.Duration
	}{
		{"*/10 * * * *", 10 * time.Minute},
		{"*/5 * * * *", 5 * time.Minute},
		{"* * * * *", time.Minute},
		{"", 5 * time.Minute},
		{"0 */2 * * *", 5 * time.Minute},
	}

	for _, tt := range tests {
		got := parseCronInterval(tt.cron)
		if got != tt.expect {
			t.Errorf("parseCronInterval(%q) = %v, want %v", tt.cron, got, tt.expect)
		}
	}
}
