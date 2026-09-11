package reseed

import (
	"context"
	"testing"

	"github.com/ranfish/pt-forward/internal/mocks"
	"github.com/ranfish/pt-forward/internal/model"
)

// §59.196 侠女×Valerie 错挂案回归三测：
// ① 主轮纯 CJK 补 area=1（简介搜索命中正确 tid，size+组名确认把关）
// ② 短词轮标题锚守卫（"1970 1080p" 光技术词不进轮次）
// ③ loosePick 双盲拒绝（源 CJK×候选无 CJK×源无 meaningful 英文词 → 拒）

const (
	misfetchSourceTitle = "侠女.1970.1080p.国语.简繁中字￡CMCT陆判"
	misfetchGroup       = "CMCT"
	misfetchSize        = int64(1234567890)
	valerieTitle        = "Valerie.and.Her.Week.of.Wonders.AKA.Valerie.a.týden.divu.1970.Criterion.Collection.1080p.Blu-ray.x264-CMCT"
	zenTitle            = "A.Touch.of.Zen.1970.4K.REMASTER.HKG.BluRay.1080p.x264.FLAC-CMCT"
)

// ① 主轮：纯 CJK 重试词（"侠女"）area=0 无果后补 area=1 简介搜索 → 命中正确 tid。
func TestMainRoundPureCJKArea1Supplement(t *testing.T) {
	adapter := &mocks.SiteAdapter{
		SearchTorrentsFn: func(ctx context.Context, config *model.SiteConfig, query string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
			area := ""
			if opts != nil {
				area = opts.SearchArea
			}
			// 主搜索与英文残余词、纯 CJK area=0：无结果（标题索引英文形态）
			if area != "1" {
				return nil, nil
			}
			// area=1 简介搜索：仅纯中文词命中正确资源
			if query == "侠女" {
				return []*model.SeedingSearchResult{
					{TorrentID: "288084", Title: zenTitle, Size: misfetchSize},
				}, nil
			}
			return nil, nil
		},
	}
	m, _, err := SearchAndVerifyMatchWithResults(context.Background(), adapter,
		&model.SiteConfig{}, "侠女 1970 1080p", misfetchGroup, misfetchSize, misfetchSourceTitle)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if m == nil {
		t.Fatal("expected match via pure-CJK area=1 supplement, got nil")
	}
	if m.TorrentID != "288084" {
		t.Fatalf("expected tid 288084, got %s", m.TorrentID)
	}
}

// ① 反证：主搜索若返回同年同规格错误候选（Valerie），size 守卫拒绝——
// 错误形态必须由补线命中正确 tid 而非错误候选。
func TestMainRoundWrongYearMatchRejected(t *testing.T) {
	adapter := &mocks.SiteAdapter{
		SearchTorrentsFn: func(ctx context.Context, config *model.SiteConfig, query string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
			area := ""
			if opts != nil {
				area = opts.SearchArea
			}
			if area != "1" && query == "1970 1080p" {
				return []*model.SeedingSearchResult{
					{TorrentID: "617566", Title: valerieTitle, Size: misfetchSize + 500000000},
				}, nil
			}
			if area == "1" && query == "侠女" {
				return []*model.SeedingSearchResult{
					{TorrentID: "288084", Title: zenTitle, Size: misfetchSize},
				}, nil
			}
			return nil, nil
		},
	}
	m, _, err := SearchAndVerifyMatchWithResults(context.Background(), adapter,
		&model.SiteConfig{}, "侠女 1970 1080p", misfetchGroup, misfetchSize, misfetchSourceTitle)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if m == nil || m.TorrentID != "288084" {
		t.Fatalf("expected correct tid 288084 (Valerie size-guarded out), got %+v", m)
	}
}

// ② 短词轮守卫：短词剥完剩 "1970 1080p"（无标题锚）→ 不进轮次 →
// 即使站点对光年份词返回 Valerie 也不搜索不错挂。
func TestLooseShortKeywordAnchorGuard(t *testing.T) {
	searched := map[string]int{}
	adapter := &mocks.SiteAdapter{
		SearchTorrentsFn: func(ctx context.Context, config *model.SiteConfig, query string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
			searched[query]++
			// 站点对任意年份词搜索都返回 Valerie（模拟真实错挂形态）
			if query == "1970 1080p" {
				return []*model.SeedingSearchResult{
					{TorrentID: "617566", Title: valerieTitle, Size: misfetchSize + 500000000},
				}, nil
			}
			return nil, nil
		},
	}
	m := SearchAndVerifyLoose(context.Background(), adapter,
		&model.SiteConfig{}, "侠女 1970 1080p", misfetchGroup, misfetchSourceTitle)
	if m != nil {
		t.Fatalf("expected nil (anchor guard blocks junk round), got tid %s", m.TorrentID)
	}
	if n := searched["1970 1080p"]; n != 0 {
		t.Fatalf("anchor-less short keyword should never be searched, got %d hits", n)
	}
}

// ③ loosePick 双盲拒绝：简介搜索（area=1 原词）返回纯英文候选 × 纯中文源 → 拒绝。
func TestLooseCrossLanguageBlindReject(t *testing.T) {
	adapter := &mocks.SiteAdapter{
		SearchTorrentsFn: func(ctx context.Context, config *model.SiteConfig, query string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
			return []*model.SeedingSearchResult{
				{TorrentID: "617566", Title: valerieTitle, Size: misfetchSize},
			}, nil
		},
	}
	m := SearchAndVerifyLoose(context.Background(), adapter,
		&model.SiteConfig{}, "侠女 1970 1080p", misfetchGroup, misfetchSourceTitle)
	if m != nil {
		t.Fatalf("expected blind-reject of cross-language candidate, got tid %s", m.TorrentID)
	}
}

// ③ 反证：合法 loose 场景不受影响——源英文×候选同组英文（keepfrds 系
// REPACK size 不一致形态）仍放行。
func TestLooseEnglishSourceStillPasses(t *testing.T) {
	src := "Movie.Name.2020.1080p.BluRay.x264-GRP"
	adapter := &mocks.SiteAdapter{
		SearchTorrentsFn: func(ctx context.Context, config *model.SiteConfig, query string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
			return []*model.SeedingSearchResult{
				{TorrentID: "777", Title: "Movie.Name.2020.REPACK.1080p.BluRay.x264-GRP", Size: 5555555},
			}, nil
		},
	}
	m := SearchAndVerifyLoose(context.Background(), adapter,
		&model.SiteConfig{}, "movie name 2020 1080p", "GRP", src)
	if m == nil || m.TorrentID != "777" {
		t.Fatalf("expected legitimate loose match preserved, got %+v", m)
	}
}

// keywordHasTitleAnchor 单元：
func TestKeywordHasTitleAnchor(t *testing.T) {
	cases := []struct {
		kw   string
		want bool
	}{
		{"1970 1080p", false},
		{"1080p x264", false},
		{"2020", false},
		{"侠女", true},
		{"侠女 1080p", true},
		{"valerie 1970", true},
		{"movie name 2020", true},
	}
	for _, c := range cases {
		if got := keywordHasTitleAnchor(c.kw); got != c.want {
			t.Errorf("keywordHasTitleAnchor(%q) = %v, want %v", c.kw, got, c.want)
		}
	}
}
