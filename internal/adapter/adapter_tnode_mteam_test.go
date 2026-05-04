package adapter

import (
	"context"
	"net/http"
	"net/http/httptest"
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
		if r.URL.Path != "/download.php" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("id") != "42" {
			t.Errorf("unexpected id: %s", r.URL.Query().Get("id"))
		}
		w.Header().Set("Content-Type", "application/x-bittorrent")
		_, _ = w.Write(payload)
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
		file, _, err := r.FormFile("torrent")
		if err != nil {
			t.Fatalf("form file: %v", err)
		}
		defer file.Close()
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
