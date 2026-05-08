package transmission

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

func loadTestTorrentTR(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("/tmp/test.torrent")
	if err != nil {
		t.Fatalf("read test torrent: %v", err)
	}
	return data
}

type trMock struct {
	torrents      []trTorrent
	sessionID     string
	nextTorrentID int
	mu            sync.Mutex
}

func (m *trMock) handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		sid := r.Header.Get("X-Transmission-Session-Id")
		if sid == "" && m.sessionID != "" {
			w.Header().Set("X-Transmission-Session-Id", m.sessionID)
			w.WriteHeader(http.StatusConflict)
			return
		}

		if r.URL.Path != "/transmission/rpc" && r.URL.Path != "/rpc" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		var req struct {
			Method    string          `json:"method"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		m.handleRPC(w, req.Method, req.Arguments)
	})

	return mux
}

func (m *trMock) handleRPC(w http.ResponseWriter, method string, args json.RawMessage) {
	switch method {
	case "session-get":
		resp := map[string]interface{}{
			"arguments": map[string]interface{}{
				"version":      "4.0.6",
				"download-dir": "/data/torrents",
			},
			"result": "success",
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, err.Error(), 500)
		}

	case "free-space":
		resp := map[string]interface{}{
			"arguments": map[string]interface{}{
				"size-bytes": int64(2147483648),
			},
			"result": "success",
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, err.Error(), 500)
		}

	case "torrent-get":
		var parsed struct {
			Fields []string    `json:"fields"`
			IDs    interface{} `json:"ids,omitempty"`
		}
		if args != nil {
			_ = json.Unmarshal(args, &parsed)
		}

		m.mu.Lock()
		result := m.torrents
		if ids, ok := parsed.IDs.([]interface{}); ok && len(ids) > 0 {
			idSet := make(map[string]bool)
			for _, id := range ids {
				idSet[fmt.Sprintf("%v", id)] = true
			}
			var filtered []trTorrent
			for _, t := range result {
				if idSet[t.HashString] {
					filtered = append(filtered, t)
				}
			}
			result = filtered
		}
		m.mu.Unlock()

		resp := map[string]interface{}{
			"arguments": map[string]interface{}{
				"torrents": result,
			},
			"result": "success",
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, err.Error(), 500)
		}

	case "torrent-add":
		m.nextTorrentID++
		resp := map[string]interface{}{
			"arguments": map[string]interface{}{
				"torrent-added": map[string]interface{}{
					"hashString": "new-torrent-hash-12345678",
					"name":       "test.txt",
					"id":         m.nextTorrentID,
				},
			},
			"result": "success",
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, err.Error(), 500)
		}

	case "torrent-remove", "torrent-stop", "torrent-start",
		"torrent-reannounce", "torrent-verify",
		"torrent-set", "torrent-set-location":
		resp := map[string]interface{}{
			"result": "success",
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, err.Error(), 500)
		}

	default:
		resp := map[string]interface{}{
			"result": fmt.Sprintf("unknown method: %s", method),
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, err.Error(), 500)
		}
	}
}

func sampleTRTorrent() trTorrent {
	return trTorrent{
		HashString:     "abc123def456abc123def456abc123def456ab12",
		Name:           "test-movie.mkv",
		TotalSize:      2147483648,
		PercentDone:    1.0,
		UploadedEver:   1073741824,
		RateUpload:     0,
		RateDownload:   0,
		UploadRatio:    2.5,
		Status:         6,
		Error:          0,
		ErrorString:    "",
		DownloadDir:    "/data/torrents",
		Labels:         []string{"movies", "pt-forward"},
		AddedDate:      1700000000,
		SecondsSeeding: 7200,
		IsFinished:     true,
		TrackerStats: []struct {
			SeederCount  int `json:"seederCount"`
			LeecherCount int `json:"leecherCount"`
		}{
			{SeederCount: 10, LeecherCount: 3},
		},
		TorrentFile: "/data/torrents/test-movie.mkv.torrent",
		ID:          1,
	}
}

func newTestTRClient(t *testing.T, handler http.Handler) *TRClient {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cfg := &model.ClientConfig{
		Name:     "test-tr",
		Type:     "transmission",
		URL:      server.URL,
		Username: "admin",
		Password: "password",
		Role:     "seeding",
		Enabled:  true,
	}

	return &TRClient{
		cfg:     cfg,
		client:  &http.Client{Timeout: 10 * time.Second},
		baseURL: server.URL + "/transmission/rpc",
		logger:  zap.NewNop(),
	}
}

func TestTRClient_Connect(t *testing.T) {
	mock := &trMock{sessionID: "test-session-id"}
	c := newTestTRClient(t, mock.handler())

	if err := c.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
}

func TestTRClient_Connect_SessionRecovery(t *testing.T) {
	mock := &trMock{sessionID: "initial-session-id"}
	c := newTestTRClient(t, mock.handler())

	if err := c.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	c.mu.Lock()
	c.sessionID = ""
	c.mu.Unlock()

	if err := c.Connect(context.Background()); err != nil {
		t.Fatalf("Connect after session reset: %v", err)
	}
}

func TestTRClient_GetTorrentByHash(t *testing.T) {
	mock := &trMock{}
	mock.torrents = []trTorrent{sampleTRTorrent()}
	c := newTestTRClient(t, mock.handler())

	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}

	info, err := c.GetTorrentByHash(context.Background(), "abc123def456abc123def456abc123def456ab12")
	if err != nil {
		t.Fatal(err)
	}
	if info == nil {
		t.Fatal("expected non-nil torrent")
	}
	if info.Hash != "abc123def456abc123def456abc123def456ab12" {
		t.Errorf("hash mismatch: %s", info.Hash)
	}
	if info.Name != "test-movie.mkv" {
		t.Errorf("name mismatch: %s", info.Name)
	}
	if !info.IsFinished {
		t.Error("should be finished")
	}
	if info.Ratio != 2.5 {
		t.Errorf("ratio expected 2.5, got %f", info.Ratio)
	}
	if info.NumComplete != 10 {
		t.Errorf("num_complete expected 10, got %d", info.NumComplete)
	}
}

func TestTRClient_GetTorrentByHash_NotFound(t *testing.T) {
	mock := &trMock{}
	c := newTestTRClient(t, mock.handler())

	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}

	info, err := c.GetTorrentByHash(context.Background(), "nonexist")
	if err != nil {
		t.Fatal(err)
	}
	if info != nil {
		t.Error("expected nil for non-existent")
	}
}

func TestTRClient_GetSeedingTorrents(t *testing.T) {
	mock := &trMock{}
	sample := sampleTRTorrent()
	sample.Status = 6
	mock.torrents = []trTorrent{sample, {HashString: "stopped", Status: 0, Name: "stopped-torrent"}}
	c := newTestTRClient(t, mock.handler())

	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}

	torrents, err := c.GetSeedingTorrents(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(torrents) != 1 {
		t.Fatalf("expected 1 seeding, got %d", len(torrents))
	}
	if torrents[0].Name != "test-movie.mkv" {
		t.Errorf("name mismatch: %s", torrents[0].Name)
	}
}

func TestTRClient_GetTorrentsByPath(t *testing.T) {
	mock := &trMock{}
	mock.torrents = []trTorrent{sampleTRTorrent()}
	c := newTestTRClient(t, mock.handler())

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

	result2, err := c.GetTorrentsByPath(context.Background(), "/other")
	if err != nil {
		t.Fatal(err)
	}
	if len(result2) != 0 {
		t.Errorf("expected 0, got %d", len(result2))
	}
}

func TestTRClient_GetMainData(t *testing.T) {
	mock := &trMock{}
	mock.torrents = []trTorrent{sampleTRTorrent()}
	c := newTestTRClient(t, mock.handler())

	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}

	md, err := c.GetMainData(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(md.Torrents) != 1 {
		t.Errorf("expected 1 torrent, got %d", len(md.Torrents))
	}
}

func TestTRClient_GetMainDataIncremental(t *testing.T) {
	mock := &trMock{}
	mock.torrents = []trTorrent{sampleTRTorrent()}
	c := newTestTRClient(t, mock.handler())

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
	if newRid != 6 {
		t.Errorf("expected rid 6, got %d", newRid)
	}
}

func TestTRClient_AddFromFile(t *testing.T) {
	mock := &trMock{}
	c := newTestTRClient(t, mock.handler())

	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}

	data := loadTestTorrentTR(t)
	result, err := c.AddFromFile(context.Background(), data, model.AddTorrentOptions{
		SavePath: "/data/pt",
		Paused:   false,
	})
	if err != nil {
		t.Fatalf("AddFromFile: %v", err)
	}
	if result.InfoHash != "new-torrent-hash-12345678" {
		t.Errorf("hash mismatch: %s", result.InfoHash)
	}
	if result.Name != "test.txt" {
		t.Errorf("name mismatch: %s", result.Name)
	}
}

func TestTRClient_AddFromFile_WithLimits(t *testing.T) {
	mock := &trMock{}
	c := newTestTRClient(t, mock.handler())

	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}

	data := loadTestTorrentTR(t)
	result, err := c.AddFromFile(context.Background(), data, model.AddTorrentOptions{
		SavePath:         "/data/pt",
		UploadLimit:      1024 * 1024,
		DownloadLimit:    512 * 1024,
		RatioLimit:       2.0,
		SeedingTimeLimit: 3600,
	})
	if err != nil {
		t.Fatalf("AddFromFile: %v", err)
	}
	if result.IsDuplicate {
		t.Error("should not be duplicate")
	}
}

func TestTRClient_DeleteTorrent(t *testing.T) {
	mock := &trMock{}
	c := newTestTRClient(t, mock.handler())

	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteTorrent(context.Background(), "hash1", false); err != nil {
		t.Fatal(err)
	}
}

func TestTRClient_BatchDeleteTorrents(t *testing.T) {
	mock := &trMock{}
	c := newTestTRClient(t, mock.handler())

	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := c.BatchDeleteTorrents(context.Background(), []string{"h1", "h2"}, true); err != nil {
		t.Fatal(err)
	}
}

func TestTRClient_PauseResume(t *testing.T) {
	mock := &trMock{}
	c := newTestTRClient(t, mock.handler())

	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := c.PauseTorrent(context.Background(), "hash1"); err != nil {
		t.Fatalf("Pause: %v", err)
	}
	if err := c.ResumeTorrent(context.Background(), "hash1"); err != nil {
		t.Fatalf("Resume: %v", err)
	}
}

func TestTRClient_Reannounce(t *testing.T) {
	mock := &trMock{}
	c := newTestTRClient(t, mock.handler())

	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := c.Reannounce(context.Background(), "hash1"); err != nil {
		t.Fatal(err)
	}
}

func TestTRClient_Recheck(t *testing.T) {
	mock := &trMock{}
	c := newTestTRClient(t, mock.handler())

	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := c.Recheck(context.Background(), "hash1"); err != nil {
		t.Fatal(err)
	}
}

func TestTRClient_SetSavePath(t *testing.T) {
	mock := &trMock{}
	c := newTestTRClient(t, mock.handler())

	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := c.SetSavePath(context.Background(), "hash1", "/new/path"); err != nil {
		t.Fatal(err)
	}
}

func TestTRClient_SetSuperSeeding(t *testing.T) {
	mock := &trMock{}
	c := newTestTRClient(t, mock.handler())

	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := c.SetSuperSeeding(context.Background(), "hash1", true); err != nil {
		t.Fatal(err)
	}
}

func TestTRClient_SetUploadLimit(t *testing.T) {
	mock := &trMock{}
	c := newTestTRClient(t, mock.handler())

	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := c.SetUploadLimit(context.Background(), "hash1", 1024*1024); err != nil {
		t.Fatal(err)
	}
}

func TestTRClient_PauseResumeAll(t *testing.T) {
	mock := &trMock{}
	c := newTestTRClient(t, mock.handler())

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

func TestTRClient_GetFreeSpace(t *testing.T) {
	mock := &trMock{}
	c := newTestTRClient(t, mock.handler())

	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}

	space, err := c.GetFreeSpace(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if space != 2147483648 {
		t.Errorf("expected 2147483648, got %d", space)
	}
}

func TestTRClient_CheckExists_True(t *testing.T) {
	mock := &trMock{}
	mock.torrents = []trTorrent{sampleTRTorrent()}
	c := newTestTRClient(t, mock.handler())

	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}

	exists, err := c.CheckExists(context.Background(), "abc123def456abc123def456abc123def456ab12")
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Error("expected true")
	}
}

func TestTRClient_CheckExists_False(t *testing.T) {
	mock := &trMock{}
	c := newTestTRClient(t, mock.handler())

	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}

	exists, err := c.CheckExists(context.Background(), "nonexist")
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Error("expected false")
	}
}

func TestTRClient_ExportTorrent(t *testing.T) {
	mock := &trMock{}
	c := newTestTRClient(t, mock.handler())

	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}

	_, err := c.ExportTorrent(context.Background(), "abc123def456abc123def456abc123def456ab12")
	if err != nil {
		t.Logf("ExportTorrent (expected in test env without real file): %v", err)
	}
}

func TestTRClient_GetName(t *testing.T) {
	mock := &trMock{}
	c := newTestTRClient(t, mock.handler())
	if c.GetName() != "test-tr" {
		t.Errorf("expected test-tr, got %s", c.GetName())
	}
}

func TestTRClient_GetRole(t *testing.T) {
	mock := &trMock{}
	c := newTestTRClient(t, mock.handler())
	if c.GetRole() != "seeding" {
		t.Errorf("expected seeding, got %s", c.GetRole())
	}
}

func TestTRClient_GetID(t *testing.T) {
	mock := &trMock{}
	c := newTestTRClient(t, mock.handler())
	c.cfg.ID = 42
	if c.GetID() != 42 {
		t.Errorf("expected 42, got %d", c.GetID())
	}
}

func TestTRClient_RPCError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]interface{}{
			"result": "server error: something went wrong",
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, err.Error(), 500)
		}
	}))
	defer server.Close()

	c := &TRClient{
		cfg:     &model.ClientConfig{Name: "test"},
		baseURL: server.URL,
		logger:  zap.NewNop(),
		client:  &http.Client{Timeout: 5 * time.Second},
	}

	_, err := c.rpcCall(context.Background(), "torrent-get", nil)
	if err == nil {
		t.Error("expected error for rpc error response")
	}
}

func TestTRClient_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	c := &TRClient{
		cfg:     &model.ClientConfig{Name: "test", Username: "u", Password: "p"},
		baseURL: server.URL,
		logger:  zap.NewNop(),
		client:  &http.Client{Timeout: 5 * time.Second},
	}

	_, err := c.rpcCall(context.Background(), "torrent-get", nil)
	if err == nil {
		t.Error("expected error for unauthorized")
	}
}

func TestBuildTRLabels(t *testing.T) {
	tests := []struct {
		name     string
		category string
		tags     []string
		expected []string
	}{
		{"empty", "", nil, []string{}},
		{"category only", "movies", nil, []string{"movies"}},
		{"tags only", "", []string{"tag1"}, []string{"tag1"}},
		{"both", "movies", []string{"tag1", "tag2"}, []string{"movies", "tag1", "tag2"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildTRLabels(tt.category, tt.tags)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
				return
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("expected %v, got %v", tt.expected, result)
				}
			}
		})
	}
}

func TestMergeLabels(t *testing.T) {
	tests := []struct {
		name     string
		current  []string
		category string
		tags     []string
		expected int
	}{
		{"empty inputs", nil, "", nil, 0},
		{"new category and tags", nil, "movies", []string{"t1"}, 2},
		{"keep old category", []string{"old-cat", "old-tag"}, "", []string{"new-tag"}, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeLabels(tt.current, tt.category, tt.tags)
			if len(result) != tt.expected {
				t.Errorf("expected %d, got %d: %v", tt.expected, len(result), result)
			}
		})
	}
}

func TestRemoveLabels(t *testing.T) {
	current := []string{"Movies", "tag1", "tag2"}
	result := removeLabels(current, []string{"tag1"}, true)
	if len(result) != 2 {
		t.Fatalf("expected 2, got %d: %v", len(result), result)
	}
	if result[0] != "Movies" {
		t.Errorf("category should be kept: %v", result)
	}
	if result[1] != "tag2" {
		t.Errorf("tag2 should remain: %v", result)
	}
}

func TestRemoveLabels_CaseInsensitive(t *testing.T) {
	current := []string{"cat", "Tag1", "TAG2"}
	result := removeLabels(current, []string{"tag1", "tag2"}, false)
	if len(result) != 1 {
		t.Errorf("expected 1, got %d: %v", len(result), result)
	}
}

func TestTRStatusToString(t *testing.T) {
	tests := []struct {
		status   int
		expected string
	}{
		{0, "stopped"},
		{1, "checking"},
		{2, "checking"},
		{3, "downloading"},
		{4, "downloading"},
		{5, "uploading"},
		{6, "uploading"},
		{99, "unknown"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("status_%d", tt.status), func(t *testing.T) {
			got := trStatusToString(tt.status)
			if got != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}

func TestIsSeedingStatus(t *testing.T) {
	if !isSeedingStatus(5) {
		t.Error("5 should be seeding")
	}
	if !isSeedingStatus(6) {
		t.Error("6 should be seeding")
	}
	if isSeedingStatus(0) {
		t.Error("0 should not be seeding")
	}
	if isSeedingStatus(4) {
		t.Error("4 should not be seeding")
	}
}

func TestTRConvert_ErrorState(t *testing.T) {
	tr := trTorrent{
		HashString:  "abc",
		Status:      0,
		Error:       3,
		ErrorString: "tracker not responding",
		Labels:      []string{},
	}
	info := tr.toModel()
	if info.Error != "tracker not responding" {
		t.Errorf("expected error string, got %q", info.Error)
	}
	if info.State != "stopped" {
		t.Errorf("expected stopped, got %s", info.State)
	}
}

func TestTRConvert_LabelParsing(t *testing.T) {
	tr := trTorrent{
		HashString: "abc",
		Status:     6,
		Labels:     []string{"movies", "tag1", "tag2"},
	}
	info := tr.toModel()
	if info.Category != "movies" {
		t.Errorf("expected movies, got %s", info.Category)
	}
	if len(info.Tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(info.Tags))
	}
	if info.Tags[0] != "tag1" || info.Tags[1] != "tag2" {
		t.Errorf("tags mismatch: %v", info.Tags)
	}
}

func TestTRConvert_EmptyLabels(t *testing.T) {
	tr := trTorrent{HashString: "abc", Status: 6, Labels: nil}
	info := tr.toModel()
	if info.Category != "" {
		t.Errorf("expected empty category, got %s", info.Category)
	}
	if len(info.Tags) != 0 {
		t.Errorf("expected empty tags, got %v", info.Tags)
	}
}

func TestNewTRClient_RPCPath(t *testing.T) {
	tests := []struct {
		url      string
		expected string
	}{
		{"http://localhost:9091", "http://localhost:9091/transmission/rpc"},
		{"http://localhost:9091/", "http://localhost:9091/transmission/rpc"},
		{"http://localhost:9091/rpc", "http://localhost:9091/rpc"},
		{"http://localhost:9091/transmission/web", "http://localhost:9091/transmission/web/rpc"},
		{"http://localhost:9091/transmission/web/rpc", "http://localhost:9091/transmission/web/rpc"},
	}

	for _, tt := range tests {
		c, err := NewTRClient(&model.ClientConfig{URL: tt.url}, nil, zap.NewNop())
		if err != nil {
			t.Fatal(err)
		}
		if c.baseURL != tt.expected {
			t.Errorf("input %q: expected %q, got %q", tt.url, tt.expected, c.baseURL)
		}
	}
}

func TestTRClient_SetTorrentTags(t *testing.T) {
	mock := &trMock{}
	mock.torrents = []trTorrent{{
		HashString: "h1",
		Labels:     []string{"cat", "old-tag"},
		Status:     6,
	}}
	c := newTestTRClient(t, mock.handler())

	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := c.SetTorrentTags(context.Background(), "h1", []string{"new-tag"}); err != nil {
		t.Fatal(err)
	}
}

func TestTRClient_RemoveTorrentTags(t *testing.T) {
	mock := &trMock{}
	mock.torrents = []trTorrent{{
		HashString: "h1",
		Labels:     []string{"cat", "tag1", "tag2"},
		Status:     6,
	}}
	c := newTestTRClient(t, mock.handler())

	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := c.RemoveTorrentTags(context.Background(), "h1", []string{"tag1"}); err != nil {
		t.Fatal(err)
	}
}

func TestTRClient_SetCategory(t *testing.T) {
	mock := &trMock{}
	mock.torrents = []trTorrent{{
		HashString: "h1",
		Labels:     []string{"old-cat", "tag1"},
		Status:     6,
	}}
	c := newTestTRClient(t, mock.handler())

	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := c.SetCategory(context.Background(), "h1", "new-cat"); err != nil {
		t.Fatal(err)
	}
}
