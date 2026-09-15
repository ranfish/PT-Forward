package seeding

import (
	"strings"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupReconcileDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:reconcile_" + time.Now().Format("150405.000000000") + "?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.SeedingTorrentRecord{}, &model.SeedingClientConfig{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// §59.227 场景1: record archived × qb stalledUP × 超 10min → 重激活 seeding
func TestReconcileStaleRecords_Reactivate(t *testing.T) {
	db := setupReconcileDB(t)
	db.Create(&model.SeedingClientConfig{ClientID: "c1", Enabled: true, Scope: "all"})
	old := time.Now().Add(-time.Hour)
	db.Create(&model.SeedingTorrentRecord{
		ClientID: "c1", InfoHash: "ABC123", Status: model.SeedingStatusArchived,
		LastActionBy: "rule:慢车", UpdatedAt: old, CreatedAt: old,
	})
	e := NewEngine(db, zap.NewNop())
	tm := map[string]*model.TorrentInfo{"abc123": {State: "stalledUP", Name: "x"}}
	e.reconcileStaleRecords(t.Context(), "c1", tm)

	var rec model.SeedingTorrentRecord
	db.Where("client_id = ? AND info_hash = ?", "c1", "ABC123").First(&rec)
	if rec.Status != model.SeedingStatusSeeding {
		t.Errorf("status = %q, want seeding", rec.Status)
	}
	if rec.LastActionBy != "reconcile" {
		t.Errorf("last_action_by = %q, want reconcile", rec.LastActionBy)
	}
}

// §59.227 ①防抖: 状态写入未满 10 分钟不对账（pause 生效延迟窗保护）
func TestReconcileStaleRecords_Debounce(t *testing.T) {
	db := setupReconcileDB(t)
	db.Create(&model.SeedingClientConfig{ClientID: "c1", Enabled: true, Scope: "all"})
	db.Create(&model.SeedingTorrentRecord{
		ClientID: "c1", InfoHash: "ABC123", Status: model.SeedingStatusPausedFreeEnd,
		UpdatedAt: time.Now().Add(-2 * time.Minute), // <10min
	})
	e := NewEngine(db, zap.NewNop())
	tm := map[string]*model.TorrentInfo{"abc123": {State: "stalledUP"}}
	e.reconcileStaleRecords(t.Context(), "c1", tm)

	var rec model.SeedingTorrentRecord
	db.Where("info_hash = ?", "ABC123").First(&rec)
	if rec.Status != model.SeedingStatusPausedFreeEnd {
		t.Errorf("防抖失败：status = %q, want paused_free_end（10 分钟内不应重激活）", rec.Status)
	}
}

// §59.227: pausedUP（已暂停）不对账——只有真做种态（uploading/stalledUP/forcedUP）触发
func TestReconcileStaleRecords_PausedUpState(t *testing.T) {
	db := setupReconcileDB(t)
	db.Create(&model.SeedingClientConfig{ClientID: "c1", Enabled: true, Scope: "all"})
	old := time.Now().Add(-time.Hour)
	db.Create(&model.SeedingTorrentRecord{
		ClientID: "c1", InfoHash: "ABC123", Status: model.SeedingStatusPausedRule, UpdatedAt: old,
	})
	e := NewEngine(db, zap.NewNop())
	tm := map[string]*model.TorrentInfo{"abc123": {State: "pausedUP"}}
	e.reconcileStaleRecords(t.Context(), "c1", tm)

	var rec model.SeedingTorrentRecord
	db.Where("info_hash = ?", "ABC123").First(&rec)
	if rec.Status != model.SeedingStatusPausedRule {
		t.Errorf("pausedUP 不应触发重激活：status = %q", rec.Status)
	}
}

// §59.227 ②: deleted/deleting 失联形态纳入对账
func TestReconcileStaleRecords_DeletedAndDeleting(t *testing.T) {
	db := setupReconcileDB(t)
	db.Create(&model.SeedingClientConfig{ClientID: "c1", Enabled: true, Scope: "all"})
	old := time.Now().Add(-time.Hour)
	db.Create(&model.SeedingTorrentRecord{ClientID: "c1", InfoHash: "D1", Status: model.SeedingStatusDeleted, UpdatedAt: old})
	db.Create(&model.SeedingTorrentRecord{ClientID: "c1", InfoHash: "D2", Status: model.SeedingStatusDeleting, UpdatedAt: old})
	e := NewEngine(db, zap.NewNop())
	tm := map[string]*model.TorrentInfo{"d1": {State: "stalledUP"}, "d2": {State: "uploading"}}
	e.reconcileStaleRecords(t.Context(), "c1", tm)

	var n int64
	db.Model(&model.SeedingTorrentRecord{}).Where("status = ?", model.SeedingStatusSeeding).Count(&n)
	if n != 2 {
		t.Errorf("deleted/deleting 对账数 = %d, want 2", n)
	}
}

// §59.227 ③: comment 详情页 URL tid 提取
func TestExtractTorrentIDFromComment(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://ubits.club/details.php?id=345299", "345299"},
		{"https://hhanclub.net/details.php?id=210878", "210878"},
		{"https://tracker.example.com/announce.php?passkey=x", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := extractTorrentIDFromComment(c.in); got != c.want {
			t.Errorf("extractTorrentIDFromComment(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// §59.227 2b: recordMap 幽灵不短路导入——DB count 为真相源（LOWER 归一）
func TestSyncUnmanaged_GhostRecordMap(t *testing.T) {
	db := setupReconcileDB(t)
	db.Create(&model.SeedingClientConfig{ClientID: "c1", Enabled: true, Scope: "all"})
	e := NewEngine(db, zap.NewNop())
	// 内存幽灵：recordMap 有 key 但 DB 无 record
	e.mu.Lock()
	e.recordMap[recordKey("c1", "ghost1")] = &model.SeedingTorrentRecord{ClientID: "c1", InfoHash: "ghost1"}
	e.mu.Unlock()

	// ghost1 在 qb 但 DB 无 record → 应导入（旧逻辑被幽灵 continue）
	// 大写 hash 变体：DB 有大写 record → LOWER 归一后不重复导入
	db.Create(&model.SeedingTorrentRecord{ClientID: "c1", InfoHash: "UPPERHASH", Status: model.SeedingStatusSeeding})
	tm := map[string]*model.TorrentInfo{
		"ghost1":    {State: "stalledUP", Name: "g", TrackerURL: "https://t.hhanclub.net/announce.php?passkey=x"},
		"upperhash": {State: "stalledUP", Name: "u", TrackerURL: "https://t.hhanclub.net/announce.php?passkey=x"},
	}
	e.syncUnmanagedTorrents(t.Context(), "c1", tm)

	var n int64
	db.Model(&model.SeedingTorrentRecord{}).Where("info_hash = ?", "ghost1").Count(&n)
	if n != 1 {
		t.Errorf("幽灵短路修复失败：ghost1 record 数 = %d, want 1", n)
	}
	db.Model(&model.SeedingTorrentRecord{}).Where("LOWER(info_hash) = ?", "upperhash").Count(&n)
	if n != 1 {
		t.Errorf("LOWER 归一失败：upperhash record 数 = %d, want 1（大写已存在不应重复建）", n)
	}
}

// §59.227: comment 回填 tid
func TestSyncUnmanaged_CommentTorrentID(t *testing.T) {
	db := setupReconcileDB(t)
	db.Create(&model.SeedingClientConfig{ClientID: "c1", Enabled: true, Scope: "all"})
	e := NewEngine(db, zap.NewNop())
	tm := map[string]*model.TorrentInfo{
		"cmt1": {State: "stalledUP", Name: "c", TrackerURL: "https://t.hhanclub.net/announce.php?passkey=x",
			Comment: "https://hhanclub.net/details.php?id=210878"},
	}
	e.syncUnmanagedTorrents(t.Context(), "c1", tm)
	var rec model.SeedingTorrentRecord
	db.Where("info_hash = ?", "cmt1").First(&rec)
	if rec.TorrentID != "210878" {
		t.Errorf("comment tid 回填失败：torrent_id = %q, want 210878", rec.TorrentID)
	}
	if !strings.HasPrefix(rec.Source, "imported") {
		t.Errorf("source = %q", rec.Source)
	}
}
