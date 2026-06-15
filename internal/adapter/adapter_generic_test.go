package adapter

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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
		<tr><td class="rowhead">基本信息</td><td class="rowfollow"><b><b>大小：</b></b>1.5 GB&nbsp;&nbsp;&nbsp;<b>类型:</b>&nbsp;电影</td></tr>
		<tr><td class="rowhead">种子文件</td><td class="rowfollow">种子Hash：abcdef0123456789abcdef0123456789abcdef01</td></tr>
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
	var appErr *model.AppError
	if !errors.As(err, &appErr) || appErr.Code != ErrAdapterNotFound {
		t.Errorf("expected ErrAdapterNotFound, got %v", err)
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
	if err == nil {
		t.Fatal("expected error for empty body")
	}
	if data != nil {
		t.Errorf("expected nil data, got %d bytes", len(data))
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

func TestGenericAdapter_GetTorrentDetail_DTDD(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><title>DD Torrent</title>
		<dt>基本信息</dt>
		<dd>大小：3.2 GB&nbsp;&nbsp;类型：电视剧&nbsp;&nbsp;编码：H.265&nbsp;&nbsp;分辨率：1080p</dd>
		<dt>种子文件</dt>
		<dd>info_hash: aabbccddeeff00112233445566778899aabbccdd</dd>
		</html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Cookie: "test=1"}
	config.Paths.Detail = "/details.php?id={id}"

	detail, err := a.GetTorrentDetail(context.Background(), config, "10")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Title != "DD Torrent" {
		t.Errorf("unexpected title: %s", detail.Title)
	}
	if detail.InfoHash != "aabbccddeeff00112233445566778899aabbccdd" {
		t.Errorf("unexpected infohash: %s", detail.InfoHash)
	}
	if detail.Size == 0 {
		t.Error("expected non-zero size from dd block")
	}
}

func TestGenericAdapter_GetTorrentDetail_DivLayout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><title>Div Torrent</title>
		<div>基本信息</div>
		<div>大小：5.1 GB&nbsp;&nbsp;类型：电影&nbsp;&nbsp;种子文件
		<div id="kdescr">some description here</div>
		</html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Cookie: "test=1"}
	config.Paths.Detail = "/details.php?id={id}"

	detail, err := a.GetTorrentDetail(context.Background(), config, "20")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Title != "Div Torrent" {
		t.Errorf("unexpected title: %s", detail.Title)
	}
	if detail.Size == 0 {
		t.Error("expected non-zero size from div fallback")
	}
}

func TestGenericAdapter_GetTorrentDetail_CategoryImg(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><title>Cat Torrent</title>
		<tr><td class="rowhead">类别</td><td><img src="cat.gif" alt="动漫"></td></tr>
		<tr><td class="rowhead">基本信息</td><td>大小：1.0 GB&nbsp;&nbsp;类型：动漫</td></tr>
		</html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Cookie: "test=1"}
	config.Paths.Detail = "/details.php?id={id}"

	detail, err := a.GetTorrentDetail(context.Background(), config, "30")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Title != "Cat Torrent" {
		t.Errorf("unexpected title: %s", detail.Title)
	}
	if detail.Category == "" {
		t.Error("expected non-empty category")
	}
}

func TestGenericAdapter_GetTorrentDetail_Subtitle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><title>Sub Torrent</title>
		<tr><td class="rowhead">副标题</td><td class="rowfollow">This is a subtitle</td></tr>
		<tr><td class="rowhead">基本信息</td><td>大小：2.0 GB</td></tr>
		</html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Cookie: "test=1"}
	config.Paths.Detail = "/details.php?id={id}"

	detail, err := a.GetTorrentDetail(context.Background(), config, "40")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Subtitle != "This is a subtitle" {
		t.Errorf("unexpected subtitle: %s", detail.Subtitle)
	}
}

func TestGenericAdapter_UploadTorrent_EmptyData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}
	config.Paths.Upload = "/upload.php"

	req := &model.PublishRequest{TorrentData: []byte{}}
	_, err := a.UploadTorrent(context.Background(), config, req)
	if err == nil {
		t.Fatal("expected error for empty torrent data")
	}
}

func TestGenericAdapter_UploadTorrent_WithFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<html>上传成功</html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}
	config.Paths.Upload = "/upload.php"

	req := &model.PublishRequest{
		TorrentData: []byte("d4:infod...e"),
		Title:       "test title",
		Description: "test description",
		FormFields:  map[string]string{"cat": "401", "source": "bluray"},
	}
	result, err := a.UploadTorrent(context.Background(), config, req)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Error("expected Success=true")
	}
}

func TestGenericAdapter_UploadTTG(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<html>成功 <a href="/t/789/">查看</a></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}
	config.Paths.Upload = "/upload.php"

	req := &model.PublishRequest{
		TorrentData: []byte("d4:infod...e"),
		Title:       "ttg torrent",
		FormFields:  map[string]string{"cat": "1"},
	}
	result, err := a.UploadTorrent(context.Background(), config, req)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Error("expected Success=true")
	}
}

func TestGenericAdapter_UploadYemaPT(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<html>上传成功 <a href="details.php?id=555">查看</a></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}
	config.Paths.Upload = "/upload.php"

	req := &model.PublishRequest{
		TorrentData: []byte("d4:infod...e"),
		Title:       "yema torrent",
	}
	result, err := a.UploadTorrent(context.Background(), config, req)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Error("expected Success=true")
	}
	if result.TorrentID != "555" {
		t.Errorf("expected torrentID 555, got %s", result.TorrentID)
	}
}

func TestGenericAdapter_VerifyExists_Found(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body>
		<tr><td><a href="details.php?id=42">Found Torrent</a></td></tr>
		</body></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL}
	config.Paths.Browse = "/browse.php"

	found, err := a.VerifyExists(context.Background(), config, "42")
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Error("expected found=true")
	}
}

func TestGenericAdapter_VerifyExists_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body>
		<tr><td><a href="details.php?id=99">Other Torrent</a></td></tr>
		</body></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL}
	config.Paths.Browse = "/browse.php"

	found, err := a.VerifyExists(context.Background(), config, "42")
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Error("expected found=false")
	}
}

func TestGenericAdapter_VerifyExists_SearchError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL}
	config.Paths.Browse = "/browse.php"

	found, err := a.VerifyExists(context.Background(), config, "42")
	if err == nil {
		t.Fatal("expected error on search failure")
	}
	if found {
		t.Error("expected found=false on server error")
	}
}

func TestGenericAdapter_SearchTorrents_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body><p>No results</p></body></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL}
	config.Paths.Browse = "/browse.php"

	results, err := a.SearchTorrents(context.Background(), config, "nonexistent", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestGenericAdapter_SearchTorrents_WithSizeAndSL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "search=test") {
			t.Errorf("expected search param, got %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`<html><body>
		<tr><td><a href="details.php?id=100">Big Torrent</a></td>
		<td>4.5 GB</td><td><a href="#">15</a></td><td>3</td></tr>
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
		t.Fatal("expected at least 1 result")
	}
	if results[0].Size == 0 {
		t.Error("expected non-zero size")
	}
}

func TestGenericAdapter_SearchTorrents_WithCategory(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "cat=401") {
			t.Errorf("expected cat param, got %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`<html><body>
		<tr><td><a href="details.php?id=200">Cat Torrent</a></td></tr>
		</body></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL}
	config.Paths.Browse = "/browse.php"

	opts := &model.SearchOptions{Category: "401"}
	results, err := a.SearchTorrents(context.Background(), config, "test", opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) < 1 {
		t.Error("expected at least 1 result")
	}
}

func TestGenericAdapter_SearchTorrents_ExistingQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "&search=test") {
			t.Errorf("expected &search param, got %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`<html><body></body></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL}
	config.Paths.Browse = "/browse.php?sort=seeders"

	results, err := a.SearchTorrents(context.Background(), config, "test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestGenericAdapter_DetectDiscount_API2xFree(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"discount":"2xfree"}`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}
	config.Discount.HasAPI = true
	config.Discount.APIURL = "/api/discount?id={id}"
	config.Discount.Selectors = []string{"2xfree"}

	result, err := a.DetectDiscount(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Level != model.Discount2xFree {
		t.Errorf("expected 2XFREE, got %s", result.Level)
	}
}

func TestGenericAdapter_DetectDiscount_API2xUp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"discount":"2xup"}`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}
	config.Discount.HasAPI = true
	config.Discount.APIURL = "/api/discount?id={id}"
	config.Discount.Selectors = []string{"2xup"}

	result, err := a.DetectDiscount(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Level != model.Discount2xUp {
		t.Errorf("expected 2XUP, got %s", result.Level)
	}
}

func TestGenericAdapter_DetectDiscount_API50(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"discount":"50%"}`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}
	config.Discount.HasAPI = true
	config.Discount.APIURL = "/api/discount?id={id}"
	config.Discount.Selectors = []string{"50%"}

	result, err := a.DetectDiscount(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Level != model.DiscountPercent50 {
		t.Errorf("expected PERCENT_50, got %s", result.Level)
	}
}

func TestGenericAdapter_DetectDiscount_APINone(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"discount":"none"}`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}
	config.Discount.HasAPI = true
	config.Discount.APIURL = "/api/discount?id={id}"
	config.Discount.Selectors = []string{"none"}

	result, err := a.DetectDiscount(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Level != model.DiscountNone {
		t.Errorf("expected NONE, got %s", result.Level)
	}
}

func TestGenericAdapter_DetectDiscount_PageWithClassMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><span class="pro_2xup">2xUp</span></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}
	config.Paths.Detail = "/details?id={id}"
	config.DiscountDetection.DiscountClassMapping = map[string]string{
		"pro_2xup": "2XUP",
	}

	result, err := a.DetectDiscount(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Level != model.Discount2xUp {
		t.Errorf("expected 2XUP from class mapping, got %s", result.Level)
	}
}

func TestGenericAdapter_DetectDiscount_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}
	config.Discount.HasAPI = true
	config.Discount.APIURL = "/api/discount?id={id}"
	config.Discount.Selectors = []string{"custom_nodiscount"}

	result, err := a.DetectDiscount(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Level != model.DiscountNone {
		t.Errorf("expected NONE on API error, got %s", result.Level)
	}
}

func TestGenericAdapter_GetPreciseSLData_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html>
		<td>做种数25</td>
		<td>下载数10</td>
		</html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL}
	config.Paths.Detail = "/details?id={id}"

	sl, err := a.GetPreciseSLData(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if sl.Seeders != 25 {
		t.Errorf("expected 25 seeders, got %d", sl.Seeders)
	}
	if sl.Leechers != 10 {
		t.Errorf("expected 10 leechers, got %d", sl.Leechers)
	}
}

func TestGenericAdapter_GetPreciseSLData_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL}
	config.Paths.Detail = "/details?id={id}"

	_, err := a.GetPreciseSLData(context.Background(), config, "1")
	if err == nil {
		t.Error("expected error for 403")
	}
}

func TestGenericAdapter_GetPreciseSLData_EmptyHTML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body>nothing here</body></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL}
	config.Paths.Detail = "/details?id={id}"

	sl, err := a.GetPreciseSLData(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if sl.Seeders != 0 || sl.Leechers != 0 {
		t.Errorf("expected 0/0, got %d/%d", sl.Seeders, sl.Leechers)
	}
}

func TestGenericAdapter_DetectHR_NoConfig(t *testing.T) {
	a := NewGenericAdapter("generic", NewHTTPDoer(), zap.NewNop())

	result, err := a.DetectHR(context.Background(), &model.SiteConfig{}, "1")
	if err != nil {
		t.Fatal(err)
	}
	if result.HasHR {
		t.Error("expected HasHR=false with no config")
	}
}

func TestGenericAdapter_DetectHR_NoPath(t *testing.T) {
	a := NewGenericAdapter("generic", NewHTTPDoer(), zap.NewNop())

	config := &model.SiteConfig{}
	config.HR.Selectors = []string{"hitandrun"}

	result, err := a.DetectHR(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if result.HasHR {
		t.Error("expected HasHR=false with no detail path")
	}
}

func TestGenericAdapter_DetectHR_MultiplePatterns(t *testing.T) {
	tests := []struct {
		name   string
		html   string
		wantHR bool
	}{
		{"class hitandrun", `<html><img class="hitandrun" src="x.gif" /></html>`, true},
		{"class hitandrun single quote", `<html><img class='hitandrun' src="x.gif" /></html>`, true},
		{"hit and run keyword no match", `<html>hit and run policy</html>`, false},
		{"hit&run keyword no match", `<html>hit&run enabled</html>`, false},
		{"考核 keyword no match", `<html>考核期</html>`, false},
		{"no hr", `<html>normal torrent</html>`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(tt.html))
			}))
			defer srv.Close()

			doer := &HTTPDoer{Client: srv.Client()}
			a := NewGenericAdapter("generic", doer, zap.NewNop())

			config := &model.SiteConfig{Domain: srv.URL}
			config.Paths.Detail = "/details?id={id}"

			result, err := a.DetectHR(context.Background(), config, "1")
			if err != nil {
				t.Fatal(err)
			}
			if result.HasHR != tt.wantHR {
				t.Errorf("expected HasHR=%v, got %v", tt.wantHR, result.HasHR)
			}
		})
	}
}

func TestGenericAdapter_DetectHR_CustomSeedTime(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><span class="hitandrun">HR</span></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL}
	config.Paths.Detail = "/details?id={id}"
	config.HR.DefaultSeedTimeH = 120

	result, err := a.DetectHR(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if !result.HasHR {
		t.Error("expected HasHR=true")
	}
	if result.SeedTimeH != 120 {
		t.Errorf("expected SeedTimeH=120, got %d", result.SeedTimeH)
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

func TestExtractIMDbIDGeneric(t *testing.T) {
	tests := []struct {
		name string
		link string
		want string
	}{
		{"empty", "", ""},
		{"no match", "https://example.com/movie/123", ""},
		{"full URL", "https://www.imdb.com/title/tt1234567/", "tt1234567"},
		{"bare id", "tt9876543", "tt9876543"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractIMDbIDGeneric(tt.link)
			if got != tt.want {
				t.Errorf("extractIMDbIDGeneric(%q) = %q, want %q", tt.link, got, tt.want)
			}
		})
	}
}

func TestGenericAdapter_UploadTTG_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<a href="/t/12345/">redirect</a>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL}
	req := &model.PublishRequest{
		TorrentData: []byte("d4:infod...e"),
		Title:       "Test Torrent",
		Subtitle:    "Sub",
		Description: "Desc",
		IMDbLink:    "https://www.imdb.com/title/tt1234567/",
		DoubanLink:  "https://movie.douban.com/subject/1234567/",
		Anonymous:   true,
		FormFields:  map[string]string{"cat": "401", "tags": "禁转"},
	}

	resp, err := a.uploadTTG(context.Background(), config, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success {
		t.Error("expected success")
	}
	if resp.TorrentID != "12345" {
		t.Errorf("expected torrent ID 12345, got %s", resp.TorrentID)
	}
}

func TestGenericAdapter_UploadTTG_EmptyData(t *testing.T) {
	a := NewGenericAdapter("generic", NewHTTPDoer(), zap.NewNop())
	config := &model.SiteConfig{Domain: "totheglory.im"}
	req := &model.PublishRequest{}

	_, err := a.uploadTTG(context.Background(), config, req)
	if err == nil {
		t.Fatal("expected error for empty data")
	}
}

func TestGenericAdapter_UploadTTG_Forbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	_, err := a.uploadTTG(context.Background(), &model.SiteConfig{Domain: srv.URL}, &model.PublishRequest{TorrentData: []byte("data")})
	if err == nil {
		t.Fatal("expected error for 403")
	}
}

func TestGenericAdapter_UploadTTG_ErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<span class="error">权限不足</span>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	_, err := a.uploadTTG(context.Background(), &model.SiteConfig{Domain: srv.URL}, &model.PublishRequest{TorrentData: []byte("data")})
	if err == nil {
		t.Fatal("expected error for upload failure")
	}
}

func TestGenericAdapter_UploadTTG_KeywordSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<html><body>上传成功！</body></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	resp, err := a.uploadTTG(context.Background(), &model.SiteConfig{Domain: srv.URL}, &model.PublishRequest{TorrentData: []byte("data")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success {
		t.Error("expected success")
	}
}

func TestGenericAdapter_UploadStarSpace_Video(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "video_upload") {
			t.Errorf("expected video_upload path, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<a href="details.php?id=999">link</a>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL}
	req := &model.PublishRequest{
		TorrentData: []byte("d4:infod...e"),
		Title:       "Video Title",
		Subtitle:    "Sub",
		Description: "Desc",
		FormFields: map[string]string{
			"cat":            "movie",
			"medium_sel":     "blu-ray",
			"codec_sel":      "h264",
			"audiocodec_sel": "aac",
			"standard_sel":   "1080p",
			"team_sel":       "myteam",
			"tags":           "官方,中字,3D",
		},
		ExtraFields: map[string]string{"tr_hdr": "hdr10"},
		Screenshots: []string{"https://img.example.com/1.png"},
	}

	resp, err := a.uploadStarSpace(context.Background(), config, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success {
		t.Error("expected success")
	}
	if resp.TorrentID != "999" {
		t.Errorf("expected torrent ID 999, got %s", resp.TorrentID)
	}
}

func TestGenericAdapter_UploadStarSpace_Music(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "music_upload") {
			t.Errorf("expected music_upload path, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<a href="torrent.php?id=555">link</a>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL}
	req := &model.PublishRequest{
		TorrentData: []byte("d4:infod...e"),
		Title:       "Album Title",
		Description: "Music Desc",
		FormFields:  map[string]string{"cat": "music"},
		ExtraFields: map[string]string{
			"artist":         "Artist",
			"year":           "2024",
			"image":          "https://img.example.com/cover.jpg",
			"remaster_year":  "2024",
			"remaster_title": "Deluxe",
		},
	}

	resp, err := a.uploadStarSpace(context.Background(), config, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success {
		t.Error("expected success")
	}
	if resp.TorrentID != "555" {
		t.Errorf("expected torrent ID 555, got %s", resp.TorrentID)
	}
}

func TestGenericAdapter_UploadStarSpace_MusicViaExtraField(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "music_upload") {
			t.Errorf("expected music_upload path, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`成功`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	req := &model.PublishRequest{
		TorrentData: []byte("data"),
		FormFields:  map[string]string{"cat": "other"},
		ExtraFields: map[string]string{"is_music": "true"},
	}

	resp, err := a.uploadStarSpace(context.Background(), &model.SiteConfig{Domain: srv.URL}, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success {
		t.Error("expected success")
	}
}

func TestGenericAdapter_UploadStarSpace_EmptyData(t *testing.T) {
	a := NewGenericAdapter("generic", NewHTTPDoer(), zap.NewNop())
	_, err := a.uploadStarSpace(context.Background(), &model.SiteConfig{Domain: "star-space.io"}, &model.PublishRequest{})
	if err == nil {
		t.Fatal("expected error for empty data")
	}
}

func TestGenericAdapter_DoStarSpaceUpload_Forbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	req, _ := http.NewRequest("POST", srv.URL+"/upload", nil)
	_, err := a.doStarSpaceUpload(req, &model.SiteConfig{Domain: srv.URL})
	if err == nil {
		t.Fatal("expected error for 403")
	}
}

func TestGenericAdapter_DoStarSpaceUpload_ErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<span class="error">种子重复</span>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	req, _ := http.NewRequest("POST", srv.URL+"/upload", nil)
	_, err := a.doStarSpaceUpload(req, &model.SiteConfig{Domain: srv.URL})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGenericAdapter_UploadYemaPT_JSONSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"id":42},"message":""}`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL}
	req := &model.PublishRequest{
		TorrentData: []byte("d4:infod...e"),
		Title:       "YemaPT Title",
		Subtitle:    "Sub",
		Description: "Desc",
		IMDbLink:    "https://www.imdb.com/title/tt9999999/",
		DoubanLink:  "https://movie.douban.com/subject/888888/",
		Anonymous:   true,
		FormFields: map[string]string{
			"cat":          "401",
			"medium_sel":   "bluray",
			"codec_sel":    "h265",
			"tags":         "官方,完结",
			"custom_field": "custom_val",
		},
		ExtraFields: map[string]string{"picture": "https://img.example.com/pic.jpg"},
	}

	resp, err := a.uploadYemaPT(context.Background(), config, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success {
		t.Error("expected success")
	}
	if resp.TorrentID != "42" {
		t.Errorf("expected torrent ID 42, got %s", resp.TorrentID)
	}
}

func TestGenericAdapter_UploadYemaPT_JSONFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":false,"showType":2,"message":"分类错误"}`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	_, err := a.uploadYemaPT(context.Background(), &model.SiteConfig{Domain: srv.URL}, &model.PublishRequest{TorrentData: []byte("data")})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGenericAdapter_UploadYemaPT_HTMLFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<html><body>上传成功</body></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	resp, err := a.uploadYemaPT(context.Background(), &model.SiteConfig{Domain: srv.URL}, &model.PublishRequest{TorrentData: []byte("data")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success {
		t.Error("expected success")
	}
}

func TestGenericAdapter_UploadYemaPT_EmptyData(t *testing.T) {
	a := NewGenericAdapter("generic", NewHTTPDoer(), zap.NewNop())
	_, err := a.uploadYemaPT(context.Background(), &model.SiteConfig{Domain: "yemapt.com"}, &model.PublishRequest{})
	if err == nil {
		t.Fatal("expected error for empty data")
	}
}

func TestGenericAdapter_UploadYemaPT_FinalFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`unknown response`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("generic", doer, zap.NewNop())

	_, err := a.uploadYemaPT(context.Background(), &model.SiteConfig{Domain: srv.URL}, &model.PublishRequest{TorrentData: []byte("data")})
	if err == nil {
		t.Fatal("expected error for unrecognized response")
	}
}
