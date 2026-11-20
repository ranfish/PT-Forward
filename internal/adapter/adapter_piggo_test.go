package adapter

// §59.540 锚：PiggoAdapter 契约核心——信封/公告门/搜索映射/发布 422 dedup/
// multipart 形态/单位解析。夹具来源：piggo.md 2026-11-16 九端点实测真实形态
// （§59.1339 教训：验证样例必须真实数据源）。

import (
	"fmt"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

func newPiggoTestAdapter(handler http.HandlerFunc) (*PiggoAdapter, *httptest.Server) {
	srv := httptest.NewServer(handler)
	a := NewPiggoAdapter(&HTTPDoer{Client: srv.Client()}, zap.NewNop())
	return a, srv
}

func piggoCfg(srvURL string) *model.SiteConfig {
	return &model.SiteConfig{Domain: "piggo.me", BaseURL: srvURL, APIKey: "testkey"}
}

// ① 信封+搜索映射：条目字段→SeedingSearchResult（tid/title/size/seeders）。
func TestPiggoSearch(t *testing.T) {
	a, srv := newPiggoTestAdapter(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("want POST, got %s", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer testkey" {
			t.Errorf("Bearer 缺失: %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"data": map[string]any{
				"items": []map[string]any{{
					"id": 57277, "name": "变形金刚 2023 2160P", "size": 1073741824,
					"seeders": 16, "leechers": 0, "page_url": "https://piggo.me/detail/57277",
				}},
				"total": 1,
			},
			"msg": "ok",
		})
	})
	defer srv.Close()
	res, err := a.SearchTorrents(context.Background(), piggoCfg(srv.URL), "变形金刚", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].TorrentID != "57277" || res[0].Seeders != 16 || res[0].Size != 1073741824 {
		t.Fatalf("映射错: %+v", res[0])
	}
}

// ② 公告门特殊形态：403+code=0+data.code=notice_unread → 明确文案。
func TestPiggoNoticeGate(t *testing.T) {
	a, srv := newPiggoTestAdapter(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"data": map[string]any{"code": "notice_unread", "unread_notices": 1},
			"msg":  "请先阅读公告",
		})
	})
	defer srv.Close()
	_, err := a.SearchTorrents(context.Background(), piggoCfg(srv.URL), "x", nil)
	if err == nil || !strings.Contains(err.Error(), "必读公告") {
		t.Fatalf("公告门文案缺失: %v", err)
	}
}

// ③ 422 dedup：errors.file 含「已存在」→ IsExisting 终态（duplicate 语义）。
func TestPiggoPublishDedup(t *testing.T) {
	a, srv := newPiggoTestAdapter(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 422,
			"data": map[string]any{
				"message":     "种子已存在 #57218",
				"errors":      map[string][]string{"file": {"种子已存在 #57218"}},
				"torrent_id":  57218,
			},
		})
	})
	defer srv.Close()
	resp, err := a.UploadTorrent(context.Background(), piggoCfg(srv.URL), &model.PublishRequest{
		Title: "t", Subtitle: "s", Description: "d", MediaInfo: "mi",
		TorrentData: []byte("xx"),
		FormFields:  map[string]string{"section_id": "1", "category_ids": "[3,5]"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.IsExisting || resp.ExistingID != "57218" {
		t.Fatalf("dedup 终态错: %+v", resp)
	}
}

// ④ 201 成功：成功判定=StatusCreated（非 NP 302 习惯）。
func TestPiggoPublish201(t *testing.T) {
	a, srv := newPiggoTestAdapter(func(w http.ResponseWriter, r *http.Request) {
		// multipart 内容校验：必填字段在场
		if !strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
			t.Errorf("非 multipart: %s", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"data": map[string]any{"id": 57300, "name": "t", "page_url": "https://piggo.me/detail/57300"},
		})
	})
	defer srv.Close()
	resp, err := a.UploadTorrent(context.Background(), piggoCfg(srv.URL), &model.PublishRequest{
		Title: "t", Subtitle: "s", Description: "d", MediaInfo: "mi",
		TorrentData: []byte("xx"), Anonymous: true,
		FormFields: map[string]string{"section_id": "1", "category_ids": "[3,5]"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Success || resp.TorrentID != "57300" || resp.IDSource != "api_201" {
		t.Fatalf("201 终态错: %+v", resp)
	}
}

// ⑤ 缺 section_id fail-fast（配置异常静默不发 §59.289 先例）。
func TestPiggoPublishMissingSection(t *testing.T) {
	a, srv := newPiggoTestAdapter(func(w http.ResponseWriter, r *http.Request) {
		t.Error("不应发请求")
	})
	defer srv.Close()
	_, err := a.UploadTorrent(context.Background(), piggoCfg(srv.URL), &model.PublishRequest{
		Title: "t", FormFields: map[string]string{},
	})
	if err == nil || !strings.Contains(err.Error(), "section_id") {
		t.Fatalf("fail-fast 缺失: %v", err)
	}
}

// ⑥ 人类可读体积解析："11.50 TB"→字节。
func TestParseHumanSize(t *testing.T) {
	cases := map[string]uint64{
		"1.00 GB": 1073741824, "11.50 TB": 12644383719424, "512 B": 512,
		"2 MB": 2097152,
	}
	for in, want := range cases {
		got, err := parseHumanSize(in)
		if err != nil || got != want {
			t.Errorf("%s → %d (want %d, err %v)", in, got, want, err)
		}
	}
}

// ⑦ HR 恒 false（用户定案：系统零处理）。
func TestPiggoHRFalse(t *testing.T) {
	a, srv := newPiggoTestAdapter(func(w http.ResponseWriter, r *http.Request) {})
	defer srv.Close()
	hr, err := a.DetectHR(context.Background(), piggoCfg(srv.URL), "1")
	if err != nil || hr.HasHR {
		t.Fatalf("HR 应恒 false: %+v %v", hr, err)
	}
}

// §59.541: taxonomy 八维合并锚——全维/部分维兜底/ExtraFields 优先序。
func TestPiggoTaxonomyIDs(t *testing.T) {
	// 全维显式（Ronin 形态：Blu-ray+Remux+H.265+DTS-HD MA+4K+DV+Other组+美国）
	req := &model.PublishRequest{
		FormFields: map[string]string{
			"source_id": "35", "medium_id": "28", "codec_id": "25",
			"audio_id": "21", "resolution_id": "11", "processing_id": "9",
			"team_id": "5", "region_id": "40",
		},
	}
	if got := piggoTaxonomyIDs(req); got != "[35,28,25,21,11,9,5,40]" {
		t.Errorf("全维: got %s", got)
	}
	// 空维兜底（全空→八兜底值）
	req2 := &model.PublishRequest{FormFields: map[string]string{}}
	if got := piggoTaxonomyIDs(req2); got != "[36,31,26,19,10,6,5,53]" {
		t.Errorf("全兜底: got %s", got)
	}
	// 部分维（source 显式·其余兜底）+ExtraFields 优先
	req3 := &model.PublishRequest{
		FormFields:  map[string]string{"codec_id": "24"},
		ExtraFields: map[string]string{"codec_id": "25"}, // Extra 优先
	}
	got := piggoTaxonomyIDs(req3)
	if got != "[36,31,25,19,10,6,5,53]" {
		t.Errorf("部分维+Extra优先: got %s", got)
	}
	// nil FormFields（防御）
	if got := piggoTaxonomyIDs(&model.PublishRequest{}); got != "[36,31,26,19,10,6,5,53]" {
		t.Errorf("nil 防御: got %s", got)
	}
}

// §59.541 回归审核 P1 锚: taglist applier 硬编码 Key="tagList"（系统约定）
// ——adapter 聚合双 key 收后 multipart tag_ids JSON 数组串必须在场。
// （好学 §59.1481 同型第三站·锚增标准键蒸发路径先例）
func TestPiggoPublishTaglistKey(t *testing.T) {
	var body string
	a, srv := newPiggoTestAdapter(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"code":0,"data":{"id":1}}`))
	})
	defer srv.Close()
	_, err := a.UploadTorrent(context.Background(), piggoCfg(srv.URL), &model.PublishRequest{
		Title: "t", Subtitle: "s", Description: "d", MediaInfo: "mi",
		TorrentData: []byte("xx"),
		FormFields: map[string]string{"section_id": "1", "category_ids": "[3]"},
		// taglist 模式 applier 产出形态（executor 真相源——key="tagList"）
		TagArrayFields: []model.TagKV{
			{Key: "tagList", Value: "4"}, // 中字
			{Key: "tagList", Value: "9"}, // 合集
			// §59.540 附六·二轮回归 P2: 无映射标准键直出（值集 miss 蒸发）
			{Key: "tagList", Value: "hdr10_plus"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, `"tag_ids"`+"\r\n") && !strings.Contains(body, `"tag_ids"`+"\n") {
		t.Errorf("multipart 应含 tag_ids 字段: %.200s", body)
	}
	if !strings.Contains(body, "[4,9]") {
		t.Errorf("tag_ids 应为数字数组串 [4,9]（非数字蒸发）: %.200s", body)
	}
	if strings.Contains(body, "hdr10_plus") {
		t.Errorf("无映射标准键应蒸发不进 tag_ids: %.200s", body)
	}
	// taxonomy 八维合并（全兜底形态在场）
	if !strings.Contains(body, "[36,31,26,19,10,6,5,53]") {
		t.Errorf("taxonomy_ids 空维兜底串缺失: %.300s", body)
	}
}

// §59.543: 特征辅种主通道锚——SearchByPiecesHash 传真指纹批量→map[tid]。
func TestPiggoSearchByPiecesHash(t *testing.T) {
	var batches [][]string
	a, srv := newPiggoTestAdapter(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Hashes []string `json:"hashes"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		batches = append(batches, body.Hashes)
		// 只命中其中一个 hash（tid=56292——piggo.md 真指纹实证 id）
		items := []map[string]any{}
		for _, h := range body.Hashes {
			if strings.HasPrefix(h, "aaaa") {
				items = append(items, map[string]any{"hash": h, "id": 56292, "page_url": "https://piggo.me/detail/56292"})
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"data": map[string]any{"items": items},
			"msg":  "ok",
		})
	})
	defer srv.Close()

	hashes := make([]string, 0, 101)
	for i := 0; i < 101; i++ {
		hashes = append(hashes, fmt.Sprintf("aaaa%02d%038d", i, i))
	}
	res, err := a.SearchByPiecesHash(context.Background(), piggoCfg(srv.URL), hashes)
	if err != nil {
		t.Fatal(err)
	}
	// 批边界：101 个 → 2 批（100+1）
	if len(batches) != 2 || len(batches[0]) != 100 || len(batches[1]) != 1 {
		t.Fatalf("分批错: %d 批 %v", len(batches), []int{len(batches[0]), len(batches[1])})
	}
	// mock 对所有 aaaa 前缀都回 → 101 全命中
	if len(res) != 101 {
		t.Fatalf("命中数错: %d", len(res))
	}
	for h, id := range res {
		if id != 56292 {
			t.Fatalf("tid 映射错: %s→%d", h, id)
		}
	}
}

// 空入参零请求。
func TestPiggoSearchByPiecesHash_Empty(t *testing.T) {
	a, srv := newPiggoTestAdapter(func(w http.ResponseWriter, r *http.Request) {
		t.Error("空入参不应发请求")
	})
	defer srv.Close()
	res, err := a.SearchByPiecesHash(context.Background(), piggoCfg(srv.URL), nil)
	if err != nil || len(res) != 0 {
		t.Fatalf("空入参: res=%v err=%v", res, err)
	}
}
