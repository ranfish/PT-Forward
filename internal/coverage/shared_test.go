package coverage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// fakeSharedIYUU §59.535: 共享查询层测试桩——计数问询次数+可注入结果/错误。
type fakeSharedIYUU struct {
	calls   int
	lastArg []string
	hits    map[string][]model.CoverageHit
	err     error
}

func (f *fakeSharedIYUU) GetSeededSites(ctx context.Context, infoHash string) ([]string, error) {
	return nil, nil
}

func (f *fakeSharedIYUU) BatchGetSeededSites(ctx context.Context, infoHashes []string) (map[string][]string, error) {
	return map[string][]string{}, nil
}

func (f *fakeSharedIYUU) BatchGetCoverageHits(ctx context.Context, infoHashes []string) (map[string][]model.CoverageHit, error) {
	f.calls++
	f.lastArg = infoHashes
	if f.err != nil {
		return nil, f.err
	}
	return f.hits, nil
}

func newSharedTestService(t *testing.T, fake *fakeSharedIYUU) *Service {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.SiteCoverageCache{}, &model.CoverageQueryState{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return NewService(db, fake, nil, zap.NewNop())
}

func TestQuerySharedCoverage(t *testing.T) {
	const hashA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const hashB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	window := 24 * time.Hour

	// ① 空输入：零调用零错误
	fake := &fakeSharedIYUU{}
	svc := newSharedTestService(t, fake)
	got, err := svc.QuerySharedCoverage(context.Background(), nil, window)
	if err != nil || len(got) != 0 {
		t.Errorf("① 空输入应 (空, nil): %v %v", got, err)
	}

	// ② 窗口外问询：IYUU 结果落库（probably_has·target_hash·tid）+MarkQueried
	fake2 := &fakeSharedIYUU{hits: map[string][]model.CoverageHit{
		hashA: {{SiteName: "站X", TorrentID: "123", TargetInfoHash: "cccccccccccccccccccccccccccccccccccccccc", Source: "iyuu"}},
	}}
	svc2 := newSharedTestService(t, fake2)
	res, err := svc2.QuerySharedCoverage(context.Background(), []string{hashA}, window)
	if err != nil || fake2.calls != 1 {
		t.Errorf("② 窗口外应问询一次无错: calls=%d err=%v", fake2.calls, err)
	}
	rows := res[hashA]
	if len(rows) != 1 || rows[0].Status != model.CoverageProbablyHas ||
		rows[0].Source != model.CoverageSourceIYUU ||
		rows[0].TargetInfoHash != "cccccccccccccccccccccccccccccccccccccccc" ||
		rows[0].TorrentID != "123" {
		t.Errorf("② 落库形态: %+v", rows[0])
	}
	// 再查一次（窗口内）：零 IYUU 请求+缓存返回
	res2, err := svc2.QuerySharedCoverage(context.Background(), []string{hashA}, window)
	if err != nil || fake2.calls != 1 {
		t.Errorf("② 窗口内应零请求: calls=%d err=%v", fake2.calls, err)
	}
	if len(res2[hashA]) != 1 || res2[hashA][0].SiteName != "站X" {
		t.Errorf("② 缓存返回: %+v", res2[hashA])
	}

	// ③ 先到优先：已有 tracker 行（confirmed_has·1.0）不被 IYUU 降级·但补 tid/target_hash
	fake3 := &fakeSharedIYUU{hits: map[string][]model.CoverageHit{
		hashB: {{SiteName: "站Y", TorrentID: "456", TargetInfoHash: "dddddddddddddddddddddddddddddddddddddddd"}},
	}}
	svc3 := newSharedTestService(t, fake3)
	if err := svc3.db.Create(&model.SiteCoverageCache{
		InfoHash: hashB, SiteName: "站Y", Status: model.CoverageConfirmedHas,
		Source: model.CoverageSourceTracker, Confidence: 1.0, QueriedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
	}).Error; err != nil {
		t.Fatalf("seed tracker row: %v", err)
	}
	if _, err := svc3.QuerySharedCoverage(context.Background(), []string{hashB}, window); err != nil {
		t.Fatalf("③ query: %v", err)
	}
	var after model.SiteCoverageCache
	if err := svc3.db.Where("info_hash = ? AND site_name = ?", hashB, "站Y").First(&after).Error; err != nil {
		t.Fatalf("③ load: %v", err)
	}
	if after.Status != model.CoverageConfirmedHas || after.Source != model.CoverageSourceTracker || after.Confidence != 1.0 {
		t.Errorf("③ 先到优先被破坏: %+v", after)
	}
	if after.TorrentID != "456" || after.TargetInfoHash != "dddddddddddddddddddddddddddddddddddddddd" {
		t.Errorf("③ IYUU 明细未补齐: tid=%q target=%q", after.TorrentID, after.TargetInfoHash)
	}

	// ④ IYUU 失败降级：现存行照常返回+err 透出
	fake4 := &fakeSharedIYUU{err: errors.New("iyuu down")}
	svc4 := newSharedTestService(t, fake4)
	if err := svc4.db.Create(&model.SiteCoverageCache{
		InfoHash: hashA, SiteName: "站Z", Status: model.CoverageConfirmedHas,
		Source: model.CoverageSourcePiecesHash, Confidence: 1.0, QueriedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
	}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	res4, err4 := svc4.QuerySharedCoverage(context.Background(), []string{hashA}, window)
	if err4 == nil {
		t.Errorf("④ IYUU 失败应透出 err")
	}
	if len(res4[hashA]) != 1 || res4[hashA][0].SiteName != "站Z" {
		t.Errorf("④ 降级应返回现存行: %+v", res4[hashA])
	}

	// ⑤ confirmed_not 不进返回集
	fake5 := &fakeSharedIYUU{}
	svc5 := newSharedTestService(t, fake5)
	if err := svc5.db.Exec("INSERT INTO site_coverage_cache (info_hash,site_name,status,source,confidence,queried_at,expires_at) VALUES (?,?,?,?,?,datetime('now'),datetime('now','+1 day'))",
		hashA, "站N", model.CoverageConfirmedNot, model.CoverageSourcePiecesHash, 0.95).Error; err != nil {
		t.Fatalf("seed not row: %v", err)
	}
	res5, _ := svc5.QuerySharedCoverage(context.Background(), []string{hashA}, window)
	if len(res5[hashA]) != 0 {
		t.Errorf("⑤ confirmed_not 不应返回: %+v", res5[hashA])
	}

	// ⑥ 去重：重复 hash 单次问询
	fake6 := &fakeSharedIYUU{}
	svc6 := newSharedTestService(t, fake6)
	_, _ = svc6.QuerySharedCoverage(context.Background(), []string{hashA, hashA, ""}, window)
	if fake6.calls != 1 || len(fake6.lastArg) != 1 {
		t.Errorf("⑥ 去重形态: calls=%d arg=%v", fake6.calls, fake6.lastArg)
	}
}
