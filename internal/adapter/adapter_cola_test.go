package adapter

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

// jsonBodyMode 从 search 请求体提取 mode 值序列
func jsonBodyMode(r *http.Request, out *[]string) error {
	var payload struct {
		Mode string `json:"mode"`
	}
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, &payload); err != nil {
		return err
	}
	*out = append(*out, payload.Mode)
	return nil
}

// §59.460 附五（刀2 测试债补还）: ColaAdapter 三个 Web 面特化解析锚
// （本 session 此前全部生产探针验证零单测——借拆分补齐）。

func newColaTestAdapter(t *testing.T, handler http.HandlerFunc) (*ColaAdapter, *model.SiteConfig) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	doer := NewHTTPDoer()
	a := NewColaAdapter(doer, nil)
	cfg := &model.SiteConfig{
		Domain:  srv.URL,
		BaseURL: srv.URL,
		APIKey:  "rcapi_test",
		Cookie:  "reelcore_session=test",
	}
	return a, cfg
}

// promotion 分数→档位映射锚（可乐全站免费站免费语义在 promotion）
func TestColaDiscountPromotionMapping(t *testing.T) {
	cases := []struct {
		name string
		body string
		want model.DiscountLevel
	}{
		{"免费 num=0", `{"promotion":{"download":{"num":0,"den":1}}}`, model.DiscountFree},
		{"50% 比例", `{"promotion":{"download":{"num":1,"den":2}}}`, model.DiscountPercent50},
		{"70% 比例", `{"promotion":{"download":{"num":7,"den":10}}}`, model.DiscountPercent70},
		{"未知比例回 NONE", `{"promotion":{"download":{"num":3,"den":4}}}`, model.DiscountNone},
		{"den=0 防御", `{"promotion":{"download":{"num":0,"den":0}}}`, model.DiscountNone},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a, cfg := newColaTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
				// Web 面判定锚：必须 cookie 通道（无 x-api-key 头）
				if r.Header.Get("Cookie") == "" {
					t.Error("Web 面请求缺 cookie")
				}
				_, _ = w.Write([]byte(c.body))
			})
			dr, err := a.DetectDiscount(context.Background(), cfg, "13115")
			if err != nil {
				t.Fatalf("err=%v", err)
			}
			if dr.Level != c.want {
				t.Errorf("level=%s want=%s", dr.Level, c.want)
			}
		})
	}
}

// Web 面 detail 字段映射锚（release_name/size_bytes 字符串数字/技术规格）
func TestColaDetailWebFields(t *testing.T) {
	a, cfg := newColaTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/torrents/13115" {
			t.Errorf("path=%s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
			"release_name":"Bluey.S02.2020.1080p.DSNP.WEB-DL.H.264-FROGWeb",
			"subtitle":"Bluey Season2",
			"size_bytes":"11632068466",
			"info_hash":"4A4A1F493D93372B62AE8BBFF7B493B677E6CEE3",
			"video_codec":"AVC / H.264",
			"audio":["E-AC-3 5.1"],
			"resolution":"1080p",
			"source":"WEB-DL",
			"release_group":"FROGWeb",
			"media_info":"General\nComplete name: x.mkv",
			"screenshots":["https://img.example/1.jpg","https://img.example/2.jpg"],
			"tags":["完结","国语"]
		}`))
	})
	d, err := a.GetTorrentDetail(context.Background(), cfg, "13115")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if d.Title != "Bluey.S02.2020.1080p.DSNP.WEB-DL.H.264-FROGWeb" {
		t.Errorf("title=%q", d.Title)
	}
	if d.Size != 11632068466 {
		t.Errorf("size=%d（size_bytes 字符串数字解析）", d.Size)
	}
	if d.InfoHash != "4a4a1f493d93372b62ae8bbff7b493b677e6cee3" {
		t.Errorf("infoHash=%q（期望小写归一）", d.InfoHash)
	}
	if d.Codec != "AVC / H.264" || d.AudioCodec != "E-AC-3 5.1" || d.Resolution != "1080p" || d.Source != "WEB-DL" {
		t.Errorf("规格字段: codec=%q audio=%q res=%q source=%q", d.Codec, d.AudioCodec, d.Resolution, d.Source)
	}
	if d.ReleaseGroup != "FROGWeb" {
		t.Errorf("group=%q", d.ReleaseGroup)
	}
	if len(d.Screenshots) != 2 || len(d.Tags) != 2 {
		t.Errorf("screenshots=%d tags=%d", len(d.Screenshots), len(d.Tags))
	}
}

// myPeerStatus 简名端点解析锚（{code,data:{seeder:N}} 仅数无 size）
func TestColaSeedingViaPeerStatus(t *testing.T) {
	a, cfg := newColaTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tracker/myPeerStatus" {
			t.Errorf("path=%s（期望简名端点）", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"code":"0","data":{"seeder":1}}`))
	})
	count, ok := a.seedingViaPeerStatus(context.Background(), cfg)
	if !ok || count != 1 {
		t.Errorf("ok=%v count=%d（期望 1）", ok, count)
	}
}

// Search 单轮锚（可乐无成人区语义——请求 mode 断言）
func TestColaSearchSingleMode(t *testing.T) {
	modes := []string{}
	a, cfg := newColaTestAdapter(t, func(w http.ResponseWriter, r *http.Request) {
		if err := jsonBodyMode(r, &modes); err != nil {
			t.Errorf("body: %v", err)
		}
		_, _ = w.Write([]byte(`{"code":0,"data":{"data":[{"id":13115,"name":"Bluey","size":11632068466,"status":{"seeders":3,"leechers":0,"discount":"FREE"}}]}}`))
	})
	res, err := a.SearchTorrents(context.Background(), cfg, "Bluey", nil)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(modes) != 1 || modes[0] != "normal" {
		t.Errorf("modes=%v（期望仅 normal——单轮）", modes)
	}
	if len(res) != 1 || res[0].TorrentID != "13115" {
		t.Errorf("res=%v", res)
	}
	if res[0].Adult {
		t.Error("Adult 标记不应存在")
	}
}
