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
