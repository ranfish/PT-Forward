package adapter

import (
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

func TestParseMTeamFeedParams_Defaults(t *testing.T) {
	cats, teams, pageSize, discounts := parseMTeamFeedParams("")
	if len(cats) != 0 {
		t.Errorf("expected no categories, got %v", cats)
	}
	if len(teams) != 0 {
		t.Errorf("expected no teams, got %v", teams)
	}
	if pageSize != 50 {
		t.Errorf("expected default pageSize 50, got %d", pageSize)
	}
	if !discounts["FREE"] {
		t.Errorf("expected FREE in default discounts")
	}
}

func TestParseMTeamFeedParams_WithURL(t *testing.T) {
	urlStr := "https://api.m-team.cc/api/torrent/search?categories=401,419&teams=9,44&pageSize=80&discounts=FREE,_2X_FREE"
	cats, teams, pageSize, discounts := parseMTeamFeedParams(urlStr)
	if len(cats) != 2 || cats[0] != 401 || cats[1] != 419 {
		t.Errorf("expected [401,419], got %v", cats)
	}
	if len(teams) != 2 || teams[0] != 9 || teams[1] != 44 {
		t.Errorf("expected [9,44], got %v", teams)
	}
	if pageSize != 80 {
		t.Errorf("expected pageSize 80, got %d", pageSize)
	}
	if !discounts["FREE"] || !discounts["_2X_FREE"] {
		t.Errorf("expected FREE/_2X_FREE in discounts, got %v", discounts)
	}
	if discounts["PERCENT_50"] {
		t.Errorf("expected PERCENT_50 not in custom discounts")
	}
}

func TestParseMTeamFeedParams_PageSizeClamped(t *testing.T) {
	urlStr := "https://x.com/?pageSize=999"
	_, _, pageSize, _ := parseMTeamFeedParams(urlStr)
	if pageSize != 50 {
		t.Errorf("expected pageSize fallback to 50 when > 100, got %d", pageSize)
	}
}

func TestMTeamDiscountToResult(t *testing.T) {
	cases := []struct {
		discount string
		level    model.DiscountLevel
	}{
		{"FREE", model.DiscountFree},
		{"free", model.DiscountFree},
		{"_2X_FREE", model.Discount2xFree},
		{"FREE_2XUP", model.Discount2xFree},
		{"TWOFREE", model.Discount2xFree},
		{"_2X", model.Discount2xUp},
		{"2XUP", model.Discount2xUp},
		{"_2X_PERCENT_50", model.Discount2x50},
		{"PERCENT_50", model.DiscountPercent50},
		{"PERCENT_70", model.DiscountPercent70},
		{"PERCENT_30", model.DiscountPercent30},
		{"NORMAL", model.DiscountNone},
		{"", model.DiscountNone},
		{"UNKNOWN", model.DiscountNone},
	}
	for _, c := range cases {
		dr := mTeamDiscountToResult(c.discount, "")
		if dr.Level != c.level {
			t.Errorf("discount=%q: expected %s, got %s", c.discount, c.level, dr.Level)
		}
	}
}

func TestMTeamDiscountToResult_FreeEndAt(t *testing.T) {
	dr := mTeamDiscountToResult("FREE", "2026-06-08T10:06:00Z")
	if dr.Level != model.DiscountFree {
		t.Errorf("expected DiscountFree, got %s", dr.Level)
	}
	if dr.FreeEndAt == nil {
		t.Fatal("expected FreeEndAt to be set")
	}
	if dr.FreeEndAt.Year() != 2026 {
		t.Errorf("expected year 2026, got %d", dr.FreeEndAt.Year())
	}
}

func TestMTeamDiscountToResult_NoFreeEndAtForNone(t *testing.T) {
	dr := mTeamDiscountToResult("NORMAL", "2026-06-08T10:06:00Z")
	if dr.FreeEndAt != nil {
		t.Errorf("expected FreeEndAt nil for DiscountNone, got %v", dr.FreeEndAt)
	}
}

func TestFetchItemsByAPI_NoAPIKey(t *testing.T) {
	a := NewMTeamAdapter(NewHTTPDoer(), zap.NewNop())
	_, err := a.FetchItemsByAPI(context.Background(), &model.SiteConfig{}, "https://x.com/feed", "馒头")
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
}

func TestFetchItemsByAPI_OK(t *testing.T) {
	apiResp := map[string]interface{}{
		"code": "0",
		"data": map[string]interface{}{
			"data": []map[string]interface{}{
				{
					"id":   "1191774",
					"name": "Test Torrent FREE",
					"size": 5640143360,
					"status": map[string]interface{}{
						"discount":         "FREE",
						"discountEndTime": "2026-06-10T00:00:00Z",
					},
				},
				{
					"id":   "1199999",
					"name": "Test Torrent PERCENT_50",
					"size": 1024,
					"status": map[string]interface{}{
						"discount": "PERCENT_50",
					},
				},
			},
		},
	}
	body, _ := json.Marshal(apiResp)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/torrent/search" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("missing x-api-key header: %s", r.Header.Get("x-api-key"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewMTeamAdapter(doer, zap.NewNop())

	config := &model.SiteConfig{Domain: srv.URL, APIKey: "test-key"}
	events, err := a.FetchItemsByAPI(context.Background(), config, srv.URL+"/api/torrent/search?discounts=FREE", "馒头")
	if err != nil {
		t.Fatal(err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event (FREE only), got %d", len(events))
	}

	ev := events[0]
	if ev.TorrentID != "1191774" {
		t.Errorf("expected torrentID 1191774, got %s", ev.TorrentID)
	}
	if ev.SiteName != "馒头" {
		t.Errorf("expected siteName 馒头, got %s", ev.SiteName)
	}
	if ev.DiscountLevel != model.DiscountFree {
		t.Errorf("expected DiscountFree, got %s", ev.DiscountLevel)
	}
	if !ev.IsFree {
		t.Errorf("expected IsFree true")
	}
	if ev.FreeEndAt == nil {
		t.Errorf("expected FreeEndAt to be set")
	}
	if !strings.Contains(ev.DownloadURL, "/download.php?id=1191774") {
		t.Errorf("unexpected DownloadURL: %s", ev.DownloadURL)
	}
}

func TestFetchItemsByAPI_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"code":"401","message":"Unauthorized"}`)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewMTeamAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL, APIKey: "test-key"}
	_, err := a.FetchItemsByAPI(context.Background(), config, srv.URL, "馒头")
	if err == nil {
		t.Fatal("expected error for API code != 0")
	}
	if !strings.Contains(err.Error(), "Unauthorized") {
		t.Errorf("expected error to contain 'Unauthorized', got %v", err)
	}
}

// §59.60 B1: detailViaAPI 必须检查 result.Code——API 错误（无效 tid/限流/熔断）
// 返回非 0 code 时应报错，不得构造空 detail 伪成功（243 实测空行落库根因）。
func TestGetTorrentDetail_APIError_MustFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"code":"404","message":"NO_DATA_BY_ID"}`)
	}))
	defer srv.Close()

	doer := &HTTPDoer{Client: srv.Client()}
	a := NewMTeamAdapter(doer, zap.NewNop())
	config := &model.SiteConfig{Domain: srv.URL, APIKey: "test-key"}

	detail, err := a.GetTorrentDetail(context.Background(), config, "1094449")
	if err == nil {
		t.Fatal("API code≠0 必须返回错误（原 bug: 返回空 detail 伪成功）")
	}
	if detail != nil {
		t.Fatalf("错误时不应返回 detail, got %+v", detail)
	}
}

// TestMTeamCategoryMapAuthoritative §59.317: 权威树映射——categoryList 实测两级树
// （2026-10-03）逐 ID 断言。旧表 421→cartoon（实为电影/BluRay）等全错位修正。
func TestMTeamCategoryMapAuthoritative(t *testing.T) {
	cases := map[string]string{
		// 电影（主类+子类）
		"100": "category.movie", "401": "category.movie", "419": "category.movie",
		"420": "category.movie", "421": "category.movie", "439": "category.movie",
		// 影剧/综艺
		"105": "category.tv_series", "402": "category.tv_series", "403": "category.tv_series",
		"435": "category.tv_series", "438": "category.tv_series",
		// 紀錄
		"444": "category.documentary", "404": "category.documentary",
		// 動漫（453=2026-07 新增）
		"449": "category.animation", "405": "category.animation", "453": "category.animation",
		// Music
		"406": "category.concert", "434": "category.lossless_music",
		// 遊戲/其他
		"447": "category.game", "423": "category.game", "448": "category.game",
		"407": "category.sports", "409": "category.other", "422": "category.software",
		"427": "category.ebook", "442": "category.audiobook", "451": "category.education",
	}
	for raw, want := range cases {
		if got := mteamCategoryMap[raw]; got != want {
			t.Errorf("map[%s] = %q, want %q", raw, got, want)
		}
	}
}

// TestNormalizeMTeamCategoryIDAdult §59.317: 成人集补缺（436 AV网站/440 AV Gay/
// 主类 115/120/445/446）+ 未知 ID 返回空（miss 告警路径）。
func TestNormalizeMTeamCategoryIDAdult(t *testing.T) {
	for _, raw := range []string{"436", "440", "115", "120", "445", "446", "429", "413"} {
		if got := normalizeMTeamCategoryID(raw); got != "category.adult" {
			t.Errorf("normalizeMTeamCategoryID(%s) = %q, want category.adult", raw, got)
		}
	}
	if got := normalizeMTeamCategoryID("421"); got != "category.movie" {
		t.Errorf("421 应为电影/BluRay→movie, got %q", got)
	}
	if got := normalizeMTeamCategoryID("999"); got != "" {
		t.Errorf("未知 ID 应返回空, got %q", got)
	}
	if got := normalizeMTeamCategoryID(""); got != "" {
		t.Errorf("空输入应返回空, got %q", got)
	}
}

// TestFetchItemsByAPI_CategoryMapped §59.317: RSS 链分类归一——原始 ID 不再透传
// 落 rss_torrent_seen.source_category（回归审查遗漏 A）。
func TestFetchItemsByAPI_CategoryMapped(t *testing.T) {
	apiResp := map[string]interface{}{
		"code": "0",
		"data": map[string]interface{}{
			"data": []map[string]interface{}{
				{
					"id": "1", "name": "Movie BluRay", "size": 1, "category": "421",
					"status": map[string]interface{}{"discount": "FREE"},
				},
				{
					"id": "2", "name": "Anime", "size": 1, "category": "405",
					"status": map[string]interface{}{"discount": "FREE"},
				},
				{
					"id": "3", "name": "AV", "size": 1, "category": "429",
					"status": map[string]interface{}{"discount": "FREE"},
				},
			},
		},
	}
	body, _ := json.Marshal(apiResp)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()
	a := NewMTeamAdapter(&HTTPDoer{Client: srv.Client()}, zap.NewNop())
	events, err := a.FetchItemsByAPI(context.Background(),
		&model.SiteConfig{Domain: srv.URL, APIKey: "k"}, srv.URL+"/api/torrent/search?discounts=FREE", "馒头")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
	want := map[string]string{"Movie BluRay": "category.movie", "Anime": "category.animation", "AV": "category.adult"}
	for _, ev := range events {
		if ev.Category != want[ev.Title] {
			t.Errorf("%s: category = %q, want %q", ev.Title, ev.Category, want[ev.Title])
		}
	}
}
