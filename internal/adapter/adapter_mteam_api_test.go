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
