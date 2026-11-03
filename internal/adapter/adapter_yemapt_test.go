package adapter

import (
	"fmt"
	"io"
	"strings"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

func TestYemaptAdapter_DownloadTorrent_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/torrent/generateDownloadKey":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"success":true,"data":"test-key"}`))
		case r.URL.Path == "/api/torrent/download1":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewYemaptAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, Cookie: "sid=test"}
	_, err := a.DownloadTorrent(context.Background(), config, "1")
	if err == nil {
		t.Fatal("expected error for 404")
	}
	var appErr *model.AppError
	if !errors.As(err, &appErr) || appErr.Code != ErrAdapterNotFound {
		t.Errorf("expected ErrAdapterNotFound, got %v", err)
	}
}

// §59.459: 野马 uploadViaOpenAPI 组装锚（multipart 字段/裸 AuthKey/
// 编号提取/必填兜底/regionList 重复字段）。
func TestYemaptAdapter_UploadViaOpenAPI(t *testing.T) {
	var gotAuth, gotCT string
	var gotForm map[string][]string
	var gotFile []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotCT = r.Header.Get("Content-Type")
		gotFile, _, _ = readUploadFile(r)
		gotForm = r.MultipartForm.Value
		fmt.Fprintf(w, `{"success":true,"showType":0,"data":16845}`)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewYemaptAdapter(doer, zap.NewNop())
	resp, err := a.UploadTorrent(context.Background(),
		&model.SiteConfig{Domain: srv.URL, APIKey: "yemakey"},
		&model.PublishRequest{
			TorrentData: []byte("d4:infod6:lengthi0eee"),
			Title:       "Test Movie 2026 1080p BluRay x264-GRP",
			Description: "简介[img]https://x.example/1.jpg[/img]尾",
			IMDbLink:    "https://www.imdb.com/title/tt0111161/",
			DoubanLink:  "https://movie.douban.com/subject/1292052/",
			Anonymous:   true,
			FormFields: map[string]string{
				"categoryId": "4", "medium": "2", "standard": "4",
				"codec": "1", "audiocodec": "1", "team": "7",
			},
			// §59.459 回归审核修正：tags 走 TagArrayFields 通道
			TagArrayFields: []model.TagKV{
				{Key: "tagList", Value: "6"},
				{Key: "tagList", Value: "10"},
			},
			Screenshots: []string{"https://img.example/s1.png", "https://img.example/s2.png"},
			ExtraFields: map[string]string{
				"yema_regions":    `["6","4"]`,
				"yema_poster":     "https://img.example/poster.jpg",
				"yema_translated": "测试电影",
			},
		})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Success || resp.TorrentID != "16845" {
		t.Errorf("resp: %+v", resp)
	}
	if gotAuth != "yemakey" {
		t.Errorf("auth 裸 key: %q", gotAuth)
	}
	if !strings.HasPrefix(gotCT, "multipart/form-data") {
		t.Errorf("CT: %q", gotCT)
	}
	if string(gotFile) != "d4:infod6:lengthi0eee" {
		t.Errorf("file: %q", gotFile)
	}
	// 必填+映射
	checks := map[string]string{
		"showName": "Test Movie 2026 1080p BluRay x264-GRP",
		"shortDesc": "测试电影", // ⑧ 副标题空→译名兜底
		"categoryId":              "4", "uploadUserAnonymous": "y",
		"imdb": "0111161", "douban": "1292052", // ⑥ 去 tt/纯数字
		"picture": "https://img.example/poster.jpg",
		"medium": "2", "standard": "4", "codec": "1", "team": "7",
		"hrPunishEnable": "false",
	}
	for k, want := range checks {
		if got := gotForm[k]; len(got) == 0 || got[0] != want {
			t.Errorf("field %s = %v want %q", k, got, want)
		}
	}
	// regionList/tagList/screenshotList 同名重复字段
	if len(gotForm["regionList"]) != 2 || gotForm["regionList"][0] != "6" {
		t.Errorf("regionList: %v", gotForm["regionList"])
	}
	if len(gotForm["tagList"]) != 2 {
		t.Errorf("tagList: %v", gotForm["tagList"])
	}
	if len(gotForm["screenshotList"]) != 2 {
		t.Errorf("screenshotList: %v", gotForm["screenshotList"])
	}
	// ⑤ 描述禁图
	if d := gotForm["longDesc"]; len(d) > 0 && (strings.Contains(d[0], "x.example/1.jpg") || strings.Contains(d[0], "[img")) {
		t.Errorf("longDesc 图未剥除: %q", d[0])
	}
}

func readUploadFile(r *http.Request) ([]byte, string, error) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		return nil, "", err
	}
	f, hdr, err := r.FormFile("file")
	if err != nil {
		return nil, "", err
	}
	defer f.Close()
	b, err := io.ReadAll(f)
	return b, hdr.Filename, err
}
