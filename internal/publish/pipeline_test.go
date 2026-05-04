package publish

import (
	"context"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
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
	db.AutoMigrate(&model.PublishTask{}, &model.PublishCandidate{}, &model.PublishResultRecord{})
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
	p.CreateTask(ctx, task)

	p.UpdateTaskStatus(ctx, task.ID, model.PublishTaskPublishing)

	got, _ := p.GetTask(ctx, task.ID)
	if got.Status != model.PublishTaskPublishing {
		t.Errorf("expected publishing, got %s", got.Status)
	}
}

func TestPipeline_ListTasks(t *testing.T) {
	p := NewPipeline(setupPipelineTestDB(t), zap.NewNop())
	ctx := context.Background()

	p.CreateTask(ctx, &model.PublishTask{Type: model.PublishTaskTypeManual, SourceSiteID: 1})
	p.CreateTask(ctx, &model.PublishTask{Type: model.PublishTaskTypeAuto, SourceSiteID: 2})

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
	p.CreateCandidate(ctx, candidate)

	got, err := p.GetCandidate(ctx, candidate.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.TorrentName != "Test.Torrent" {
		t.Errorf("expected Test.Torrent, got %s", got.TorrentName)
	}

	p.MarkDownloadCompleted(ctx, candidate.ID, "/data/torrents", "/data/torrents/test.torrent")
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := &model.PublishCandidate{TorrentName: tt.title}
			ok, _ := p.CheckPublishEligibility(candidate, "target")
			if ok != tt.allowed {
				t.Errorf("eligibility(%q) = %v, want %v", tt.title, ok, tt.allowed)
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

	p.CreateCandidate(ctx, &model.PublishCandidate{
		SourceSite: "s1", SourceTorrentID: "t1", InfoHash: "ih1",
		TorrentName: "test", PublishStatus: model.CandidatePending,
	})

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

	p.CreateCandidate(ctx, &model.PublishCandidate{
		SourceSite: "s1", SourceTorrentID: "t1", InfoHash: "ih1",
		TorrentName: "Good Movie", PublishStatus: model.CandidatePending,
	})

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

	p.CreateCandidate(ctx, &model.PublishCandidate{
		SourceSite: "s1", SourceTorrentID: "t1", InfoHash: "ih1",
		TorrentName: "禁转 Movie", PublishStatus: model.CandidatePending,
	})

	_, err := p.PublishCandidate(ctx, 1)
	if err == nil {
		t.Error("expected error for ineligible candidate")
	}
}

func TestPipeline_ProcessPending_Skipped(t *testing.T) {
	p := NewPipeline(setupPipelineTestDB(t), zap.NewNop())
	ctx := context.Background()

	p.CreateCandidate(ctx, &model.PublishCandidate{
		SourceSite: "s1", SourceTorrentID: "t1", InfoHash: "ih1",
		TorrentName: "独占 Content", PublishStatus: model.CandidatePending,
	})

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

	p.CreateCandidate(ctx, &model.PublishCandidate{
		SourceSite: "s1", SourceTorrentID: "t1", InfoHash: "ih1",
		TorrentName: "Normal Movie", PublishStatus: model.CandidatePending,
		DownloadCompleted: true,
	})

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

	p.CreateCandidate(ctx, &model.PublishCandidate{
		SourceSite: "s1", SourceTorrentID: "t1", InfoHash: "ih1",
		TorrentName: "test", PublishStatus: model.CandidatePending,
	})

	p.CreateResult(ctx, &model.PublishResultRecord{
		CandidateID: 1, TargetSite: "target1", Status: "success",
	})

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
			p.CreateCandidate(ctx, &model.PublishCandidate{
				SourceSite: "s1", SourceTorrentID: "t1", InfoHash: "ih1",
				TorrentName: tt.title, PublishStatus: model.CandidatePending,
			})

			_, err := p.PublishCandidate(ctx, 1)
			if err == nil {
				t.Errorf("expected error for title %q", tt.title)
			}

			db := setupPipelineTestDB(t)
			p2 := NewPipeline(db, zap.NewNop())
			p2.CreateCandidate(ctx, &model.PublishCandidate{
				SourceSite: "s1", SourceTorrentID: "t1", InfoHash: "ih1",
				TorrentName: tt.title, PublishStatus: model.CandidatePending,
			})
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
	p.CreateCandidate(ctx, candidate)

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

	p.CreateCandidate(ctx, &model.PublishCandidate{
		SourceSite: "s1", SourceTorrentID: "t1", InfoHash: "ih1",
		TorrentName: "p1", PublishStatus: model.CandidatePending,
	})
	p.CreateCandidate(ctx, &model.PublishCandidate{
		SourceSite: "s2", SourceTorrentID: "t2", InfoHash: "ih2",
		TorrentName: "p2", PublishStatus: model.CandidateDone,
	})
	p.CreateCandidate(ctx, &model.PublishCandidate{
		SourceSite: "s3", SourceTorrentID: "t3", InfoHash: "ih3",
		TorrentName: "p3", PublishStatus: "downloading",
	})

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
		p.CreateTask(ctx, &model.PublishTask{Type: model.PublishTaskTypeAuto, SourceSiteID: uint(i + 1)})
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
	p.CreateTask(ctx, &model.PublishTask{Type: model.PublishTaskTypeManual, SourceSiteID: 1})

	tasks, _, _ := p.ListTasks(ctx, 0, 10)
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
}

func TestPipeline_MarkDownloadCompleted(t *testing.T) {
	p := NewPipeline(setupPipelineTestDB(t), zap.NewNop())
	ctx := context.Background()
	p.CreateCandidate(ctx, &model.PublishCandidate{
		SourceSite: "s1", SourceTorrentID: "t1", InfoHash: "ih1",
		TorrentName: "test", PublishStatus: model.CandidatePending,
	})
	p.MarkDownloadCompleted(ctx, 1, "/data/torrents", "/data/torrents/test.torrent")
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

	p.CreateCandidate(ctx, &model.PublishCandidate{
		SourceSite: "s1", SourceTorrentID: "t1", InfoHash: "ih1",
		TorrentName: "test", PublishStatus: model.CandidatePending,
	})

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
	db.AutoMigrate(
		&model.PublishTask{},
		&model.PublishCandidate{},
		&model.PublishResultRecord{},
		&model.PublishGroup{},
		&model.PublishGroupMember{},
		&model.PublishGroupStatusHistory{},
	)
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

	p.CreateGroup(ctx, 1, "h1", "s1", "t1")
	p.CreateGroup(ctx, 2, "h2", "s2", "t2")
	p.CreateGroup(ctx, 3, "h3", "s3", "t3")

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

	p.AddGroupMember(ctx, group.ID, &model.PublishGroupMember{
		InfoHash: "ih001",
		SiteName: "site-a",
		Status:   model.MemberStatusNew,
	})
	p.AddGroupMember(ctx, group.ID, &model.PublishGroupMember{
		InfoHash: "ih002",
		SiteName: "site-b",
		Status:   model.MemberStatusNew,
	})

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
	p.AddGroupMember(ctx, group.ID, member)

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
	p.AddGroupMember(ctx, group.ID, &model.PublishGroupMember{
		InfoHash: "ih001", SiteName: "site-a", Status: model.MemberStatusUploaded,
	})
	p.AddGroupMember(ctx, group.ID, &model.PublishGroupMember{
		InfoHash: "ih002", SiteName: "site-b", Status: model.MemberStatusSeedingConfirmed,
	})

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
	p.AddGroupMember(ctx, group.ID, &model.PublishGroupMember{
		InfoHash: "ih001", SiteName: "site-a", Status: model.MemberStatusUploaded,
	})
	p.AddGroupMember(ctx, group.ID, &model.PublishGroupMember{
		InfoHash: "ih002", SiteName: "site-b", Status: model.MemberStatusError,
	})

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
	p.AddGroupMember(ctx, group.ID, m1)
	p.AddGroupMember(ctx, group.ID, m2)

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
	p.AddGroupMember(ctx, group.ID, &model.PublishGroupMember{
		InfoHash: "ih001", SiteName: "site-a", Status: model.MemberStatusNew,
	})
	p.AddGroupMember(ctx, group.ID, &model.PublishGroupMember{
		InfoHash: "ih002", SiteName: "site-b", Status: model.MemberStatusUploading,
	})

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
	p.AddGroupMember(ctx, group.ID, &model.PublishGroupMember{
		InfoHash: "ih001", SiteName: "site-a", Status: model.MemberStatusNew,
	})
	p.AddGroupMember(ctx, group.ID, &model.PublishGroupMember{
		InfoHash: "ih002", SiteName: "site-b", Status: model.MemberStatusNew,
	})

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

	p.CreateCandidate(ctx, &model.PublishCandidate{
		SourceSite: "s1", SourceTorrentID: "t1", InfoHash: "ih1",
		TorrentName: "Normal Movie", PublishStatus: model.CandidateCompleted,
		DownloadCompleted: false,
	})

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
