package adapter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

// §59.159: NP 家族重复上传识别——302 details.php?id=N&existed=1 → IsExisting。
func TestGenericAdapter_UploadTorrent_ExistingRedirect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/takeupload.php" {
			// NP 重复上传标准行为：302 已存在种详情页 + existed=1
			http.Redirect(w, r, "/details.php?id=4242&existed=1", http.StatusFound)
			return
		}
		// details 页（重定向落点——body 含 id，旧逻辑会误判新发布）
		_, _ = w.Write([]byte(`<html><body>详情页 <a href="details.php?id=4242">种子</a></body></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("nexusphp", doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=abc"}
	config.Paths.Upload = "/takeupload.php"

	resp, err := a.UploadTorrent(context.Background(), config, &model.PublishRequest{
		TorrentData: []byte("d4:infod...e"),
		Title:       "Test.2026",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Success || !resp.IsExisting {
		t.Errorf("应识别为 Success+IsExisting（站上已有）, got Success=%v IsExisting=%v", resp.Success, resp.IsExisting)
	}
	if resp.ExistingID != "4242" {
		t.Errorf("ExistingID 应为 4242（重定向捕获）, got %q", resp.ExistingID)
	}
	if resp.TorrentID != "" {
		t.Errorf("已存在响应不应误填新发布 TorrentID, got %q", resp.TorrentID)
	}
}

// 旧版 stderr 200 页形态：文本"该种子已存在"含详情链接。
func TestGenericAdapter_UploadTorrent_ExistingText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body><p class="stderr">该种子已存在：<a href="details.php?id=777">查看</a></p></body></html>`))
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewGenericAdapter("nexusphp", doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=abc"}
	config.Paths.Upload = "/takeupload.php"

	resp, err := a.UploadTorrent(context.Background(), config, &model.PublishRequest{
		TorrentData: []byte("d4:infod...e"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.IsExisting || resp.ExistingID != "777" {
		t.Errorf("文本形态应识别已存在+ID 777, got %+v", resp)
	}
}
