package orphan

import (
	"context"
	"errors"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

// §59.185 A: 限流类错误识别——"domain rate limit acquire failed"（我方排队超时）。
func TestIsRateLimitErr(t *testing.T) {
	if !isRateLimitErr(errors.New("search failed: domain rate limit acquire failed: context deadline exceeded")) {
		t.Fatal("domain rate limit 错误应识别")
	}
	if isRateLimitErr(errors.New("HTTP 429 during search")) {
		t.Fatal("HTTP 429 站方错误不属此类（不重试搜索）")
	}
	if isRateLimitErr(nil) {
		t.Fatal("nil 不识别")
	}
}

// fakeRateLimitAdapter 首次返回限流错误，第二次成功
type fakeRateLimitAdapter struct {
	model.SiteAdapter // 嵌入接口：其余方法 nil 兜底（本测试只调 SearchTorrents）
	calls             int
}

func (f *fakeRateLimitAdapter) SearchTorrents(ctx context.Context, config *model.SiteConfig, keyword string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
	f.calls++
	if f.calls == 1 {
		return nil, errors.New("search failed: domain rate limit acquire failed: context deadline exceeded")
	}
	return []*model.SeedingSearchResult{{TorrentID: "t1", Title: "Movie", Size: 100}}, nil
}

// §59.185 A: searchWithBackoff 限流退避重试——一次退避后成功。
func TestSearchWithBackoff(t *testing.T) {
	r := &Recovery{logger: zap.NewNop()}
	fake := &fakeRateLimitAdapter{}
	results, err := r.searchWithBackoff(context.Background(), fake, &model.SiteConfig{}, "kw")
	if err != nil {
		t.Fatalf("应重试成功: %v", err)
	}
	if len(results) != 1 || fake.calls != 2 {
		t.Fatalf("calls=%d results=%d（期望 1 次失败+10s 退避+1 次成功）", fake.calls, len(results))
	}
}

// 非限流错误不重试（直接返回）
type fakePlainErrorAdapter struct {
	model.SiteAdapter
	calls int
}

func (f *fakePlainErrorAdapter) SearchTorrents(ctx context.Context, config *model.SiteConfig, keyword string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
	f.calls++
	return nil, errors.New("HTTP 468 during search")
}

func TestSearchWithBackoff_NoRetryOnPlainError(t *testing.T) {
	r := &Recovery{logger: zap.NewNop()}
	fake := &fakePlainErrorAdapter{}
	_, err := r.searchWithBackoff(context.Background(), fake, &model.SiteConfig{}, "kw")
	if err == nil {
		t.Fatal("应返回错误")
	}
	if fake.calls != 1 {
		t.Fatalf("非限流错误不应重试, calls=%d", fake.calls)
	}
}
