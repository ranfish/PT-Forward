package orphan

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// crossSeedFakeClient §59.359: 只实现 GetTorrentByHash 的最小 fake——
// 嵌入接口未用方法零成本。
type crossSeedFakeClient struct {
	model.DownloaderClient
	ti *model.TorrentInfo
}

func (c *crossSeedFakeClient) GetTorrentByHash(ctx context.Context, hash string) (*model.TorrentInfo, error) {
	if c.ti == nil {
		return nil, os.ErrNotExist
	}
	return c.ti, nil
}

type crossSeedProvider struct {
	client model.DownloaderClient
}

func (p *crossSeedProvider) Get(clientUID uint) (model.DownloaderClient, error) {
	if p.client == nil {
		return nil, os.ErrNotExist
	}
	return p.client, nil
}
func (p *crossSeedProvider) ListClients() []uint { return nil }

func newCrossSeedTestRecovery(t *testing.T) (*Recovery, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "xseed.db")), &gorm.Config{})
	if err != nil {
		t.Skip("sqlite unavailable:", err)
	}
	if err := db.AutoMigrate(&model.OrphanCrossSeedRecord{}); err != nil {
		t.Fatal(err)
	}
	r := &Recovery{db: db, clientProvider: &crossSeedProvider{}, logger: zap.NewNop()}
	return r, db
}

// TestCrossSeedCheck §59.359 三态锚：
// ① 已有+路径不符 → 仲裁错误+落记录（下次扫描可提示）
// ② 已有+路径相同 → 幂等放行（重复恢复场景，§59.203 语义保留）
// ③ 无条目 → 放行（正常恢复）
// ④ 尾斜杠归一化：tr 返回 SavePath 常带尾斜杠，不归一化会误判路径不符。
func TestCrossSeedCheck(t *testing.T) {
	r, db := newCrossSeedTestRecovery(t)
	orphan := &Entry{Path: "/PT7/FRDS/coll/Movie.2020", SavePath: "/PT7/FRDS/coll"}

	// ① 已有+路径不符 → 短路+落记录
	r.clientProvider = &crossSeedProvider{client: &crossSeedFakeClient{
		ti: &model.TorrentInfo{Hash: "abc123", SavePath: "/PT6/other", Name: "Movie.2020"},
	}}
	xerr := r.crossSeedCheck(context.Background(), "abc123", orphan, 1, "/PT7/FRDS/coll")
	if xerr == nil {
		t.Fatal("跨目录做种应短路返回 CrossSeedError")
	}
	if xerr.SeededPath != "/PT6/other" {
		t.Fatalf("SeededPath=%q want /PT6/other", xerr.SeededPath)
	}
	var cnt int64
	db.Model(&model.OrphanCrossSeedRecord{}).Count(&cnt)
	if cnt != 1 {
		t.Fatalf("记录数=%d want 1（仲裁结果应落表）", cnt)
	}

	// ② 已有+路径相同（含尾斜杠归一化形态）→ 幂等放行
	r.clientProvider = &crossSeedProvider{client: &crossSeedFakeClient{
		ti: &model.TorrentInfo{Hash: "abc123", SavePath: "/PT7/FRDS/coll/", Name: "Movie.2020"},
	}}
	if xerr := r.crossSeedCheck(context.Background(), "abc123", orphan, 1, "/PT7/FRDS/coll"); xerr != nil {
		t.Fatalf("同路径已有条目应幂等放行，got %v", xerr)
	}

	// ③ 无条目 → 放行
	r.clientProvider = &crossSeedProvider{client: &crossSeedFakeClient{}}
	if xerr := r.crossSeedCheck(context.Background(), "abc123", orphan, 1, "/PT7/FRDS/coll"); xerr != nil {
		t.Fatalf("无已有条目应放行，got %v", xerr)
	}
}

// TestCrossSeedErrorAsWrap §59.359: downloadWithFallback 的 wrap 链
// （fmt.Errorf %w）errors.As 可穿透——短路判定依赖此性质。
func TestCrossSeedErrorAsWrap(t *testing.T) {
	wrapped := fmt.Errorf("site=luckpt tid=123: %w", &CrossSeedError{InfoHash: "abc", SeededPath: "/PT6/x"})
	var xerr *CrossSeedError
	if !errors.As(wrapped, &xerr) {
		t.Fatal("wrap 链 errors.As 应穿透")
	}
	if xerr.SeededPath != "/PT6/x" {
		t.Fatalf("SeededPath=%q want /PT6/x", xerr.SeededPath)
	}
}

// TestMarkCrossSeeded §59.359 扫描侧锚：
// ① 记录命中（path+client 匹配）→ Entry 标 CrossSeeded+SeededPaths
// ② client 不匹配 → 不标
// ③ 失效记录清理：参与扫描的 client 中 path 已不在孤儿列表 → 删
// ④ 未参与扫描 client 的记录不清理（防误删）。
func TestMarkCrossSeeded(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "m.db")), &gorm.Config{})
	if err != nil {
		t.Skip("sqlite unavailable:", err)
	}
	if err := db.AutoMigrate(&model.OrphanCrossSeedRecord{}); err != nil {
		t.Fatal(err)
	}
	s := &Scanner{db: db, logger: zap.NewNop()}

	db.Create(&model.OrphanCrossSeedRecord{ClientUID: 1, OrphanPath: "/p/a", InfoHash: "h1", SeededPath: "/other/a"})
	db.Create(&model.OrphanCrossSeedRecord{ClientUID: 9, OrphanPath: "/p/b", InfoHash: "h2", SeededPath: "/other/b"}) // client 未匹配
	db.Create(&model.OrphanCrossSeedRecord{ClientUID: 1, OrphanPath: "/p/gone", InfoHash: "h3", SeededPath: "/other/c"}) // 数据已删

	orphans := []Entry{
		{Path: "/p/a", ClientUIDs: []uint{1}},
		{Path: "/p/b", ClientUIDs: []uint{2}}, // 记录 client=9 ∉ ClientUIDs
	}
	scanned := map[uint]map[string][]string{1: {}}
	s.markCrossSeeded(orphans, scanned)

	if !orphans[0].CrossSeeded || len(orphans[0].SeededPaths) != 1 || orphans[0].SeededPaths[0] != "/other/a" {
		t.Fatalf("① 命中记录应标记: %+v", orphans[0])
	}
	if orphans[1].CrossSeeded {
		t.Fatal("② client 不匹配不应标记")
	}

	var cnt int64
	db.Model(&model.OrphanCrossSeedRecord{}).Where("orphan_path = ?", "/p/gone").Count(&cnt)
	if cnt != 0 {
		t.Fatal("③ 失效记录（path 已不在孤儿列表）应清理")
	}
	db.Model(&model.OrphanCrossSeedRecord{}).Where("orphan_path = ?", "/p/a").Count(&cnt)
	if cnt != 1 {
		t.Fatal("命中记录应保留")
	}
}
