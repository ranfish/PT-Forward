package api

// §59.541 附十 修4 回归锚：Dolby 键存量清洗（幂等三态）。
import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"

	"github.com/ranfish/pt-forward/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupDolbyFixDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.TorrentMetadata{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestHandleFixStaleDolbyTags(t *testing.T) {
	db := setupDolbyFixDB(t)
	h := &PublishTorrentsHandler{db: db, logger: zap.NewNop()}

	mk := func(hash, site, tid, tags, mi string) {
		db.Create(&model.TorrentMetadata{
			InfoHash: hash, SiteName: site, TorrentID: tid, Title: tid, Tags: tags, MediaInfo: mi,
		})
	}
	mk("hash-a", "站A", "1", `["chinese_subtitle","dolby_atmos"]`, "General\nFormat : Matroska\n")
	mk("hash-b", "站B", "2", `["dolby_vision"]`, "HDR format : dvhe.07\n")
	mk("hash-c", "站C", "3", `["dolby_atmos"]`, "Commercial name : Dolby Digital Atmos\n")
	mk("hash-d", "站D", "4", `["dolby_atmos","dolby_vision","high_rating"]`, "")

	// ① 首次清洗
	req := httptest.NewRequest(http.MethodPost, "/api/v1/publish/seeds/fix-stale-dolby", nil)
	rec := httptest.NewRecorder()
	h.handleFixStaleDolbyTags(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("① HTTP %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Data struct {
			Cleaned int `json:"cleaned"`
			Scanned int `json:"scanned"`
		} `json:"data"`
	}
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Data.Scanned != 4 {
		t.Fatalf("① scanned 应 4，got %d", resp.Data.Scanned)
	}
	if resp.Data.Cleaned != 2 { // A+D 两行清洗
		t.Fatalf("① cleaned 应 2（A+D），got %d", resp.Data.Cleaned)
	}

	// 行断言
	get := func(hash string) model.TorrentMetadata {
		var m model.TorrentMetadata
		db.Where("info_hash = ?", hash).First(&m)
		return m
	}
	var tagsA []string
	json.Unmarshal([]byte(get("hash-a").Tags), &tagsA)
	if len(tagsA) != 1 || tagsA[0] != "chinese_subtitle" {
		t.Fatalf("A 行应只留 chinese_subtitle，got %v", tagsA)
	}
	if get("hash-b").Tags != `["dolby_vision"]` {
		t.Fatalf("B 行真键不应动，got %s", get("hash-b").Tags)
	}
	if get("hash-c").Tags != `["dolby_atmos"]` {
		t.Fatalf("C 行真键不应动，got %s", get("hash-c").Tags)
	}
	var tagsD []string
	json.Unmarshal([]byte(get("hash-d").Tags), &tagsD)
	if len(tagsD) != 1 || tagsD[0] != "high_rating" {
		t.Fatalf("D 行应只留 high_rating，got %v", tagsD)
	}

	// ② 幂等：二次调用 cleaned=0
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/publish/seeds/fix-stale-dolby", nil)
	rec2 := httptest.NewRecorder()
	h.handleFixStaleDolbyTags(rec2, req2)
	var resp2 struct {
		Data struct {
			Cleaned int `json:"cleaned"`
		} `json:"data"`
	}
	json.NewDecoder(bytes.NewReader(rec2.Body.Bytes())).Decode(&resp2)
	if resp2.Data.Cleaned != 0 {
		t.Fatalf("② 二次调用应清洗 0 行（幂等），got %d", resp2.Data.Cleaned)
	}
}
