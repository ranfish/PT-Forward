package ptgen

import (
	"context"
	"encoding/json"
	"strings"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupPTGenDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.PTGenCache{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestProvider_Query_Empty(t *testing.T) {
	db := setupPTGenDB(t)
	p := NewProvider(db, zap.NewNop())
	_, err := p.Query(context.Background(), "")
	if err == nil {
		t.Error("expected error for empty query")
	}
}

func TestProvider_Query_CacheHit(t *testing.T) {
	db := setupPTGenDB(t)
	p := NewProvider(db, zap.NewNop())

	cache := &model.PTGenCache{
		QueryKey:     "tt1234567",
		ChineseTitle: "测试电影",
		PosterURL:    "https://example.com/poster.jpg",
		DoubanURL:    "https://movie.douban.com/123",
		BBCode:       "[b]test bbcode[/b]",
		Source:       "cache",
	}
	db.Create(cache)

	result, err := p.Query(context.Background(), "tt1234567")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Cached {
		t.Error("expected cached=true")
	}
	if result.ChineseTitle != "测试电影" {
		t.Errorf("expected 测试电影, got %s", result.ChineseTitle)
	}
	if result.PosterURL != "https://example.com/poster.jpg" {
		t.Errorf("poster url mismatch: %s", result.PosterURL)
	}
}

func TestProvider_Query_RemoteSuccess(t *testing.T) {
	db := setupPTGenDB(t)
	p := NewProvider(db, zap.NewNop())

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if err := json.NewEncoder(w).Encode(map[string]any{
			"chinese_title": "测试电影",
			"foreign_title": "Test Movie",
			"year":          "2024",
			"poster":        "https://example.com/p.jpg",
			"douban_url":    "https://movie.douban.com/123",
			"imdb_url":      "https://www.imdb.com/title/tt1234567",
			"imdb_id":       "tt1234567",
			"douban_rating": "8.5",
			"imdb_rating":   "7.8",
			"introduction":  "A test movie",
			"bbcode":        "[b]test[/b]",
			"region":        []string{"中国大陆"},
			"genre":         []string{"剧情"},
			"director":      []string{"张三"},
		}); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer server.Close()

	p.client = &http.Client{}
	origEndpoints := p.queryRemote

	result, err := p.queryEndpoint(context.Background(), server.URL+"/api", "tt1234567")
	if err != nil {
		t.Fatal(err)
	}

	if result.ChineseTitle != "测试电影" {
		t.Errorf("expected 测试电影, got %s", result.ChineseTitle)
	}
	if result.Year != "2024" {
		t.Errorf("expected 2024, got %s", result.Year)
	}
	if result.IMDBID != "tt1234567" {
		t.Errorf("expected tt1234567, got %s", result.IMDBID)
	}
	if len(result.Region) != 1 || result.Region[0] != "中国大陆" {
		t.Errorf("region mismatch: %v", result.Region)
	}
	if len(result.Genre) != 1 || result.Genre[0] != "剧情" {
		t.Errorf("genre mismatch: %v", result.Genre)
	}

	_ = origEndpoints
}

func TestProvider_Query_RemoteFail_NoCache(t *testing.T) {
	db := setupPTGenDB(t)
	p := NewProvider(db, zap.NewNop())

	_, err := p.Query(context.Background(), "nonexistent_query_xyz")
	if err == nil {
		t.Error("expected error when remote fails and no cache")
	}
}

func TestProvider_Query_CacheSaveAndReuse(t *testing.T) {
	db := setupPTGenDB(t)
	p := NewProvider(db, zap.NewNop())

	called := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called++
		if err := json.NewEncoder(w).Encode(map[string]any{
			"chinese_title": "缓存测试",
			"poster":        "https://example.com/p.jpg",
			"bbcode":        "[b]test[/b]",
		}); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer server.Close()

	p.client = &http.Client{}
	result, err := p.queryEndpoint(context.Background(), server.URL, "tt999")
	if err != nil {
		t.Fatal(err)
	}
	result.Source = server.URL

	if err := p.saveCache(context.Background(), "tt999", result); err != nil {
		t.Fatal(err)
	}

	cached, err := p.getCache(context.Background(), "tt999")
	if err != nil {
		t.Fatal(err)
	}
	if cached.ChineseTitle != "缓存测试" {
		t.Errorf("expected 缓存测试, got %s", cached.ChineseTitle)
	}
}

func TestProvider_CleanExpiredCache(t *testing.T) {
	db := setupPTGenDB(t)
	p := NewProvider(db, zap.NewNop())

	db.Create(&model.PTGenCache{
		QueryKey:     "old",
		ChineseTitle: "Old",
		UpdatedAt:    time.Now().AddDate(0, 0, -60),
	})
	db.Create(&model.PTGenCache{
		QueryKey:     "recent",
		ChineseTitle: "Recent",
		UpdatedAt:    time.Now(),
	})

	deleted, err := p.CleanExpiredCache(context.Background(), 30)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 deleted, got %d", deleted)
	}

	var count int64
	db.Model(&model.PTGenCache{}).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 remaining, got %d", count)
	}
}

func TestProvider_SupportsNameSearch(t *testing.T) {
	p := NewProvider(nil, zap.NewNop())
	if !p.SupportsNameSearch() {
		t.Error("should support name search")
	}
}

func TestProvider_QueryEndpoint_Error(t *testing.T) {
	db := setupPTGenDB(t)
	p := NewProvider(db, zap.NewNop())

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer server.Close()

	p.client = &http.Client{}
	_, err := p.queryEndpoint(context.Background(), server.URL, "tt1")
	if err == nil {
		t.Error("expected error for 500 response")
	}
}

// §59.46: doubaninfo format 字段解析（BBCode 口径统一）
func TestQueryDoubanInfoFormatFallback(t *testing.T) {
	// mock doubaninfo 响应：无 bbcode 只有 format
	payload := `{"success":true,"chinese_title":"幻想","poster":"https://doubaninfo.com/dbposter/x.jpg",
		"format":"[img]https://doubaninfo.com/dbposter/x.jpg[/img]\n\n◎片　　名　幻想"}`
	var raw map[string]any
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		t.Fatal(err)
	}
	got := parseBBCodeField(raw)
	if got == "" || !strings.Contains(got, "◎片") {
		t.Errorf("format 应兜底: %q", got)
	}
	raw2 := map[string]any{"bbcode": "BB", "format": "FF"}
	if parseBBCodeField(raw2) != "BB" {
		t.Error("bbcode 应优先于 format")
	}
}

// §59.173: QueryForce 绕缓存直连——缓存命中时 Query 返回 Cached=true，
// QueryForce 必须绕过缓存重查（无远程 endpoint 时报错而非吃缓存）。
func TestProviderQueryForceBypassesCache(t *testing.T) {
	db := setupPTGenDB(t)
	p := NewProvider(db, zap.NewNop())
	q := "https://movie.douban.com/subject/force_test/"
	// 种缓存：30 天内新鲜
	if err := db.Create(&model.PTGenCache{
		QueryKey: q, ChineseTitle: "缓存标题", BBCode: "[b]cache[/b]",
		JSONData: `{"chinese_title":"缓存标题"}`, UpdatedAt: time.Now(),
	}).Error; err != nil {
		t.Fatal(err)
	}
	// Query：命中缓存
	r1, err := p.Query(context.Background(), q)
	if err != nil || r1 == nil || !r1.Cached || r1.ChineseTitle != "缓存标题" {
		t.Fatalf("Query 应命中缓存: %+v err=%v", r1, err)
	}
	// QueryForce：无 endpoint 配置 → 必须报错（证明绕过了缓存直达远端）
	r2, err := p.QueryForce(context.Background(), q)
	if err == nil {
		t.Fatalf("QueryForce 应绕缓存（无 endpoint 必然失败）却成功: %+v", r2)
	}
	if r2 != nil && r2.Cached {
		t.Fatal("QueryForce 不得返回缓存结果")
	}
}
