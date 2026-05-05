package adapter

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

func TestNexusPHP_Framework(t *testing.T) {
	a := NewNexusPHPAdapter(NewHTTPDoer(), zap.NewNop())
	if a.Framework() != "nexusphp" {
		t.Errorf("expected nexusphp, got %s", a.Framework())
	}
}

func TestNexusPHP_ParseRSS(t *testing.T) {
	a := NewNexusPHPAdapter(NewHTTPDoer(), zap.NewNop())
	_, err := a.ParseRSS(context.Background(), "", &model.SiteConfig{})
	if err == nil {
		t.Error("ParseRSS should error")
	}
}

func TestNexusPHP_DownloadTorrent_OK(t *testing.T) {
	payload := []byte("d4:infod...e")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("id") != "42" {
			t.Error("missing id param")
		}
		if r.URL.Query().Get("passkey") != "pk1" {
			t.Error("missing passkey")
		}
		if r.Header.Get("Cookie") != "sid=test" {
			t.Error("missing cookie")
		}
		w.Header().Set("Content-Type", "application/x-bittorrent")
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewNexusPHPAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL, Passkey: "pk1", Cookie: "sid=test"}

	data, err := a.DownloadTorrent(context.Background(), config, "42")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(payload) {
		t.Errorf("unexpected data")
	}
}

func TestNexusPHP_DownloadTorrent_Forbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewNexusPHPAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL}

	_, err := a.DownloadTorrent(context.Background(), config, "1")
	if err == nil {
		t.Fatal("expected error for 403")
	}
}

func TestNexusPHP_DownloadTorrent_HTML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html>error</html>"))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewNexusPHPAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL}

	_, err := a.DownloadTorrent(context.Background(), config, "1")
	if err == nil {
		t.Fatal("expected error for HTML response")
	}
}

func TestNexusPHP_GetTorrentDetail_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><title>Movie 2024 1080p</title>
		<td>info_hash</td><td>aabbccddeeff00112233445566778899aabbccdd</td>
		<td>大小<td>2.5 GB</td></td>
		<td>分类<td>电影</td></td>
		<span class="tag tag-free">Free</span>
		</html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewNexusPHPAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL, Cookie: "test=1"}

	detail, err := a.GetTorrentDetail(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Title != "Movie 2024 1080p" {
		t.Errorf("title: %s", detail.Title)
	}
	if detail.InfoHash != "aabbccddeeff00112233445566778899aabbccdd" {
		t.Errorf("hash: %s", detail.InfoHash)
	}
	if detail.Size == 0 {
		t.Error("expected non-zero size")
	}
	if detail.Category != "电影" {
		t.Errorf("category: %s", detail.Category)
	}
	if len(detail.Tags) == 0 {
		t.Error("expected tags")
	}
}

func TestNexusPHP_GetTorrentDetail_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewNexusPHPAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL}

	_, err := a.GetTorrentDetail(context.Background(), config, "1")
	if err == nil {
		t.Error("expected error")
	}
}

func TestNexusPHP_DetectDiscount_Free(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><span class="pro_free">Free</span></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewNexusPHPAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL}

	result, err := a.DetectDiscount(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Level != model.DiscountFree {
		t.Errorf("expected FREE, got %s", result.Level)
	}
}

func TestNexusPHP_DetectDiscount_2xUp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><span class="pro_2up">2xUp</span></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewNexusPHPAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL}

	result, err := a.DetectDiscount(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Level != model.Discount2xUp {
		t.Errorf("expected 2XUP, got %s", result.Level)
	}
}

func TestNexusPHP_DetectDiscount_API(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"discount": "free"}`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewNexusPHPAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL}
	config.Discount.HasAPI = true
	config.Discount.APIURL = "/api/discount?id={id}"
	config.Discount.Selectors = []string{"free"}

	result, err := a.DetectDiscount(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Level != model.DiscountFree {
		t.Errorf("expected FREE, got %s", result.Level)
	}
}

func TestNexusPHP_DetectDiscount_None(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html>Normal torrent</html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewNexusPHPAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL}

	result, err := a.DetectDiscount(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Level != model.DiscountNone {
		t.Errorf("expected NONE, got %s", result.Level)
	}
}

func TestNexusPHP_DetectHR(t *testing.T) {
	tests := []struct {
		name      string
		html      string
		selectors []string
		hasHR     bool
	}{
		{"no selectors", "<html>normal</html>", nil, false},
		{"hit and run", "<html>Hit and Run policy</html>", []string{"hr"}, true},
		{"考核", "<html>考核期内</html>", []string{"hr"}, true},
		{"no HR", "<html>normal torrent</html>", []string{"hr"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(tt.html))
			}))
			defer srv.Close()

			doer := &HTTPDoer{Client: srv.Client()}
			a := NewNexusPHPAdapter(doer, zap.NewNop())
			config := &model.SiteConfig{Domain: srv.URL}
			config.HR.Selectors = tt.selectors

			result, err := a.DetectHR(context.Background(), config, "1")
			if err != nil {
				t.Fatal(err)
			}
			if result.HasHR != tt.hasHR {
				t.Errorf("expected HasHR=%v, got %v", tt.hasHR, result.HasHR)
			}
		})
	}
}

func TestNexusPHP_UploadTorrent_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Cookie") != "sid=test" {
			t.Error("missing cookie")
		}
		_, _ = w.Write([]byte(`<html>上传成功 <a href="details.php?id=789">查看</a></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewNexusPHPAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}

	req := &model.PublishRequest{
		TorrentData: []byte("d4:infod...e"),
		Title:       "Test",
		Subtitle:    "Sub",
		Description: "Desc",
		Anonymous:   true,
	}
	result, err := a.UploadTorrent(context.Background(), config, req)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Error("expected success")
	}
	if result.TorrentID != "789" {
		t.Errorf("expected 789, got %s", result.TorrentID)
	}
}

func TestNexusPHP_UploadTorrent_Forbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewNexusPHPAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}

	result, err := a.UploadTorrent(context.Background(), config, &model.PublishRequest{TorrentData: []byte("data")})
	if err != nil {
		t.Fatal(err)
	}
	if result.Success {
		t.Error("expected failure")
	}
}

func TestNexusPHP_UploadTorrent_EmptyData(t *testing.T) {
	a := NewNexusPHPAdapter(NewHTTPDoer(), zap.NewNop())
	_, err := a.UploadTorrent(context.Background(), &model.SiteConfig{}, &model.PublishRequest{})
	if err == nil {
		t.Error("expected error for empty data")
	}
}

func TestNexusPHP_SearchTorrents_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("search") != "test" {
			t.Error("missing search param")
		}
		_, _ = w.Write([]byte(`<html>
		<tr class="torrent-row"><td><a href="details.php?id=10">Movie A</a></td><td>1.5 GB</td></tr>
		<tr class="torrent-row"><td><a href="details.php?id=20">Movie B</a></td><td>700 MB</td></tr>
		</html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewNexusPHPAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL}

	results, err := a.SearchTorrents(context.Background(), config, "test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2, got %d", len(results))
	}
	if results[0].TorrentID != "10" {
		t.Errorf("first ID: %s", results[0].TorrentID)
	}
}

func TestNexusPHP_SearchTorrents_WithCategory(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cat := r.URL.Query().Get("cat")
		if cat != "123" {
			t.Errorf("expected cat=123, got %s", cat)
		}
		_, _ = w.Write([]byte(`<html></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewNexusPHPAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL}

	_, err := a.SearchTorrents(context.Background(), config, "test", &model.SearchOptions{Category: "123"})
	if err != nil {
		t.Fatal(err)
	}
}

func TestNexusPHP_GetTorrentInfoHash_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><title>Test</title>
		<td>info_hash</td><td>aaBBccDDeeFF00112233445566778899aaBBccDD</td></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewNexusPHPAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL}

	hash, err := a.GetTorrentInfoHash(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if hash != "aabbccddeeff00112233445566778899aabbccdd" {
		t.Errorf("hash: %s", hash)
	}
}

func TestNexusPHP_SupportsSearchByPiecesHash(t *testing.T) {
	a := NewNexusPHPAdapter(NewHTTPDoer(), zap.NewNop())
	if a.SupportsSearchByPiecesHash() {
		t.Error("should be false")
	}
}

func TestNexusPHP_GetBatchSLData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html>Seeders: 10, Leechers: 5</html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewNexusPHPAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL}

	result, err := a.GetBatchSLData(context.Background(), config, []string{"1", "2"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2, got %d", len(result))
	}
}

func TestParseSizeStr(t *testing.T) {
	tests := []struct {
		input  string
		expect int64
	}{
		{"1 GB", 1073741824},
		{"2.5 GB", 2684354560},
		{"500 MB", 524288000},
		{"1 TB", 1099511627776},
		{"1024 KB", 1048576},
		{"invalid", 0},
		{"", 0},
		{"1,024 MB", 1073741824},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseSizeStr(tt.input)
			if got != tt.expect {
				t.Errorf("parseSizeStr(%q) = %d, want %d", tt.input, got, tt.expect)
			}
		})
	}
}

func TestExtractTags(t *testing.T) {
	html := `<span class="tag tag-free">Free</span><span class="tag tag-2x">2x</span>`
	tags := extractTags(html)
	if len(tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(tags))
	}
	if tags[0] != "Free" {
		t.Errorf("first tag: %s", tags[0])
	}
}

func TestBuildURL(t *testing.T) {
	tests := []struct {
		domain string
		path   string
		id     string
		pk     string
		want   string
	}{
		{"example.com", "/download.php", "42", "pk1", "https://example.com/download.php?id=42&passkey=pk1"},
		{"https://x.com", "/dl.php", "1", "", "https://x.com/dl.php?id=1"},
	}

	for _, tt := range tests {
		got := buildURL(tt.domain, tt.path, tt.id, tt.pk)
		if got != tt.want {
			t.Errorf("buildURL() = %q, want %q", got, tt.want)
		}
	}
}

func TestPool_Get(t *testing.T) {
	pool := NewPool(NewFactory(zap.NewNop()), zap.NewNop())

	a, err := pool.Get(context.Background(), "test.com", "nexusphp")
	if err != nil {
		t.Fatal(err)
	}
	if a == nil {
		t.Fatal("expected adapter")
	}

	a2, _ := pool.Get(context.Background(), "test.com", "nexusphp")
	if a != a2 {
		t.Error("expected cached adapter")
	}
}

func TestPool_GetWithFramework(t *testing.T) {
	pool := NewPool(NewFactory(zap.NewNop()), zap.NewNop())

	a, err := pool.GetWithFramework(context.Background(), "test.com", "nexusphp")
	if err != nil {
		t.Fatal(err)
	}
	if a == nil {
		t.Fatal("expected adapter")
	}
	if a.Framework() != "nexusphp" {
		t.Errorf("expected nexusphp, got %s", a.Framework())
	}
}

func TestPool_Rebuild(t *testing.T) {
	pool := NewPool(NewFactory(zap.NewNop()), zap.NewNop())
	if _, err := pool.Get(context.Background(), "test.com", "generic"); err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if pool.Count() != 1 {
		t.Error("expected 1")
	}

	if err := pool.Rebuild(context.Background(), "test.com"); err != nil {
		t.Fatalf("Rebuild failed: %v", err)
	}
	if pool.Count() != 0 {
		t.Error("expected 0 after rebuild")
	}
}

func TestPool_Remove(t *testing.T) {
	pool := NewPool(NewFactory(zap.NewNop()), zap.NewNop())
	if _, err := pool.Get(context.Background(), "a.com", "generic"); err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if _, err := pool.Get(context.Background(), "b.com", "generic"); err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if pool.Count() != 2 {
		t.Error("expected 2")
	}

	pool.Remove("a.com")
	if pool.Count() != 1 {
		t.Errorf("expected 1 after remove, got %d", pool.Count())
	}
}

func TestPool_Close(t *testing.T) {
	pool := NewPool(NewFactory(zap.NewNop()), zap.NewNop())
	if _, err := pool.Get(context.Background(), "x.com", "generic"); err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if err := pool.Close(context.Background()); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	if pool.Count() != 0 {
		t.Error("expected 0 after close")
	}
}

func TestPool_Start(t *testing.T) {
	pool := NewPool(NewFactory(zap.NewNop()), zap.NewNop())
	if err := pool.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestIsMusicCategory(t *testing.T) {
	tests := []struct {
		cat  string
		want bool
	}{
		{"406", true},
		{"408", true},
		{"409", true},
		{"401", false},
		{"402", false},
		{"music", true},
		{"HQ Audio", true},
		{"音频", true},
		{"Movies", false},
		{"", false},
	}
	for _, tt := range tests {
		got := isMusicCategory(tt.cat)
		if got != tt.want {
			t.Errorf("isMusicCategory(%q) = %v, want %v", tt.cat, got, tt.want)
		}
	}
}

func TestNexusPHPUpload_MusicRouting(t *testing.T) {
	var uploadPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uploadPath = r.URL.Path
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `details.php?id=1`)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewNexusPHPAdapter(doer, zap.NewNop())

	t.Run("music category routes to takeupload", func(t *testing.T) {
		config := &model.SiteConfig{
			Domain: srv.URL,
			Cookie: "session=test",
			SiteDefault: model.SiteDefault{
				Paths: model.SitePathsConfig{
					Upload:     "/upload.php",
					TakeUpload: "/upload_music.php",
				},
			},
		}
		req := &model.PublishRequest{
			TorrentData: []byte("d4:infod6:lengthi0eee"),
			Title:       "Test",
			FormFields:  map[string]string{"category": "408"},
		}
		_, _ = a.UploadTorrent(context.Background(), config, req)
		if uploadPath != "/upload_music.php" {
			t.Errorf("expected /upload_music.php, got %s", uploadPath)
		}
	})

	t.Run("non-music category uses default upload", func(t *testing.T) {
		config := &model.SiteConfig{
			Domain: srv.URL,
			Cookie: "session=test",
			SiteDefault: model.SiteDefault{
				Paths: model.SitePathsConfig{
					Upload:     "/upload.php",
					TakeUpload: "/upload_music.php",
				},
			},
		}
		req := &model.PublishRequest{
			TorrentData: []byte("d4:infod6:lengthi0eee"),
			Title:       "Test",
			FormFields:  map[string]string{"category": "401"},
		}
		_, _ = a.UploadTorrent(context.Background(), config, req)
		if uploadPath != "/upload.php" {
			t.Errorf("expected /upload.php, got %s", uploadPath)
		}
	})
}
