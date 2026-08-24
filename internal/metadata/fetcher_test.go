package metadata

import (
	"context"
	"testing"

	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/ranfish/pt-forward/internal/model"
)

func noFBDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.TorrentMetadata{}, &model.SiteCoverageCache{}); err != nil {
		t.Fatal(err)
	}
	return db
}

// §59.65: FetchFromSiteNoFallback 直取——失败必须报错，不得内部 IYUU 兜底
//（老 FetchAndStore 内嵌 fetchWithIYUUFallback，藏在 FetchAndStoreDirect 委托里
// 抢跑 §59.61 降级链——The.Boys 实锤: 朋友抖动 → 克隆 iyuu_cache 覆盖源站语义）。
func TestFetchFromSiteNoFallback_FailsWithoutIYUU(t *testing.T) {
	if testing.Short() {
		t.Skip("需要 DB")
	}
	// 构造: 无 siteProvider 站点 → fetchFromSite 报错 → 必须原样返回错误（nil meta）
	f := NewFetcher(noFBDB(t), zap.NewNop(), nil)
	meta, err := f.FetchFromSiteNoFallback(context.Background(), "aaaa000000000000000000000000000000000000", "朋友", "2782026")
	if err == nil || meta != nil {
		t.Fatalf("直取失败应报错且无兜底: meta=%v err=%v", meta, err)
	}
}
