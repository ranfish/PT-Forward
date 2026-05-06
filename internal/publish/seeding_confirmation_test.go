package publish

import (
	"context"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupConfirmDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.AutoMigrate(&model.PublishGroup{}, &model.PublishGroupMember{}, &model.ClientConfig{})
	return db
}

type mockDownloaderChecker struct {
	result *model.TorrentInfo
	err    error
}

func (m *mockDownloaderChecker) GetTorrentInfo(ctx context.Context, clientID uint, infoHash string) (*model.TorrentInfo, error) {
	return m.result, m.err
}

func TestSeedingConfirmation_NoMembers(t *testing.T) {
	db := setupConfirmDB(t)
	sc := NewSeedingConfirmation(db, nil)
	checker := &mockDownloaderChecker{}

	err := sc.CheckOnce(context.Background(), checker)
	assert.NoError(t, err)
}

func TestSeedingConfirmation_ConfirmUploaded(t *testing.T) {
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

	checker := &mockDownloaderChecker{
		result: &model.TorrentInfo{
			Hash:       "abc123",
			State:      "uploading",
			IsFinished: true,
		},
	}

	sc := NewSeedingConfirmation(db, nil)
	err := sc.CheckOnce(context.Background(), checker)
	assert.NoError(t, err)

	var updated model.PublishGroupMember
	db.First(&updated, member.ID)
	assert.Equal(t, model.MemberStatusSeedingConfirmed, updated.Status)
}

func TestSeedingConfirmation_NotYetSeeding(t *testing.T) {
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

	checker := &mockDownloaderChecker{
		result: &model.TorrentInfo{
			Hash:       "abc123",
			State:      "downloading",
			IsFinished: false,
		},
	}

	sc := NewSeedingConfirmation(db, nil)
	err := sc.CheckOnce(context.Background(), checker)
	assert.NoError(t, err)

	var updated model.PublishGroupMember
	db.First(&updated, member.ID)
	assert.Equal(t, model.MemberStatusUploaded, updated.Status)
}

func TestSeedingConfirmation_SkipNoInfoHash(t *testing.T) {
	db := setupConfirmDB(t)

	group := model.PublishGroup{Status: "active"}
	db.Create(&group)

	now := time.Now().Add(-10 * time.Minute)
	member := model.PublishGroupMember{
		PublishGroupID: group.ID,
		InfoHash:       "",
		SiteName:       "target",
		Status:         model.MemberStatusUploaded,
		UpdatedAt:      now,
	}
	db.Create(&member)

	checker := &mockDownloaderChecker{}
	sc := NewSeedingConfirmation(db, nil)
	err := sc.CheckOnce(context.Background(), checker)
	assert.NoError(t, err)

	var updated model.PublishGroupMember
	db.First(&updated, member.ID)
	assert.Equal(t, model.MemberStatusUploaded, updated.Status)
}

func TestSeedingConfirmation_SkipRecentMembers(t *testing.T) {
	db := setupConfirmDB(t)

	group := model.PublishGroup{Status: "active"}
	db.Create(&group)

	member := model.PublishGroupMember{
		PublishGroupID: group.ID,
		InfoHash:       "abc123",
		SiteName:       "target",
		Status:         model.MemberStatusUploaded,
		UpdatedAt:      time.Now(),
	}
	db.Create(&member)

	checker := &mockDownloaderChecker{
		result: &model.TorrentInfo{Hash: "abc123", State: "uploading", IsFinished: true},
	}

	sc := NewSeedingConfirmation(db, nil)
	err := sc.CheckOnce(context.Background(), checker)
	assert.NoError(t, err)

	var updated model.PublishGroupMember
	db.First(&updated, member.ID)
	assert.Equal(t, model.MemberStatusUploaded, updated.Status)
}
