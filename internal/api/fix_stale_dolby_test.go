package api

// §59.541 附十 修4（v3 推理同源判定）回归锚：幂等+真键保留+假键清洗。
import (
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
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.TorrentMetadata{}, &model.TorrentSnapshot{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestHandleFixStaleDolbyTags(t *testing.T) {
	db := setupDolbyFixDB(t)
	h := &PublishTorrentsHandler{db: db, logger: zap.NewNop()}

	mk := func(hash, site, tid, tags, mi, srcMI, bdinfo string) {
		db.Create(&model.TorrentMetadata{
			InfoHash: hash, SiteName: site, TorrentID: tid, Title: tid,
			Tags: tags, MediaInfo: mi, SourceMediaInfo: srcMI, BDInfo: bdinfo,
		})
	}
	// 行A：atmos 键×全通道无证据 → 清洗目标（文本残留）
	mk("hash-a", "站A", "1", `["chinese_subtitle","dolby_atmos"]`, "General\nFormat : Matroska\n", "", "")
	// 行B：vision 键×MI dvhe → 真键（MI 层推理认可）
	mk("hash-b", "站B", "2", `["dolby_vision"]`, "Video\nHDR format : dvhe.07\n", "", "")
	// 行C：atmos 键×MI Atmos 商业名 → 真键
	mk("hash-c", "站C", "3", `["dolby_atmos"]`, "Audio\nCommercial name : Dolby Digital Atmos\n", "", "")
	// 行D：双脏键×全通道无证据 → 都清
	mk("hash-d", "站D", "4", `["dolby_atmos","dolby_vision","high_rating"]`, "", "", "")
	// 行E（v2 锚）：local MI 空+source MI 有段结构 DV/Atmos → 真键
	// （真实形态：Video 段+HDR format 行——无段结构的散行推理不认）
	mk("hash-e", "站E", "5", `["dolby_atmos","dolby_vision"]`, "",
		"Video\nHDR format : Dolby Vision Profile 8\nFormat : HEVC\n\nAudio\nCommercial name : Dolby TrueHD with Dolby Atmos\n", "")
	// 行F（v3 锚·优堡 341301 形态）：MI 全空+BDInfo 的 Video 行含 DV → 真键
	// （extractBDInfoHDR 通道：MI 无 Video 段时 NFO 是 DoVi 铁证——真实
	// BDInfo 报告形态「* Video: ... Dolby Vision」）
	mk("hash-f", "站F", "6", `["dolby_vision","dolby_atmos"]`, "", "",
		"QUICK SUMMARY:\nDisc Title: They Will Kill You\n* Video: 2160p 23.976 fps 16:9 Dolby Vision HDR10 / HEVC\nAudio: English / Dolby TrueHD/Atmos Audio / 7.1")

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
	if resp.Data.Scanned != 6 {
		t.Fatalf("① scanned 应 6，got %d", resp.Data.Scanned)
	}
	if resp.Data.Cleaned != 3 { // A+D 全清 + F 混合清（atmos）
		t.Fatalf("① cleaned 应 3（A+D+F-atmos），got %d", resp.Data.Cleaned)
	}

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
	for _, h2 := range []string{"hash-b", "hash-c", "hash-e"} {
		before := map[string]string{
			"hash-b": `["dolby_vision"]`, "hash-c": `["dolby_atmos"]`,
			"hash-e": `["dolby_atmos","dolby_vision"]`,
		}[h2]
		if get(h2).Tags != before {
			t.Fatalf("%s 行真键不应动：got %s want %s", h2, get(h2).Tags, before)
		}
	}
	// F 行混合态（§59.529 既有语义）：BDInfo DV 通道→vision 保留；
	// atmos 只认 MI（无 BDInfo 通道）→ 清除（对齐当前推理的忠实结果）
	if get("hash-f").Tags != `["dolby_vision"]` {
		t.Fatalf("F 行应为混合态 [\"dolby_vision\"]（vision 保留·atmos 清），got %s", get("hash-f").Tags)
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
	json.NewDecoder(rec2.Body).Decode(&resp2)
	if resp2.Data.Cleaned != 0 {
		t.Fatalf("② 二次调用应清洗 0 行（幂等），got %d", resp2.Data.Cleaned)
	}
}
