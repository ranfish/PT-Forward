package adapter

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
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
		fmt.Fprintf(w, `<html><title>Movie 2024 1080p</title>
		<tr><td class="rowhead">类别与标签</td><td class="rowfollow"><a href="torrents.php?cat=401"><img class="c_movie" src="pic/cattrans.gif" alt="电影 (Movie)" /></a></td></tr>
		<tr><td class="rowhead">副标题</td><td class="rowfollow">好东西/爱情神话2 | 导演: 邵艺辉</td></tr>
		<tr><td class="rowhead">基本信息</td><td class="rowfollow"><b><b>大小：</b></b>2.5 GB&nbsp;&nbsp;&nbsp;<b>类型:</b>&nbsp;电影&nbsp;&nbsp;&nbsp;<b>编码:</b>&nbsp;H.265&nbsp;&nbsp;&nbsp;<b>分辨率:</b>&nbsp;1080p&nbsp;&nbsp;&nbsp;<b>媒介:</b>&nbsp;WEB-DL&nbsp;&nbsp;&nbsp;<b>音频编码:</b>&nbsp;DDP&nbsp;&nbsp;&nbsp;<b>制作组:</b>&nbsp;CMCT&nbsp;&nbsp;&nbsp;<b>地区:</b>&nbsp;大陆</td></tr>
		<tr><td class="rowhead">种子文件</td><td class="rowfollow"><b>文件数：</b>3个文件 &nbsp; &nbsp; 种子Hash：aabbccddeeff00112233445566778899aabbccdd<br /></td></tr>
		<div id='kdescr'>[img]poster[/img] some description</div>
		<span class="tag tag-free">Free</span>
		</html>`)
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
	if detail.Codec != "H.265" {
		t.Errorf("codec: %s", detail.Codec)
	}
	if detail.Resolution != "1080p" {
		t.Errorf("resolution: %s", detail.Resolution)
	}
	if detail.Source != "WEB-DL" {
		t.Errorf("source: %s", detail.Source)
	}
	if detail.AudioCodec != "DDP" {
		t.Errorf("audio_codec: %s", detail.AudioCodec)
	}
	if detail.ReleaseGroup != "CMCT" {
		t.Errorf("release_group: %s", detail.ReleaseGroup)
	}
	if detail.Region != "大陆" {
		t.Errorf("region: %s", detail.Region)
	}
	if detail.Subtitle != "好东西/爱情神话2 | 导演: 邵艺辉" {
		t.Errorf("subtitle: %s", detail.Subtitle)
	}
	if detail.Description == "" {
		t.Error("expected description")
	}
	if len(detail.Tags) == 0 {
		t.Error("expected tags")
	}
}

func TestNexusPHP_GetTorrentDetail_CSSNoise(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `<html>
		<link rel="stylesheet" href="styles/curtain_imageresizer.css?20241021" type="text/css" />
		<style>body { font-size: 14px; }</style>
		<title>CSS Noise Test</title>
		<td>info_hash</td><td>aabbccddeeff00112233445566778899aabbccdd</td>
		<tr><td class="rowhead">基本信息</td><td><b><b>大小：</b></b>1.01 GB</td></tr>
		<tr><td class="rowhead">类别与标签</td><td class="rowfollow"><a href="torrents.php?cat=404"><img class="c_tv" src="pic/cattrans.gif" alt="电视剧 (TV Series)" /></a></td></tr>
		<div>File Size: 510 MiB</div>
		<div>Size: 449 MiB (88%%)</div>
		</html>`)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewNexusPHPAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL, Cookie: "test=1"}

	detail, err := a.GetTorrentDetail(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	sizeGB := float64(detail.Size) / 1024 / 1024 / 1024
	if sizeGB < 0.9 || sizeGB > 1.2 {
		t.Errorf("expected ~1.01 GB, got %.2f GB", sizeGB)
	}
	if detail.Category != "电视剧" {
		t.Errorf("category: %s", detail.Category)
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

func TestNexusPHPUpload_MusicFieldMapping(t *testing.T) {
	var receivedFields url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseMultipartForm(10 << 20)
		receivedFields = r.MultipartForm.Value
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `details.php?id=1`)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewNexusPHPAdapter(doer, zap.NewNop())

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

	t.Run("music fields mapped to upload_music form names", func(t *testing.T) {
		req := &model.PublishRequest{
			TorrentData: []byte("d4:infod6:lengthi0eee"),
			Title:       "Test Album",
			FormFields: map[string]string{
				"category":      "408",
				"music_artist":  "SomeArtist",
				"music_album":   "GreatAlbum",
				"music_year":    "2024",
				"music_format":  "1",
				"music_medium":  "1",
				"music_publish": "1",
			},
		}
		_, _ = a.UploadTorrent(context.Background(), config, req)

		if v := receivedFields.Get("type"); v != "408" {
			t.Errorf("category not mapped to type: got %q", v)
		}
		if v := receivedFields.Get("artists"); v != "SomeArtist" {
			t.Errorf("music_artist not mapped to artists: got %q", v)
		}
		if v := receivedFields.Get("album"); v != "GreatAlbum" {
			t.Errorf("music_album not mapped to album: got %q", v)
		}
		if v := receivedFields.Get("year"); v != "2024" {
			t.Errorf("music_year not mapped to year: got %q", v)
		}
		if v := receivedFields.Get("format_type"); v != "1" {
			t.Errorf("music_format not mapped to format_type: got %q", v)
		}
		if v := receivedFields.Get("medium_type"); v != "1" {
			t.Errorf("music_medium not mapped to medium_type: got %q", v)
		}
		if v := receivedFields.Get("publish_type"); v != "1" {
			t.Errorf("music_publish not mapped to publish_type: got %q", v)
		}
	})
}

func TestNexusPHP_GetTorrentDetail_EnglishColon(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `<html><title>Test Torrent</title>
		<tr >
		<td  class="rowhead nowrap" valign="top" align="right">基本信息</td>
		<td  class="rowfollow" valign="top" align="left"><b>大小:</b>&nbsp;246.407 MiB&nbsp;&nbsp;&nbsp;<b>类型:</b>&nbsp;Lossless Music</td>
		</tr>
		<tr >
		<td  class="rowhead nowrap" valign="top" align="right">种子信息</td>
		<td  class="rowfollow" valign="top" align="left"><b>种子散列值:</b>&nbsp;3da3b3a67d3490c45b7beaad64881ba393f8530f</td>
		</tr>
		</html>`)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewNexusPHPAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL, Cookie: "test=1"}

	detail, err := a.GetTorrentDetail(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	sizeMB := float64(detail.Size) / 1024 / 1024
	if sizeMB < 245 || sizeMB > 248 {
		t.Errorf("expected ~246 MB, got %.2f MB", sizeMB)
	}
	if detail.Category != "音乐" {
		t.Errorf("category: %s", detail.Category)
	}
	if detail.InfoHash != "3da3b3a67d3490c45b7beaad64881ba393f8530f" {
		t.Errorf("hash: %s", detail.InfoHash)
	}
}

func TestNexusPHP_GetTorrentDetail_DataHashAttr(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `<html><title>Test</title>
		<tr><td class="rowhead">基本信息</td><td class="rowfollow"><b><b>大小：</b></b>8.5 GB&nbsp;&nbsp;&nbsp;<b>类型:</b>&nbsp;剧集</td></tr>
		<tr><td class="rowhead">种子文件</td><td class="rowfollow">
		<a id="copyTorrentHashLink" href="javascript:void(0)" data-hash="d7331ff3492618d2ca68cfd1938ed01b3bb8f0b7" onclick="copyTorrentHash()"><b>种子Hash</b></a>
		</td></tr>
		</html>`)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewNexusPHPAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL, Cookie: "test=1"}

	detail, err := a.GetTorrentDetail(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if detail.InfoHash != "d7331ff3492618d2ca68cfd1938ed01b3bb8f0b7" {
		t.Errorf("hash: %s", detail.InfoHash)
	}
	if detail.Category != "电视剧" {
		t.Errorf("category: %s", detail.Category)
	}
}

func TestNexusPHP_GetTorrentDetail_CategoryPriority(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `<html><title>Test</title>
		<tr><td class="rowhead">类别与标签</td><td class="rowfollow"><a href="torrents.php?cat=401"><img class="c_movie" src="pic/cattrans.gif" alt="电影 (Movie)" /></a></td></tr>
		<tr><td class="rowhead">基本信息</td><td class="rowfollow"><b><b>大小：</b></b>1.5 GB&nbsp;&nbsp;&nbsp;<b>类型:</b>&nbsp;电视剧</td></tr>
		</html>`)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewNexusPHPAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL}

	detail, err := a.GetTorrentDetail(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Category != "电影" {
		t.Errorf("category should come from img alt, got: %s", detail.Category)
	}
}

func TestNexusPHP_GetTorrentDetail_NoCategoryRow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `<html><title>Test</title>
		<tr><td class="rowhead">基本信息</td><td class="rowfollow"><b><b>大小：</b></b>500 MB&nbsp;&nbsp;&nbsp;<b>类型:</b>&nbsp;动漫</td></tr>
		</html>`)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewNexusPHPAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL}

	detail, err := a.GetTorrentDetail(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Category != "动漫" {
		t.Errorf("category should fallback to 类型, got: %s", detail.Category)
	}
}

func TestNexusPHP_GetPreciseSLData_RowBased(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `<html><title>Test</title>
		<tr><td class="rowhead"><span id="seeders"></span><span id="leechers"></span>同伴<br /></td>
		<td class="rowfollow"><div id="peercount"><b>42个做种者</b> | <b>7个下载者</b></div></td></tr>
		</html>`)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewNexusPHPAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL}

	sl, err := a.GetPreciseSLData(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if sl.Seeders != 42 {
		t.Errorf("seeders: %d", sl.Seeders)
	}
	if sl.Leechers != 7 {
		t.Errorf("leechers: %d", sl.Leechers)
	}
}

func TestNexusPHP_GetPreciseSLData_InlineInBasicInfo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `<html><title>Test</title>
		<tr><td class="rowhead">基本信息</td><td class="rowfollow" colspan="">
		<b><b>大小：</b></b>10 GB&nbsp;&nbsp;&nbsp;<b>类型:</b>&nbsp;电影
		<span style="float:right;display:inline-flex;align-items:center;gap:10px;">
		<span id="seeders" title="做种">140</span>
		<span id="leechers" title="下载">64</span>
		</span>
		</td></tr>
		</html>`)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewNexusPHPAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL}

	sl, err := a.GetPreciseSLData(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if sl.Seeders != 140 {
		t.Errorf("seeders: %d", sl.Seeders)
	}
	if sl.Leechers != 64 {
		t.Errorf("leechers: %d", sl.Leechers)
	}
}

func TestExtractBasicInfoFields(t *testing.T) {
	tests := []struct {
		name       string
		row        string
		wantSize   int64
		wantCat    string
		wantCodec  string
		wantRes    string
		wantSrc    string
		wantAudio  string
		wantTeam   string
		wantRegion string
	}{
		{
			name: "full fields",
			row: `<tr><td class="rowhead">基本信息</td><td class="rowfollow">` +
				`<b><b>大小：</b></b>40.38 GB&nbsp;&nbsp;&nbsp;` +
				`<b>类型:</b>&nbsp;剧集&nbsp;&nbsp;&nbsp;` +
				`<b>媒介:&nbsp;</b>WEB-DL&nbsp;&nbsp;&nbsp;` +
				`<b>编码:&nbsp;</b>H.265(HEVC)&nbsp;&nbsp;&nbsp;` +
				`<b>分辨率:&nbsp;</b>4K&nbsp;&nbsp;&nbsp;` +
				`<b>音频编码:&nbsp;</b>DD/AC3&nbsp;&nbsp;&nbsp;` +
				`<b>制作组:&nbsp;</b>ADWeb&nbsp;&nbsp;&nbsp;` +
				`<b>地区:&nbsp;</b>大陆</td></tr>`,
			wantSize:   43357694853,
			wantCat:    "剧集",
			wantCodec:  "H.265(HEVC)",
			wantRes:    "4K",
			wantSrc:    "WEB-DL",
			wantAudio:  "DD/AC3",
			wantTeam:   "ADWeb",
			wantRegion: "大陆",
		},
		{
			name: "english colon with MiB",
			row: `<tr><td class="rowhead">基本信息</td><td class="rowfollow">` +
				`<b>大小:</b>&nbsp;1.5 GiB&nbsp;&nbsp;&nbsp;` +
				`<b>类型:</b>&nbsp;Music</td></tr>`,
			wantSize: 1610612736,
			wantCat:  "Music",
		},
		{
			name: "quality label for source",
			row: `<tr><td class="rowhead">基本信息</td><td class="rowfollow">` +
				`<b><b>大小：</b></b>20.36 GB&nbsp;&nbsp;&nbsp;` +
				`<b>类型:</b>&nbsp;电影 (Movie)&nbsp;&nbsp;&nbsp;` +
				`<b>质量:&nbsp;</b>WEB-DL&nbsp;&nbsp;&nbsp;` +
				`<b>地区:&nbsp;</b>大陆</td></tr>`,
			wantSize:   21861383536,
			wantCat:    "电影 (Movie)",
			wantSrc:    "WEB-DL",
			wantRegion: "大陆",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detail := &model.TorrentDetail{}
			extractBasicInfoFields(tt.row, detail)

			if tt.wantSize > 0 && detail.Size != tt.wantSize {
				t.Errorf("size: got %d, want %d", detail.Size, tt.wantSize)
			}
			if tt.wantCat != "" && detail.Category != tt.wantCat {
				t.Errorf("category: got %q, want %q", detail.Category, tt.wantCat)
			}
			if tt.wantCodec != "" && detail.Codec != tt.wantCodec {
				t.Errorf("codec: got %q, want %q", detail.Codec, tt.wantCodec)
			}
			if tt.wantRes != "" && detail.Resolution != tt.wantRes {
				t.Errorf("resolution: got %q, want %q", detail.Resolution, tt.wantRes)
			}
			if tt.wantSrc != "" && detail.Source != tt.wantSrc {
				t.Errorf("source: got %q, want %q", detail.Source, tt.wantSrc)
			}
			if tt.wantAudio != "" && detail.AudioCodec != tt.wantAudio {
				t.Errorf("audio_codec: got %q, want %q", detail.AudioCodec, tt.wantAudio)
			}
			if tt.wantTeam != "" && detail.ReleaseGroup != tt.wantTeam {
				t.Errorf("release_group: got %q, want %q", detail.ReleaseGroup, tt.wantTeam)
			}
			if tt.wantRegion != "" && detail.Region != tt.wantRegion {
				t.Errorf("region: got %q, want %q", detail.Region, tt.wantRegion)
			}
		})
	}
}

func TestNormalizeCategory(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{"Movies", "电影"},
		{"Movies/电影", "电影"},
		{"Movies(电影)", "电影"},
		{"Movies（🎬 电影）", "电影"},
		{"电影", "电影"},
		{"高清电影", "电影"},
		{"TV Series", "电视剧"},
		{"TV Series/电视剧", "电视剧"},
		{"电视剧", "电视剧"},
		{"剧集", "电视剧"},
		{"短剧", "电视剧"},
		{"TV Shows(综艺)", "综艺"},
		{"综艺", "综艺"},
		{"Anime(动漫)", "动漫"},
		{"Animations/动漫、动画", "动漫"},
		{"动漫", "动漫"},
		{"动画", "动漫"},
		{"动漫(Animations)", "动漫"},
		{"Documentaries.纪录片", "纪录片"},
		{"纪录片", "纪录片"},
		{"Music/Music", "音乐"},
		{"音乐", "音乐"},
		{"音乐 / Music", "音乐"},
		{"Lossless Music", "音乐"},
		{"Book(书籍、杂志、报刊、有声书)", "书籍"},
		{"Program", "软件"},
		{"学习", "教育"},
		{"文档", "文档"},
		{"漫画", "漫画"},
		{"Nature", "纪录片"},
		{"成人漫畫", "成人漫畫"},
		{"高中部", "高中部"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			got := NormalizeCategory(tt.raw)
			if got != tt.want {
				t.Errorf("NormalizeCategory(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestNexusPHP_GetTorrentDetail_DDBased_ZeusStyle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `<html><title>Beethoven's Ninth Symphony</title>
		<dt>副标题</dt><dd>古典音乐-贝多芬：第九交响曲</dd>
		<dt>基本信息</dt><dd><dl class="minor-list properties">
		<dt>大小</dt><dd>1.95 GiB<dt>类型</dt><dd><a href="torrents.php?cat=406">音乐MV</a></dd>
		<dt>分辨率</dt><dd><a href="torrents.php?standard=3">720p</dd>
		</dl></dd>
		<dt>种子文件</dt><dd>种子Hash：aa11bb22cc33dd44ee55ff66aa77bb88cc99dd00</dd>
		</html>`)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewNexusPHPAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL, Cookie: "test=1"}

	detail, err := a.GetTorrentDetail(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Subtitle != "古典音乐-贝多芬：第九交响曲" {
		t.Errorf("subtitle: %s", detail.Subtitle)
	}
	if detail.Size == 0 {
		t.Error("expected non-zero size")
	}
	sizeGB := float64(detail.Size) / 1024 / 1024 / 1024
	if sizeGB < 1.9 || sizeGB > 2.1 {
		t.Errorf("expected ~1.95 GiB, got %.2f GB", sizeGB)
	}
	if detail.InfoHash != "aa11bb22cc33dd44ee55ff66aa77bb88cc99dd00" {
		t.Errorf("hash: %s", detail.InfoHash)
	}
}

func TestNexusPHP_GetTorrentDetail_DivBased_HhanclubStyle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `<html><title>哥谭 第二季</title>
		<div class="font-bold leading-6">副标题</div>
		<div class="font-bold leading-6">哥谭 第二季 全22集 | 类型:剧情</div>
		<div class="font-bold leading-6">基本信息</div>
		<div class="grid gap-y-5 grid-cols-4">
		<div><span class="font-bold"><b>大小：</b></span><span class="">60.05 GB</span></div>
		<div><span class="font-bold">类型:&nbsp;&nbsp;</span><span class="">电视剧</span></div>
		<div><span class="font-bold">编码:&nbsp;&nbsp;</span><span class="">AVC/H264/x264</span></div>
		<div><span class="font-bold">分辨率:&nbsp;&nbsp;</span><span class="">1080p</span></div>
		</div>
		<div class="font-bold leading-6">种子文件</div>
		<div class="grid">
		<span class="font-bold">Hash码：</span><span class="font-bold">0a40a917758564a54c046c2bb80311a38c3a9a0a</span>
		</div>
		</html>`)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewNexusPHPAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL, Cookie: "test=1"}

	detail, err := a.GetTorrentDetail(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Size == 0 {
		t.Error("expected non-zero size")
	}
	sizeGB := float64(detail.Size) / 1024 / 1024 / 1024
	if sizeGB < 59 || sizeGB > 61 {
		t.Errorf("expected ~60 GB, got %.2f GB", sizeGB)
	}
	if detail.Category != "电视剧" {
		t.Errorf("category: %s", detail.Category)
	}
	if detail.Codec != "AVC/H264/x264" {
		t.Errorf("codec: %s", detail.Codec)
	}
	if detail.Resolution != "1080p" {
		t.Errorf("resolution: %s", detail.Resolution)
	}
	if detail.InfoHash != "0a40a917758564a54c046c2bb80311a38c3a9a0a" {
		t.Errorf("hash: %s", detail.InfoHash)
	}
}

func TestParseSizeStr_MiB(t *testing.T) {
	tests := []struct {
		input  string
		expect int64
	}{
		{"246 MiB", 257949696},
		{"1.5 GiB", 1610612736},
		{"1 TiB", 1099511627776},
		{"500 KiB", 512000},
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

func TestNexusPHP_GetTorrentDetail_RealHTML(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real HTML fixture test in short mode")
	}

	entries, err := os.ReadDir("testdata")
	if err != nil {
		t.Skip("no testdata directory")
	}

	a := NewNexusPHPAdapter(NewHTTPDoer(), zap.NewNop())
	var sizeOK, catOK, hashOK, subOK int
	var total int

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".html") {
			continue
		}
		domain := strings.TrimSuffix(entry.Name(), ".html")

		htmlBytes, err := os.ReadFile("testdata/" + entry.Name())
		if err != nil || len(htmlBytes) < 5000 {
			continue
		}

		total++
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write(htmlBytes)
		}))

		detail, detailErr := a.GetTorrentDetail(context.Background(), &model.SiteConfig{Domain: srv.URL}, "1")
		srv.Close()

		if detailErr != nil {
			t.Logf("%-30s ERROR: %v", domain, detailErr)
			continue
		}

		if detail.Size > 0 {
			sizeOK++
		}
		if detail.Category != "" {
			catOK++
		}
		if detail.InfoHash != "" {
			hashOK++
		}
		if detail.Subtitle != "" {
			subOK++
		}

		titlePreview := strings.TrimSpace(detail.Title)
		if len(titlePreview) > 50 {
			titlePreview = titlePreview[:50]
		}

		t.Logf("%-30s Size=%-5v Cat=%-10s Hash=%-5v Sub=%-5v %s",
			domain,
			detail.Size > 0,
			detail.Category,
			detail.InfoHash != "",
			detail.Subtitle != "",
			titlePreview)
	}

	t.Logf("=== Summary: %d sites tested ===", total)
	t.Logf("Size: %d/%d (%.0f%%)", sizeOK, total, float64(sizeOK)/float64(total)*100)
	t.Logf("Category: %d/%d (%.0f%%)", catOK, total, float64(catOK)/float64(total)*100)
	t.Logf("InfoHash: %d/%d (%.0f%%)", hashOK, total, float64(hashOK)/float64(total)*100)
	t.Logf("Subtitle: %d/%d (%.0f%%)", subOK, total, float64(subOK)/float64(total)*100)

	if sizeOK < total*90/100 {
		t.Errorf("Size extraction rate too low: %d/%d (%.0f%%), expected >=90%%", sizeOK, total, float64(sizeOK)/float64(total)*100)
	}
	if catOK < total*85/100 {
		t.Errorf("Category extraction rate too low: %d/%d (%.0f%%), expected >=85%%", catOK, total, float64(catOK)/float64(total)*100)
	}
}

func TestNexusPHP_VerifyExists_Found(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html>
		<tr class="torrent-row"><td><a href="details.php?id=42"><b>&nbsp;Movie A</b></a></td><td>1.5 GB</td></tr>
		<tr class="torrent-row"><td><a href="details.php?id=99"><b>&nbsp;Movie B</b></a></td><td>700 MB</td></tr>
		</html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewNexusPHPAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL}

	found, err := a.VerifyExists(context.Background(), config, "42")
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Error("expected true")
	}
}

func TestNexusPHP_VerifyExists_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html>
		<tr class="torrent-row"><td><a href="details.php?id=10"><b>&nbsp;Movie A</b></a></td><td>1.5 GB</td></tr>
		<tr class="torrent-row"><td><a href="details.php?id=20"><b>&nbsp;Movie B</b></a></td><td>700 MB</td></tr>
		</html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewNexusPHPAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL}

	found, err := a.VerifyExists(context.Background(), config, "999")
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Error("expected false")
	}
}

func TestNexusPHP_VerifyExists_SearchError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewNexusPHPAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL}

	found, err := a.VerifyExists(context.Background(), config, "42")
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Error("expected false on search error")
	}
}

func TestNexusPHP_DetectDiscount_API_2xFree(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"discount": "2xfree"}`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewNexusPHPAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL}
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
	if result.Multiplier != 2.0 {
		t.Errorf("expected multiplier 2.0, got %f", result.Multiplier)
	}
}

func TestNexusPHP_DetectDiscount_API_2xUp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"discount": "2xup"}`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewNexusPHPAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL}
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

func TestNexusPHP_DetectDiscount_API_Percent50(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"discount": "50%"}`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewNexusPHPAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL}
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

func TestNexusPHP_DetectDiscount_API_Percent30(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"discount": "30%"}`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewNexusPHPAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL}
	config.Discount.HasAPI = true
	config.Discount.APIURL = "/api/discount?id={id}"
	config.Discount.Selectors = []string{"30%"}

	result, err := a.DetectDiscount(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Level != model.DiscountPercent30 {
		t.Errorf("expected PERCENT_30, got %s", result.Level)
	}
}

func TestNexusPHP_DetectDiscount_API_None(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"discount": "normal"}`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewNexusPHPAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL}
	config.Discount.HasAPI = true
	config.Discount.APIURL = "/api/discount?id={id}"
	config.Discount.Selectors = []string{"normal"}

	result, err := a.DetectDiscount(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Level != model.DiscountNone {
		t.Errorf("expected NONE, got %s", result.Level)
	}
}

func TestNexusPHP_DetectDiscount_API_NetworkError(t *testing.T) {
	a := NewNexusPHPAdapter(NewHTTPDoer(), zap.NewNop())
	config := &model.SiteConfig{Domain: "http://127.0.0.1:1"}
	config.Discount.HasAPI = true
	config.Discount.APIURL = "/api/discount?id={id}"
	config.Discount.Selectors = []string{"free"}

	_, err := a.DetectDiscount(context.Background(), config, "1")
	if err == nil {
		t.Fatal("expected error on network error")
	}
}

func TestNexusPHP_DetectHR_NonHTMLResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"error": "not found"}`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewNexusPHPAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL}
	config.HR.Selectors = []string{"hr"}

	result, err := a.DetectHR(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if result.HasHR {
		t.Error("expected HasHR=false for non-HTML JSON response")
	}
}

func TestNexusPHP_DetectHR_HitAndRunClass(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><td class="hitandrun">HR</td></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewNexusPHPAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL}
	config.HR.Selectors = []string{"hr"}
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

func TestNexusPHP_UploadTorrent_ErrorPClass(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><p class="error">上传失败：文件大小超出限制</p></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewNexusPHPAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}

	req := &model.PublishRequest{
		TorrentData: []byte("d4:infod...e"),
		Title:       "Test",
	}
	_, err := a.UploadTorrent(context.Background(), config, req)
	if err == nil {
		t.Fatal("expected error for error p tag")
	}
	var appErr *model.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %T", err)
	}
	if !strings.Contains(appErr.Message, "上传失败") {
		t.Errorf("error message should contain 上传失败, got: %s", appErr.Message)
	}
}

func TestNexusPHP_UploadTorrent_DuplicateDetected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><p>上传失败：该种子已经存在 (duplicate torrent)</p></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewNexusPHPAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}

	req := &model.PublishRequest{
		TorrentData: []byte("d4:infod...e"),
		Title:       "Test",
	}
	_, err := a.UploadTorrent(context.Background(), config, req)
	if err == nil {
		t.Fatal("expected error for duplicate")
	}
	var appErr *model.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %T", err)
	}
	if !strings.Contains(appErr.Message, "duplicate") {
		t.Errorf("error message should mention duplicate, got: %s", appErr.Message)
	}
}

func TestNexusPHP_UploadTorrent_UnknownResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body>Something unexpected happened</body></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewNexusPHPAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}

	req := &model.PublishRequest{
		TorrentData: []byte("d4:infod...e"),
		Title:       "Test",
	}
	_, err := a.UploadTorrent(context.Background(), config, req)
	if err == nil {
		t.Fatal("expected error for unknown response")
	}
	var appErr *model.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != 15001 {
		t.Errorf("expected code 15001, got %d", appErr.Code)
	}
}

func TestNexusPHP_UploadTorrent_SuccessKeyword(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body>Upload succeeded</body></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewNexusPHPAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}

	req := &model.PublishRequest{
		TorrentData: []byte("d4:infod...e"),
		Title:       "Test",
	}
	result, err := a.UploadTorrent(context.Background(), config, req)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Error("expected success")
	}
}

func TestParseNexusPHPBrowse_EmptyResults(t *testing.T) {
	html := `<html><body><p>No torrents found</p></body></html>`
	config := &model.SiteConfig{Domain: "example.com"}
	results := parseNexusPHPBrowse(html, config)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestParseNexusPHPBrowse_AltDetailLinkRe(t *testing.T) {
	html := `<html>
	<tr><td><a href="details.php?id=55&amp;hit=1">Movie Without Bold</a></td>
	<td class="rowfollow">1.5<br />GB</td>
	<td><a href="dllist=1#seeders">15</a></td>
	<td><a href="dllist=1#leechers">3</a></td>
	</tr>
	</html>`
	config := &model.SiteConfig{Domain: "example.com"}
	results := parseNexusPHPBrowse(html, config)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].TorrentID != "55" {
		t.Errorf("expected ID 55, got %s", results[0].TorrentID)
	}
	if results[0].Title != "Movie Without Bold" {
		t.Errorf("expected title 'Movie Without Bold', got %s", results[0].Title)
	}
	if results[0].Seeders != 15 {
		t.Errorf("expected 15 seeders, got %d", results[0].Seeders)
	}
	if results[0].Leechers != 3 {
		t.Errorf("expected 3 leechers, got %d", results[0].Leechers)
	}
}

func TestParseNexusPHPBrowse_WithSeedersLeechers(t *testing.T) {
	html := `<html>
	<tr><td><a href="details.php?id=100"><b>&nbsp;Seeded Torrent</b></a></td>
	<td class="rowfollow">4.50<br />GB</td>
	<td><a href="dllist=1#seeders">200</a></td>
	<td><a href="dllist=1#leechers">50</a></td>
	</tr>
	</html>`
	config := &model.SiteConfig{Domain: "example.com"}
	results := parseNexusPHPBrowse(html, config)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.TorrentID != "100" {
		t.Errorf("expected ID 100, got %s", r.TorrentID)
	}
	if r.Seeders != 200 {
		t.Errorf("expected 200 seeders, got %d", r.Seeders)
	}
	if r.Leechers != 50 {
		t.Errorf("expected 50 leechers, got %d", r.Leechers)
	}
	sizeGB := float64(r.Size) / 1024 / 1024 / 1024
	if sizeGB < 4.4 || sizeGB > 4.6 {
		t.Errorf("expected ~4.5 GB, got %.2f GB", sizeGB)
	}
}

func TestErrorConstructors(t *testing.T) {
	cause := errors.New("root cause")
	cases := []struct {
		name       string
		err        *model.AppError
		wantCode   int
		retryable  bool
		wantMsg    string
		wantErrMsg string
	}{
		{"networkError", networkError("net fail", cause), ErrAdapterNetwork, true, "net fail", "net fail: root cause"},
		{"httpError", httpError("http fail", cause), ErrAdapterHTTP, true, "http fail", "http fail: root cause"},
		{"parseError", parseError("parse fail", cause), ErrAdapterParse, false, "parse fail", "parse fail: root cause"},
		{"uploadError", uploadError("upload fail", cause), ErrAdapterUpload, false, "upload fail", "upload fail: root cause"},
		{"downloadError", downloadError("dl fail", cause), ErrAdapterDownload, false, "dl fail", "dl fail: root cause"},
		{"notFoundError", notFoundError("not found"), ErrAdapterNotFound, false, "not found", "not found"},
		{"configError", configError("bad config"), ErrAdapterConfig, false, "bad config", "bad config"},
		{"authError", authError("auth fail", cause), ErrAdapterAuth, false, "auth fail", "auth fail: root cause"},
		{"searchError", searchError("search fail", cause), ErrAdapterSearch, false, "search fail", "search fail: root cause"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.err.Code != tc.wantCode {
				t.Errorf("code: got %d, want %d", tc.err.Code, tc.wantCode)
			}
			if tc.err.Message != tc.wantMsg {
				t.Errorf("message: got %q, want %q", tc.err.Message, tc.wantMsg)
			}
			if tc.err.Retryable != tc.retryable {
				t.Errorf("retryable: got %v, want %v", tc.err.Retryable, tc.retryable)
			}
			if tc.err.Error() != tc.wantErrMsg {
				t.Errorf("Error(): got %q, want %q", tc.err.Error(), tc.wantErrMsg)
			}
		})
	}
}

func TestErrorConstructors_CauseUnwrap(t *testing.T) {
	cause := errors.New("root")
	err := networkError("wrap", cause)
	if !errors.Is(err, cause) {
		t.Error("expected errors.Is to match cause")
	}
}

func TestErrorConstructors_NilCause(t *testing.T) {
	err := notFoundError("gone")
	if err.Unwrap() != nil {
		t.Error("expected nil cause")
	}
}

func TestFmtES(t *testing.T) {
	got := fmtES("hello %s %d", "world", 42)
	if got != "hello world 42" {
		t.Errorf("got %q, want %q", got, "hello world 42")
	}
}
