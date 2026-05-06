package reseed

import (
	"context"
	"fmt"
	"testing"
	"time"

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
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestEngine_CreateAndGetTask(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	task := &model.ReseedTask{Name: "test-task", Enabled: true, ClientIDs: "c1,c2"}
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

	db.Create(&model.SeedingTorrentRecord{
		ClientID: "c1", InfoHash: "ih1", SiteName: "site1",
		TorrentID: "t1", Status: model.SeedingStatusSeeding,
	})

	result, err := e.RunTask(context.Background(), task)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.TotalSources != 1 {
		t.Errorf("expected 1 source, got %d", result.TotalSources)
	}
}

func TestEngine_RunEnabledTasks(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	if err := e.CreateTask(context.Background(), &model.ReseedTask{Name: "enabled1", Enabled: true, ClientIDs: "c1"}); err != nil {
		t.Fatal(err)
	}
	if err := e.CreateTask(context.Background(), &model.ReseedTask{Name: "disabled", Enabled: false, ClientIDs: "c1"}); err != nil {
		t.Fatal(err)
	}
	if err := e.CreateTask(context.Background(), &model.ReseedTask{Name: "enabled2", Enabled: true, ClientIDs: "c2"}); err != nil {
		t.Fatal(err)
	}

	if err := e.RunEnabledTasks(context.Background()); err != nil {
		t.Fatalf("run enabled: %v", err)
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
	if appErr.Code != 40001 {
		t.Errorf("expected code 40001, got %d", appErr.Code)
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
		TargetSite: "s2", TargetTorrentID: "t2", MatchMethod: "infohash_exact",
		Confidence: 1.0, Status: model.MatchStatusMatched,
	})

	m, err := e.FindMatchByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if m.MatchMethod != "infohash_exact" {
		t.Errorf("expected infohash_exact, got %s", m.MatchMethod)
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
	e := NewEngine(db, zap.NewNop())

	task := &model.ReseedTask{
		Name: "filtered", Enabled: true, ClientIDs: "c1",
		SourceSiteIDs: "site1",
	}
	if err := e.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create: %v", err)
	}

	db.Create(&model.SeedingTorrentRecord{
		ClientID: "c1", InfoHash: "ih1", SiteName: "site1",
		TorrentID: "t1", Status: model.SeedingStatusSeeding,
	})
	db.Create(&model.SeedingTorrentRecord{
		ClientID: "c1", InfoHash: "ih2", SiteName: "site2",
		TorrentID: "t2", Status: model.SeedingStatusSeeding,
	})

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

func TestEngine_RunEnabledTasks2(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	if err := e.CreateTask(context.Background(), &model.ReseedTask{Name: "task1", Enabled: true, ClientIDs: "c1"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := e.CreateTask(context.Background(), &model.ReseedTask{Name: "task2", Enabled: true, ClientIDs: ""}); err != nil {
		t.Fatalf("create: %v", err)
	}

	err := e.RunEnabledTasks(context.Background())
	if err != nil {
		t.Fatalf("RunEnabledTasks: %v", err)
	}
}

func TestEngine_SaveAndFindMatch(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	match := &model.ReseedMatch{
		ClientID: "c1", SourceSite: "s1", SourceTorrentID: "t1", SourceInfoHash: "ih1",
		TargetSite: "s2", TargetTorrentID: "t2", MatchMethod: "infohash_exact",
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
	e := NewEngine(db, zap.NewNop())

	task := &model.ReseedTask{Name: "cancel-test", Enabled: true, ClientIDs: "c1"}
	if err := e.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create: %v", err)
	}

	db.Create(&model.SeedingTorrentRecord{
		ClientID: "c1", InfoHash: "ih1", SiteName: "site1",
		TorrentID: "t1", Status: model.SeedingStatusSeeding,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := e.RunTask(ctx, task)
	if err == nil {
		t.Error("expected error with canceled context")
	}
}

func TestEngine_CancelTask_Noop(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())
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

	err := e.SetNegativeCache(context.Background(), "ih1", "site1", "size mismatch", "pieces_hash", 3, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	var entry model.ReseedNegativeCache
	db.Where("source_info_hash = ?", "ih1").First(&entry)
	if entry.SourceSite != "site1" {
		t.Errorf("expected site1, got %s", entry.SourceSite)
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

	if err := e.SetNegativeCache(context.Background(), "ih1", "site1", "reason1", "method1", 3, 24*time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := e.SetNegativeCache(context.Background(), "ih2", "site2", "reason2", "method2", 3, 24*time.Hour); err != nil {
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
	e.SetClientProvider(nil)
}

func TestEngine_SetFingerprintRepo(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())
	e.SetFingerprintRepo(nil)
}

func TestEngine_SetIYUUService(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())
	e.SetIYUUService(nil)
}

func TestEngine_RunTask_DuplicateExists(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())
	task := &model.ReseedTask{Name: "dup-task", Enabled: true, ClientIDs: "c1"}
	if err := e.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create: %v", err)
	}

	db.Create(&model.SeedingTorrentRecord{
		ClientID: "c1", InfoHash: "ih_dup", SiteName: "site1",
		TorrentID: "t1", Status: model.SeedingStatusSeeding,
	})
	db.Create(&model.ReseedMatch{
		ClientID: "c1", SourceSite: "site1", SourceTorrentID: "t1", SourceInfoHash: "ih_dup",
		TargetSite: "site2", TargetTorrentID: "t2", MatchMethod: "infohash_exact",
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
	candidates := e.findCandidates(context.Background(), rec, nil, nil, 1.0, task)
	if candidates != nil {
		t.Error("expected nil for nil provider")
	}
}

func TestEngine_findCandidates_WithProvider(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	adapter := &mocks.SiteAdapter{
		SearchTorrentsFn: func(ctx context.Context, config *model.SiteConfig, query string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
			return []*model.SeedingSearchResult{
				{TorrentID: "target_t1", Size: 1073741824},
			}, nil
		},
		GetTorrentInfoHashFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) (string, error) {
			return "ih1", nil
		},
	}

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

	task := &model.ReseedTask{Name: "fc-prov", Enabled: true}
	rec := model.SeedingTorrentRecord{InfoHash: "ih1", SiteName: "source_site"}
	candidates := e.findCandidates(context.Background(), rec, nil, nil, 1.0, task)
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
	if candidates[0].MatchMethod != "infohash_exact" {
		t.Errorf("expected infohash_exact, got %s", candidates[0].MatchMethod)
	}
	if candidates[0].TargetSite != "target_site" {
		t.Errorf("expected target_site, got %s", candidates[0].TargetSite)
	}
}

func TestEngine_matchLayer1InfoHash_Found(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	adapter := &mocks.SiteAdapter{
		SearchTorrentsFn: func(ctx context.Context, config *model.SiteConfig, query string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
			return []*model.SeedingSearchResult{
				{TorrentID: "t1", Size: 1000},
			}, nil
		},
		GetTorrentInfoHashFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) (string, error) {
			return "abc123", nil
		},
	}

	rec := model.SeedingTorrentRecord{InfoHash: "abc123", SiteName: "site1"}
	c := e.matchLayer1InfoHash(context.Background(), adapter, &model.SiteConfig{}, rec, "site2")
	if c == nil {
		t.Fatal("expected candidate, got nil")
	}
	if c.MatchMethod != "infohash_exact" {
		t.Errorf("expected infohash_exact, got %s", c.MatchMethod)
	}
	if c.Confidence != 1.0 {
		t.Errorf("expected confidence 1.0, got %f", c.Confidence)
	}
}

func TestEngine_matchLayer1InfoHash_EmptyHash(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())
	adapter := &mocks.SiteAdapter{}
	rec := model.SeedingTorrentRecord{InfoHash: "", SiteName: "site1"}
	c := e.matchLayer1InfoHash(context.Background(), adapter, &model.SiteConfig{}, rec, "site2")
	if c != nil {
		t.Error("expected nil for empty info hash")
	}
}

func TestEngine_matchLayer1InfoHash_SearchError(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())
	adapter := &mocks.SiteAdapter{
		SearchTorrentsFn: func(ctx context.Context, config *model.SiteConfig, query string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
			return nil, fmt.Errorf("search error")
		},
	}
	rec := model.SeedingTorrentRecord{InfoHash: "ih1", SiteName: "site1"}
	c := e.matchLayer1InfoHash(context.Background(), adapter, &model.SiteConfig{}, rec, "site2")
	if c != nil {
		t.Error("expected nil on search error")
	}
}

func TestEngine_matchLayer2SizeTitle_WithDB(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	db.Create(&model.ContentFingerprint{
		InfoHash:  "ih_source",
		SiteName:  "source_site",
		Title:     "Some Movie 2023",
		TotalSize: 1073741824,
	})

	adapter := &mocks.SiteAdapter{
		SearchTorrentsFn: func(ctx context.Context, config *model.SiteConfig, query string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
			return []*model.SeedingSearchResult{
				{TorrentID: "t_target", Size: 1073741824},
			}, nil
		},
	}

	rec := model.SeedingTorrentRecord{InfoHash: "ih_source", SiteName: "source_site"}
	c := e.matchLayer2SizeTitle(context.Background(), adapter, &model.SiteConfig{}, rec, "target_site", 1.0)
	if c == nil {
		t.Fatal("expected candidate, got nil")
	}
	if c.MatchMethod != "size_title" {
		t.Errorf("expected size_title, got %s", c.MatchMethod)
	}
	if c.Confidence != 0.85 {
		t.Errorf("expected confidence 0.85, got %f", c.Confidence)
	}
}

func TestEngine_matchLayer2SizeTitle_NoFingerprint(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())
	adapter := &mocks.SiteAdapter{}
	rec := model.SeedingTorrentRecord{InfoHash: "nonexist", SiteName: "site1"}
	c := e.matchLayer2SizeTitle(context.Background(), adapter, &model.SiteConfig{}, rec, "site2", 1.0)
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
		Title:     "Another Movie",
		TotalSize: 1073741824,
	})

	adapter := &mocks.SiteAdapter{
		SearchTorrentsFn: func(ctx context.Context, config *model.SiteConfig, query string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
			return []*model.SeedingSearchResult{
				{TorrentID: "t1", Size: 2147483648},
			}, nil
		},
	}

	rec := model.SeedingTorrentRecord{InfoHash: "ih_src2", SiteName: "source_site"}
	c := e.matchLayer2SizeTitle(context.Background(), adapter, &model.SiteConfig{}, rec, "target_site", 1.0)
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
	c := e.matchLayer3Fingerprint(context.Background(), rec, "target_site")
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
	c := e.matchLayer3Fingerprint(context.Background(), rec, "target_site")
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
	c := e.matchLayer3Fingerprint(context.Background(), rec, "target_site")
	if c != nil {
		t.Error("expected nil when no pieces_hash and no files_hash")
	}
}

func TestEngine_injectMatch_NoProvider(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())
	match := &model.ReseedMatch{ClientID: "c1", SourceSite: "s1", TargetSite: "s2"}
	task := &model.ReseedTask{}
	err := e.injectMatch(context.Background(), match, task)
	if err == nil {
		t.Error("expected error when no providers set")
	}
}

func TestEngine_injectMatch_FailSiteInfo(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())
	e.SetSiteProvider(&mocks.SiteInfoProvider{
		GetSiteInfoFn: func(ctx context.Context, siteName string) (*model.SiteInfo, error) {
			return nil, fmt.Errorf("site not found")
		},
	})
	e.SetClientProvider(&mocks.DownloaderProvider{
		GetFn: func(clientID string) (model.DownloaderClient, error) {
			return &mocks.DownloaderClient{}, nil
		},
	})

	match := &model.ReseedMatch{
		ClientID: "c1", SourceSite: "s1", SourceTorrentID: "t1", SourceInfoHash: "ih1",
		TargetSite: "s2", TargetTorrentID: "t2", MatchMethod: "infohash_exact",
		Confidence: 1.0, Status: model.MatchStatusMatched,
	}
	db.Create(match)
	task := &model.ReseedTask{Name: "inj-fail", Enabled: true}
	if err := e.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create: %v", err)
	}

	err := e.injectMatch(context.Background(), match, task)
	if err == nil {
		t.Error("expected error when GetSiteInfo fails")
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
		TargetSite: "target_site", TargetTorrentID: "t2", MatchMethod: "infohash_exact",
		Confidence: 1.0, Status: model.MatchStatusMatched,
	}
	db.Create(match)
	task := &model.ReseedTask{Name: "inj-ok", Enabled: true, ReseedCategory: "cross-seed"}
	if err := e.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create: %v", err)
	}

	err := e.injectMatch(context.Background(), match, task)
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
		TargetSite: "target_site", TargetTorrentID: "t2", MatchMethod: "infohash_exact",
		Confidence: 1.0, Status: model.MatchStatusMatched,
	}
	db.Create(match)
	task := &model.ReseedTask{Name: "inj-exists", Enabled: true}
	if err := e.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create: %v", err)
	}

	err := e.injectMatch(context.Background(), match, task)
	if err == nil {
		t.Error("expected error for already exists")
	}
	var updated model.ReseedMatch
	db.First(&updated, match.ID)
	if updated.Status != model.MatchStatusFailed {
		t.Errorf("expected failed, got %s", updated.Status)
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
	candidates := e.queryIYUU(context.Background(), rec, []string{"target_site"}, nil)
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
	e.SetIYUUService(&mockIYUUService{
		queryReseedFn: func(ctx context.Context, infoHashes []string) ([]*model.IYUUReseedResult, error) {
			return nil, fmt.Errorf("iyuu error")
		},
	})
	rec := model.SeedingTorrentRecord{InfoHash: "ih1", SiteName: "s1"}
	candidates := e.queryIYUU(context.Background(), rec, nil, nil)
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
	candidates := e.queryIYUU(context.Background(), rec, nil, []string{"excluded_site"})
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
	candidates := e.queryIYUU(context.Background(), rec, nil, nil)
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
	candidates := e.findCandidates(context.Background(), rec, nil, []string{"excluded"}, 1.0, task)
	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates (excluded+disabled+same site), got %d", len(candidates))
	}
}

func TestEngine_findCandidates_WithTargetSites(t *testing.T) {
	db := setupReseedDB(t)
	e := NewEngine(db, zap.NewNop())

	adapter := &mocks.SiteAdapter{
		SearchTorrentsFn: func(ctx context.Context, config *model.SiteConfig, query string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
			return []*model.SeedingSearchResult{{TorrentID: "t1", Size: 1000}}, nil
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

	task := &model.ReseedTask{Name: "fc-targets", Enabled: true}
	rec := model.SeedingTorrentRecord{InfoHash: "ih_match", SiteName: "source_site"}
	candidates := e.findCandidates(context.Background(), rec, []string{"site_a"}, nil, 1.0, task)
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
	if candidates[0].TargetSite != "site_a" {
		t.Errorf("expected site_a, got %s", candidates[0].TargetSite)
	}
}
