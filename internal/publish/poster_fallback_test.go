package publish

import (
	"context"
	"errors"
	"testing"
)

// §59.42: 可信图源白名单判定
func TestIsTrustedPoster(t *testing.T) {
	cases := []struct {
		url  string
		want bool
	}{
		{"https://doubaninfo.com/dbposter/doubaninfo_com_1294182.jpg", true},
		{"https://img3.pixhost.cc/images/5068/760562844_ptgen_poster_gkji6w.jpg", true},
		{"https://img9.pixhost.to/abc/123.jpg", true},
		{"https://t1.pixho.st/abc/123.jpg", true},
		{"https://pt.keepfrds.com/poster_douban/w185/2782320.webp", false}, // 朋友站代理豆瓣（实测 404 形态）
		{"https://img9.doubanio.com/view/photo/l/public/p123.jpg", false},  // 豆瓣官方（外发必裂）
		{"", false},
		{"not-a-url", false},
	}
	for _, c := range cases {
		if got := IsTrustedPoster(c.url); got != c.want {
			t.Errorf("IsTrustedPoster(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

// §59.42: 两级 fallback 链（mock querier，不依赖真实 endpoint）
func TestRunPosterFallback(t *testing.T) {
	ctx := context.Background()

	trusted := "https://doubaninfo.com/dbposter/x.jpg"
	trusted2 := "https://img3.pixhost.cc/images/1/2.jpg"

	// mock 探活：可信域一律活，其他死（测试不发真实网络请求）
	origAlive := posterAliveFn
	posterAliveFn = func(ctx context.Context, u string) bool { return IsTrustedPoster(u) }
	defer func() { posterAliveFn = origAlive }()

	t.Run("可信原图保留", func(t *testing.T) {
		res := RunPosterFallback(ctx, trusted, "q", nil)
		if res.Poster != trusted || res.Source != "site" {
			t.Errorf("可信应保留: %+v", res)
		}
	})
	t.Run("不可信_首选成功", func(t *testing.T) {
		q1 := func(ctx context.Context, q string) (string, error) { return trusted, nil }
		res := RunPosterFallback(ctx, "https://pt.keepfrds.com/x.webp", "q", []PTGenQuerier{q1})
		if res.Poster != trusted || res.Source != "ptgen_doubaninfo" || res.Original != "https://pt.keepfrds.com/x.webp" {
			t.Errorf("首选应替换: %+v", res)
		}
	})
	t.Run("首选死链_次选成功", func(t *testing.T) {
		dead := "https://dead.example.com/404.jpg"
		q1 := func(ctx context.Context, q string) (string, error) { return dead, nil }
		q2 := func(ctx context.Context, q string) (string, error) { return trusted2, nil }
		res := RunPosterFallback(ctx, "https://pt.keepfrds.com/x.webp", "q", []PTGenQuerier{q1, q2})
		if res.Poster != trusted2 || res.Source != "ptgen_cspt" {
			t.Errorf("次选应替换: %+v", res)
		}
	})
	t.Run("全死_保留原值", func(t *testing.T) {
		dead := "https://dead.example.com/404.jpg"
		q1 := func(ctx context.Context, q string) (string, error) { return dead, nil }
		q2 := func(ctx context.Context, q string) (string, error) { return "", errors.New("fail") }
		res := RunPosterFallback(ctx, "https://pt.keepfrds.com/x.webp", "q", []PTGenQuerier{q1, q2})
		if res.Poster != "https://pt.keepfrds.com/x.webp" || res.Source != "ptgen_dead" {
			t.Errorf("全死应保留原值: %+v", res)
		}
	})
	t.Run("querier全错误_保留", func(t *testing.T) {
		q1 := func(ctx context.Context, q string) (string, error) { return "", errors.New("err") }
		res := RunPosterFallback(ctx, "https://pt.keepfrds.com/x.webp", "q", []PTGenQuerier{q1})
		if res.Source != "ptgen_dead" {
			t.Errorf("全错应 ptgen_dead: %+v", res)
		}
	})
	t.Run("空query跳过", func(t *testing.T) {
		called := false
		q1 := func(ctx context.Context, q string) (string, error) { called = true; return "", nil }
		res := RunPosterFallback(ctx, "https://pt.keepfrds.com/x.webp", "", []PTGenQuerier{q1})
		if called || res.Source != "ptgen_dead" {
			t.Errorf("空 query 不应调用 querier: called=%v %+v", called, res)
		}
	})
}
