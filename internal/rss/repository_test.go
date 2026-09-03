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
	if err := repo.Create(ctx, sub); err != nil {
		t.Fatal(err)
	}

	if err := repo.Delete(ctx, sub.ID); err != nil {
		t.Fatal(err)
	}

	_, err := repo.GetByID(ctx, sub.ID)
	if err == nil {
		t.Fatal("expected error after soft delete")
	}
}

func TestRepository_List(t *testing.T) {
	repo := NewRepository(setupRepoTestDB(t))
	ctx := context.Background()

	if err := repo.Create(ctx, &model.RSSSubscription{Name: "b-sub", SiteName: "s1", URLs: []string{"https://x.com/rss"}}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(ctx, &model.RSSSubscription{Name: "a-sub", SiteName: "s2", URLs: []string{"https://y.com/rss"}}); err != nil {
		t.Fatal(err)
	}

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

	if err := repo.Create(ctx, &model.RSSSubscription{Name: "active", SiteName: "s", URLs: []string{"https://x.com/rss"}, Enabled: true, Paused: false}); err != nil {
		t.Fatal(err)
	}

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

	if err := repo.Create(ctx, &model.RSSSubscription{Name: "unique", SiteName: "s", URLs: []string{"https://x.com/rss"}}); err != nil {
		t.Fatal(err)
	}

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
		Status:         "pushed",
	}
	if err := repo.MarkSeen(ctx, seen); err != nil {
		t.Fatal(err)
	}

	isSeen, err := repo.IsSeen(ctx, "example", "42", "1")
	if err != nil {
		t.Fatal(err)
	}
	if !isSeen {
		t.Error("should be seen")
	}

	isSeen2, _ := repo.IsSeen(ctx, "example", "99", "1")
	if isSeen2 {
		t.Error("should not be seen")
	}

	// §59.167 修复：status="seen" 视为已见过（原断言"waiting for push 不算 seen"
	// 的白名单语义与写入侧恒 "seen" 断裂——全订阅每轮重复分发实证；重试走 ClearSeen）
	seenWaiting := &model.RSSTorrentSeen{
		SiteName:       "example",
		TorrentID:      "43",
		SubscriptionID: "1",
		Title:          "Waiting",
		Status:         "seen",
	}
	if err := repo.MarkSeen(ctx, seenWaiting); err != nil {
		t.Fatal(err)
	}
	isSeen3, _ := repo.IsSeen(ctx, "example", "43", "1")
	if !isSeen3 {
		t.Error("status=seen 应视为已见过（§59.167 存在性判定）")
	}

	// §59.167 语义更新：存在性判定（见过即去重）——status="seen" 也算 seen
	seenStatusSeen := &model.RSSTorrentSeen{
		SiteName:       "siteA",
		TorrentID:      "tid-seen-status",
		SubscriptionID: "9",
		InfoHash:       "aaaa000000000000000000000000000000000000",
		Status:         "seen",
	}
	if err := repo.MarkSeen(ctx, seenStatusSeen); err != nil {
		t.Fatal(err)
	}
	if ok, _ := repo.IsSeen(ctx, "siteA", "tid-seen-status", "9"); !ok {
		t.Error("status=seen 应视为已见过（§59.167 修复——恒 seen 写入与白名单断裂）")
	}
}

func TestRepository_ListSeenBySite(t *testing.T) {
	repo := NewRepository(setupRepoTestDB(t))
	ctx := context.Background()

	since := time.Now().Add(-time.Hour)
	if err := repo.MarkSeen(ctx, &model.RSSTorrentSeen{SiteName: "s1", TorrentID: "1", SubscriptionID: "1", Status: "seen"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.MarkSeen(ctx, &model.RSSTorrentSeen{SiteName: "s1", TorrentID: "2", SubscriptionID: "1", Status: "seen"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.MarkSeen(ctx, &model.RSSTorrentSeen{SiteName: "s2", TorrentID: "3", SubscriptionID: "1", Status: "seen"}); err != nil {
		t.Fatal(err)
	}

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

// TestClearSeen §59.33: 清 seen 重推语义（指定清 / 全清 / is_fake_hash 排除）。
func TestClearSeen(t *testing.T) {
	db := setupRepoTestDB(t)
	repo := NewRepository(db)

	// 造 3 行：2 真实 + 1 fake_hash
	for i, tid := range []string{"t1", "t2", "t3"} {
		seen := &model.RSSTorrentSeen{
			SiteName: "站A", TorrentID: tid, SubscriptionID: "7",
			Title: "x", Status: "seen",
			IsFakeHash: i == 2, // t3 是侧载
		}
		if err := repo.MarkSeen(context.Background(), seen); err != nil {
			t.Fatal(err)
		}
	}

	// 指定清 t1
	n, err := repo.ClearSeen(context.Background(), "7", []string{"t1"})
	if err != nil || n != 1 {
		t.Fatalf("clear t1: n=%d err=%v", n, err)
	}
	var cnt int64
	db.Model(&model.RSSTorrentSeen{}).Where("subscription_id = ? AND torrent_id = ?", "7", "t1").Count(&cnt)
	if cnt != 0 {
		t.Error("t1 should be hard-deleted")
	}

	// 全清：只剩 t2（t3 是 fake 不清）
	n, err = repo.ClearSeen(context.Background(), "7", nil)
	if err != nil || n != 1 {
		t.Fatalf("clear all: n=%d err=%v (want 1, t3 fake excluded)", n, err)
	}
	db.Model(&model.RSSTorrentSeen{}).Where("subscription_id = ?", "7").Count(&cnt)
	if cnt != 1 {
		t.Errorf("fake_hash row must survive, got %d rows", cnt)
	}
}

// TestListSeenBySubscription §59.33: 列表按 updated_at 倒序 + limit。
func TestListSeenBySubscription(t *testing.T) {
	db := setupRepoTestDB(t)
	repo := NewRepository(db)
	for _, tid := range []string{"a", "b", "c"} {
		repo.MarkSeen(context.Background(), &model.RSSTorrentSeen{
			SiteName: "站A", TorrentID: tid, SubscriptionID: "9", Status: "seen",
		})
	}
	items, err := repo.ListSeenBySubscription(context.Background(), "9", 2)
	if err != nil || len(items) != 2 {
		t.Fatalf("list: %v %d", err, len(items))
	}
	other, _ := repo.ListSeenBySubscription(context.Background(), "8", 10)
	if len(other) != 0 {
		t.Error("subscription isolation broken")
	}
}

// §59.120: MarkStatus 订阅隔离——A 订阅打勾不得污染 B 订阅的 seen 行
//（§59.17 每订阅独立语义的漏改残留，§59.33 审计 #9）。
func TestMarkStatusSubscriptionIsolation(t *testing.T) {
	db := setupRepoTestDB(t)
	r := NewRepository(db)
	ctx := context.Background()
	// 两订阅同站同 tid（Status 模型默认 'seen'——先置区分性初始值 pending）
	db.Create(&model.RSSTorrentSeen{SiteName: "testsit", TorrentID: "501", SubscriptionID: "subA", Title: "t", Status: "pending"})
	db.Create(&model.RSSTorrentSeen{SiteName: "testsit", TorrentID: "501", SubscriptionID: "subB", Title: "t", Status: "pending"})

	r.MarkStatus(ctx, "subA", "testsit", "501", "seen")

	var bStatus string
	db.Model(&model.RSSTorrentSeen{}).
		Where("site_name = ? AND torrent_id = ? AND subscription_id = ?", "testsit", "501", "subB").
		Pluck("status", &bStatus)
	if bStatus != "pending" {
		t.Errorf("B 订阅被 A 污染: status=%q（应保持 pending）", bStatus)
	}
	var aStatus string
	db.Model(&model.RSSTorrentSeen{}).
		Where("site_name = ? AND torrent_id = ? AND subscription_id = ?", "testsit", "501", "subA").
		Pluck("status", &aStatus)
	if aStatus != "seen" {
		t.Errorf("A 订阅应已 seen: %q", aStatus)
	}
}
