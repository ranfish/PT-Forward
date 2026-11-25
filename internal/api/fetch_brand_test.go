package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// §59.578 刀2 锚一：烙印写点三边界（口径 A）——瞬态不烙/终态烙/
// 重获失败不烙（数据在手可发布）。音乐种烙印不豁免（音乐链失败同烙——
// 烙印判定在链尾·与路由无关）。
func TestMarkFetchFailed(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:fetchbrand_"+t.Name()+".db?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&model.TorrentSnapshot{}, &model.TorrentMetadata{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	h := &PublishTorrentsHandler{db: db, logger: zap.NewNop()}
	db.Create(&model.TorrentSnapshot{Hash: "brand1", ClientUID: 2, Name: "N1", SavePath: "/p"})

	// ① 终态失败 → 烙
	h.markFetchFailed(context.Background(), 2, "brand1", true, errors.New("无可用源站（制作组未映射 + 无覆盖）"))
	var snap1 model.TorrentSnapshot
	db.Where("hash = ?", "brand1").First(&snap1)
	if snap1.FetchFailedAt == nil || snap1.FetchError == "" {
		t.Fatalf("① 终态失败未烙印: at=%v err=%q", snap1.FetchFailedAt, snap1.FetchError)
	}
	if snap1.FetchError != "无可用源站（制作组未映射 + 无覆盖）" {
		t.Errorf("① 烙印错误串不保真: %q", snap1.FetchError)
	}

	// ② 瞬态失败 → 不烙（清印后验证）
	h.clearFetchBrandByHash(context.Background(), "brand1")
	h.markFetchFailed(context.Background(), 2, "brand1", true, errors.New("Get x: context deadline exceeded"))
	var snap2 model.TorrentSnapshot
	db.Where("hash = ?", "brand1").First(&snap2)
	if snap2.FetchFailedAt != nil {
		t.Errorf("② 瞬态失败不应烙印: at=%v", snap2.FetchFailedAt)
	}

	// ②b 瞬态扩四形态（§59.578 刀2: refused/DNS/reset/EOF）
	for _, te := range []string{
		"dial tcp 1.2.3.4:443: connect: connection refused",
		"dial tcp: lookup site.example: no such host",
		"read: connection reset by peer",
		"unexpected EOF",
	} {
		h.markFetchFailed(context.Background(), 2, "brand1", true, errors.New(te))
		var snapB model.TorrentSnapshot
		db.Where("hash = ?", "brand1").First(&snapB)
		if snapB.FetchFailedAt != nil {
			t.Errorf("②b 瞬态扩形态不应烙印（%s）: at=%v", te, snapB.FetchFailedAt)
		}
	}

	// ③ 重获失败（fresh=false）→ 不烙
	h.markFetchFailed(context.Background(), 2, "brand1", false, errors.New("详情页解析失败: 站方改版"))
	var snap3 model.TorrentSnapshot
	db.Where("hash = ?", "brand1").First(&snap3)
	if snap3.FetchFailedAt != nil {
		t.Errorf("③ 重获失败不应烙印（数据在手）: at=%v", snap3.FetchFailedAt)
	}
}

// §59.578 刀2 锚二：弹窗排除——任一 hash 烙印 → 整 name 排除
// （走过获取链的下次不再显示·瞬态不烙故可重试颗保留）。
func TestSnapshotUnconfiguredBrandExclusion(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:fetchbrand2_"+t.Name()+".db?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&model.TorrentSnapshot{}, &model.TorrentMetadata{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	h := &DownloadHandler{db: db}
	now := time.Now()
	seed := []model.TorrentSnapshot{
		// sel：未获取无烙印 → 显示
		{Hash: "sel1", ClientUID: 2, Name: "Movie A 2020 1080p-GOODGRP", SavePath: "/p", Progress: 1.0},
		// skip-印：同 name 另一 hash 烙印 → 整 name 排除（任一语义）
		{Hash: "skipa", ClientUID: 2, Name: "Movie B", SavePath: "/p", Progress: 1.0},
		{Hash: "skipb", ClientUID: 2, Name: "Movie B", SavePath: "/p", Progress: 1.0, FetchFailedAt: &now, FetchError: "无可用源站"},
		// skip-瞬态：烙印为 NULL（瞬态不烙）→ name 保留显示（可重试）
		{Hash: "sel2", ClientUID: 2, Name: "Movie C", SavePath: "/p", Progress: 1.0},
	}
	for i := range seed {
		if err := db.Create(&seed[i]).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/downloads/snapshot-unconfigured?client_id=2&save_path=/p", nil)
	h.handleSnapshotUnconfigured(w, r)
	var resp struct {
		Data struct {
			Items []struct {
				Hash string `json:"hash"`
				Name string `json:"name"`
			} `json:"items"`
			Total int `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse: %v", err)
	}
	got := map[string]bool{}
	for _, it := range resp.Data.Items {
		got[it.Name] = true
	}
	if !got["Movie A 2020 1080p-GOODGRP"] || !got["Movie C"] {
		t.Errorf("未烙印 name 应显示: %v", got)
	}
	if got["Movie B"] {
		t.Errorf("烙印 name（任一 hash）应整 name 排除: %v", got)
	}
}

// §59.578 刀2 锚三：清除回归——deleteSeed 同步清烙印（「清除过种子信息
// 的种子可以回到获取未配置」闭环接线——只删 metadata 不清印=通道断）。
func TestDeleteSeedClearsBrand(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:fetchbrand3_"+t.Name()+".db?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&model.TorrentSnapshot{}, &model.TorrentMetadata{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	h := &PublishTorrentsHandler{db: db, logger: zap.NewNop()}
	now := time.Now()
	// 误烙颗（无 metadata+烙印）——清除是它唯一恢复路径
	db.Create(&model.TorrentSnapshot{Hash: "rec1", ClientUID: 2, Name: "Movie D", SavePath: "/p", FetchFailedAt: &now, FetchError: "详情页解析失败"})
	// 同簇第二 hash（簇级全清验证）
	db.Create(&model.TorrentSnapshot{Hash: "rec2", ClientUID: 2, Name: "Movie D", SavePath: "/p", FetchFailedAt: &now, FetchError: "无可用源站"})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("DELETE", "/api/v1/publish/seeds/rec1", nil)
	h.handleDeleteSeed(w, r)

	var count int64
	db.Model(&model.TorrentSnapshot{}).Where("name = ? AND fetch_failed_at IS NOT NULL", "Movie D").Count(&count)
	if count != 0 {
		t.Errorf("清除后簇级烙印应全清（残留 %d 行）——弹窗重现通道断", count)
	}
}

// §59.578 刀2 锚四：成功清印——获取成功=走过且成功的语义终结（残留
// 烙印会让「清除 metadata→弹窗重现」通道断裂——force 重取成功场景）。
func TestClearFetchBrand(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:fetchbrand4_"+t.Name()+".db?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&model.TorrentSnapshot{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	h := &PublishTorrentsHandler{db: db, logger: zap.NewNop()}
	now := time.Now()
	// 同簇两 hash 烙印+异簇同名（跨路径·§59.316 附 D 紧键域不连坐）
	db.Create(&model.TorrentSnapshot{Hash: "ok1", ClientUID: 2, Name: "Movie E", SavePath: "/p1", FetchFailedAt: &now, FetchError: "x"})
	db.Create(&model.TorrentSnapshot{Hash: "ok2", ClientUID: 2, Name: "Movie E", SavePath: "/p1", FetchFailedAt: &now, FetchError: "x"})
	db.Create(&model.TorrentSnapshot{Hash: "ok3", ClientUID: 2, Name: "Movie E", SavePath: "/p2", FetchFailedAt: &now, FetchError: "x"})

	h.clearFetchBrand(context.Background(), 2, "/p1", "Movie E")

	var remain int64
	db.Model(&model.TorrentSnapshot{}).Where("save_path = ? AND fetch_failed_at IS NOT NULL", "/p1").Count(&remain)
	if remain != 0 {
		t.Errorf("同簇烙印应全清（残留 %d）", remain)
	}
	var other int64
	db.Model(&model.TorrentSnapshot{}).Where("save_path = ? AND fetch_failed_at IS NOT NULL", "/p2").Count(&other)
	if other != 1 {
		t.Errorf("异簇同名不应连坐（应保留 1·实 %d）——§59.316 附 D 紧键域", other)
	}
}
