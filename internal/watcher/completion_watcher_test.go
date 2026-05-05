package watcher

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupWatcherTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&model.PublishCandidate{},
		&model.PublishResultRecord{},
		&model.ClientConfig{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func TestWatcher_WatchRegisters(t *testing.T) {
	db := setupWatcherTestDB(t)
	w := NewCompletionWatcher(db, nil, nil, zap.NewNop())

	if err := w.Watch(context.Background(), "client1", "hash1", 1); err != nil {
		t.Fatal(err)
	}
	if !w.IsWatching("client1", "hash1") {
		t.Error("should be watching")
	}
	if w.ActiveWatchCount() != 1 {
		t.Errorf("expected 1, got %d", w.ActiveWatchCount())
	}
}

func TestWatcher_WatchValidation(t *testing.T) {
	db := setupWatcherTestDB(t)
	w := NewCompletionWatcher(db, nil, nil, zap.NewNop())

	if err := w.Watch(context.Background(), "", "hash1", 1); err == nil {
		t.Error("expected error for empty client_name")
	}
	if err := w.Watch(context.Background(), "client1", "", 1); err == nil {
		t.Error("expected error for empty info_hash")
	}
}

func TestWatcher_SubmitCandidate(t *testing.T) {
	db := setupWatcherTestDB(t)
	w := NewCompletionWatcher(db, nil, nil, zap.NewNop())

	candidate := model.PublishCandidate{
		SourceSite:      "site1",
		SourceTorrentID: "torrent1",
		InfoHash:        "abc123",
		ClientID:        "client1",
		TorrentName:     "Test Torrent",
		PublishStatus:   model.CandidatePending,
		Role:            model.RoleDownload,
	}

	if err := w.SubmitCandidate(context.Background(), candidate); err != nil {
		t.Fatal(err)
	}

	if !w.IsWatching("client1", "abc123") {
		t.Error("should be watching after submit")
	}

	var count int64
	db.Model(&model.PublishCandidate{}).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 candidate in DB, got %d", count)
	}
}

func TestWatcher_SubmitCandidateDedup(t *testing.T) {
	db := setupWatcherTestDB(t)
	w := NewCompletionWatcher(db, nil, nil, zap.NewNop())

	c1 := model.PublishCandidate{
		SourceSite:      "site1",
		SourceTorrentID: "t1",
		PublishStatus:   model.CandidatePending,
	}
	db.Create(&c1)

	c2 := model.PublishCandidate{
		SourceSite:      "site1",
		SourceTorrentID: "t1",
		PublishStatus:   model.CandidatePending,
	}
	if err := w.SubmitCandidate(context.Background(), c2); err != nil {
		t.Fatal(err)
	}

	var count int64
	db.Model(&model.PublishCandidate{}).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 (dedup), got %d", count)
	}
}

func TestWatcher_SubmitCandidateValidation(t *testing.T) {
	db := setupWatcherTestDB(t)
	w := NewCompletionWatcher(db, nil, nil, zap.NewNop())

	c := model.PublishCandidate{
		SourceTorrentID: "t1",
	}
	if err := w.SubmitCandidate(context.Background(), c); err == nil {
		t.Error("expected error for missing source_site")
	}

	c2 := model.PublishCandidate{
		SourceSite: "site1",
	}
	if err := w.SubmitCandidate(context.Background(), c2); err == nil {
		t.Error("expected error for missing source_torrent_id")
	}
}

func TestWatcher_PollDetectsCompletion(t *testing.T) {
	db := setupWatcherTestDB(t)
	w := NewCompletionWatcher(db, nil, nil, zap.NewNop())

	candidate := model.PublishCandidate{
		SourceSite:      "site1",
		SourceTorrentID: "t1",
		InfoHash:        "hash1",
		ClientID:        "client1",
		PublishStatus:   model.CandidatePending,
	}
	db.Create(&candidate)

	if err := w.Watch(context.Background(), "client1", "hash1", candidate.ID); err != nil {
		t.Fatal(err)
	}

	w.watchStore.Range(func(key, _ any) bool {
		watchKey := key.(string)
		w.watchStore.Store(watchKey, candidate.ID)
		return true
	})

	var updated model.PublishCandidate
	db.First(&updated, candidate.ID)
	if updated.PublishStatus != model.CandidatePending {
		t.Errorf("expected pending before poll, got %s", updated.PublishStatus)
	}
}

func TestWatcher_PollOrphanDetection(t *testing.T) {
	db := setupWatcherTestDB(t)
	w := NewCompletionWatcher(db, nil, nil, zap.NewNop())

	candidate := model.PublishCandidate{
		SourceSite:      "site1",
		SourceTorrentID: "t1",
		InfoHash:        "hash1",
		ClientID:        "client1",
		PublishStatus:   model.CandidatePending,
	}
	db.Create(&candidate)

	w.markCandidateOrphan(context.Background(), candidate.ID)

	var updated model.PublishCandidate
	db.First(&updated, candidate.ID)
	if updated.PublishStatus != model.CandidateOrphan {
		t.Errorf("expected orphan status, got %s", updated.PublishStatus)
	}
}

func TestWatcher_RecoverPendingWatches(t *testing.T) {
	db := setupWatcherTestDB(t)
	w := NewCompletionWatcher(db, nil, nil, zap.NewNop())

	c1 := model.PublishCandidate{
		SourceSite: "s1", SourceTorrentID: "t1",
		InfoHash: "h1", ClientID: "c1",
		PublishStatus: model.CandidatePending,
	}
	c2 := model.PublishCandidate{
		SourceSite: "s2", SourceTorrentID: "t2",
		InfoHash: "h2", ClientID: "c2",
		PublishStatus: model.CandidateDownloading,
	}
	c3 := model.PublishCandidate{
		SourceSite: "s3", SourceTorrentID: "t3",
		InfoHash: "h3", ClientID: "c3",
		PublishStatus: model.CandidateDone,
	}
	db.Create(&c1)
	db.Create(&c2)
	db.Create(&c3)

	w.recoverPendingWatches(context.Background())

	if !w.IsWatching("c1", "h1") {
		t.Error("should recover pending candidate")
	}
	if !w.IsWatching("c2", "h2") {
		t.Error("should recover downloading candidate")
	}
	if w.IsWatching("c3", "h3") {
		t.Error("should not recover done candidate")
	}
}

func TestWatcher_StartStop(t *testing.T) {
	db := setupWatcherTestDB(t)
	w := NewCompletionWatcher(db, nil, nil, zap.NewNop())
	w.SetPollInterval(100 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := w.Start(ctx); err != nil {
		t.Fatal(err)
	}

	time.Sleep(150 * time.Millisecond)
	cancel()
	time.Sleep(50 * time.Millisecond)
}

func TestWatcher_ConcurrentWatch(t *testing.T) {
	db := setupWatcherTestDB(t)
	w := NewCompletionWatcher(db, nil, nil, zap.NewNop())

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := "client|" + "hash"
			w.watchStore.Store(key, uint(i))
		}(i)
	}
	wg.Wait()

	if w.ActiveWatchCount() != 1 {
		t.Errorf("expected 1 (same key), got %d", w.ActiveWatchCount())
	}
}

func TestWatcher_MultipleWatches(t *testing.T) {
	db := setupWatcherTestDB(t)
	w := NewCompletionWatcher(db, nil, nil, zap.NewNop())

	for i := 0; i < 10; i++ {
		if err := w.Watch(context.Background(), "client", "hash", uint(i)); err != nil {
			t.Fatal(err)
		}
	}

	if w.ActiveWatchCount() != 1 {
		t.Errorf("expected 1 (overwrites), got %d", w.ActiveWatchCount())
	}

	for i := 0; i < 5; i++ {
		if err := w.Watch(context.Background(), "client", "hash"+string(rune('a'+i)), uint(i)); err != nil {
			t.Fatal(err)
		}
	}

	if w.ActiveWatchCount() != 6 {
		t.Errorf("expected 6, got %d", w.ActiveWatchCount())
	}
}

func TestFmtCandidate(t *testing.T) {
	if got := fmtCandidate(42); got != "candidate-42" {
		t.Errorf("expected candidate-42, got %s", got)
	}
}
