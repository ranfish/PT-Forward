package metadata

import (
	"context"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// 17 刀2（v0.0.1668）: persistCoverageHits 刷新锚（V-02 防回归）——
// 同指针+.Error 丢弃双缺陷。修复后断言：已存在行刷新为新值（皇后 tid
// 缓存化）+错误路径日志可见（暂不锚日志——刷新语义为主张）。
//
// 阴性对照：退回同指针 → 断言新值落库 FAIL。
func TestPersistCoverageHits_RefreshOnExisting(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:persist_hits_anchor?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.SiteCoverageCache{}); err != nil {
		t.Fatal(err)
	}
	db.Where("1=1").Delete(&model.SiteCoverageCache{})

	f := &Fetcher{db: db, logger: zap.NewNop()}
	ctx := context.Background()

	// 旧行（旧 tid）
	f.db.Create(&model.SiteCoverageCache{
		InfoHash: "hitshash", SiteName: "hitsite",
		Status: model.CoverageConfirmedHas, Source: model.CoverageSourceIYUU,
		TorrentID: "old-tid", Confidence: 1.0,
		QueriedAt: time.Unix(1000, 0), ExpiresAt: time.Unix(1000, 0),
	})

	// 新命中（新 tid）——persistCoverageHits 应刷新
	f.persistCoverageHits(ctx, "hitshash", []model.CoverageHit{
		{SiteName: "hitsite", TorrentID: "new-tid"},
	})

	var out model.SiteCoverageCache
	if err := f.db.Where("info_hash = ? AND site_name = ?", "hitshash", "hitsite").
		First(&out).Error; err != nil {
		t.Fatal(err)
	}
	if out.TorrentID != "new-tid" {
		t.Errorf("TorrentID=%q want new-tid（刷新失败=同指针别名回归——V-02）", out.TorrentID)
	}
	if out.QueriedAt.Unix() <= 1000 {
		t.Errorf("QueriedAt=%d 应刷新为当前时间（保持 1000=刷新失效）", out.QueriedAt.Unix())
	}
}
