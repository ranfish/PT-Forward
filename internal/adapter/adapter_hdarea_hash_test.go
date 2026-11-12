package adapter

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"go.uber.org/zap"
)

// hdareaHashShelf hashes 端点货架（刀3）。
type hdareaHashShelf struct{ response string }

func (s *hdareaHashShelf) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: 200, Status: "200 OK", Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(s.response)),
		Request:    req,
	}, nil
}

const hashMatchResp = `{"success":true,"code":0,"message":"OK","data":{"count":1,"items":[{"info_hash":"302426f332ccf0e456dda74186b8786b467e2f4f","id":460261,"name":"Sunshine Through the Rain 2026","size":69204663901,"seeders":2,"download_url":"https://hdarea.club/download.php?id=460261&dl_token=SANITIZED"}]}}`

func newHDAreaHashAdapter() *HDAreaAdapter {
	return NewHDAreaAdapter(&HTTPDoer{Client: &http.Client{Transport: &hdareaHashShelf{response: hashMatchResp}}}, zap.NewNop())
}

// TestHDAreaBatchMatch 单批匹配→结果映射。
func TestHDAreaBatchMatch(t *testing.T) {
	a := newHDAreaHashAdapter()
	results, err := a.BatchMatchHashes(context.Background(), hdareaTestConfig(),
		[]string{"302426f332ccf0e456dda74186b8786b467e2f4f"})
	if err != nil {
		t.Fatalf("BatchMatchHashes: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("结果数=%d want 1", len(results))
	}
	r := results[0]
	if r.TorrentID != "460261" {
		t.Errorf("TorrentID=%q want 460261", r.TorrentID)
	}
	if r.Size != 69204663901 {
		t.Errorf("Size=%d", r.Size)
	}
	if !strings.Contains(r.DownloadURL, "download.php") {
		t.Errorf("DownloadURL=%q", r.DownloadURL)
	}
}

// TestHDAreaBatchMatchEmpty 空入参→nil 零调用。
func TestHDAreaBatchMatchEmpty(t *testing.T) {
	a := newHDAreaHashAdapter()
	results, err := a.BatchMatchHashes(context.Background(), hdareaTestConfig(), nil)
	if err != nil {
		t.Fatalf("BatchMatchHashes: %v", err)
	}
	if results != nil {
		t.Errorf("空入参应返回 nil: %v", results)
	}
}

// TestHDAreaHashMatcher 接口断言。
func TestHDAreaHashMatcher(t *testing.T) {
	var _ HashMatcher = NewHDAreaAdapter(nil, nil)
}
