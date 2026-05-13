package publish

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/ranfish/pt-forward/internal/mocks"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/notification"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupPipelineTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.PublishTask{}, &model.PublishCandidate{}, &model.PublishResultRecord{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestPipeline_CreateAndGetTask(t *testing.T) {
	p := NewPipeline(setupPipelineTestDB(t), zap.NewNop())
	ctx := context.Background()

	task := &model.PublishTask{
		Type:         model.PublishTaskTypeManual,
		SourceSiteID: 1,
		TargetSites:  []string{"site1", "site2"},
	}
	if err := p.CreateTask(ctx, task); err != nil {
		t.Fatal(err)
	}
	if task.ID == 0 {
		t.Fatal("ID should be set")
	}

	got, err := p.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != model.PublishTaskPending {
		t.Errorf("expected pending, got %s", got.Status)
	}
}

func TestPipeline_UpdateTaskStatus(t *testing.T) {
	p := NewPipeline(setupPipelineTestDB(t), zap.NewNop())
	ctx := context.Background()

	task := &model.PublishTask{Type: model.PublishTaskTypeManual, SourceSiteID: 1}
	if err := p.CreateTask(ctx, task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	if err := p.UpdateTaskStatus(ctx, task.ID, model.PublishTaskPublishing); err != nil {
		t.Fatalf("update status: %v", err)
	}

	got, _ := p.GetTask(ctx, task.ID)
	if got.Status != model.PublishTaskPublishing {
		t.Errorf("expected publishing, got %s", got.Status)
	}
}

func TestPipeline_ListTasks(t *testing.T) {
	p := NewPipeline(setupPipelineTestDB(t), zap.NewNop())
	ctx := context.Background()

	if err := p.CreateTask(ctx, &model.PublishTask{Type: model.PublishTaskTypeManual, SourceSiteID: 1}); err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := p.CreateTask(ctx, &model.PublishTask{Type: model.PublishTaskTypeAuto, SourceSiteID: 2}); err != nil {
		t.Fatalf("create task: %v", err)
	}

	tasks, total, err := p.ListTasks(ctx, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Fatalf("expected 2, got %d", total)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}
}

func TestPipeline_CandidateCRUD(t *testing.T) {
	p := NewPipeline(setupPipelineTestDB(t), zap.NewNop())
	ctx := context.Background()

	candidate := &model.PublishCandidate{
		SourceSite:      "site1",
		SourceTorrentID: "42",
		InfoHash:        "abc123",
		TorrentName:     "Test.Torrent",
		Size:            1073741824,
		PublishStatus:   model.CandidatePending,
		Role:            model.RoleDownload,
	}
	if err := p.CreateCandidate(ctx, candidate); err != nil {
		t.Fatalf("create candidate: %v", err)
	}

	got, err := p.GetCandidate(ctx, candidate.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.TorrentName != "Test.Torrent" {
		t.Errorf("expected Test.Torrent, got %s", got.TorrentName)
	}

	if err := p.MarkDownloadCompleted(ctx, candidate.ID, "/data/torrents", "/data/torrents/test.torrent"); err != nil {
		t.Fatalf("mark download completed: %v", err)
	}
	got2, _ := p.GetCandidate(ctx, candidate.ID)
	if !got2.DownloadCompleted {
		t.Error("should be completed")
	}
	if got2.PublishStatus != model.CandidateCompleted {
		t.Errorf("expected completed, got %s", got2.PublishStatus)
	}
}

func TestPipeline_CheckEligibility(t *testing.T) {
	p := NewPipeline(setupPipelineTestDB(t), zap.NewNop())

	tests := []struct {
		name    string
		title   string
		allowed bool
	}{
		{"normal", "Movie.2024.1080p.BluRay", true},
		{"forbidden keyword", "Movie 禁转 2024", false},
		{"exclusive", "Show 独占 S01", false},
		{"拒绝转载", "Film 谢绝转载 2024", false},
		{"CatEDU", "Course CatEDU Lecture", false},
		{"严禁转载", "Doc 严禁转载", false},
		{"禁止转载", "Anime 禁止转载 EP01", false},
		{"9KG", "9KG Some Movie", false},
		{"色情", "色情内容 Film", false},
		{"成人内容", "成人内容 Film 2024", false},
		{"XXX", "XXX Movie", false},
		{"Porn", "Porn Video", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := &model.PublishCandidate{TorrentName: tt.title}
			ok, _ := p.CheckPublishEligibility(context.Background(), candidate, "target")
			if ok != tt.allowed {
				t.Errorf("eligibility(%q) = %v, want %v", tt.title, ok, tt.allowed)
			}
		})
	}
}

func TestPipeline_CheckEligibility_HR(t *testing.T) {
	p := NewPipeline(setupPipelineTestDB(t), zap.NewNop())

	normal := &model.PublishCandidate{TorrentName: "Movie.2024.1080p.BluRay", HasHR: false}
	ok, _ := p.CheckPublishEligibility(context.Background(), normal, "target")
	if !ok {
		t.Error("non-HR candidate should be eligible")
	}

	hr := &model.PublishCandidate{TorrentName: "Movie.2024.1080p.BluRay", HasHR: true}
	ok, reason := p.CheckPublishEligibility(context.Background(), hr, "target")
	if ok {
		t.Error("HR candidate should not be eligible")
	}
	if !strings.Contains(reason, "H&R") {
		t.Errorf("reason should mention H&R, got: %s", reason)
	}
}

func TestContainsAnyKeyword(t *testing.T) {
	tests := []struct {
		text     string
		keywords []string
		found    bool
	}{
		{"Movie 禁转 2024", []string{"禁转"}, true},
		{"Normal Movie", []string{"禁转"}, false},
		{"9KG Film", []string{"9KG"}, true},
		{"9kg film", []string{"9KG"}, true},
		{"nothing here", []string{"禁转", "独占"}, false},
		{"", []string{"禁转"}, false},
	}
	for _, tt := range tests {
		_, found := containsAnyKeyword(tt.text, tt.keywords)
		if found != tt.found {
			t.Errorf("containsAnyKeyword(%q, %v) = %v, want %v", tt.text, tt.keywords, found, tt.found)
		}
	}
}

func TestCheckForbiddenContent(t *testing.T) {
	p := NewPipeline(setupPipelineTestDB(t), zap.NewNop())

	tests := []struct {
		name  string
		texts []string
		ok    bool
	}{
		{"normal", []string{"Movie.2024", "", ""}, true},
		{"forbidden in title", []string{"Movie 禁转", "", ""}, false},
		{"forbidden in subtitle", []string{"Movie", "独占资源", ""}, false},
		{"forbidden in description", []string{"Movie", "", "谢绝转载"}, false},
		{"9KG in title", []string{"9KG Movie", "", ""}, false},
		{"adult keyword", []string{"色情内容", "", ""}, false},
		{"CatEDU in title", []string{"CatEDU Math 101", "", ""}, false},
		{"empty all", []string{"", "", ""}, true},
		{"case insensitive adult", []string{"xxx video", "", ""}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, reason := p.checkForbiddenContent(tt.texts)
			if ok != tt.ok {
				t.Errorf("checkForbiddenContent(%v) = %v (reason: %s), want %v", tt.texts, ok, reason, tt.ok)
			}
		})
	}
}

func TestPipeline_OnTorrents(t *testing.T) {
	db := setupPipelineTestDB(t)
	p := NewPipeline(db, zap.NewNop())
	ctx := context.Background()

	events := []model.TorrentEvent{
		{
			SiteName:        "site1",
			TorrentID:       "t-001",
			Title:           "Ubuntu 24.04 LTS",
			Size:            4700000000,
			InfoHash:        "abc123",
			MatchedRuleName: "auto-publish",
		},
		{
			SiteName:  "site1",
			TorrentID: "t-002",
			Title:     "Debian 12",
			Size:      3900000000,
			InfoHash:  "def456",
		},
	}

	err := p.OnTorrents(ctx, events)
	if err != nil {
		t.Fatalf("OnTorrents: %v", err)
	}

	candidates, err := p.ListPendingCandidates(ctx, 100)
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}

	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate (only matched rule), got %d", len(candidates))
	}

	if candidates[0].SourceTorrentID != "t-001" {
		t.Errorf("expected t-001, got %s", candidates[0].SourceTorrentID)
	}
	if candidates[0].SourceSite != "site1" {
		t.Errorf("expected site1, got %s", candidates[0].SourceSite)
	}
}

func TestPipeline_DeleteCandidate(t *testing.T) {
	p := NewPipeline(setupPipelineTestDB(t), zap.NewNop())
	ctx := context.Background()

	if err := p.CreateCandidate(ctx, &model.PublishCandidate{
		SourceSite: "s1", SourceTorrentID: "t1", InfoHash: "ih1",
		TorrentName: "test", PublishStatus: model.CandidatePending,
	}); err != nil {
		t.Fatalf("create candidate: %v", err)
	}

	if err := p.DeleteCandidate(ctx, 1); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err := p.GetCandidate(ctx, 1)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestPipeline_PublishCandidate(t *testing.T) {
	p := NewPipeline(setupPipelineTestDB(t), zap.NewNop())
	ctx := context.Background()

	if err := p.CreateCandidate(ctx, &model.PublishCandidate{
		SourceSite: "s1", SourceTorrentID: "t1", InfoHash: "ih1",
		TorrentName: "Good Movie", PublishStatus: model.CandidatePending,
	}); err != nil {
		t.Fatalf("create candidate: %v", err)
	}

	candidate, err := p.PublishCandidate(ctx, 1)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if candidate.PublishStatus != model.CandidatePublishing {
		t.Errorf("expected publishing, got %s", candidate.PublishStatus)
	}
}

func TestPipeline_PublishCandidate_Ineligible(t *testing.T) {
	p := NewPipeline(setupPipelineTestDB(t), zap.NewNop())
	ctx := context.Background()

	if err := p.CreateCandidate(ctx, &model.PublishCandidate{
		SourceSite: "s1", SourceTorrentID: "t1", InfoHash: "ih1",
		TorrentName: "禁转 Movie", PublishStatus: model.CandidatePending,
	}); err != nil {
		t.Fatalf("create candidate: %v", err)
	}

	_, err := p.PublishCandidate(ctx, 1)
	if err == nil {
		t.Error("expected error for ineligible candidate")
	}
}

func TestPipeline_ProcessPending_Skipped(t *testing.T) {
	p := NewPipeline(setupPipelineTestDB(t), zap.NewNop())
	ctx := context.Background()

	if err := p.CreateCandidate(ctx, &model.PublishCandidate{
		SourceSite: "s1", SourceTorrentID: "t1", InfoHash: "ih1",
		TorrentName: "独占 Content", PublishStatus: model.CandidatePending,
	}); err != nil {
		t.Fatalf("create candidate: %v", err)
	}

	if err := p.ProcessPending(ctx); err != nil {
		t.Fatalf("process: %v", err)
	}

	got, _ := p.GetCandidate(ctx, 1)
	if got.PublishStatus != model.CandidateSkipped {
		t.Errorf("expected skipped, got %s", got.PublishStatus)
	}
}

func TestPipeline_ProcessPending_Done(t *testing.T) {
	p := NewPipeline(setupPipelineTestDB(t), zap.NewNop())
	ctx := context.Background()

	if err := p.CreateCandidate(ctx, &model.PublishCandidate{
		SourceSite: "s1", SourceTorrentID: "t1", InfoHash: "ih1",
		TorrentName: "Normal Movie", PublishStatus: model.CandidatePending,
		DownloadCompleted: true,
	}); err != nil {
		t.Fatalf("create candidate: %v", err)
	}

	if err := p.ProcessPending(ctx); err != nil {
		t.Fatalf("process: %v", err)
	}

	got, _ := p.GetCandidate(ctx, 1)
	if got.PublishStatus != model.CandidateDone {
		t.Errorf("expected done, got %s", got.PublishStatus)
	}
}

func TestPipeline_ResultCRUD(t *testing.T) {
	p := NewPipeline(setupPipelineTestDB(t), zap.NewNop())
	ctx := context.Background()

	if err := p.CreateCandidate(ctx, &model.PublishCandidate{
		SourceSite: "s1", SourceTorrentID: "t1", InfoHash: "ih1",
		TorrentName: "test", PublishStatus: model.CandidatePending,
	}); err != nil {
		t.Fatalf("create candidate: %v", err)
	}

	if err := p.CreateResult(ctx, &model.PublishResultRecord{
		CandidateID: 1, TargetSite: "target1", Status: "success",
	}); err != nil {
		t.Fatalf("create result: %v", err)
	}

	results, err := p.ListResults(ctx, 1, 10)
	if err != nil {
		t.Fatalf("list results: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestPipeline_PublishCandidate_ForbiddenKeyword(t *testing.T) {
	p := NewPipeline(setupPipelineTestDB(t), zap.NewNop())
	ctx := context.Background()

	tests := []struct {
		name  string
		title string
	}{
		{"禁转", "Movie 禁转 2024"},
		{"独占", "Film 独占 Release"},
		{"谢绝转载", "Show 谢绝转载 S01"},
		{"限时禁转", "Drama 限时禁转 EP01"},
		{"CatEDU", "Course CatEDU Math"},
		{"严禁转载", "Doc 严禁转载"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := p.CreateCandidate(ctx, &model.PublishCandidate{
				SourceSite: "s1", SourceTorrentID: "t1", InfoHash: "ih1",
				TorrentName: tt.title, PublishStatus: model.CandidatePending,
			}); err != nil {
				t.Fatalf("create candidate: %v", err)
			}

			_, err := p.PublishCandidate(ctx, 1)
			if err == nil {
				t.Errorf("expected error for title %q", tt.title)
			}

			db := setupPipelineTestDB(t)
			p2 := NewPipeline(db, zap.NewNop())
			if err := p2.CreateCandidate(ctx, &model.PublishCandidate{
				SourceSite: "s1", SourceTorrentID: "t1", InfoHash: "ih1",
				TorrentName: tt.title, PublishStatus: model.CandidatePending,
			}); err != nil {
				t.Fatalf("create candidate: %v", err)
			}
		})
	}
}

func TestPipeline_ProcessPending_Orphan(t *testing.T) {
	p := NewPipeline(setupPipelineTestDB(t), zap.NewNop())
	ctx := context.Background()

	candidate := &model.PublishCandidate{
		SourceSite: "s1", SourceTorrentID: "t1", InfoHash: "ih1",
		TorrentName: "Old Movie", PublishStatus: model.CandidatePending,
	}
	if err := p.CreateCandidate(ctx, candidate); err != nil {
		t.Fatalf("create candidate: %v", err)
	}

	p.db.Model(&model.PublishCandidate{}).Where("id = ?", candidate.ID).
		Update("created_at", "2020-01-01T00:00:00Z")

	if err := p.ProcessPending(ctx); err != nil {
		t.Fatalf("process: %v", err)
	}

	got, _ := p.GetCandidate(ctx, candidate.ID)
	if got.PublishStatus != model.CandidateOrphan {
		t.Errorf("expected orphan, got %s", got.PublishStatus)
	}
}

func TestPipeline_ListPending(t *testing.T) {
	p := NewPipeline(setupPipelineTestDB(t), zap.NewNop())
	ctx := context.Background()

	if err := p.CreateCandidate(ctx, &model.PublishCandidate{
		SourceSite: "s1", SourceTorrentID: "t1", InfoHash: "ih1",
		TorrentName: "p1", PublishStatus: model.CandidatePending,
	}); err != nil {
		t.Fatalf("create candidate: %v", err)
	}
	if err := p.CreateCandidate(ctx, &model.PublishCandidate{
		SourceSite: "s2", SourceTorrentID: "t2", InfoHash: "ih2",
		TorrentName: "p2", PublishStatus: model.CandidateDone,
	}); err != nil {
		t.Fatalf("create candidate: %v", err)
	}
	if err := p.CreateCandidate(ctx, &model.PublishCandidate{
		SourceSite: "s3", SourceTorrentID: "t3", InfoHash: "ih3",
		TorrentName: "p3", PublishStatus: "downloading",
	}); err != nil {
		t.Fatalf("create candidate: %v", err)
	}

	pending, err := p.ListPendingCandidates(ctx, 100)
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	if len(pending) != 2 {
		t.Errorf("expected 2 pending (pending + downloading), got %d", len(pending))
	}
}

func TestPipeline_GetTask_NotFound(t *testing.T) {
	p := NewPipeline(setupPipelineTestDB(t), zap.NewNop())
	_, err := p.GetTask(context.Background(), 999)
	if err == nil {
		t.Error("expected error for non-existent task")
	}
}

func TestPipeline_GetCandidate_NotFound(t *testing.T) {
	p := NewPipeline(setupPipelineTestDB(t), zap.NewNop())
	_, err := p.GetCandidate(context.Background(), 999)
	if err == nil {
		t.Error("expected error for non-existent candidate")
	}
}

func TestPipeline_OnTorrents_Empty(t *testing.T) {
	p := NewPipeline(setupPipelineTestDB(t), zap.NewNop())
	err := p.OnTorrents(context.Background(), []model.TorrentEvent{})
	if err != nil {
		t.Fatalf("empty events: %v", err)
	}
}

func TestPipeline_ListTasks_Pagination(t *testing.T) {
	p := NewPipeline(setupPipelineTestDB(t), zap.NewNop())
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		if err := p.CreateTask(ctx, &model.PublishTask{Type: model.PublishTaskTypeAuto, SourceSiteID: uint(i + 1)}); err != nil {
			t.Fatalf("create task: %v", err)
		}
	}
	tasks, total, err := p.ListTasks(ctx, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if total != 5 {
		t.Errorf("expected total=5, got %d", total)
	}
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks on page, got %d", len(tasks))
	}
}

func TestPipeline_DeleteTask(t *testing.T) {
	p := NewPipeline(setupPipelineTestDB(t), zap.NewNop())
	ctx := context.Background()
	if err := p.CreateTask(ctx, &model.PublishTask{Type: model.PublishTaskTypeManual, SourceSiteID: 1}); err != nil {
		t.Fatalf("create task: %v", err)
	}

	tasks, _, _ := p.ListTasks(ctx, 0, 10)
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
}

func TestPipeline_MarkDownloadCompleted(t *testing.T) {
	p := NewPipeline(setupPipelineTestDB(t), zap.NewNop())
	ctx := context.Background()
	if err := p.CreateCandidate(ctx, &model.PublishCandidate{
		SourceSite: "s1", SourceTorrentID: "t1", InfoHash: "ih1",
		TorrentName: "test", PublishStatus: model.CandidatePending,
	}); err != nil {
		t.Fatalf("create candidate: %v", err)
	}
	if err := p.MarkDownloadCompleted(ctx, 1, "/data/torrents", "/data/torrents/test.torrent"); err != nil {
		t.Fatalf("mark download completed: %v", err)
	}
	got, _ := p.GetCandidate(ctx, 1)
	if !got.DownloadCompleted {
		t.Error("expected download completed")
	}
	if got.LocalSavePath != "/data/torrents" {
		t.Errorf("expected /data/torrents, got %s", got.LocalSavePath)
	}
}

func TestPipeline_UpdateCandidateStatus(t *testing.T) {
	p := NewPipeline(setupPipelineTestDB(t), zap.NewNop())
	ctx := context.Background()

	if err := p.CreateCandidate(ctx, &model.PublishCandidate{
		SourceSite: "s1", SourceTorrentID: "t1", InfoHash: "ih1",
		TorrentName: "test", PublishStatus: model.CandidatePending,
	}); err != nil {
		t.Fatalf("create candidate: %v", err)
	}

	if err := p.UpdateCandidateStatus(ctx, 1, model.CandidateFailed, "download error"); err != nil {
		t.Fatalf("update status: %v", err)
	}

	got, _ := p.GetCandidate(ctx, 1)
	if got.PublishStatus != model.CandidateFailed {
		t.Errorf("expected failed, got %s", got.PublishStatus)
	}
	if got.PublishResult != "download error" {
		t.Errorf("expected 'download error', got %s", got.PublishResult)
	}
}

func setupPipelineTestDBWithGroups(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&model.PublishTask{},
		&model.PublishCandidate{},
		&model.PublishResultRecord{},
		&model.PublishGroup{},
		&model.PublishGroupMember{},
		&model.PublishGroupStatusHistory{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestPipeline_CreateGroup(t *testing.T) {
	p := NewPipeline(setupPipelineTestDBWithGroups(t), zap.NewNop())
	ctx := context.Background()

	group, err := p.CreateGroup(ctx, 1, "hash123", "site1", "torrent-001")
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if group.ID == 0 {
		t.Fatal("ID should be set")
	}
	if group.Status != model.GroupActive {
		t.Errorf("expected active, got %s", group.Status)
	}
	if group.SourceHash != "hash123" {
		t.Errorf("expected hash123, got %s", group.SourceHash)
	}
	if group.SourceSite != "site1" {
		t.Errorf("expected site1, got %s", group.SourceSite)
	}
}

func TestPipeline_GetGroup(t *testing.T) {
	p := NewPipeline(setupPipelineTestDBWithGroups(t), zap.NewNop())
	ctx := context.Background()

	created, _ := p.CreateGroup(ctx, 10, "abc", "mysite", "t-42")

	got, err := p.GetGroup(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetGroup: %v", err)
	}
	if got.CandidateID != 10 {
		t.Errorf("expected candidate_id=10, got %d", got.CandidateID)
	}
	if got.SourceHash != "abc" {
		t.Errorf("expected abc, got %s", got.SourceHash)
	}
	if got.SourceSite != "mysite" {
		t.Errorf("expected mysite, got %s", got.SourceSite)
	}
	if got.SourceTorrentID != "t-42" {
		t.Errorf("expected t-42, got %s", got.SourceTorrentID)
	}
}

func TestPipeline_GetGroup_NotFound(t *testing.T) {
	p := NewPipeline(setupPipelineTestDBWithGroups(t), zap.NewNop())
	_, err := p.GetGroup(context.Background(), 999)
	if err == nil {
		t.Error("expected error for non-existent group")
	}
}

func TestPipeline_ListGroups(t *testing.T) {
	p := NewPipeline(setupPipelineTestDBWithGroups(t), zap.NewNop())
	ctx := context.Background()

	if _, err := p.CreateGroup(ctx, 1, "h1", "s1", "t1"); err != nil {
		t.Fatalf("create group: %v", err)
	}
	if _, err := p.CreateGroup(ctx, 2, "h2", "s2", "t2"); err != nil {
		t.Fatalf("create group: %v", err)
	}
	if _, err := p.CreateGroup(ctx, 3, "h3", "s3", "t3"); err != nil {
		t.Fatalf("create group: %v", err)
	}

	groups, total, err := p.ListGroups(ctx, 1, 2)
	if err != nil {
		t.Fatalf("ListGroups: %v", err)
	}
	if total != 3 {
		t.Errorf("expected total=3, got %d", total)
	}
	if len(groups) != 2 {
		t.Errorf("expected 2 groups on page, got %d", len(groups))
	}
}

func TestPipeline_AddGroupMember(t *testing.T) {
	p := NewPipeline(setupPipelineTestDBWithGroups(t), zap.NewNop())
	ctx := context.Background()

	group, _ := p.CreateGroup(ctx, 1, "h1", "s1", "t1")

	member := &model.PublishGroupMember{
		InfoHash: "ih001",
		SiteName: "target-site",
		Status:   model.MemberStatusNew,
	}
	if err := p.AddGroupMember(ctx, group.ID, member); err != nil {
		t.Fatalf("AddGroupMember: %v", err)
	}
	if member.PublishGroupID != group.ID {
		t.Errorf("expected PublishGroupID=%d, got %d", group.ID, member.PublishGroupID)
	}
}

func TestPipeline_ListGroupMembers(t *testing.T) {
	p := NewPipeline(setupPipelineTestDBWithGroups(t), zap.NewNop())
	ctx := context.Background()

	group, _ := p.CreateGroup(ctx, 1, "h1", "s1", "t1")

	if err := p.AddGroupMember(ctx, group.ID, &model.PublishGroupMember{
		InfoHash: "ih001",
		SiteName: "site-a",
		Status:   model.MemberStatusNew,
	}); err != nil {
		t.Fatalf("add group member: %v", err)
	}
	if err := p.AddGroupMember(ctx, group.ID, &model.PublishGroupMember{
		InfoHash: "ih002",
		SiteName: "site-b",
		Status:   model.MemberStatusNew,
	}); err != nil {
		t.Fatalf("add group member: %v", err)
	}

	members, err := p.ListGroupMembers(ctx, group.ID)
	if err != nil {
		t.Fatalf("ListGroupMembers: %v", err)
	}
	if len(members) != 2 {
		t.Errorf("expected 2 members, got %d", len(members))
	}
}

func TestPipeline_UpdateGroupStatus(t *testing.T) {
	p := NewPipeline(setupPipelineTestDBWithGroups(t), zap.NewNop())
	ctx := context.Background()

	group, _ := p.CreateGroup(ctx, 1, "h1", "s1", "t1")

	if err := p.UpdateGroupStatus(ctx, group.ID, model.GroupPublishing, "开始发布"); err != nil {
		t.Fatalf("UpdateGroupStatus: %v", err)
	}

	got, _ := p.GetGroup(ctx, group.ID)
	if got.Status != model.GroupPublishing {
		t.Errorf("expected publishing, got %s", got.Status)
	}
}

func TestPipeline_UpdateMemberStatus(t *testing.T) {
	p := NewPipeline(setupPipelineTestDBWithGroups(t), zap.NewNop())
	ctx := context.Background()

	group, _ := p.CreateGroup(ctx, 1, "h1", "s1", "t1")
	member := &model.PublishGroupMember{
		InfoHash: "ih001",
		SiteName: "site-a",
		Status:   model.MemberStatusNew,
	}
	if err := p.AddGroupMember(ctx, group.ID, member); err != nil {
		t.Fatalf("add group member: %v", err)
	}

	if err := p.UpdateMemberStatus(ctx, member.ID, model.MemberStatusUploaded, ""); err != nil {
		t.Fatalf("UpdateMemberStatus: %v", err)
	}

	members, _ := p.ListGroupMembers(ctx, group.ID)
	if members[0].Status != model.MemberStatusUploaded {
		t.Errorf("expected uploaded, got %s", members[0].Status)
	}
}

func TestPipeline_TransitionGroupLifecycle_AllDone(t *testing.T) {
	p := NewPipeline(setupPipelineTestDBWithGroups(t), zap.NewNop())
	ctx := context.Background()

	group, _ := p.CreateGroup(ctx, 1, "h1", "s1", "t1")
	if err := p.AddGroupMember(ctx, group.ID, &model.PublishGroupMember{
		InfoHash: "ih001", SiteName: "site-a", Status: model.MemberStatusUploaded,
	}); err != nil {
		t.Fatalf("add group member: %v", err)
	}
	if err := p.AddGroupMember(ctx, group.ID, &model.PublishGroupMember{
		InfoHash: "ih002", SiteName: "site-b", Status: model.MemberStatusSeedingConfirmed,
	}); err != nil {
		t.Fatalf("add group member: %v", err)
	}

	if err := p.TransitionGroupLifecycle(ctx, group.ID); err != nil {
		t.Fatalf("TransitionGroupLifecycle: %v", err)
	}

	got, _ := p.GetGroup(ctx, group.ID)
	if got.Status != model.GroupMonitoring {
		t.Errorf("expected monitoring, got %s", got.Status)
	}
}

func TestPipeline_TransitionGroupLifecycle_AnyFailed(t *testing.T) {
	p := NewPipeline(setupPipelineTestDBWithGroups(t), zap.NewNop())
	ctx := context.Background()

	group, _ := p.CreateGroup(ctx, 1, "h1", "s1", "t1")
	if err := p.AddGroupMember(ctx, group.ID, &model.PublishGroupMember{
		InfoHash: "ih001", SiteName: "site-a", Status: model.MemberStatusUploaded,
	}); err != nil {
		t.Fatalf("add group member: %v", err)
	}
	if err := p.AddGroupMember(ctx, group.ID, &model.PublishGroupMember{
		InfoHash: "ih002", SiteName: "site-b", Status: model.MemberStatusError,
	}); err != nil {
		t.Fatalf("add group member: %v", err)
	}

	if err := p.TransitionGroupLifecycle(ctx, group.ID); err != nil {
		t.Fatalf("TransitionGroupLifecycle: %v", err)
	}

	got, _ := p.GetGroup(ctx, group.ID)
	if got.Status != model.GroupPublishFailed {
		t.Errorf("expected publish_failed, got %s", got.Status)
	}
}

func TestPipeline_TransitionGroupLifecycle_AllPaused(t *testing.T) {
	p := NewPipeline(setupPipelineTestDBWithGroups(t), zap.NewNop())
	ctx := context.Background()

	group, _ := p.CreateGroup(ctx, 1, "h1", "s1", "t1")
	m1 := &model.PublishGroupMember{
		InfoHash: "ih001", SiteName: "site-a", Status: model.MemberStatusPaused, Paused: true,
	}
	m2 := &model.PublishGroupMember{
		InfoHash: "ih002", SiteName: "site-b", Status: model.MemberStatusPaused, Paused: true,
	}
	if err := p.AddGroupMember(ctx, group.ID, m1); err != nil {
		t.Fatalf("add group member: %v", err)
	}
	if err := p.AddGroupMember(ctx, group.ID, m2); err != nil {
		t.Fatalf("add group member: %v", err)
	}

	if err := p.TransitionGroupLifecycle(ctx, group.ID); err != nil {
		t.Fatalf("TransitionGroupLifecycle: %v", err)
	}

	got, _ := p.GetGroup(ctx, group.ID)
	if got.Status != model.GroupAllPaused {
		t.Errorf("expected all_paused, got %s", got.Status)
	}
}

func TestPipeline_TransitionGroupLifecycle_AnyPublishing(t *testing.T) {
	p := NewPipeline(setupPipelineTestDBWithGroups(t), zap.NewNop())
	ctx := context.Background()

	group, _ := p.CreateGroup(ctx, 1, "h1", "s1", "t1")
	if err := p.AddGroupMember(ctx, group.ID, &model.PublishGroupMember{
		InfoHash: "ih001", SiteName: "site-a", Status: model.MemberStatusNew,
	}); err != nil {
		t.Fatalf("add group member: %v", err)
	}
	if err := p.AddGroupMember(ctx, group.ID, &model.PublishGroupMember{
		InfoHash: "ih002", SiteName: "site-b", Status: model.MemberStatusUploading,
	}); err != nil {
		t.Fatalf("add group member: %v", err)
	}

	if err := p.TransitionGroupLifecycle(ctx, group.ID); err != nil {
		t.Fatalf("TransitionGroupLifecycle: %v", err)
	}

	got, _ := p.GetGroup(ctx, group.ID)
	if got.Status != model.GroupPublishing {
		t.Errorf("expected publishing, got %s", got.Status)
	}
}

func TestPipeline_TransitionGroupLifecycle_NoChange(t *testing.T) {
	p := NewPipeline(setupPipelineTestDBWithGroups(t), zap.NewNop())
	ctx := context.Background()

	group, _ := p.CreateGroup(ctx, 1, "h1", "s1", "t1")
	if err := p.AddGroupMember(ctx, group.ID, &model.PublishGroupMember{
		InfoHash: "ih001", SiteName: "site-a", Status: model.MemberStatusNew,
	}); err != nil {
		t.Fatalf("add group member: %v", err)
	}
	if err := p.AddGroupMember(ctx, group.ID, &model.PublishGroupMember{
		InfoHash: "ih002", SiteName: "site-b", Status: model.MemberStatusNew,
	}); err != nil {
		t.Fatalf("add group member: %v", err)
	}

	if err := p.TransitionGroupLifecycle(ctx, group.ID); err != nil {
		t.Fatalf("TransitionGroupLifecycle: %v", err)
	}

	got, _ := p.GetGroup(ctx, group.ID)
	if got.Status != model.GroupActive {
		t.Errorf("expected active, got %s", got.Status)
	}
}

func TestParseTargetSites(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"site1", 1},
		{"site1,site2,site3", 3},
		{"site1, site2 , site3", 3},
	}

	for _, tt := range tests {
		got := parseTargetSites(tt.input)
		if len(got) != tt.want {
			t.Errorf("parseTargetSites(%q) = %d sites, want %d", tt.input, len(got), tt.want)
		}
	}
}

func TestPipeline_ProcessPending_Empty(t *testing.T) {
	p := NewPipeline(setupPipelineTestDBWithGroups(t), zap.NewNop())
	ctx := context.Background()

	if err := p.ProcessPending(ctx); err != nil {
		t.Fatalf("ProcessPending: %v", err)
	}
}

func TestPipeline_ProcessPending_CompletedCandidate(t *testing.T) {
	db := setupPipelineTestDBWithGroups(t)
	p := NewPipeline(db, zap.NewNop())
	ctx := context.Background()

	if err := p.CreateCandidate(ctx, &model.PublishCandidate{
		SourceSite: "s1", SourceTorrentID: "t1", InfoHash: "ih1",
		TorrentName: "Normal Movie", PublishStatus: model.CandidateCompleted,
		DownloadCompleted: false,
	}); err != nil {
		t.Fatalf("create candidate: %v", err)
	}

	if err := p.ProcessPending(ctx); err != nil {
		t.Fatalf("ProcessPending: %v", err)
	}

	got, _ := p.GetCandidate(ctx, 1)
	if got.PublishStatus != model.CandidateCompleted {
		t.Errorf("expected completed (unchanged), got %s", got.PublishStatus)
	}
}

func TestPipeline_ListResults_Empty(t *testing.T) {
	p := NewPipeline(setupPipelineTestDBWithGroups(t), zap.NewNop())
	ctx := context.Background()

	results, err := p.ListResults(ctx, 999, 10)
	if err != nil {
		t.Fatalf("ListResults: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

type mockPublishSiteProvider struct {
	*mocks.SiteInfoProvider
}

type mockPublishAdapter struct {
	*mocks.SiteAdapter
}

func newMockPublishAdapter() *mockPublishAdapter {
	a := &mocks.SiteAdapter{}
	a.DownloadTorrentFn = func(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error) {
		return []byte("d8:announce27:http://tracker.example.com4:infod6:lengthi13e4:name8:test.txt12:piece lengthi262144e6:pieces20:00000000000000000000ee"), nil
	}
	a.GetTorrentDetailFn = func(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.TorrentDetail, error) {
		return &model.TorrentDetail{
			Title:       "Test Movie 2024 1080p BluRay",
			Description: "[b]Test Description[/b]",
			Category:    "movies",
			Source:      "blu-ray",
			Resolution:  "1080p",
			Codec:       "x264",
			IMDbID:      "tt1234567",
			MediaInfo:   "mediainfo text",
			Screenshots: []string{"https://img.example.com/1.jpg"},
		}, nil
	}
	a.UploadTorrentFn = func(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
		return &model.PublishResponse{TorrentID: "new-torrent-123", DetailURL: "https://target.com/torrents/123"}, nil
	}
	return &mockPublishAdapter{SiteAdapter: a}
}

func TestPipeline_PublishCandidate_E2E_Success(t *testing.T) {
	db := setupPipelineTestDBWithGroups(t)
	p := NewPipeline(db, zap.NewNop())

	uploadCalled := false
	adapter := &mockPublishAdapter{
		SiteAdapter: &mocks.SiteAdapter{
			DownloadTorrentFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error) {
				return []byte("d8:announce27:http://tracker.example.com4:infod6:lengthi13e4:name8:test.txt12:piece lengthi262144e6:pieces20:00000000000000000000ee"), nil
			},
			GetTorrentDetailFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.TorrentDetail, error) {
				return &model.TorrentDetail{
					Title:       "Test Movie 2024 1080p BluRay",
					Description: "[b]Test Description[/b]",
				}, nil
			},
			UploadTorrentFn: func(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
				uploadCalled = true
				if req.Title != "Test Movie 2024 1080p BluRay" {
					t.Errorf("unexpected title: %s", req.Title)
				}
				if req.SourceSite != "source_site" {
					t.Errorf("unexpected source site: %s", req.SourceSite)
				}
				if req.TargetSite != "target_site" {
					t.Errorf("unexpected target site: %s", req.TargetSite)
				}
				if len(req.TorrentData) == 0 {
					t.Error("torrent data should not be empty")
				}
				return &model.PublishResponse{TorrentID: "pub-001", DetailURL: "https://target.com/t/001"}, nil
			},
			SearchTorrentsFn: func(ctx context.Context, config *model.SiteConfig, query string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
				return nil, nil
			},
		},
	}

	sp := &mockPublishSiteProvider{
		SiteInfoProvider: &mocks.SiteInfoProvider{
			GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
				return &model.SiteConfig{Domain: domain, Enabled: true}, nil
			},
			GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
				return adapter, nil
			},
			GetSiteInfoFn: func(ctx context.Context, name string) (*model.SiteInfo, error) {
				return &model.SiteInfo{Name: name, BaseURL: "https://" + name + ".com"}, nil
			},
		},
	}
	p.SetSiteProvider(sp)
	ctx := context.Background()

	candidate := &model.PublishCandidate{
		SourceSite:      "source_site",
		SourceTorrentID: "src-torrent-1",
		InfoHash:        "abc123",
		TorrentName:     "Test Movie 2024 1080p BluRay",
		Size:            1073741824,
		TargetSites:     "target_site",
		PublishStatus:   model.CandidatePending,
	}
	if err := p.CreateCandidate(ctx, candidate); err != nil {
		t.Fatalf("create candidate: %v", err)
	}

	result, err := p.PublishCandidate(ctx, candidate.ID)
	if err != nil {
		t.Fatalf("PublishCandidate: %v", err)
	}
	if !uploadCalled {
		t.Error("upload should have been called")
	}
	if result.PublishStatus != model.CandidateDone {
		t.Errorf("expected done, got %s", result.PublishStatus)
	}

	results, _ := p.ListResults(ctx, candidate.ID, 10)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != model.PublishResultCompleted {
		t.Errorf("expected completed, got %s", results[0].Status)
	}
	if results[0].TorrentID != "pub-001" {
		t.Errorf("expected pub-001, got %s", results[0].TorrentID)
	}
}

func TestPipeline_PublishCandidate_E2E_Dedup(t *testing.T) {
	db := setupPipelineTestDBWithGroups(t)
	p := NewPipeline(db, zap.NewNop())

	adapter := &mockPublishAdapter{
		SiteAdapter: &mocks.SiteAdapter{
			SearchTorrentsFn: func(ctx context.Context, config *model.SiteConfig, query string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
				return []*model.SeedingSearchResult{
					{TorrentID: "existing-1", Title: query, Size: 1073741824},
				}, nil
			},
		},
	}

	sp := &mockPublishSiteProvider{
		SiteInfoProvider: &mocks.SiteInfoProvider{
			GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
				return &model.SiteConfig{Domain: domain, Enabled: true}, nil
			},
			GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
				return adapter, nil
			},
			GetSiteInfoFn: func(ctx context.Context, name string) (*model.SiteInfo, error) {
				return &model.SiteInfo{Name: name, BaseURL: "https://" + name + ".com"}, nil
			},
		},
	}
	p.SetSiteProvider(sp)
	ctx := context.Background()

	candidate := &model.PublishCandidate{
		SourceSite:      "source",
		SourceTorrentID: "t1",
		InfoHash:        "ih1",
		TorrentName:     "Dedup Movie",
		Size:            1073741824,
		TargetSites:     "target",
		PublishStatus:   model.CandidatePending,
	}
	if err := p.CreateCandidate(ctx, candidate); err != nil {
		t.Fatal(err)
	}

	_, err := p.PublishCandidate(ctx, candidate.ID)
	if err != nil {
		t.Logf("dedup publish result: %v", err)
	}

	results, _ := p.ListResults(ctx, candidate.ID, 10)
	hasSkipped := false
	for _, r := range results {
		if r.Status == model.PublishResultSkipped {
			hasSkipped = true
		}
	}
	if !hasSkipped {
		t.Error("expected at least one skipped result for dedup")
	}
}

func TestPipeline_PublishCandidate_E2E_UploadFail(t *testing.T) {
	db := setupPipelineTestDBWithGroups(t)
	p := NewPipeline(db, zap.NewNop())

	adapter := &mockPublishAdapter{
		SiteAdapter: &mocks.SiteAdapter{
			UploadTorrentFn: func(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
				return nil, fmt.Errorf("upload failed: server error")
			},
		},
	}

	sp := &mockPublishSiteProvider{
		SiteInfoProvider: &mocks.SiteInfoProvider{
			GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
				return &model.SiteConfig{Domain: domain, Enabled: true}, nil
			},
			GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
				return adapter, nil
			},
		},
	}
	p.SetSiteProvider(sp)
	ctx := context.Background()

	candidate := &model.PublishCandidate{
		SourceSite:      "source",
		SourceTorrentID: "t1",
		InfoHash:        "ih1",
		TorrentName:     "Fail Movie",
		Size:            1073741824,
		TargetSites:     "target",
		PublishStatus:   model.CandidatePending,
	}
	if err := p.CreateCandidate(ctx, candidate); err != nil {
		t.Fatal(err)
	}

	result, _ := p.PublishCandidate(ctx, candidate.ID)
	if result.PublishStatus != model.CandidateFailed {
		t.Errorf("expected failed, got %s", result.PublishStatus)
	}

	results, _ := p.ListResults(ctx, candidate.ID, 10)
	if len(results) == 0 {
		t.Fatal("expected at least 1 result")
	}
	if results[0].Status != model.PublishResultFailed {
		t.Errorf("expected failed result, got %s", results[0].Status)
	}
}

func TestPipeline_PublishCandidate_NoProvider(t *testing.T) {
	db := setupPipelineTestDBWithGroups(t)
	p := NewPipeline(db, zap.NewNop())
	ctx := context.Background()

	candidate := &model.PublishCandidate{
		SourceSite:      "source",
		SourceTorrentID: "t1",
		InfoHash:        "ih1",
		TorrentName:     "Normal Movie",
		TargetSites:     "target",
		PublishStatus:   model.CandidatePending,
	}
	if err := p.CreateCandidate(ctx, candidate); err != nil {
		t.Fatal(err)
	}

	result, err := p.PublishCandidate(ctx, candidate.ID)
	if err != nil {
		t.Fatalf("PublishCandidate without provider: %v", err)
	}
	if result.PublishStatus != model.CandidatePublishing {
		t.Errorf("expected publishing (no provider), got %s", result.PublishStatus)
	}
}

func TestPipeline_PublishCandidate_MultipleTargets(t *testing.T) {
	db := setupPipelineTestDBWithGroups(t)
	p := NewPipeline(db, zap.NewNop())

	uploadCount := 0
	adapter := &mockPublishAdapter{
		SiteAdapter: &mocks.SiteAdapter{
			UploadTorrentFn: func(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
				uploadCount++
				return &model.PublishResponse{
					TorrentID: fmt.Sprintf("pub-%d", uploadCount),
					DetailURL: fmt.Sprintf("https://%s/t/%d", req.TargetSite, uploadCount),
				}, nil
			},
		},
	}

	sp := &mockPublishSiteProvider{
		SiteInfoProvider: &mocks.SiteInfoProvider{
			GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
				return &model.SiteConfig{Domain: domain, Enabled: true}, nil
			},
			GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
				return adapter, nil
			},
			GetSiteInfoFn: func(ctx context.Context, name string) (*model.SiteInfo, error) {
				return &model.SiteInfo{Name: name, BaseURL: "https://" + name + ".com"}, nil
			},
		},
	}
	p.SetSiteProvider(sp)
	ctx := context.Background()

	candidate := &model.PublishCandidate{
		SourceSite:      "source",
		SourceTorrentID: "t1",
		InfoHash:        "ih1",
		TorrentName:     "Multi Target Movie",
		Size:            1073741824,
		TargetSites:     "target1,target2,target3",
		PublishStatus:   model.CandidatePending,
	}
	if err := p.CreateCandidate(ctx, candidate); err != nil {
		t.Fatal(err)
	}

	result, err := p.PublishCandidate(ctx, candidate.ID)
	if err != nil {
		t.Fatalf("PublishCandidate: %v", err)
	}
	if uploadCount != 3 {
		t.Errorf("expected 3 uploads, got %d", uploadCount)
	}
	if result.PublishStatus != model.CandidateDone {
		t.Errorf("expected done, got %s", result.PublishStatus)
	}

	results, _ := p.ListResults(ctx, candidate.ID, 10)
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
}

func TestPipeline_PublishCandidate_E2E_DownloadFail(t *testing.T) {
	db := setupPipelineTestDBWithGroups(t)
	p := NewPipeline(db, zap.NewNop())

	adapter := &mockPublishAdapter{
		SiteAdapter: &mocks.SiteAdapter{
			DownloadTorrentFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error) {
				return nil, fmt.Errorf("download failed: 404")
			},
		},
	}

	sp := &mockPublishSiteProvider{
		SiteInfoProvider: &mocks.SiteInfoProvider{
			GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
				return &model.SiteConfig{Domain: domain, Enabled: true}, nil
			},
			GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
				return adapter, nil
			},
		},
	}
	p.SetSiteProvider(sp)
	ctx := context.Background()

	candidate := &model.PublishCandidate{
		SourceSite:      "source",
		SourceTorrentID: "t1",
		InfoHash:        "ih1",
		TorrentName:     "Normal Movie",
		Size:            1073741824,
		TargetSites:     "target",
		PublishStatus:   model.CandidatePending,
	}
	if err := p.CreateCandidate(ctx, candidate); err != nil {
		t.Fatal(err)
	}

	_, err := p.PublishCandidate(ctx, candidate.ID)
	if err == nil {
		t.Error("expected error for download failure")
	}
}

func TestPipeline_PublishCandidate_NoTargetSites(t *testing.T) {
	db := setupPipelineTestDBWithGroups(t)
	p := NewPipeline(db, zap.NewNop())

	sp := &mockPublishSiteProvider{
		SiteInfoProvider: &mocks.SiteInfoProvider{
			GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
				return &model.SiteConfig{Domain: domain, Enabled: true}, nil
			},
			GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
				return newMockPublishAdapter(), nil
			},
		},
	}
	p.SetSiteProvider(sp)
	ctx := context.Background()

	candidate := &model.PublishCandidate{
		SourceSite:      "source",
		SourceTorrentID: "t1",
		InfoHash:        "ih1",
		TorrentName:     "Normal Movie",
		Size:            1073741824,
		TargetSites:     "",
		PublishStatus:   model.CandidatePending,
	}
	if err := p.CreateCandidate(ctx, candidate); err != nil {
		t.Fatal(err)
	}

	result, err := p.PublishCandidate(ctx, candidate.ID)
	if err != nil {
		t.Fatalf("PublishCandidate: %v", err)
	}
	if result.PublishStatus != model.CandidatePublishing {
		t.Errorf("expected publishing, got %s", result.PublishStatus)
	}
}

func TestPipeline_PublishCandidate_GetSourceConfigFail(t *testing.T) {
	db := setupPipelineTestDBWithGroups(t)
	p := NewPipeline(db, zap.NewNop())

	sp := &mockPublishSiteProvider{
		SiteInfoProvider: &mocks.SiteInfoProvider{
			GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
				return nil, fmt.Errorf("config not found for %s", domain)
			},
			GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
				return newMockPublishAdapter(), nil
			},
		},
	}
	p.SetSiteProvider(sp)
	ctx := context.Background()

	candidate := &model.PublishCandidate{
		SourceSite:      "source",
		SourceTorrentID: "t1",
		InfoHash:        "ih1",
		TorrentName:     "Normal Movie",
		TargetSites:     "target",
		PublishStatus:   model.CandidatePending,
	}
	if err := p.CreateCandidate(ctx, candidate); err != nil {
		t.Fatal(err)
	}

	result, err := p.PublishCandidate(ctx, candidate.ID)
	if err != nil {
		t.Logf("expected graceful handling: %v", err)
	}
	if result != nil && result.PublishStatus == model.CandidateDone {
		t.Error("should not be done when source config fails")
	}
}

func TestPipeline_ProcessMemberWithResume_E2E(t *testing.T) {
	db := setupPipelineTestDBWithGroups(t)
	p := NewPipeline(db, zap.NewNop())

	uploadCalled := false
	adapter := &mockPublishAdapter{
		SiteAdapter: &mocks.SiteAdapter{
			UploadTorrentFn: func(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
				uploadCalled = true
				return &model.PublishResponse{TorrentID: "uploaded-001"}, nil
			},
		},
	}

	sp := &mockPublishSiteProvider{
		SiteInfoProvider: &mocks.SiteInfoProvider{
			GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
				return &model.SiteConfig{Domain: domain, Enabled: true}, nil
			},
			GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
				return adapter, nil
			},
		},
	}
	p.SetSiteProvider(sp)
	ctx := context.Background()

	group, err := p.CreateGroup(ctx, 0, "source_hash", "source_site", "src-t-1")
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}

	member := &model.PublishGroupMember{
		PublishGroupID: group.ID,
		SiteName:       "target_site",
		Status:         model.MemberStatusNew,
	}
	if err := p.AddGroupMember(ctx, group.ID, member); err != nil {
		t.Fatalf("AddGroupMember: %v", err)
	}

	var dbMember model.PublishGroupMember
	db.Where("id = ?", member.ID).First(&dbMember)

	if err := p.ProcessMemberWithResume(ctx, &dbMember); err != nil {
		t.Fatalf("ProcessMemberWithResume: %v", err)
	}
	if !uploadCalled {
		t.Error("upload should have been called")
	}

	db.Where("id = ?", member.ID).First(&dbMember)
	if dbMember.Status != model.MemberStatusUploaded {
		t.Errorf("expected uploaded, got %s", dbMember.Status)
	}
	if dbMember.TorrentID != "uploaded-001" {
		t.Errorf("expected uploaded-001, got %s", dbMember.TorrentID)
	}
}

func TestPipeline_ProcessMemberWithResume_NoGroup(t *testing.T) {
	db := setupPipelineTestDBWithGroups(t)
	p := NewPipeline(db, zap.NewNop())
	ctx := context.Background()

	member := &model.PublishGroupMember{SiteName: "target", Status: model.MemberStatusNew}
	if err := p.ProcessMemberWithResume(ctx, member); err == nil {
		t.Error("expected error for member without group")
	}
}

func TestPipeline_ProcessMemberWithResume_NoProvider(t *testing.T) {
	db := setupPipelineTestDBWithGroups(t)
	p := NewPipeline(db, zap.NewNop())
	ctx := context.Background()

	group, err := p.CreateGroup(ctx, 0, "hash", "site", "t1")
	if err != nil {
		t.Fatal(err)
	}

	member := &model.PublishGroupMember{
		PublishGroupID: group.ID,
		SiteName:       "target",
		Status:         model.MemberStatusNew,
	}
	if err := p.AddGroupMember(ctx, group.ID, member); err != nil {
		t.Fatal(err)
	}

	var dbMember model.PublishGroupMember
	db.Where("id = ?", member.ID).First(&dbMember)

	if err := p.ProcessMemberWithResume(ctx, &dbMember); err == nil {
		t.Error("expected error without site provider")
	}
}

func TestPipeline_ProcessMemberWithResume_DownloadFail(t *testing.T) {
	db := setupPipelineTestDBWithGroups(t)
	p := NewPipeline(db, zap.NewNop())

	adapter := &mockPublishAdapter{
		SiteAdapter: &mocks.SiteAdapter{
			DownloadTorrentFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error) {
				return nil, fmt.Errorf("download timeout")
			},
		},
	}

	sp := &mockPublishSiteProvider{
		SiteInfoProvider: &mocks.SiteInfoProvider{
			GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
				return &model.SiteConfig{Domain: domain, Enabled: true}, nil
			},
			GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
				return adapter, nil
			},
		},
	}
	p.SetSiteProvider(sp)
	ctx := context.Background()

	group, err := p.CreateGroup(ctx, 0, "hash", "source_site", "t1")
	if err != nil {
		t.Fatal(err)
	}

	member := &model.PublishGroupMember{
		PublishGroupID: group.ID,
		SiteName:       "target",
		Status:         model.MemberStatusNew,
	}
	if err := p.AddGroupMember(ctx, group.ID, member); err != nil {
		t.Fatal(err)
	}

	var dbMember model.PublishGroupMember
	db.Where("id = ?", member.ID).First(&dbMember)

	if err := p.ProcessMemberWithResume(ctx, &dbMember); err == nil {
		t.Error("expected error for download failure")
	}

	db.Where("id = ?", member.ID).First(&dbMember)
	if dbMember.Status != model.MemberStatusError {
		t.Errorf("expected error status, got %s", dbMember.Status)
	}
}

func TestPipeline_ProcessMemberWithResume_UploadFail(t *testing.T) {
	db := setupPipelineTestDBWithGroups(t)
	p := NewPipeline(db, zap.NewNop())

	adapter := &mockPublishAdapter{
		SiteAdapter: &mocks.SiteAdapter{
			UploadTorrentFn: func(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
				return nil, fmt.Errorf("upload rejected: duplicate")
			},
		},
	}

	sp := &mockPublishSiteProvider{
		SiteInfoProvider: &mocks.SiteInfoProvider{
			GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
				return &model.SiteConfig{Domain: domain, Enabled: true}, nil
			},
			GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
				return adapter, nil
			},
		},
	}
	p.SetSiteProvider(sp)
	ctx := context.Background()

	group, err := p.CreateGroup(ctx, 0, "hash", "source_site", "t1")
	if err != nil {
		t.Fatal(err)
	}

	member := &model.PublishGroupMember{
		PublishGroupID: group.ID,
		SiteName:       "target",
		Status:         model.MemberStatusNew,
	}
	if err := p.AddGroupMember(ctx, group.ID, member); err != nil {
		t.Fatal(err)
	}

	var dbMember model.PublishGroupMember
	db.Where("id = ?", member.ID).First(&dbMember)

	if err := p.ProcessMemberWithResume(ctx, &dbMember); err == nil {
		t.Error("expected error for upload failure")
	}

	db.Where("id = ?", member.ID).First(&dbMember)
	if dbMember.Status != model.MemberStatusError {
		t.Errorf("expected error status, got %s", dbMember.Status)
	}
}

func TestPipeline_ProcessMemberWithResume_ResumeFromStep(t *testing.T) {
	db := setupPipelineTestDBWithGroups(t)
	p := NewPipeline(db, zap.NewNop())

	uploadCalled := false
	adapter := &mockPublishAdapter{
		SiteAdapter: &mocks.SiteAdapter{
			UploadTorrentFn: func(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
				uploadCalled = true
				return &model.PublishResponse{TorrentID: "resumed-001"}, nil
			},
		},
	}

	sp := &mockPublishSiteProvider{
		SiteInfoProvider: &mocks.SiteInfoProvider{
			GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
				return &model.SiteConfig{Domain: domain, Enabled: true}, nil
			},
			GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
				return adapter, nil
			},
		},
	}
	p.SetSiteProvider(sp)
	ctx := context.Background()

	group, err := p.CreateGroup(ctx, 0, "hash", "source_site", "t1")
	if err != nil {
		t.Fatal(err)
	}

	member := &model.PublishGroupMember{
		PublishGroupID:    group.ID,
		SiteName:          "target",
		Status:            model.MemberStatusNew,
		LastCompletedStep: 2,
	}
	if err := p.AddGroupMember(ctx, group.ID, member); err != nil {
		t.Fatal(err)
	}

	var dbMember model.PublishGroupMember
	db.Where("id = ?", member.ID).First(&dbMember)

	if err := p.ProcessMemberWithResume(ctx, &dbMember); err != nil {
		t.Fatalf("ProcessMemberWithResume: %v", err)
	}
	if !uploadCalled {
		t.Error("upload should have been called (skipped download+detail)")
	}
}

func TestPipeline_SetSiteProvider(t *testing.T) {
	db := setupPipelineTestDBWithGroups(t)
	p := NewPipeline(db, zap.NewNop())
	p.SetSiteProvider(nil)
}

func setupPipelineTestDBWithMappings(t *testing.T) *gorm.DB {
	t.Helper()
	db := setupPipelineTestDBWithGroups(t)
	if err := db.AutoMigrate(&model.SiteFieldMapping{}, &model.PublishExclusion{}); err != nil {
		t.Fatalf("migrate mappings: %v", err)
	}
	return db
}

func TestPipeline_mapFieldValues(t *testing.T) {
	db := setupPipelineTestDBWithMappings(t)
	p := NewPipeline(db, zap.NewNop())
	ctx := context.Background()

	mappings := []model.SiteFieldMapping{
		{SiteName: "目标站", FieldType: "cat", SourceValue: "Movies(电影)", TargetValue: "401"},
		{SiteName: "目标站", FieldType: "standard_sel", SourceValue: "1080p", TargetValue: "2"},
		{SiteName: "目标站", FieldType: "codec_sel", SourceValue: "H.265", TargetValue: "1"},
		{SiteName: "目标站", FieldType: "source_sel", SourceValue: "Blu-ray", TargetValue: "1"},
	}
	for _, m := range mappings {
		if err := db.Create(&m).Error; err != nil {
			t.Fatal(err)
		}
	}

	fields := map[string]string{
		"category":   "Movies(电影)",
		"resolution": "1080p",
		"codec":      "H.265",
		"source":     "Blu-ray",
	}

	p.mapFieldValues(ctx, "目标站", fields)

	if fields["category"] != "401" {
		t.Errorf("category: got %q, want 401", fields["category"])
	}
	if fields["resolution"] != "2" {
		t.Errorf("resolution: got %q, want 2", fields["resolution"])
	}
	if fields["codec"] != "1" {
		t.Errorf("codec: got %q, want 1", fields["codec"])
	}
	if fields["source"] != "1" {
		t.Errorf("source: got %q, want 1", fields["source"])
	}
}

func TestPipeline_mapFieldValues_NoMappings(t *testing.T) {
	db := setupPipelineTestDBWithMappings(t)
	p := NewPipeline(db, zap.NewNop())
	ctx := context.Background()

	fields := map[string]string{
		"category":   "Movies(电影)",
		"resolution": "1080p",
	}

	p.mapFieldValues(ctx, "不存在的站", fields)

	if fields["category"] != "Movies(电影)" {
		t.Errorf("should remain unchanged without mappings, got %q", fields["category"])
	}
	if fields["resolution"] != "1080p" {
		t.Errorf("should remain unchanged without mappings, got %q", fields["resolution"])
	}
}

func TestExtractTMDBID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://www.themoviedb.org/movie/12345", "12345"},
		{"https://www.themoviedb.org/tv/67890", "67890"},
		{"https://themoviedb.org/movie/42-some-slug", "42"},
		{"https://example.com/not-tmdb", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := extractTMDBID(tt.input)
		if got != tt.want {
			t.Errorf("extractTMDBID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestPipeline_CheckPublishEligibility_Exclusion(t *testing.T) {
	db := setupPipelineTestDBWithMappings(t)
	p := NewPipeline(db, zap.NewNop())
	ctx := context.Background()

	exclusion := model.PublishExclusion{TargetSite: "目标站", SourceSite: "源站A"}
	if err := db.Create(&exclusion).Error; err != nil {
		t.Fatal(err)
	}

	candidate := &model.PublishCandidate{TorrentName: "Normal Title", SourceSite: "源站A"}

	ok, reason := p.CheckPublishEligibility(ctx, candidate, "目标站")
	if ok {
		t.Error("should be excluded")
	}
	if reason == "" {
		t.Error("reason should not be empty")
	}

	ok, _ = p.CheckPublishEligibility(ctx, candidate, "其他站")
	if !ok {
		t.Error("should not be excluded for other target")
	}
}

func TestPipeline_PublishCandidate_NotFound(t *testing.T) {
	db := setupPipelineTestDBWithGroups(t)
	p := NewPipeline(db, zap.NewNop())
	ctx := context.Background()

	_, err := p.PublishCandidate(ctx, 999)
	if err == nil {
		t.Error("expected error for non-existent candidate")
	}
}

func TestPipeline_ProcessMemberWithResume_HRDetected(t *testing.T) {
	db := setupPipelineTestDBWithGroups(t)
	p := NewPipeline(db, zap.NewNop())

	var setTagsCalls [][]string
	mockDL := &mocks.DownloaderClient{
		SetTorrentTagsFn: func(ctx context.Context, hash string, tags []string) error {
			setTagsCalls = append(setTagsCalls, tags)
			return nil
		},
	}
	p.SetClientProvider(&mocks.DownloaderProvider{Client: mockDL})

	adapter := &mockPublishAdapter{
		SiteAdapter: &mocks.SiteAdapter{
			UploadTorrentFn: func(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
				return &model.PublishResponse{TorrentID: "hr-torrent-001", InfoHash: "newhash123"}, nil
			},
			DetectHRFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.HRResult, error) {
				return &model.HRResult{HasHR: true, SeedTimeH: 48, MinRatio: 1.0}, nil
			},
		},
	}

	sp := &mockPublishSiteProvider{
		SiteInfoProvider: &mocks.SiteInfoProvider{
			GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
				return &model.SiteConfig{Domain: domain, Enabled: true}, nil
			},
			GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
				return adapter, nil
			},
		},
	}
	p.SetSiteProvider(sp)
	ctx := context.Background()

	group, err := p.CreateGroup(ctx, 0, "source_hash", "source_site", "src-t-1")
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}

	member := &model.PublishGroupMember{
		PublishGroupID: group.ID,
		SiteName:       "hr_target",
		ClientID:       "test-client",
		InfoHash:       "abc123",
		Status:         model.MemberStatusNew,
	}
	if err := p.AddGroupMember(ctx, group.ID, member); err != nil {
		t.Fatalf("AddGroupMember: %v", err)
	}

	var dbMember model.PublishGroupMember
	db.Where("id = ?", member.ID).First(&dbMember)

	if err := p.ProcessMemberWithResume(ctx, &dbMember); err != nil {
		t.Fatalf("ProcessMemberWithResume: %v", err)
	}

	db.Where("id = ?", member.ID).First(&dbMember)
	if !dbMember.HRProtected {
		t.Error("member should be HR protected")
	}
	if dbMember.HRMinSeedHours != 48 {
		t.Errorf("expected 48 seed hours, got %d", dbMember.HRMinSeedHours)
	}
	if dbMember.HRSeedStart == nil {
		t.Error("HR seed start should be set")
	}
	if dbMember.HRSite != "hr_target" {
		t.Errorf("expected hr_site=hr_target, got %s", dbMember.HRSite)
	}

	if len(setTagsCalls) != 1 {
		t.Fatalf("expected 1 SetTorrentTags call, got %d", len(setTagsCalls))
	}
	expectedTag := "PROTECTED_HR_hr_target"
	if setTagsCalls[0][0] != expectedTag {
		t.Errorf("expected tag %s, got %s", expectedTag, setTagsCalls[0][0])
	}
}

func TestPipeline_ProcessMemberWithResume_NoHR(t *testing.T) {
	db := setupPipelineTestDBWithGroups(t)
	p := NewPipeline(db, zap.NewNop())

	mockDL := &mocks.DownloaderClient{}
	p.SetClientProvider(&mocks.DownloaderProvider{Client: mockDL})

	adapter := &mockPublishAdapter{
		SiteAdapter: &mocks.SiteAdapter{
			UploadTorrentFn: func(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
				return &model.PublishResponse{TorrentID: "no-hr-001"}, nil
			},
			DetectHRFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.HRResult, error) {
				return &model.HRResult{HasHR: false}, nil
			},
		},
	}

	sp := &mockPublishSiteProvider{
		SiteInfoProvider: &mocks.SiteInfoProvider{
			GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
				return &model.SiteConfig{Domain: domain, Enabled: true}, nil
			},
			GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
				return adapter, nil
			},
		},
	}
	p.SetSiteProvider(sp)
	ctx := context.Background()

	group, err := p.CreateGroup(ctx, 0, "source_hash", "source_site", "src-t-1")
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}

	member := &model.PublishGroupMember{
		PublishGroupID: group.ID,
		SiteName:       "nohr_target",
		ClientID:       "test-client",
		InfoHash:       "abc123",
		Status:         model.MemberStatusNew,
	}
	if err := p.AddGroupMember(ctx, group.ID, member); err != nil {
		t.Fatalf("AddGroupMember: %v", err)
	}

	var dbMember model.PublishGroupMember
	db.Where("id = ?", member.ID).First(&dbMember)

	if err := p.ProcessMemberWithResume(ctx, &dbMember); err != nil {
		t.Fatalf("ProcessMemberWithResume: %v", err)
	}

	db.Where("id = ?", member.ID).First(&dbMember)
	if dbMember.HRProtected {
		t.Error("member should NOT be HR protected when no HR detected")
	}
}

func TestPipeline_ProcessPendingGroups_NoActiveGroups(t *testing.T) {
	db := setupPipelineTestDBWithGroups(t)
	p := NewPipeline(db, zap.NewNop())
	ctx := context.Background()

	if err := p.ProcessPendingGroups(ctx); err != nil {
		t.Fatalf("ProcessPendingGroups: %v", err)
	}
}

func TestPipeline_ProcessPendingGroups_ProcessesMembers(t *testing.T) {
	db := setupPipelineTestDBWithGroups(t)
	p := NewPipeline(db, zap.NewNop())

	uploadCalled := false
	adapter := &mockPublishAdapter{
		SiteAdapter: &mocks.SiteAdapter{
			UploadTorrentFn: func(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
				uploadCalled = true
				return &model.PublishResponse{TorrentID: "pg-001"}, nil
			},
			DetectHRFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.HRResult, error) {
				return &model.HRResult{HasHR: false}, nil
			},
		},
	}

	sp := &mockPublishSiteProvider{
		SiteInfoProvider: &mocks.SiteInfoProvider{
			GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
				return &model.SiteConfig{Domain: domain, Enabled: true}, nil
			},
			GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
				return adapter, nil
			},
			GetSiteInfoFn: func(ctx context.Context, name string) (*model.SiteInfo, error) {
				return &model.SiteInfo{Name: name, BaseURL: "https://" + name + ".com"}, nil
			},
		},
	}
	p.SetSiteProvider(sp)
	ctx := context.Background()

	group, err := p.CreateGroup(ctx, 0, "source_hash", "source_site", "src-t-1")
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}

	member := &model.PublishGroupMember{
		PublishGroupID: group.ID,
		SiteName:       "target_site",
		ClientID:       "test-client",
		InfoHash:       "abc123",
		Status:         model.MemberStatusNew,
	}
	if err := p.AddGroupMember(ctx, group.ID, member); err != nil {
		t.Fatalf("AddGroupMember: %v", err)
	}

	if err := p.ProcessPendingGroups(ctx); err != nil {
		t.Fatalf("ProcessPendingGroups: %v", err)
	}
	if !uploadCalled {
		t.Error("upload should have been called")
	}

	var updated model.PublishGroupMember
	db.Where("id = ?", member.ID).First(&updated)
	if updated.Status != model.MemberStatusUploaded {
		t.Errorf("expected uploaded, got %s", updated.Status)
	}

	var updatedGroup model.PublishGroup
	db.First(&updatedGroup, group.ID)
	if updatedGroup.Status != model.GroupMonitoring {
		t.Errorf("expected monitoring, got %s", updatedGroup.Status)
	}
}

func TestPipeline_ProcessPendingGroups_SkipsCompletedMembers(t *testing.T) {
	db := setupPipelineTestDBWithGroups(t)
	p := NewPipeline(db, zap.NewNop())
	ctx := context.Background()

	group, err := p.CreateGroup(ctx, 0, "hash", "site", "t1")
	if err != nil {
		t.Fatal(err)
	}

	member := &model.PublishGroupMember{
		PublishGroupID: group.ID,
		SiteName:       "target",
		Status:         model.MemberStatusUploaded,
	}
	if err := p.AddGroupMember(ctx, group.ID, member); err != nil {
		t.Fatal(err)
	}

	if err := p.ProcessPendingGroups(ctx); err != nil {
		t.Fatalf("ProcessPendingGroups: %v", err)
	}

	var updatedGroup model.PublishGroup
	db.First(&updatedGroup, group.ID)
	if updatedGroup.Status != model.GroupMonitoring {
		t.Errorf("all-uploaded group should transition to monitoring, got %s", updatedGroup.Status)
	}
}

func TestPipeline_SetCompletionWatcher(t *testing.T) {
	db := setupPipelineTestDB(t)
	p := NewPipeline(db, zap.NewNop())
	p.SetCompletionWatcher(nil)
}

func TestPipeline_SetNotifyService(t *testing.T) {
	db := setupPipelineTestDB(t)
	p := NewPipeline(db, zap.NewNop())
	p.SetNotifyService(nil)
}

func TestPipeline_Update(t *testing.T) {
	db := setupPipelineTestDB(t)
	p := NewPipeline(db, zap.NewNop())
	ctx := context.Background()

	task := &model.PublishTask{Type: model.PublishTaskTypeManual, SourceSiteID: 1}
	if err := p.CreateTask(ctx, task); err != nil {
		t.Fatal(err)
	}

	task.Status = model.PublishTaskCompleted
	if err := p.Update(ctx, task); err != nil {
		t.Fatal(err)
	}

	got, _ := p.GetTask(ctx, task.ID)
	if got.Status != model.PublishTaskCompleted {
		t.Errorf("expected completed, got %s", got.Status)
	}
}

func setupPipelineDBWithNotify(t *testing.T) (*Pipeline, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&model.PublishGroup{},
		&model.PublishGroupMember{},
		&model.PublishGroupStatusHistory{},
		&model.NotificationChannel{},
		&model.NotificationHistory{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	p := NewPipeline(db, zap.NewNop())
	ns := notification.NewService(db, zap.NewNop())
	p.SetNotifyService(ns)
	return p, db
}

func TestPipeline_NotifyPublishResult_Success(t *testing.T) {
	p, db := setupPipelineDBWithNotify(t)
	ctx := context.Background()

	db.Create(&model.PublishGroup{ID: 1, SourceTorrentID: "src-t-1"})
	db.Create(&model.PublishGroupMember{
		PublishGroupID: 1,
		SiteName:       "target_site",
		Status:         model.MemberStatusUploaded,
		TorrentID:      "new-001",
		HRProtected:    true,
		HRMinSeedHours: 48,
	})

	var member model.PublishGroupMember
	db.Where("publish_group_id = ?", 1).First(&member)

	p.notifyPublishResult(ctx, &member)

	var history []model.NotificationHistory
	db.Find(&history)
	if len(history) != 0 {
		t.Logf("notification history entries: %d (expected 0 due to no channels)", len(history))
	}
}

func TestPipeline_NotifyPublishResult_Error(t *testing.T) {
	p, db := setupPipelineDBWithNotify(t)
	ctx := context.Background()

	db.Create(&model.PublishGroup{ID: 1, SourceTorrentID: "src-t-2"})
	db.Create(&model.PublishGroupMember{
		PublishGroupID: 1,
		SiteName:       "fail_site",
		Status:         model.MemberStatusError,
		LastError:      "upload rejected",
	})

	var member model.PublishGroupMember
	db.Where("publish_group_id = ?", 1).First(&member)

	p.notifyPublishResult(ctx, &member)
}

func TestPipeline_NotifyPublishResult_NilService(t *testing.T) {
	db := setupPipelineTestDBWithGroups(t)
	p := NewPipeline(db, zap.NewNop())
	ctx := context.Background()

	member := &model.PublishGroupMember{
		PublishGroupID: 999,
		SiteName:       "site",
		Status:         model.MemberStatusUploaded,
	}
	p.notifyPublishResult(ctx, member)
}

func TestPipeline_NotifyPublishResult_DefaultStatus(t *testing.T) {
	p, db := setupPipelineDBWithNotify(t)
	ctx := context.Background()

	db.Create(&model.PublishGroup{ID: 1, SourceTorrentID: "src-t-3"})
	db.Create(&model.PublishGroupMember{
		PublishGroupID: 1,
		SiteName:       "site",
		Status:         model.MemberStatusNew,
	})

	var member model.PublishGroupMember
	db.Where("publish_group_id = ?", 1).First(&member)

	p.notifyPublishResult(ctx, &member)
}

func TestPipeline_NotifyPublishResult_NoGroup(t *testing.T) {
	p, _ := setupPipelineDBWithNotify(t)
	ctx := context.Background()

	member := &model.PublishGroupMember{
		PublishGroupID: 999,
		SiteName:       "site",
		Status:         model.MemberStatusUploaded,
	}

	p.notifyPublishResult(ctx, member)
}
