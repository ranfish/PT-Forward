package adapter

import (
	"bytes"
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
	case strings.Contains(u, "/api/v1/users/"):
		file = "testdata/hxpt/users.json"
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
	if st.BonusPoints != 2275805 {
		t.Errorf("BonusPoints=%v want 2275805（me/spark balance）", st.BonusPoints)
	}
	if st.SeedingSize != 1347914317403 {
		t.Errorf("SeedingSize=%d want 1347914317403（users/{id} 全量做种体积）", st.SeedingSize)
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

// ── 发布（刀3）──

// hxptUploadCapture 捕获型 RoundTripper：记录 multipart 字段+header，
// 回放预设响应（断言请求形态用）。
type hxptUploadCapture struct {
	respBody string
	status   int
	fields   map[string]string
	fileSize int
	headers  http.Header
}

func (c *hxptUploadCapture) RoundTrip(req *http.Request) (*http.Response, error) {
	c.headers = req.Header.Clone()
	rb, _ := io.ReadAll(req.Body)
	if err := req.Body.Close(); err != nil {
		return nil, err
	}
	// 重放 body 供 multipart 解析
	req.Body = io.NopCloser(bytes.NewReader(rb))
	if strings.Contains(req.Header.Get("Content-Type"), "multipart/form-data") {
		req.Header.Set("Content-Type", strings.SplitN(req.Header.Get("Content-Type"), ";", 2)[0]+"; boundary="+extractBoundary(req.Header.Get("Content-Type")))
		r, err := req.MultipartReader()
		if err == nil {
			c.fields = map[string]string{}
			for {
				p, perr := r.NextPart()
				if perr == io.EOF {
					break
				}
				if perr != nil {
					break
				}
				name := p.FormName()
				if p.FileName() != "" {
					b, _ := io.ReadAll(p)
					c.fileSize = len(b)
					continue
				}
				b, _ := io.ReadAll(p)
				if old, ok := c.fields[name]; ok {
					c.fields[name] = old + "," + string(b) // 数组语义聚合（断言用）
				} else {
					c.fields[name] = string(b)
				}
			}
		}
	}
	status := c.status
	if status == 0 {
		status = 200
	}
	return &http.Response{
		StatusCode: status, Status: "200 OK", Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(c.respBody)),
		Request:    req,
	}, nil
}

func extractBoundary(ct string) string {
	i := strings.Index(ct, "boundary=")
	if i < 0 {
		return ""
	}
	return strings.Trim(strings.TrimSpace(ct[i+len("boundary="):]), `"`)
}

func newHxptUploadAdapter(capture *hxptUploadCapture) *HxptAdapter {
	return NewHxptAdapter(&HTTPDoer{Client: &http.Client{Transport: capture}}, zap.NewNop())
}

func hxptUploadReq() *model.PublishRequest {
	return &model.PublishRequest{
		TorrentData: []byte("d8:announce"),
		FormFields: map[string]string{
			"category_id": "8",
			"media":       "443",
			"codec":       "435",
			"source":      "451",
			"team":        "469",
		},
		TagArrayFields: []model.TagKV{
			{Key: "tagList", Value: "9"},
			{Key: "tagList", Value: "10"},
			{Key: "tagList", Value: "dolby_vision"}, // 站方值集 miss 标准键直出→ParseInt 蒸发
		},
		Title:       "Test Doc 2026 1080p WEB-DL H264-UBWEB",
		Subtitle:    "测试副标题",
		Description: "[quote]简介[/quote]",
		MediaInfo:   "General\nComplete name: test.mkv",
		IMDbLink:    "https://www.imdb.com/title/tt1234567/",
		Anonymous:   true,
	}
}

// TestHxptUploadForm 上传 FormData 形态断言（字段完整/sections 数值/tags
// 数组/CSRF header/恒不发字段缺席）。
func TestHxptUploadForm(t *testing.T) {
	capture := &hxptUploadCapture{
		respBody: `{"code":0,"message":"ok","data":{"id":23999}}`,
	}
	a := newHxptUploadAdapter(capture)
	resp, err := a.UploadTorrent(context.Background(), hxptTestConfig(), hxptUploadReq())
	if err != nil {
		t.Fatalf("UploadTorrent: %v", err)
	}
	// 响应解析
	if !resp.Success || resp.TorrentID != "23999" {
		t.Errorf("resp=%+v want tid 23999", resp)
	}
	if resp.DetailURL != "https://www.hxpt.org/torrent/23999" {
		t.Errorf("DetailURL=%q", resp.DetailURL)
	}
	if resp.IDSource != "api_response" {
		t.Errorf("IDSource=%q", resp.IDSource)
	}
	// header
	if got := capture.headers.Get("x-csrf-token"); got != "TESTCSRF123" {
		t.Errorf("x-csrf-token=%q want TESTCSRF123（double-submit）", got)
	}
	if got := capture.headers.Get("Cookie"); !strings.Contains(got, "flux_session") {
		t.Errorf("Cookie header=%q", got)
	}
	// FormData 字段
	if capture.fields["category_id"] != "8" {
		t.Errorf("category_id=%q", capture.fields["category_id"])
	}
	if capture.fields["anonymous"] != "true" {
		t.Errorf("anonymous=%q", capture.fields["anonymous"])
	}
	if capture.fields["name"] == "" || capture.fields["small_descr"] == "" ||
		capture.fields["descr"] == "" || capture.fields["mediainfo"] == "" {
		t.Errorf("文本字段缺失: %v", capture.fields)
	}
	if capture.fields["imdb"] != "https://www.imdb.com/title/tt1234567/" {
		t.Errorf("imdb=%q", capture.fields["imdb"])
	}
	// sections：纯数字 dict_id JSON
	if capture.fields["sections"] != `{"codec":435,"media":443,"source":451,"team":469}` {
		t.Errorf("sections=%q", capture.fields["sections"])
	}
	// tags：数值数组 JSON
	if capture.fields["tags"] != "[9,10]" {
		t.Errorf("tags=%q", capture.fields["tags"])
	}
	// file
	if capture.fileSize != len("d8:announce") {
		t.Errorf("file size=%d", capture.fileSize)
	}
	// 恒不发字段缺席（用户定案 price 不传+站方管理功能+音乐向）
	for _, absent := range []string{"price", "pos_state", "pick_type", "hr_punish", "log", "nfo", "tmdb"} {
		if v, ok := capture.fields[absent]; ok {
			t.Errorf("字段 %s 恒不发却出现=%q", absent, v)
		}
	}
}

// TestHxptUploadValidation1002 code=1002 字段校验错误透出。
func TestHxptUploadValidation1002(t *testing.T) {
	capture := &hxptUploadCapture{
		respBody: `{"code":1002,"message":"validation failed","data":{"name":["标题不能为空"],"category_id":["分类必选"]}}`,
	}
	a := newHxptUploadAdapter(capture)
	_, err := a.UploadTorrent(context.Background(), hxptTestConfig(), hxptUploadReq())
	if err == nil {
		t.Fatal("1002 应返回错误")
	}
	msg := err.Error()
	if !strings.Contains(msg, "name") || !strings.Contains(msg, "标题不能为空") {
		t.Errorf("校验错误应透出字段明细: %s", msg)
	}
}

// TestHxptUploadMissingCategory category_id 缺失 fail-fast。
func TestHxptUploadMissingCategory(t *testing.T) {
	capture := &hxptUploadCapture{}
	a := newHxptUploadAdapter(capture)
	req := hxptUploadReq()
	delete(req.FormFields, "category_id")
	_, err := a.UploadTorrent(context.Background(), hxptTestConfig(), req)
	if err == nil || !strings.Contains(err.Error(), "category_id") {
		t.Errorf("缺 category_id 应 fail-fast: %v", err)
	}
}

// TestHxptUploadSectionsNonNumeric sections 非数值 dict_id 拒绝（防
// 值集映射误配 label 值静默错投）。
func TestHxptUploadSectionsNonNumeric(t *testing.T) {
	capture := &hxptUploadCapture{}
	a := newHxptUploadAdapter(capture)
	req := hxptUploadReq()
	req.FormFields["media"] = "视频" // label 误配形态
	_, err := a.UploadTorrent(context.Background(), hxptTestConfig(), req)
	if err == nil || !strings.Contains(err.Error(), "media") {
		t.Errorf("非 dict_id 值应拒绝: %v", err)
	}
}

// TestHxptUploadEmptyFieldsOptional 空字段不发+空 sections/tags 整体缺席
// （对齐站方 l() 条件追加语义）。
func TestHxptUploadEmptyFieldsOptional(t *testing.T) {
	capture := &hxptUploadCapture{
		respBody: `{"code":0,"message":"ok","data":{"id":1}}`,
	}
	a := newHxptUploadAdapter(capture)
	req := hxptUploadReq()
	req.Subtitle = ""
	req.Description = ""
	req.MediaInfo = ""
	req.IMDbLink = ""
	req.FormFields = map[string]string{"category_id": "1"} // 四维全空
	req.TagArrayFields = nil
	if _, err := a.UploadTorrent(context.Background(), hxptTestConfig(), req); err != nil {
		t.Fatalf("全可选空应成功: %v", err)
	}
	for _, absent := range []string{"small_descr", "descr", "mediainfo", "imdb", "sections", "tags"} {
		if v, ok := capture.fields[absent]; ok {
			t.Errorf("空字段 %s 不应出现=%q", absent, v)
		}
	}
}

// TestSiteBaseURL §59.533 联合回归补修：请求基址公共构造四态
// （stats_sync 生产链把 site.BaseURL 塞 config.Domain——好学/好大
// 双协议爆雷实证）。
func TestSiteBaseURL(t *testing.T) {
	cases := []struct {
		baseURL, domain, want string
	}{
		{"", "www.hxpt.org", "https://www.hxpt.org"},                                 // 裸域（标准形态）
		{"https://hdarea.club/", "", "https://hdarea.club"},                         // BaseURL 优先
		{"", "https://www.hxpt.org", "https://www.hxpt.org"},                        // ★ stats_sync 塞法（本案）
		{"", "http://old.example.com", "http://old.example.com"},                    // http 协议域
	}
	for _, c := range cases {
		cfg := &model.SiteConfig{BaseURL: c.baseURL, Domain: c.domain}
		if got := siteBaseURL(cfg); got != c.want {
			t.Errorf("siteBaseURL(base=%q domain=%q)=%q want %q", c.baseURL, c.domain, got, c.want)
		}
	}
}
