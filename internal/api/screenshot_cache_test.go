package api

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/ranfish/pt-forward/internal/model"
)

type mockShotStrategy struct {
	called int
	result []string
}

func (m *mockShotStrategy) ApplyScreenshotStrategy(ctx context.Context, name, savePath string, source []string, isLocal bool) []string {
	m.called++
	return m.result
}

// §59.63: 观察期内命中 → 跳过探活/策略/mpv 全链，直接复用缓存链接并传播
func TestScreenshotCache_HitSkipsStrategy(t *testing.T) {
	db := clusterTestDB(t)
	if err := db.AutoMigrate(&model.ClusterScreenshotCache{}); err != nil {
		t.Fatal(err)
	}
	mock := &mockShotStrategy{result: []string{"https://pixhost/NEW.jpg"}}
	h := &PublishTorrentsHandler{db: db, logger: zap.NewNop(), screenshotCacheDays: 30, shotStrategy: mock}

	cached := []string{"https://pixhost/C1.jpg", "https://pixhost/C2.jpg", "https://pixhost/C3.jpg"}
	data, _ := json.Marshal(cached)
	db.Create(&model.ClusterScreenshotCache{ClientID: "PT0", SavePath: "/p", Name: "N", Screenshots: string(data), UpdatedAt: time.Now()})

	db.Create(&model.TorrentSnapshot{Hash: "cself00000000000000000000000000000000", ClientID: "PT0", Name: "N", SavePath: "/p"})
	db.Create(&model.TorrentSnapshot{Hash: "csib000000000000000000000000000000000", ClientID: "PT0", Name: "N", SavePath: "/p"})
	db.Create(&model.TorrentMetadata{InfoHash: "cself00000000000000000000000000000000", SiteName: "朋友", Title: "t", Screenshots: "", FetchSource: "rss_detail"})
	db.Create(&model.TorrentMetadata{InfoHash: "csib000000000000000000000000000000000", SiteName: "朋友", Title: "t", Screenshots: "", FetchSource: "cluster"})

	h.applyScreenshotStrategy("PT0", "cself00000000000000000000000000000000", "朋友", "N", "/p", true)

	if mock.called != 0 {
		t.Errorf("缓存命中不应调用策略: called=%d", mock.called)
	}
	var self, sib model.TorrentMetadata
	db.Where("info_hash = ?", "cself00000000000000000000000000000000").First(&self)
	db.Where("info_hash = ?", "csib000000000000000000000000000000000").First(&sib)
	if len(model.ParseScreenshotColumn(self.Screenshots)) != 3 {
		t.Errorf("首副本应写入缓存链接: %q", self.Screenshots)
	}
	if len(model.ParseScreenshotColumn(sib.Screenshots)) != 3 {
		t.Errorf("簇行应被传播缓存链接: %q", sib.Screenshots)
	}
}

// §59.63: 观察期过期 → miss 走正常策略，成功后刷新缓存锚点与内容
func TestScreenshotCache_ExpiredMissRunsStrategy(t *testing.T) {
	db := clusterTestDB(t)
	if err := db.AutoMigrate(&model.ClusterScreenshotCache{}); err != nil {
		t.Fatal(err)
	}
	mock := &mockShotStrategy{result: []string{"https://pixhost/a.jpg", "https://pixhost/b.jpg", "https://pixhost/c.jpg"}}
	h := &PublishTorrentsHandler{db: db, logger: zap.NewNop(), screenshotCacheDays: 30, shotStrategy: mock}

	db.Create(&model.ClusterScreenshotCache{ClientID: "PT0", SavePath: "/p", Name: "N",
		Screenshots: `["https://pixhost/OLD.jpg"]`, UpdatedAt: time.Now().AddDate(0, 0, -31)})

	db.Create(&model.TorrentSnapshot{Hash: "dself00000000000000000000000000000000", ClientID: "PT0", Name: "N", SavePath: "/p"})
	db.Create(&model.TorrentMetadata{InfoHash: "dself00000000000000000000000000000000", SiteName: "朋友", Title: "t", Screenshots: "", FetchSource: "rss_detail"})

	h.applyScreenshotStrategy("PT0", "dself00000000000000000000000000000000", "朋友", "N", "/p", true)

	if mock.called != 1 {
		t.Fatalf("过期应走策略: called=%d", mock.called)
	}
	var row model.ClusterScreenshotCache
	db.Where("client_id = ? AND save_path = ? AND name = ?", "PT0", "/p", "N").First(&row)
	if len(model.ParseScreenshotColumn(row.Screenshots)) != 3 {
		t.Errorf("缓存应刷新为策略终态: %q", row.Screenshots)
	}
	if time.Since(row.UpdatedAt) > time.Minute {
		t.Errorf("缓存锚点应刷新: %v", row.UpdatedAt)
	}
}

// §59.63: same 早退（final==source）不写缓存——空值/无变化无意义
func TestScreenshotCache_SameExitNoWrite(t *testing.T) {
	db := clusterTestDB(t)
	if err := db.AutoMigrate(&model.ClusterScreenshotCache{}); err != nil {
		t.Fatal(err)
	}
	// 空源 + 策略空产出（全失败形态）→ same 早退（v0.0.703 warn 路径）不写缓存。
	// 注: 非空 same 场景需真实探活（purge 会 HEAD 假链清空 source），不在此测。
	mock := &mockShotStrategy{result: nil}
	h := &PublishTorrentsHandler{db: db, logger: zap.NewNop(), screenshotCacheDays: 30, shotStrategy: mock}

	db.Create(&model.TorrentSnapshot{Hash: "eself00000000000000000000000000000000", ClientID: "PT0", Name: "N", SavePath: "/p"})
	db.Create(&model.TorrentMetadata{InfoHash: "eself00000000000000000000000000000000", SiteName: "朋友", Title: "t", Screenshots: "", FetchSource: "rss_detail"})

	h.applyScreenshotStrategy("PT0", "eself00000000000000000000000000000000", "朋友", "N", "/p", true)

	var n int64
	db.Model(&model.ClusterScreenshotCache{}).Count(&n)
	if n != 0 {
		t.Errorf("same 早退不应写缓存: rows=%d", n)
	}
}
