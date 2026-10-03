package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/publish"
	"go.uber.org/zap"
)

// §59.281: OTA 任务门控——BusyTaskDesc 空闲态
func TestBusyTaskDescIdle(t *testing.T) {
	h := &PublishTorrentsHandler{}
	h.siteBatch.tasks = map[string]*siteBatchTask{}
	h.siteBatch.active = map[string]string{}
	if d := h.BusyTaskDesc(); d != "" {
		t.Errorf("空闲态应空, got %q", d)
	}
}

// §59.286: 主链 PTGen 三级查询键列表（空键跳过）
func TestMainlinePTGenQueryKeys(t *testing.T) {
	cases := []struct {
		douban, imdb, title string
		want                []string
	}{
		{"https://douban.com/s/1", "https://imdb.com/tt1", "T", []string{"https://douban.com/s/1", "https://imdb.com/tt1", "T"}},
		{"", "https://imdb.com/tt1", "T", []string{"https://imdb.com/tt1", "T"}}, // Diesel 案
		{"", "", "Diesel 2025", []string{"Diesel 2025"}},
		{"", "", "", nil},
	}
	for _, c := range cases {
		got := mainlineQueryKeys(c.douban, c.imdb, c.title)
		if len(got) != len(c.want) {
			t.Errorf("got %v want %v", got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("got %v want %v", got, c.want)
			}
		}
	}
}

// §59.286: 三级链短路——douban 失败落 imdb 成功（stub PTGenAnalyzer）
type stubPTGen struct {
	fail map[string]bool
}

func (s *stubPTGen) AnalyzePTGen(_ context.Context, q string) (*model.PTGenResult, error) {
	if s.fail[q] {
		return nil, fmt.Errorf("stub fail")
	}
	return &model.PTGenResult{RawBBCode: "[desc]", PosterURL: "https://img.doubanio.com/p.jpg"}, nil
}
func (s *stubPTGen) AnalyzePTGenForce(ctx context.Context, q string) (*model.PTGenResult, error) {
	return s.AnalyzePTGen(ctx, q)
}

func TestMainlinePTGenChainShortCircuit(t *testing.T) {
	h := &PublishTorrentsHandler{ptgen: &stubPTGen{fail: map[string]bool{"https://douban.com/x": true}}}
	keys := mainlineQueryKeys("https://douban.com/x", "https://imdb.com/tt1", "")
	var hit *model.PTGenResult
	for _, k := range keys {
		r, err := h.ptgen.AnalyzePTGen(context.Background(), k)
		if err == nil && r != nil && r.RawBBCode != "" {
			hit = r
			break
		}
	}
	if hit == nil || hit.PosterURL == "" {
		t.Errorf("链未在 imdb 级短路: %+v", hit)
	}
}

// §59.288: batch-review info_hashes 形态——hash→id 解析（内存 DB）
func TestBatchReviewByHashes(t *testing.T) {
	db := clusterTestDB(t)
	_ = &PublishTorrentsHandler{db: db, logger: zap.NewNop()}
	db.Create(&model.TorrentMetadata{InfoHash: "brhash000000000000000000000000000000000", SiteName: "站A", Title: "t", Reviewed: false})
	db.Create(&model.TorrentMetadata{InfoHash: "brhash000000000000000000000000000000000", SiteName: "站B", Title: "t", Reviewed: false})

	var ids []uint
	db.Model(&model.TorrentMetadata{}).Where("info_hash IN ?", []string{"brhash000000000000000000000000000000000"}).Pluck("id", &ids)
	if len(ids) != 2 {
		t.Fatalf("hash 解析应得 2 行, got %d", len(ids))
	}
	db.Model(&model.TorrentMetadata{}).Where("id IN ?", ids).Update("reviewed", true)
	var n int64
	db.Model(&model.TorrentMetadata{}).Where("info_hash = ? AND reviewed = 1", "brhash000000000000000000000000000000000").Count(&n)
	if n != 2 {
		t.Errorf("两行应均审: %d", n)
	}
}

// §59.316: batch-review hash 形态簇口径解析——簇代表 hash（仅快照行）+ 元数据在
// 兄弟 hash 下：直连解析空→"ids required"误拒（243 八连案）；簇解析后同簇兄弟
// 元数据均命中并置 reviewed。
func TestBatchReviewClusterResolution(t *testing.T) {
	db := clusterTestDB(t)
	h := &PublishTorrentsHandler{db: db, logger: zap.NewNop()}
	// 簇：代表 hash 只有快照；元数据挂在兄弟 hash（甲站）
	db.Create(&model.TorrentSnapshot{Hash: "rephash000000000000000000000000000000000", ClientUID: 1, SavePath: "/v", Name: "n"})
	db.Create(&model.TorrentSnapshot{Hash: "sibhash000000000000000000000000000000000", ClientUID: 1, SavePath: "/v", Name: "n"})
	db.Create(&model.TorrentMetadata{InfoHash: "sibhash000000000000000000000000000000000", SiteName: "甲站", Title: "t", Reviewed: false})
	// 无关簇：不受牵连
	db.Create(&model.TorrentSnapshot{Hash: "othhash000000000000000000000000000000000", ClientUID: 2, SavePath: "/v", Name: "o"})
	db.Create(&model.TorrentMetadata{InfoHash: "othhash000000000000000000000000000000000", SiteName: "丙站", Title: "o", Reviewed: false})

	body := `{"info_hashes":["rephash000000000000000000000000000000000"],"reviewed":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/publish/seed-data/batch-review", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.handleBatchReview(rec, req)

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Updated int `json:"updated"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Code != 0 || resp.Data.Updated != 1 {
		t.Fatalf("簇代表 hash 应审 1 行: code=%d updated=%d body=%s", resp.Code, resp.Data.Updated, rec.Body.String())
	}
	var n int64
	db.Model(&model.TorrentMetadata{}).Where("info_hash = ? AND reviewed = 1", "sibhash000000000000000000000000000000000").Count(&n)
	if n != 1 {
		t.Errorf("兄弟 hash 元数据应已审: %d", n)
	}
	db.Model(&model.TorrentMetadata{}).Where("info_hash = ? AND reviewed = 1", "othhash000000000000000000000000000000000").Count(&n)
	if n != 0 {
		t.Errorf("无关簇不应受牵连: %d", n)
	}
}

// §59.316 附 A: 批量发布进度标题簇口径解析——代表 hash 直查在漂移簇失配（标题空），
// ResourceResolver 紧键圈簇后命中兄弟 hash 元数据。
func TestSiteBatchCurTitleClusterResolution(t *testing.T) {
	db := clusterTestDB(t)
	rep := "rep" + strings.Repeat("0", 37)
	sib := "sib" + strings.Repeat("0", 37)
	h := &PublishTorrentsHandler{db: db, logger: zap.NewNop(),
		resourceResolver: publish.NewResourceResolver(db)}
	db.Create(&model.TorrentSnapshot{Hash: rep, ClientUID: 1, SavePath: "/v", Name: "n"})
	db.Create(&model.TorrentSnapshot{Hash: sib, ClientUID: 1, SavePath: "/v", Name: "n"})
	db.Create(&model.TorrentMetadata{InfoHash: sib, SiteName: "A", Title: "漂移簇标题", Reviewed: false})
	if got := h.siteBatchCurTitle(context.Background(), rep); got != "漂移簇标题" {
		t.Errorf("漂移簇代表 hash 应解析到标题, got %q", got)
	}
	if got := h.siteBatchCurTitle(context.Background(), "noexist"+strings.Repeat("0", 33)); got != "" {
		t.Errorf("无快照无元数据应留空, got %q", got)
	}
	// resolver 未注入（测试/降级）→ 回退直查仍可用
	h2 := &PublishTorrentsHandler{db: db, logger: zap.NewNop()}
	if got := h2.siteBatchCurTitle(context.Background(), sib); got != "漂移簇标题" {
		t.Errorf("回退直查应命中, got %q", got)
	}
}

// §59.316 附 C: 列表紧键域——同名跨下载器簇不互相借数据（B 簇无元数据 →
// unfetched，不再借 A 簇 meta 显示假 ready）；同簇兄弟 hash 正常命中（H1 保持）。
func TestListSeedsTightClusterScope(t *testing.T) {
	db := clusterTestDB(t)
	h := &PublishTorrentsHandler{db: db, logger: zap.NewNop(),
		resourceResolver: publish.NewResourceResolver(db)}
	Zeros := func(n int) string { return strings.Repeat("0", n) }
	// 簇 A（client 1 /v）：代表 hash 仅快照，元数据挂兄弟 hash（漂移簇）
	db.Create(&model.TorrentSnapshot{Hash: "a" + "rep" + Zeros(36), ClientUID: 1, SavePath: "/v", Name: "shared.name.2023"})
	db.Create(&model.TorrentSnapshot{Hash: "a" + "sib" + Zeros(36), ClientUID: 1, SavePath: "/v", Name: "shared.name.2023"})
	db.Create(&model.TorrentMetadata{InfoHash: "a" + "sib" + Zeros(36), SiteName: "A", Title: "T", Poster: "p", Description: "d", Screenshots: `["u"]`, Tags: `["x"]`, MediaInfo: "mi", DoubanURL: "https://d", Reviewed: true})
	// 簇 B（client 2 /w）：同名、无任何元数据
	db.Create(&model.TorrentSnapshot{Hash: "b" + "rep" + Zeros(36), ClientUID: 2, SavePath: "/w", Name: "shared.name.2023"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/publish/seeds?page=1&page_size=50", nil)
	rec := httptest.NewRecorder()
	h.handleListSeeds(rec, req)
	var resp struct {
		Data struct {
			Items []map[string]interface{} `json:"items"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	stByClient := map[uint]interface{}{}
	for _, it := range resp.Data.Items {
		stByClient[uint(it["client_id"].(float64))] = it["status"]
	}
	if s := stByClient[1]; s != "reviewed" {
		t.Errorf("簇 A（漂移代表+兄弟元数据）应为 reviewed, got %v", s)
	}
	if s := stByClient[2]; s != "unfetched" {
		t.Errorf("簇 B（同名跨下载器无元数据）应为 unfetched——不得借用簇 A 数据, got %v", s)
	}
}

// §59.316 附 D: 清除紧键域——删簇 A 代表 hash 只清 A 簇，同名簇 B 元数据保留。
func TestDeleteSeedTightScope(t *testing.T) {
	db := clusterTestDB(t)
	h := &PublishTorrentsHandler{db: db, logger: zap.NewNop()}
	Zeros := func(n int) string { return strings.Repeat("0", n) }
	db.Create(&model.TorrentSnapshot{Hash: "a" + "rep" + Zeros(36), ClientUID: 1, SavePath: "/v", Name: "shared.name.2023"})
	db.Create(&model.TorrentSnapshot{Hash: "a" + "sib" + Zeros(36), ClientUID: 1, SavePath: "/v", Name: "shared.name.2023"})
	db.Create(&model.TorrentMetadata{InfoHash: "a" + "sib" + Zeros(36), SiteName: "A", Title: "T"})
	db.Create(&model.TorrentSnapshot{Hash: "b" + "rep" + Zeros(36), ClientUID: 2, SavePath: "/w", Name: "shared.name.2023"})
	db.Create(&model.TorrentMetadata{InfoHash: "b" + "rep" + Zeros(36), SiteName: "B", Title: "T"})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/publish/seeds/"+"a"+"rep"+Zeros(36), nil)
	rec := httptest.NewRecorder()
	h.handleDeleteSeed(rec, req)

	var n int64
	db.Model(&model.TorrentMetadata{}).Where("info_hash = ?", "a"+"sib"+Zeros(36)).Count(&n)
	if n != 0 {
		t.Errorf("簇 A 兄弟元数据应被清除: %d", n)
	}
	db.Model(&model.TorrentMetadata{}).Where("info_hash = ?", "b"+"rep"+Zeros(36)).Count(&n)
	if n != 1 {
		t.Errorf("同名簇 B 元数据不应连坐清除: %d", n)
	}
}
