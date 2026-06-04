package adapter

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

func TestTNodeAdapter_Framework(t *testing.T) {
	a := NewTNodeAdapter(NewHTTPDoer(), zap.NewNop())
	if a.Framework() != "tnode" {
		t.Errorf("expected tnode, got %s", a.Framework())
	}
}

func TestTNodeAdapter_DownloadTorrent_OK(t *testing.T) {
	payload := []byte("d4:infod6:lengthi0eee")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/index":
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<html><meta name="x-csrf-token" content="test-csrf-token"/></html>`))
		case "/api/torrent/download/42":
			if r.Header.Get("x-csrf-token") != "test-csrf-token" {
				t.Errorf("missing csrf token header")
			}
			w.Header().Set("Content-Type", "application/x-bittorrent")
			w.Write(payload)
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewTNodeAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}
	data, err := a.DownloadTorrent(context.Background(), config, "42")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(payload) {
		t.Errorf("unexpected data")
	}
}

func TestTNodeAdapter_DownloadTorrent_Forbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewTNodeAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}
	_, err := a.DownloadTorrent(context.Background(), config, "1")
	if err == nil {
		t.Fatal("expected error for 403")
	}
}

func TestTNodeAdapter_GetTorrentDetail(t *testing.T) {
	html := `<!DOCTYPE html><html><head><title>Test Torrent Title</title></head><body>
	<tr><td>info_hash</td><td>ABCDEF0123456789ABCDEF0123456789ABCDEF00</td></tr>
	<tr><td>大小</td><td>1.5 GB</td></tr>
	<tr><td>分类</td><td>电影</td></tr>
	</body></html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewTNodeAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}
	detail, err := a.GetTorrentDetail(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Title != "Test Torrent Title" {
		t.Errorf("unexpected title: %s", detail.Title)
	}
	if detail.InfoHash != "abcdef0123456789abcdef0123456789abcdef00" {
		t.Errorf("unexpected info_hash: %s", detail.InfoHash)
	}
}

func TestTNodeAdapter_DetectDiscount_Free(t *testing.T) {
	html := `<html><body><td class="pro_free">免费</td></body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewTNodeAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}
	result, err := a.DetectDiscount(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Level != model.DiscountFree {
		t.Errorf("expected free, got %s", result.Level)
	}
}

func TestTNodeAdapter_DetectHR(t *testing.T) {
	html := `<html><body><td class="hitandrun">Hit and Run</td></body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewTNodeAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}
	result, err := a.DetectHR(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if !result.HasHR {
		t.Error("expected HR detected")
	}
	if result.SeedTimeH != 72 {
		t.Errorf("expected seed time 72h, got %d", result.SeedTimeH)
	}
}

func TestMTeamAdapter_Framework(t *testing.T) {
	a := NewMTeamAdapter(NewHTTPDoer(), zap.NewNop())
	if a.Framework() != "mteam" {
		t.Errorf("expected mteam, got %s", a.Framework())
	}
}

func TestMTeamAdapter_DownloadViaWeb(t *testing.T) {
	payload := []byte("d4:infod6:lengthi0eee")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/download.php" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/x-bittorrent")
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewMTeamAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}
	data, err := a.DownloadTorrent(context.Background(), config, "42")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(payload) {
		t.Errorf("unexpected data")
	}
}

func TestMTeamAdapter_DetailViaWeb(t *testing.T) {
	html := `<!DOCTYPE html><html><head><title>MT Torrent</title></head><body>
	<tr><td>info_hash</td><td>AAEE0123456789AAEE0123456789AAEE01234567</td></tr>
	<tr><td>大小</td><td>2 GB</td></tr>
	</body></html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewMTeamAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}
	detail, err := a.GetTorrentDetail(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Title != "MT Torrent" {
		t.Errorf("unexpected title: %s", detail.Title)
	}
}

func TestMTeamAdapter_UploadTorrent_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Fatalf("parse multipart: %v", err)
		}
		if r.FormValue("name") != "Test Upload" {
			t.Errorf("unexpected name: %s", r.FormValue("name"))
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("form file: %v", err)
		}
		defer func() { _ = file.Close() }()
		_, _ = file.Read(make([]byte, 1))

		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html>Upload succeeded <a href="details.php?id=99">here</a></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewMTeamAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}
	resp, err := a.UploadTorrent(context.Background(), config, &model.PublishRequest{
		TorrentData: []byte("d4:infod6:lengthi0eee"),
		Title:       "Test Upload",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Success {
		t.Errorf("expected success, got: %s", resp.ErrorMessage)
	}
	if resp.TorrentID != "99" {
		t.Errorf("expected torrent ID 99, got %s", resp.TorrentID)
	}
}

func TestMTeamAdapter_UploadTorrent_NoData(t *testing.T) {
	a := NewMTeamAdapter(NewHTTPDoer(), zap.NewNop())
	config := &model.SiteConfig{Domain: "https://xp.m-team.cc"}

	_, err := a.UploadTorrent(context.Background(), config, &model.PublishRequest{})
	if err == nil {
		t.Fatal("expected error for empty torrent data")
	}
}

func TestTNodeAdapter_GetPreciseSLData_OK(t *testing.T) {
	html := `<html><body>
	<tr><td>做种人数: <span>15</span></td></tr>
	<tr><td>下载数: <span>3</span></td></tr>
	</body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewTNodeAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}
	sl, err := a.GetPreciseSLData(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if sl.Seeders != 15 {
		t.Errorf("expected 15 seeders, got %d", sl.Seeders)
	}
	if sl.Leechers != 3 {
		t.Errorf("expected 3 leechers, got %d", sl.Leechers)
	}
}

func TestTNodeAdapter_GetTorrentInfoHash_OK(t *testing.T) {
	html := `<!DOCTYPE html><html><head><title>Hash Test</title></head><body>
	<tr><td>info_hash</td><td>DEADBEEF0123456789ABCDEF0123456789ABCDEF</td></tr>
	</body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewTNodeAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}
	hash, err := a.GetTorrentInfoHash(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if hash != "deadbeef0123456789abcdef0123456789abcdef" {
		t.Errorf("unexpected hash: %s", hash)
	}
}

func TestTNodeAdapter_UploadTorrent_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/torrent/upload":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html><head><meta name="x-csrf-token" content="testcsrf123"></head><body></body></html>`))
		case "/api/torrent/upload":
			if r.Method != "POST" {
				t.Errorf("expected POST, got %s", r.Method)
			}
			if r.Header.Get("x-csrf-token") != "testcsrf123" {
				t.Errorf("missing or wrong csrf token header: %s", r.Header.Get("x-csrf-token"))
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":200,"data":{"code":"UPLOAD_SUCCESS","id":"42"}}`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewTNodeAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}
	resp, err := a.UploadTorrent(context.Background(), config, &model.PublishRequest{
		TorrentData: []byte("d4:infod6:lengthi0eee"),
		Title:       "Upload Test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Success {
		t.Errorf("expected success, got: %s", resp.ErrorMessage)
	}
	if resp.TorrentID != "42" {
		t.Errorf("expected torrent ID 42, got %s", resp.TorrentID)
	}
}

func TestTNodeAdapter_UploadTorrent_NoData(t *testing.T) {
	a := NewTNodeAdapter(NewHTTPDoer(), zap.NewNop())
	config := &model.SiteConfig{Domain: "https://example.com"}

	_, err := a.UploadTorrent(context.Background(), config, &model.PublishRequest{})
	if err == nil {
		t.Fatal("expected error for empty torrent data")
	}
}

func TestTNodeAdapter_fetchCSRFToken_MetaTag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head><meta name="x-csrf-token" content="abc123"></head></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewTNodeAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}
	token, err := a.fetchCSRFToken(context.Background(), config, srv.URL+"/upload")
	if err != nil {
		t.Fatal(err)
	}
	if token != "abc123" {
		t.Errorf("expected abc123, got %s", token)
	}
}

func TestTNodeAdapter_fetchCSRFToken_Alternate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><script>var csrf_token="xyz789";</script></body></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewTNodeAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}
	token, err := a.fetchCSRFToken(context.Background(), config, srv.URL+"/upload")
	if err != nil {
		t.Fatal(err)
	}
	if token != "xyz789" {
		t.Errorf("expected xyz789, got %s", token)
	}
}

func TestTNodeAdapter_fetchCSRFToken_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><p>No CSRF here</p></body></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewTNodeAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}
	_, err := a.fetchCSRFToken(context.Background(), config, srv.URL+"/upload")
	if err == nil {
		t.Fatal("expected error when CSRF token not found")
	}
}

func TestTNodeAdapter_UploadTorrent_Duplicate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/torrent/upload":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html><head><meta name="x-csrf-token" content="dupcsrf"></head></html>`))
		case "/api/torrent/upload":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":"TORRENT_ALREADY_UPLOAD"}`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewTNodeAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}
	resp, err := a.UploadTorrent(context.Background(), config, &model.PublishRequest{
		TorrentData: []byte("d4:infod6:lengthi0eee"),
	})
	if err == nil {
		t.Fatal("expected error for duplicate")
	}
	var appErr *model.AppError
	if !errors.As(err, &appErr) || appErr.Code != 14001 {
		t.Fatalf("expected AppError 14001, got %v", err)
	}
	_ = resp
}

func TestTNodeAdapter_UploadTorrent_HtmlSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/torrent/upload":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html><head><meta name="x-csrf-token" content="htmlcsrf"></head></html>`))
		case "/api/torrent/upload":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html><body>上传成功！</body></html>`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewTNodeAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}
	resp, err := a.UploadTorrent(context.Background(), config, &model.PublishRequest{
		TorrentData: []byte("d4:infod6:lengthi0eee"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Success {
		t.Errorf("expected success for HTML with 成功, got: %s", resp.ErrorMessage)
	}
}

func TestMTeamAdapter_setAPIHeaders(t *testing.T) {
	a := NewMTeamAdapter(NewHTTPDoer(), zap.NewNop())
	req := httptest.NewRequest("POST", "/api/test", nil)
	a.setAPIHeaders(req, "test-api-key-123")
	if req.Header.Get("x-api-key") != "test-api-key-123" {
		t.Errorf("expected x-api-key header, got %s", req.Header.Get("x-api-key"))
	}
	if req.Header.Get("Accept") != "application/json" {
		t.Errorf("expected Accept: application/json")
	}
	if req.Header.Get("User-Agent") != mteamUserAgent {
		t.Errorf("expected custom User-Agent")
	}
}

func TestMTeamAdapter_DownloadViaAPI(t *testing.T) {
	payload := []byte("d4:infod6:lengthi0eee")
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "mykey" {
			t.Errorf("expected x-api-key header")
		}
		switch r.URL.Path {
		case "/api/torrent/genDlToken":
			if r.Method != "POST" {
				t.Errorf("expected POST, got %s", r.Method)
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"data":"%s/dl?sign=abc123"}`, srv.URL)
		case "/dl":
			w.Header().Set("Content-Type", "application/x-bittorrent")
			_, _ = w.Write(payload)
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewMTeamAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, APIKey: "mykey"}
	data, err := a.DownloadTorrent(context.Background(), config, "42")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(payload) {
		t.Errorf("unexpected data")
	}
}

func TestMTeamAdapter_DetailViaAPI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/torrent/detail" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {
				"name": "API Torrent",
				"size": 1073741824,
				"infoHash": "AABBCCDD0123456789AABBCCDD0123456789AABB",
				"category": "movie",
				"seeders": 20,
				"leechers": 5,
				"status": {"discount": "free", "hr": false},
				"mediaInfo": "General\nComplete name: test.mkv",
				"imdb": "tt1234567",
				"tags": ["test", "api"]
			}
		}`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewMTeamAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, APIKey: "mykey"}
	detail, err := a.GetTorrentDetail(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Title != "API Torrent" {
		t.Errorf("unexpected title: %s", detail.Title)
	}
	if detail.InfoHash != "aabbccdd0123456789aabbccdd0123456789aabb" {
		t.Errorf("unexpected infoHash: %s", detail.InfoHash)
	}
	if detail.Size != 1073741824 {
		t.Errorf("unexpected size: %d", detail.Size)
	}
	if detail.Category != "movie" {
		t.Errorf("unexpected category: %s", detail.Category)
	}
}

func TestMTeamAdapter_DetectDiscount_Free(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"status":{"discount":"free"}}}`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewMTeamAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, APIKey: "mykey"}
	result, err := a.DetectDiscount(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Level != model.DiscountFree {
		t.Errorf("expected free, got %s", result.Level)
	}
}

func TestMTeamAdapter_DetectHR(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"status":{"hr":true}}}`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewMTeamAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, APIKey: "mykey"}
	result, err := a.DetectHR(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if !result.HasHR {
		t.Error("expected HR detected")
	}
	if result.SeedTimeH != 72 {
		t.Errorf("expected seed time 72h, got %d", result.SeedTimeH)
	}
}

func TestMTeamAdapter_GetPreciseSLData_API(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"status":{"seeders":10,"leechers":5}}}`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewMTeamAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, APIKey: "mykey"}
	sl, err := a.GetPreciseSLData(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if sl.Seeders != 10 {
		t.Errorf("expected 10 seeders, got %d", sl.Seeders)
	}
	if sl.Leechers != 5 {
		t.Errorf("expected 5 leechers, got %d", sl.Leechers)
	}
}

func TestMTeamAdapter_GetTorrentInfoHash(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"name":"Hash Test","size":0,"infoHash":"CCDDEEFF00112233445566778899AABBCCDDEEFF","category":"movie","seeders":0,"leechers":0,"status":{"discount":"","hr":false},"tags":[]}}`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewMTeamAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, APIKey: "mykey"}
	hash, err := a.GetTorrentInfoHash(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if hash != "ccddeeff00112233445566778899aabbccddeeff" {
		t.Errorf("unexpected hash: %s", hash)
	}
}

func TestMTeamAdapter_UploadViaAPI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/torrent/createOredit" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("x-api-key") != "testkey" {
			t.Errorf("expected x-api-key header, got %s", r.Header.Get("x-api-key"))
		}
		if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "multipart/form-data") {
			t.Errorf("expected multipart content type, got %s", ct)
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"data":{"id":42},"message":"success"}`)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewMTeamAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{
		Domain: srv.URL,
		APIKey: "testkey",
	}
	req := &model.PublishRequest{
		TorrentData: []byte("d4:infod6:lengthi0eee"),
		Title:       "Test Torrent",
		Subtitle:    "Test Sub",
		Description: "Test Desc",
		IMDbLink:    "https://www.imdb.com/title/tt1234567/",
		DoubanLink:  "https://movie.douban.com/subject/123/",
		MediaInfo:   "mediainfo text",
		FormFields: map[string]string{
			"category":   "401",
			"standard":   "1",
			"videoCodec": "1",
			"source":     "1",
		},
	}

	resp, err := a.UploadTorrent(context.Background(), config, req)
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Success {
		t.Errorf("expected success, got error: %s", resp.ErrorMessage)
	}
	if resp.TorrentID != "42" {
		t.Errorf("expected torrent_id 42, got %s", resp.TorrentID)
	}
}

func TestMTeamAdapter_UploadViaWeb(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/upload.php" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<html>details.php?id=99</html>`)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewMTeamAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{
		Domain: srv.URL,
		Cookie: "session=abc",
	}
	req := &model.PublishRequest{
		TorrentData: []byte("d4:infod6:lengthi0eee"),
		Title:       "Web Upload",
	}

	resp, err := a.UploadTorrent(context.Background(), config, req)
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Success {
		t.Errorf("expected success, got: %s", resp.ErrorMessage)
	}
	if resp.TorrentID != "99" {
		t.Errorf("expected torrent_id 99, got %s", resp.TorrentID)
	}
}

func TestMTeamAdapter_SearchTorrents_API(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/torrent/search" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("x-api-key") != "mykey" {
			t.Errorf("expected x-api-key header, got %s", r.Header.Get("x-api-key"))
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"code":"0","data":{"data":[{"id":"123","name":"Test Torrent","size":1073741824,"status":{"seeders":10,"leechers":2,"discount":"FREE"}},{"id":"456","name":"Another Torrent","size":536870912,"status":{"seeders":5,"leechers":1,"discount":""}}]}}`)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewMTeamAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, APIKey: "mykey"}
	results, err := a.SearchTorrents(context.Background(), config, "test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].TorrentID != "123" {
		t.Errorf("expected torrent ID 123, got %s", results[0].TorrentID)
	}
	if results[0].Title != "Test Torrent" {
		t.Errorf("unexpected title: %s", results[0].Title)
	}
	if results[0].Size != 1073741824 {
		t.Errorf("unexpected size: %d", results[0].Size)
	}
	if results[0].Seeders != 10 {
		t.Errorf("expected 10 seeders, got %d", results[0].Seeders)
	}
	if results[1].TorrentID != "456" {
		t.Errorf("expected torrent ID 456, got %s", results[1].TorrentID)
	}
}

func TestMTeamAdapter_VerifyExists_Found(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"code":"0","data":{"data":[{"id":"42","name":"Found","size":0,"status":{"seeders":0,"leechers":0,"discount":""}}]}}`)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewMTeamAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, APIKey: "mykey"}
	found, err := a.VerifyExists(context.Background(), config, "42")
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Error("expected found")
	}
}

func TestMTeamAdapter_VerifyExists_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"code":"0","data":{"data":[{"id":"99","name":"Other","size":0,"status":{"seeders":0,"leechers":0,"discount":""}}]}}`)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewMTeamAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, APIKey: "mykey"}
	found, err := a.VerifyExists(context.Background(), config, "42")
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Error("expected not found")
	}
}

func TestMTeamAdapter_DiscountViaWeb_Free(t *testing.T) {
	html := `<html><body><td class="pro_free">免费</td></body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/details.php" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewMTeamAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}
	result, err := a.DetectDiscount(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Level != model.DiscountFree {
		t.Errorf("expected free, got %s", result.Level)
	}
}

func TestMTeamAdapter_DiscountViaWeb_2xFree(t *testing.T) {
	html := `<html><body><td class="pro_free2up">2x免费</td></body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewMTeamAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}
	result, err := a.DetectDiscount(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Level != model.Discount2xFree {
		t.Errorf("expected 2xfree, got %s", result.Level)
	}
	if result.Multiplier != 2.0 {
		t.Errorf("expected multiplier 2.0, got %f", result.Multiplier)
	}
}

func TestMTeamAdapter_DiscountViaWeb_None(t *testing.T) {
	html := `<html><body><td class="normal">Normal</td></body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewMTeamAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}
	result, err := a.DetectDiscount(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Level != model.DiscountNone {
		t.Errorf("expected none, got %s", result.Level)
	}
}

func TestMTeamAdapter_SLViaWeb_OK(t *testing.T) {
	html := `<html><body>
	<tr><td>做种人数: <span>20</span></td></tr>
	<tr><td>下载人数: <span>8</span></td></tr>
	</body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewMTeamAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}
	sl, err := a.GetPreciseSLData(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if sl.Seeders != 20 {
		t.Errorf("expected 20 seeders, got %d", sl.Seeders)
	}
	if sl.Leechers != 8 {
		t.Errorf("expected 8 leechers, got %d", sl.Leechers)
	}
}

func TestMTeamAdapter_GenDlToken_EmptyData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/torrent/genDlToken" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"data":""}`)
		}
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewMTeamAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL, APIKey: "key"}

	_, err := a.DownloadTorrent(context.Background(), config, "1")
	if err == nil {
		t.Error("expected error for empty download URL")
	}
}

func TestTNode_VerifyExists_Found(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body>
		<tr><td><a href="details.php?id=42">Found Torrent</a></td></tr>
		</body></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewTNodeAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}
	config.Paths.Browse = "/browse.php"

	found, err := a.VerifyExists(context.Background(), config, "42")
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Error("expected found=true")
	}
}

func TestTNode_VerifyExists_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body>
		<tr><td><a href="details.php?id=99">Other Torrent</a></td></tr>
		</body></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewTNodeAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}
	config.Paths.Browse = "/browse.php"

	found, err := a.VerifyExists(context.Background(), config, "42")
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Error("expected found=false")
	}
}

func TestFlexInt_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int64
	}{
		{"number", `42`, 42},
		{"string number", `"123"`, 123},
		{"empty string", `""`, 0},
		{"whitespace", `  `, 0},
		{"invalid string", `"abc"`, 0},
		{"zero", `0`, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var f flexInt
			err := f.UnmarshalJSON([]byte(tt.input))
			if err != nil && tt.name != "whitespace" && tt.name != "invalid string" {
				t.Errorf("unexpected error: %v", err)
			}
			if int64(f) != tt.want {
				t.Errorf("flexInt.UnmarshalJSON(%q) = %d, want %d", tt.input, int64(f), tt.want)
			}
		})
	}
}

func TestMTeam_DetectHR_API_HR(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"status":{"hr":true}}}`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewMTeamAdapter(doer, zap.NewNop())

	result, err := a.DetectHR(context.Background(), &model.SiteConfig{Domain: srv.URL, APIKey: "test-key"}, "1")
	if err != nil {
		t.Fatal(err)
	}
	if !result.HasHR {
		t.Error("expected HasHR=true")
	}
}

func TestMTeam_DetectHR_API_NoHR(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"status":{"hr":false}}}`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewMTeamAdapter(doer, zap.NewNop())

	result, err := a.DetectHR(context.Background(), &model.SiteConfig{Domain: srv.URL, APIKey: "test-key"}, "1")
	if err != nil {
		t.Fatal(err)
	}
	if result.HasHR {
		t.Error("expected HasHR=false")
	}
}

func TestMTeam_DetectHR_Web_HR(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body><img class="hitandrun" src="x.gif" /></body></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewMTeamAdapter(doer, zap.NewNop())

	result, err := a.DetectHR(context.Background(), &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}, "1")
	if err != nil {
		t.Fatal(err)
	}
	if !result.HasHR {
		t.Error("expected HasHR=true")
	}
}

func TestMTeam_DetectHR_Web_NoHR(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<html><body>no hr here</body></html>`)
	}))
	defer srv.Close()

	a := NewMTeamAdapter(&HTTPDoer{Client: srv.Client()}, zap.NewNop())
	config := &model.SiteConfig{}
	config.Domain = srv.URL
	config.Cookie = "test=1"

	result, err := a.DetectHR(context.Background(), config, "42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.HasHR {
		t.Error("expected HasHR=false")
	}
}

func TestMTeam_DetectHRAndDiscount_API_Combined(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"data":{"status":{"discount":"FREE","discountEndTime":"2026-12-31T23:59:59Z","hr":true}}}`)
	}))
	defer srv.Close()

	a := NewMTeamAdapter(&HTTPDoer{Client: srv.Client()}, zap.NewNop())
	config := &model.SiteConfig{}
	config.Domain = srv.URL
	config.APIKey = "test-key"
	config.HR = model.SiteHRConfig{DefaultSeedTimeH: 48}

	hrResult, discResult, err := a.DetectHRAndDiscount(context.Background(), config, "123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hrResult == nil || !hrResult.HasHR {
		t.Error("expected HasHR=true")
	}
	if hrResult.SeedTimeH != 48 {
		t.Errorf("expected SeedTimeH=48, got %d", hrResult.SeedTimeH)
	}
	if discResult == nil || discResult.Level != model.DiscountFree {
		t.Errorf("expected DiscountFree, got %v", discResult)
	}
	if discResult.FreeEndAt == nil {
		t.Error("expected FreeEndAt to be set")
	}
}

func TestMTeam_DetectHRAndDiscount_API_Normal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"data":{"status":{"discount":"NORMAL","hr":false}}}`)
	}))
	defer srv.Close()

	a := NewMTeamAdapter(&HTTPDoer{Client: srv.Client()}, zap.NewNop())
	config := &model.SiteConfig{}
	config.Domain = srv.URL
	config.APIKey = "test-key"

	hrResult, discResult, err := a.DetectHRAndDiscount(context.Background(), config, "456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hrResult == nil || hrResult.HasHR {
		t.Error("expected HasHR=false")
	}
	if discResult == nil || discResult.Level != model.DiscountNone {
		t.Errorf("expected DiscountNone, got %v", discResult)
	}
}

func TestMTeam_DetectHRAndDiscount_WebFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<html><body>no hr no discount</body></html>`)
	}))
	defer srv.Close()

	a := NewMTeamAdapter(&HTTPDoer{Client: srv.Client()}, zap.NewNop())
	config := &model.SiteConfig{}
	config.Domain = srv.URL
	config.Cookie = "test=1"

	hrResult, discResult, err := a.DetectHRAndDiscount(context.Background(), config, "123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hrResult == nil || hrResult.HasHR {
		t.Error("expected HasHR=false")
	}
	if discResult == nil || discResult.Level != model.DiscountNone {
		t.Errorf("expected DiscountNone, got %v", discResult)
	}
}
