// §59.31 幽灵种子巡检 v2 单测：hitKeyword / TrackerDomain / 冷却调度 / 纯标记语义。
package seeding

import (
	"context"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

func TestHitKeyword(t *testing.T) {
	kws := []string{"torrent not found", "unregistered", "Torrent not registered with this tracker"}
	cases := []struct {
		msg  string
		want bool
	}{
		{"Torrent not found", true},
		{"TORRENT NOT FOUND", true},
		{"torrent not found with this tracker", true},
		{"Unregistered torrent", true},
		{"Torrent not registered with this tracker", true},
		{"connection timed out", false},
		{"", false},
		{"success", false},
	}
	for _, c := range cases {
		if got := hitKeyword(c.msg, kws); got != c.want {
			t.Errorf("hitKeyword(%q) = %v, want %v", c.msg, got, c.want)
		}
	}
	if hitKeyword("anything", []string{""}) {
		t.Error("empty keyword must be skipped")
	}
}

func TestTrackerDomain(t *testing.T) {
	cases := []struct{ url, want string }{
		{"https://pt.keepfrds.com/announce/xxx", "pt.keepfrds.com"},
		{"http://tracker.example.com:8080/announce", "tracker.example.com"},
		{"https://chdbits.xyz/announce.php?passkey=secret", "chdbits.xyz"},
		{"tracker.no-scheme.com/announce", "tracker.no-scheme.com"},
	}
	for _, c := range cases {
		tm := model.TrackerMessage{URL: c.url, Msg: "x"}
		if got := tm.TrackerDomain(); got != c.want {
			t.Errorf("TrackerDomain(%q) = %q, want %q", c.url, got, c.want)
		}
	}
}

// TestPatrolCooldown 冷却写入与判定（遗漏 A）。
func TestPatrolCooldown(t *testing.T) {
	e := NewEngine(setupEngineTestDB(t), zap.NewNop())
	now := time.Now()

	if e.inUnregCooldown("PT0", "hash1", now, 30*time.Minute) {
		t.Error("no entry → not in cooldown")
	}
	e.setUnregCooldown("PT0", "hash1", now)

	if !e.inUnregCooldown("PT0", "hash1", now.Add(10*time.Minute), 30*time.Minute) {
		t.Error("within window → in cooldown")
	}
	if e.inUnregCooldown("PT0", "hash1", now.Add(31*time.Minute), 30*time.Minute) {
		t.Error("past window → expired")
	}
	if e.inUnregCooldown("PT1", "hash1", now, 30*time.Minute) {
		t.Error("different client → separate")
	}
}

// TestMarkUnregistered 纯标记语义：只写 unregistered 三列，不改 status（检测无副作用）。
func TestMarkUnregistered(t *testing.T) {
	db := setupEngineTestDB(t)
	e := NewEngine(db, zap.NewNop())
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer e.Stop(context.Background())

	rec := &model.SeedingTorrentRecord{
		ClientID: "PT0", InfoHash: "aa", SiteName: "朋友",
		Status: model.SeedingStatusSeeding,
	}
	if err := db.Create(rec).Error; err != nil {
		t.Fatal(err)
	}
	e.mu.Lock()
	e.recordMap[recordKey("PT0", "aa")] = rec
	e.mu.Unlock()

	e.markUnregistered(context.Background(), rec, "torrent not found", "pt.keepfrds.com")

	var got model.SeedingTorrentRecord
	db.Where("id = ?", rec.ID).First(&got)
	if !got.Unregistered {
		t.Error("unregistered not marked")
	}
	if got.UnregisteredMsg != "torrent not found" {
		t.Errorf("msg snapshot = %q", got.UnregisteredMsg)
	}
	if got.UnregisteredTracker != "pt.keepfrds.com" {
		t.Errorf("tracker = %q", got.UnregisteredTracker)
	}
	if got.Status != model.SeedingStatusSeeding {
		t.Errorf("status must not change (pure detection), got %v", got.Status)
	}
}
