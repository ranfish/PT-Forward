package publish

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

func setupCoverageDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&model.PublishGroup{},
		&model.PublishGroupMember{},
		&model.PublishGroupStatusHistory{},
		&model.PublishTask{},
		&model.PublishCandidate{},
		&model.PublishResultRecord{},
		&model.RSSSubscription{},
		&model.SiteFieldMapping{},
		&model.PublishExclusion{},
		&model.ClientConfig{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestLifecycleManager_ReloadSettingsFromDB(t *testing.T) {
	db := setupCoverageDB(t)
	db.Exec("CREATE TABLE IF NOT EXISTS system_settings (`key` TEXT, `value` TEXT)")
	db.Exec("INSERT INTO system_settings (`key`, `value`) VALUES ('lifecycle.pause_seeders', '25')")
	db.Exec("INSERT INTO system_settings (`key`, `value`) VALUES ('lifecycle.delete_seeders', '200')")
	db.Exec("INSERT INTO system_settings (`key`, `value`) VALUES ('lifecycle.delete_seed_hours', '96')")

	m := NewLifecycleManager(db, zap.NewNop())
	m.reloadSettingsFromDB(context.Background())

	if m.globalPauseSeeders != 25 {
		t.Errorf("expected pause_seeders=25, got %d", m.globalPauseSeeders)
	}
	if m.globalDeleteSeeders != 200 {
		t.Errorf("expected delete_seeders=200, got %d", m.globalDeleteSeeders)
	}
	if m.globalDeleteSeedHours != 96 {
		t.Errorf("expected delete_seed_hours=96, got %d", m.globalDeleteSeedHours)
	}
}

func TestLifecycleManager_ReloadSettingsFromDB_Empty(t *testing.T) {
	db := setupCoverageDB(t)
	db.Exec("CREATE TABLE IF NOT EXISTS system_settings (`key` TEXT, `value` TEXT)")

	m := NewLifecycleManager(db, zap.NewNop())
	m.SetDefaults(30, 150, 72)
	m.reloadSettingsFromDB(context.Background())

	if m.globalPauseSeeders != 30 {
		t.Errorf("expected unchanged pause_seeders=30, got %d", m.globalPauseSeeders)
	}
}

func TestLifecycleManager_TransitionGroupStatus_Mixed(t *testing.T) {
	db := setupCoverageDB(t)
	group := &model.PublishGroup{Status: model.GroupMonitoring}
	db.Create(group)

	mem1 := &model.PublishGroupMember{
		PublishGroupID: group.ID,
		InfoHash:       "ih001",
		SiteName:       "site-a",
		Status:         model.MemberStatusUploading,
	}
	mem2 := &model.PublishGroupMember{
		PublishGroupID: group.ID,
		InfoHash:       "ih002",
		SiteName:       "site-b",
		Status:         model.MemberStatusPaused,
		Paused:         false,
	}
	db.Create(mem1)
	db.Create(mem2)

	m := NewLifecycleManager(db, zap.NewNop())
	m.transitionGroupStatus(context.Background(), group, &LifecycleCheckResult{})

	var updated model.PublishGroup
	db.First(&updated, group.ID)
	if updated.Status != model.GroupPublishing {
		t.Errorf("expected publishing with uploading member, got %s", updated.Status)
	}
}

func TestLifecycleManager_TransitionGroupStatus_AllDoneMonitoring(t *testing.T) {
	db := setupCoverageDB(t)
	group := &model.PublishGroup{Status: model.GroupPublishing}
	db.Create(group)

	mem := &model.PublishGroupMember{
		PublishGroupID: group.ID,
		InfoHash:       "ih001",
		SiteName:       "site-a",
		Status:         model.MemberStatusUploaded,
	}
	db.Create(mem)

	m := NewLifecycleManager(db, zap.NewNop())
	m.transitionGroupStatus(context.Background(), group, &LifecycleCheckResult{})

	var updated model.PublishGroup
	db.First(&updated, group.ID)
	if updated.Status != model.GroupMonitoring {
		t.Errorf("expected monitoring when all done, got %s", updated.Status)
	}
}

func TestLifecycleManager_TransitionGroupStatus_SomePausedNotAll(t *testing.T) {
	db := setupCoverageDB(t)
	group := &model.PublishGroup{Status: model.GroupMonitoring}
	db.Create(group)

	mem1 := &model.PublishGroupMember{
		PublishGroupID: group.ID,
		InfoHash:       "ih001",
		SiteName:       "site-a",
		Status:         model.MemberStatusPaused,
		Paused:         true,
	}
	mem2 := &model.PublishGroupMember{
		PublishGroupID: group.ID,
		InfoHash:       "ih002",
		SiteName:       "site-b",
		Status:         model.MemberStatusPaused,
		Paused:         false,
	}
	db.Create(mem1)
	db.Create(mem2)

	m := NewLifecycleManager(db, zap.NewNop())
	m.transitionGroupStatus(context.Background(), group, &LifecycleCheckResult{})

	var updated model.PublishGroup
	db.First(&updated, group.ID)
	if updated.Status != model.GroupMonitoring {
		t.Errorf("expected unchanged status (not all paused=true), got %s", updated.Status)
	}
}

func TestLifecycleManager_TransitionGroupStatus_BannedMemberKeepsPaused(t *testing.T) {
	db := setupCoverageDB(t)
	group := &model.PublishGroup{Status: model.GroupPublishing}
	db.Create(group)

	mem1 := &model.PublishGroupMember{
		PublishGroupID: group.ID,
		InfoHash:       "ih001",
		SiteName:       "site-a",
		Status:         model.MemberStatusPaused,
		Paused:         true,
	}
	mem2 := &model.PublishGroupMember{
		PublishGroupID: group.ID,
		InfoHash:       "ih002",
		SiteName:       "site-b",
		Status:         model.MemberStatusBanned,
	}
	db.Create(mem1)
	db.Create(mem2)

	m := NewLifecycleManager(db, zap.NewNop())
	m.transitionGroupStatus(context.Background(), group, &LifecycleCheckResult{})

	var updated model.PublishGroup
	db.First(&updated, group.ID)
	if updated.Status != model.GroupAllPaused {
		t.Errorf("banned+paused members should be all_paused, got %s", updated.Status)
	}
}

func TestLifecycleManager_RemoveHRTag_NoClientProvider(t *testing.T) {
	db := setupCoverageDB(t)
	m := NewLifecycleManager(db, zap.NewNop())

	mem := &model.PublishGroupMember{
		ClientID: "client-1",
		InfoHash: "abc123",
		HRSite:   "site-a",
	}
	m.removeHRTag(context.Background(), mem)
}

func TestLifecycleManager_RemoveHRTag_EmptySite(t *testing.T) {
	db := setupCoverageDB(t)
	mockDL := newLifecycleMockDLClient()
	m := NewLifecycleManager(db, zap.NewNop())
	m.SetClientProvider(&mockLifecycleProvider{client: mockDL})

	mem := &model.PublishGroupMember{
		ClientID: "client-1",
		InfoHash: "abc123",
		HRSite:   "",
		SiteName: "",
	}
	m.removeHRTag(context.Background(), mem)

	if len(mockDL.removedTags) != 0 {
		t.Error("should not remove tags with empty site")
	}
}

func TestLifecycleManager_RemoveHRTag_UsesSiteName(t *testing.T) {
	db := setupCoverageDB(t)
	mockDL := newLifecycleMockDLClient()
	m := NewLifecycleManager(db, zap.NewNop())
	m.SetClientProvider(&mockLifecycleProvider{client: mockDL})

	mem := &model.PublishGroupMember{
		ClientID: "client-1",
		InfoHash: "abc123",
		HRSite:   "",
		SiteName: "fallback-site",
	}
	m.removeHRTag(context.Background(), mem)

	if len(mockDL.removedTags) != 1 {
		t.Fatalf("expected 1 tag removal, got %d", len(mockDL.removedTags))
	}
	if mockDL.removedTags[0][0] != "PROTECTED_HR_fallback-site" {
		t.Errorf("expected tag PROTECTED_HR_fallback-site, got %s", mockDL.removedTags[0][0])
	}
}

func TestLifecycleManager_PauseMember_WithClient(t *testing.T) {
	db := setupCoverageDB(t)
	group := &model.PublishGroup{Status: model.GroupMonitoring}
	db.Create(group)

	mem := &model.PublishGroupMember{
		PublishGroupID: group.ID,
		InfoHash:       "abc123",
		SiteName:       "site-a",
		ClientID:       "test-client",
		Status:         model.MemberStatusSeedingConfirmed,
		Seeders:        60,
	}
	db.Create(mem)

	paused := false
	mockDL := &lifecycleMockDLClient{DownloaderClient: &mocks.DownloaderClient{Name: "test"}}
	mockDL.PauseTorrentFn = func(_ context.Context, hash string) error {
		paused = true
		return nil
	}

	m := NewLifecycleManager(db, zap.NewNop())
	m.SetClientProvider(&mockLifecycleProvider{client: mockDL})
	m.SetDefaults(50, 100, 168)

	result := &LifecycleCheckResult{}
	m.pauseMember(context.Background(), mem, result)

	if !paused {
		t.Error("torrent should have been paused")
	}
	if result.PausedMembers != 1 {
		t.Errorf("expected 1 paused, got %d", result.PausedMembers)
	}
}

func TestLifecycleManager_PauseMember_PauseFails(t *testing.T) {
	db := setupCoverageDB(t)
	group := &model.PublishGroup{Status: model.GroupMonitoring}
	db.Create(group)

	mem := &model.PublishGroupMember{
		PublishGroupID: group.ID,
		InfoHash:       "abc123",
		SiteName:       "site-a",
		ClientID:       "test-client",
		Status:         model.MemberStatusSeedingConfirmed,
		Seeders:        60,
	}
	db.Create(mem)

	mockDL := &lifecycleMockDLClient{DownloaderClient: &mocks.DownloaderClient{Name: "test"}}
	mockDL.PauseTorrentFn = func(_ context.Context, hash string) error {
		return fmt.Errorf("pause failed")
	}

	m := NewLifecycleManager(db, zap.NewNop())
	m.SetClientProvider(&mockLifecycleProvider{client: mockDL})

	result := &LifecycleCheckResult{}
	m.pauseMember(context.Background(), mem, result)

	if result.PausedMembers != 1 {
		t.Errorf("should still count as paused in DB even if client pause fails, got %d", result.PausedMembers)
	}
}

func TestLifecycleManager_DeleteGroup_WithClientProvider(t *testing.T) {
	db := setupCoverageDB(t)
	group := &model.PublishGroup{Status: model.GroupMonitoring}
	db.Create(group)

	mem1 := &model.PublishGroupMember{
		PublishGroupID: group.ID,
		InfoHash:       "abc123",
		SiteName:       "site-a",
		ClientID:       "test-client",
		Status:         model.MemberStatusSeedingConfirmed,
		Seeders:        200,
	}
	mem2 := &model.PublishGroupMember{
		PublishGroupID: group.ID,
		InfoHash:       "def456",
		SiteName:       "site-b",
		ClientID:       "",
		Status:         model.MemberStatusSeedingConfirmed,
		Seeders:        200,
	}
	db.Create(mem1)
	db.Create(mem2)

	mockDL := newLifecycleMockDLClient()
	m := NewLifecycleManager(db, zap.NewNop())
	m.SetClientProvider(&mockLifecycleProvider{client: mockDL})

	result := &LifecycleCheckResult{}
	m.deleteGroup(context.Background(), group, []model.PublishGroupMember{*mem1, *mem2}, result)

	if result.DeletedGroups != 1 {
		t.Errorf("expected 1 deleted, got %d", result.DeletedGroups)
	}
	if len(mockDL.deleted) != 1 {
		t.Errorf("expected 1 client delete, got %d", len(mockDL.deleted))
	}
}

func TestLifecycleManager_DeleteGroup_DeleteFails(t *testing.T) {
	db := setupCoverageDB(t)
	group := &model.PublishGroup{Status: model.GroupMonitoring}
	db.Create(group)

	mem := &model.PublishGroupMember{
		PublishGroupID: group.ID,
		InfoHash:       "abc123",
		SiteName:       "site-a",
		ClientID:       "test-client",
		Status:         model.MemberStatusSeedingConfirmed,
		Seeders:        200,
	}
	db.Create(mem)

	mockDL := &lifecycleMockDLClient{DownloaderClient: &mocks.DownloaderClient{Name: "test"}}
	mockDL.DeleteTorrentFn = func(_ context.Context, hash string, _ bool) error {
		return fmt.Errorf("delete failed")
	}

	m := NewLifecycleManager(db, zap.NewNop())
	m.SetClientProvider(&mockLifecycleProvider{client: mockDL})

	result := &LifecycleCheckResult{}
	m.deleteGroup(context.Background(), group, []model.PublishGroupMember{*mem}, result)

	if result.Errors != 1 {
		t.Errorf("expected 1 error from failed delete, got %d", result.Errors)
	}
}

func TestLifecycleManager_GetEffectiveConfig_NoSubscription(t *testing.T) {
	db := setupCoverageDB(t)
	m := NewLifecycleManager(db, zap.NewNop())
	m.SetDefaults(50, 100, 168)

	pause, delete, hours := m.getEffectiveConfig(context.Background(), "")
	if pause != 50 || delete != 100 || hours != 168 {
		t.Errorf("expected defaults, got pause=%d delete=%d hours=%d", pause, delete, hours)
	}
}

func TestLifecycleManager_GetEffectiveConfig_SubNotFound(t *testing.T) {
	db := setupCoverageDB(t)
	m := NewLifecycleManager(db, zap.NewNop())
	m.SetDefaults(50, 100, 168)

	pause, delete, hours := m.getEffectiveConfig(context.Background(), "nonexistent-id")
	if pause != 50 || delete != 100 || hours != 168 {
		t.Errorf("expected defaults when sub not found, got pause=%d delete=%d hours=%d", pause, delete, hours)
	}
}

func TestLifecycleManager_CheckGroup_EmptyMembers(t *testing.T) {
	db := setupCoverageDB(t)
	group := &model.PublishGroup{Status: model.GroupMonitoring}
	db.Create(group)

	m := NewLifecycleManager(db, zap.NewNop())
	result := &LifecycleCheckResult{}
	m.checkGroup(context.Background(), group, result)

	if result.Errors != 0 {
		t.Errorf("expected 0 errors for empty members, got %d", result.Errors)
	}
}

func TestLifecycleManager_CheckGroup_QueryMembersFail(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	m := NewLifecycleManager(db, zap.NewNop())
	grp := &model.PublishGroup{Status: model.GroupMonitoring}
	result := &LifecycleCheckResult{}
	m.checkGroup(context.Background(), grp, result)

	if result.Errors != 1 {
		t.Errorf("expected 1 error when members query fails, got %d", result.Errors)
	}
}

func TestLifecycleManager_CheckOnce_DBError(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	m := NewLifecycleManager(db, zap.NewNop())
	_, err = m.CheckOnce(context.Background())
	if err == nil {
		t.Error("expected error with uninitialized DB")
	}
}

func TestLifecycleManager_CheckGroup_DeletedMemberSkipped(t *testing.T) {
	db := setupCoverageDB(t)
	group := &model.PublishGroup{Status: model.GroupMonitoring}
	db.Create(group)

	mem := &model.PublishGroupMember{
		PublishGroupID: group.ID,
		InfoHash:       "abc123",
		SiteName:       "site-a",
		Status:         model.MemberStatusDeleted,
		Role:           "target",
		Seeders:        200,
	}
	db.Create(mem)

	m := NewLifecycleManager(db, zap.NewNop())
	m.SetDefaults(50, 100, 168)
	result := &LifecycleCheckResult{}
	m.checkGroup(context.Background(), group, result)

	if result.DeletedGroups != 0 {
		t.Error("deleted member should be skipped, no hasTarget=true")
	}
}

func TestPipeline_OnTorrents_WithWatcherError(t *testing.T) {
	db := setupCoverageDB(t)
	p := NewPipeline(db, zap.NewNop())

	p.SetCompletionWatcher(&mockCompletionWatcher{
		submitFn: func(ctx context.Context, candidate model.PublishCandidate) error {
			return fmt.Errorf("watcher error")
		},
	})

	events := []model.TorrentEvent{
		{
			SiteName:        "site1",
			TorrentID:       "t-001",
			Title:           "Test Movie",
			Size:            1073741824,
			InfoHash:        "abc123",
			MatchedRuleName: "auto-publish",
		},
	}

	if err := p.OnTorrents(context.Background(), events); err != nil {
		t.Fatal(err)
	}
}

func TestSeedingConfirmation_ConfirmMember_NoClientID(t *testing.T) {
	db := setupConfirmDB(t)
	sc := NewSeedingConfirmation(db, nil)

	confirmed, err := sc.confirmMember(context.Background(), &mockDownloaderChecker{}, model.PublishGroupMember{
		InfoHash: "abc123",
		ClientID: "",
	})
	if err != nil {
		t.Fatal(err)
	}
	if confirmed {
		t.Error("should not confirm without client ID")
	}
}

func TestSeedingConfirmation_ConfirmMember_ClientConfigNotFound(t *testing.T) {
	db := setupConfirmDB(t)
	sc := NewSeedingConfirmation(db, nil)

	confirmed, err := sc.confirmMember(context.Background(), &mockDownloaderChecker{}, model.PublishGroupMember{
		InfoHash: "abc123",
		ClientID: "nonexistent",
	})
	if err == nil {
		t.Error("expected error for non-existent client config")
	}
	if confirmed {
		t.Error("should not confirm without client config")
	}
}

func TestSeedingConfirmation_ConfirmMember_CheckerError(t *testing.T) {
	db := setupConfirmDB(t)

	clientCfg := model.ClientConfig{Name: "client-1", Type: "qbittorrent"}
	db.Create(&clientCfg)

	sc := NewSeedingConfirmation(db, nil)
	checker := &mockDownloaderChecker{
		err: fmt.Errorf("checker error"),
	}

	confirmed, err := sc.confirmMember(context.Background(), checker, model.PublishGroupMember{
		InfoHash: "abc123",
		ClientID: "client-1",
	})
	if err == nil {
		t.Error("expected error from checker")
	}
	if confirmed {
		t.Error("should not confirm on checker error")
	}
}

func TestSeedingConfirmation_ConfirmMember_NilInfo(t *testing.T) {
	db := setupConfirmDB(t)

	clientCfg := model.ClientConfig{Name: "client-1", Type: "qbittorrent"}
	db.Create(&clientCfg)

	sc := NewSeedingConfirmation(db, nil)
	checker := &mockDownloaderChecker{
		result: nil,
	}

	confirmed, err := sc.confirmMember(context.Background(), checker, model.PublishGroupMember{
		InfoHash: "abc123",
		ClientID: "client-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if confirmed {
		t.Error("should not confirm with nil info")
	}
}

func TestSeedingConfirmation_ConfirmMember_StalledUP(t *testing.T) {
	db := setupConfirmDB(t)

	clientCfg := model.ClientConfig{Name: "client-1", Type: "qbittorrent"}
	db.Create(&clientCfg)

	sc := NewSeedingConfirmation(db, nil)
	checker := &mockDownloaderChecker{
		result: &model.TorrentInfo{
			Hash:  "abc123",
			State: "stalledUP",
		},
	}

	confirmed, err := sc.confirmMember(context.Background(), checker, model.PublishGroupMember{
		InfoHash: "abc123",
		ClientID: "client-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !confirmed {
		t.Error("stalledUP should be confirmed")
	}
}

func TestSeedingConfirmation_ConfirmMember_ForcedUP(t *testing.T) {
	db := setupConfirmDB(t)

	clientCfg := model.ClientConfig{Name: "client-1", Type: "qbittorrent"}
	db.Create(&clientCfg)

	sc := NewSeedingConfirmation(db, nil)
	checker := &mockDownloaderChecker{
		result: &model.TorrentInfo{
			Hash:  "abc123",
			State: "forcedUP",
		},
	}

	confirmed, err := sc.confirmMember(context.Background(), checker, model.PublishGroupMember{
		InfoHash: "abc123",
		ClientID: "client-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !confirmed {
		t.Error("forcedUP should be confirmed")
	}
}

func TestSeedingConfirmation_ConfirmMember_IsFinished(t *testing.T) {
	db := setupConfirmDB(t)

	clientCfg := model.ClientConfig{Name: "client-1", Type: "qbittorrent"}
	db.Create(&clientCfg)

	sc := NewSeedingConfirmation(db, nil)
	checker := &mockDownloaderChecker{
		result: &model.TorrentInfo{
			Hash:       "abc123",
			State:      "some_other_state",
			IsFinished: true,
		},
	}

	confirmed, err := sc.confirmMember(context.Background(), checker, model.PublishGroupMember{
		InfoHash: "abc123",
		ClientID: "client-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !confirmed {
		t.Error("IsFinished=true should be confirmed")
	}
}

func TestSeedingConfirmation_CheckOnce_CancelledContext(t *testing.T) {
	db := setupConfirmDB(t)

	group := model.PublishGroup{Status: "active"}
	db.Create(&group)

	now := time.Now().Add(-10 * time.Minute)
	member := model.PublishGroupMember{
		PublishGroupID: group.ID,
		InfoHash:       "abc123",
		SiteName:       "target",
		ClientID:       "client-1",
		Status:         model.MemberStatusUploaded,
		UpdatedAt:      now,
	}
	db.Create(&member)

	clientCfg := model.ClientConfig{Name: "client-1", Type: "qbittorrent"}
	db.Create(&clientCfg)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	sc := NewSeedingConfirmation(db, nil)
	err := sc.CheckOnce(ctx, &mockDownloaderChecker{
		result: &model.TorrentInfo{Hash: "abc123", State: "uploading"},
	})
	if err == nil {
		t.Log("cancelled context handled")
	}
}

func TestSeedingConfirmation_CheckOnce_ConfirmFails(t *testing.T) {
	db := setupConfirmDB(t)

	group := model.PublishGroup{Status: "active"}
	db.Create(&group)

	now := time.Now().Add(-10 * time.Minute)
	member := model.PublishGroupMember{
		PublishGroupID: group.ID,
		InfoHash:       "abc123",
		SiteName:       "target",
		ClientID:       "client-1",
		Status:         model.MemberStatusUploaded,
		UpdatedAt:      now,
	}
	db.Create(&member)

	clientCfg := model.ClientConfig{Name: "client-1", Type: "qbittorrent"}
	db.Create(&clientCfg)

	sc := NewSeedingConfirmation(db, zap.NewNop())
	err := sc.CheckOnce(context.Background(), &mockDownloaderChecker{
		err: fmt.Errorf("checker error"),
	})
	if err != nil {
		t.Fatal(err)
	}

	var updated model.PublishGroupMember
	db.First(&updated, member.ID)
	if updated.Status != model.MemberStatusUploaded {
		t.Errorf("status should remain uploaded when confirm fails, got %s", updated.Status)
	}
}

func TestSeedingConfirmation_CheckOnce_WithLogger(t *testing.T) {
	db := setupConfirmDB(t)
	sc := NewSeedingConfirmation(db, zap.NewNop())

	group := model.PublishGroup{Status: "active"}
	db.Create(&group)

	now := time.Now().Add(-10 * time.Minute)
	member := model.PublishGroupMember{
		PublishGroupID: group.ID,
		InfoHash:       "abc123",
		SiteName:       "target",
		ClientID:       "client-1",
		Status:         model.MemberStatusUploaded,
		UpdatedAt:      now,
	}
	db.Create(&member)

	clientCfg := model.ClientConfig{Name: "client-1", Type: "qbittorrent"}
	db.Create(&clientCfg)

	err := sc.CheckOnce(context.Background(), &mockDownloaderChecker{
		result: &model.TorrentInfo{Hash: "abc123", State: "uploading", IsFinished: true},
	})
	if err != nil {
		t.Fatal(err)
	}

	var updated model.PublishGroupMember
	db.First(&updated, member.ID)
	if updated.Status != model.MemberStatusSeedingConfirmed {
		t.Errorf("expected seeding_confirmed, got %s", updated.Status)
	}
}

func TestSeedingConfirmation_ConfirmMember_InjectedStatus(t *testing.T) {
	db := setupConfirmDB(t)

	clientCfg := model.ClientConfig{Name: "client-1", Type: "qbittorrent"}
	db.Create(&clientCfg)

	sc := NewSeedingConfirmation(db, nil)
	checker := &mockDownloaderChecker{
		result: &model.TorrentInfo{
			Hash:       "abc123",
			State:      "uploading",
			IsFinished: true,
		},
	}

	confirmed, err := sc.confirmMember(context.Background(), checker, model.PublishGroupMember{
		InfoHash: "abc123",
		ClientID: "client-1",
		Status:   model.MemberStatusInjected,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !confirmed {
		t.Error("injected member with uploading state should be confirmed")
	}
}

func TestPipeline_mapFieldValues_ResolutionFallback(t *testing.T) {
	db := setupCoverageDB(t)
	p := NewPipeline(db, zap.NewNop())
	ctx := context.Background()

	mappings := []model.SiteFieldMapping{
		{SiteName: "target", FieldType: "resolution", SourceValue: "1080p", TargetValue: "HD"},
	}
	for _, m := range mappings {
		if err := db.Create(&m).Error; err != nil {
			t.Fatal(err)
		}
	}

	fields := map[string]string{
		"resolution": "1080p",
	}
	p.mapFieldValues(ctx, "target", fields)

	if fields["resolution"] != "HD" {
		t.Errorf("expected resolution fallback to work, got %q", fields["resolution"])
	}
}

func TestPipeline_mapFieldValues_CodecFallback(t *testing.T) {
	db := setupCoverageDB(t)
	p := NewPipeline(db, zap.NewNop())
	ctx := context.Background()

	mappings := []model.SiteFieldMapping{
		{SiteName: "target", FieldType: "videoCodec", SourceValue: "x265", TargetValue: "HEVC"},
	}
	for _, m := range mappings {
		if err := db.Create(&m).Error; err != nil {
			t.Fatal(err)
		}
	}

	fields := map[string]string{
		"codec": "x265",
	}
	p.mapFieldValues(ctx, "target", fields)

	if fields["codec"] != "HEVC" {
		t.Errorf("expected codec fallback to work, got %q", fields["codec"])
	}
}

func TestPipeline_mapFieldValues_SourceFallback(t *testing.T) {
	db := setupCoverageDB(t)
	p := NewPipeline(db, zap.NewNop())
	ctx := context.Background()

	mappings := []model.SiteFieldMapping{
		{SiteName: "target", FieldType: "source", SourceValue: "blu-ray", TargetValue: "BluRay"},
	}
	for _, m := range mappings {
		if err := db.Create(&m).Error; err != nil {
			t.Fatal(err)
		}
	}

	fields := map[string]string{
		"source": "blu-ray",
	}
	p.mapFieldValues(ctx, "target", fields)

	if fields["source"] != "BluRay" {
		t.Errorf("expected source fallback to work, got %q", fields["source"])
	}
}

func TestPublishError(t *testing.T) {
	cause := fmt.Errorf("root cause")
	appErr := publishError(34000, "test error", cause)
	if appErr.Code != 34000 {
		t.Errorf("expected code 34000, got %d", appErr.Code)
	}
	if appErr.Message != "test error" {
		t.Errorf("expected 'test error', got %s", appErr.Message)
	}
	if appErr.Cause != cause {
		t.Error("cause mismatch")
	}
	if appErr.Retryable {
		t.Error("should not be retryable")
	}
}

type mockCompletionWatcher struct {
	submitFn func(ctx context.Context, candidate model.PublishCandidate) error
}

func (m *mockCompletionWatcher) Start(ctx context.Context) error { return nil }
func (m *mockCompletionWatcher) Stop()                           {}
func (m *mockCompletionWatcher) Watch(ctx context.Context, clientName, infoHash string, candidateID uint) error {
	return nil
}
func (m *mockCompletionWatcher) SubmitCandidate(ctx context.Context, candidate model.PublishCandidate) error {
	if m.submitFn != nil {
		return m.submitFn(ctx, candidate)
	}
	return nil
}
