package seeding

import (
	"context"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupFreeWaitDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return db
}

type mockDiscountChecker struct {
	discounts map[string]model.DiscountLevel
	err       error
}

func (m *mockDiscountChecker) CheckDiscount(ctx context.Context, siteName, torrentID string) (model.DiscountLevel, error) {
	if m.err != nil {
		return model.DiscountNone, m.err
	}
	key := siteName + "|" + torrentID
	if d, ok := m.discounts[key]; ok {
		return d, nil
	}
	return model.DiscountNone, nil
}

func TestFreeWaitMonitor_Add(t *testing.T) {
	db := setupFreeWaitDB(t)
	m := NewFreeWaitMonitor(db, zap.NewNop())

	m.Add("site-a", "123", "hash1", "Test Torrent", 1024, nil)
	assert.Equal(t, 1, m.PendingCount())

	m.Add("site-a", "123", "hash1", "Test Torrent", 1024, nil)
	assert.Equal(t, 1, m.PendingCount())
}

func TestFreeWaitMonitor_Remove(t *testing.T) {
	db := setupFreeWaitDB(t)
	m := NewFreeWaitMonitor(db, zap.NewNop())

	m.Add("site-a", "123", "hash1", "Test", 1024, nil)
	assert.Equal(t, 1, m.PendingCount())

	m.Remove("site-a", "123")
	assert.Equal(t, 0, m.PendingCount())
}

func TestFreeWaitMonitor_CheckOnce_BecameFree(t *testing.T) {
	db := setupFreeWaitDB(t)
	m := NewFreeWaitMonitor(db, zap.NewNop())

	m.Add("site-a", "123", "hash1", "Test Torrent", 1024, nil)
	m.Add("site-a", "456", "hash2", "Not Free", 2048, nil)

	checker := &mockDiscountChecker{
		discounts: map[string]model.DiscountLevel{
			"site-a|123": model.DiscountFree,
		},
	}

	added := []string{}
	addFunc := func(ctx context.Context, e *freeWaitEntry) error {
		added = append(added, e.TorrentID)
		return nil
	}

	processed := m.CheckOnce(context.Background(), checker, addFunc)
	assert.Equal(t, 1, processed)
	assert.Equal(t, []string{"123"}, added)
	assert.Equal(t, 1, m.PendingCount())
}

func TestFreeWaitMonitor_CheckOnce_Expired(t *testing.T) {
	db := setupFreeWaitDB(t)
	m := NewFreeWaitMonitor(db, zap.NewNop())

	before := time.Now().Add(-1 * time.Hour)
	m.Add("site-a", "123", "hash1", "Test", 1024, &before)

	checker := &mockDiscountChecker{}
	processed := m.CheckOnce(context.Background(), checker, nil)
	assert.Equal(t, 0, processed)
	assert.Equal(t, 0, m.PendingCount())
}

func TestFreeWaitMonitor_CheckOnce_NoneFree(t *testing.T) {
	db := setupFreeWaitDB(t)
	m := NewFreeWaitMonitor(db, zap.NewNop())

	m.Add("site-a", "123", "hash1", "Test", 1024, nil)

	checker := &mockDiscountChecker{
		discounts: map[string]model.DiscountLevel{},
	}

	processed := m.CheckOnce(context.Background(), checker, nil)
	assert.Equal(t, 0, processed)
	assert.Equal(t, 1, m.PendingCount())
}

func TestFreeWaitMonitor_ClearAll(t *testing.T) {
	db := setupFreeWaitDB(t)
	m := NewFreeWaitMonitor(db, zap.NewNop())

	m.Add("site-a", "123", "hash1", "Test", 1024, nil)
	m.Add("site-b", "456", "hash2", "Test2", 2048, nil)

	m.ClearAll()
	assert.Equal(t, 0, m.PendingCount())
}

func TestFreeWaitMonitor_AddEmptyTorrentID(t *testing.T) {
	db := setupFreeWaitDB(t)
	m := NewFreeWaitMonitor(db, zap.NewNop())

	m.Add("site-a", "", "hash1", "Test", 1024, nil)
	assert.Equal(t, 0, m.PendingCount())
}
