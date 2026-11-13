package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// §59.516 v0.0.1462 锚：source_description JS 残留清洗——
// 含 JS 行清空·干净行零触碰·幂等（二轮 changed=0）。
func TestFixSourceDescription(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&model.TorrentMetadata{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	h := &PublishTorrentsHandler{db: db}
	rows := []model.TorrentMetadata{
		{InfoHash: "js1", SourceDescription: "[img]test[/img]\nfunction displayRelay(id) {\n jQuery.get('viewpeerlist.php')"},
		{InfoHash: "js2", SourceDescription: "jQuery(function() { initDLTable(); });"},
		{InfoHash: "clean", SourceDescription: "[img]https://example.com/poster.jpg[/img]\n◎片名 测试"},
		{InfoHash: "empty", SourceDescription: ""},
	}
	for i := range rows {
		if err := db.Create(&rows[i]).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	// 一轮清洗
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/publish/seeds/fix-source-description", nil)
	h.handleFixSourceDescription(w, r)
	var resp struct {
		Data struct {
			Total   int `json:"total"`
			Changed int `json:"changed"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("resp: %v", err)
	}
	if resp.Data.Total != 2 || resp.Data.Changed != 2 {
		t.Errorf("一轮: total=%d changed=%d（应 2/2）", resp.Data.Total, resp.Data.Changed)
	}
	// JS 行被清空
	var js1 model.TorrentMetadata
	db.Where("info_hash = ?", "js1").First(&js1)
	if js1.SourceDescription != "" {
		t.Errorf("js1 应清空")
	}
	// 干净行零触碰
	var clean model.TorrentMetadata
	db.Where("info_hash = ?", "clean").First(&clean)
	if clean.SourceDescription == "" {
		t.Error("clean 行不应被清空")
	}
	// 幂等：二轮 changed=0
	w2 := httptest.NewRecorder()
	h.handleFixSourceDescription(w2, r)
	var resp2 struct {
		Data struct {
			Total   int `json:"total"`
			Changed int `json:"changed"`
		} `json:"data"`
	}
	json.Unmarshal(w2.Body.Bytes(), &resp2)
	if resp2.Data.Total != 0 || resp2.Data.Changed != 0 {
		t.Errorf("幂等: total=%d changed=%d（应 0/0）", resp2.Data.Total, resp2.Data.Changed)
	}
}
