package ptgen

import (
	"context"
	"encoding/json"
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
	db.AutoMigrate(&model.PTGenCache{})
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
		json.NewEncoder(w).Encode(map[string]any{
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
		})
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
		json.NewEncoder(w).Encode(map[string]any{
			"chinese_title": "缓存测试",
			"poster":        "https://example.com/p.jpg",
			"bbcode":        "[b]test[/b]",
		})
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
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	p.client = &http.Client{}
	_, err := p.queryEndpoint(context.Background(), server.URL, "tt1")
	if err == nil {
		t.Error("expected error for 500 response")
	}
}
