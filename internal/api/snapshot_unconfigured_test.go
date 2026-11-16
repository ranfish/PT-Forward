package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// §59.537 锚：批量获取「未配置种子」弹窗条目清单完成度边界——
// ①未完成种不显示 ②完成种显示 ③资源级已配置判定不受完成度影响
// （唯一挂 metadata 的副本回退 <100% 时·资源仍算已配置不误现身未配置）。
func TestSnapshotUnconfiguredProgressBoundary(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&model.TorrentSnapshot{}, &model.TorrentMetadata{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	h := &DownloadHandler{db: db}
	snaps := []model.TorrentSnapshot{
		// 完成·无 metadata → 应显示
		{Hash: "donehash00000000000000000000000000000000", ClientUID: 1, SavePath: "/v", Name: "done.2023", Progress: 1},
		// 未完成·无 metadata → 不显示（§59.537 边界）
		{Hash: "parthash00000000000000000000000000000000", ClientUID: 1, SavePath: "/v", Name: "partial.2023", Progress: 0.4},
		// 完成·挂 metadata（资源已配置）→ 不显示
		{Hash: "cfghash000000000000000000000000000000000", ClientUID: 1, SavePath: "/v", Name: "configured.2023", Progress: 1},
		// ③资源级：同 name 变体——挂 metadata 的副本完成度回退·资源仍已配置
		{Hash: "reghash00000000000000000000000000000000", ClientUID: 1, SavePath: "/v", Name: "regressed.2023", Progress: 0.2},
	}
	for i := range snaps {
		if err := db.Create(&snaps[i]).Error; err != nil {
			t.Fatalf("seed snap: %v", err)
		}
	}
	metas := []model.TorrentMetadata{
		{InfoHash: "cfghash000000000000000000000000000000000", SiteName: "s1", TorrentID: "1"},
		{InfoHash: "reghash00000000000000000000000000000000", SiteName: "s1", TorrentID: "2"},
	}
	for i := range metas {
		if err := db.Create(&metas[i]).Error; err != nil {
			t.Fatalf("seed meta: %v", err)
		}
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/downloads/snapshot-unconfigured?client_id=1&save_path=/v", nil)
	h.handleSnapshotUnconfigured(w, r)
	if w.Code != 200 {
		t.Fatalf("status: %d", w.Code)
	}
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
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Data.Total != 1 || len(resp.Data.Items) != 1 {
		t.Fatalf("expect only done row, got total=%d items=%+v", resp.Data.Total, resp.Data.Items)
	}
	if resp.Data.Items[0].Name != "done.2023" {
		t.Fatalf("expect done.2023, got %s", resp.Data.Items[0].Name)
	}
}
