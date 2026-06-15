package reseed

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/fingerprint"
	"github.com/ranfish/pt-forward/internal/mocks"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupReseedDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&model.ReseedTask{},
		&model.ReseedMatch{},
		&model.ReseedNegativeCache{},
		&model.ContentFingerprint{},
		&model.SeedingTorrentRecord{},
		&model.SearchCache{},
		&model.ClientConfig{},
		&model.Site{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func setupEngineWithMockClient(t *testing.T, db *gorm.DB, torrents []*model.TorrentInfo, siteMap map[string]string) *Engine {
	t.Helper()
	e := NewEngine(db, zap.NewNop())
	e.SetClientProvider(&mocks.DownloaderProvider{
		GetFn: func(clientID string) (model.DownloaderClient, error) {
			return &mocks.DownloaderClient{
				Name: clientID,
				GetAllTorrentsFn: func(ctx context.Context) ([]*model.TorrentInfo, error) {
					return torrents, nil
				},
			}, nil
		},
	})
	resolver := NewTrackerSiteResolver()
	var sites []*model.Site
	for domain, name := range siteMap {
		sites = append(sites, &model.Site{Domain: domain, Name: name})
	}
	resolver.BuildIndex(sites)
	e.SetTrackerResolver(resolver)
	return e
}

func TestEngine_CreateAndGetTask(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	task := &model.ReseedTask{Name: "test-task", Enabled: true}
	if err := e.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create: %v", err)
	}
	if task.ID == 0 {
		t.Error("expected non-zero ID")
	}

	got, err := e.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "test-task" {
		t.Errorf("expected test-task, got %s", got.Name)
	}
	if got.Status != model.ReseedTaskIdle {
		t.Errorf("expected idle, got %s", got.Status)
	}
}

func TestEngine_UpdateTask(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	task := &model.ReseedTask{Name: "up-task", Enabled: true, ClientIDs: "c1"}
	if err := e.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create: %v", err)
	}

	task.Enabled = false
	task.Name = "updated"
	if err := e.UpdateTask(context.Background(), task); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, _ := e.GetTask(context.Background(), task.ID)
	if got.Name != "updated" {
		t.Errorf("expected updated, got %s", got.Name)
	}
	if got.Enabled != false {
		t.Error("expected disabled")
	}
}

func TestEngine_DeleteTask(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	task := &model.ReseedTask{Name: "del-task", Enabled: true, ClientIDs: "c1"}
	if err := e.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := e.DeleteTask(context.Background(), task.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err := e.GetTask(context.Background(), task.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestEngine_RunTaskWithRecords(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	task := &model.ReseedTask{Name: "run-task", Enabled: true, ClientIDs: "c1"}
	if err := e.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create: %v", err)
	}

	e.SetClientProvider(&mocks.DownloaderProvider{
		GetFn: func(clientID string) (model.DownloaderClient, error) {
			return &mocks.DownloaderClient{
				Name: "c1",
				GetAllTorrentsFn: func(ctx context.Context) ([]*model.TorrentInfo, error) {
					return []*model.TorrentInfo{
						{Hash: "ih1", TrackerURL: "https://tracker.site1.com/announce", Progress: 1.0},
					}, nil
				},
			}, nil
		},
	})

	resolver := NewTrackerSiteResolver()
	resolver.BuildIndex([]*model.Site{
		{Domain: "site1.com", Name: "site1"},
	})
	e.SetTrackerResolver(resolver)

	result, err := e.RunTask(context.Background(), task)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.TotalSources != 1 {
		t.Errorf("expected 1 source, got %d", result.TotalSources)
	}
}

func TestEngine_MatchRetry(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	if err := e.CreateTask(context.Background(), &model.ReseedTask{Name: "rt", Enabled: true}); err != nil {
		t.Fatalf("create: %v", err)
	}
	db.Create(&model.ReseedMatch{
		ClientID: "c1", SourceSite: "s1", SourceTorrentID: "t1", SourceInfoHash: "ih1",
		TargetSite: "s2", TargetTorrentID: "t2", MatchMethod: "pieces_hash",
		Confidence: 0.9, Status: model.MatchStatusFailed, FailReason: "test",
	})

	match, err := e.RetryMatch(context.Background(), 1)
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	if match.Status != model.MatchStatusMatched {
		t.Errorf("expected matched, got %s", match.Status)
	}
	if match.RetryCount != 1 {
		t.Errorf("expected retry_count=1, got %d", match.RetryCount)
	}
}

func TestEngine_DeleteNegativeCache(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	db.Create(&model.ReseedNegativeCache{
		SourceSite: "s1", SourceTorrentID: "t1", SourceInfoHash: "ih1",
		ExcludedTargets: "s2", ExpiresAt: model.ReseedNegativeCache{}.ExpiresAt,
	})

	deleted, err := e.DeleteNegativeCache(context.Background(), "ih1", "s1")
	if err != nil {
		t.Fatalf("delete neg cache: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 deleted, got %d", deleted)
	}
}

func TestMatchDecision_SameHash(t *testing.T) {
	input := MatchInput{
		SourceInfoHash: "abc123",
		TargetInfoHash: "abc123",
		SourceSize:     1000,
		TargetSize:     1000,
	}
	decision := MatchDecision(input, 1.0)
	if decision != model.DecisionSameInfoHash {
		t.Errorf("expected SAME_INFO_HASH, got %s", decision)
	}
}

func TestMatchDecision_ExactMatch(t *testing.T) {
	input := MatchInput{
		SourceInfoHash: "aaa",
		TargetInfoHash: "bbb",
		SourceSize:     1073741824,
		TargetSize:     1073741824,
	}
	decision := MatchDecision(input, 1.0)
	if decision != model.DecisionMatch {
		t.Errorf("expected MATCH, got %s", decision)
	}
}

func TestMatchDecision_SizeOnlyMatch(t *testing.T) {
	input := MatchInput{
		SourceInfoHash: "aaa",
		TargetInfoHash: "bbb",
		SourceSize:     1073741825,
		TargetSize:     1073741824,
	}
	decision := MatchDecision(input, 1.0)
	if decision != model.DecisionMatchSizeOnly {
		t.Errorf("expected MATCH_SIZE_ONLY, got %s", decision)
	}
}

func TestMatchDecision_FuzzyMismatch(t *testing.T) {
	input := MatchInput{
		SourceInfoHash: "aaa",
		TargetInfoHash: "bbb",
		SourceSize:     1127428915,
		TargetSize:     1073741824,
	}
	decision := MatchDecision(input, 1.0)
	if decision != model.DecisionFuzzySizeMismatch {
		t.Errorf("expected FUZZY_SIZE_MISMATCH, got %s", decision)
	}
}

func TestMatchDecision_SizeMismatch(t *testing.T) {
	input := MatchInput{
		SourceInfoHash: "aaa",
		TargetInfoHash: "bbb",
		SourceSize:     2147483648,
		TargetSize:     1073741824,
	}
	decision := MatchDecision(input, 1.0)
	if decision != model.DecisionSizeMismatch {
		t.Errorf("expected SIZE_MISMATCH, got %s", decision)
	}
}

func TestMatchDecision_ZeroSize(t *testing.T) {
	input := MatchInput{
		SourceInfoHash: "aaa",
		TargetInfoHash: "bbb",
		SourceSize:     0,
		TargetSize:     1000,
	}
	decision := MatchDecision(input, 1.0)
	if decision != model.DecisionNoDownloadLink {
		t.Errorf("expected NO_DOWNLOAD_LINK, got %s", decision)
	}
}

func TestEngine_RetryMatch_NonFailedRejected(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	db.Create(&model.ReseedMatch{
		ClientID: "c1", SourceSite: "s1", SourceTorrentID: "t1", SourceInfoHash: "ih1",
		TargetSite: "s2", TargetTorrentID: "t2", MatchMethod: "pieces_hash",
		Confidence: 0.9, Status: model.MatchStatusMatched,
	})

	_, err := e.RetryMatch(context.Background(), 1)
	if err == nil {
		t.Error("expected error for non-failed match")
	}
	appErr, ok := err.(*model.AppError)
	if !ok {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != ErrReseedGeneric {
		t.Errorf("expected code %d, got %d", ErrReseedGeneric, appErr.Code)
	}
}

func TestEngine_RetryMatch_NotFound(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	_, err := e.RetryMatch(context.Background(), 999)
	if err == nil {
		t.Error("expected error for non-existent match")
	}
}

func TestEngine_DeleteNegativeCache_NotFound(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	deleted, err := e.DeleteNegativeCache(context.Background(), "nonexist", "s1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleted != 0 {
		t.Errorf("expected 0 deleted, got %d", deleted)
	}
}

func TestEngine_DeleteNegativeCache_EmptySite(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	db.Create(&model.ReseedNegativeCache{
		SourceSite: "s1", SourceTorrentID: "t1", SourceInfoHash: "ih1",
		ExcludedTargets: "s2",
	})

	deleted, err := e.DeleteNegativeCache(context.Background(), "ih1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 deleted with empty site filter, got %d", deleted)
	}
}

func TestEngine_ListTasks(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	if err := e.CreateTask(context.Background(), &model.ReseedTask{Name: "b-task", Enabled: true, ClientIDs: "c1"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := e.CreateTask(context.Background(), &model.ReseedTask{Name: "a-task", Enabled: true, ClientIDs: "c1"}); err != nil {
		t.Fatalf("create: %v", err)
	}

	tasks, err := e.ListTasks(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}
	if tasks[0].Name != "a-task" {
		t.Errorf("expected sorted by name, got %s first", tasks[0].Name)
	}
}

func TestEngine_FindMatchByID(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	db.Create(&model.ReseedMatch{
		ClientID: "c1", SourceSite: "s1", SourceTorrentID: "t1", SourceInfoHash: "ih1",
		TargetSite: "s2", TargetTorrentID: "t2", MatchMethod: "pieces_hash",
		Confidence: 1.0, Status: model.MatchStatusMatched,
	})

	m, err := e.FindMatchByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if m.MatchMethod != "pieces_hash" {
		t.Errorf("expected pieces_hash, got %s", m.MatchMethod)
	}
}

func TestEngine_FindMatchByID_NotFound(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	_, err := e.FindMatchByID(context.Background(), 999)
	if err == nil {
		t.Error("expected error for non-existent match")
	}
}

func TestEngine_RunTask_EmptyClientIDs(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	task := &model.ReseedTask{Name: "empty-clients", Enabled: true, ClientIDs: ""}
	if err := e.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create: %v", err)
	}

	result, err := e.RunTask(context.Background(), task)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.TotalSources != 0 {
		t.Errorf("expected 0 sources, got %d", result.TotalSources)
	}
}

func TestEngine_RunTask_SourceSiteFilter(t *testing.T) {
	db := setupReseedDB(t)
	torrents := []*model.TorrentInfo{
		{Hash: "ih1", TrackerURL: "https://tracker.site1.com/announce", Progress: 1.0},
		{Hash: "ih2", TrackerURL: "https://tracker.site2.com/announce", Progress: 1.0},
	}
	e := setupEngineWithMockClient(t, db, torrents, map[string]string{
		"site1.com": "site1",
		"site2.com": "site2",
	})

	task := &model.ReseedTask{
		Name: "filtered", Enabled: true, ClientIDs: "c1",
		SourceSiteIDs: "site1",
	}
	if err := e.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create: %v", err)
	}

	result, err := e.RunTask(context.Background(), task)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.TotalSources != 2 {
		t.Errorf("expected 2 total sources, got %d", result.TotalSources)
	}
	if result.Skipped != 1 {
		t.Errorf("expected 1 skipped (site2 filtered), got %d", result.Skipped)
	}
}

func TestNormalizeTitle(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{"  Some Movie 2023 1080p BluRay x264  ", "some movie 2023"},
		{"Short.Title", "short.title"},
		{"", ""},
		{"Movie Name [Group] (2023)", "movie name"},
		{"A Very Long Title That Exceeds Fifty Characters In Length And Should Be Truncated Here", "a very long title that exceeds fifty characters in length and should be truncate"},
	}

	for _, tt := range tests {
		got := NormalizeTitle(tt.input)
		if got != tt.expect {
			t.Errorf("NormalizeTitle(%q) = %q, want %q", tt.input, got, tt.expect)
		}
	}
}

func TestEngine_SetSiteProvider(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())
	if e.siteProvider != nil {
		t.Fatal("expected nil siteProvider before Set")
	}
	e.SetSiteProvider(nil)
}

func TestEngine_StartStop(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	if err := e.CreateTask(context.Background(), &model.ReseedTask{Name: "auto1", Enabled: true, ClientIDs: "c1"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := e.CreateTask(context.Background(), &model.ReseedTask{Name: "disabled", Enabled: false, ClientIDs: "c1"}); err != nil {
		t.Fatalf("create: %v", err)
	}

	ctx := context.Background()
	if err := e.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}

	e.Stop()
}

func TestEngine_SaveAndFindMatch(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	match := &model.ReseedMatch{
		ClientID: "c1", SourceSite: "s1", SourceTorrentID: "t1", SourceInfoHash: "ih1",
		TargetSite: "s2", TargetTorrentID: "t2", MatchMethod: "pieces_hash",
		Confidence: 1.0, Status: model.MatchStatusMatched,
	}
	if err := e.SaveMatch(context.Background(), match); err != nil {
		t.Fatalf("save: %v", err)
	}

	matches, err := e.FindMatchesByInfoHash(context.Background(), "ih1")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1, got %d", len(matches))
	}
	if matches[0].TargetSite != "s2" {
		t.Errorf("expected s2, got %s", matches[0].TargetSite)
	}
}

func TestEngine_FindMatchesByInfoHash_Empty(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	matches, err := e.FindMatchesByInfoHash(context.Background(), "nonexist")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Errorf("expected 0, got %d", len(matches))
	}
}

func TestEngine_MatchDecision_Layered(t *testing.T) {
	tests := []struct {
		name       string
		sourceHash string
		targetHash string
		sourceSize int64
		targetSize int64
		tolerance  float64
		expect     model.DecisionType
	}{
		{"same hash", "abc", "abc", 1000, 1000, 1.0, model.DecisionSameInfoHash},
		{"zero source size", "a", "b", 0, 1000, 1.0, model.DecisionNoDownloadLink},
		{"zero target size", "a", "b", 1000, 0, 1.0, model.DecisionNoDownloadLink},
		{"exact match", "a", "b", 1073741824, 1073741824, 1.0, model.DecisionMatch},
		{"size only match 0.5%", "a", "b", 1073741824, 1073741824 + 5*1024*1024, 1.0, model.DecisionMatchSizeOnly},
		{"fuzzy mismatch 3%", "a", "b", 1073741824, 1073741824 + 32*1024*1024, 1.0, model.DecisionFuzzySizeMismatch},
		{"total mismatch", "a", "b", 2147483648, 1073741824, 1.0, model.DecisionSizeMismatch},
		{"custom tolerance 5%", "a", "b", 1073741824, 1073741824 + 40*1024*1024, 5.0, model.DecisionMatchSizeOnly},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := MatchInput{
				SourceInfoHash: tt.sourceHash,
				TargetInfoHash: tt.targetHash,
				SourceSize:     tt.sourceSize,
				TargetSize:     tt.targetSize,
			}
			got := MatchDecision(input, tt.tolerance)
			if got != tt.expect {
				t.Errorf("got %s, want %s", got, tt.expect)
			}
		})
	}
}

func TestEngine_RunTask_Canceled(t *testing.T) {
	db := setupReseedDB(t)
	torrents := []*model.TorrentInfo{
		{Hash: "ih1", TrackerURL: "https://tracker.site1.com/announce", Progress: 1.0},
	}
	e := setupEngineWithMockClient(t, db, torrents, map[string]string{
		"site1.com": "site1",
	})

	task := &model.ReseedTask{Name: "cancel-test", Enabled: true, ClientIDs: "c1"}
	if err := e.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := e.RunTask(ctx, task)
	if err != nil {
		t.Logf("run returned error (acceptable): %v", err)
	}
	_ = result
}

func TestEngine_CancelTask_Noop(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())
	e.mu.Lock()
	cancelCount := len(e.tasks)
	e.mu.Unlock()
	if cancelCount != 0 {
		t.Errorf("expected 0 tasks, got %d", cancelCount)
	}
	e.CancelTask(999)
}

func TestParseClientIDs(t *testing.T) {
	tests := []struct {
		input  string
		expect int
	}{
		{"c1,c2,c3", 3},
		{"c1", 1},
		{"", 0},
		{"c1, c2 , c3 ", 3},
		{",,", 0},
	}

	for _, tt := range tests {
		got := ParseClientIDs(tt.input)
		if len(got) != tt.expect {
			t.Errorf("ParseClientIDs(%q) = %d, want %d", tt.input, len(got), tt.expect)
		}
	}
}

func TestEngine_SetNegativeCache(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	err := e.SetNegativeCache(context.Background(), "sourceSite1", "ih1", "targetSite1", "pieces_hash", 3, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	var entry model.ReseedNegativeCache
	db.Where("source_info_hash = ?", "ih1").First(&entry)
	if entry.SourceSite != "sourceSite1" {
		t.Errorf("expected sourceSite1, got %s", entry.SourceSite)
	}
	if entry.ExcludedTargets != "targetSite1" {
		t.Errorf("expected targetSite1, got %s", entry.ExcludedTargets)
	}
	if entry.LastMethod != "pieces_hash" {
		t.Errorf("expected pieces_hash, got %s", entry.LastMethod)
	}
	if entry.ExpiresAt.IsZero() {
		t.Error("expires_at should not be zero")
	}
}

func TestEngine_GetNegativeCacheByHashes(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	if err := e.SetNegativeCache(context.Background(), "s1", "ih1", "site1", "method1", 3, 24*time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := e.SetNegativeCache(context.Background(), "s2", "ih2", "site2", "method2", 3, 24*time.Hour); err != nil {
		t.Fatal(err)
	}

	entries, err := e.GetNegativeCacheByHashes(context.Background(), []string{"ih1", "ih2"})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2, got %d", len(entries))
	}

	entries, err = e.GetNegativeCacheByHashes(context.Background(), []string{"ih3"})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 for non-existent hash, got %d", len(entries))
	}

	entries, err = e.GetNegativeCacheByHashes(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if entries != nil {
		t.Error("expected nil for empty input")
	}
}

func TestEngine_GetNegativeCacheByHashes_Expired(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	db.Create(&model.ReseedNegativeCache{
		SourceSite: "s1", SourceTorrentID: "t1", SourceInfoHash: "ih_expired",
		ExcludedTargets: "s2", ExpiresAt: time.Now().Add(-1 * time.Hour),
	})

	entries, err := e.GetNegativeCacheByHashes(context.Background(), []string{"ih_expired"})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 for expired entry, got %d", len(entries))
	}
}

func TestEngine_FlushNegativeCache(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	db.Create(&model.ReseedNegativeCache{
		SourceSite: "s1", SourceTorrentID: "t1", SourceInfoHash: "ih1",
		ExcludedTargets: "s2", ExpiresAt: time.Now().Add(-1 * time.Hour),
	})
	db.Create(&model.ReseedNegativeCache{
		SourceSite: "s2", SourceTorrentID: "t2", SourceInfoHash: "ih2",
		ExcludedTargets: "s3", ExpiresAt: time.Now().Add(24 * time.Hour),
	})

	flushed, err := e.FlushNegativeCache(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if flushed != 1 {
		t.Errorf("expected 1 flushed, got %d", flushed)
	}

	var remaining int64
	db.Model(&model.ReseedNegativeCache{}).Count(&remaining)
	if remaining != 1 {
		t.Errorf("expected 1 remaining, got %d", remaining)
	}
}

type mockIYUUService struct {
	queryReseedFn      func(ctx context.Context, infoHashes []string) ([]*model.IYUUReseedResult, error)
	getSeededSitesFn   func(ctx context.Context, infoHash string) ([]string, error)
	getSiteListFn      func(ctx context.Context) ([]model.IYUUSite, error)
	reportExistingFn   func(ctx context.Context, sidList []int) error
	pingFn             func(ctx context.Context) error
	sendNotificationFn func(ctx context.Context, text, desp string) error
}

func (m *mockIYUUService) QueryReseed(ctx context.Context, infoHashes []string) ([]*model.IYUUReseedResult, error) {
	if m.queryReseedFn != nil {
		return m.queryReseedFn(ctx, infoHashes)
	}
	return nil, nil
}
func (m *mockIYUUService) GetSeededSites(ctx context.Context, infoHash string) ([]string, error) {
	if m.getSeededSitesFn != nil {
		return m.getSeededSitesFn(ctx, infoHash)
	}
	return nil, nil
}
func (m *mockIYUUService) GetSiteList(ctx context.Context) ([]model.IYUUSite, error) {
	if m.getSiteListFn != nil {
		return m.getSiteListFn(ctx)
	}
	return nil, nil
}
func (m *mockIYUUService) ReportExisting(ctx context.Context, sidList []int) error {
	if m.reportExistingFn != nil {
		return m.reportExistingFn(ctx, sidList)
	}
	return nil
}
func (m *mockIYUUService) Ping(ctx context.Context) error {
	if m.pingFn != nil {
		return m.pingFn(ctx)
	}
	return nil
}
func (m *mockIYUUService) SendNotification(ctx context.Context, text, desp string) error {
	if m.sendNotificationFn != nil {
		return m.sendNotificationFn(ctx, text, desp)
	}
	return nil
}

func setupReseedDBWithIYUU(t *testing.T) *gorm.DB {
	t.Helper()
	db := setupReseedDB(t)
	if err := db.AutoMigrate(&model.IYUUSiteMapping{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func TestCheckPublishEligibility(t *testing.T) {
	tests := []struct {
		title  string
		expect bool
	}{
		{"", true},
		{"Some normal title", true},
		{"Movie Name 禁转 2023", false},
		{"Movie Name 独占 release", false},
		{"Title 谢绝转载 extra", false},
		{"Movie 限时禁转 info", false},
		{"Title 严禁转载 data", false},
		{"CatEDU release group", false},
		{"CatEDU.Something.2023", false},
	}
	for _, tt := range tests {
		got := checkPublishEligibility(tt.title)
		if got != tt.expect {
			t.Errorf("checkPublishEligibility(%q) = %v, want %v", tt.title, got, tt.expect)
		}
	}
}

func TestHasMatchMethod(t *testing.T) {
	if !hasMatchMethod("", "iyuu") {
		t.Error("expected true for empty methods string")
	}
	if !hasMatchMethod("iyuu,size_title", "iyuu") {
		t.Error("expected true when method is in list")
	}
	if hasMatchMethod("size_title,infohash", "iyuu") {
		t.Error("expected false when method is not in list")
	}
	if !hasMatchMethod("iyuu", "iyuu") {
		t.Error("expected true for single method match")
	}
}

func TestEngine_SetClientProvider(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())
	if e.clientProvider != nil {
		t.Fatal("expected nil clientProvider before Set")
	}
	e.SetClientProvider(nil)
}

func TestEngine_SetFingerprintRepo(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())
	if e.fpRepo != nil {
		t.Fatal("expected nil fpRepo before Set")
	}
	e.SetFingerprintRepo(nil)
}

func TestEngine_SetIYUUService(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())
	if e.iyuuService != nil {
		t.Fatal("expected nil iyuuService before Set")
	}
	e.SetIYUUService(nil)
}

func TestEngine_RunTask_DuplicateExists(t *testing.T) {
	db := setupReseedDB(t)
	torrents := []*model.TorrentInfo{
		{Hash: "ih_dup", TrackerURL: "https://tracker.site1.com/announce", Progress: 1.0},
	}
	e := setupEngineWithMockClient(t, db, torrents, map[string]string{
		"site1.com": "site1",
	})

	fpRepo := fingerprint.NewRepository(db, zap.NewNop())
	if err := fpRepo.Save(context.Background(), &model.ContentFingerprint{
		InfoHash: "ih_dup", SiteName: "site1",
		Title: "Test.Torrent.2023.1080p.BluRay.x264-GROUPD", TotalSize: 1073741824,
	}); err != nil {
		t.Fatal(err)
	}
	e.SetFingerprintRepo(fpRepo)

	adapter := &mocks.SiteAdapter{
		SearchTorrentsFn: func(ctx context.Context, config *model.SiteConfig, query string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
			return []*model.SeedingSearchResult{
				{TorrentID: "t2", Title: "Test.Torrent.2023.1080p.BluRay.x264-GROUPD", Size: 1073741824},
			}, nil
		},
	}
	e.SetSiteProvider(&mocks.SiteInfoProvider{
		ListSitesFn: func(ctx context.Context) ([]*model.SiteInfo, error) {
			return []*model.SiteInfo{
				{Name: "site2", BaseURL: "https://site2.example.com", Enabled: true},
			}, nil
		},
		GetSiteInfoFn: func(ctx context.Context, siteName string) (*model.SiteInfo, error) {
			return &model.SiteInfo{Name: "site2", BaseURL: "https://site2.example.com", Enabled: true}, nil
		},
		GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
			return &model.SiteConfig{Enabled: true}, nil
		},
		GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
			return adapter, nil
		},
	})

	task := &model.ReseedTask{Name: "dup-task", Enabled: true, ClientIDs: "c1", TargetSiteIDs: "site2", MatchMethods: "size_title"}
	if err := e.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create: %v", err)
	}

	db.Create(&model.ReseedMatch{
		ClientID: "c1", SourceSite: "site1", SourceTorrentID: "ih_dup", SourceInfoHash: "ih_dup",
		TargetSite: "site2", TargetTorrentID: "t2", MatchMethod: "pieces_hash",
		Confidence: 1.0, Status: model.MatchStatusMatched,
	})

	result, err := e.RunTask(context.Background(), task)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.DuplicateExists != 1 {
		t.Errorf("expected 1 duplicate, got %d", result.DuplicateExists)
	}
}

func TestEngine_findCandidates_NilProvider(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())
	task := &model.ReseedTask{Name: "fc-nil", Enabled: true}
	rec := model.SeedingTorrentRecord{InfoHash: "ih1", SiteName: "site1"}
	candidates := e.findCandidates(context.Background(), sourceTorrent{InfoHash: rec.InfoHash, SiteName: rec.SiteName}, e.preloadSites(context.Background(), nil, nil), nil, 1.0, task, nil, nil, nil, nil, nil)
	if candidates != nil {
		t.Error("expected nil when provider is nil")
	}
}

func TestEngine_findCandidates_WithProvider(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	adapter := &mocks.SiteAdapter{
		SearchTorrentsFn: func(ctx context.Context, config *model.SiteConfig, query string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
			return []*model.SeedingSearchResult{
				{TorrentID: "target_t1", Title: "Ubuntu.24.04.2024.1080p.BluRay.x264-GROUPX", Size: 1073741824},
			}, nil
		},
	}

	fpRepo := fingerprint.NewRepository(db, zap.NewNop())
	if err := fpRepo.Save(context.Background(), &model.ContentFingerprint{
		InfoHash: "ih1", SiteName: "source_site",
		Title: "Ubuntu.24.04.2024.1080p.BluRay.x264-GROUPX", TotalSize: 1073741824,
	}); err != nil {
		t.Fatal(err)
	}
	e.SetFingerprintRepo(fpRepo)

	sp := &mocks.SiteInfoProvider{
		ListSitesFn: func(ctx context.Context) ([]*model.SiteInfo, error) {
			return []*model.SiteInfo{
				{Name: "target_site", BaseURL: "https://target.example.com", Enabled: true},
			}, nil
		},
		GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
			return &model.SiteConfig{Enabled: true}, nil
		},
		GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
			return adapter, nil
		},
	}
	e.SetSiteProvider(sp)

	task := &model.ReseedTask{Name: "fc-prov", Enabled: true, MatchMethods: "size_title"}
	candidates := e.findCandidates(context.Background(), sourceTorrent{InfoHash: "ih1", SiteName: "source_site"}, e.preloadSites(context.Background(), nil, nil), e.preloadFingerprints(context.Background(), []string{"ih1"}), 1.0, task, nil, nil, nil, nil, nil)
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
	if candidates[0].MatchMethod != "search_verify" {
		t.Errorf("expected search_verify, got %s", candidates[0].MatchMethod)
	}
	if candidates[0].TargetSite != "target_site" {
		t.Errorf("expected target_site, got %s", candidates[0].TargetSite)
	}
}

func TestEngine_matchLayer2SearchVerify_WithDB(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	db.Create(&model.ContentFingerprint{
		InfoHash:  "ih_source",
		SiteName:  "source_site",
		Title:     "Some.Movie.2023.1080p.BluRay.x264-GROUP1",
		TotalSize: 1073741824,
	})

	adapter := &mocks.SiteAdapter{
		SearchTorrentsFn: func(ctx context.Context, config *model.SiteConfig, query string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
			return []*model.SeedingSearchResult{
				{TorrentID: "t_target", Title: "Some.Movie.2023.1080p.BluRay.x264-GROUP1", Size: 1073741824},
			}, nil
		},
	}

	rec := model.SeedingTorrentRecord{InfoHash: "ih_source", SiteName: "source_site"}
	fc := e.preloadFingerprints(context.Background(), []string{rec.InfoHash})
	c := e.matchLayer2SizeTitle(context.Background(), adapter, &model.SiteConfig{}, rec.InfoHash, rec.SiteName, "target_site", 1.0, fc)
	if c == nil {
		t.Fatal("expected candidate, got nil")
	}
	if c.MatchMethod != "search_verify" {
		t.Errorf("expected search_verify, got %s", c.MatchMethod)
	}
	if c.Confidence != 0.95 {
		t.Errorf("expected confidence 0.95, got %f", c.Confidence)
	}
}

func TestEngine_matchLayer2SizeTitle_NoFingerprint(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())
	adapter := &mocks.SiteAdapter{}
	rec := model.SeedingTorrentRecord{InfoHash: "nonexist", SiteName: "site1"}
	c := e.matchLayer2SearchVerify(context.Background(), adapter, &model.SiteConfig{}, rec.InfoHash, rec.SiteName, "site2", nil, nil)
	if c != nil {
		t.Error("expected nil when no fingerprint found")
	}
}

func TestEngine_matchLayer2SizeTitle_SizeMismatch(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	db.Create(&model.ContentFingerprint{
		InfoHash:  "ih_src2",
		SiteName:  "source_site",
		Title:     "Another.Movie.2023.1080p.BluRay.x264-GROUP2",
		TotalSize: 1073741824,
	})

	adapter := &mocks.SiteAdapter{
		SearchTorrentsFn: func(ctx context.Context, config *model.SiteConfig, query string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
			return []*model.SeedingSearchResult{
				{TorrentID: "t1", Title: "Another.Movie.2023.1080p.BluRay.x264-GROUP2", Size: 2147483648},
			}, nil
		},
	}

	rec := model.SeedingTorrentRecord{InfoHash: "ih_src2", SiteName: "source_site"}
	fc := e.preloadFingerprints(context.Background(), []string{rec.InfoHash})
	c := e.matchLayer2SizeTitle(context.Background(), adapter, &model.SiteConfig{}, rec.InfoHash, rec.SiteName, "target_site", 1.0, fc)
	if c != nil {
		t.Error("expected nil for size mismatch")
	}
}

func TestEngine_matchLayer3Fingerprint_WithDB(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	db.Create(&model.ContentFingerprint{
		InfoHash:   "ih_fp_src",
		SiteName:   "source_site",
		PiecesHash: "ph_match",
		TotalSize:  1073741824,
	})
	db.Create(&model.ContentFingerprint{
		InfoHash:   "ih_fp_tgt",
		SiteName:   "target_site",
		PiecesHash: "ph_match",
		TotalSize:  1073741824,
		TorrentID:  "t_fp_target",
	})

	rec := model.SeedingTorrentRecord{InfoHash: "ih_fp_src", SiteName: "source_site"}
	fc := e.preloadFingerprints(context.Background(), []string{rec.InfoHash})
	c := e.matchLayer3Fingerprint(context.Background(), rec.InfoHash, rec.SiteName, "target_site", fc)
	if c == nil {
		t.Fatal("expected candidate, got nil")
	}
	if c.MatchMethod != "fingerprint" {
		t.Errorf("expected fingerprint, got %s", c.MatchMethod)
	}
	if c.TargetTorrentID != "t_fp_target" {
		t.Errorf("expected t_fp_target, got %s", c.TargetTorrentID)
	}
	if c.Confidence != 1.0 {
		t.Errorf("expected confidence 1.0, got %f", c.Confidence)
	}
}

func TestEngine_matchLayer3Fingerprint_NoFingerprint(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())
	rec := model.SeedingTorrentRecord{InfoHash: "nonexist", SiteName: "site1"}
	c := e.matchLayer3Fingerprint(context.Background(), rec.InfoHash, rec.SiteName, "target_site", nil)
	if c != nil {
		t.Error("expected nil when no fingerprint found")
	}
}

func TestEngine_matchLayer3Fingerprint_EmptyPiecesHash(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())
	db.Create(&model.ContentFingerprint{
		InfoHash:  "ih_empty_ph",
		SiteName:  "source_site",
		TotalSize: 1073741824,
	})
	rec := model.SeedingTorrentRecord{InfoHash: "ih_empty_ph", SiteName: "source_site"}
	fc := e.preloadFingerprints(context.Background(), []string{rec.InfoHash})
	c := e.matchLayer3Fingerprint(context.Background(), rec.InfoHash, rec.SiteName, "target_site", fc)
	if c != nil {
		t.Error("expected nil when no pieces_hash and no files_hash")
	}
}

func makePreloadedSites(targetSite string, config *model.SiteConfig, adapter model.SiteAdapter) *preloadedSites {
	return &preloadedSites{
		configs:    map[string]*model.SiteConfig{targetSite: config},
		adapters:   map[string]model.SiteAdapter{targetSite: adapter},
		siteLimits: map[string]*model.Site{},
	}
}

func TestEngine_injectMatch_NoProvider(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())
	match := &model.ReseedMatch{ClientID: "c1", SourceSite: "s1", TargetSite: "s2"}
	task := &model.ReseedTask{}
	err := e.injectMatch(context.Background(), match, task, nil)
	if err == nil {
		t.Error("expected error when no providers set")
	}
}

func TestEngine_injectMatch_FailSiteInfo(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	match := &model.ReseedMatch{
		ClientID: "c1", SourceSite: "s1", SourceTorrentID: "t1", SourceInfoHash: "ih1",
		TargetSite: "s2", TargetTorrentID: "t2", MatchMethod: "pieces_hash",
		Confidence: 1.0, Status: model.MatchStatusMatched,
	}
	db.Create(match)
	task := &model.ReseedTask{Name: "inj-fail", Enabled: true}
	if err := e.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create: %v", err)
	}

	err := e.injectMatch(context.Background(), match, task, &preloadedSites{
		configs:    map[string]*model.SiteConfig{},
		adapters:   map[string]model.SiteAdapter{},
		siteLimits: map[string]*model.Site{},
	})
	if err == nil {
		t.Error("expected error when config not preloaded")
	}
	var updated model.ReseedMatch
	db.First(&updated, match.ID)
	if updated.Status != model.MatchStatusFailed {
		t.Errorf("expected failed status, got %s", updated.Status)
	}
	if updated.FailReason == "" {
		t.Error("expected non-empty fail reason")
	}
}

func TestEngine_injectMatch_Success(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	adapter := &mocks.SiteAdapter{
		DownloadTorrentFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error) {
			return []byte("fake torrent data"), nil
		},
	}
	sp := &mocks.SiteInfoProvider{
		GetSiteInfoFn: func(ctx context.Context, siteName string) (*model.SiteInfo, error) {
			return &model.SiteInfo{Name: "target_site", BaseURL: "https://target.example.com"}, nil
		},
		GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
			return &model.SiteConfig{Enabled: true}, nil
		},
		GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
			return adapter, nil
		},
	}
	client := &mocks.DownloaderClient{
		AddFromFileFn: func(ctx context.Context, data []byte, opts model.AddTorrentOptions) (*model.AddResult, error) {
			return &model.AddResult{InfoHash: "new_hash_123"}, nil
		},
		CheckExistsFn: func(ctx context.Context, infoHash string) (bool, error) {
			return false, nil
		},
		RecheckFn: func(ctx context.Context, hash string) error {
			return nil
		},
		GetTorrentByHashFn: func(ctx context.Context, hash string) (*model.TorrentInfo, error) {
			return &model.TorrentInfo{Hash: hash, Progress: 1.0, State: "pausedUP"}, nil
		},
		ResumeTorrentFn: func(ctx context.Context, hash string) error {
			return nil
		},
		DeleteTorrentFn: func(ctx context.Context, hash string, deleteFiles bool) error {
			return nil
		},
	}
	cp := &mocks.DownloaderProvider{
		GetFn: func(clientID string) (model.DownloaderClient, error) {
			return client, nil
		},
	}
	e.SetSiteProvider(sp)
	e.SetClientProvider(cp)

	match := &model.ReseedMatch{
		ClientID: "c1", SourceSite: "s1", SourceTorrentID: "t1", SourceInfoHash: "ih1",
		TargetSite: "target_site", TargetTorrentID: "t2", MatchMethod: "pieces_hash",
		Confidence: 1.0, Status: model.MatchStatusMatched,
	}
	db.Create(match)
	task := &model.ReseedTask{Name: "inj-ok", Enabled: true, ReseedCategory: "cross-seed"}
	if err := e.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create: %v", err)
	}

	ps := makePreloadedSites("target_site", &model.SiteConfig{Enabled: true}, adapter)
	err := e.injectMatch(context.Background(), match, task, ps)
	if err != nil {
		t.Fatalf("inject: %v", err)
	}
	var updated model.ReseedMatch
	db.First(&updated, match.ID)
	if updated.Status != model.MatchStatusInjected {
		t.Errorf("expected injected, got %s", updated.Status)
	}
	if updated.TargetInfoHash != "new_hash_123" {
		t.Errorf("expected new_hash_123, got %s", updated.TargetInfoHash)
	}
	if updated.InjectedAt == nil {
		t.Error("expected injected_at to be set")
	}
}

func TestEngine_injectMatch_AlreadyExists(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	adapter := &mocks.SiteAdapter{
		DownloadTorrentFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error) {
			return []byte("fake"), nil
		},
	}
	sp := &mocks.SiteInfoProvider{
		GetSiteInfoFn: func(ctx context.Context, siteName string) (*model.SiteInfo, error) {
			return &model.SiteInfo{Name: "target_site", BaseURL: "https://target.example.com"}, nil
		},
		GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
			return &model.SiteConfig{Enabled: true}, nil
		},
		GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
			return adapter, nil
		},
	}
	client := &mocks.DownloaderClient{
		AddFromFileFn: func(ctx context.Context, data []byte, opts model.AddTorrentOptions) (*model.AddResult, error) {
			return nil, fmt.Errorf("torrent already exists in client")
		},
		GetTorrentByHashFn: func(ctx context.Context, hash string) (*model.TorrentInfo, error) {
			return &model.TorrentInfo{Hash: hash, SavePath: "/data"}, nil
		},
	}
	cp := &mocks.DownloaderProvider{
		GetFn: func(clientID string) (model.DownloaderClient, error) {
			return client, nil
		},
	}
	e.SetSiteProvider(sp)
	e.SetClientProvider(cp)

	match := &model.ReseedMatch{
		ClientID: "c1", SourceSite: "s1", SourceTorrentID: "t1", SourceInfoHash: "ih1",
		TargetSite: "target_site", TargetTorrentID: "t2", MatchMethod: "pieces_hash",
		Confidence: 1.0, Status: model.MatchStatusMatched,
	}
	db.Create(match)
	task := &model.ReseedTask{Name: "inj-exists", Enabled: true}
	if err := e.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create: %v", err)
	}

	ps := makePreloadedSites("target_site", &model.SiteConfig{Enabled: true}, adapter)
	err := e.injectMatch(context.Background(), match, task, ps)
	if err != nil {
		t.Fatalf("unexpected error for already exists: %v", err)
	}
	var updated model.ReseedMatch
	db.First(&updated, match.ID)
	if updated.Status != model.MatchStatusInjected {
		t.Errorf("expected injected, got %s", updated.Status)
	}
	if updated.DecisionType != string(model.DecisionAlreadyExists) {
		t.Errorf("expected %s, got %s", model.DecisionAlreadyExists, updated.DecisionType)
	}
}

func TestEngine_queryIYUU_WithResults(t *testing.T) {
	db := setupReseedDBWithIYUU(t)
	e := NewEngine(db, zap.NewNop())

	db.Create(&model.IYUUSiteMapping{
		IYUUSid:    1,
		SiteDomain: "https://target.example.com",
		SiteName:   "target_site",
	})
	e.SetSiteProvider(&mocks.SiteInfoProvider{
		GetSiteInfoFn: func(ctx context.Context, siteName string) (*model.SiteInfo, error) {
			return &model.SiteInfo{Name: "target_site", BaseURL: "https://target.example.com"}, nil
		},
	})
	e.SetIYUUService(&mockIYUUService{
		queryReseedFn: func(ctx context.Context, infoHashes []string) ([]*model.IYUUReseedResult, error) {
			return []*model.IYUUReseedResult{
				{
					SourceInfoHash: "ih_source",
					Targets: []model.IYUUTarget{
						{Sid: 1, TorrentID: 100, InfoHash: "ih_target"},
					},
				},
			}, nil
		},
	})

	rec := model.SeedingTorrentRecord{InfoHash: "ih_source", SiteName: "source_site"}
	iyuuResults := map[string][]*model.IYUUReseedResult{
		"ih_source": {{
			SourceInfoHash: "ih_source",
			Targets: []model.IYUUTarget{
				{Sid: 1, TorrentID: 100, InfoHash: "ih_target"},
			},
		}},
	}
	candidates := e.filterIYUUResults(sourceTorrent{InfoHash: rec.InfoHash, SiteName: rec.SiteName}, iyuuResults, map[int]string{1: "target_site"}, []string{"target_site"}, nil)
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
	if candidates[0].MatchMethod != "iyuu" {
		t.Errorf("expected iyuu, got %s", candidates[0].MatchMethod)
	}
	if candidates[0].TargetTorrentID != "100" {
		t.Errorf("expected 100, got %s", candidates[0].TargetTorrentID)
	}
	if candidates[0].Confidence != 0.9 {
		t.Errorf("expected 0.9, got %f", candidates[0].Confidence)
	}
}

func TestEngine_queryIYUU_Error(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())
	rec := model.SeedingTorrentRecord{InfoHash: "ih1", SiteName: "s1"}
	candidates := e.filterIYUUResults(sourceTorrent{InfoHash: rec.InfoHash, SiteName: rec.SiteName}, nil, nil, nil, nil)
	if candidates != nil {
		t.Error("expected nil on IYUU error")
	}
}

func TestEngine_queryIYUU_ExcludedSites(t *testing.T) {
	db := setupReseedDBWithIYUU(t)
	e := NewEngine(db, zap.NewNop())

	db.Create(&model.IYUUSiteMapping{
		IYUUSid:    2,
		SiteDomain: "https://excluded.example.com",
		SiteName:   "excluded_site",
	})
	e.SetSiteProvider(&mocks.SiteInfoProvider{
		GetSiteInfoFn: func(ctx context.Context, siteName string) (*model.SiteInfo, error) {
			return &model.SiteInfo{Name: siteName}, nil
		},
	})
	e.SetIYUUService(&mockIYUUService{
		queryReseedFn: func(ctx context.Context, infoHashes []string) ([]*model.IYUUReseedResult, error) {
			return []*model.IYUUReseedResult{
				{
					SourceInfoHash: "ih1",
					Targets: []model.IYUUTarget{
						{Sid: 2, TorrentID: 200, InfoHash: "ih2"},
					},
				},
			}, nil
		},
	})

	rec := model.SeedingTorrentRecord{InfoHash: "ih1", SiteName: "source_site"}
	iyuuResults := map[string][]*model.IYUUReseedResult{
		"ih1": {{
			SourceInfoHash: "ih1",
			Targets: []model.IYUUTarget{
				{Sid: 2, TorrentID: 200, InfoHash: "ih2"},
			},
		}},
	}
	candidates := e.filterIYUUResults(sourceTorrent{InfoHash: rec.InfoHash, SiteName: rec.SiteName}, iyuuResults, map[int]string{2: "excluded_site"}, nil, []string{"excluded_site"})
	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates for excluded site, got %d", len(candidates))
	}
}

func TestEngine_queryIYUU_SameSite(t *testing.T) {
	db := setupReseedDBWithIYUU(t)
	e := NewEngine(db, zap.NewNop())

	db.Create(&model.IYUUSiteMapping{
		IYUUSid:    3,
		SiteDomain: "https://source.example.com",
		SiteName:   "source_site",
	})
	e.SetSiteProvider(&mocks.SiteInfoProvider{
		GetSiteInfoFn: func(ctx context.Context, siteName string) (*model.SiteInfo, error) {
			return &model.SiteInfo{Name: siteName}, nil
		},
	})
	e.SetIYUUService(&mockIYUUService{
		queryReseedFn: func(ctx context.Context, infoHashes []string) ([]*model.IYUUReseedResult, error) {
			return []*model.IYUUReseedResult{
				{
					SourceInfoHash: "ih1",
					Targets: []model.IYUUTarget{
						{Sid: 3, TorrentID: 300, InfoHash: "ih3"},
					},
				},
			}, nil
		},
	})

	rec := model.SeedingTorrentRecord{InfoHash: "ih1", SiteName: "source_site"}
	iyuuResults := map[string][]*model.IYUUReseedResult{
		"ih1": {{
			SourceInfoHash: "ih1",
			Targets: []model.IYUUTarget{
				{Sid: 3, TorrentID: 300, InfoHash: "ih3"},
			},
		}},
	}
	candidates := e.filterIYUUResults(sourceTorrent{InfoHash: rec.InfoHash, SiteName: rec.SiteName}, iyuuResults, map[int]string{3: "source_site"}, nil, nil)
	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates for same site, got %d", len(candidates))
	}
}

func TestEngine_iyuuSidToSite_WithMapping(t *testing.T) {
	db := setupReseedDBWithIYUU(t)
	e := NewEngine(db, zap.NewNop())

	db.Create(&model.IYUUSiteMapping{
		IYUUSid:    5,
		SiteDomain: "https://site5.example.com",
		SiteName:   "site5",
	})
	e.SetSiteProvider(&mocks.SiteInfoProvider{
		GetSiteInfoFn: func(ctx context.Context, siteName string) (*model.SiteInfo, error) {
			return &model.SiteInfo{Name: siteName}, nil
		},
	})

	result := e.iyuuSidToSite(context.Background(), 5)
	if result != "site5" {
		t.Errorf("expected site5, got %s", result)
	}
}

func TestEngine_iyuuSidToSite_NoMapping(t *testing.T) {
	db := setupReseedDBWithIYUU(t)
	e := NewEngine(db, zap.NewNop())
	result := e.iyuuSidToSite(context.Background(), 999)
	if result != "" {
		t.Errorf("expected empty string, got %s", result)
	}
}

func TestEngine_iyuuSidToSite_FallbackByURL(t *testing.T) {
	db := setupReseedDBWithIYUU(t)
	e := NewEngine(db, zap.NewNop())

	db.Create(&model.IYUUSiteMapping{
		IYUUSid:    7,
		SiteDomain: "https://site7.example.com",
		SiteName:   "site7",
	})
	e.SetSiteProvider(&mocks.SiteInfoProvider{
		GetSiteInfoFn: func(ctx context.Context, siteName string) (*model.SiteInfo, error) {
			return nil, fmt.Errorf("not found by name")
		},
		GetSiteInfoByURLFn: func(ctx context.Context, baseURL string) (*model.SiteInfo, error) {
			return &model.SiteInfo{Name: "site7"}, nil
		},
	})

	result := e.iyuuSidToSite(context.Background(), 7)
	if result != "site7" {
		t.Errorf("expected site7, got %s", result)
	}
}

func TestEngine_findCandidates_ExcludedAndDisabled(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())
	e.SetSiteProvider(&mocks.SiteInfoProvider{
		ListSitesFn: func(ctx context.Context) ([]*model.SiteInfo, error) {
			return []*model.SiteInfo{
				{Name: "excluded", BaseURL: "https://ex.example.com", Enabled: true},
				{Name: "disabled", BaseURL: "https://dis.example.com", Enabled: false},
				{Name: "source_site", BaseURL: "https://src.example.com", Enabled: true},
			}, nil
		},
		GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
			return &model.SiteConfig{Enabled: true}, nil
		},
		GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
			return &mocks.SiteAdapter{
				SearchTorrentsFn: func(ctx context.Context, config *model.SiteConfig, query string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
					return []*model.SeedingSearchResult{{TorrentID: "t1", Size: 100}}, nil
				},
				GetTorrentInfoHashFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) (string, error) {
					return "", nil
				},
			}, nil
		},
	})

	task := &model.ReseedTask{Name: "fc-excl", Enabled: true}
	rec := model.SeedingTorrentRecord{InfoHash: "ih1", SiteName: "source_site"}
	candidates := e.findCandidates(context.Background(), sourceTorrent{InfoHash: rec.InfoHash, SiteName: rec.SiteName}, e.preloadSites(context.Background(), nil, []string{"excluded"}), nil, 1.0, task, nil, nil, nil, nil, nil)
	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates (excluded+disabled+same site), got %d", len(candidates))
	}
}

func TestEngine_findCandidates_WithTargetSites(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	adapter := &mocks.SiteAdapter{
		SearchTorrentsFn: func(ctx context.Context, config *model.SiteConfig, query string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
			return []*model.SeedingSearchResult{{TorrentID: "t1", Title: "Test.Torrent.2024.1080p.BluRay.x264-GROUPZ", Size: 1073741824}}, nil
		},
	}
	e.SetSiteProvider(&mocks.SiteInfoProvider{
		GetSiteInfoFn: func(ctx context.Context, siteName string) (*model.SiteInfo, error) {
			return &model.SiteInfo{Name: siteName, BaseURL: "https://" + siteName + ".com", Enabled: true}, nil
		},
		GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
			return &model.SiteConfig{Enabled: true}, nil
		},
		GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
			return adapter, nil
		},
	})

	fpRepo := fingerprint.NewRepository(db, zap.NewNop())
	if err := fpRepo.Save(context.Background(), &model.ContentFingerprint{
		InfoHash: "ih_match", SiteName: "source_site",
		Title: "Test.Torrent.2024.1080p.BluRay.x264-GROUPZ", TotalSize: 1073741824,
	}); err != nil {
		t.Fatal(err)
	}
	e.SetFingerprintRepo(fpRepo)

	task := &model.ReseedTask{Name: "fc-targets", Enabled: true, MatchMethods: "size_title"}
	candidates := e.findCandidates(context.Background(), sourceTorrent{InfoHash: "ih_match", SiteName: "source_site"}, e.preloadSites(context.Background(), []string{"site_a"}, nil), e.preloadFingerprints(context.Background(), []string{"ih_match"}), 1.0, task, nil, nil, nil, nil, nil)
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
	if candidates[0].TargetSite != "site_a" {
		t.Errorf("expected site_a, got %s", candidates[0].TargetSite)
	}
}

func TestEngine_ListByClientID(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	e.CreateTask(context.Background(), &model.ReseedTask{Name: "t1", Enabled: true, ClientIDs: "c1"})
	e.CreateTask(context.Background(), &model.ReseedTask{Name: "t2", Enabled: true, ClientIDs: "c1,c2"})
	e.CreateTask(context.Background(), &model.ReseedTask{Name: "t3", Enabled: true, ClientIDs: "c2,c3"})
	e.CreateTask(context.Background(), &model.ReseedTask{Name: "t4", Enabled: true, ClientIDs: "c3"})

	tasks, err := e.ListByClientID(context.Background(), "c1")
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks for c1, got %d", len(tasks))
	}

	tasks, err = e.ListByClientID(context.Background(), "c2")
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks for c2, got %d", len(tasks))
	}

	tasks, err = e.ListByClientID(context.Background(), "c3")
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks for c3, got %d", len(tasks))
	}

	tasks, err = e.ListByClientID(context.Background(), "c99")
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks for c99, got %d", len(tasks))
	}
}

func TestEngine_ListEnabled(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())
	ctx := context.Background()

	e.CreateTask(ctx, &model.ReseedTask{Name: "idle1", Enabled: true, ClientIDs: "c1"})
	e.CreateTask(ctx, &model.ReseedTask{Name: "running1", Enabled: true, ClientIDs: "c1"})
	task3 := &model.ReseedTask{Name: "completed1", Enabled: true, ClientIDs: "c1"}
	e.CreateTask(ctx, task3)
	e.db.Model(task3).Update("status", model.ReseedTaskCompleted)
	e.CreateTask(ctx, &model.ReseedTask{Name: "disabled1", Enabled: false, ClientIDs: "c1"})

	tasks, err := e.ListEnabled(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Errorf("expected 2 enabled tasks (idle+running), got %d", len(tasks))
	}
}

func TestEngine_BatchSaveMatches(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	matches := []*model.ReseedMatch{
		{ClientID: "c1", SourceSite: "s1", SourceTorrentID: "t1", SourceInfoHash: "ih1",
			TargetSite: "s2", TargetTorrentID: "t2", MatchMethod: "pieces_hash",
			Confidence: 1.0, Status: model.MatchStatusMatched},
		{ClientID: "c1", SourceSite: "s1", SourceTorrentID: "t3", SourceInfoHash: "ih3",
			TargetSite: "s2", TargetTorrentID: "t4", MatchMethod: "size_title",
			Confidence: 0.85, Status: model.MatchStatusMatched},
	}
	if err := e.BatchSaveMatches(context.Background(), matches); err != nil {
		t.Fatal(err)
	}

	var count int64
	db.Model(&model.ReseedMatch{}).Count(&count)
	if count != 2 {
		t.Errorf("expected 2 matches, got %d", count)
	}
}

func TestEngine_UpdateMatchStatus(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	db.Create(&model.ReseedMatch{
		ClientID: "c1", SourceSite: "s1", SourceTorrentID: "t1", SourceInfoHash: "ih1",
		TargetSite: "s2", TargetTorrentID: "t2", MatchMethod: "pieces_hash",
		Confidence: 1.0, Status: model.MatchStatusMatched,
	})

	if err := e.UpdateMatchStatus(context.Background(), 1, string(model.MatchStatusFailed), "inject error"); err != nil {
		t.Fatal(err)
	}

	var m model.ReseedMatch
	db.First(&m, 1)
	if m.Status != model.MatchStatusFailed {
		t.Errorf("expected failed, got %s", m.Status)
	}
	if m.FailReason != "inject error" {
		t.Errorf("expected 'inject error', got %s", m.FailReason)
	}
}

func TestEngine_CancelTask_Existing(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	e.CreateTask(context.Background(), &model.ReseedTask{Name: "cancel-exist", Enabled: true, ClientIDs: "c1"})
	ctx := context.Background()
	e.Start(ctx)

	if _, ok := e.tasks[1]; !ok {
		t.Fatal("expected task 1 to be registered")
	}

	e.CancelTask(1)

	if _, ok := e.tasks[1]; ok {
		t.Error("expected task 1 to be removed after cancel")
	}
}

func TestEngine_Start_ReplaceExisting(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	e.CreateTask(context.Background(), &model.ReseedTask{Name: "replace", Enabled: true, ClientIDs: "c1"})
	ctx := context.Background()
	e.Start(ctx)

	e.Start(ctx)

	if len(e.tasks) != 1 {
		t.Errorf("expected 1 task after double start, got %d", len(e.tasks))
	}
}

func TestEngine_RunTask_BlockedTitle(t *testing.T) {
	db := setupReseedDB(t)
	torrents := []*model.TorrentInfo{
		{Hash: "ih_blocked", TrackerURL: "https://tracker.site1.com/announce", Progress: 1.0},
	}
	e := setupEngineWithMockClient(t, db, torrents, map[string]string{
		"site1.com": "site1",
	})
	e.SetFingerprintRepo(fingerprint.NewRepository(db, zap.NewNop()))

	db.Create(&model.ContentFingerprint{
		InfoHash: "ih_blocked", SiteName: "site1",
		Title: "Some.Title.禁转.2023", TotalSize: 1073741824,
	})

	task := &model.ReseedTask{Name: "blocked-test", Enabled: true, ClientIDs: "c1"}
	e.CreateTask(context.Background(), task)

	result, err := e.RunTask(context.Background(), task)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Blocked != 1 {
		t.Errorf("expected 1 blocked, got %d", result.Blocked)
	}
}

func TestEngine_RunTask_WithProviders(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	adapter := &mocks.SiteAdapter{
		SearchTorrentsFn: func(ctx context.Context, config *model.SiteConfig, query string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
			return []*model.SeedingSearchResult{{TorrentID: "t_tgt", Title: "Normal.Title.2024.1080p.BluRay.x264-GROUPF", Size: 1073741824}}, nil
		},
		GetTorrentInfoHashFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) (string, error) {
			return "ih_match", nil
		},
		DownloadTorrentFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error) {
			return []byte("torrent-data"), nil
		},
	}
	sp := &mocks.SiteInfoProvider{
		GetSiteInfoFn: func(ctx context.Context, siteName string) (*model.SiteInfo, error) {
			return &model.SiteInfo{Name: siteName, BaseURL: "https://" + siteName + ".com", Enabled: true}, nil
		},
		GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
			return &model.SiteConfig{Enabled: true}, nil
		},
		GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
			return adapter, nil
		},
	}
	dlClient := &mocks.DownloaderClient{
		AddFromFileFn: func(ctx context.Context, data []byte, opts model.AddTorrentOptions) (*model.AddResult, error) {
			return &model.AddResult{InfoHash: "new_ih"}, nil
		},
		RecheckFn: func(ctx context.Context, hash string) error {
			return nil
		},
		GetTorrentByHashFn: func(ctx context.Context, hash string) (*model.TorrentInfo, error) {
			return &model.TorrentInfo{Hash: hash, Progress: 1.0, State: "pausedUP"}, nil
		},
		ResumeTorrentFn: func(ctx context.Context, hash string) error {
			return nil
		},
		DeleteTorrentFn: func(ctx context.Context, hash string, deleteFiles bool) error {
			return nil
		},
		GetAllTorrentsFn: func(ctx context.Context) ([]*model.TorrentInfo, error) {
			return []*model.TorrentInfo{
				{Hash: "ih_full", TrackerURL: "https://tracker.site1.com/announce", Progress: 1.0},
			}, nil
		},
	}
	cp := &mocks.DownloaderProvider{
		GetFn: func(clientID string) (model.DownloaderClient, error) {
			return dlClient, nil
		},
	}
	e.SetSiteProvider(sp)
	e.SetClientProvider(cp)

	resolver := NewTrackerSiteResolver()
	resolver.BuildIndex([]*model.Site{{Domain: "site1.com", Name: "site1"}})
	e.SetTrackerResolver(resolver)

	task := &model.ReseedTask{
		Name: "full-run", Enabled: true, ClientIDs: "c1",
		TargetSiteIDs: "target_site", ReseedCategory: "cross-seed",
		InjectionIntervalS: 0,
	}
	e.CreateTask(context.Background(), task)

	db.Create(&model.ContentFingerprint{
		InfoHash: "ih_full", SiteName: "site1",
		Title: "Normal.Title.2024.1080p.BluRay.x264-GROUPF", TotalSize: 1073741824,
	})

	result, err := e.RunTask(context.Background(), task)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Injected != 1 {
		t.Errorf("expected 1 injected, got %d", result.Injected)
	}
}

func TestEngine_RunTask_MaxInjectionsLimit(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	adapter := &mocks.SiteAdapter{
		SearchTorrentsFn: func(ctx context.Context, config *model.SiteConfig, query string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
			return []*model.SeedingSearchResult{{TorrentID: "tgt1", Size: 1073741824}}, nil
		},
		GetTorrentInfoHashFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) (string, error) {
			return "ih_match", nil
		},
	}
	e.SetSiteProvider(&mocks.SiteInfoProvider{
		GetSiteInfoFn: func(ctx context.Context, siteName string) (*model.SiteInfo, error) {
			return &model.SiteInfo{Name: siteName, BaseURL: "https://" + siteName + ".com", Enabled: true}, nil
		},
		GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
			return &model.SiteConfig{Enabled: true}, nil
		},
		GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
			return adapter, nil
		},
	})

	task := &model.ReseedTask{
		Name: "max-inj", Enabled: true, ClientIDs: "c1",
		TargetSiteIDs: "target_site", MaxInjectionsPerRun: 1,
	}
	e.CreateTask(context.Background(), task)

	db.Create(&model.SeedingTorrentRecord{
		ClientID: "c1", InfoHash: "ih1", SiteName: "site1",
		TorrentID: "t1", Status: model.SeedingStatusSeeding,
	})
	db.Create(&model.SeedingTorrentRecord{
		ClientID: "c1", InfoHash: "ih2", SiteName: "site1",
		TorrentID: "t2", Status: model.SeedingStatusSeeding,
	})

	result, err := e.RunTask(context.Background(), task)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Matched > 1 {
		t.Errorf("expected at most 1 matched with limit, got %d", result.Matched)
	}
}

func TestEngine_RunTask_NoRecords(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	task := &model.ReseedTask{Name: "no-recs", Enabled: true, ClientIDs: "c1"}
	e.CreateTask(context.Background(), task)

	result, err := e.RunTask(context.Background(), task)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.TotalSources != 0 {
		t.Errorf("expected 0 sources, got %d", result.TotalSources)
	}
}

func TestEngine_RunTask_DefaultSizeTolerance(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	task := &model.ReseedTask{
		Name: "default-tol", Enabled: true, ClientIDs: "c1",
		SizeTolerancePercent: 0,
	}
	e.CreateTask(context.Background(), task)

	e.SetClientProvider(&mocks.DownloaderProvider{
		GetFn: func(clientID string) (model.DownloaderClient, error) {
			return &mocks.DownloaderClient{
				Name: "c1",
				GetAllTorrentsFn: func(ctx context.Context) ([]*model.TorrentInfo, error) {
					return []*model.TorrentInfo{
						{Hash: "ih1", TrackerURL: "https://tracker.site1.com/announce", Progress: 1.0},
					}, nil
				},
			}, nil
		},
	})

	resolver := NewTrackerSiteResolver()
	resolver.BuildIndex([]*model.Site{
		{Domain: "site1.com", Name: "site1"},
	})
	e.SetTrackerResolver(resolver)

	result, err := e.RunTask(context.Background(), task)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.TotalSources != 1 {
		t.Errorf("expected 1 source, got %d", result.TotalSources)
	}
}

func TestEngine_findCandidates_GetSiteInfoError(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())
	e.SetSiteProvider(&mocks.SiteInfoProvider{
		GetSiteInfoFn: func(ctx context.Context, siteName string) (*model.SiteInfo, error) {
			return nil, fmt.Errorf("site not found")
		},
	})

	task := &model.ReseedTask{Name: "fc-err", Enabled: true}
	rec := model.SeedingTorrentRecord{InfoHash: "ih1", SiteName: "source_site"}
	candidates := e.findCandidates(context.Background(), sourceTorrent{InfoHash: rec.InfoHash, SiteName: rec.SiteName}, e.preloadSites(context.Background(), []string{"bad_site"}, nil), nil, 1.0, task, nil, nil, nil, nil, nil)
	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates when GetSiteInfo fails, got %d", len(candidates))
	}
}

func TestEngine_findCandidates_ListSitesError(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())
	e.SetSiteProvider(&mocks.SiteInfoProvider{
		ListSitesFn: func(ctx context.Context) ([]*model.SiteInfo, error) {
			return nil, fmt.Errorf("db error")
		},
	})

	task := &model.ReseedTask{Name: "fc-lserr", Enabled: true}
	rec := model.SeedingTorrentRecord{InfoHash: "ih1", SiteName: "source_site"}
	candidates := e.findCandidates(context.Background(), sourceTorrent{InfoHash: rec.InfoHash, SiteName: rec.SiteName}, e.preloadSites(context.Background(), nil, nil), nil, 1.0, task, nil, nil, nil, nil, nil)
	if candidates != nil {
		t.Error("expected nil when ListSites fails")
	}
}

func TestEngine_findCandidates_GetSiteConfigError(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())
	e.SetSiteProvider(&mocks.SiteInfoProvider{
		ListSitesFn: func(ctx context.Context) ([]*model.SiteInfo, error) {
			return []*model.SiteInfo{{Name: "tgt", BaseURL: "https://tgt.com", Enabled: true}}, nil
		},
		GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
			return nil, fmt.Errorf("config error")
		},
	})

	task := &model.ReseedTask{Name: "fc-cfgerr", Enabled: true}
	rec := model.SeedingTorrentRecord{InfoHash: "ih1", SiteName: "source_site"}
	candidates := e.findCandidates(context.Background(), sourceTorrent{InfoHash: rec.InfoHash, SiteName: rec.SiteName}, e.preloadSites(context.Background(), nil, nil), nil, 1.0, task, nil, nil, nil, nil, nil)
	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates when GetSiteConfig fails, got %d", len(candidates))
	}
}

func TestEngine_findCandidates_GetAdapterError(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())
	e.SetSiteProvider(&mocks.SiteInfoProvider{
		ListSitesFn: func(ctx context.Context) ([]*model.SiteInfo, error) {
			return []*model.SiteInfo{{Name: "tgt", BaseURL: "https://tgt.com", Enabled: true}}, nil
		},
		GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
			return &model.SiteConfig{Enabled: true}, nil
		},
		GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
			return nil, fmt.Errorf("adapter error")
		},
	})

	task := &model.ReseedTask{Name: "fc-adaperr", Enabled: true}
	rec := model.SeedingTorrentRecord{InfoHash: "ih1", SiteName: "source_site"}
	candidates := e.findCandidates(context.Background(), sourceTorrent{InfoHash: rec.InfoHash, SiteName: rec.SiteName}, e.preloadSites(context.Background(), nil, nil), nil, 1.0, task, nil, nil, nil, nil, nil)
	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates when GetAdapter fails, got %d", len(candidates))
	}
}

func TestEngine_matchLayer2SizeTitle_EmptyKeyword(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	db.Create(&model.ContentFingerprint{
		InfoHash: "ih_empty_title", SiteName: "source_site",
		Title: "", TotalSize: 1073741824,
	})

	adapter := &mocks.SiteAdapter{}
	rec := model.SeedingTorrentRecord{InfoHash: "ih_empty_title", SiteName: "source_site"}
	fc := e.preloadFingerprints(context.Background(), []string{rec.InfoHash})
	c := e.matchLayer2SizeTitle(context.Background(), adapter, &model.SiteConfig{}, rec.InfoHash, rec.SiteName, "target_site", 1.0, fc)
	if c != nil {
		t.Error("expected nil for empty normalized title")
	}
}
func TestEngine_matchLayer2SizeTitle_SearchError(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	db.Create(&model.ContentFingerprint{
		InfoHash: "ih_search_err", SiteName: "source_site",
		Title: "Some.Movie.2023.1080p.BluRay.x264-GROUP3", TotalSize: 1073741824,
	})

	adapter := &mocks.SiteAdapter{
		SearchTorrentsFn: func(ctx context.Context, config *model.SiteConfig, query string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
			return nil, fmt.Errorf("search error")
		},
	}
	rec := model.SeedingTorrentRecord{InfoHash: "ih_search_err", SiteName: "source_site"}
	fc := e.preloadFingerprints(context.Background(), []string{rec.InfoHash})
	c := e.matchLayer2SizeTitle(context.Background(), adapter, &model.SiteConfig{}, rec.InfoHash, rec.SiteName, "target_site", 1.0, fc)
	if c != nil {
		t.Error("expected nil on search error")
	}
}

func TestEngine_matchLayer2SizeTitle_GroupMismatch(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	db.Create(&model.ContentFingerprint{
		InfoHash: "ih_best", SiteName: "source_site",
		Title: "Best.Movie.2023.1080p.BluRay.x264-GROUPA", TotalSize: 1073741824,
	})

	adapter := &mocks.SiteAdapter{
		SearchTorrentsFn: func(ctx context.Context, config *model.SiteConfig, query string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
			return []*model.SeedingSearchResult{
				{TorrentID: "t1", Title: "Best.Movie.2023.1080p.BluRay.x264-GROUPB", Size: 1073741824},
			}, nil
		},
	}
	rec := model.SeedingTorrentRecord{InfoHash: "ih_best", SiteName: "source_site"}
	fc := e.preloadFingerprints(context.Background(), []string{rec.InfoHash})
	c := e.matchLayer2SizeTitle(context.Background(), adapter, &model.SiteConfig{}, rec.InfoHash, rec.SiteName, "target_site", 1.0, fc)
	if c != nil {
		t.Error("expected nil when group name mismatch")
	}
}

func TestEngine_matchLayer2SizeTitle_NoTorrentID(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	db.Create(&model.ContentFingerprint{
		InfoHash: "ih_notid", SiteName: "source_site",
		Title: "Some.Title.2023.1080p.BluRay.x264-GROUP4", TotalSize: 1073741824,
	})

	adapter := &mocks.SiteAdapter{
		SearchTorrentsFn: func(ctx context.Context, config *model.SiteConfig, query string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
			return []*model.SeedingSearchResult{{TorrentID: "", Title: "Some.Title.2023.1080p.BluRay.x264-GROUP4", Size: 1073741824}}, nil
		},
	}
	rec := model.SeedingTorrentRecord{InfoHash: "ih_notid", SiteName: "source_site"}
	fc := e.preloadFingerprints(context.Background(), []string{rec.InfoHash})
	c := e.matchLayer2SizeTitle(context.Background(), adapter, &model.SiteConfig{}, rec.InfoHash, rec.SiteName, "target_site", 1.0, fc)
	if c != nil {
		t.Error("expected nil when no torrent ID in results")
	}
}

func TestEngine_matchLayer3Fingerprint_SizeOnlyMatch(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	db.Create(&model.ContentFingerprint{
		InfoHash: "ih_size_src", SiteName: "source_site",
		FilesHash: "fh_match", TotalSize: 1073741824,
	})
	db.Create(&model.ContentFingerprint{
		InfoHash: "ih_size_tgt", SiteName: "target_site",
		FilesHash: "fh_other", TotalSize: 1073741824,
		TorrentID: "t_size_target",
	})

	rec := model.SeedingTorrentRecord{InfoHash: "ih_size_src", SiteName: "source_site"}
	fc := e.preloadFingerprints(context.Background(), []string{rec.InfoHash})
	c := e.matchLayer3Fingerprint(context.Background(), rec.InfoHash, rec.SiteName, "target_site", fc)
	if c == nil {
		t.Fatal("expected candidate via size-only match, got nil")
	}
	if c.Confidence != 0.7 {
		t.Errorf("expected 0.7, got %f", c.Confidence)
	}
}

func TestEngine_matchLayer3Fingerprint_NoTorrentID(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	db.Create(&model.ContentFingerprint{
		InfoHash: "ih_notid_src", SiteName: "source_site",
		PiecesHash: "ph_no_tid", TotalSize: 1073741824,
	})
	db.Create(&model.ContentFingerprint{
		InfoHash: "ih_notid_tgt", SiteName: "target_site",
		PiecesHash: "ph_no_tid", TotalSize: 1073741824,
		TorrentID: "",
	})

	rec := model.SeedingTorrentRecord{InfoHash: "ih_notid_src", SiteName: "source_site"}
	fc := e.preloadFingerprints(context.Background(), []string{rec.InfoHash})
	c := e.matchLayer3Fingerprint(context.Background(), rec.InfoHash, rec.SiteName, "target_site", fc)
	if c != nil {
		t.Error("expected nil when no torrent ID on target")
	}
}

func TestEngine_injectMatch_DownloadError(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	adapter := &mocks.SiteAdapter{
		DownloadTorrentFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error) {
			return nil, fmt.Errorf("download failed")
		},
	}
	sp := &mocks.SiteInfoProvider{
		GetSiteInfoFn: func(ctx context.Context, siteName string) (*model.SiteInfo, error) {
			return &model.SiteInfo{Name: "tgt", BaseURL: "https://tgt.com"}, nil
		},
		GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
			return &model.SiteConfig{Enabled: true}, nil
		},
		GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
			return adapter, nil
		},
	}
	e.SetSiteProvider(sp)
	e.SetClientProvider(&mocks.DownloaderProvider{
		GetFn: func(clientID string) (model.DownloaderClient, error) {
			return &mocks.DownloaderClient{}, nil
		},
	})

	match := &model.ReseedMatch{
		ClientID: "c1", SourceSite: "s1", SourceTorrentID: "t1", SourceInfoHash: "ih1",
		TargetSite: "tgt", TargetTorrentID: "t2", MatchMethod: "pieces_hash",
		Confidence: 1.0, Status: model.MatchStatusMatched,
	}
	db.Create(match)
	task := &model.ReseedTask{Name: "inj-dlerr", Enabled: true}
	e.CreateTask(context.Background(), task)

	ps := makePreloadedSites("tgt", &model.SiteConfig{Enabled: true}, adapter)
	err := e.injectMatch(context.Background(), match, task, ps)
	if err == nil {
		t.Error("expected error when download fails")
	}
}

func TestEngine_injectMatch_EmptyTorrentData(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	adapter := &mocks.SiteAdapter{
		DownloadTorrentFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error) {
			return []byte{}, nil
		},
	}
	sp := &mocks.SiteInfoProvider{
		GetSiteInfoFn: func(ctx context.Context, siteName string) (*model.SiteInfo, error) {
			return &model.SiteInfo{Name: "tgt", BaseURL: "https://tgt.com"}, nil
		},
		GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
			return &model.SiteConfig{Enabled: true}, nil
		},
		GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
			return adapter, nil
		},
	}
	e.SetSiteProvider(sp)
	e.SetClientProvider(&mocks.DownloaderProvider{
		GetFn: func(clientID string) (model.DownloaderClient, error) {
			return &mocks.DownloaderClient{}, nil
		},
	})

	match := &model.ReseedMatch{
		ClientID: "c1", SourceSite: "s1", SourceTorrentID: "t1", SourceInfoHash: "ih1",
		TargetSite: "tgt", TargetTorrentID: "t2", MatchMethod: "pieces_hash",
		Confidence: 1.0, Status: model.MatchStatusMatched,
	}
	db.Create(match)
	task := &model.ReseedTask{Name: "inj-empty", Enabled: true}
	e.CreateTask(context.Background(), task)

	ps := makePreloadedSites("tgt", &model.SiteConfig{Enabled: true}, adapter)
	err := e.injectMatch(context.Background(), match, task, ps)
	if err == nil {
		t.Error("expected error when torrent data is empty")
	}
}

func TestEngine_injectMatch_AddFromFileError(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	adapter := &mocks.SiteAdapter{
		DownloadTorrentFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error) {
			return []byte("data"), nil
		},
	}
	sp := &mocks.SiteInfoProvider{
		GetSiteInfoFn: func(ctx context.Context, siteName string) (*model.SiteInfo, error) {
			return &model.SiteInfo{Name: "tgt", BaseURL: "https://tgt.com"}, nil
		},
		GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
			return &model.SiteConfig{Enabled: true}, nil
		},
		GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
			return adapter, nil
		},
	}
	dlClient := &mocks.DownloaderClient{
		AddFromFileFn: func(ctx context.Context, data []byte, opts model.AddTorrentOptions) (*model.AddResult, error) {
			return nil, fmt.Errorf("add failed: connection refused")
		},
	}
	e.SetSiteProvider(sp)
	e.SetClientProvider(&mocks.DownloaderProvider{
		GetFn: func(clientID string) (model.DownloaderClient, error) {
			return dlClient, nil
		},
	})

	match := &model.ReseedMatch{
		ClientID: "c1", SourceSite: "s1", SourceTorrentID: "t1", SourceInfoHash: "ih1",
		TargetSite: "tgt", TargetTorrentID: "t2", MatchMethod: "pieces_hash",
		Confidence: 1.0, Status: model.MatchStatusMatched,
	}
	db.Create(match)
	task := &model.ReseedTask{Name: "inj-adderr", Enabled: true}
	e.CreateTask(context.Background(), task)

	ps := makePreloadedSites("tgt", &model.SiteConfig{Enabled: true}, adapter)
	err := e.injectMatch(context.Background(), match, task, ps)
	if err == nil {
		t.Error("expected error when AddFromFile fails")
	}
	var updated model.ReseedMatch
	db.First(&updated, match.ID)
	if updated.Status != model.MatchStatusFailed {
		t.Errorf("expected failed, got %s", updated.Status)
	}
}

func TestEngine_injectMatch_GetClientError(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	adapter := &mocks.SiteAdapter{
		DownloadTorrentFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error) {
			return []byte("data"), nil
		},
	}
	sp := &mocks.SiteInfoProvider{
		GetSiteInfoFn: func(ctx context.Context, siteName string) (*model.SiteInfo, error) {
			return &model.SiteInfo{Name: "tgt", BaseURL: "https://tgt.com"}, nil
		},
		GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
			return &model.SiteConfig{Enabled: true}, nil
		},
		GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
			return adapter, nil
		},
	}
	e.SetSiteProvider(sp)
	e.SetClientProvider(&mocks.DownloaderProvider{
		GetFn: func(clientID string) (model.DownloaderClient, error) {
			return nil, fmt.Errorf("client not found")
		},
	})

	match := &model.ReseedMatch{
		ClientID: "c1", SourceSite: "s1", SourceTorrentID: "t1", SourceInfoHash: "ih1",
		TargetSite: "tgt", TargetTorrentID: "t2", MatchMethod: "pieces_hash",
		Confidence: 1.0, Status: model.MatchStatusMatched,
	}
	db.Create(match)
	task := &model.ReseedTask{Name: "inj-clerr", Enabled: true}
	e.CreateTask(context.Background(), task)

	ps := makePreloadedSites("tgt", &model.SiteConfig{Enabled: true}, adapter)
	err := e.injectMatch(context.Background(), match, task, ps)
	if err == nil {
		t.Error("expected error when GetClient fails")
	}
}

func TestEngine_queryIYUU_TargetSiteFilter(t *testing.T) {
	db := setupReseedDBWithIYUU(t)
	e := NewEngine(db, zap.NewNop())

	db.Create(&model.IYUUSiteMapping{
		IYUUSid: 10, SiteDomain: "https://a.com", SiteName: "site_a",
	})
	db.Create(&model.IYUUSiteMapping{
		IYUUSid: 11, SiteDomain: "https://b.com", SiteName: "site_b",
	})
	e.SetSiteProvider(&mocks.SiteInfoProvider{
		GetSiteInfoFn: func(ctx context.Context, siteName string) (*model.SiteInfo, error) {
			return &model.SiteInfo{Name: siteName}, nil
		},
	})
	e.SetIYUUService(&mockIYUUService{
		queryReseedFn: func(ctx context.Context, infoHashes []string) ([]*model.IYUUReseedResult, error) {
			return []*model.IYUUReseedResult{
				{
					SourceInfoHash: "ih1",
					Targets: []model.IYUUTarget{
						{Sid: 10, TorrentID: 100, InfoHash: "ih_a"},
						{Sid: 11, TorrentID: 200, InfoHash: "ih_b"},
					},
				},
			}, nil
		},
	})

	rec := model.SeedingTorrentRecord{InfoHash: "ih1", SiteName: "src"}
	iyuuResults := map[string][]*model.IYUUReseedResult{
		"ih1": {{
			SourceInfoHash: "ih1",
			Targets: []model.IYUUTarget{
				{Sid: 10, TorrentID: 100, InfoHash: "ih_a"},
				{Sid: 11, TorrentID: 200, InfoHash: "ih_b"},
			},
		}},
	}
	candidates := e.filterIYUUResults(sourceTorrent{InfoHash: rec.InfoHash, SiteName: rec.SiteName}, iyuuResults, map[int]string{10: "site_a", 11: "site_b"}, []string{"site_a"}, nil)
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate (target filter), got %d", len(candidates))
	}
	if candidates[0].TargetSite != "site_a" {
		t.Errorf("expected site_a, got %s", candidates[0].TargetSite)
	}
}

func TestEngine_queryIYUU_WrongSourceHash(t *testing.T) {
	db := setupReseedDBWithIYUU(t)
	e := NewEngine(db, zap.NewNop())
	e.SetIYUUService(&mockIYUUService{
		queryReseedFn: func(ctx context.Context, infoHashes []string) ([]*model.IYUUReseedResult, error) {
			return []*model.IYUUReseedResult{
				{
					SourceInfoHash: "other_hash",
					Targets: []model.IYUUTarget{
						{Sid: 1, TorrentID: 100, InfoHash: "ih_tgt"},
					},
				},
			}, nil
		},
	})

	rec := model.SeedingTorrentRecord{InfoHash: "ih1", SiteName: "src"}
	iyuuResults := map[string][]*model.IYUUReseedResult{
		"other_hash": {{
			SourceInfoHash: "other_hash",
			Targets: []model.IYUUTarget{
				{Sid: 1, TorrentID: 100, InfoHash: "ih_tgt"},
			},
		}},
	}
	candidates := e.filterIYUUResults(sourceTorrent{InfoHash: rec.InfoHash, SiteName: rec.SiteName}, iyuuResults, map[int]string{}, nil, nil)
	if len(candidates) != 0 {
		t.Errorf("expected 0 for mismatched source hash, got %d", len(candidates))
	}
}

func TestNormalizeTitle_SpecialCases(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{"【字幕组】影片名 1080p", "影片名"},
		{"Title（备注）Something", "titlesomething"},
		{"Movie 2160p BluRay Extra", "movie"},
		{"Movie x264 Extra", "movie"},
		{"Movie hevc Extra", "movie"},
		{"Movie web-dl Extra", "movie"},
		{"Movie remux Extra", "movie"},
		{"Movie bdrip Extra", "movie"},
		{"Movie hdrip Extra", "movie"},
		{"Movie webrip Extra", "movie"},
		{"Movie h264 Extra", "movie"},
		{"Movie h265 Extra", "movie"},
		{"Movie bluray Extra", "movie"},
		{"AB x264", "ab x264"},
		{"  \t  ", ""},
	}

	for _, tt := range tests {
		got := NormalizeTitle(tt.input)
		if got != tt.expect {
			t.Errorf("NormalizeTitle(%q) = %q, want %q", tt.input, got, tt.expect)
		}
	}
}

// §33.32 — Test helper: wraps mocks.SiteAdapter + piecesHashSearcher
type testPiecesHashAdapter struct {
	*mocks.SiteAdapter
	searchByPiecesHashFn func(ctx context.Context, config *model.SiteConfig, piecesHashes []string) (map[string]int, error)
}

func (a *testPiecesHashAdapter) SearchByPiecesHash(ctx context.Context, config *model.SiteConfig, piecesHashes []string) (map[string]int, error) {
	if a.searchByPiecesHashFn != nil {
		return a.searchByPiecesHashFn(ctx, config, piecesHashes)
	}
	return nil, nil
}

func TestEngine_matchLayer0PiecesHash_Found(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	adapter := &testPiecesHashAdapter{
		SiteAdapter: &mocks.SiteAdapter{SupportsSearchByPiecesHashVal: true},
		searchByPiecesHashFn: func(_ context.Context, _ *model.SiteConfig, hashes []string) (map[string]int, error) {
			return map[string]int{"ph_abc": 12345}, nil
		},
	}

	fc := &fpCache{byKey: map[string]*model.ContentFingerprint{
		"ih1|site1": {PiecesHash: "ph_abc", InfoHash: "ih1", SiteName: "site1"},
	}}

	rec := model.SeedingTorrentRecord{InfoHash: "ih1", SiteName: "site1"}
	config := &model.SiteConfig{Passkey: "31ab1c9e2bf5533b4d23e94b2cad5cd9", SupportsPiecesHashAPI: true}

	c := e.matchLayer0PiecesHash(context.Background(), adapter, config, rec.InfoHash, rec.SiteName, "target_site", fc)
	if c == nil {
		t.Fatal("expected candidate, got nil")
	}
	if c.MatchMethod != "pieces_hash" {
		t.Errorf("expected pieces_hash, got %s", c.MatchMethod)
	}
	if c.TargetTorrentID != "12345" {
		t.Errorf("expected 12345, got %s", c.TargetTorrentID)
	}
	if c.Confidence != 1.0 {
		t.Errorf("expected confidence 1.0, got %f", c.Confidence)
	}
	if c.TargetSite != "target_site" {
		t.Errorf("expected target_site, got %s", c.TargetSite)
	}
}

func TestEngine_matchLayer0PiecesHash_NoFingerprint(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	adapter := &testPiecesHashAdapter{
		SiteAdapter: &mocks.SiteAdapter{SupportsSearchByPiecesHashVal: true},
	}

	fc := &fpCache{byKey: map[string]*model.ContentFingerprint{}}
	rec := model.SeedingTorrentRecord{InfoHash: "ih1", SiteName: "site1"}
	config := &model.SiteConfig{Passkey: "pk"}

	c := e.matchLayer0PiecesHash(context.Background(), adapter, config, rec.InfoHash, rec.SiteName, "site2", fc)
	if c != nil {
		t.Error("expected nil when no fingerprint")
	}
}

func TestEngine_matchLayer0PiecesHash_EmptyPiecesHash(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	adapter := &testPiecesHashAdapter{
		SiteAdapter: &mocks.SiteAdapter{SupportsSearchByPiecesHashVal: true},
	}

	fc := &fpCache{byKey: map[string]*model.ContentFingerprint{
		"ih1|site1": {PiecesHash: "", InfoHash: "ih1", SiteName: "site1"},
	}}

	rec := model.SeedingTorrentRecord{InfoHash: "ih1", SiteName: "site1"}
	config := &model.SiteConfig{Passkey: "pk"}

	c := e.matchLayer0PiecesHash(context.Background(), adapter, config, rec.InfoHash, rec.SiteName, "site2", fc)
	if c != nil {
		t.Error("expected nil when empty pieces_hash")
	}
}

func TestEngine_matchLayer0PiecesHash_NotSupported(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	adapter := &testPiecesHashAdapter{
		SiteAdapter: &mocks.SiteAdapter{SupportsSearchByPiecesHashVal: false},
	}

	fc := &fpCache{byKey: map[string]*model.ContentFingerprint{
		"ih1|site1": {PiecesHash: "ph_abc"},
	}}

	rec := model.SeedingTorrentRecord{InfoHash: "ih1", SiteName: "site1"}
	config := &model.SiteConfig{Passkey: "pk"}

	c := e.matchLayer0PiecesHash(context.Background(), adapter, config, rec.InfoHash, rec.SiteName, "site2", fc)
	if c != nil {
		t.Error("expected nil when adapter does not support pieces_hash")
	}
}

func TestEngine_matchLayer0PiecesHash_SiteConfigDisabled(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	called := false
	adapter := &testPiecesHashAdapter{
		SiteAdapter: &mocks.SiteAdapter{SupportsSearchByPiecesHashVal: true},
		searchByPiecesHashFn: func(_ context.Context, _ *model.SiteConfig, _ []string) (map[string]int, error) {
			called = true
			return map[string]int{"ph_abc": 1}, nil
		},
	}

	fc := &fpCache{byKey: map[string]*model.ContentFingerprint{
		"ih1|site1": {PiecesHash: "ph_abc"},
	}}

	rec := model.SeedingTorrentRecord{InfoHash: "ih1", SiteName: "site1"}
	config := &model.SiteConfig{Passkey: "pk", SupportsPiecesHashAPI: false}

	c := e.matchLayer0PiecesHash(context.Background(), adapter, config, rec.InfoHash, rec.SiteName, "site2", fc)
	if c != nil {
		t.Error("expected nil when site config disables pieces_hash API")
	}
	if called {
		t.Error("SearchByPiecesHash should not be called when SupportsPiecesHashAPI is false")
	}
}

func TestEngine_matchLayer0PiecesHash_NoCreds(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	adapter := &testPiecesHashAdapter{
		SiteAdapter: &mocks.SiteAdapter{SupportsSearchByPiecesHashVal: true},
	}

	fc := &fpCache{byKey: map[string]*model.ContentFingerprint{
		"ih1|site1": {PiecesHash: "ph_abc"},
	}}

	rec := model.SeedingTorrentRecord{InfoHash: "ih1", SiteName: "site1"}
	config := &model.SiteConfig{Passkey: "", Cookie: ""}

	c := e.matchLayer0PiecesHash(context.Background(), adapter, config, rec.InfoHash, rec.SiteName, "site2", fc)
	if c != nil {
		t.Error("expected nil when no passkey and no cookie")
	}
}

func TestEngine_matchLayer0PiecesHash_CookieOnly(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	adapter := &testPiecesHashAdapter{
		SiteAdapter: &mocks.SiteAdapter{SupportsSearchByPiecesHashVal: true},
		searchByPiecesHashFn: func(_ context.Context, _ *model.SiteConfig, _ []string) (map[string]int, error) {
			return map[string]int{"ph_abc": 88888}, nil
		},
	}

	fc := &fpCache{byKey: map[string]*model.ContentFingerprint{
		"ih1|site1": {PiecesHash: "ph_abc"},
	}}

	rec := model.SeedingTorrentRecord{InfoHash: "ih1", SiteName: "site1"}
	config := &model.SiteConfig{Passkey: "", Cookie: "session=abc123", SupportsPiecesHashAPI: true}

	c := e.matchLayer0PiecesHash(context.Background(), adapter, config, rec.InfoHash, rec.SiteName, "site2", fc)
	if c == nil {
		t.Fatal("expected candidate with cookie-only auth")
	}
	if c.MatchMethod != "pieces_hash" {
		t.Errorf("expected pieces_hash, got %s", c.MatchMethod)
	}
	if c.TargetTorrentID != "88888" {
		t.Errorf("expected 88888, got %s", c.TargetTorrentID)
	}
}

func TestEngine_matchLayer0PiecesHash_APIError(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	adapter := &testPiecesHashAdapter{
		SiteAdapter: &mocks.SiteAdapter{SupportsSearchByPiecesHashVal: true},
		searchByPiecesHashFn: func(_ context.Context, _ *model.SiteConfig, _ []string) (map[string]int, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}

	fc := &fpCache{byKey: map[string]*model.ContentFingerprint{
		"ih1|site1": {PiecesHash: "ph_abc"},
	}}

	rec := model.SeedingTorrentRecord{InfoHash: "ih1", SiteName: "site1"}
	config := &model.SiteConfig{Passkey: "pk"}

	c := e.matchLayer0PiecesHash(context.Background(), adapter, config, rec.InfoHash, rec.SiteName, "site2", fc)
	if c != nil {
		t.Error("expected nil on API error")
	}
}

func TestEngine_matchLayer0PiecesHash_NoMatch(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	adapter := &testPiecesHashAdapter{
		SiteAdapter: &mocks.SiteAdapter{SupportsSearchByPiecesHashVal: true},
		searchByPiecesHashFn: func(_ context.Context, _ *model.SiteConfig, _ []string) (map[string]int, error) {
			return map[string]int{}, nil
		},
	}

	fc := &fpCache{byKey: map[string]*model.ContentFingerprint{
		"ih1|site1": {PiecesHash: "ph_abc"},
	}}

	rec := model.SeedingTorrentRecord{InfoHash: "ih1", SiteName: "site1"}
	config := &model.SiteConfig{Passkey: "pk"}

	c := e.matchLayer0PiecesHash(context.Background(), adapter, config, rec.InfoHash, rec.SiteName, "site2", fc)
	if c != nil {
		t.Error("expected nil when no match")
	}
}

func TestFindMainVideoFile(t *testing.T) {
	tests := []struct {
		name     string
		fileTree map[string]int64
		want     string
	}{
		{
			name:     "single mkv",
			fileTree: map[string]int64{"movie.mkv": 1000000},
			want:     "movie.mkv",
		},
		{
			name: "largest video wins",
			fileTree: map[string]int64{
				"small.mkv":    500000,
				"big.mkv":      5000000,
				"cover.jpg":    100000,
				"info.txt":     1000,
			},
			want: "big.mkv",
		},
		{
			name: "no video files",
			fileTree: map[string]int64{
				"cover.jpg": 100000,
				"info.txt":  1000,
			},
			want: "",
		},
		{
			name:     "empty tree",
			fileTree: map[string]int64{},
			want:     "",
		},
		{
			name: "subdirectory path",
			fileTree: map[string]int64{
				"Subs/sub1.srt":                 50000,
				"Movie.2020.mkv":                5000000,
				"Movie.2020_s.jpg":              100000,
			},
			want: "Movie.2020.mkv",
		},
		{
			name: "mp4 and mkv, mkv larger",
			fileTree: map[string]int64{
				"a.mp4": 3000000,
				"b.mkv": 4000000,
			},
			want: "b.mkv",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findMainVideoFile(tt.fileTree)
			if got != tt.want {
				t.Errorf("findMainVideoFile() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractFromFileTree(t *testing.T) {
	fileTree := map[string]int64{
		"[斯巴达克斯].Spartacus.1960.4K.Restored.Edition.BluRay.1080p.x264.DTS.2Audios-CMCT.mkv": 23628070195,
		"[斯巴达克斯].Spartacus.1960.4K.Restored.Edition.BluRay.1080p.x264.DTS.2Audios-CMCT_s.jpg": 800000,
		"斯巴达克斯 1960 蓝光封面.jpg": 500000,
		"斯巴达克斯 1960 内容简介.txt":  1000,
	}

	keyword, groupName := extractFromFileTree(fileTree)

	if keyword == "" {
		t.Fatal("expected non-empty keyword")
	}
	if !strings.Contains(keyword, "Spartacus") {
		t.Errorf("keyword should contain 'Spartacus', got %q", keyword)
	}
	if groupName != "CMCT" {
		t.Errorf("groupName should be CMCT, got %q", groupName)
	}
}

func TestExtractFromFileTree_NoVideo(t *testing.T) {
	fileTree := map[string]int64{
		"cover.jpg": 100000,
		"info.txt":  1000,
	}
	keyword, groupName := extractFromFileTree(fileTree)
	if keyword != "" {
		t.Errorf("expected empty keyword, got %q", keyword)
	}
	if groupName != "" {
		t.Errorf("expected empty groupName, got %q", groupName)
	}
}

func TestKeywordStartsWithYear(t *testing.T) {
	tests := []struct {
		keyword string
		want    bool
	}{
		{"2023 1080p", true},
		{"1960 4K", true},
		{"1934 FRA 1080p", true},
		{"2013 Extended Cut 1080p", true},
		{"The Matrix 1999 1080p", false},
		{"Spartacus 1960 4K", false},
		{"1080p", false},
		{"4K", false},
		{"", false},
		{"ab", false},
	}
	for _, tt := range tests {
		t.Run(tt.keyword, func(t *testing.T) {
			got := keywordStartsWithYear(tt.keyword)
			if got != tt.want {
				t.Errorf("keywordStartsWithYear(%q) = %v, want %v", tt.keyword, got, tt.want)
			}
		})
	}
}
