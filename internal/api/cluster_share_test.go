package api

import (
	"context"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func clusterTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	return db
}

// §59.61 第 4 步: 簇传播——获取成功后站点数据+MI 复制到无数据副本行
func TestPropagateClusterMetadata(t *testing.T) {
	db := clusterTestDB(t)
	h := &PublishTorrentsHandler{db: db, logger: zap.NewNop()}
	// 簇: self + 2 siblings（快照）
	for _, hh := range []string{"selfhash0000000000000000000000000000000000000", "sib10000000000000000000000000000000000000", "sib20000000000000000000000000000000000000"} {
		db.Create(&model.TorrentSnapshot{Hash: hh, ClientID: "PT0", Name: "Arco.2025", SavePath: "/x"})
	}
	// self 完整 metadata
	db.Create(&model.TorrentMetadata{InfoHash: "selfhash0000000000000000000000000000000000000", SiteName: "朋友", TorrentID: "123", Title: "Arco", Description: "d", Poster: "p", MediaInfo: "mi", MediaInfoSource: "local"})
	// sibling1 已有自己的行（不覆盖）；sibling2 无行（应复制）
	db.Create(&model.TorrentMetadata{InfoHash: "sib10000000000000000000000000000000000000", SiteName: "财神", Title: "existing"})

	h.propagateClusterMetadata(context.Background(), "PT0", "/x", "Arco.2025",
		"selfhash0000000000000000000000000000000000000", "朋友")

	var cpy model.TorrentMetadata
	if err := db.Where("info_hash = ?", "sib20000000000000000000000000000000000000").First(&cpy).Error; err != nil {
		t.Fatalf("sibling2 应有复制行: %v", err)
	}
	if cpy.SiteName != "朋友" || cpy.TorrentID != "123" || cpy.MediaInfo != "mi" || cpy.MediaInfoSource != "local" {
		t.Errorf("复制内容不完整: %+v", cpy)
	}
	if cpy.FetchSource != "cluster" {
		t.Errorf("fetch_source 应为 cluster: %q", cpy.FetchSource)
	}
	var sib1 model.TorrentMetadata
	db.Where("info_hash = ?", "sib10000000000000000000000000000000000000").First(&sib1)
	if sib1.SiteName != "财神" {
		t.Errorf("已有行不应被覆盖: %+v", sib1)
	}
}

// §59.61 第 4 步: 批内跳过——簇内已有完整数据（含传播行）的副本跳过获取
func TestHasCompleteMetadata(t *testing.T) {
	db := clusterTestDB(t)
	h := &PublishTorrentsHandler{db: db, logger: zap.NewNop()}
	empty := "e000000000000000000000000000000000000000"
	full := "f000000000000000000000000000000000000000"
	if h.hasCompleteMetadata(context.Background(), empty) {
		t.Error("无行应返回 false")
	}
	db.Create(&model.TorrentMetadata{InfoHash: full, SiteName: "朋友", Title: "t", Description: "d"})
	if !h.hasCompleteMetadata(context.Background(), full) {
		t.Error("完整行应返回 true")
	}
}

// §59.61 第 4 步: 截图二次传播——策略完成后补簇内空截图行
func TestPropagateClusterScreenshots(t *testing.T) {
	db := clusterTestDB(t)
	h := &PublishTorrentsHandler{db: db, logger: zap.NewNop()}
	db.Create(&model.TorrentSnapshot{Hash: "shotself0000000000000000000000000000000000", ClientID: "PT0", Name: "N", SavePath: "/s"})
	db.Create(&model.TorrentSnapshot{Hash: "shotsib00000000000000000000000000000000000", ClientID: "PT0", Name: "N", SavePath: "/s"})
	db.Create(&model.TorrentMetadata{InfoHash: "shotself0000000000000000000000000000000000", SiteName: "朋友", Title: "t", Screenshots: `["https://a/1.jpg"]`})
	db.Create(&model.TorrentMetadata{InfoHash: "shotsib00000000000000000000000000000000000", SiteName: "朋友", Title: "t", Screenshots: ""})
	// 已有截图的第三方行（不同簇）不应被写
	db.Create(&model.TorrentMetadata{InfoHash: "other0000000000000000000000000000000000000", SiteName: "朋友", Title: "t", Screenshots: ""})

	h.propagateClusterScreenshots(context.Background(), "PT0", "/s", "N",
		"shotself0000000000000000000000000000000000", `["https://a/1.jpg"]`)

	var sib model.TorrentMetadata
	db.Where("info_hash = ?", "shotsib00000000000000000000000000000000000").First(&sib)
	if sib.Screenshots != `["https://a/1.jpg"]` {
		t.Errorf("簇内空行应补截图: %q", sib.Screenshots)
	}
	var other model.TorrentMetadata
	db.Where("info_hash = ?", "other0000000000000000000000000000000000000").First(&other)
	if other.Screenshots != "" {
		t.Error("非簇行不应被写")
	}
}
