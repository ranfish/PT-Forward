package metadata

import (
	"github.com/ranfish/pt-forward/internal/util"
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

// §59.99/§59.136: 站点运营标记过滤——副标题/标题尾部 "[中性种子(NL)]"（朋友站零魔力
// 标识, NBSP 前缀——与 §59.64 标题侧 [2X 50%] 同族）、百分比/返利族 "[50%]" "[30%]" "[75%]"
// 不属于内容, 转发无意义。混排 "[50%] [禁转]" 只剥运营标记, [禁转] 是合规判据保留。
func TestStripSiteOperationMarkers(t *testing.T) {
	cases := []struct{ in, want string }{
		{"【醉拳2】mUHD作品 4k 杜比视界版本\u00a0\u00a0\u00a0 [中性种子(NL)]", "【醉拳2】mUHD作品 4k 杜比视界版本"},
		{"正常副标题", "正常副标题"},
		{"【名】简介 简繁字幕 带章节名    [中性种子(NL)]", "【名】简介 简繁字幕 带章节名"},
		{"标题 [中性种子(NL)]", "标题"},
		// §59.136: 百分比/返利族
		{"危情24小时.Butterfly.On.A.Wheel.2007     [50%]", "危情24小时.Butterfly.On.A.Wheel.2007"},
		{"副标题 [30%]", "副标题"},
		{"副标题 [75%]", "副标题"},
		{"副标题\u00a0\u00a0 [2X 50%]", "副标题"},
		{"副标题 [Free 50%]", "副标题"},
		// §59.136: 混排——运营标记剥除, [禁转] 保留
		{"简繁双语特效四字幕\u00a0\u00a0\u00a0 [中性种子(NL)] [禁转]", "简繁双语特效四字幕 [禁转]"},
		{"副标题 [50%] [禁转]", "副标题 [禁转]"},
		{"副标题 [50%] [中性种子(NL)] [谢绝转载]", "副标题 [谢绝转载]"},
		// [禁转] 单独存在: 不剥
		{"副标题 [禁转]", "副标题 [禁转]"},
	}
	for _, c := range cases {
		if got := util.StripSiteOperationMarkers(c.in); got != c.want {
			t.Errorf("strip(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
