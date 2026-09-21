package metadata

import (
	"context"
	"testing"

	"github.com/ranfish/pt-forward/internal/fingerprint"
	"github.com/ranfish/pt-forward/internal/mocks"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/setting"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestHashVerify_ThreeStates(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	_ = db.AutoMigrate(&model.TorrentMetadata{}, &model.Site{})
	repo := setting.NewRepository(db)

	// 代码内构造合法 .torrent（BuildSingleFileTorrent bencode 自洽——CI 无 examples/
	// 目录，手写字节串 announce 长度差 1 即解析错乱 §59.252 教训）
	pieces := make([]byte, 200)
	for i := range pieces {
		pieces[i] = byte(i)
	}
	torrentData, err := fingerprint.BuildSingleFileTorrent(
		"http://tracker.example.com/announce", "Big.Movie.2025", 10737418240, 4194304, pieces)
	if err != nil {
		t.Fatalf("构造 .torrent 失败: %v", err)
	}
	tm, err := fingerprint.ComputeFromTorrent(torrentData)
	if err != nil {
		t.Fatalf(".torrent 解析失败: %v", err)
	}
	if tm.Name == "" || tm.InfoHash == "" {
		t.Fatalf("解析结果不全: %+v", tm)
	}

	// mock adapter：下载返回该 .torrent
	adapter := &mocks.SiteAdapter{
		DownloadTorrentFn: func(_ context.Context, _ *model.SiteConfig, _ string) ([]byte, error) {
			return torrentData, nil
		},
	}
	sp := &mocks.SiteInfoProvider{
		GetAdapterFn: func(_ context.Context, _ string) (model.SiteAdapter, error) { return adapter, nil },
		GetSiteConfigFn: func(_ context.Context, _ string) (*model.SiteConfig, error) {
			return &model.SiteConfig{}, nil
		},
	}
	f := NewFetcher(db, zap.NewNop(), sp, repo)

	// 状态①（D3 通过不触发）由上层分支保证——此处测终审函数本体：
	// 状态②：hash 相等 → 终审通过 + name 提取
	hv := f.hashVerifyDirect(context.Background(), "s", "1", tm.InfoHash)
	if hv == nil {
		t.Fatal("hash 相等应放行")
	}
	if hv.Name != tm.Name {
		t.Fatalf("原始名提取: %q != %q", hv.Name, tm.Name)
	}
	// 状态③：hash 不等 → nil（维持拒）
	if hv := f.hashVerifyDirect(context.Background(), "s", "1", "deadbeef"+tm.InfoHash[8:]); hv != nil {
		t.Fatal("hash 不等应维持拒")
	}
}
