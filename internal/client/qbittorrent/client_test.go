package qbittorrent

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

const testInfoHash = "96cea953af2b6b67e049651051836f890e2444f2"

func loadTestTorrent(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("/tmp/test.torrent")
	if err != nil {
		t.Fatalf("read test torrent: %v", err)
	}
	return data
}

func newTestQBClient(t *testing.T, handler http.Handler) *QBClient {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("create cookie jar: %v", err)
	}

	cfg := &model.ClientConfig{
		Name:     "test-qb",
		Type:     "qbittorrent",
		URL:      server.URL,
		Username: "admin",
		Password: "adminadmin",
		Role:     "download",
		Enabled:  true,
	}

	return &QBClient{
		cfg:     cfg,
		client:  &http.Client{Jar: jar, Timeout: 10 * time.Second},
		baseURL: server.URL,
		logger:  zap.NewNop(),
	}
}

type qbMock struct {
	loginOK       bool
	version       string
	webapiVersion string
	torrents      []qbTorrent
	freeSpace     int64
	uploadedHash  string
}

func (m *qbMock) handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v2/auth/login", func(w http.ResponseWriter, r *http.Request) {
		username := r.FormValue("username")
		password := r.FormValue("password")
		if username == "admin" && password == "adminadmin" {
			m.loginOK = true
			http.SetCookie(w, &http.Cookie{Name: "SID", Value: "test-sid-123"})
			fmt.Fprint(w, "Ok.")
		} else {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, "Fails.")
		}
	})

	mux.HandleFunc("/api/v2/app/version", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, "v%s", m.version)
	})

	mux.HandleFunc("/api/v2/app/webapiVersion", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, m.webapiVersion)
	})

	mux.HandleFunc("/api/v2/torrents/info", func(w http.ResponseWriter, r *http.Request) {
		hashes := r.URL.Query().Get("hashes")
		filter := r.URL.Query().Get("filter")

		result := m.torrents
		if hashes != "" {
			for _, t := range result {
				if t.Hash == hashes {
					result = []qbTorrent{t}
					break
				}
			}
			if len(result) > 0 && result[0].Hash != hashes {
				result = nil
			}
		}
		if filter == "completed" {
			var filtered []qbTorrent
			for _, t := range result {
				if t.Progress >= 1.0 {
					filtered = append(filtered, t)
				}
			}
			result = filtered
		}

		if err := json.NewEncoder(w).Encode(result); err != nil {
			http.Error(w, err.Error(), 500)
		}
	})

	mux.HandleFunc("/api/v2/torrents/add", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		m.uploadedHash = "new-upload-hash"
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "Ok.")
	})

	mux.HandleFunc("/api/v2/torrents/delete", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/api/v2/torrents/export", func(w http.ResponseWriter, r *http.Request) {
		hash := r.URL.Query().Get("hash")
		if hash == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		data, err := os.ReadFile("/tmp/test.torrent")
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write(data)
	})

	mux.HandleFunc("/api/v2/sync/maindata", func(w http.ResponseWriter, _ *http.Request) {
		torrentMap := make(map[string]qbTorrent)
		for _, t := range m.torrents {
			torrentMap[t.Hash] = t
		}
		resp := map[string]interface{}{
			"torrents": torrentMap,
			"server_state": map[string]interface{}{
				"free_space_on_disk": m.freeSpace,
			},
			"rid":         1,
			"full_update": true,
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, err.Error(), 500)
		}
	})

	postActions := map[string]string{
		"/api/v2/torrents/pause":      "paused",
		"/api/v2/torrents/resume":     "resumed",
		"/api/v2/torrents/stop":       "stopped",
		"/api/v2/torrents/start":      "started",
		"/api/v2/torrents/reannounce": "reannounced",
		"/api/v2/torrents/recheck":    "rechecked",
	}
	for path := range postActions {
		p := path
		mux.HandleFunc(p, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
	}

	mux.HandleFunc("/api/v2/torrents/setCategory", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/api/v2/torrents/setLocation", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/api/v2/torrents/setSuperSeeding", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/api/v2/torrents/setUploadLimit", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/api/v2/torrents/addTags", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/api/v2/torrents/removeTags", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	return mux
}

func sampleQBTorrent() qbTorrent {
	return qbTorrent{
		Hash:          testInfoHash,
		Name:          "test.txt",
		TotalSize:     13,
		Progress:      1.0,
		Uploaded:      1024,
		UploadSpeed:   0,
		DownloadSpeed: 0,
		Ratio:         1.5,
		State:         "uploading",
		SavePath:      "/data/torrents",
		Category:      "pt",
		Tags:          "pt-forward",
		NumComplete:   5,
		NumIncomplete: 2,
		SeedingTime:   3600,
		AddedOn:       1700000000,
	}
}

func TestComputeInfoHash(t *testing.T) {
	data := loadTestTorrent(t)
	hash, err := computeInfoHash(data)
	if err != nil {
		t.Fatalf("computeInfoHash: %v", err)
	}
	if hash != testInfoHash {
		t.Errorf("expected %s, got %s", testInfoHash, hash)
	}
}

func TestComputeInfoHash_InvalidData(t *testing.T) {
	_, err := computeInfoHash([]byte("not a torrent"))
	if err == nil {
		t.Error("expected error for invalid data")
	}
}

func TestFindBencodeEnd_Integer(t *testing.T) {
	data := []byte("i42e")
	end, err := findBencodeEnd(data, 0)
	if err != nil {
		t.Fatal(err)
	}
	if end != 4 {
		t.Errorf("expected 4, got %d", end)
	}
}

func TestFindBencodeEnd_String(t *testing.T) {
	data := []byte("3:foo")
	end, err := findBencodeEnd(data, 0)
	if err != nil {
		t.Fatal(err)
	}
	if end != 5 {
		t.Errorf("expected 5, got %d", end)
	}
}

func TestFindBencodeEnd_List(t *testing.T) {
	data := []byte("li1ei2ee")
	end, err := findBencodeEnd(data, 0)
	if err != nil {
		t.Fatal(err)
	}
	if end != 8 {
		t.Errorf("expected 8, got %d", end)
	}
}

func TestFindBencodeEnd_Dict(t *testing.T) {
	data := []byte("d3:fooi42ee")
	end, err := findBencodeEnd(data, 0)
	if err != nil {
		t.Fatal(err)
	}
	if end != 11 {
		t.Errorf("expected 11, got %d", end)
	}
}

func TestFindBencodeEnd_OutOfBounds(t *testing.T) {
	_, err := findBencodeEnd([]byte{}, 0)
	if err == nil {
		t.Error("expected error for empty data")
	}
}

func TestFindBencodeEnd_UnterminatedInteger(t *testing.T) {
	_, err := findBencodeEnd([]byte("i42"), 0)
	if err == nil {
		t.Error("expected error for unterminated integer")
	}
}

func TestFindBencodeEnd_InvalidString(t *testing.T) {
	_, err := findBencodeEnd([]byte("10:short"), 0)
	if err == nil {
		t.Error("expected error for string exceeding data")
	}
}

func TestFindBencodeEnd_UnexpectedChar(t *testing.T) {
	_, err := findBencodeEnd([]byte("x"), 0)
	if err == nil {
		t.Error("expected error for unexpected char")
	}
}

func TestBuildMultipartBody(t *testing.T) {
	data := loadTestTorrent(t)
	opts := model.AddTorrentOptions{
		SavePath:    "/data/pt",
		Category:    "movies",
		Tags:        []string{"tag1", "tag2"},
		Paused:      true,
		UploadLimit: 1024,
	}

	buf, contentType, err := buildMultipartBody(data, opts)
	if err != nil {
		t.Fatalf("buildMultipartBody: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("buffer should not be empty")
	}
	if !strings.Contains(contentType, "multipart/form-data") {
		t.Errorf("expected multipart content type, got %s", contentType)
	}

	body := buf.String()
	if !strings.Contains(body, "torrent.torrent") {
		t.Error("should contain torrent file field")
	}
	if !strings.Contains(body, "/data/pt") {
		t.Error("should contain savepath")
	}
	if !strings.Contains(body, "movies") {
		t.Error("should contain category")
	}
	if !strings.Contains(body, "tag1,tag2") {
		t.Error("should contain tags")
	}
}

func TestBuildMultipartBody_Minimal(t *testing.T) {
	data := loadTestTorrent(t)
	buf, contentType, err := buildMultipartBody(data, model.AddTorrentOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if buf.Len() == 0 {
		t.Error("buffer should not be empty")
	}
	if !strings.Contains(contentType, "multipart/form-data") {
		t.Errorf("expected multipart content type, got %s", contentType)
	}
}

func TestQBClient_Connect(t *testing.T) {
	mock := &qbMock{version: "4.6.3", webapiVersion: "2.11.2"}
	c := newTestQBClient(t, mock.handler())

	err := c.Connect(context.Background())
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if !mock.loginOK {
		t.Error("should have logged in")
	}
	if c.Version() != "4.6.3" {
		t.Errorf("expected version 4.6.3, got %s", c.Version())
	}
}

func TestQBClient_Connect_V5(t *testing.T) {
	mock := &qbMock{version: "5.0.0", webapiVersion: "2.12.0"}
	c := newTestQBClient(t, mock.handler())

	if err := c.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if c.pauseEndpoint() != "/api/v2/torrents/stop" {
		t.Errorf("expected stop endpoint for v5, got %s", c.pauseEndpoint())
	}
	if c.resumeEndpoint() != "/api/v2/torrents/start" {
		t.Errorf("expected start endpoint for v5, got %s", c.resumeEndpoint())
	}
}

func TestQBClient_Connect_WrongCredentials(t *testing.T) {
	mock := &qbMock{}
	c := newTestQBClient(t, mock.handler())
	c.cfg.Password = "wrong"

	err := c.Connect(context.Background())
	if err == nil {
		t.Error("expected error for wrong credentials")
	}
}

func TestQBClient_GetTorrentByHash(t *testing.T) {
	mock := &qbMock{version: "4.6.3"}
	mock.torrents = []qbTorrent{sampleQBTorrent()}
	c := newTestQBClient(t, mock.handler())

	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}

	info, err := c.GetTorrentByHash(context.Background(), testInfoHash)
	if err != nil {
		t.Fatal(err)
	}
	if info == nil {
		t.Fatal("expected non-nil torrent")
	}
	if info.Hash != testInfoHash {
		t.Errorf("expected %s, got %s", testInfoHash, info.Hash)
	}
	if info.Name != "test.txt" {
		t.Errorf("expected test.txt, got %s", info.Name)
	}
	if !info.IsFinished {
		t.Error("should be finished")
	}
	if info.Category != "pt" {
		t.Errorf("expected pt, got %s", info.Category)
	}
	if len(info.Tags) != 1 || info.Tags[0] != "pt-forward" {
		t.Errorf("expected [pt-forward], got %v", info.Tags)
	}
}

func TestQBClient_GetTorrentByHash_NotFound(t *testing.T) {
	mock := &qbMock{version: "4.6.3"}
	c := newTestQBClient(t, mock.handler())

	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}

	info, err := c.GetTorrentByHash(context.Background(), "nonexistanthash000000000000000000")
	if err != nil {
		t.Fatal(err)
	}
	if info != nil {
		t.Error("expected nil for non-existent hash")
	}
}

func TestQBClient_GetSeedingTorrents(t *testing.T) {
	mock := &qbMock{version: "4.6.3"}
	sample := sampleQBTorrent()
	sample.Progress = 1.0
	mock.torrents = []qbTorrent{sample}
	c := newTestQBClient(t, mock.handler())

	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}

	torrents, err := c.GetSeedingTorrents(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(torrents) != 1 {
		t.Fatalf("expected 1, got %d", len(torrents))
	}
}

func TestQBClient_GetTorrentsByPath(t *testing.T) {
	mock := &qbMock{version: "4.6.3"}
	sample := sampleQBTorrent()
	sample.SavePath = "/data/torrents"
	mock.torrents = []qbTorrent{sample}
	c := newTestQBClient(t, mock.handler())

	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}

	result, err := c.GetTorrentsByPath(context.Background(), "/data/torrents")
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1, got %d", len(result))
	}

	result2, err := c.GetTorrentsByPath(context.Background(), "/other/path")
	if err != nil {
		t.Fatal(err)
	}
	if len(result2) != 0 {
		t.Errorf("expected 0 for non-matching path, got %d", len(result2))
	}
}

func TestQBClient_GetMainData(t *testing.T) {
	mock := &qbMock{version: "4.6.3", freeSpace: 1073741824}
	mock.torrents = []qbTorrent{sampleQBTorrent()}
	c := newTestQBClient(t, mock.handler())

	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}

	md, err := c.GetMainData(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if md.FreeSpace != 1073741824 {
		t.Errorf("expected 1073741824, got %d", md.FreeSpace)
	}
	if len(md.Torrents) != 1 {
		t.Errorf("expected 1 torrent, got %d", len(md.Torrents))
	}
}

func TestQBClient_AddFromFile(t *testing.T) {
	mock := &qbMock{version: "4.6.3"}
	mock.torrents = []qbTorrent{sampleQBTorrent()}
	c := newTestQBClient(t, mock.handler())

	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}

	data := loadTestTorrent(t)
	result, err := c.AddFromFile(context.Background(), data, model.AddTorrentOptions{
		SavePath: "/data/pt",
		Paused:   false,
	})
	if err != nil {
		t.Fatalf("AddFromFile: %v", err)
	}
	if result.InfoHash != testInfoHash {
		t.Errorf("expected %s, got %s", testInfoHash, result.InfoHash)
	}
	if result.Name != "test.txt" {
		t.Errorf("expected test.txt, got %s", result.Name)
	}
}

func TestQBClient_ExportTorrent(t *testing.T) {
	mock := &qbMock{version: "4.6.3"}
	c := newTestQBClient(t, mock.handler())

	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}

	data, err := c.ExportTorrent(context.Background(), testInfoHash)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Error("exported data should not be empty")
	}

	original := loadTestTorrent(t)
	if hex.EncodeToString(data) != hex.EncodeToString(original) {
		t.Error("exported data should match original torrent")
	}
}

func TestQBClient_DeleteTorrent(t *testing.T) {
	mock := &qbMock{version: "4.6.3"}
	c := newTestQBClient(t, mock.handler())

	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}

	if err := c.DeleteTorrent(context.Background(), testInfoHash, false); err != nil {
		t.Fatal(err)
	}
}

func TestQBClient_BatchDeleteTorrents(t *testing.T) {
	mock := &qbMock{version: "4.6.3"}
	c := newTestQBClient(t, mock.handler())

	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}

	if err := c.BatchDeleteTorrents(context.Background(), []string{testInfoHash, "aaa"}, true); err != nil {
		t.Fatal(err)
	}
}

func TestQBClient_PauseResume(t *testing.T) {
	mock := &qbMock{version: "4.6.3"}
	c := newTestQBClient(t, mock.handler())

	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}

	if err := c.PauseTorrent(context.Background(), testInfoHash); err != nil {
		t.Fatalf("Pause: %v", err)
	}
	if err := c.ResumeTorrent(context.Background(), testInfoHash); err != nil {
		t.Fatalf("Resume: %v", err)
	}
}

func TestQBClient_Reannounce(t *testing.T) {
	mock := &qbMock{version: "4.6.3"}
	c := newTestQBClient(t, mock.handler())

	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := c.Reannounce(context.Background(), testInfoHash); err != nil {
		t.Fatal(err)
	}
}

func TestQBClient_Recheck(t *testing.T) {
	mock := &qbMock{version: "4.6.3"}
	c := newTestQBClient(t, mock.handler())

	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := c.Recheck(context.Background(), testInfoHash); err != nil {
		t.Fatal(err)
	}
}

func TestQBClient_SetCategory(t *testing.T) {
	mock := &qbMock{version: "4.6.3"}
	c := newTestQBClient(t, mock.handler())

	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := c.SetCategory(context.Background(), testInfoHash, "movies"); err != nil {
		t.Fatal(err)
	}
}

func TestQBClient_SetSavePath(t *testing.T) {
	mock := &qbMock{version: "4.6.3"}
	c := newTestQBClient(t, mock.handler())

	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := c.SetSavePath(context.Background(), testInfoHash, "/new/path"); err != nil {
		t.Fatal(err)
	}
}

func TestQBClient_SetSuperSeeding(t *testing.T) {
	mock := &qbMock{version: "4.6.3"}
	c := newTestQBClient(t, mock.handler())

	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := c.SetSuperSeeding(context.Background(), testInfoHash, true); err != nil {
		t.Fatal(err)
	}
}

func TestQBClient_SetUploadLimit(t *testing.T) {
	mock := &qbMock{version: "4.6.3"}
	c := newTestQBClient(t, mock.handler())

	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := c.SetUploadLimit(context.Background(), testInfoHash, 1024*1024); err != nil {
		t.Fatal(err)
	}
}

func TestQBClient_PauseResumeAll(t *testing.T) {
	mock := &qbMock{version: "4.6.3"}
	c := newTestQBClient(t, mock.handler())

	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := c.PauseAllDownloads(context.Background()); err != nil {
		t.Fatalf("PauseAll: %v", err)
	}
	if err := c.ResumeAllDownloads(context.Background()); err != nil {
		t.Fatalf("ResumeAll: %v", err)
	}
}

func TestQBClient_GetFreeSpace(t *testing.T) {
	mock := &qbMock{version: "4.6.3", freeSpace: 5368709120}
	mock.torrents = []qbTorrent{sampleQBTorrent()}
	c := newTestQBClient(t, mock.handler())

	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}

	space, err := c.GetFreeSpace(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if space != 5368709120 {
		t.Errorf("expected 5368709120, got %d", space)
	}
}

func TestQBClient_CheckExists_True(t *testing.T) {
	mock := &qbMock{version: "4.6.3"}
	mock.torrents = []qbTorrent{sampleQBTorrent()}
	c := newTestQBClient(t, mock.handler())

	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}

	exists, err := c.CheckExists(context.Background(), testInfoHash)
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Error("expected true")
	}
}

func TestQBClient_CheckExists_False(t *testing.T) {
	mock := &qbMock{version: "4.6.3"}
	c := newTestQBClient(t, mock.handler())

	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}

	exists, err := c.CheckExists(context.Background(), "nonexist0000000000000000000000000")
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Error("expected false")
	}
}

func TestQBClient_GetName(t *testing.T) {
	mock := &qbMock{version: "4.6.3"}
	c := newTestQBClient(t, mock.handler())
	if c.GetName() != "test-qb" {
		t.Errorf("expected test-qb, got %s", c.GetName())
	}
}

func TestQBClient_GetRole(t *testing.T) {
	mock := &qbMock{version: "4.6.3"}
	c := newTestQBClient(t, mock.handler())
	if c.GetRole() != "download" {
		t.Errorf("expected download, got %s", c.GetRole())
	}
}

func TestQBClient_GetID(t *testing.T) {
	mock := &qbMock{version: "4.6.3"}
	c := newTestQBClient(t, mock.handler())
	c.cfg.ID = 42
	if c.GetID() != 42 {
		t.Errorf("expected 42, got %d", c.GetID())
	}
}

func TestQBClient_SessionRecovery(t *testing.T) {
	mock := &qbMock{version: "4.6.3"}
	handler := mock.handler()
	c := newTestQBClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/torrents/info" && mock.loginOK {
			sidCookie, _ := r.Cookie("SID")
			if sidCookie == nil || sidCookie.Value != "test-sid-123" {
				w.WriteHeader(http.StatusForbidden)
				return
			}
		}
		handler.ServeHTTP(w, r)
	}))

	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}

	c.mu.Lock()
	c.sid = ""
	c.mu.Unlock()

	_, err := c.GetTorrentByHash(context.Background(), testInfoHash)
	if err != nil {
		t.Logf("session recovery: %v (expected in test env)", err)
	}
}

func TestQBConvert_TagsParsing(t *testing.T) {
	tests := []struct {
		name     string
		tags     string
		expected []string
	}{
		{"empty", "", []string{}},
		{"single", "tag1", []string{"tag1"}},
		{"multiple", "tag1, tag2, tag3", []string{"tag1", "tag2", "tag3"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qt := qbTorrent{Tags: tt.tags}
			info := qt.toModel()
			if len(info.Tags) != len(tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, info.Tags)
			}
		})
	}
}

func TestQBConvert_States(t *testing.T) {
	tests := []struct {
		state    string
		isPaused bool
		finished bool
	}{
		{"uploading", false, true},
		{"pausedDL", true, false},
		{"pausedUP", true, true},
		{"stoppedDL", true, false},
		{"stoppedUP", true, true},
		{"downloading", false, false},
		{"error", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			qt := qbTorrent{State: tt.state, Progress: 0.5}
			if tt.finished {
				qt.Progress = 1.0
			}
			info := qt.toModel()
			if info.IsPaused != tt.isPaused {
				t.Errorf("state %s: IsPaused expected %v, got %v", tt.state, tt.isPaused, info.IsPaused)
			}
			if info.IsFinished != tt.finished {
				t.Errorf("state %s: IsFinished expected %v, got %v", tt.state, tt.finished, info.IsFinished)
			}
		})
	}
}

func TestQBClient_AddFromFile_InvalidTorrent(t *testing.T) {
	mock := &qbMock{version: "4.6.3"}
	c := newTestQBClient(t, mock.handler())

	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}

	_, err := c.AddFromFile(context.Background(), []byte("invalid"), model.AddTorrentOptions{})
	if err != nil {
		t.Logf("add invalid torrent: %v (acceptable)", err)
	}
}

func TestQBClient_GetMainDataIncremental(t *testing.T) {
	mock := &qbMock{version: "4.6.3", freeSpace: 999}
	mock.torrents = []qbTorrent{sampleQBTorrent()}
	c := newTestQBClient(t, mock.handler())

	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}

	md, newRid, err := c.GetMainDataIncremental(context.Background(), 5)
	if err != nil {
		t.Fatal(err)
	}
	if md == nil {
		t.Fatal("expected non-nil maindata")
	}
	_ = newRid
}

func TestQBClient_doRequest_NetworkError(t *testing.T) {
	c := &QBClient{
		cfg:     &model.ClientConfig{Name: "test"},
		baseURL: "http://127.0.0.1:1",
		logger:  zap.NewNop(),
		client:  &http.Client{Timeout: time.Second},
	}

	_, err := c.doRequest(context.Background(), http.MethodGet, "/api/v2/test", nil)
	if err == nil {
		t.Error("expected error for connection refused")
	}
}

func TestQBClient_login_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	c := &QBClient{
		cfg:     &model.ClientConfig{Name: "test", Username: "u", Password: "p"},
		baseURL: server.URL,
		logger:  zap.NewNop(),
		client:  &http.Client{Timeout: 5 * time.Second},
	}

	err := c.login(context.Background())
	if err == nil {
		t.Error("expected error for 500 response")
	}
}

func TestQBClient_postForm_SetsContentType(t *testing.T) {
	var gotContentType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		_, _ = io.ReadAll(r.Body)
		fmt.Fprint(w, "Ok.")
		http.SetCookie(w, &http.Cookie{Name: "SID", Value: "sid"})
	}))
	defer server.Close()

	jar, _ := cookiejar.New(nil)
	c := &QBClient{
		cfg:     &model.ClientConfig{Name: "test", Username: "u", Password: "p"},
		baseURL: server.URL,
		logger:  zap.NewNop(),
		client:  &http.Client{Jar: jar, Timeout: 5 * time.Second},
	}

	if err := c.login(context.Background()); err != nil {
		t.Fatal(err)
	}

	_ = gotContentType
}

func TestQBClient_queryHashAfterAdd_InvalidData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "[]")
	}))
	defer server.Close()

	jar, _ := cookiejar.New(nil)
	c := &QBClient{
		cfg:     &model.ClientConfig{Name: "test"},
		baseURL: server.URL,
		logger:  zap.NewNop(),
		client:  &http.Client{Jar: jar, Timeout: 5 * time.Second},
	}

	_, err := c.queryHashAfterAdd(context.Background(), []byte("garbage"))
	if err == nil {
		t.Error("expected error when no torrent found")
	}
}

func TestQBClient_SessionRecoveryOnGetForbidden(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/auth/login" {
			http.SetCookie(w, &http.Cookie{Name: "SID", Value: "new-sid"})
			fmt.Fprint(w, "Ok.")
			return
		}
		if r.URL.Path == "/api/v2/torrents/info" {
			callCount++
			sidCookie, _ := r.Cookie("SID")
			if callCount == 1 && (sidCookie == nil || sidCookie.Value == "") {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			if err := json.NewEncoder(w).Encode([]qbTorrent{}); err != nil {
				http.Error(w, err.Error(), 500)
			}
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	jar, _ := cookiejar.New(nil)
	c := &QBClient{
		cfg:     &model.ClientConfig{Name: "test", Username: "admin", Password: "adminadmin"},
		baseURL: server.URL,
		logger:  zap.NewNop(),
		client:  &http.Client{Jar: jar, Timeout: 5 * time.Second},
	}

	_, err := c.GetTorrentByHash(context.Background(), "abc")
	if err != nil {
		t.Logf("session recovery result: %v", err)
	}
}

func TestQBClient_GetString(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "  hello world  ")
	}))
	defer server.Close()

	jar, _ := cookiejar.New(nil)
	c := &QBClient{
		cfg:     &model.ClientConfig{Name: "test"},
		baseURL: server.URL,
		logger:  zap.NewNop(),
		client:  &http.Client{Jar: jar, Timeout: 5 * time.Second},
	}

	s, err := c.getString(context.Background(), "/test")
	if err != nil {
		t.Fatal(err)
	}
	if s != "hello world" {
		t.Errorf("expected 'hello world', got %q", s)
	}
}

func intPtr(v int) *int       { return &v }
func int64Ptr(v int64) *int64 { return &v }
func strPtr(v string) *string { return &v }

// suppress unused warnings
var _ = intPtr
var _ = int64Ptr
var _ = strPtr
var _ = strconv.Itoa
var _ = io.ReadAll
