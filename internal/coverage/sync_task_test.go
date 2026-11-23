package coverage

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
)

// TestCoverageWindow §59.535 刀3: 窗口=调度周期推导（「多久查一轮缓存保鲜多久」）。
func TestCoverageWindow(t *testing.T) {
	cases := []struct {
		schedule string
		want     time.Duration
	}{
		{"0 4 * * *", 24 * time.Hour},    // 每天 4 点=24h
		{"0 */12 * * *", 12 * time.Hour}, // 每 12h
		{"0 */6 * * *", 6 * time.Hour},   // 每 6h
		{"", 24 * time.Hour},             // 空=默认
		{"not a cron", 24 * time.Hour},   // 非法=fallback
	}
	for _, c := range cases {
		got := model.CoverageWindow(c.schedule)
		if got != c.want {
			t.Errorf("CoverageWindow(%q)=%v want %v", c.schedule, got, c.want)
		}
	}
}

// TestRunCoverageSync §59.535 刀3: 任务体三态——空快照零请求/有 hash 走共享
// 层落库/防重入。
func TestRunCoverageSync(t *testing.T) {
	const hashA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	// ① 空快照：零 IYUU 调用
	fake := &fakeSharedIYUU{}
	svc := newSharedTestService(t, fake)
	if err := svc.db.AutoMigrate(&model.TorrentSnapshot{}); err != nil {
		t.Fatalf("migrate snapshot: %v", err)
	}
	if err := svc.RunCoverageSync(context.Background(), 24*time.Hour); err != nil {
		t.Fatalf("① 空快照: %v", err)
	}
	if fake.calls != 0 {
		t.Errorf("① 空快照应零 IYUU 调用: %d", fake.calls)
	}

	// ② 有 100% hash：走共享层落库（probably_has·target_hash）·未完成不查
	fake2 := &fakeSharedIYUU{hits: map[string][]model.CoverageHit{
		hashA: {{SiteName: "站X", TorrentID: "1", TargetInfoHash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}},
	}}
	svc2 := newSharedTestService(t, fake2)
	if err := svc2.db.AutoMigrate(&model.TorrentSnapshot{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := svc2.db.Exec("INSERT INTO torrent_snapshots (hash,client_uid,name,state,progress,last_seen,created_at,updated_at) VALUES (?,?,?,?,?,datetime('now'),datetime('now'),datetime('now'))",
		hashA, 1, "种子A", "seeding", 1.0).Error; err != nil {
		t.Fatalf("seed snapshot: %v", err)
	}
	if err := svc2.db.Exec("INSERT INTO torrent_snapshots (hash,client_uid,name,state,progress,last_seen,created_at,updated_at) VALUES (?,?,?,?,?,datetime('now'),datetime('now'),datetime('now'))",
		"cccccccccccccccccccccccccccccccccccccccc", 1, "未完成", "downloading", 0.5).Error; err != nil {
		t.Fatalf("seed partial: %v", err)
	}
	if err := svc2.RunCoverageSync(context.Background(), 24*time.Hour); err != nil {
		t.Fatalf("② run: %v", err)
	}
	if fake2.calls != 1 || len(fake2.lastArg) != 1 || fake2.lastArg[0] != hashA {
		t.Errorf("② 应只查 100%% hash: calls=%d arg=%s", fake2.calls, fmt.Sprint(fake2.lastArg))
	}
	var cnt int64
	svc2.db.Model(&model.SiteCoverageCache{}).Where("info_hash = ?", hashA).Count(&cnt)
	if cnt != 1 {
		t.Errorf("② 覆盖行应落库: %d", cnt)
	}
	st := svc2.CoverageStatus()
	if st["running"] != false || st["last_stats"] == "" {
		t.Errorf("② 状态透出: %+v", st)
	}

	// ③ 防重入：running 中再跑返回 ErrCoverageRunning
	svc3 := newSharedTestService(t, fake)
	if err := svc3.db.AutoMigrate(&model.TorrentSnapshot{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	svc3.covRun.running = true
	if err := svc3.RunCoverageSync(context.Background(), time.Hour); !errors.Is(err, ErrCoverageRunning) {
		t.Errorf("③ 防重入: %v", err)
	}
}
