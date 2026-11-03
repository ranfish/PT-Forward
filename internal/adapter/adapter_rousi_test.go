package adapter

import (
	"strings"
	"context"
	"encoding/json"
	"errors"
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

	config := &model.SiteConfig{Domain: srv.URL, APIKey: "testkey"}
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

	config := &model.SiteConfig{Domain: srv.URL, APIKey: "testkey"}
	_, err := a.DownloadTorrent(context.Background(), config, "42")
	if err == nil {
		t.Fatal("expected error for 500")
	}
}

func TestRousiAdapter_DownloadTorrent_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewRousiAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, APIKey: "testkey"}
	_, err := a.DownloadTorrent(context.Background(), config, "42")
	if err == nil {
		t.Fatal("expected error for 404")
	}
	var appErr *model.AppError
	if !errors.As(err, &appErr) || appErr.Code != ErrAdapterNotFound {
		t.Errorf("expected ErrAdapterNotFound, got %v", err)
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
				"description": "Desc content",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(resp)
		_, _ = w.Write(b)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewRousiAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, APIKey: "testkey"}
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
	// §59.457 附三: Subtitle/Description 防丢字段锚（首版 struct 解析 Subtitle
	// 未赋值+Description 未解析——获取链完整性缺口端到端实证抓出）
	if detail.Subtitle != "Sub" {
		t.Errorf("unexpected subtitle: %q", detail.Subtitle)
	}
	if detail.Description != "Desc content" {
		t.Errorf("unexpected description: %q", detail.Description)
	}
}

func TestRousiAdapter_GetTorrentDetail_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewRousiAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, APIKey: "testkey"}
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

	config := &model.SiteConfig{Domain: srv.URL, APIKey: "testkey"}
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

	config := &model.SiteConfig{Domain: srv.URL, APIKey: "testkey"}
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

	config := &model.SiteConfig{Domain: srv.URL, APIKey: "testkey"}
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

	config := &model.SiteConfig{Domain: srv.URL, APIKey: "testkey"}
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

	config := &model.SiteConfig{Domain: srv.URL, APIKey: "testkey"}
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

	config := &model.SiteConfig{Domain: srv.URL, APIKey: "testkey"}
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

	config := &model.SiteConfig{Domain: srv.URL, APIKey: "testkey"}
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

	config := &model.SiteConfig{Domain: srv.URL, APIKey: "testkey"}
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

	config := &model.SiteConfig{Domain: srv.URL, APIKey: "testkey"}
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

	config := &model.SiteConfig{Domain: srv.URL, APIKey: "testkey"}
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

	config := &model.SiteConfig{Domain: srv.URL, APIKey: "testkey"}
	_, err := a.DetectDiscount(context.Background(), config, "1")
	if err == nil {
		t.Fatal("expected error on HTTP 500 response")
	}
}

func TestRousiAdapter_DetectHR(t *testing.T) {
	a := NewRousiAdapter(NewHTTPDoer(), zap.NewNop())
	config := &model.SiteConfig{Domain: "https://rousi.pro", APIKey: "testkey"}
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

	config := &model.SiteConfig{Domain: srv.URL, APIKey: "testkey"}
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

	config := &model.SiteConfig{Domain: srv.URL, APIKey: "testkey"}
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
			"message": "success",
			"data": map[string]interface{}{
				"id": 10264, "uuid": "rt-abc-123", "info_hash": "aa", "status": "pending",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(resp)
		_, _ = w.Write(b)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewRousiAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, APIKey: "testkey"}
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
	// §59.458-⑧: 数字 id 权威（uuid 不消费）；pending/approved 均成功
	if resp.TorrentID != "10264" {
		t.Errorf("expected torrent ID 10264, got %s", resp.TorrentID)
	}
	if resp.DetailURL != srv.URL+"/torrents/10264" {
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

	config := &model.SiteConfig{Domain: srv.URL, APIKey: "testkey"}
	resp, err := a.UploadTorrent(context.Background(), config, &model.PublishRequest{
		TorrentData: []byte("d4:infod6:lengthi0eee"),
		Title:       "Dup",
	})
	if err == nil {
		t.Fatal("expected error for API error")
	}
	var appErr *model.AppError
	if !errors.As(err, &appErr) || appErr.Code != 15001 {
		t.Fatalf("expected AppError 15001, got %v", err)
	}
	_ = resp
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
		// §59.458-②③: genre/region 从 ExtraFields 终态直投
		genres, _ := attrs["genre"].([]interface{})
		if len(genres) != 2 || genres[0] != "动画" {
			t.Errorf("unexpected genre: %v", attrs["genre"])
		}
		if attrs["region"] != "mainland-china" {
			t.Errorf("unexpected region: %v", attrs["region"])
		}
		// §59.458-⑥: imdb/douban 顶层 URL（不进 attributes）
		if body["imdb"] != "https://www.imdb.com/title/tt1234567/" {
			t.Errorf("unexpected top-level imdb: %v", body["imdb"])
		}
		// §59.458-④: medium=web-dl → release-type（rt 集分流）
		if attrs["release-type"] != "web-dl" {
			t.Errorf("unexpected release-type: %v", attrs["release-type"])
		}

		resp := map[string]interface{}{
			"code":    0,
			"message": "success",
			"data":    map[string]interface{}{"id": 20001, "uuid": "rt-xyz-789", "status": "approved"},
		}
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(resp)
		_, _ = w.Write(b)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewRousiAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, APIKey: "testkey"}
	resp, err := a.UploadTorrent(context.Background(), config, &model.PublishRequest{
		TorrentData: []byte("d4:infod6:lengthi0eee"),
		Title:       "Attr Test",
		FormFields: map[string]string{
			"cat":        "movie",
			"resolution": "1080p",
			"medium":     "web-dl",
		},
		IMDbLink: "https://www.imdb.com/title/tt1234567/",
		ExtraFields: map[string]string{
			"rs_genre":  `["动画","喜剧"]`,
			"rs_region": "mainland-china",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Success {
		t.Errorf("expected success, got: %s", resp.ErrorMessage)
	}
	if resp.TorrentID != "20001" {
		t.Errorf("expected torrent ID 20001, got %s", resp.TorrentID)
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

	config := &model.SiteConfig{Domain: srv.URL, APIKey: "testkey"}
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

	config := &model.SiteConfig{Domain: srv.URL, APIKey: "testkey"}
	_, err := a.GetTorrentInfoHash(context.Background(), config, "1")
	if err == nil {
		t.Fatal("expected error for not found")
	}
}

func TestRousiAdapter_UploadWithImages(t *testing.T) {
	var receivedBody map[string]interface{}
	var gotIdemKey bool

	// mock 图床（封面+截图下载源——dataURL 现算即弃语义）
	imgSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("\x89PNG-fake-image-bytes"))
	}))
	defer imgSrv.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		gotIdemKey = r.Header.Get("Idempotency-Key") != ""
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"code":0,"data":{"id":30001,"uuid":"img-test","status":"pending"}}`)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewRousiAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, APIKey: "testkey"}
	req := &model.PublishRequest{
		TorrentData: []byte("d4:infod6:lengthi0eee"),
		Title:       "Test With Images",
		Description: "简介[img]https://img.example.com/1.png[/img]尾部",
		IMDbLink:    "https://www.imdb.com/title/tt1234/",
		DoubanLink:  "https://movie.douban.com/subject/5678/",
		// §59.459 附二: URL 形态=Tab3 图床链——现场下载 base64（非 URL 防御跳过）
		Screenshots: []string{imgSrv.URL + "/shot1.jpg", "/local/path.png", imgSrv.URL + "/shot2.jpg"},
		ExtraFields: map[string]string{
			"rs_poster": imgSrv.URL + "/poster.jpg",
			"tmdb_id":   "12345",
		},
	}

	resp, err := a.UploadTorrent(context.Background(), config, req)
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Success {
		t.Errorf("expected success: %s", resp.ErrorMessage)
	}
	if !gotIdemKey {
		t.Error("expected Idempotency-Key header")
	}
	// §59.458-⑥: 链接顶层（tmdb 定案不投）
	if receivedBody["imdb"] != "https://www.imdb.com/title/tt1234/" {
		t.Errorf("imdb: %v", receivedBody["imdb"])
	}
	if receivedBody["tmdb_id"] != nil {
		t.Errorf("tmdb 不应投递: %v", receivedBody["tmdb_id"])
	}
	// §59.458-⑤: description 禁图
	if desc, _ := receivedBody["description"].(string); strings.Contains(desc, "img.example.com") || strings.Contains(desc, "[img") {
		t.Errorf("description 图未剥除: %q", desc)
	}
	// §59.459 附二: images=封面+截图 URL 下载（本地路径形态防御跳过）=3 张
	images, ok := receivedBody["images"].([]interface{})
	if !ok {
		t.Fatal("expected images in payload")
	}
	if len(images) != 3 {
		t.Fatalf("expected 3 images（封面+2 URL 截图，路径跳过）, got %d", len(images))
	}
	for i, im := range images {
		if dataURL, _ := im.(string); !strings.HasPrefix(dataURL, "data:image/") {
			t.Errorf("image[%d] 非 dataURL: %q", i, dataURL[:min(30, len(dataURL))])
		}
	}
	// §59.459 附二: ≤6 数量门锚（7 张 URL 输入 → 封面+5 截图=6 截断）
	many := make([]string, 7)
	for i := range many {
		many[i] = imgSrv.URL + "/s.jpg"
	}
	req2 := &model.PublishRequest{
		TorrentData: []byte("d4:infod6:lengthi0eee"),
		Title:       "Gate Test",
		Screenshots: many,
		ExtraFields: map[string]string{"rs_poster": imgSrv.URL + "/p.jpg"},
	}
	receivedBody = nil
	if _, err := a.UploadTorrent(context.Background(), config, req2); err != nil {
		t.Fatal(err)
	}
	if imgs := receivedBody["images"].([]interface{}); len(imgs) != 6 {
		t.Errorf("数量门: got %d want 6", len(imgs))
	}
}

func TestRousiAdapter_SearchTorrents_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/torrents" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer testkey" {
			t.Errorf("missing Bearer auth header")
		}
		if r.URL.Query().Get("keyword") != "test" {
			t.Errorf("unexpected keyword param: %s", r.URL.Query().Get("keyword"))
		}
		resp := map[string]interface{}{
			"code": 0,
			"data": map[string]interface{}{
				"torrents": []map[string]interface{}{
					{"id": 1, "uuid": "abc-123", "title": "Test Torrent", "size": 1024, "seeders": 5, "leechers": 2, "category": "movie"},
					{"id": 2, "uuid": "def-456", "title": "Another Torrent", "size": 2048, "seeders": 3, "leechers": 1, "category": "tv"},
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

	config := &model.SiteConfig{Domain: srv.URL, APIKey: "testkey"}
	results, err := a.SearchTorrents(context.Background(), config, "test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].TorrentID != "abc-123" {
		t.Errorf("expected torrent ID abc-123, got %s", results[0].TorrentID)
	}
	if results[0].Title != "Test Torrent" {
		t.Errorf("unexpected title: %s", results[0].Title)
	}
	if results[0].Size != 1024 {
		t.Errorf("unexpected size: %d", results[0].Size)
	}
	if results[0].Seeders != 5 {
		t.Errorf("expected 5 seeders, got %d", results[0].Seeders)
	}
	if results[1].TorrentID != "def-456" {
		t.Errorf("expected torrent ID def-456, got %s", results[1].TorrentID)
	}
}

func TestRousiAdapter_VerifyExists_Found(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"code": 0,
			"data": map[string]interface{}{
				"torrents": []map[string]interface{}{
					{"id": 1, "uuid": "target-uuid", "title": "Found", "size": 1024, "seeders": 5, "leechers": 2, "category": "movie"},
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

	config := &model.SiteConfig{Domain: srv.URL, APIKey: "testkey"}
	found, err := a.VerifyExists(context.Background(), config, "target-uuid")
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Error("expected found")
	}
}

func TestRousiAdapter_VerifyExists_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"code": 0,
			"data": map[string]interface{}{
				"torrents": []map[string]interface{}{
					{"id": 1, "uuid": "other-uuid", "title": "Other", "size": 1024, "seeders": 5, "leechers": 2, "category": "movie"},
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

	config := &model.SiteConfig{Domain: srv.URL, APIKey: "testkey"}
	found, err := a.VerifyExists(context.Background(), config, "target-uuid")
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Error("expected not found")
	}
}

func TestRousiAdapter_DetectHR_IgnoresPageContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body>Hit and Run</body></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewRousiAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, APIKey: "testkey"}
	result, err := a.DetectHR(context.Background(), config, "1")
	if err != nil {
		t.Fatal(err)
	}
	if result.HasHR {
		t.Error("rousI never reports HR regardless of page content")
	}
	if result.SeedTimeH != 0 {
		t.Errorf("expected SeedTimeH 0, got %d", result.SeedTimeH)
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
