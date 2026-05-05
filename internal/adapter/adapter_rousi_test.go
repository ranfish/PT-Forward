package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

func TestRousiAdapter_Framework(t *testing.T) {
	a := NewRousiAdapter(NewHTTPDoer(), zap.NewNop())
	if a.Framework() != "rousi" {
		t.Errorf("expected rousi, got %s", a.Framework())
	}
}

func TestRousiAdapter_DownloadTorrent_OK(t *testing.T) {
	payload := []byte("d4:infod6:lengthi0eee")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expected := "/api/torrent/42/download/testkey"
		if r.URL.Path != expected {
			t.Errorf("unexpected path: %s, want %s", r.URL.Path, expected)
		}
		w.Header().Set("Content-Type", "application/x-bittorrent")
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewRousiAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Passkey: "testkey"}
	data, err := a.DownloadTorrent(context.Background(), config, "42")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(payload) {
		t.Errorf("unexpected data")
	}
}

func TestRousiAdapter_DownloadTorrent_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewRousiAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Passkey: "testkey"}
	_, err := a.DownloadTorrent(context.Background(), config, "1")
	if err == nil {
		t.Fatal("expected error for 500")
	}
}

func TestRousiAdapter_GetTorrentDetail_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer testkey" {
			t.Errorf("missing Bearer auth header")
		}
		resp := map[string]interface{}{
			"code": 0,
			"data": map[string]interface{}{
				"uuid":      "abc-123",
				"title":     "Test Torrent",
				"subtitle":  "Sub",
				"size":      1024,
				"info_hash": "AABBCCDD11223344AABBCCDD11223344AABBCCDD",
				"category":  "movie",
				"seeders":   5,
				"leechers":  2,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(resp)
		_, _ = w.Write(b)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewRousiAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Passkey: "testkey"}
	detail, err := a.GetTorrentDetail(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Title != "Test Torrent" {
		t.Errorf("unexpected title: %s", detail.Title)
	}
	if detail.InfoHash != "aabbccdd11223344aabbccdd11223344aabbccdd" {
		t.Errorf("unexpected info_hash: %s", detail.InfoHash)
	}
}

func TestRousiAdapter_GetTorrentDetail_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewRousiAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Passkey: "testkey"}
	detail, err := a.GetTorrentDetail(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if detail != nil {
		t.Errorf("expected nil detail for 404")
	}
}

func TestRousiAdapter_GetTorrentDetail_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewRousiAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Passkey: "testkey"}
	_, err := a.GetTorrentDetail(context.Background(), config, "1")
	if err == nil {
		t.Fatal("expected error for 500")
	}
}

func TestRousiAdapter_DetectDiscount_Free(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]interface{}{
			"data": map[string]interface{}{
				"promotion": map[string]interface{}{
					"is_active":       true,
					"type":            1,
					"down_multiplier": 0,
					"up_multiplier":   1,
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(resp)
		_, _ = w.Write(b)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewRousiAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Passkey: "testkey"}
	result, err := a.DetectDiscount(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Level != model.DiscountFree {
		t.Errorf("expected FREE, got %s", result.Level)
	}
}

func TestRousiAdapter_DetectDiscount_2xUp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]interface{}{
			"data": map[string]interface{}{
				"promotion": map[string]interface{}{
					"is_active":       true,
					"type":            2,
					"down_multiplier": 1,
					"up_multiplier":   2,
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(resp)
		_, _ = w.Write(b)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewRousiAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Passkey: "testkey"}
	result, err := a.DetectDiscount(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Level != model.Discount2xUp {
		t.Errorf("expected 2XUP, got %s", result.Level)
	}
	if result.Multiplier != 2.0 {
		t.Errorf("expected multiplier 2.0, got %f", result.Multiplier)
	}
}

func TestRousiAdapter_DetectDiscount_Free2x(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]interface{}{
			"data": map[string]interface{}{
				"promotion": map[string]interface{}{
					"is_active":       true,
					"type":            3,
					"down_multiplier": 0,
					"up_multiplier":   2,
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(resp)
		_, _ = w.Write(b)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewRousiAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Passkey: "testkey"}
	result, err := a.DetectDiscount(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Level != model.DiscountFree {
		t.Errorf("expected FREE, got %s", result.Level)
	}
	if result.Multiplier != 2.0 {
		t.Errorf("expected multiplier 2.0, got %f", result.Multiplier)
	}
}

func TestRousiAdapter_DetectDiscount_50Percent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]interface{}{
			"data": map[string]interface{}{
				"promotion": map[string]interface{}{
					"is_active":       true,
					"type":            4,
					"down_multiplier": 0.5,
					"up_multiplier":   1,
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(resp)
		_, _ = w.Write(b)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewRousiAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Passkey: "testkey"}
	result, err := a.DetectDiscount(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Level != model.DiscountPercent50 {
		t.Errorf("expected PERCENT_50, got %s", result.Level)
	}
}

func TestRousiAdapter_DetectDiscount_2xFree(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]interface{}{
			"data": map[string]interface{}{
				"promotion": map[string]interface{}{
					"is_active":       true,
					"type":            5,
					"down_multiplier": 0,
					"up_multiplier":   2,
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(resp)
		_, _ = w.Write(b)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewRousiAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Passkey: "testkey"}
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

func TestRousiAdapter_DetectDiscount_2x50(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]interface{}{
			"data": map[string]interface{}{
				"promotion": map[string]interface{}{
					"is_active":       true,
					"type":            6,
					"down_multiplier": 0.5,
					"up_multiplier":   2,
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(resp)
		_, _ = w.Write(b)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewRousiAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Passkey: "testkey"}
	result, err := a.DetectDiscount(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Level != model.Discount2x50 {
		t.Errorf("expected 2X50, got %s", result.Level)
	}
}

func TestRousiAdapter_DetectDiscount_30Percent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]interface{}{
			"data": map[string]interface{}{
				"promotion": map[string]interface{}{
					"is_active":       true,
					"type":            7,
					"down_multiplier": 0.3,
					"up_multiplier":   1,
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(resp)
		_, _ = w.Write(b)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewRousiAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Passkey: "testkey"}
	result, err := a.DetectDiscount(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Level != model.DiscountPercent30 {
		t.Errorf("expected PERCENT_30, got %s", result.Level)
	}
}

func TestRousiAdapter_DetectDiscount_FallbackFree(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]interface{}{
			"data": map[string]interface{}{
				"promotion": map[string]interface{}{
					"is_active":       true,
					"type":            99,
					"down_multiplier": 0,
					"up_multiplier":   1,
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(resp)
		_, _ = w.Write(b)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewRousiAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Passkey: "testkey"}
	result, err := a.DetectDiscount(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Level != model.DiscountFree {
		t.Errorf("expected FREE fallback, got %s", result.Level)
	}
}

func TestRousiAdapter_DetectDiscount_NoPromo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]interface{}{
			"data": map[string]interface{}{
				"promotion": map[string]interface{}{
					"is_active":       false,
					"type":            0,
					"down_multiplier": 1,
					"up_multiplier":   1,
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(resp)
		_, _ = w.Write(b)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewRousiAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Passkey: "testkey"}
	result, err := a.DetectDiscount(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Level != model.DiscountNone {
		t.Errorf("expected NONE, got %s", result.Level)
	}
}

func TestRousiAdapter_DetectDiscount_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewRousiAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Passkey: "testkey"}
	result, err := a.DetectDiscount(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Level != model.DiscountNone {
		t.Errorf("expected NONE on error, got %s", result.Level)
	}
}

func TestRousiAdapter_DetectHR(t *testing.T) {
	a := NewRousiAdapter(NewHTTPDoer(), zap.NewNop())
	config := &model.SiteConfig{Domain: "https://rousi.pro", Passkey: "testkey"}
	result, err := a.DetectHR(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if result.HasHR {
		t.Error("expected no HR for rousi")
	}
}

func TestRousiAdapter_GetPreciseSLData_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]interface{}{
			"data": map[string]interface{}{
				"seeders":  10,
				"leechers": 5,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(resp)
		_, _ = w.Write(b)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewRousiAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Passkey: "testkey"}
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

func TestRousiAdapter_GetPreciseSLData_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewRousiAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Passkey: "testkey"}
	sl, err := a.GetPreciseSLData(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if sl.Seeders != 0 || sl.Leechers != 0 {
		t.Errorf("expected empty SLData on error, got seeders=%d leechers=%d", sl.Seeders, sl.Leechers)
	}
}

func TestRousiAdapter_UploadTorrent_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/torrents" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer testkey" {
			t.Errorf("missing Bearer auth header")
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("unexpected content type: %s", r.Header.Get("Content-Type"))
		}
		resp := map[string]interface{}{
			"code":    0,
			"message": "",
			"data": map[string]interface{}{
				"UUID": "abc-123",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(resp)
		_, _ = w.Write(b)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewRousiAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Passkey: "testkey"}
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
	if resp.TorrentID != "abc-123" {
		t.Errorf("expected torrent ID abc-123, got %s", resp.TorrentID)
	}
	if resp.DetailURL != srv.URL+"/torrent/abc-123" {
		t.Errorf("unexpected detail URL: %s", resp.DetailURL)
	}
}

func TestRousiAdapter_UploadTorrent_ApiError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]interface{}{
			"code":    1,
			"message": "duplicate",
			"data":    nil,
		}
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(resp)
		_, _ = w.Write(b)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewRousiAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Passkey: "testkey"}
	resp, err := a.UploadTorrent(context.Background(), config, &model.PublishRequest{
		TorrentData: []byte("d4:infod6:lengthi0eee"),
		Title:       "Dup",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Success {
		t.Error("expected failure for API error")
	}
	if resp.ErrorMessage == "" {
		t.Error("expected error message")
	}
}

func TestRousiAdapter_UploadTorrent_NoData(t *testing.T) {
	a := NewRousiAdapter(NewHTTPDoer(), zap.NewNop())
	config := &model.SiteConfig{Domain: "https://rousi.pro"}

	_, err := a.UploadTorrent(context.Background(), config, &model.PublishRequest{})
	if err == nil {
		t.Fatal("expected error for empty torrent data")
	}
}

func TestRousiAdapter_UploadTorrent_WithAttributes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["title"] != "Attr Test" {
			t.Errorf("unexpected title: %v", body["title"])
		}
		if body["category"] != "movie" {
			t.Errorf("unexpected category: %v", body["category"])
		}
		attrs, ok := body["attributes"].(map[string]interface{})
		if !ok {
			t.Fatal("expected attributes map")
		}
		if attrs["resolution"] != "1080p" {
			t.Errorf("unexpected resolution: %v", attrs["resolution"])
		}
		if attrs["source"] != "blu-ray" {
			t.Errorf("unexpected source: %v", attrs["source"])
		}
		if attrs["imdb"] != "https://www.imdb.com/title/tt1234567/" {
			t.Errorf("unexpected imdb: %v", attrs["imdb"])
		}

		resp := map[string]interface{}{
			"code":    0,
			"message": "",
			"data":    map[string]interface{}{"UUID": "xyz-789"},
		}
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(resp)
		_, _ = w.Write(b)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewRousiAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Passkey: "testkey"}
	resp, err := a.UploadTorrent(context.Background(), config, &model.PublishRequest{
		TorrentData: []byte("d4:infod6:lengthi0eee"),
		Title:       "Attr Test",
		FormFields: map[string]string{
			"cat":        "movie",
			"resolution": "1080p",
			"source":     "blu-ray",
		},
		IMDbLink: "https://www.imdb.com/title/tt1234567/",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Success {
		t.Errorf("expected success, got: %s", resp.ErrorMessage)
	}
	if resp.TorrentID != "xyz-789" {
		t.Errorf("expected torrent ID xyz-789, got %s", resp.TorrentID)
	}
}

func TestRousiAdapter_GetTorrentInfoHash_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]interface{}{
			"code": 0,
			"data": map[string]interface{}{
				"uuid":      "abc-123",
				"title":     "Test",
				"info_hash": "DEADBEEFDEADBEEFDEADBEEFDEADBEEFDEADBEEF",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(resp)
		_, _ = w.Write(b)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewRousiAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Passkey: "testkey"}
	hash, err := a.GetTorrentInfoHash(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if hash != "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef" {
		t.Errorf("unexpected hash: %s", hash)
	}
}

func TestRousiAdapter_GetTorrentInfoHash_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewRousiAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Passkey: "testkey"}
	_, err := a.GetTorrentInfoHash(context.Background(), config, "1")
	if err == nil {
		t.Fatal("expected error for not found")
	}
}

func TestRousiAdapter_UploadWithImages(t *testing.T) {
	var receivedBody map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"code":0,"data":{"uuid":"img-test-uuid"}}`)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewRousiAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Passkey: "testpk"}
	req := &model.PublishRequest{
		TorrentData: []byte("d4:infod6:lengthi0eee"),
		Title:       "Test With Images",
		IMDbLink:    "https://www.imdb.com/title/tt1234/",
		DoubanLink:  "https://movie.douban.com/subject/5678/",
		Screenshots: []string{"https://img.example.com/1.png", "https://img.example.com/2.png"},
		ExtraFields: map[string]string{"tmdb_id": "12345"},
	}

	resp, err := a.UploadTorrent(context.Background(), config, req)
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Success {
		t.Errorf("expected success: %s", resp.ErrorMessage)
	}

	attrs, ok := receivedBody["attributes"].(map[string]interface{})
	if !ok {
		t.Fatal("expected attributes in payload")
	}
	if attrs["imdb"] != "https://www.imdb.com/title/tt1234/" {
		t.Errorf("expected imdb in attributes, got %v", attrs["imdb"])
	}
	if attrs["douban"] != "https://movie.douban.com/subject/5678/" {
		t.Errorf("expected douban in attributes, got %v", attrs["douban"])
	}
	if attrs["tmdb"] != "12345" {
		t.Errorf("expected tmdb in attributes, got %v", attrs["tmdb"])
	}

	images, ok := receivedBody["images"].([]interface{})
	if !ok {
		t.Fatal("expected images in payload")
	}
	if len(images) != 2 {
		t.Errorf("expected 2 images, got %d", len(images))
	}
}

func TestResolveField(t *testing.T) {
	fields := map[string]string{"category": "401", "cat": "movie"}

	if v := resolveField(fields, "cat", "category"); v != "movie" {
		t.Errorf("expected first key 'cat' to win, got %q", v)
	}

	single := map[string]string{"category": "401"}
	if v := resolveField(single, "cat", "category"); v != "401" {
		t.Errorf("expected fallback to 'category', got %q", v)
	}

	empty := map[string]string{}
	if v := resolveField(empty, "cat", "category"); v != "" {
		t.Errorf("expected empty string, got %q", v)
	}
}
