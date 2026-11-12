package adapter

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"go.uber.org/zap"
)

// hdareaCheckShelf check 端点货架（刀2：三态夹具——exists_active/clean/banned）。
type hdareaCheckShelf struct{ response string }

func (s *hdareaCheckShelf) RoundTrip(req *http.Request) (*http.Response, error) {
	// 回放预设响应
	body := s.response
	return &http.Response{
		StatusCode: 200, Status: "200 OK", Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}, nil
}

const checkExistsActive = `{"success":true,"code":0,"message":"OK","data":{"ok":true,"can_upload":false,"status":"exists_active","reason":"站点已存在相同活种","normalized_infohash":"302426f332ccf0e456dda74186b8786b467e2f4f","existing_torrent":{"id":460261,"name":"Sunshine Through the Rain 2026","size":69204663901,"seeders":2}}}`

const checkClean = `{"success":true,"code":0,"message":"OK","data":{"ok":true,"can_upload":true,"status":"clean","reason":"本站暂无此资源，允许上传发布。"}}`

const checkBanned = `{"success":true,"code":0,"message":"OK","data":{"ok":true,"can_upload":false,"status":"banned_duplicate","reason":"站点禁止发布的资源","normalized_infohash":"aaaabbbbccccdddd"}}`

func newHDAreaCheckAdapter(response string) *HDAreaAdapter {
	shelf := &hdareaCheckShelf{response: response}
	return NewHDAreaAdapter(&HTTPDoer{Client: &http.Client{Transport: shelf}}, zap.NewNop())
}

// TestHDAreaCheckExists exists_active 态→CanUpload=false+ExistingID。
func TestHDAreaCheckExists(t *testing.T) {
	a := newHDAreaCheckAdapter(checkExistsActive)
	td, _ := os.ReadFile("testdata/hdarea/dl.bin")
	pc, err := a.CheckBeforeUpload(context.Background(), hdareaTestConfig(), td)
	if err != nil {
		t.Fatalf("CheckBeforeUpload: %v", err)
	}
	if pc.CanUpload {
		t.Error("CanUpload 应 false（exists_active）")
	}
	if pc.ExistingID != "460261" {
		t.Errorf("ExistingID=%q want 460261", pc.ExistingID)
	}
	if pc.NormalizedHash != "302426f332ccf0e456dda74186b8786b467e2f4f" {
		t.Errorf("NormalizedHash=%q", pc.NormalizedHash)
	}
	if pc.IsBanned {
		t.Error("IsBanned 应 false（exists_active 不是 banned）")
	}
}

// TestHDAreaCheckClean clean 态→CanUpload=true。
func TestHDAreaCheckClean(t *testing.T) {
	a := newHDAreaCheckAdapter(checkClean)
	td, _ := os.ReadFile("testdata/hdarea/dl.bin")
	pc, err := a.CheckBeforeUpload(context.Background(), hdareaTestConfig(), td)
	if err != nil {
		t.Fatalf("CheckBeforeUpload: %v", err)
	}
	if !pc.CanUpload {
		t.Error("CanUpload 应 true（clean）")
	}
	if pc.ExistingID != "" {
		t.Errorf("ExistingID 应空: %q", pc.ExistingID)
	}
}

// TestHDAreaCheckBanned banned 态→IsBanned=true（E1：映射 failed 终态）。
func TestHDAreaCheckBanned(t *testing.T) {
	a := newHDAreaCheckAdapter(checkBanned)
	td, _ := os.ReadFile("testdata/hdarea/dl.bin")
	pc, err := a.CheckBeforeUpload(context.Background(), hdareaTestConfig(), td)
	if err != nil {
		t.Fatalf("CheckBeforeUpload: %v", err)
	}
	if pc.CanUpload {
		t.Error("CanUpload 应 false（banned）")
	}
	if !pc.IsBanned {
		t.Error("IsBanned 应 true（banned_duplicate 判定）")
	}
}

// TestHDAreaPreChecker 接口断言（PreChecker 接口满足性）。
func TestHDAreaPreChecker(t *testing.T) {
	var _ PreChecker = NewHDAreaAdapter(nil, nil)
}
