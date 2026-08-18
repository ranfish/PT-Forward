package metadata

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/adapter"
	"github.com/ranfish/pt-forward/internal/metadata/extract"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/site"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// §59.36 仲裁①路径真实链路验证（29 开发环境，2026-08-18）
//
// 背景：29 无本地文件下载器（PT0=243 远程 TR），localMI 用站内真实 MI 文本
// 模拟（Hail Mary 详情页 Audio #1: TrueHD Atmos——MI 实证内容物真身）。
// 链路其余环节全部真实：真实搜索朋友站 → 音频 token 冲突 → 现场抓候选
// 详情页 MI → 仲裁比对。
//
// 验证两条分支：
//   A. 源 MI=TrueHD（真身）→ 仲裁一致 → 放行 tid=2782301
//   B. 源 MI=DTS-HD MA（站内唯一候选 MI=TrueHD ≠ 源 DTS）→ 确定性出局
//      → 不进 loose → 失败

const (
	hailMaryName = "挽救计划.Project.Hail.Mary.2026.UHD.BluRay.2160p.x265.DV.HDR10plus.DTS-HD.MA.5.1.mUHD-FRDS"
	hailMaryHash = "a02a53bf5330b98a838a2a1a4915db5e408fb8c2"
	hailMarySize = int64(35057700630)

	// 站内真实 MI（详情页 Audio #1 片段，内容物真身 = TrueHD Atmos）
	miTrueHD = "Audio #1\nID : 2\nFormat : MLP FBA 16-ch\nFormat/Info : Meridian Lossless Packing FBA with 16-channel presentation\nCommercial Name : Dolby TrueHD with Dolby Atmos\nLanguage : English\n"
	// 模拟 DTS-HD MA 源（站内候选 MI 与之不一致 → 应确定性出局）
	miDTSHDMA = "Audio #1\nFormat : DTS XLL\nCommercial Name : DTS-HD Master Audio\nLanguage : English\n"
)

func newRealFetcher(t *testing.T) (*Fetcher, *gorm.DB) {
	// 真实站依赖（keepfrds 搜索响应波动敏感）：显式开启才跑，
	// 29 手动验证 / 定向 CI；全量 go test ./... 免受站点限流 flaky 影响
	if os.Getenv("PTF_E2E_REAL_SITE") == "" {
		t.Skip("真实站 E2E：设置 PTF_E2E_REAL_SITE=1 开启")
	}
	if _, err := os.Stat("/home/incast/PT-Forward/data/pt-forward.db"); err != nil {
		t.Skip("非 29 环境")
	}
	db, err := gorm.Open(sqlite.Open("/home/incast/PT-Forward/data/pt-forward.db"), &gorm.Config{})
	if err != nil {
		t.Skipf("无 29 DB: %v", err)
	}
	logger := zap.NewNop()
	publicExtractor := extract.NewPublicExtractor("", "")
	engine := extract.NewEngine(publicExtractor, nil)
	factory := adapter.NewFactory(logger, engine)
	provider := site.NewProvider(db, factory, logger)
	return NewFetcher(db, logger, provider), db
}

func TestArbitrationPathA_TrueHDMatch(t *testing.T) {
	f, db := newRealFetcher(t)
	db.Where("title LIKE ?", "%Hail Mary%").Delete(&model.TorrentMetadata{})

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	meta, err := f.FetchAndStoreBySearch(ctx, hailMaryHash, "朋友", hailMaryName, hailMarySize, miTrueHD)
	if err != nil {
		t.Fatalf("分支A（MI 一致应放行）: %v", err)
	}
	if meta == nil || meta.TorrentID != "2782301" {
		t.Errorf("分支A: 应仲裁命中 2782301, got %+v", meta)
	}
	t.Logf("分支A PASS: tid=%s", meta.TorrentID)
}

func TestArbitrationPathB_MismatchOut(t *testing.T) {
	f, db := newRealFetcher(t)
	db.Where("title LIKE ?", "%Hail Mary%").Delete(&model.TorrentMetadata{})

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	_, err := f.FetchAndStoreBySearch(ctx, hailMaryHash, "朋友", hailMaryName, hailMarySize, miDTSHDMA)
	if err == nil {
		t.Fatal("分支B（源 DTS-HD MA vs 候选 TrueHD → 确定性出局）应失败，却成功了")
	}
	t.Logf("分支B PASS（确定性出局，不进 loose）: %v", err)
}

// 补充说明（验证后记录）：
// 分支A耗时 ~7s（搜索 + 仲裁抓详情页 + 正常抓详情入库）
// 分支B耗时 ~9s（搜索 + 仲裁抓详情页 + MI 不一致出局 + 不进 loose 直接失败）
// 两条分支均真实请求朋友站（29 开发环境 cookie）
