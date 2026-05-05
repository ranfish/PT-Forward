package adapter

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

func TestGenericAdapter_Framework(t *testing.T) {
	a := NewGenericAdapter("test_fw", NewHTTPDoer(), zap.NewNop())
	if a.Framework() != "test_fw" {
		t.Errorf("expected test_fw, got %s", a.Framework())
	}
}

func TestGenericAdapter_DownloadTorrent_OK(t *testing.T) {
	payload := []byte("d4:infod...e")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "sid=abc" {
			t.Error("missing Cookie header")
		}
		w.Header().Set("Content-Type", "application/x-bittorrent")
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	config := &model.SiteConfig{
		Domain:  srv.URL,
		Passkey: "pk123",
		Cookie:  "sid=abc",
	}
	config.Paths.Detail = "/download.php?id={id}&passkey={passkey}"

	data, err := a.DownloadTorrent(context.Background(), config, "42")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(payload) {
		t.Errorf("unexpected data: %q", data)
	}
}

func TestGenericAdapter_DownloadTorrent_NoURL(t *testing.T) {
	a := NewGenericAdapter("generic", NewHTTPDoer(), zap.NewNop())
	config := &model.SiteConfig{Domain: "https://x.com"}

	_, err := a.DownloadTorrent(context.Background(), config, "1")
	if err == nil {
		t.Fatal("expected error for no download URL")
	}
}

func TestGenericAdapter_DownloadTorrent_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL}
	config.Paths.Detail = "/dl?id={id}"

	_, err := a.DownloadTorrent(context.Background(), config, "1")
	if err == nil {
		t.Fatal("expected error for 403")
	}
}

func TestGenericAdapter_ParseRSS_NotImplemented(t *testing.T) {
	a := NewGenericAdapter("generic", NewHTTPDoer(), zap.NewNop())
	_, err := a.ParseRSS(context.Background(), "", &model.SiteConfig{})
	if err == nil {
		t.Error("ParseRSS should error")
	}
}

func TestGenericAdapter_GetTorrentDetail_NoPath(t *testing.T) {
	a := NewGenericAdapter("generic", NewHTTPDoer(), zap.NewNop())
	_, err := a.GetTorrentDetail(context.Background(), &model.SiteConfig{}, "1")
	if err == nil {
		t.Error("GetTorrentDetail should error without path")
	}
}

func TestGenericAdapter_GetTorrentDetail_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><title>Test Torrent</title>
		<td>info_hash</td><td>abcdef0123456789abcdef0123456789abcdef01</td>
		<td>大小</td><td>1.5 GB</td>
		</html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Cookie: "test=1"}
	config.Paths.Detail = "/details.php?id={id}"

	detail, err := a.GetTorrentDetail(context.Background(), config, "42")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Title != "Test Torrent" {
		t.Errorf("unexpected title: %s", detail.Title)
	}
	if detail.InfoHash != "abcdef0123456789abcdef0123456789abcdef01" {
		t.Errorf("unexpected infohash: %s", detail.InfoHash)
	}
	if detail.Size == 0 {
		t.Error("expected non-zero size")
	}
}

func TestGenericAdapter_GetBatchSLData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html>Seeders: 10, Leechers: 5</html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL}
	config.Paths.Detail = "/details?id={id}"

	result, err := a.GetBatchSLData(context.Background(), config, []string{"1", "2"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 results, got %d", len(result))
	}
}

func TestGenericAdapter_GetPreciseSLData_NoPath(t *testing.T) {
	a := NewGenericAdapter("generic", NewHTTPDoer(), zap.NewNop())
	_, err := a.GetPreciseSLData(context.Background(), &model.SiteConfig{}, "1")
	if err == nil {
		t.Error("GetPreciseSLData should error without path")
	}
}

func TestGenericAdapter_DetectDiscount(t *testing.T) {
	a := NewGenericAdapter("generic", NewHTTPDoer(), zap.NewNop())
	result, err := a.DetectDiscount(context.Background(), &model.SiteConfig{}, "1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Level != model.DiscountNone {
		t.Errorf("expected NONE, got %s", result.Level)
	}
}

func TestGenericAdapter_DetectDiscount_Free(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><span class="pro_free">Free</span></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL}
	config.Paths.Detail = "/details?id={id}"

	result, err := a.DetectDiscount(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Level != model.DiscountFree {
		t.Errorf("expected FREE, got %s", result.Level)
	}
}

func TestGenericAdapter_DetectHR(t *testing.T) {
	a := NewGenericAdapter("generic", NewHTTPDoer(), zap.NewNop())
	result, err := a.DetectHR(context.Background(), &model.SiteConfig{}, "1")
	if err != nil {
		t.Fatal(err)
	}
	if result.HasHR {
		t.Error("expected HasHR=false")
	}
}

func TestGenericAdapter_DetectHR_Found(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><span class="hitandrun">HR</span></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL}
	config.Paths.Detail = "/details?id={id}"
	config.HR.Selectors = []string{"hitandrun"}

	result, err := a.DetectHR(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if !result.HasHR {
		t.Error("expected HasHR=true")
	}
	if result.SeedTimeH != 72 {
		t.Errorf("expected SeedTimeH=72, got %d", result.SeedTimeH)
	}
}

func TestGenericAdapter_UploadTorrent_NoPath(t *testing.T) {
	a := NewGenericAdapter("generic", NewHTTPDoer(), zap.NewNop())
	_, err := a.UploadTorrent(context.Background(), &model.SiteConfig{}, nil)
	if err == nil {
		t.Error("UploadTorrent should error without path")
	}
}

func TestGenericAdapter_SearchTorrents_NoPath(t *testing.T) {
	a := NewGenericAdapter("generic", NewHTTPDoer(), zap.NewNop())
	_, err := a.SearchTorrents(context.Background(), &model.SiteConfig{}, "test", nil)
	if err == nil {
		t.Error("SearchTorrents should error without path")
	}
}

func TestGenericAdapter_SearchTorrents_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body>
		<tr><td><a href="details.php?id=100">Test Torrent</a></td></tr>
		</body></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL}
	config.Paths.Browse = "/browse.php"

	results, err := a.SearchTorrents(context.Background(), config, "test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) < 1 {
		t.Error("expected at least 1 result")
	}
}

func TestGenericAdapter_GetTorrentInfoHash_NoPath(t *testing.T) {
	a := NewGenericAdapter("generic", NewHTTPDoer(), zap.NewNop())
	_, err := a.GetTorrentInfoHash(context.Background(), &model.SiteConfig{}, "1")
	if err == nil {
		t.Error("GetTorrentInfoHash should error without path")
	}
}

func TestGenericAdapter_GetTorrentInfoHash_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><title>Test</title>
		<td>info_hash</td><td>aaBBccDDeeFF00112233445566778899aaBBccDD</td>
		</html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL}
	config.Paths.Detail = "/details?id={id}"

	hash, err := a.GetTorrentInfoHash(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if hash != "aabbccddeeff00112233445566778899aabbccdd" {
		t.Errorf("unexpected hash: %s", hash)
	}
}

func TestGenericAdapter_SupportsSearchByPiecesHash(t *testing.T) {
	a := NewGenericAdapter("generic", NewHTTPDoer(), zap.NewNop())
	if a.SupportsSearchByPiecesHash() {
		t.Error("SupportsSearchByPiecesHash should be false")
	}
}

func TestGenericAdapter_UploadTorrent_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}
	config.Paths.Upload = "/upload.php"

	req := &model.PublishRequest{TorrentData: []byte("d4:infod...e")}
	result, err := a.UploadTorrent(context.Background(), config, req)
	if err == nil {
		t.Fatal("expected error for 500")
	}
	var appErr *model.AppError
	if !errors.As(err, &appErr) || appErr.Code != 15001 {
		t.Fatalf("expected AppError 15001, got %v", err)
	}
	if result != nil {
		t.Error("expected nil result")
	}
}

func TestGenericAdapter_UploadTorrent_Forbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}
	config.Paths.Upload = "/upload.php"

	req := &model.PublishRequest{TorrentData: []byte("d4:infod...e")}
	result, err := a.UploadTorrent(context.Background(), config, req)
	if err == nil {
		t.Fatal("expected error for 403")
	}
	var appErr *model.AppError
	if !errors.As(err, &appErr) || appErr.Code != 14003 {
		t.Fatalf("expected AppError 14003, got %v", err)
	}
	if result != nil {
		t.Error("expected nil result")
	}
}

func TestGenericAdapter_UploadTorrent_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Cookie") != "sid=test" {
			t.Error("missing Cookie header")
		}
		contentType := r.Header.Get("Content-Type")
		if contentType == "" || len(contentType) < 10 {
			t.Error("missing Content-Type for multipart")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<html>上传成功 <a href="details.php?id=456">查看</a></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}
	config.Paths.Upload = "/upload.php"

	req := &model.PublishRequest{TorrentData: []byte("d4:infod...e"), Title: "test torrent"}
	result, err := a.UploadTorrent(context.Background(), config, req)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Error("expected Success=true")
	}
	if result.TorrentID != "456" {
		t.Errorf("expected torrentID 456, got %s", result.TorrentID)
	}
}

func TestGenericAdapter_UploadTorrent_Duplicate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<html><span class="error">种子已存在</span></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}
	config.Paths.Upload = "/upload.php"

	req := &model.PublishRequest{TorrentData: []byte("d4:infod...e")}
	result, err := a.UploadTorrent(context.Background(), config, req)
	if err == nil {
		t.Fatal("expected error for duplicate")
	}
	var appErr *model.AppError
	if !errors.As(err, &appErr) || appErr.Code != 15001 {
		t.Fatalf("expected AppError 15001, got %v", err)
	}
	_ = result
}

func TestGenericAdapter_DownloadTorrent_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL}
	config.Paths.Detail = "/dl?id={id}"

	_, err := a.DownloadTorrent(context.Background(), config, "999")
	if err == nil {
		t.Fatal("expected error for 404")
	}
}

func TestGenericAdapter_DownloadTorrent_EmptyBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL}
	config.Paths.Detail = "/dl?id={id}"

	data, err := a.DownloadTorrent(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 0 {
		t.Errorf("expected empty body, got %d bytes", len(data))
	}
}

func TestGenericAdapter_SearchTorrents_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL}
	config.Paths.Browse = "/browse.php"

	_, err := a.SearchTorrents(context.Background(), config, "test", nil)
	if err == nil {
		t.Error("expected error for 503")
	}
}

func TestGenericAdapter_GetTorrentDetail_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL}
	config.Paths.Detail = "/details?id={id}"

	_, err := a.GetTorrentDetail(context.Background(), config, "1")
	if err == nil {
		t.Error("expected error for 500")
	}
}

func TestGenericAdapter_DetectDiscount_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL}
	config.Paths.Detail = "/details?id={id}"

	result, err := a.DetectDiscount(context.Background(), config, "1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Level != model.DiscountNone {
		t.Errorf("expected NONE on server error, got %s", result.Level)
	}
}

func TestGenericAdapter_BuildGenericURL(t *testing.T) {
	tests := []struct {
		name    string
		config  *model.SiteConfig
		pathTpl string
		id      string
		want    string
	}{
		{
			"full template",
			&model.SiteConfig{Domain: "example.com", Passkey: "pk1"},
			"/download.php?id={id}&passkey={passkey}",
			"42",
			"https://example.com/download.php?id=42&passkey=pk1",
		},
		{
			"absolute URL",
			&model.SiteConfig{},
			"https://cdn.example.com/torrent/{id}",
			"99",
			"https://cdn.example.com/torrent/99",
		},
		{
			"empty template",
			&model.SiteConfig{Domain: "example.com"},
			"",
			"1",
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildGenericURL(tt.config, tt.pathTpl, tt.id)
			if got != tt.want {
				t.Errorf("buildGenericURL() = %q, want %q", got, tt.want)
			}
		})
	}
}
