package adapter

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

// hdareaShelf 货架 RoundTripper（§59.338 离线验证）：按 URL 特征回放
// testdata/hdarea/ 消毒夹具（2026-11-12 活体探针真实响应·§59.1339
// 真实数据源原则）。
type hdareaShelf struct{ hits map[string]int }

func (s *hdareaShelf) RoundTrip(req *http.Request) (*http.Response, error) {
	u := req.URL.String()
	var file string
	switch {
	case strings.Contains(u, "download.php"):
		file = "testdata/hdarea/dl.bin"
	case strings.Contains(u, "genDlToken"):
		file = "testdata/hdarea/gentoken.json"
	case strings.Contains(u, "user/profile"):
		file = "testdata/hdarea/profile.json"
	case strings.Contains(u, "/torrents/460261"):
		file = "testdata/hdarea/detail.json"
	case strings.Contains(u, "/torrents?"):
		file = "testdata/hdarea/search.json"
	case strings.Contains(u, "/torrents/upload"):
		file = "testdata/hdarea/gentoken.json" // 上传响应形态借用（刀2 夹具化）
	default:
		file = "testdata/hdarea/search.json"
	}
	body, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	s.hits[file]++
	return &http.Response{
		StatusCode: 200, Status: "200 OK", Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(string(body))),
		Request:    req,
	}, nil
}

func newHDAreaTestAdapter() (*HDAreaAdapter, *hdareaShelf) {
	shelf := &hdareaShelf{hits: map[string]int{}}
	a := NewHDAreaAdapter(&HTTPDoer{Client: &http.Client{Transport: shelf}}, zap.NewNop())
	return a, shelf
}

func hdareaTestConfig() *model.SiteConfig {
	return &model.SiteConfig{Domain: "hdarea.club", BaseURL: "https://hdarea.club", Enabled: true, APIKey: "TESTKEY"}
}

// TestHDAreaSearch 搜索→结果映射（真实夹具·id/标题/大小/做种/促销）。
func TestHDAreaSearch(t *testing.T) {
	a, _ := newHDAreaTestAdapter()
	items, err := a.SearchTorrents(context.Background(), hdareaTestConfig(), "Dune", nil)
	if err != nil {
		t.Fatalf("SearchTorrents: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("夹具应有搜索结果")
	}
	first := items[0]
	if first.TorrentID != "447041" {
		t.Errorf("TorrentID=%q want 447041", first.TorrentID)
	}
	if !strings.Contains(first.Title, "Dunes") {
		t.Errorf("Title=%q 应含 Dunes", first.Title)
	}
	if first.Size != 7319239549 {
		t.Errorf("Size=%d want 7319239549", first.Size)
	}
}

// TestHDAreaDetail 详情映射（标题/副标/分类/MI 提取/imdb 豆瓣/info_hash）。
func TestHDAreaDetail(t *testing.T) {
	a, _ := newHDAreaTestAdapter()
	d, err := a.GetTorrentDetail(context.Background(), hdareaTestConfig(), "460261")
	if err != nil {
		t.Fatalf("GetTorrentDetail: %v", err)
	}
	if !strings.Contains(d.Title, "Sunshine Through the Rain") {
		t.Errorf("Title=%q", d.Title)
	}
	if !strings.Contains(d.Subtitle, "今天是太阳雨") {
		t.Errorf("Subtitle=%q", d.Subtitle)
	}
	if d.Category != "TV Series" {
		t.Errorf("Category=%q want TV Series", d.Category)
	}
	if d.InfoHash != "302426f332ccf0e456dda74186b8786b467e2f4f" {
		t.Errorf("InfoHash=%q", d.InfoHash)
	}
	if !strings.Contains(d.IMDbURL, "imdb.com") || !strings.Contains(d.DoubanURL, "douban.com") {
		t.Errorf("IMDb/Douban 缺失: %q %q", d.IMDbURL, d.DoubanURL)
	}
	if d.MediaInfo == "" || !strings.Contains(d.MediaInfo, "General") {
		t.Errorf("MediaInfo 提取失败（descr BBCode 内 quote 块）: %q", truncate(d.MediaInfo, 80))
	}
	if d.Size != 69204663901 {
		t.Errorf("Size=%d", d.Size)
	}
}

// TestHDAreaDiscount 促销映射（detail 夹具 sp_state=2 免费+promotion_until）。
func TestHDAreaDiscount(t *testing.T) {
	a, _ := newHDAreaTestAdapter()
	dc, err := a.DetectDiscount(context.Background(), hdareaTestConfig(), "460261")
	if err != nil {
		t.Fatalf("DetectDiscount: %v", err)
	}
	if dc.Level != model.DiscountFree {
		t.Errorf("Level=%v want FREE（dl_factor=0）", dc.Level)
	}
	if dc.FreeEndAt == nil {
		t.Error("FreeEndAt 应在场（promotion_until）")
	}
}

// TestHDAreaStats profile 映射（上传/下载/分享率/魔力/做种）。
func TestHDAreaStats(t *testing.T) {
	a, _ := newHDAreaTestAdapter()
	st, err := a.FetchUserStats(context.Background(), hdareaTestConfig())
	if err != nil {
		t.Fatalf("FetchUserStats: %v", err)
	}
	if st.Username != "ranfish" {
		t.Errorf("Username=%q", st.Username)
	}
	if st.UploadBytes != 212828947337560 {
		t.Errorf("UploadBytes=%d", st.UploadBytes)
	}
	if st.SeedingCount != 5761 {
		t.Errorf("SeedingCount=%d", st.SeedingCount)
	}
}

// TestHDAreaThinStubs 薄实现契约（无 HR/无 SL/pieces-hash false）。
func TestHDAreaThinStubs(t *testing.T) {
	a, _ := newHDAreaTestAdapter()
	ctx := context.Background()
	cfg := hdareaTestConfig()
	hr, err := a.DetectHR(ctx, cfg, "1")
	if err != nil || hr.HasHR {
		t.Errorf("DetectHR 应恒 false（好大无 HR 实证）: %v %v", hr, err)
	}
	if a.SupportsSearchByPiecesHash() {
		t.Error("SupportsSearchByPiecesHash 应 false（API 无该能力记载）")
	}
	sl, err := a.GetPreciseSLData(ctx, cfg, "1")
	if sl != nil || err != nil {
		t.Errorf("GetPreciseSLData 应空: %v %v", sl, err)
	}
	batch, err := a.GetBatchSLData(ctx, cfg, []string{"1"})
	if err != nil || len(batch) != 0 {
		t.Errorf("GetBatchSLData 应空 map: %v %v", batch, err)
	}
}

// TestHDAreaInfoHash tid→infohash（detail 响应直取）。
func TestHDAreaInfoHash(t *testing.T) {
	a, _ := newHDAreaTestAdapter()
	h, err := a.GetTorrentInfoHash(context.Background(), hdareaTestConfig(), "460261")
	if err != nil {
		t.Fatalf("GetTorrentInfoHash: %v", err)
	}
	if h != "302426f332ccf0e456dda74186b8786b467e2f4f" {
		t.Errorf("InfoHash=%q", h)
	}
}

// TestHDAreaDownload 短效凭证下载（双层信封解析+直链 GET）。
func TestHDAreaDownload(t *testing.T) {
	a, _ := newHDAreaTestAdapter()
	data, err := a.DownloadTorrent(context.Background(), hdareaTestConfig(), "460261")
	if err != nil {
		t.Fatalf("DownloadTorrent: %v", err)
	}
	if !strings.HasPrefix(string(data), "d8:announce") {
		t.Errorf("响应非 bencode: %q", truncate(string(data), 30))
	}
}
