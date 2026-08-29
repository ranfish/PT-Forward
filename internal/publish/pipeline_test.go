package publish

import (
	"context"
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

func TestPipeline_SetSiteProvider(t *testing.T) {
	db := setupPipelineTestDBWithGroups(t)
	p := NewPipeline(db, zap.NewNop())
	p.SetSiteProvider(nil)
	if p.siteProvider != nil {
		t.Error("expected siteProvider to be nil")
	}
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

func TestPipeline_SetCompletionWatcher(t *testing.T) {
	db := setupPipelineTestDB(t)
	p := NewPipeline(db, zap.NewNop())
	p.SetCompletionWatcher(nil)
	if p.completionWatcher != nil {
		t.Error("expected completionWatcher to be nil")
	}
}

func TestPipeline_SetNotifyService(t *testing.T) {
	db := setupPipelineTestDB(t)
	p := NewPipeline(db, zap.NewNop())
	p.SetNotifyService(nil)
	if p.notifyService != nil {
		t.Error("expected notifyService to be nil")
	}
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
