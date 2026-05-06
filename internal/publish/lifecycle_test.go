package publish

import (
	"context"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/mocks"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupLifecycleDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&model.PublishGroup{},
		&model.PublishGroupMember{},
		&model.RSSSubscription{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func createTestGroup(t *testing.T, db *gorm.DB, status model.PublishGroupStatus, seedStart *time.Time) *model.PublishGroup {
	t.Helper()
	group := &model.PublishGroup{
		Status:        status,
		SeedStartTime: seedStart,
	}
	if err := db.Create(group).Error; err != nil {
		t.Fatalf("create group: %v", err)
	}
	return group
}

func createTestMember(t *testing.T, db *gorm.DB, groupID uint, role string, seeders int, status model.MemberStatus, paused bool) *model.PublishGroupMember {
	t.Helper()
	siteName := "target-site"
	if role == "source" {
		siteName = "source-site"
	}
	mem := &model.PublishGroupMember{
		PublishGroupID: groupID,
		Role:           role,
		SiteName:       siteName,
		ClientID:       "test-client",
		InfoHash:       "abc123",
		Status:         status,
		Seeders:        seeders,
		Paused:         paused,
	}
	if err := db.Create(mem).Error; err != nil {
		t.Fatalf("create member: %v", err)
	}
	return mem
}

func TestLifecycleManager_NoGroups(t *testing.T) {
	db := setupLifecycleDB(t)
	m := NewLifecycleManager(db, zap.NewNop())
	result, err := m.CheckOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.CheckedGroups != 0 {
		t.Errorf("expected 0 groups, got %d", result.CheckedGroups)
	}
}

func TestLifecycleManager_PauseMember_HighSeeders(t *testing.T) {
	db := setupLifecycleDB(t)
	group := createTestGroup(t, db, model.GroupMonitoring, nil)
	createTestMember(t, db, group.ID, "target", 60, model.MemberStatusSeedingConfirmed, false)

	m := NewLifecycleManager(db, zap.NewNop())
	m.SetDefaults(50, 100, 168)

	result, err := m.CheckOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.PausedMembers != 1 {
		t.Errorf("expected 1 paused member, got %d", result.PausedMembers)
	}

	var mem model.PublishGroupMember
	db.First(&mem, "publish_group_id = ?", group.ID)
	if !mem.Paused {
		t.Error("member should be paused")
	}
	if mem.Status != model.MemberStatusPaused {
		t.Errorf("expected status paused, got %s", mem.Status)
	}
}

func TestLifecycleManager_DeleteGroup_AllAboveDeleteSeeders(t *testing.T) {
	db := setupLifecycleDB(t)
	group := createTestGroup(t, db, model.GroupMonitoring, nil)
	createTestMember(t, db, group.ID, "source", 0, model.MemberStatusSeedingConfirmed, false)
	createTestMember(t, db, group.ID, "target", 120, model.MemberStatusSeedingConfirmed, false)

	m := NewLifecycleManager(db, zap.NewNop())
	m.SetDefaults(50, 100, 168)

	result, err := m.CheckOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.DeletedGroups != 1 {
		t.Errorf("expected 1 deleted group, got %d", result.DeletedGroups)
	}

	var updated model.PublishGroup
	db.First(&updated, group.ID)
	if updated.Status != model.GroupDeleted {
		t.Errorf("expected group deleted, got %s", updated.Status)
	}
}

func TestLifecycleManager_DeleteGroup_BySeedHours(t *testing.T) {
	db := setupLifecycleDB(t)
	seedStart := time.Now().Add(-200 * time.Hour)
	group := createTestGroup(t, db, model.GroupMonitoring, &seedStart)
	createTestMember(t, db, group.ID, "target", 10, model.MemberStatusSeedingConfirmed, false)

	m := NewLifecycleManager(db, zap.NewNop())
	m.SetDefaults(50, 100, 168)

	result, err := m.CheckOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.DeletedGroups != 1 {
		t.Errorf("expected 1 deleted group (by hours), got %d", result.DeletedGroups)
	}
}

func TestLifecycleManager_HRProtection(t *testing.T) {
	db := setupLifecycleDB(t)
	group := createTestGroup(t, db, model.GroupMonitoring, nil)
	seedStart := time.Now().Add(-1 * time.Hour)
	mem := createTestMember(t, db, group.ID, "target", 120, model.MemberStatusSeedingConfirmed, false)
	mem.HRProtected = true
	mem.HRMinSeedHours = 72
	mem.HRSeedStart = &seedStart
	db.Save(mem)

	m := NewLifecycleManager(db, zap.NewNop())
	m.SetDefaults(50, 100, 168)

	result, err := m.CheckOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.DeletedGroups != 0 {
		t.Errorf("HR protected group should not be deleted, got %d", result.DeletedGroups)
	}
}

func TestLifecycleManager_HRReleased(t *testing.T) {
	db := setupLifecycleDB(t)
	group := createTestGroup(t, db, model.GroupMonitoring, nil)
	seedStart := time.Now().Add(-100 * time.Hour)
	mem := createTestMember(t, db, group.ID, "target", 120, model.MemberStatusSeedingConfirmed, false)
	mem.HRProtected = true
	mem.HRMinSeedHours = 72
	mem.HRSeedStart = &seedStart
	db.Save(mem)

	m := NewLifecycleManager(db, zap.NewNop())
	m.SetDefaults(50, 100, 168)

	result, err := m.CheckOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.DeletedGroups != 1 {
		t.Errorf("HR released group should be deleted, got %d", result.DeletedGroups)
	}
}

func TestLifecycleManager_SubscriptionOverride(t *testing.T) {
	db := setupLifecycleDB(t)
	sub := &model.RSSSubscription{
		Name:                  "test-sub",
		Enabled:               true,
		SiteName:              "testsite",
		ClientID:              "test-client",
		URLs:                  []string{"http://example.com/rss"},
		LifecyclePauseSeeders: 20,
	}
	if err := db.Create(sub).Error; err != nil {
		t.Fatalf("create sub: %v", err)
	}

	group := &model.PublishGroup{
		Status:         model.GroupMonitoring,
		SubscriptionID: "1",
	}
	db.Create(group)
	createTestMember(t, db, group.ID, "target", 25, model.MemberStatusSeedingConfirmed, false)

	m := NewLifecycleManager(db, zap.NewNop())
	m.SetDefaults(50, 100, 168)

	result, err := m.CheckOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.PausedMembers != 1 {
		t.Errorf("subscription override (pause=20) should trigger pause at seeders=25, got paused=%d", result.PausedMembers)
	}
}

func TestLifecycleManager_SkipSourceMembers(t *testing.T) {
	db := setupLifecycleDB(t)
	group := createTestGroup(t, db, model.GroupMonitoring, nil)
	createTestMember(t, db, group.ID, "source", 200, model.MemberStatusSeedingConfirmed, false)

	m := NewLifecycleManager(db, zap.NewNop())
	m.SetDefaults(50, 100, 168)

	result, err := m.CheckOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.PausedMembers != 0 {
		t.Errorf("source member should be skipped, got paused=%d", result.PausedMembers)
	}
	if result.DeletedGroups != 0 {
		t.Errorf("source-only group should not be deleted, got deleted=%d", result.DeletedGroups)
	}
}

func TestLifecycleManager_GroupStatusTransition(t *testing.T) {
	db := setupLifecycleDB(t)
	group := createTestGroup(t, db, model.GroupMonitoring, nil)
	createTestMember(t, db, group.ID, "target", 10, model.MemberStatusPaused, true)

	m := NewLifecycleManager(db, zap.NewNop())
	m.SetDefaults(50, 100, 168)

	_, err := m.CheckOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	var updated model.PublishGroup
	db.First(&updated, group.ID)
	if updated.Status != model.GroupAllPaused {
		t.Errorf("expected all_paused, got %s", updated.Status)
	}
}

func TestLifecycleManager_SetDefaults(t *testing.T) {
	m := NewLifecycleManager(nil, zap.NewNop())
	m.SetDefaults(0, 0, 0)
	if m.globalPauseSeeders != 50 {
		t.Error("zero should not override default")
	}
	m.SetDefaults(30, 200, 48)
	if m.globalPauseSeeders != 30 {
		t.Errorf("expected 30, got %d", m.globalPauseSeeders)
	}
}

type lifecycleMockDLClient struct {
	*mocks.DownloaderClient
	removedTags [][]string
	deleted     []string
}

func newLifecycleMockDLClient() *lifecycleMockDLClient {
	m := &lifecycleMockDLClient{DownloaderClient: &mocks.DownloaderClient{Name: "test"}}
	m.DeleteTorrentFn = func(_ context.Context, hash string, _ bool) error {
		m.deleted = append(m.deleted, hash)
		return nil
	}
	m.RemoveTorrentTagsFn = func(_ context.Context, _ string, tags []string) error {
		m.removedTags = append(m.removedTags, tags)
		return nil
	}
	return m
}

type mockLifecycleProvider struct {
	client *lifecycleMockDLClient
}

func (m *mockLifecycleProvider) Get(clientID string) (model.DownloaderClient, error) {
	return m.client, nil
}
func (m *mockLifecycleProvider) ListClients() []string { return nil }

func TestLifecycleManager_HRReleased_RemovesTag(t *testing.T) {
	db := setupLifecycleDB(t)
	group := createTestGroup(t, db, model.GroupMonitoring, nil)
	seedStart := time.Now().Add(-100 * time.Hour)
	mem := createTestMember(t, db, group.ID, "target", 120, model.MemberStatusSeedingConfirmed, false)
	mem.HRProtected = true
	mem.HRMinSeedHours = 72
	mem.HRSeedStart = &seedStart
	mem.HRSite = "hr-site"
	db.Save(mem)

	mockDL := newLifecycleMockDLClient()
	m := NewLifecycleManager(db, zap.NewNop())
	m.SetClientProvider(&mockLifecycleProvider{client: mockDL})
	m.SetDefaults(50, 100, 168)

	result, err := m.CheckOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.DeletedGroups != 1 {
		t.Errorf("HR released group should be deleted, got %d", result.DeletedGroups)
	}
	if len(mockDL.removedTags) != 1 {
		t.Errorf("expected 1 RemoveTorrentTags call, got %d", len(mockDL.removedTags))
	} else if mockDL.removedTags[0][0] != "PROTECTED_HR_hr-site" {
		t.Errorf("expected tag PROTECTED_HR_hr-site, got %s", mockDL.removedTags[0][0])
	}
}

func TestLifecycleManager_HRProtected_NotReleased(t *testing.T) {
	db := setupLifecycleDB(t)
	group := createTestGroup(t, db, model.GroupMonitoring, nil)
	seedStart := time.Now().Add(-1 * time.Hour)
	mem := createTestMember(t, db, group.ID, "target", 120, model.MemberStatusSeedingConfirmed, false)
	mem.HRProtected = true
	mem.HRMinSeedHours = 72
	mem.HRSeedStart = &seedStart
	mem.HRSite = "hr-site"
	db.Save(mem)

	mockDL := newLifecycleMockDLClient()
	m := NewLifecycleManager(db, zap.NewNop())
	m.SetClientProvider(&mockLifecycleProvider{client: mockDL})
	m.SetDefaults(50, 100, 168)

	result, err := m.CheckOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.DeletedGroups != 0 {
		t.Errorf("HR protected group should not be deleted, got %d", result.DeletedGroups)
	}
	if len(mockDL.removedTags) != 0 {
		t.Errorf("should not remove tags when HR not released, got %d calls", len(mockDL.removedTags))
	}
}

func TestLifecycleManager_DeleteGroup_RemovesHRTag(t *testing.T) {
	db := setupLifecycleDB(t)
	group := createTestGroup(t, db, model.GroupMonitoring, nil)
	mem := createTestMember(t, db, group.ID, "target", 200, model.MemberStatusSeedingConfirmed, false)
	mem.HRProtected = true
	mem.HRMinSeedHours = 72
	mem.HRSite = "hr-site"
	db.Save(mem)

	mockDL := newLifecycleMockDLClient()
	m := NewLifecycleManager(db, zap.NewNop())
	m.SetClientProvider(&mockLifecycleProvider{client: mockDL})
	m.SetDefaults(50, 100, 168)

	result, err := m.CheckOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.DeletedGroups != 1 {
		t.Errorf("expected 1 deleted group, got %d", result.DeletedGroups)
	}
	if len(mockDL.removedTags) != 1 {
		t.Errorf("expected 1 RemoveTorrentTags call when deleting HR group, got %d", len(mockDL.removedTags))
	}
}
