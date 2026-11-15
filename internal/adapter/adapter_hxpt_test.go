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

// hxptShelf 货架 RoundTripper（§59.338 离线验证）：按 URL 特征回放
// testdata/hxpt/ 消毒夹具（2026-11-15 活体探针真实响应·§59.1339
// 真实数据源原则）。
type hxptShelf struct{ hits map[string]int }

func (s *hxptShelf) RoundTrip(req *http.Request) (*http.Response, error) {
	u := req.URL.String()
	var file string
	switch {
	case strings.Contains(u, "/api/v1/me"):
		file = "testdata/hxpt/me.json"
	case strings.Contains(u, "/download"):
		file = "testdata/hxpt/dl.bin"
	case strings.Contains(u, "/torrents/23924/detail"):
		file = "testdata/hxpt/detail.json"
	case strings.Contains(u, "/torrents/23924"):
		file = "testdata/hxpt/meta.json"
	case strings.Contains(u, "/torrents?"):
		file = "testdata/hxpt/search.json"
	default:
		file = "testdata/hxpt/search.json"
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

func newHxptTestAdapter() (*HxptAdapter, *hxptShelf) {
	shelf := &hxptShelf{hits: map[string]int{}}
	a := NewHxptAdapter(&HTTPDoer{Client: &http.Client{Transport: shelf}}, zap.NewNop())
	return a, shelf
}

func hxptTestConfig() *model.SiteConfig {
	return &model.SiteConfig{Domain: "www.hxpt.org", BaseURL: "https://www.hxpt.org",
		Enabled: true, Cookie: "flux_session=abc; flux_csrf=TESTCSRF123"}
}

// TestHxptSearch 搜索→结果映射（search 唯一参数·真实夹具条目形态）。
func TestHxptSearch(t *testing.T) {
	a, _ := newHxptTestAdapter()
	items, err := a.SearchTorrents(context.Background(), hxptTestConfig(), "Taken", nil)
	if err != nil {
		t.Fatalf("SearchTorrents: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("夹具应 1 条结果，got %d", len(items))
	}
	first := items[0]
	if first.TorrentID != "23924" {
		t.Errorf("TorrentID=%q want 23924", first.TorrentID)
	}
	if !strings.Contains(first.Title, "Taken Natalee Holloway") {
		t.Errorf("Title=%q", first.Title)
	}
	if first.Size != 5508157951 {
		t.Errorf("Size=%d want 5508157951", first.Size)
	}
	if first.Seeders != 6 {
		t.Errorf("Seeders=%d want 6", first.Seeders)
	}
	if first.PublishAt.IsZero() {
		t.Error("PublishAt 应解析 created_at（RFC3339 纳秒）")
	}
}

// TestHxptDetail 双端点合并（/{id} 元数据 + /{id}/detail 内容）。
func TestHxptDetail(t *testing.T) {
	a, shelf := newHxptTestAdapter()
	d, err := a.GetTorrentDetail(context.Background(), hxptTestConfig(), "23924")
	if err != nil {
		t.Fatalf("GetTorrentDetail: %v", err)
	}
	if d.InfoHash != "aee3fbfc00b9681de8649ceaeef489c126181ef7" {
		t.Errorf("InfoHash=%q", d.InfoHash)
	}
	if d.Category != "纪录片" {
		t.Errorf("Category=%q want 纪录片（category_id=8 映射）", d.Category)
	}
	if !strings.Contains(d.Subtitle, "被夺走的女儿") {
		t.Errorf("Subtitle=%q（/{id} 端点 small_descr）", d.Subtitle)
	}
	if !strings.Contains(d.Description, "ARDTU工具自动发布") {
		t.Errorf("Description=%q（/detail 端点 descr）", d.Description)
	}
	if !strings.Contains(d.MediaInfo, "Complete name") {
		t.Errorf("MediaInfo 应含 MI 正文（/detail 独立字段·非 descr 提取）")
	}
	if d.ReleaseGroup != "UBWEB" {
		t.Errorf("ReleaseGroup=%q want UBWEB（标题尾提取）", d.ReleaseGroup)
	}
	if shelf.hits["testdata/hxpt/meta.json"] != 1 || shelf.hits["testdata/hxpt/detail.json"] != 1 {
		t.Errorf("双端点都应各命中一次: %v", shelf.hits)
	}
}

// TestHxptDetectHR hr_policy 直读（{enabled,seed_hours} 真实形态）。
func TestHxptDetectHR(t *testing.T) {
	a, _ := newHxptTestAdapter()
	hr, err := a.DetectHR(context.Background(), hxptTestConfig(), "23924")
	if err != nil {
		t.Fatalf("DetectHR: %v", err)
	}
	if !hr.HasHR {
		t.Fatal("hr_policy.enabled=true 应 HasHR")
	}
	if hr.SeedTimeH != 96 {
		t.Errorf("SeedTimeH=%d want 96", hr.SeedTimeH)
	}
}

// TestHxptDetectDiscount promotion=null（全站无促销实证形态→NONE）。
func TestHxptDetectDiscount(t *testing.T) {
	a, _ := newHxptTestAdapter()
	d, err := a.DetectDiscount(context.Background(), hxptTestConfig(), "23924")
	if err != nil {
		t.Fatalf("DetectDiscount: %v", err)
	}
	if d.Level != model.DiscountNone {
		t.Errorf("Level=%q want NONE（null 实证形态）", d.Level)
	}
}

// TestHxptCSRFToken double-submit 提取（cookie flux_csrf 值）。
func TestHxptCSRFToken(t *testing.T) {
	cfg := hxptTestConfig()
	if got := hxptCSRFToken(cfg); got != "TESTCSRF123" {
		t.Errorf("CSRFToken=%q want TESTCSRF123", got)
	}
	cfg.Cookie = "flux_session=abc"
	if got := hxptCSRFToken(cfg); got != "" {
		t.Errorf("无 flux_csrf 应空串，got %q", got)
	}
}

// TestHxptUserStats /api/v1/me 映射（上传/下载/做种/分享率计算）。
func TestHxptUserStats(t *testing.T) {
	a, _ := newHxptTestAdapter()
	st, err := a.FetchUserStats(context.Background(), hxptTestConfig())
	if err != nil {
		t.Fatalf("FetchUserStats: %v", err)
	}
	if st.Username != "<作者>" {
		t.Errorf("Username=%q", st.Username)
	}
	if st.UserClass != "发布员" {
		t.Errorf("UserClass=%q", st.UserClass)
	}
	if st.UploadBytes != 5878089979811 {
		t.Errorf("UploadBytes=%d", st.UploadBytes)
	}
	if st.SeedingCount != 22 {
		t.Errorf("SeedingCount=%d want 22", st.SeedingCount)
	}
	wantRatio := float64(5878089979811) / float64(47209923087)
	if st.Ratio < wantRatio-0.01 || st.Ratio > wantRatio+0.01 {
		t.Errorf("Ratio=%f want ~%.2f", st.Ratio, wantRatio)
	}
}

// TestHxptSLData /{id} seeders/leechers 直读。
func TestHxptSLData(t *testing.T) {
	a, _ := newHxptTestAdapter()
	sl, err := a.GetPreciseSLData(context.Background(), hxptTestConfig(), "23924")
	if err != nil {
		t.Fatalf("GetPreciseSLData: %v", err)
	}
	if sl.Seeders != 6 || sl.Leechers != 0 {
		t.Errorf("SL=%+v want seeders=6 leechers=0", sl)
	}
}

// TestHxptGroupOfName 标题尾组名提取边界。
func TestHxptGroupOfName(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Movie 2026 1080p WEB-DL H264-UBWEB", "UBWEB"},
		{"Movie 2026 1080p BluRay x265-cXcY@FRDS", "cXcY@FRDS"},
		{"NoGroup Title", ""},          // 无 - 分隔
		{"Title-Trailing Dash-", ""},   // 尾空
		{"Has Space-Group Name", ""},   // 尾段含空格（非组形态）
	}
	for _, c := range cases {
		if got := hxptGroupOfName(c.in); got != c.want {
			t.Errorf("hxptGroupOfName(%q)=%q want %q", c.in, got, c.want)
		}
	}
}
