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
	apiResp := map[string]interface{}{
		"response": map[string]interface{}{
			"torrent": map[string]interface{}{
				"filePath": "movie.mkv",
				"size":     1073741824,
				"infoHash": "AABBCCDDEEFF00112233445566778899aabbccdd",
				"seeders":  10,
				"leechers": 5,
			},
			"group": map[string]interface{}{
				"name": "Movie 2024",
				"category": map[string]interface{}{
					"name": "Movies",
				},
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
	if detail.Category != "Movies" {
		t.Errorf("category: %s", detail.Category)
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
	result, err := a.DetectDiscount(context.Background(), &model.SiteConfig{}, "1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Level != model.DiscountNone {
		t.Errorf("expected NONE, got %s", result.Level)
	}
}

func TestGazelle_DetectHR(t *testing.T) {
	a := NewGazelleAdapter(NewHTTPDoer(), zap.NewNop())
	result, err := a.DetectHR(context.Background(), &model.SiteConfig{}, "1")
	if err != nil {
		t.Fatal(err)
	}
	if result.HasHR {
		t.Error("expected no HR")
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
			"response": map[string]interface{}{
				"torrent": map[string]interface{}{"infoHash": "aaBBccDDeeFF00112233445566778899aaBBccDD", "size": 100, "filePath": "f"},
				"group":   map[string]interface{}{"name": "T", "category": map[string]interface{}{"name": "M"}},
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
		got := resolveBase(&model.SiteConfig{Domain: tt.domain})
		if got != tt.expect {
			t.Errorf("resolveBase(%q) = %q, want %q", tt.domain, got, tt.expect)
		}
	}
}
