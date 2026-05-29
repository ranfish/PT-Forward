package watcher

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"
	"unsafe"

	"github.com/ranfish/pt-forward/internal/client"
	"github.com/ranfish/pt-forward/internal/mocks"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/publish"
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
		w.watchStore.Store(watchKey, watchEntry{candidateID: candidate.ID, submittedAt: time.Now()})
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
	w.Stop()
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
			w.watchStore.Store(key, watchEntry{candidateID: uint(i), submittedAt: time.Now()})
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

func injectMockClient(t *testing.T, mgr *client.Manager, name string, dl model.DownloaderClient) {
	t.Helper()
	v := reflect.ValueOf(mgr).Elem()
	f := v.FieldByName("clients")
	clients := *(*map[string]model.DownloaderClient)(unsafe.Pointer(f.UnsafeAddr()))
	clients[name] = dl
}

func TestWatcher_OnWatchCompleted_HappyPath(t *testing.T) {
	db := setupWatcherTestDB(t)
	pipeline := publish.NewPipeline(db, zap.NewNop())
	w := NewCompletionWatcher(db, nil, pipeline, zap.NewNop())

	candidate := model.PublishCandidate{
		SourceSite:      "site1",
		SourceTorrentID: "t1",
		TorrentName:     "Clean Torrent",
		PublishStatus:   model.CandidatePending,
	}
	db.Create(&candidate)

	w.onWatchCompleted(context.Background(), candidate.ID, &model.TorrentInfo{
		SavePath: "/data/downloads",
	})

	var updated model.PublishCandidate
	db.First(&updated, candidate.ID)
	if !updated.DownloadCompleted {
		t.Error("expected download_completed")
	}
	if updated.LocalSavePath != "/data/downloads" {
		t.Errorf("expected /data/downloads, got %s", updated.LocalSavePath)
	}
	if updated.CompletedAt == nil {
		t.Error("expected completed_at to be set")
	}
}

func TestWatcher_OnWatchCompleted_PipelineNil(t *testing.T) {
	db := setupWatcherTestDB(t)
	w := NewCompletionWatcher(db, nil, nil, zap.NewNop())

	candidate := model.PublishCandidate{
		SourceSite:      "site1",
		SourceTorrentID: "t1",
		PublishStatus:   model.CandidatePending,
	}
	db.Create(&candidate)

	w.onWatchCompleted(context.Background(), candidate.ID, &model.TorrentInfo{
		SavePath: "/data/test",
	})

	var updated model.PublishCandidate
	db.First(&updated, candidate.ID)
	if !updated.DownloadCompleted {
		t.Error("expected download_completed")
	}
	if updated.PublishStatus != model.CandidateCompleted {
		t.Errorf("expected completed, got %s", updated.PublishStatus)
	}
	if updated.LocalSavePath != "/data/test" {
		t.Errorf("expected /data/test, got %s", updated.LocalSavePath)
	}
}

func TestWatcher_OnWatchCompleted_DBUpdateFailure(t *testing.T) {
	db := setupWatcherTestDB(t)
	w := NewCompletionWatcher(db, nil, nil, zap.NewNop())

	candidate := model.PublishCandidate{
		SourceSite:      "site1",
		SourceTorrentID: "t1",
		PublishStatus:   model.CandidatePending,
	}
	db.Create(&candidate)

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.Close()

	w.onWatchCompleted(context.Background(), candidate.ID, &model.TorrentInfo{
		SavePath: "/data/test",
	})
}

func TestWatcher_OnWatchCompleted_PipelineFailure(t *testing.T) {
	db := setupWatcherTestDB(t)
	pipeline := publish.NewPipeline(db, zap.NewNop())
	w := NewCompletionWatcher(db, nil, pipeline, zap.NewNop())

	candidate := model.PublishCandidate{
		SourceSite:      "site1",
		SourceTorrentID: "t1",
		TorrentName:     "Test 禁转 Torrent",
		PublishStatus:   model.CandidatePending,
	}
	db.Create(&candidate)

	w.onWatchCompleted(context.Background(), candidate.ID, &model.TorrentInfo{
		SavePath: "/data/test",
	})

	var updated model.PublishCandidate
	db.First(&updated, candidate.ID)
	if updated.PublishStatus != model.CandidateSkipped {
		t.Errorf("expected skipped, got %s", updated.PublishStatus)
	}
}

func TestWatcher_PollOnce_MalformedKey(t *testing.T) {
	db := setupWatcherTestDB(t)
	mgr := client.NewManager(db, zap.NewNop())
	w := NewCompletionWatcher(db, mgr, nil, zap.NewNop())

	w.watchStore.Store("malformed_key", watchEntry{candidateID: 999, submittedAt: time.Now()})

	w.pollOnce(context.Background())

	if _, ok := w.watchStore.Load("malformed_key"); ok {
		t.Error("malformed key should be deleted")
	}
}

func TestWatcher_PollOnce_GetClientFailure(t *testing.T) {
	db := setupWatcherTestDB(t)
	mgr := client.NewManager(db, zap.NewNop())
	w := NewCompletionWatcher(db, mgr, nil, zap.NewNop())

	candidate := model.PublishCandidate{
		SourceSite:      "site1",
		SourceTorrentID: "t1",
		PublishStatus:   model.CandidatePending,
	}
	db.Create(&candidate)

	w.watchStore.Store("missing_client|hash1", watchEntry{candidateID: candidate.ID, submittedAt: time.Now()})

	w.pollOnce(context.Background())

	if _, ok := w.watchStore.Load("missing_client|hash1"); !ok {
		t.Error("watch should not be removed on Get failure")
	}
}

func TestWatcher_PollOnce_GetTorrentByHashError(t *testing.T) {
	db := setupWatcherTestDB(t)
	mgr := client.NewManager(db, zap.NewNop())
	w := NewCompletionWatcher(db, mgr, nil, zap.NewNop())

	mockDL := &mocks.DownloaderClient{
		GetTorrentByHashFn: func(ctx context.Context, hash string) (*model.TorrentInfo, error) {
			return nil, errors.New("connection refused")
		},
	}
	injectMockClient(t, mgr, "client1", mockDL)

	candidate := model.PublishCandidate{
		SourceSite:      "site1",
		SourceTorrentID: "t1",
		PublishStatus:   model.CandidatePending,
	}
	db.Create(&candidate)

	w.watchStore.Store("client1|hash1", watchEntry{candidateID: candidate.ID, submittedAt: time.Now()})

	w.pollOnce(context.Background())

	if _, ok := w.watchStore.Load("client1|hash1"); !ok {
		t.Error("watch should not be removed on GetTorrentByHash error")
	}

	var updated model.PublishCandidate
	db.First(&updated, candidate.ID)
	if updated.PublishStatus != model.CandidatePending {
		t.Errorf("expected pending, got %s", updated.PublishStatus)
	}
}

func TestWatcher_PollOnce_TorrentNotFound(t *testing.T) {
	db := setupWatcherTestDB(t)
	mgr := client.NewManager(db, zap.NewNop())
	w := NewCompletionWatcher(db, mgr, nil, zap.NewNop())

	mockDL := &mocks.DownloaderClient{}
	injectMockClient(t, mgr, "client1", mockDL)

	candidate := model.PublishCandidate{
		SourceSite:      "site1",
		SourceTorrentID: "t1",
		PublishStatus:   model.CandidatePending,
	}
	db.Create(&candidate)

	w.watchStore.Store("client1|hash1", watchEntry{candidateID: candidate.ID, submittedAt: time.Now()})

	w.pollOnce(context.Background())

	if _, ok := w.watchStore.Load("client1|hash1"); ok {
		t.Error("watch should be removed for orphan")
	}

	var updated model.PublishCandidate
	db.First(&updated, candidate.ID)
	if updated.PublishStatus != model.CandidateOrphan {
		t.Errorf("expected orphan, got %s", updated.PublishStatus)
	}
	if updated.SkipReason != "种子在下载器中不存在" {
		t.Errorf("expected orphan skip reason, got %s", updated.SkipReason)
	}
}

func TestWatcher_PollOnce_DetectsCompletion(t *testing.T) {
	db := setupWatcherTestDB(t)
	mgr := client.NewManager(db, zap.NewNop())
	pipeline := publish.NewPipeline(db, zap.NewNop())
	w := NewCompletionWatcher(db, mgr, pipeline, zap.NewNop())

	mockDL := &mocks.DownloaderClient{
		GetTorrentByHashFn: func(ctx context.Context, hash string) (*model.TorrentInfo, error) {
			return &model.TorrentInfo{
				Hash:       "hash1",
				IsFinished: true,
				SavePath:   "/data/complete",
			}, nil
		},
	}
	injectMockClient(t, mgr, "client1", mockDL)

	candidate := model.PublishCandidate{
		SourceSite:      "site1",
		SourceTorrentID: "t1",
		TorrentName:     "Clean Torrent",
		PublishStatus:   model.CandidatePending,
	}
	db.Create(&candidate)

	w.watchStore.Store("client1|hash1", watchEntry{candidateID: candidate.ID, submittedAt: time.Now()})

	w.pollOnce(context.Background())

	if _, ok := w.watchStore.Load("client1|hash1"); ok {
		t.Error("watch should be removed after completion")
	}

	var updated model.PublishCandidate
	db.First(&updated, candidate.ID)
	if !updated.DownloadCompleted {
		t.Error("expected download_completed")
	}
	if updated.LocalSavePath != "/data/complete" {
		t.Errorf("expected /data/complete, got %s", updated.LocalSavePath)
	}
}
