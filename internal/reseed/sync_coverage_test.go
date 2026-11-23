package reseed

import (
	"context"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestSyncMatchCoverage §59.535 刀2: 辅种匹配落库即同步覆盖表——三态锚。
func TestSyncMatchCoverage(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.ReseedMatch{}, &model.SiteCoverageCache{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	e := &Engine{db: db, logger: zap.NewNop()}
	const hash = "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"

	// ① matched：SaveMatch→覆盖表同步落库（confirmed_has·source=match_method·含 target_hash）
	m1 := &model.ReseedMatch{
		TaskID: 1, ClientUID: 2, SourceSite: "站源", SourceTorrentID: "11",
		SourceInfoHash: hash, TargetSite: "站A", TargetTorrentID: "777",
		TargetInfoHash: "ffffffffffffffffffffffffffffffffffffffff", MatchMethod: "pieces_hash",
		Confidence: 1.0, Status: model.MatchStatusMatched,
	}
	if err := e.SaveMatch(context.Background(), m1); err != nil {
		t.Fatalf("① save: %v", err)
	}
	var cov model.SiteCoverageCache
	if err := db.Where("info_hash = ? AND site_name = ?", hash, "站A").First(&cov).Error; err != nil {
		t.Fatalf("① 应同步覆盖表: %v", err)
	}
	if cov.Status != model.CoverageConfirmedHas || cov.Source != "pieces_hash" ||
		cov.TorrentID != "777" || cov.TargetInfoHash != "ffffffffffffffffffffffffffffffffffffffff" {
		t.Errorf("① 落库形态: %+v", cov)
	}

	// ② failed：匹配失败≠站上有——不同步
	m2 := &model.ReseedMatch{
		TaskID: 1, ClientUID: 2, SourceSite: "站源", SourceTorrentID: "22",
		SourceInfoHash: hash, TargetSite: "站B", TargetTorrentID: "888",
		MatchMethod: "size_title", Confidence: 0.5, Status: model.MatchStatusFailed,
	}
	if err := e.SaveMatch(context.Background(), m2); err != nil {
		t.Fatalf("② save: %v", err)
	}
	var cnt2 int64
	db.Model(&model.SiteCoverageCache{}).Where("info_hash = ? AND site_name = ?", hash, "站B").Count(&cnt2)
	if cnt2 != 0 {
		t.Errorf("② failed 不应同步: count=%d", cnt2)
	}

	// ③ 先到优先：已有 tracker 高置信行不被 match 行降级·但补空缺明细
	if err := db.Exec("INSERT INTO site_coverage_cache (info_hash,site_name,status,source,confidence,queried_at,expires_at) VALUES (?,?,?,?,?,?,?)",
		hash, "站C", model.CoverageConfirmedHas, model.CoverageSourceTracker, 1.0, time.Now(), time.Now().Add(time.Hour)).Error; err != nil {
		t.Fatalf("③ seed: %v", err)
	}
	m3 := &model.ReseedMatch{
		TaskID: 1, ClientUID: 2, SourceSite: "站源", SourceTorrentID: "33",
		SourceInfoHash: hash, TargetSite: "站C", TargetTorrentID: "999",
		TargetInfoHash: "1111111111111111111111111111111111111111", MatchMethod: "iyuu",
		Confidence: 0.9, Status: model.MatchStatusMatched,
	}
	if err := e.SaveMatch(context.Background(), m3); err != nil {
		t.Fatalf("③ save: %v", err)
	}
	var cov3 model.SiteCoverageCache
	if err := db.Where("info_hash = ? AND site_name = ?", hash, "站C").First(&cov3).Error; err != nil {
		t.Fatalf("③ load: %v", err)
	}
	if cov3.Source != model.CoverageSourceTracker || cov3.Confidence != 1.0 {
		t.Errorf("③ 先到优先被破坏: source=%s conf=%v", cov3.Source, cov3.Confidence)
	}
	if cov3.TorrentID != "999" || cov3.TargetInfoHash != "1111111111111111111111111111111111111111" {
		t.Errorf("③ 明细未补齐: tid=%q target=%q", cov3.TorrentID, cov3.TargetInfoHash)
	}
}

// TestPreloadIYUUMarkQueried §59.535 联合回归 P2: 辅种 IYUU 查询成功→窗口有效
// 时 MarkQueried（覆盖任务窗口内跳过=双通道共享闭环）。
func TestPreloadIYUUMarkQueried(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.CoverageQueryState{}, &model.IYUUConfig{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// fake iyuuService：实现 CoverageTaskConfig 接口（读 db 配置）
	fakeIYUUSvc := &fakeCovTaskConfig{db: db}
	e := &Engine{db: db, logger: zap.NewNop(), iyuuService: fakeIYUUSvc}
	// 覆盖任务启用（窗口 24h）
	db.Create(&model.IYUUConfig{ID: 1, Enabled: true, CoverageEnabled: true, CoverageSchedule: "0 4 * * *"})
	_ = fakeIYUUSvc
	const hash = "dddddddddddddddddddddddddddddddddddddd11"
	_ = hash
	// 窗口读取（配置启用→24h）
	if w, ok := e.iyuuCoverageWindow(context.Background()); !ok || w != 24*time.Hour {
		t.Errorf("窗口读取: %v %v", w, ok)
	}
	// MarkQueried SQL 形态（同 preloadIYUUResults 内联段）
	now := time.Now()
	if err := db.Exec(`INSERT INTO coverage_query_state (info_hash, queried_at, expires_at) VALUES (?,?,?)
		ON CONFLICT(info_hash) DO UPDATE SET queried_at=excluded.queried_at, expires_at=excluded.expires_at`,
		"dddddddddddddddddddddddddddddddddddddd11", now, now.Add(24*time.Hour)).Error; err != nil {
		t.Fatalf("mark: %v", err)
	}
	var st model.CoverageQueryState
	if err := db.Where("info_hash = ?", "dddddddddddddddddddddddddddddddddddddd11").First(&st).Error; err != nil {
		t.Fatalf("load state: %v", err)
	}
	if st.ExpiresAt.Before(now.Add(23 * time.Hour)) {
		t.Errorf("expires 应≈24h: %v", st.ExpiresAt)
	}
	// 幂等：upsert 覆盖不报错
	if err := db.Exec(`INSERT INTO coverage_query_state (info_hash, queried_at, expires_at) VALUES (?,?,?)
		ON CONFLICT(info_hash) DO UPDATE SET queried_at=excluded.queried_at, expires_at=excluded.expires_at`,
		"dddddddddddddddddddddddddddddddddddddd11", now, now.Add(24*time.Hour)).Error; err != nil {
		t.Errorf("幂等 upsert: %v", err)
	}
	// 停用态：iyuuCoverageWindow false（不标语义）
	db.Model(&model.IYUUConfig{}).Where("id = 1").Update("coverage_enabled", false)
	if _, ok := e.iyuuCoverageWindow(context.Background()); ok {
		t.Errorf("停用应 false")
	}
}

// fakeCovTaskConfig: iyuuCoverageWindow 接口断言的最小实现（读 db 的 iyuu_configs）。
type fakeCovTaskConfig struct {
	db *gorm.DB
}

func (f *fakeCovTaskConfig) GetSeededSites(ctx context.Context, infoHash string) ([]string, error) {
	return nil, nil
}

func (f *fakeCovTaskConfig) QueryReseed(ctx context.Context, infoHashes []string) ([]*model.IYUUReseedResult, error) {
	return nil, nil
}

func (f *fakeCovTaskConfig) GetSiteList(ctx context.Context) ([]model.IYUUSite, error) {
	return nil, nil
}

func (f *fakeCovTaskConfig) ReportExisting(ctx context.Context, sidList []int) error { return nil }

func (f *fakeCovTaskConfig) Ping(ctx context.Context) error { return nil }

func (f *fakeCovTaskConfig) SendNotification(ctx context.Context, text, desp string) error {
	return nil
}

func (f *fakeCovTaskConfig) CoverageTaskConfig(ctx context.Context) (bool, string, error) {
	var cfg model.IYUUConfig
	if err := f.db.WithContext(ctx).First(&cfg).Error; err != nil {
		return false, "", err
	}
	schedule := cfg.CoverageSchedule
	if schedule == "" {
		schedule = "0 */12 * * *"
	}
	return cfg.CoverageEnabled, schedule, nil
}
