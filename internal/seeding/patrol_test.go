// §59.31 幽灵种子巡检 v2 单测：hitKeyword / TrackerDomain / 冷却调度 / 纯标记语义。
package seeding

import (
	"context"
	"fmt"
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
	if got.UnregisteredAt == nil {
		t.Error("unregistered_at not set")
	}
	if got.Status != model.SeedingStatusSeeding {
		t.Errorf("status must not change (pure detection), got %v", got.Status)
	}
}

// patrolMockClient §59.31 二审 NEW-1：qb 分发/上限/游标单测的 mock 下载器。
type patrolMockClient struct {
	// GetTrackerMessagesAll 返回表：hash → 消息列表（不存在 = 模拟 API 错误）
	msgs    map[string][]model.TrackerMessage
	errSet  map[string]bool
	calls   []string // 记录调用顺序
}

func (m *patrolMockClient) GetTrackerMessagesAll(_ context.Context, hash string) ([]model.TrackerMessage, error) {
	m.calls = append(m.calls, hash)
	if m.errSet[hash] {
		return nil, fmt.Errorf("simulated api error")
	}
	return m.msgs[hash], nil
}

// TestPatrolDispatch 两路分发语义：有信号者进池扫（带冷却），无信号者进全量兜底。
func TestPatrolDispatch(t *testing.T) {
	db := setupEngineTestDB(t)
	e := NewEngine(db, zap.NewNop())
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer e.Stop(context.Background())

	// 构造 3 个候选：hash-a 有信号，hash-b/c 无信号
	for _, h := range []string{"hash-a", "hash-b", "hash-c"} {
		rec := &model.SeedingTorrentRecord{
			ClientID: "QB1", InfoHash: h, SiteName: "测试",
			Status: model.SeedingStatusSeeding,
		}
		if err := db.Create(rec).Error; err != nil {
			t.Fatal(err)
		}
		e.mu.Lock()
		e.recordMap[recordKey("QB1", h)] = rec
		e.mu.Unlock()
	}

	mock := &patrolMockClient{
		msgs: map[string][]model.TrackerMessage{
			"hash-a": {{URL: "https://t.example.com/announce", Msg: "torrent not found"}},
			"hash-b": {{URL: "https://t.example.com/announce", Msg: ""}},
			"hash-c": {{URL: "https://t.example.com/announce", Msg: ""}},
		},
	}

	// 直接调 checkUnregisteredTorrents 的 qbittorrent 分支（框架查 DB clients 表，
	// 单测 DB 无该表行 → clientFramework 返回 ""，走 default 兜底=全量。
	// 因此这里直接测两个 batch 函数 + 分区逻辑的等价组合）
	e.patrolConfirmBatch(context.Background(), "QB1", mock, []*model.SeedingTorrentRecord{
		e.recordMap[recordKey("QB1", "hash-a")],
	}, []string{"torrent not found"})
	e.patrolFullScanBatch(context.Background(), "QB1", mock, []*model.SeedingTorrentRecord{
		e.recordMap[recordKey("QB1", "hash-b")],
		e.recordMap[recordKey("QB1", "hash-c")],
	}, []string{"torrent not found"})

	// hash-a 应被标记（池扫命中）
	a := e.recordMap[recordKey("QB1", "hash-a")]
	if !a.Unregistered || a.UnregisteredMsg != "torrent not found" {
		t.Errorf("hash-a should be marked via suspect pool, got %+v", a.UnregisteredMsg)
	}
	// b/c 未命中（msg 为空）
	if e.recordMap[recordKey("QB1", "hash-b")].Unregistered {
		t.Error("hash-b should not be marked")
	}
	if e.recordMap[recordKey("QB1", "hash-c")].Unregistered {
		t.Error("hash-c should not be marked")
	}
}

// TestPatrolCapAttempts 上限按尝试计（含失败，二审 NEW-2）。
func TestPatrolCapAttempts(t *testing.T) {
	db := setupEngineTestDB(t)
	e := NewEngine(db, zap.NewNop())
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer e.Stop(context.Background())

	// 60 个候选全部 API 失败（错误风暴）
	var suspects []*model.SeedingTorrentRecord
	mock := &patrolMockClient{errSet: map[string]bool{}}
	for i := 0; i < 60; i++ {
		h := fmt.Sprintf("err-%02d", i)
		rec := &model.SeedingTorrentRecord{
			ClientID: "QB1", InfoHash: h, SiteName: "测试",
			Status: model.SeedingStatusSeeding,
		}
		if err := db.Create(rec).Error; err != nil {
			t.Fatal(err)
		}
		suspects = append(suspects, rec)
		mock.errSet[h] = true
	}

	e.patrolConfirmBatch(context.Background(), "QB1", mock, suspects, []string{"torrent not found"})

	if len(mock.calls) > e.patrolBatchSize() {
		t.Errorf("error storm: %d attempts exceed cap %d", len(mock.calls), e.patrolBatchSize())
	}
}

// TestPatrolCursorRoundRobin 全量兜底游标轮转（排序后确定性）。
func TestPatrolCursorRoundRobin(t *testing.T) {
	db := setupEngineTestDB(t)
	e := NewEngine(db, zap.NewNop())
	if err := e.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer e.Stop(context.Background())

	mock := &patrolMockClient{msgs: map[string][]model.TrackerMessage{}}
	var records []*model.SeedingTorrentRecord
	for i := 0; i < 7; i++ {
		h := fmt.Sprintf("h%d", i)
		rec := &model.SeedingTorrentRecord{
			ClientID: "QB2", InfoHash: h, SiteName: "测试",
			Status: model.SeedingStatusSeeding,
		}
		if err := db.Create(rec).Error; err != nil {
			t.Fatal(err)
		}
		records = append(records, rec)
		mock.msgs[h] = []model.TrackerMessage{{URL: "https://t.example.com/announce", Msg: ""}}
	}

	// batchSize=50 > 7 → 一 tick 全扫完，游标=7；下一 tick wrap 到 0 重扫
	e.patrolFullScanBatch(context.Background(), "QB2", mock, records, []string{"kw"})
	if len(mock.calls) != 7 {
		t.Fatalf("first pass: %d calls, want 7", len(mock.calls))
	}
	mock.calls = nil
	e.patrolFullScanBatch(context.Background(), "QB2", mock, records, []string{"kw"})
	if len(mock.calls) != 7 {
		t.Fatalf("after wrap: %d calls, want 7 (cursor wrapped)", len(mock.calls))
	}
	// §59.31 三审缺口1：排序确定性守护——按 InfoHash 升序（删 sort.Slice 此处失败）
	for i, call := range mock.calls {
		if want := fmt.Sprintf("h%d", i); call != want {
			t.Errorf("call[%d] = %q, want %q (sorted round-robin)", i, call, want)
		}
	}
}
