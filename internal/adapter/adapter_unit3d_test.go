package adapter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

func TestUnit3D_Framework(t *testing.T) {
	a := NewUnit3DAdapter(NewHTTPDoer(), zap.NewNop())
	if a.Framework() != "unit3d" {
		t.Errorf("expected unit3d, got %s", a.Framework())
	}
}

func TestUnit3D_DownloadTorrent_OK(t *testing.T) {
	payload := []byte("d4:infod...e")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.SetCookie(w, &http.Cookie{Name: "test_session", Value: "s1"})
			return
		}
		if r.Header.Get("Authorization") != "Bearer ak1" {
			t.Error("missing Bearer token")
		}
		w.Write(payload)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewUnit3DAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL, APIKey: "ak1"}

	data, err := a.DownloadTorrent(context.Background(), config, "42")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(payload) {
		t.Error("data mismatch")
	}
}

func TestUnit3D_DownloadTorrent_Forbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewUnit3DAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL}

	_, err := a.DownloadTorrent(context.Background(), config, "1")
	if err == nil {
		t.Error("expected error for 403")
	}
}

func TestUnit3D_DownloadTorrent_CustomPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.SetCookie(w, &http.Cookie{Name: "test_session", Value: "s1"})
			return
		}
		if r.URL.Path != "/torrents/download/42" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewUnit3DAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL}

	_, err := a.DownloadTorrent(context.Background(), config, "42")
	if err != nil {
		t.Fatal(err)
	}
}

func TestUnit3D_GetTorrentDetail_API(t *testing.T) {
	apiResp := map[string]interface{}{
		"data": map[string]interface{}{
			"name":       "Unit3D Movie",
			"size":       2147483648,
			"info_hash":  "ccddEEFF00112233445566778899AABBccddeeff",
			"category":   "Movie",
			"seeders":    30,
			"leechers":   10,
			"type":       "BluRay",
			"resolution": "1080p",
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(apiResp)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewUnit3DAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL, APIKey: "key"}

	detail, err := a.GetTorrentDetail(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Title != "Unit3D Movie" {
		t.Errorf("title: %s", detail.Title)
	}
	if detail.Size != 2147483648 {
		t.Errorf("size: %d", detail.Size)
	}
	if detail.Source != "BluRay" {
		t.Errorf("source: %s", detail.Source)
	}
	if detail.Resolution != "1080p" {
		t.Errorf("resolution: %s", detail.Resolution)
	}
}

func TestUnit3D_GetTorrentDetail_Web(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`<html><title>Web Title :: Site</title>
		info_hash: aabbccddeeff00112233445566778899aabbccdd
		<td>Size<td>2 GB</td></td>
		</html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewUnit3DAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL}

	detail, err := a.GetTorrentDetail(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Title != "Web Title" {
		t.Errorf("title: %s", detail.Title)
	}
}

func TestUnit3D_DetectDiscount_Free(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`<html>This torrent is freeleech!</html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewUnit3DAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL}

	result, err := a.DetectDiscount(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Level != model.DiscountFree {
		t.Errorf("expected FREE, got %s", result.Level)
	}
}

func TestUnit3D_DetectDiscount_2xFree(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`<html>freeleech and double upload</html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewUnit3DAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL}

	result, err := a.DetectDiscount(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Level != model.Discount2xFree {
		t.Errorf("expected 2XFREE, got %s", result.Level)
	}
}

func TestUnit3D_DetectDiscount_2xUp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`<html>double upload bonus</html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewUnit3DAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL}

	result, err := a.DetectDiscount(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Level != model.Discount2xUp {
		t.Errorf("expected 2XUP, got %s", result.Level)
	}
}

func TestUnit3D_DetectDiscount_None(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`<html>Normal torrent</html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewUnit3DAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL}

	result, err := a.DetectDiscount(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Level != model.DiscountNone {
		t.Errorf("expected NONE, got %s", result.Level)
	}
}

func TestUnit3D_DetectHR(t *testing.T) {
	a := NewUnit3DAdapter(NewHTTPDoer(), zap.NewNop())
	result, err := a.DetectHR(context.Background(), &model.SiteConfig{}, "1")
	if err != nil {
		t.Fatal(err)
	}
	if result.HasHR {
		t.Error("expected no HR")
	}
}

func TestUnit3D_GetPreciseSLData_API(t *testing.T) {
	apiResp := map[string]interface{}{
		"data": map[string]interface{}{
			"seeders":  25,
			"leechers": 7,
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(apiResp)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewUnit3DAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL, APIKey: "key"}

	sl, err := a.GetPreciseSLData(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if sl.Seeders != 25 {
		t.Errorf("seeders: %d", sl.Seeders)
	}
}

func TestUnit3D_GetPreciseSLData_Web(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`<html>Seeders<td>12</td>Leechers<td>4</td></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewUnit3DAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL}

	sl, err := a.GetPreciseSLData(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if sl.Seeders != 12 {
		t.Errorf("seeders: %d", sl.Seeders)
	}
}

func TestUnit3D_UploadTorrent_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.Write([]byte(`<html><meta name="csrf-token" content="test-csrf-token"></html>`))
			return
		}
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.Write([]byte(`<html>success redirect to /torrents/999</html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewUnit3DAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}

	result, err := a.UploadTorrent(context.Background(), config, &model.PublishRequest{
		TorrentData: []byte("d4:infod...e"),
		Title:       "Test",
		Description: "Desc",
		IMDbLink:    "https://imdb.com/tt123",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Error("expected success")
	}
	if result.TorrentID != "999" {
		t.Errorf("expected 999, got %s", result.TorrentID)
	}
}

func TestUnit3D_UploadTorrent_EmptyData(t *testing.T) {
	a := NewUnit3DAdapter(NewHTTPDoer(), zap.NewNop())
	_, err := a.UploadTorrent(context.Background(), &model.SiteConfig{}, &model.PublishRequest{})
	if err == nil {
		t.Error("expected error")
	}
}

func TestUnit3D_UploadTorrent_Fail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`<html><span class="error">Upload rejected</span></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewUnit3DAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL}

	result, err := a.UploadTorrent(context.Background(), config, &model.PublishRequest{TorrentData: []byte("data")})
	if err != nil {
		t.Fatal(err)
	}
	if result.Success {
		t.Error("expected failure")
	}
}

func TestUnit3D_GetTorrentInfoHash_API(t *testing.T) {
	apiResp := map[string]interface{}{
		"data": map[string]interface{}{
			"name": "T", "size": 100, "info_hash": "aaBBccDDeeFF00112233445566778899aaBBccDD",
			"category": "M", "seeders": 1, "leechers": 0, "type": "B", "resolution": "1080p",
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(apiResp)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewUnit3DAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL, APIKey: "key"}

	hash, err := a.GetTorrentInfoHash(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if hash != "aabbccddeeff00112233445566778899aabbccdd" {
		t.Errorf("hash: %s", hash)
	}
}
