package fingerprint

import (
	"context"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestSyncFetchCoverage §59.535 刀4: 指纹落库→覆盖表同步三态。
func TestSyncFetchCoverage(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.ContentFingerprint{}, &model.SiteCoverageCache{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := NewRepository(db, zap.NewNop())
	const hash = "1234567890123456789012345678901234567890"

	// ① Save→覆盖行（confirmed_has/fetch/1.0/tid）
	fp := &model.ContentFingerprint{
		InfoHash: hash, SiteName: "站源", TorrentID: "999",
		PiecesHash: "ph1", TotalSize: 100, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := repo.Save(context.Background(), fp); err != nil {
		t.Fatalf("① save: %v", err)
	}
	var cov model.SiteCoverageCache
	if err := db.Where("info_hash = ? AND site_name = ?", hash, "站源").First(&cov).Error; err != nil {
		t.Fatalf("① 应同步覆盖表: %v", err)
	}
	if cov.Status != model.CoverageConfirmedHas || cov.Source != model.CoverageSourceFetch ||
		cov.Confidence != 1.0 || cov.TorrentID != "999" {
		t.Errorf("① 落库形态: %+v", cov)
	}

	// ② 先到优先：已有 tracker 行不被 fetch 降级·但补空缺 tid
	if err := db.Exec("INSERT INTO site_coverage_cache (info_hash,site_name,status,source,confidence,queried_at,expires_at) VALUES (?,?,?,?,?,?,?)",
		hash, "站T", model.CoverageConfirmedHas, model.CoverageSourceTracker, 0.8, time.Now(), time.Now().Add(time.Hour)).Error; err != nil {
		t.Fatalf("② seed: %v", err)
	}
	repo2 := NewRepository(db, zap.NewNop())
	fp2 := &model.ContentFingerprint{
		InfoHash: hash, SiteName: "站T", TorrentID: "888",
		PiecesHash: "ph2", TotalSize: 100, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := repo2.Save(context.Background(), fp2); err != nil {
		t.Fatalf("② save: %v", err)
	}
	var cov2 model.SiteCoverageCache
	if err := db.Where("info_hash = ? AND site_name = ?", hash, "站T").First(&cov2).Error; err != nil {
		t.Fatalf("② load: %v", err)
	}
	if cov2.Source != model.CoverageSourceTracker || cov2.Confidence != 0.8 {
		t.Errorf("② 先到优先被破坏: %+v", cov2)
	}
	if cov2.TorrentID != "888" {
		t.Errorf("② tid 未补: %q", cov2.TorrentID)
	}

	// ③ BatchSave→批量同步
	fpB := &model.ContentFingerprint{
		InfoHash: "abcdefabcdefabcdefabcdefabcdefabcdefabcd", SiteName: "站B", TorrentID: "7",
		PiecesHash: "ph3", TotalSize: 1, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := repo.BatchSave(context.Background(), []*model.ContentFingerprint{fpB}); err != nil {
		t.Fatalf("③ batch: %v", err)
	}
	var cnt int64
	db.Model(&model.SiteCoverageCache{}).Where("site_name = ?", "站B").Count(&cnt)
	if cnt != 1 {
		t.Errorf("③ 批量同步: %d", cnt)
	}
}
