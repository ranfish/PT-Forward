package coverage

import (
	"context"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// 17 刀3（v0.0.1668）: Upsert 刷新锚（V-01 防回归）——已存在行以变化
// 字段写入必须刷新为新值（原同指针别名保持旧值——GORM finisher_api
// Find(dest) 污染 assigns）。
//
// 阴性对照：upsertCoverage 退回同指针（Assign(record).FirstOrCreate
// (record)）→ 断言新值落库 FAIL（旧值保持）。
func TestUpsertCoverage_RefreshOnExisting(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:upsert_refresh_anchor?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.SiteCoverageCache{}); err != nil {
		t.Fatal(err)
	}
	db.Where("1=1").Delete(&model.SiteCoverageCache{})

	s := &Service{db: db}
	ctx := context.Background()
	oldTime := time.Unix(1000, 0)
	newTime := time.Unix(2000, 0)

	// 旧行（confirmed_not/tracker）
	if err := s.UpsertCoverage(ctx, &model.SiteCoverageCache{
		InfoHash: "anchorhash", SiteName: "anchorsite",
		Status:     model.CoverageConfirmedNot,
		Source:     model.CoverageSourceTracker,
		Confidence: 1.0,
		QueriedAt:  oldTime, ExpiresAt: oldTime,
	}); err != nil {
		t.Fatal(err)
	}

	// 变化字段写入（confirmed_has/iyuu/t-new/newTime）
	if err := s.UpsertCoverage(ctx, &model.SiteCoverageCache{
		InfoHash: "anchorhash", SiteName: "anchorsite",
		Status:     model.CoverageConfirmedHas,
		Source:     model.CoverageSourceIYUU,
		TorrentID:  "t-new",
		Confidence: 1.0,
		QueriedAt:  newTime, ExpiresAt: newTime,
	}); err != nil {
		t.Fatal(err)
	}

	var out model.SiteCoverageCache
	if err := db.Where("info_hash = ? AND site_name = ?", "anchorhash", "anchorsite").
		First(&out).Error; err != nil {
		t.Fatal(err)
	}
	if out.Status != model.CoverageConfirmedHas {
		t.Errorf("Status=%q want confirmed_has（刷新失败=同指针别名回归——V-01）", out.Status)
	}
	if out.Source != model.CoverageSourceIYUU {
		t.Errorf("Source=%q want iyuu", out.Source)
	}
	if out.TorrentID != "t-new" {
		t.Errorf("TorrentID=%q want t-new", out.TorrentID)
	}
	if out.QueriedAt.Unix() != 2000 {
		t.Errorf("QueriedAt=%d want 2000（保持 1000=刷新失效）", out.QueriedAt.Unix())
	}
}

// 17 刀3: 端到端二次写入锚（F-01 场景转正）——「① IYUU 先写带
// TorrentID → ② tracker 后写覆盖」注释明示后写生效语义。原同指针缺陷
// 使后写失效（Status 保持先写值）。修复后断言后写字段落库。
func TestUpsertCoverage_SecondWriteWins(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:upsert_second_write?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.SiteCoverageCache{}); err != nil {
		t.Fatal(err)
	}
	db.Where("1=1").Delete(&model.SiteCoverageCache{})

	s := &Service{db: db}
	ctx := context.Background()
	now := time.Now()

	// ① IYUU 先写（带 TorrentID）
	if err := s.UpsertCoverage(ctx, &model.SiteCoverageCache{
		InfoHash: "secondhash", SiteName: "secondsite",
		Status: model.CoverageConfirmedHas, Source: model.CoverageSourceIYUU,
		TorrentID: "iyuu-tid", Confidence: 1.0,
		QueriedAt: now, ExpiresAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	// ② tracker 后写（覆盖同名——无 TorrentID）
	if err := s.UpsertCoverage(ctx, &model.SiteCoverageCache{
		InfoHash: "secondhash", SiteName: "secondsite",
		Status: model.CoverageConfirmedHas, Source: model.CoverageSourceTracker,
		Confidence: 1.0,
		QueriedAt:  now, ExpiresAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	var out model.SiteCoverageCache
	if err := db.Where("info_hash = ? AND site_name = ?", "secondhash", "secondsite").
		First(&out).Error; err != nil {
		t.Fatal(err)
	}
	if out.Source != model.CoverageSourceTracker {
		t.Errorf("二次写入 Source=%q want tracker（后写覆盖同名语义——保持 iyuu=V-01 回归）", out.Source)
	}
}
