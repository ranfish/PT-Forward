package adapter

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

func TestGazelle_Framework(t *testing.T) {
	a := NewGazelleAdapter(NewHTTPDoer(), zap.NewNop())
	if a.Framework() != "gazelle" {
		t.Errorf("expected gazelle, got %s", a.Framework())
	}
}

func TestGazelle_DownloadTorrent_OK(t *testing.T) {
	payload := []byte("d4:infod...e")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("id") != "42" {
			t.Error("missing id")
		}
		if r.URL.Query().Get("passkey") != "pk1" {
			t.Error("missing passkey")
		}
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGazelleAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL, Passkey: "pk1"}

	data, err := a.DownloadTorrent(context.Background(), config, "42")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(payload) {
		t.Error("data mismatch")
	}
}

func TestGazelle_DownloadTorrent_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGazelleAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL, Passkey: "pk1"}

	_, err := a.DownloadTorrent(context.Background(), config, "42")
	if err == nil {
		t.Fatal("expected error for 404")
	}
	var appErr *model.AppError
	if !errors.As(err, &appErr) || appErr.Code != ErrAdapterNotFound {
		t.Errorf("expected ErrAdapterNotFound, got %v", err)
	}
}

func TestGazelle_DownloadTorrent_AuthKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("authkey") != "ak1" {
			t.Error("missing authkey")
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGazelleAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL, AuthKey: "ak1"}

	_, err := a.DownloadTorrent(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
}

func TestGazelle_DownloadTorrent_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGazelleAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL}

	_, err := a.DownloadTorrent(context.Background(), config, "1")
	if err == nil {
		t.Error("expected error")
	}
}

func TestGazelle_GetTorrentDetail_API(t *testing.T) {
	// §59.462: sections/ajax 实测结构（status+平铺 categoryName+版本四槽）
	apiResp := map[string]interface{}{
		"status": "success",
		"response": map[string]interface{}{
			"torrent": map[string]interface{}{
				"size":       1073741824,
				"infoHash":   "AABBCCDDEEFF00112233445566778899aabbccdd",
				"seeders":    10,
				"leechers":   5,
				"codec":      "x264",
				"resolution": "1080p",
				"container":  "MKV",
				"processing": "Encode",
				"source":     "Blu-ray",
				"fileName":   "movie.mkv",
			},
			"group": map[string]interface{}{
				"name":        "Movie 2024",
				"subName":     "测试影片",
				"year":        2024,
				"categoryName": "Movies",
			},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if err := json.NewEncoder(w).Encode(apiResp); err != nil {
			t.Errorf("encode: %v", err)
		}
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGazelleAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL, APIKey: "test-key"}

	detail, err := a.GetTorrentDetail(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Title != "Movie 2024" {
		t.Errorf("title: %s", detail.Title)
	}
	if detail.Size != 1073741824 {
		t.Errorf("size: %d", detail.Size)
	}
	// §59.462: 实测结构无独立 Category 槽（纯电影站 Movies 恒定）——断言版本槽
	if detail.Codec != "x264" || detail.Resolution != "1080p" || detail.ReleaseGroup != "Encode" {
		t.Errorf("版本槽: codec=%q res=%q processing=%q", detail.Codec, detail.Resolution, detail.ReleaseGroup)
	}
}

func TestGazelle_GetTorrentDetail_Web(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><title>Web Movie :: Site</title>
		info_hash: aabbccddeeff00112233445566778899aabbccdd
		<td>Size<td>1.5 GB</td></td>
		</html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGazelleAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}

	detail, err := a.GetTorrentDetail(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Title != "Web Movie" {
		t.Errorf("title: %s", detail.Title)
	}
}

func TestGazelle_DetectDiscount(t *testing.T) {
	a := NewGazelleAdapter(NewHTTPDoer(), zap.NewNop())
	_, err := a.DetectDiscount(context.Background(), &model.SiteConfig{}, "1")
	if err == nil {
		t.Fatal("expected error when no BaseURL configured")
	}
}

func TestGazelle_DetectHR(t *testing.T) {
	a := NewGazelleAdapter(NewHTTPDoer(), zap.NewNop())
	_, err := a.DetectHR(context.Background(), &model.SiteConfig{}, "1")
	if err == nil {
		t.Fatal("expected error when no BaseURL configured")
	}
}

func TestGazelle_GetPreciseSLData_API(t *testing.T) {
	apiResp := map[string]interface{}{
		"response": map[string]interface{}{
			"torrent": map[string]interface{}{
				"seeders":  20,
				"leechers": 8,
			},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if err := json.NewEncoder(w).Encode(apiResp); err != nil {
			t.Errorf("encode: %v", err)
		}
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGazelleAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL, APIKey: "key"}

	sl, err := a.GetPreciseSLData(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if sl.Seeders != 20 {
		t.Errorf("seeders: %d", sl.Seeders)
	}
	if sl.Leechers != 8 {
		t.Errorf("leechers: %d", sl.Leechers)
	}
}

func TestGazelle_GetPreciseSLData_Web(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html>Seeders<td>15</td>Leechers<td>3</td></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGazelleAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL}

	sl, err := a.GetPreciseSLData(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if sl.Seeders != 15 {
		t.Errorf("seeders: %d", sl.Seeders)
	}
}

func TestGazelle_UploadTorrent_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		_, _ = w.Write([]byte(`<html>success <a href="torrents.php?torrentid=555">view</a></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGazelleAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}

	result, err := a.UploadTorrent(context.Background(), config, &model.PublishRequest{
		TorrentData: []byte("d4:infod...e"),
		Title:       "Test Movie",
		Description: "A test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Error("expected success")
	}
	if result.TorrentID != "555" {
		t.Errorf("expected 555, got %s", result.TorrentID)
	}
}

func TestGazelle_UploadTorrent_EmptyData(t *testing.T) {
	a := NewGazelleAdapter(NewHTTPDoer(), zap.NewNop())
	_, err := a.UploadTorrent(context.Background(), &model.SiteConfig{}, &model.PublishRequest{})
	if err == nil {
		t.Error("expected error")
	}
}

func TestGazelle_UploadTorrent_Fail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><span class="error">Duplicate torrent</span></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGazelleAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL}

	result, err := a.UploadTorrent(context.Background(), config, &model.PublishRequest{TorrentData: []byte("data")})
	if err == nil {
		t.Fatal("expected error for upload failure")
	}
	var appErr *model.AppError
	if !errors.As(err, &appErr) || appErr.Code != 15001 {
		t.Fatalf("expected AppError 15001, got %v", err)
	}
	if result != nil {
		t.Error("expected nil result")
	}
}

func TestGazelle_GetTorrentInfoHash_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]interface{}{
			"status": "success",
			"response": map[string]interface{}{
				"torrent": map[string]interface{}{"infoHash": "aaBBccDDeeFF00112233445566778899aaBBccDD", "size": 100, "fileName": "f"},
				"group":   map[string]interface{}{"name": "T", "year": 2024},
			},
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("encode: %v", err)
		}
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGazelleAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL, APIKey: "key"}

	hash, err := a.GetTorrentInfoHash(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if hash != "aabbccddeeff00112233445566778899aabbccdd" {
		t.Errorf("hash: %s", hash)
	}
}

func TestResolveBase(t *testing.T) {
	tests := []struct {
		domain string
		expect string
	}{
		{"example.com", "https://example.com"},
		{"https://example.com/", "https://example.com"},
		{"http://test.com", "http://test.com"},
	}

	for _, tt := range tests {
		got := resolveBaseURL(&model.SiteConfig{Domain: tt.domain})
		if got != tt.expect {
			t.Errorf("resolveBaseURL(%q) = %q, want %q", tt.domain, got, tt.expect)
		}
	}
}

func TestGazelle_GetTorrentDetail_APIFull(t *testing.T) {
	apiResp := map[string]interface{}{
		"status": "success",
		"response": map[string]interface{}{
			"torrent": map[string]interface{}{
				"size":       5368709120,
				"infoHash":   "11223344556677889900AABBCCDDEEFF00112233",
				"seeders":    50,
				"leechers":   15,
				"codec":      "H.265",
				"resolution": "2160p",
				"processing": "BD66",
				"mediainfos": []string{"General - Complete name: movie.mkv"},
				"fileName":   "movie.2024.bluray.mkv",
			},
			"group": map[string]interface{}{
				"name":         "Full API Movie",
				"subName":      "全字段影片",
				"year":         2024,
				"categoryName": "Movies",
				"imdbId":       "tt1234567",
				"doubanId":     "3592120",
				"conver":       "https://ptpimg.me/cover.jpg",
				"tags":         []string{"action", "drama"},
			},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if err := json.NewEncoder(w).Encode(apiResp); err != nil {
			t.Errorf("encode: %v", err)
		}
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGazelleAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL, APIKey: "test-key"}

	detail, err := a.GetTorrentDetail(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Title != "Full API Movie" {
		t.Errorf("title: %s", detail.Title)
	}
	if detail.Size != 5368709120 {
		t.Errorf("size: %d", detail.Size)
	}
	if detail.InfoHash != "11223344556677889900aabbccddeeff00112233" {
		t.Errorf("info_hash: %s", detail.InfoHash)
	}
	// §59.462 实测结构断言（双层全字段）
	if detail.Subtitle == "" || detail.IMDbID != "tt1234567" || detail.DoubanID != "3592120" {
		t.Errorf("双层字段: sub=%q imdb=%q douban=%q", detail.Subtitle, detail.IMDbID, detail.DoubanID)
	}
	if detail.Codec != "H.265" || detail.Resolution != "2160p" || detail.ReleaseGroup != "BD66" {
		t.Errorf("版本槽: codec=%q res=%q processing=%q", detail.Codec, detail.Resolution, detail.ReleaseGroup)
	}
	if detail.MediaInfo == "" || detail.PosterURL != "https://ptpimg.me/cover.jpg" {
		t.Errorf("mi/poster: %q %q", detail.MediaInfo[:min(20, len(detail.MediaInfo))], detail.PosterURL)
	}
}
func min(a, b int) int { if a < b { return a }; return b }

func TestGazelle_VerifyExists_Found(t *testing.T) {
	// §59.462: SearchTorrents 改 browse AJAX——mock JSON 响应
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"success","response":{"results":[{"groupId":1,"groupName":"Found Torrent","groupYear":2024,"torrents":[{"torrentId":42,"size":1073741824,"seeders":3,"leechers":0}]}]}}`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGazelleAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}

	found, err := a.VerifyExists(context.Background(), config, "42")
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Error("expected found=true")
	}
}

func TestGazelle_VerifyExists_NotFound(t *testing.T) {
	// §59.462: 组命中但 tid 不匹配（展平后无 42）
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"success","response":{"results":[{"groupId":1,"groupName":"Other","groupYear":2020,"torrents":[{"torrentId":99,"size":100,"seeders":1,"leechers":0}]}]}}`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGazelleAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}

	found, err := a.VerifyExists(context.Background(), config, "42")
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Error("expected found=false")
	}
}

func TestGazelle_DetectDiscount_Free(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body><span class="freeleech">Free</span></body></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGazelleAdapter(doer, zap.NewNop())

	result, err := a.DetectDiscount(context.Background(), &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}, "1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Level != model.DiscountFree {
		t.Errorf("expected DiscountFree, got %v", result.Level)
	}
}

func TestGazelle_DetectDiscount_2xFree(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body>freeleech double upload zone</body></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGazelleAdapter(doer, zap.NewNop())

	result, err := a.DetectDiscount(context.Background(), &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}, "1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Level != model.Discount2xFree {
		t.Errorf("expected Discount2xFree, got %v", result.Level)
	}
}

func TestGazelle_DetectDiscount_2xUp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body>2x upload bonus</body></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGazelleAdapter(doer, zap.NewNop())

	result, err := a.DetectDiscount(context.Background(), &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}, "1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Level != model.Discount2xUp {
		t.Errorf("expected Discount2xUp, got %v", result.Level)
	}
}

func TestGazelle_DetectDiscount_50Percent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body>half download deal</body></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGazelleAdapter(doer, zap.NewNop())

	result, err := a.DetectDiscount(context.Background(), &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}, "1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Level != model.DiscountPercent50 {
		t.Errorf("expected DiscountPercent50, got %v", result.Level)
	}
}

func TestGazelle_DetectDiscount_None(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body>normal torrent page</body></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGazelleAdapter(doer, zap.NewNop())

	result, err := a.DetectDiscount(context.Background(), &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}, "1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Level != model.DiscountNone {
		t.Errorf("expected DiscountNone, got %v", result.Level)
	}
}

func TestGazelle_DetectHR_Found(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body><span class="hitandrun">H&R</span></body></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGazelleAdapter(doer, zap.NewNop())

	result, err := a.DetectHR(context.Background(), &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}, "1")
	if err != nil {
		t.Fatal(err)
	}
	if !result.HasHR {
		t.Error("expected HasHR=true")
	}
}

func TestGazelle_DetectHR_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body>regular torrent</body></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGazelleAdapter(doer, zap.NewNop())

	result, err := a.DetectHR(context.Background(), &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}, "1")
	if err != nil {
		t.Fatal(err)
	}
	if result.HasHR {
		t.Error("expected HasHR=false")
	}
}
