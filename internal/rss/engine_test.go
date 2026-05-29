package rss

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/event"
	"github.com/ranfish/pt-forward/internal/filter"
	"github.com/ranfish/pt-forward/internal/mocks"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupEngineDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err, "open db")
	for _, m := range []interface{}{
		&model.Site{},
		&model.RSSSubscription{},
		&model.RSSTorrentSeen{},
		&model.FilterRule{},
		&model.TorrentEvent{},
		&model.SeedingTorrentRecord{},
		&model.PublishCandidate{},
	} {
		require.NoError(t, db.AutoMigrate(m), "migrate")
	}
	return db
}

func newEngineWithDB(t *testing.T) (*Engine, *gorm.DB) {
	t.Helper()
	db := setupEngineDB(t)
	eng := NewEngine(db, zap.NewNop())
	return eng, db
}

const rssXMLTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
<channel>
<title>Test Feed</title>
<link>https://example.com</link>
%s
</channel>
</rss>`

func rssItem(title, link, guid, hash, size string) string {
	return fmt.Sprintf(`<item>
<title>%s</title>
<link>%s</link>
<guid>%s</guid>
<enclosure url="%s" length="%s" type="application/x-bittorrent"/>
<hash>%s</hash>
<size>%s</size>
</item>`, title, link, guid, link, size, hash, size)
}

func serveRSS(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func serveRssWithItems(t *testing.T, items string) *httptest.Server {
	t.Helper()
	xml := fmt.Sprintf(rssXMLTemplate, items)
	return serveRSS(t, xml)
}

func makeSub(db *gorm.DB, t *testing.T, name, siteName string, urls []string) *model.RSSSubscription {
	t.Helper()
	sub := &model.RSSSubscription{
		Name:     name,
		SiteName: siteName,
		URLs:     urls,
		Cron:     "*/5 * * * *",
		Enabled:  true,
	}
	require.NoError(t, db.Create(sub).Error)
	return sub
}

func makeSite(db *gorm.DB, t *testing.T, name, domain string) *model.Site {
	t.Helper()
	site := &model.Site{
		Name:         name,
		Domain:       domain,
		BaseURL:      "https://" + domain,
		Framework:    "nexusphp",
		HashStrategy: "xml_tag",
		SizeStrategy: "xml_tag",
		IDStrategy:   "query_param",
		IDPattern:    "id",
		Enabled:      true,
	}
	require.NoError(t, db.Create(site).Error)
	return site
}

func TestEngine_SiteHRStrategy_NilProvider(t *testing.T) {
	eng, _ := newEngineWithDB(t)
	result := eng.siteHRStrategy(context.Background(), "somesite")
	require.Equal(t, "protect", result)
}

func TestEngine_SiteHRStrategy_Skip(t *testing.T) {
	eng, _ := newEngineWithDB(t)
	eng.SetSiteProvider(&mocks.SiteInfoProvider{
		GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
			return &model.SiteConfig{HRStrategy: "skip"}, nil
		},
	})
	require.Equal(t, "skip", eng.siteHRStrategy(context.Background(), "somesite"))
}

func TestEngine_SiteHRStrategy_Ignore(t *testing.T) {
	eng, _ := newEngineWithDB(t)
	eng.SetSiteProvider(&mocks.SiteInfoProvider{
		GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
			return &model.SiteConfig{HRStrategy: "ignore"}, nil
		},
	})
	require.Equal(t, "ignore", eng.siteHRStrategy(context.Background(), "somesite"))
}

func TestEngine_SiteHRStrategy_ProtectDefault(t *testing.T) {
	eng, _ := newEngineWithDB(t)
	eng.SetSiteProvider(&mocks.SiteInfoProvider{
		GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
			return &model.SiteConfig{HRStrategy: "protect"}, nil
		},
	})
	require.Equal(t, "protect", eng.siteHRStrategy(context.Background(), "somesite"))
}

func TestEngine_SiteHRStrategy_ConfigError(t *testing.T) {
	eng, _ := newEngineWithDB(t)
	eng.SetSiteProvider(&mocks.SiteInfoProvider{
		GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
			return nil, fmt.Errorf("not found")
		},
	})
	require.Equal(t, "protect", eng.siteHRStrategy(context.Background(), "somesite"))
}

func TestEngine_SiteHRStrategy_NilConfig(t *testing.T) {
	eng, _ := newEngineWithDB(t)
	eng.SetSiteProvider(&mocks.SiteInfoProvider{
		GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
			return nil, nil
		},
	})
	require.Equal(t, "protect", eng.siteHRStrategy(context.Background(), "somesite"))
}

func TestEngine_SiteHRStrategy_UnknownStrategyFallsToProtect(t *testing.T) {
	eng, _ := newEngineWithDB(t)
	eng.SetSiteProvider(&mocks.SiteInfoProvider{
		GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
			return &model.SiteConfig{HRStrategy: "bogus"}, nil
		},
	})
	require.Equal(t, "protect", eng.siteHRStrategy(context.Background(), "somesite"))
}

func TestEngine_DetectHRAndDiscount_NilProvider(t *testing.T) {
	eng, _ := newEngineWithDB(t)
	evt := &model.RSSTorrentEvent{TorrentID: "42", SiteName: "site"}
	eng.detectHRAndDiscount(context.Background(), evt, "site")
	require.False(t, evt.HasHR)
	require.False(t, evt.IsFree)
}

func TestEngine_DetectHRAndDiscount_EmptyTorrentID(t *testing.T) {
	eng, _ := newEngineWithDB(t)
	eng.SetSiteProvider(&mocks.SiteInfoProvider{})
	evt := &model.RSSTorrentEvent{TorrentID: "", SiteName: "site"}
	eng.detectHRAndDiscount(context.Background(), evt, "site")
	require.False(t, evt.HasHR)
}

func TestEngine_DetectHRAndDiscount_HRDetected(t *testing.T) {
	eng, _ := newEngineWithDB(t)
	adapter := &mocks.SiteAdapter{
		DetectHRFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.HRResult, error) {
			return &model.HRResult{HasHR: true, SeedTimeH: 96}, nil
		},
		DetectDiscountFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.DiscountResult, error) {
			return nil, nil
		},
	}
	eng.SetSiteProvider(&mocks.SiteInfoProvider{
		GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
			return adapter, nil
		},
		GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
			return &model.SiteConfig{}, nil
		},
	})
	evt := &model.RSSTorrentEvent{TorrentID: "42", SiteName: "site"}
	eng.detectHRAndDiscount(context.Background(), evt, "site")
	require.True(t, evt.HasHR)
	require.Equal(t, 96, evt.HRSeedTimeH)
}

func TestEngine_DetectHRAndDiscount_HRDefaultSeedTime(t *testing.T) {
	eng, _ := newEngineWithDB(t)
	adapter := &mocks.SiteAdapter{
		DetectHRFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.HRResult, error) {
			return &model.HRResult{HasHR: true, SeedTimeH: 0}, nil
		},
		DetectDiscountFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.DiscountResult, error) {
			return nil, nil
		},
	}
	eng.SetSiteProvider(&mocks.SiteInfoProvider{
		GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
			return adapter, nil
		},
		GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
			return &model.SiteConfig{}, nil
		},
	})
	evt := &model.RSSTorrentEvent{TorrentID: "42", SiteName: "site"}
	eng.detectHRAndDiscount(context.Background(), evt, "site")
	require.True(t, evt.HasHR)
	require.Equal(t, 72, evt.HRSeedTimeH)
}

func TestEngine_DetectHRAndDiscount_FreeDiscount(t *testing.T) {
	eng, _ := newEngineWithDB(t)
	freeEnd := time.Now().Add(24 * time.Hour)
	adapter := &mocks.SiteAdapter{
		DetectHRFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.HRResult, error) {
			return &model.HRResult{HasHR: false}, nil
		},
		DetectDiscountFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.DiscountResult, error) {
			return &model.DiscountResult{Level: model.DiscountFree, FreeEndAt: &freeEnd}, nil
		},
	}
	eng.SetSiteProvider(&mocks.SiteInfoProvider{
		GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
			return adapter, nil
		},
		GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
			return &model.SiteConfig{}, nil
		},
	})
	evt := &model.RSSTorrentEvent{TorrentID: "42", SiteName: "site"}
	eng.detectHRAndDiscount(context.Background(), evt, "site")
	require.True(t, evt.IsFree)
	require.NotNil(t, evt.FreeEndAt)
}

func TestEngine_DetectHRAndDiscount_NoneDiscount(t *testing.T) {
	eng, _ := newEngineWithDB(t)
	adapter := &mocks.SiteAdapter{
		DetectHRFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.HRResult, error) {
			return &model.HRResult{HasHR: false}, nil
		},
		DetectDiscountFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.DiscountResult, error) {
			return &model.DiscountResult{Level: model.DiscountNone}, nil
		},
	}
	eng.SetSiteProvider(&mocks.SiteInfoProvider{
		GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
			return adapter, nil
		},
		GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
			return &model.SiteConfig{}, nil
		},
	})
	evt := &model.RSSTorrentEvent{TorrentID: "42", SiteName: "site"}
	eng.detectHRAndDiscount(context.Background(), evt, "site")
	require.False(t, evt.IsFree)
}

func TestEngine_DetectHRAndDiscount_AdapterError(t *testing.T) {
	eng, _ := newEngineWithDB(t)
	eng.SetSiteProvider(&mocks.SiteInfoProvider{
		GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
			return nil, fmt.Errorf("no adapter")
		},
	})
	evt := &model.RSSTorrentEvent{TorrentID: "42", SiteName: "site"}
	eng.detectHRAndDiscount(context.Background(), evt, "site")
	require.False(t, evt.HasHR)
}

func TestEngine_DetectHRAndDiscount_2xFreeDiscount(t *testing.T) {
	eng, _ := newEngineWithDB(t)
	adapter := &mocks.SiteAdapter{
		DetectHRFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.HRResult, error) {
			return &model.HRResult{HasHR: false}, nil
		},
		DetectDiscountFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.DiscountResult, error) {
			return &model.DiscountResult{Level: model.Discount2xFree}, nil
		},
	}
	eng.SetSiteProvider(&mocks.SiteInfoProvider{
		GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
			return adapter, nil
		},
		GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
			return &model.SiteConfig{}, nil
		},
	})
	evt := &model.RSSTorrentEvent{TorrentID: "42", SiteName: "site"}
	eng.detectHRAndDiscount(context.Background(), evt, "site")
	require.True(t, evt.IsFree)
}

func TestEngine_StartStop(t *testing.T) {
	eng, db := newEngineWithDB(t)
	makeSite(db, t, "testsit", "testsit.com")
	makeSub(db, t, "sub1", "testsit", []string{"https://testsit.com/rss"})

	err := eng.Start(context.Background())
	require.NoError(t, err)
	require.Len(t, eng.tasks, 1)

	eng.Stop()
	require.Len(t, eng.tasks, 0)
}

func TestEngine_Start_DBError(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	eng := NewEngine(db, zap.NewNop())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = eng.Start(ctx)
	require.Error(t, err)
}

func TestEngine_Stop_CleansAllTasks(t *testing.T) {
	eng, db := newEngineWithDB(t)
	makeSite(db, t, "s1", "s1.com")
	makeSite(db, t, "s2", "s2.com")
	makeSub(db, t, "sub1", "s1", []string{"https://s1.com/rss"})
	makeSub(db, t, "sub2", "s2", []string{"https://s2.com/rss"})

	require.NoError(t, eng.Start(context.Background()))
	require.Len(t, eng.tasks, 2)

	eng.Stop()
	require.Len(t, eng.tasks, 0)
}

func TestEngine_Trigger_Success(t *testing.T) {
	eng, db := newEngineWithDB(t)
	makeSite(db, t, "testsit", "testsit.com")

	srv := serveRssWithItems(t, rssItem(
		"Ubuntu 24.04",
		"https://testsit.com/download.php?id=100",
		"100",
		"abc123def456abc123def456abc123def456abcd",
		"4700000000",
	))

	sub := makeSub(db, t, "trigger-sub", "testsit", []string{srv.URL})
	eng.fetcher = NewFetcherWithClient(srv.Client(), zap.NewNop())

	var dispatched []model.TorrentEvent
	var dispatchMu sync.Mutex
	d := event.NewDispatcher(zap.NewNop())
	d.Register("rss_new", &mocks.EventHandler{
		Fn: func(ctx context.Context, events []model.TorrentEvent) error {
			dispatchMu.Lock()
			defer dispatchMu.Unlock()
			dispatched = append(dispatched, events...)
			return nil
		},
	})
	eng.SetDispatcher(d)

	require.NoError(t, eng.Trigger(context.Background(), sub.ID))
	time.Sleep(500 * time.Millisecond)

	dispatchMu.Lock()
	defer dispatchMu.Unlock()
	require.Len(t, dispatched, 1)
	require.Equal(t, "100", dispatched[0].TorrentID)
	require.Equal(t, "Ubuntu 24.04", dispatched[0].Title)
}

func TestEngine_Trigger_SubNotFound(t *testing.T) {
	eng, _ := newEngineWithDB(t)
	err := eng.Trigger(context.Background(), 9999)
	require.Error(t, err)
	require.Equal(t, 13002, err.(*model.AppError).Code)
}

func TestEngine_Trigger_SubDisabled(t *testing.T) {
	eng, db := newEngineWithDB(t)
	sub := &model.RSSSubscription{
		Name: "disabled-sub", SiteName: "s", URLs: []string{"https://x.com/rss"},
	}
	require.NoError(t, db.Create(sub).Error)
	require.NoError(t, db.Model(sub).Update("enabled", false).Error)

	err := eng.Trigger(context.Background(), sub.ID)
	require.Error(t, err)
	require.Equal(t, 13003, err.(*model.AppError).Code)
}

func TestEngine_AddSubscription(t *testing.T) {
	eng, db := newEngineWithDB(t)
	makeSite(db, t, "testsit", "testsit.com")

	srv := serveRssWithItems(t, rssItem(
		"Test Torrent",
		"https://testsit.com/download.php?id=200",
		"200",
		"aa22bb33cc44dd55ee66ff77aa88bb99cc00ddee",
		"1000000000",
	))

	sub := &model.RSSSubscription{
		Name: "dynamic-sub", SiteName: "testsit", URLs: []string{srv.URL},
		Cron: "*/5 * * * *", Enabled: true,
	}
	require.NoError(t, db.Create(sub).Error)

	eng.fetcher = NewFetcherWithClient(srv.Client(), zap.NewNop())
	eng.AddSubscription(context.Background(), sub)

	require.Len(t, eng.tasks, 1)
	_, ok := eng.tasks[sub.ID]
	require.True(t, ok)

	eng.Stop()
}

func TestEngine_RemoveSubscription(t *testing.T) {
	eng, db := newEngineWithDB(t)
	makeSite(db, t, "testsit", "testsit.com")

	sub := makeSub(db, t, "rem-sub", "testsit", []string{"https://testsit.com/rss"})

	require.NoError(t, eng.Start(context.Background()))
	require.Len(t, eng.tasks, 1)

	eng.RemoveSubscription(sub.ID)
	require.Len(t, eng.tasks, 0)

	eng.Stop()
}

func TestEngine_RemoveSubscription_NotRunning(t *testing.T) {
	eng, _ := newEngineWithDB(t)
	eng.RemoveSubscription(9999)
	require.Len(t, eng.tasks, 0)
}

func TestEngine_FetchOnce_HappyPath(t *testing.T) {
	eng, db := newEngineWithDB(t)
	makeSite(db, t, "testsit", "testsit.com")

	srv := serveRssWithItems(t, rssItem(
		"Ubuntu 24.04 LTS",
		"https://testsit.com/download.php?id=501",
		"501",
		"aa111bb222cc333dd444ee555ff666aa777bb88",
		"4700000000",
	))

	sub := makeSub(db, t, "happy-sub", "testsit", []string{srv.URL})
	eng.fetcher = NewFetcherWithClient(srv.Client(), zap.NewNop())

	var dispatched []model.TorrentEvent
	d := event.NewDispatcher(zap.NewNop())
	d.Register("rss_new", &mocks.EventHandler{
		Fn: func(ctx context.Context, events []model.TorrentEvent) error {
			dispatched = append(dispatched, events...)
			return nil
		},
	})
	eng.SetDispatcher(d)

	eng.fetchOnce(context.Background(), sub)

	require.Len(t, dispatched, 1)
	require.Equal(t, "501", dispatched[0].TorrentID)
	require.Equal(t, "Ubuntu 24.04 LTS", dispatched[0].Title)
	require.Equal(t, int64(4700000000), dispatched[0].Size)
	require.Equal(t, "testsit", dispatched[0].SiteName)

	isSeen, err := eng.repo.IsSeen(context.Background(), "testsit", "501")
	require.NoError(t, err)
	require.True(t, isSeen)
}

func TestEngine_FetchOnce_AlreadySeen(t *testing.T) {
	eng, db := newEngineWithDB(t)
	makeSite(db, t, "testsit", "testsit.com")

	srv := serveRssWithItems(t, rssItem(
		"Seen Torrent",
		"https://testsit.com/download.php?id=600",
		"600",
		"bb222cc333dd444ee555ff666aa777bb88cc99dd",
		"2000000000",
	))

	sub := makeSub(db, t, "seen-sub", "testsit", []string{srv.URL})
	eng.fetcher = NewFetcherWithClient(srv.Client(), zap.NewNop())

	require.NoError(t, eng.repo.MarkSeen(context.Background(), &model.RSSTorrentSeen{
		SiteName: "testsit", TorrentID: "600", SubscriptionID: uintToString(sub.ID), Status: "seen",
	}))

	dispatchCount := 0
	d := event.NewDispatcher(zap.NewNop())
	d.Register("rss_new", &mocks.EventHandler{
		Fn: func(ctx context.Context, events []model.TorrentEvent) error {
			dispatchCount += len(events)
			return nil
		},
	})
	eng.SetDispatcher(d)

	eng.fetchOnce(context.Background(), sub)
	require.Equal(t, 0, dispatchCount)
}

func TestEngine_FetchOnce_FilterReject(t *testing.T) {
	eng, db := newEngineWithDB(t)
	makeSite(db, t, "testsit", "testsit.com")

	require.NoError(t, db.Create(&model.FilterRule{
		Name: "reject-adult", RuleType: "reject", Enabled: true, Priority: 1,
		Conditions: []model.RuleCondition{
			{Key: "title", CompareType: model.CompareContain, Value: "XXX"},
		},
	}).Error)
	require.NoError(t, db.Create(&model.FilterRule{
		Name: "accept-all", RuleType: "accept", Enabled: true, Priority: 2,
		Conditions: []model.RuleCondition{
			{Key: "title", CompareType: model.CompareRegExp, Value: ".*"},
		},
	}).Error)

	srv := serveRssWithItems(t, rssItem(
		"Bad XXX Content",
		"https://testsit.com/download.php?id=700",
		"700",
		"cc333dd444ee555ff666aa777bb88cc99ddaa00ee",
		"1000000000",
	))

	sub := makeSub(db, t, "reject-sub", "testsit", []string{srv.URL})
	eng.fetcher = NewFetcherWithClient(srv.Client(), zap.NewNop())
	eng.SetFilterEngine(filter.NewEngine(filter.NewRepository(db), zap.NewNop()))

	dispatchCount := 0
	d := event.NewDispatcher(zap.NewNop())
	d.Register("rss_new", &mocks.EventHandler{
		Fn: func(ctx context.Context, events []model.TorrentEvent) error {
			dispatchCount += len(events)
			return nil
		},
	})
	eng.SetDispatcher(d)

	eng.fetchOnce(context.Background(), sub)
	require.Equal(t, 0, dispatchCount)
}

func TestEngine_FetchOnce_SiteNotFound(t *testing.T) {
	eng, db := newEngineWithDB(t)

	srv := serveRssWithItems(t, rssItem(
		"Ghost Torrent",
		"https://nosite.com/download.php?id=800",
		"800",
		"dd444ee555ff666aa777bb88cc99ddaa00eebb11",
		"3000000000",
	))

	sub := makeSub(db, t, "nosite-sub", "nosite", []string{srv.URL})
	eng.fetcher = NewFetcherWithClient(srv.Client(), zap.NewNop())

	dispatchCount := 0
	d := event.NewDispatcher(zap.NewNop())
	d.Register("rss_new", &mocks.EventHandler{
		Fn: func(ctx context.Context, events []model.TorrentEvent) error {
			dispatchCount += len(events)
			return nil
		},
	})
	eng.SetDispatcher(d)

	eng.fetchOnce(context.Background(), sub)
	require.Equal(t, 0, dispatchCount)
}

func TestEngine_FetchOnce_FetchError(t *testing.T) {
	eng, db := newEngineWithDB(t)
	makeSite(db, t, "testsit", "testsit.com")

	sub := makeSub(db, t, "err-sub", "testsit", []string{"http://127.0.0.1:1/fail"})
	eng.fetcher = NewFetcher(zap.NewNop())

	dispatchCount := 0
	d := event.NewDispatcher(zap.NewNop())
	d.Register("rss_new", &mocks.EventHandler{
		Fn: func(ctx context.Context, events []model.TorrentEvent) error {
			dispatchCount += len(events)
			return nil
		},
	})
	eng.SetDispatcher(d)

	eng.fetchOnce(context.Background(), sub)
	require.Equal(t, 0, dispatchCount)
}

func TestEngine_FetchOnce_NoFilterMatch(t *testing.T) {
	eng, db := newEngineWithDB(t)
	makeSite(db, t, "testsit", "testsit.com")

	srv := serveRssWithItems(t, rssItem(
		"Windows 11 Pro",
		"https://testsit.com/download.php?id=900",
		"900",
		"ee555ff666aa777bb88cc99ddaa00eebb11cc22",
		"5000000000",
	))

	sub := makeSub(db, t, "nomatch-sub", "testsit", []string{srv.URL})
	eng.fetcher = NewFetcherWithClient(srv.Client(), zap.NewNop())

	dispatchCount := 0
	d := event.NewDispatcher(zap.NewNop())
	d.Register("rss_new", &mocks.EventHandler{
		Fn: func(ctx context.Context, events []model.TorrentEvent) error {
			dispatchCount += len(events)
			return nil
		},
	})
	eng.SetDispatcher(d)

	eng.fetchOnce(context.Background(), sub)
	require.Equal(t, 1, dispatchCount)
}

func TestEngine_FetchOnce_MultipleItems(t *testing.T) {
	eng, db := newEngineWithDB(t)
	makeSite(db, t, "testsit", "testsit.com")

	items := rssItem("Torrent A", "https://testsit.com/download.php?id=101", "101",
		"aaa111bbb222ccc333ddd444eee555fff666aaa1", "1000") +
		rssItem("Torrent B", "https://testsit.com/download.php?id=102", "102",
			"bbb222ccc333ddd444eee555fff666aaa111bbb2", "2000") +
		rssItem("Torrent C", "https://testsit.com/download.php?id=103", "103",
			"ccc333ddd444eee555fff666aaa111bbb222ccc3", "3000")

	srv := serveRssWithItems(t, items)
	sub := makeSub(db, t, "multi-sub", "testsit", []string{srv.URL})
	eng.fetcher = NewFetcherWithClient(srv.Client(), zap.NewNop())

	var dispatched []model.TorrentEvent
	d := event.NewDispatcher(zap.NewNop())
	d.Register("rss_new", &mocks.EventHandler{
		Fn: func(ctx context.Context, events []model.TorrentEvent) error {
			dispatched = append(dispatched, events...)
			return nil
		},
	})
	eng.SetDispatcher(d)

	eng.fetchOnce(context.Background(), sub)
	require.Len(t, dispatched, 3)
}

func TestEngine_FetchOnce_MultipleURLs(t *testing.T) {
	eng, db := newEngineWithDB(t)
	makeSite(db, t, "testsit", "testsit.com")

	srv1 := serveRssWithItems(t, rssItem("Feed1 Item", "https://testsit.com/download.php?id=201", "201",
		"ddd444eee555fff666aaa111bbb222ccc333ddd4", "1000"))
	srv2 := serveRssWithItems(t, rssItem("Feed2 Item", "https://testsit.com/download.php?id=202", "202",
		"eee555fff666aaa111bbb222ccc333ddd444eee5", "2000"))

	sub := makeSub(db, t, "multiurl-sub", "testsit", []string{srv1.URL, srv2.URL})
	client := srv1.Client()
	eng.fetcher = NewFetcherWithClient(client, zap.NewNop())

	var dispatched []model.TorrentEvent
	d := event.NewDispatcher(zap.NewNop())
	d.Register("rss_new", &mocks.EventHandler{
		Fn: func(ctx context.Context, events []model.TorrentEvent) error {
			dispatched = append(dispatched, events...)
			return nil
		},
	})
	eng.SetDispatcher(d)

	eng.fetchOnce(context.Background(), sub)
	require.Len(t, dispatched, 2)
}

func TestEngine_FetchOnce_HRSkip(t *testing.T) {
	eng, db := newEngineWithDB(t)
	makeSite(db, t, "testsit", "testsit.com")

	require.NoError(t, db.Create(&model.FilterRule{
		Name: "accept-all", RuleType: "accept", Enabled: true, Priority: 1,
		Conditions: []model.RuleCondition{
			{Key: "title", CompareType: model.CompareRegExp, Value: ".*"},
		},
	}).Error)

	adapter := &mocks.SiteAdapter{
		DetectHRFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.HRResult, error) {
			return &model.HRResult{HasHR: true, SeedTimeH: 72}, nil
		},
		DetectDiscountFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.DiscountResult, error) {
			return nil, nil
		},
	}
	eng.SetSiteProvider(&mocks.SiteInfoProvider{
		GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
			return adapter, nil
		},
		GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
			return &model.SiteConfig{HRStrategy: "skip"}, nil
		},
	})

	srv := serveRssWithItems(t, rssItem("HR Torrent", "https://testsit.com/download.php?id=301", "301",
		"fff666aaa111bbb222ccc333ddd444eee555fff6", "1000"))
	sub := makeSub(db, t, "hr-skip-sub", "testsit", []string{srv.URL})
	eng.fetcher = NewFetcherWithClient(srv.Client(), zap.NewNop())
	eng.SetFilterEngine(filter.NewEngine(filter.NewRepository(db), zap.NewNop()))

	dispatchCount := 0
	d := event.NewDispatcher(zap.NewNop())
	d.Register("rss_new", &mocks.EventHandler{
		Fn: func(ctx context.Context, events []model.TorrentEvent) error {
			dispatchCount += len(events)
			return nil
		},
	})
	eng.SetDispatcher(d)

	eng.fetchOnce(context.Background(), sub)
	require.Equal(t, 0, dispatchCount)
}

func TestEngine_FetchOnce_HRIgnore(t *testing.T) {
	eng, db := newEngineWithDB(t)
	makeSite(db, t, "testsit", "testsit.com")

	adapter := &mocks.SiteAdapter{
		DetectHRFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.HRResult, error) {
			return &model.HRResult{HasHR: true, SeedTimeH: 72}, nil
		},
		DetectDiscountFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.DiscountResult, error) {
			return nil, nil
		},
	}
	eng.SetSiteProvider(&mocks.SiteInfoProvider{
		GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
			return adapter, nil
		},
		GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
			return &model.SiteConfig{HRStrategy: "ignore"}, nil
		},
	})

	srv := serveRssWithItems(t, rssItem("HR Torrent", "https://testsit.com/download.php?id=302", "302",
		"aaa777bbb888ccc999ddd000aaa111bbb222ccc3", "1000"))
	sub := makeSub(db, t, "hr-ignore-sub", "testsit", []string{srv.URL})
	eng.fetcher = NewFetcherWithClient(srv.Client(), zap.NewNop())

	var dispatched []model.TorrentEvent
	d := event.NewDispatcher(zap.NewNop())
	d.Register("rss_new", &mocks.EventHandler{
		Fn: func(ctx context.Context, events []model.TorrentEvent) error {
			dispatched = append(dispatched, events...)
			return nil
		},
	})
	eng.SetDispatcher(d)

	eng.fetchOnce(context.Background(), sub)
	require.Len(t, dispatched, 1)
	require.True(t, dispatched[0].HasHR)
	require.Equal(t, 72, dispatched[0].HRSeedTimeH)
}

func TestEngine_FetchOnce_NoDispatcher(t *testing.T) {
	eng, db := newEngineWithDB(t)
	makeSite(db, t, "testsit", "testsit.com")

	srv := serveRssWithItems(t, rssItem("Some Torrent", "https://testsit.com/download.php?id=401", "401",
		"bbb888ccc999ddd000aaa111bbb222ccc333ddd4", "1000"))
	sub := makeSub(db, t, "nodispatch-sub", "testsit", []string{srv.URL})
	eng.fetcher = NewFetcherWithClient(srv.Client(), zap.NewNop())

	require.NotPanics(t, func() {
		eng.fetchOnce(context.Background(), sub)
	})

	isSeen, err := eng.repo.IsSeen(context.Background(), "testsit", "401")
	require.NoError(t, err)
	require.True(t, isSeen)
}

func TestEngine_FetchOnce_FreeTorrent(t *testing.T) {
	eng, db := newEngineWithDB(t)
	makeSite(db, t, "testsit", "testsit.com")

	freeEnd := time.Now().Add(24 * time.Hour)
	adapter := &mocks.SiteAdapter{
		DetectHRFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.HRResult, error) {
			return &model.HRResult{HasHR: false}, nil
		},
		DetectDiscountFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.DiscountResult, error) {
			return &model.DiscountResult{Level: model.DiscountFree, FreeEndAt: &freeEnd}, nil
		},
	}
	eng.SetSiteProvider(&mocks.SiteInfoProvider{
		GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
			return adapter, nil
		},
		GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
			return &model.SiteConfig{}, nil
		},
	})

	srv := serveRssWithItems(t, rssItem("Free Torrent", "https://testsit.com/download.php?id=402", "402",
		"ccc999ddd000aaa111bbb222ccc333ddd444eee5", "1000"))
	sub := makeSub(db, t, "free-sub", "testsit", []string{srv.URL})
	eng.fetcher = NewFetcherWithClient(srv.Client(), zap.NewNop())

	var dispatched []model.TorrentEvent
	d := event.NewDispatcher(zap.NewNop())
	d.Register("rss_new", &mocks.EventHandler{
		Fn: func(ctx context.Context, events []model.TorrentEvent) error {
			dispatched = append(dispatched, events...)
			return nil
		},
	})
	eng.SetDispatcher(d)

	eng.fetchOnce(context.Background(), sub)
	require.Len(t, dispatched, 1)
	require.Equal(t, model.DiscountFree, dispatched[0].Discount)
	require.NotNil(t, dispatched[0].FreeEndAt)
}

func TestEngine_FetchOnce_MixedSeenAndNew(t *testing.T) {
	eng, db := newEngineWithDB(t)
	makeSite(db, t, "testsit", "testsit.com")

	items := rssItem("Old Torrent", "https://testsit.com/download.php?id=501", "501",
		"ddd000aaa111bbb222ccc333ddd444eee555fff6", "1000") +
		rssItem("New Torrent", "https://testsit.com/download.php?id=502", "502",
			"eee111bbb222ccc333ddd444eee555fff666aaa7", "2000")

	srv := serveRssWithItems(t, items)
	sub := makeSub(db, t, "mix-sub", "testsit", []string{srv.URL})
	eng.fetcher = NewFetcherWithClient(srv.Client(), zap.NewNop())

	require.NoError(t, eng.repo.MarkSeen(context.Background(), &model.RSSTorrentSeen{
		SiteName: "testsit", TorrentID: "501", SubscriptionID: uintToString(sub.ID), Status: "seen",
	}))

	var dispatched []model.TorrentEvent
	d := event.NewDispatcher(zap.NewNop())
	d.Register("rss_new", &mocks.EventHandler{
		Fn: func(ctx context.Context, events []model.TorrentEvent) error {
			dispatched = append(dispatched, events...)
			return nil
		},
	})
	eng.SetDispatcher(d)

	eng.fetchOnce(context.Background(), sub)
	require.Len(t, dispatched, 1)
	require.Equal(t, "502", dispatched[0].TorrentID)
}

type mockProviderWithDownloader struct {
	*mocks.SiteInfoProvider
	GetDownloaderClientFn func(ctx context.Context, clientID string) (model.DownloaderClient, error)
}

func (m *mockProviderWithDownloader) GetDownloaderClient(ctx context.Context, clientID string) (model.DownloaderClient, error) {
	if m.GetDownloaderClientFn != nil {
		return m.GetDownloaderClientFn(ctx, clientID)
	}
	return nil, nil
}

func TestEngine_CheckDiskBudget_NilProvider(t *testing.T) {
	eng, _ := newEngineWithDB(t)
	sub := &model.RSSSubscription{ClientID: "client1"}
	err := eng.checkDiskBudget(context.Background(), sub, 1000)
	require.NoError(t, err)
}

func TestEngine_CheckDiskBudget_SufficientSpace(t *testing.T) {
	eng, _ := newEngineWithDB(t)
	provider := &mockProviderWithDownloader{
		SiteInfoProvider: &mocks.SiteInfoProvider{},
		GetDownloaderClientFn: func(ctx context.Context, clientID string) (model.DownloaderClient, error) {
			return &mocks.DownloaderClient{
				GetMainDataFn: func(ctx context.Context) (*model.Maindata, error) {
					return &model.Maindata{FreeSpace: 100 * 1024 * 1024 * 1024}, nil
				},
			}, nil
		},
	}
	eng.SetSiteProvider(provider)

	sub := &model.RSSSubscription{ClientID: "client1", DiskBudgetMinGB: 10}
	err := eng.checkDiskBudget(context.Background(), sub, 1024*1024*1024)
	require.NoError(t, err)
}

func TestEngine_CheckDiskBudget_InsufficientSpace(t *testing.T) {
	eng, _ := newEngineWithDB(t)
	provider := &mockProviderWithDownloader{
		SiteInfoProvider: &mocks.SiteInfoProvider{},
		GetDownloaderClientFn: func(ctx context.Context, clientID string) (model.DownloaderClient, error) {
			return &mocks.DownloaderClient{
				GetMainDataFn: func(ctx context.Context) (*model.Maindata, error) {
					return &model.Maindata{FreeSpace: 5 * 1024 * 1024 * 1024}, nil
				},
			}, nil
		},
	}
	eng.SetSiteProvider(provider)

	sub := &model.RSSSubscription{ClientID: "client1", DiskBudgetMinGB: 10}
	err := eng.checkDiskBudget(context.Background(), sub, 4*1024*1024*1024)
	require.Error(t, err)
}

func TestEngine_CheckDiskBudget_GetClientError(t *testing.T) {
	eng, _ := newEngineWithDB(t)
	provider := &mockProviderWithDownloader{
		SiteInfoProvider: &mocks.SiteInfoProvider{},
		GetDownloaderClientFn: func(ctx context.Context, clientID string) (model.DownloaderClient, error) {
			return nil, fmt.Errorf("client not found")
		},
	}
	eng.SetSiteProvider(provider)

	sub := &model.RSSSubscription{ClientID: "client1"}
	err := eng.checkDiskBudget(context.Background(), sub, 1000)
	require.Error(t, err)
	require.Contains(t, err.Error(), "获取下载器失败")
}

func TestEngine_CheckDiskGuard_Disabled(t *testing.T) {
	eng, _ := newEngineWithDB(t)
	sub := &model.RSSSubscription{ClientID: "client1", DiskGuardEnabled: false, DiskGuardThreshold: 1073741824}
	err := eng.checkDiskGuard(context.Background(), sub)
	require.NoError(t, err)
}

func TestEngine_CheckDiskGuard_SufficientSpace(t *testing.T) {
	eng, _ := newEngineWithDB(t)
	provider := &mockProviderWithDownloader{
		SiteInfoProvider: &mocks.SiteInfoProvider{},
		GetDownloaderClientFn: func(ctx context.Context, clientID string) (model.DownloaderClient, error) {
			return &mocks.DownloaderClient{
				GetMainDataFn: func(ctx context.Context) (*model.Maindata, error) {
					return &model.Maindata{FreeSpace: 50 * 1024 * 1024 * 1024}, nil
				},
			}, nil
		},
	}
	eng.SetSiteProvider(provider)

	sub := &model.RSSSubscription{ClientID: "client1", DiskGuardEnabled: true, DiskGuardThreshold: 1073741824}
	err := eng.checkDiskGuard(context.Background(), sub)
	require.NoError(t, err)
}

func TestEngine_CheckDiskGuard_InsufficientSpace(t *testing.T) {
	eng, _ := newEngineWithDB(t)
	provider := &mockProviderWithDownloader{
		SiteInfoProvider: &mocks.SiteInfoProvider{},
		GetDownloaderClientFn: func(ctx context.Context, clientID string) (model.DownloaderClient, error) {
			return &mocks.DownloaderClient{
				GetMainDataFn: func(ctx context.Context) (*model.Maindata, error) {
					return &model.Maindata{FreeSpace: 500 * 1024 * 1024}, nil
				},
			}, nil
		},
	}
	eng.SetSiteProvider(provider)

	sub := &model.RSSSubscription{ClientID: "client1", DiskGuardEnabled: true, DiskGuardThreshold: 1073741824}
	err := eng.checkDiskGuard(context.Background(), sub)
	require.Error(t, err)
	require.Contains(t, err.Error(), "磁盘守卫拦截")
}

func TestEngine_CheckDiskGuard_GetClientError(t *testing.T) {
	eng, _ := newEngineWithDB(t)
	provider := &mockProviderWithDownloader{
		SiteInfoProvider: &mocks.SiteInfoProvider{},
		GetDownloaderClientFn: func(ctx context.Context, clientID string) (model.DownloaderClient, error) {
			return nil, fmt.Errorf("client not found")
		},
	}
	eng.SetSiteProvider(provider)

	sub := &model.RSSSubscription{ClientID: "client1", DiskGuardEnabled: true, DiskGuardThreshold: 1073741824}
	err := eng.checkDiskGuard(context.Background(), sub)
	require.Error(t, err)
	require.Contains(t, err.Error(), "磁盘守卫：获取下载器失败")
}

func TestEngine_CheckDiskGuard_NilProvider(t *testing.T) {
	eng, _ := newEngineWithDB(t)
	sub := &model.RSSSubscription{ClientID: "client1", DiskGuardEnabled: true, DiskGuardThreshold: 1073741824}
	err := eng.checkDiskGuard(context.Background(), sub)
	require.NoError(t, err)
}

func TestEngine_CheckDiskGuard_ZeroThreshold(t *testing.T) {
	eng, _ := newEngineWithDB(t)
	provider := &mockProviderWithDownloader{
		SiteInfoProvider: &mocks.SiteInfoProvider{},
		GetDownloaderClientFn: func(ctx context.Context, clientID string) (model.DownloaderClient, error) {
			return &mocks.DownloaderClient{
				GetMainDataFn: func(ctx context.Context) (*model.Maindata, error) {
					return &model.Maindata{FreeSpace: 0}, nil
				},
			}, nil
		},
	}
	eng.SetSiteProvider(provider)

	sub := &model.RSSSubscription{ClientID: "client1", DiskGuardEnabled: true, DiskGuardThreshold: 0}
	err := eng.checkDiskGuard(context.Background(), sub)
	require.NoError(t, err)
}

func TestEngine_FetchOnce_DiskBudgetSkip(t *testing.T) {
	eng, db := newEngineWithDB(t)
	makeSite(db, t, "testsit", "testsit.com")

	require.NoError(t, db.Create(&model.FilterRule{
		Name: "accept-all", RuleType: "accept", Enabled: true, Priority: 1,
		Conditions: []model.RuleCondition{
			{Key: "title", CompareType: model.CompareRegExp, Value: ".*"},
		},
	}).Error)

	provider := &mockProviderWithDownloader{
		SiteInfoProvider: &mocks.SiteInfoProvider{
			GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
				return &mocks.SiteAdapter{
					DetectHRFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.HRResult, error) {
						return &model.HRResult{HasHR: false}, nil
					},
					DetectDiscountFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.DiscountResult, error) {
						return &model.DiscountResult{Level: model.DiscountNone}, nil
					},
				}, nil
			},
			GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
				return &model.SiteConfig{}, nil
			},
		},
		GetDownloaderClientFn: func(ctx context.Context, clientID string) (model.DownloaderClient, error) {
			return &mocks.DownloaderClient{
				GetMainDataFn: func(ctx context.Context) (*model.Maindata, error) {
					return &model.Maindata{FreeSpace: 1 * 1024 * 1024 * 1024}, nil
				},
			}, nil
		},
	}
	eng.SetSiteProvider(provider)

	srv := serveRssWithItems(t, rssItem("Big Torrent", "https://testsit.com/download.php?id=601", "601",
		"fff000aaa111bbb222ccc333ddd444eee555fff0", "5000000000"))
	sub := makeSub(db, t, "disk-sub", "testsit", []string{srv.URL})
	sub.ClientID = "client1"
	sub.DiskBudgetEnabled = true
	sub.DiskBudgetMinGB = 10
	require.NoError(t, db.Save(sub).Error)

	eng.fetcher = NewFetcherWithClient(srv.Client(), zap.NewNop())
	eng.SetFilterEngine(filter.NewEngine(filter.NewRepository(db), zap.NewNop()))

	dispatchCount := 0
	d := event.NewDispatcher(zap.NewNop())
	d.Register("rss_new", &mocks.EventHandler{
		Fn: func(ctx context.Context, events []model.TorrentEvent) error {
			dispatchCount += len(events)
			return nil
		},
	})
	eng.SetDispatcher(d)

	eng.fetchOnce(context.Background(), sub)
	require.Equal(t, 0, dispatchCount)
}

func TestEngine_FetchOnce_CancelledContext(t *testing.T) {
	eng, db := newEngineWithDB(t)
	makeSite(db, t, "testsit", "testsit.com")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	sub := makeSub(db, t, "cancel-sub", "testsit", []string{"https://testsit.com/rss"})

	require.NotPanics(t, func() {
		eng.fetchOnce(ctx, sub)
	})
}

func TestDerefStr(t *testing.T) {
	require.Equal(t, "", derefStr(nil))
	s := "hello"
	require.Equal(t, "hello", derefStr(&s))
}

func TestUintToString(t *testing.T) {
	require.Equal(t, "0", uintToString(0))
	require.Equal(t, "42", uintToString(42))
	require.Equal(t, "100", uintToString(100))
}

func TestDiscountLevel_FallbackLogic(t *testing.T) {
	evt := &model.RSSTorrentEvent{TorrentID: "1", SiteName: "test"}

	discountStr := string(evt.DiscountLevel)
	require.Equal(t, "", discountStr)

	evt.IsFree = true
	discountStr = string(evt.DiscountLevel)
	if discountStr == "" {
		if evt.IsFree {
			discountStr = string(model.DiscountFree)
		} else {
			discountStr = string(model.DiscountNone)
		}
	}
	require.Equal(t, string(model.DiscountFree), discountStr)

	evt.IsFree = false
	evt.DiscountLevel = model.Discount2xFree
	discountStr = string(evt.DiscountLevel)
	require.Equal(t, string(model.Discount2xFree), discountStr)
}

func TestDiscountLevel_DetectHRAndDiscount_SetsLevel(t *testing.T) {
	eng, _ := newEngineWithDB(t)
	freeEndAt := time.Now().Add(24 * time.Hour)
	adapter := &mocks.SiteAdapter{
		DetectHRFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.HRResult, error) {
			return &model.HRResult{HasHR: false}, nil
		},
		DetectDiscountFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.DiscountResult, error) {
			return &model.DiscountResult{
				Level:     model.Discount2xUp,
				FreeEndAt: &freeEndAt,
			}, nil
		},
	}
	eng.SetSiteProvider(&mocks.SiteInfoProvider{
		GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
			return adapter, nil
		},
		GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
			return &model.SiteConfig{}, nil
		},
	})

	evt := &model.RSSTorrentEvent{TorrentID: "42", SiteName: "site"}
	eng.detectHRAndDiscount(context.Background(), evt, "site")

	require.Equal(t, model.Discount2xUp, evt.DiscountLevel)
}

func TestEngine_FetchOnce_RegexReplacement(t *testing.T) {
	eng, db := newEngineWithDB(t)
	makeSite(db, t, "regexsite", "regexsite.com")

	srv := serveRssWithItems(t, rssItem(
		"[GroupX] Some Title [1080p]",
		"https://regexsite.com/download.php?id=701",
		"701",
		"cc333dd444ee555ff666aa777bb88cc99dd00",
		"3000000000",
	))

	sub := makeSub(db, t, "regex-sub", "regexsite", []string{srv.URL})
	sub.UseCustomRegex = true
	sub.RegexStr = `^\[.*?\]\s*`
	sub.ReplaceStr = ""
	require.NoError(t, db.Save(sub).Error)

	eng.fetcher = NewFetcherWithClient(srv.Client(), zap.NewNop())

	var dispatched []model.TorrentEvent
	d := event.NewDispatcher(zap.NewNop())
	d.Register("rss_new", &mocks.EventHandler{
		Fn: func(ctx context.Context, events []model.TorrentEvent) error {
			dispatched = append(dispatched, events...)
			return nil
		},
	})
	eng.SetDispatcher(d)

	eng.fetchOnce(context.Background(), sub)

	require.Len(t, dispatched, 1)
	require.Equal(t, "Some Title [1080p]", dispatched[0].Title)
}

func TestEngine_FetchOnce_RegexReplacement_InvalidRegex(t *testing.T) {
	eng, db := newEngineWithDB(t)
	makeSite(db, t, "regexsite2", "regexsite2.com")

	srv := serveRssWithItems(t, rssItem(
		"[GroupX] Title",
		"https://regexsite2.com/download.php?id=702",
		"702",
		"dd444ee555ff666aa777bb88cc99dd00ee11",
		"2000000000",
	))

	sub := makeSub(db, t, "regex-sub2", "regexsite2", []string{srv.URL})
	sub.UseCustomRegex = true
	sub.RegexStr = "[invalid"
	sub.ReplaceStr = ""
	require.NoError(t, db.Save(sub).Error)

	eng.fetcher = NewFetcherWithClient(srv.Client(), zap.NewNop())

	var dispatched []model.TorrentEvent
	d := event.NewDispatcher(zap.NewNop())
	d.Register("rss_new", &mocks.EventHandler{
		Fn: func(ctx context.Context, events []model.TorrentEvent) error {
			dispatched = append(dispatched, events...)
			return nil
		},
	})
	eng.SetDispatcher(d)

	eng.fetchOnce(context.Background(), sub)

	require.Len(t, dispatched, 1)
	require.Equal(t, "[GroupX] Title", dispatched[0].Title)
}

func TestEngine_FetchOnce_AcceptRejectRules(t *testing.T) {
	eng, db := newEngineWithDB(t)
	makeSite(db, t, "rulesite", "rulesite.com")

	acceptRule := &model.FilterRule{
		Name:      "accept-free",
		RuleType:  "accept",
		Enabled:   true,
		Conditions: []model.RuleCondition{
			{Key: "title", CompareType: model.CompareContain, Value: "Ubuntu"},
		},
	}
	require.NoError(t, db.Create(acceptRule).Error)

	rejectRule := &model.FilterRule{
		Name:      "reject-spam",
		RuleType:  "reject",
		Enabled:   true,
		Conditions: []model.RuleCondition{
			{Key: "title", CompareType: model.CompareContain, Value: "spam"},
		},
	}
	require.NoError(t, db.Create(rejectRule).Error)

	srv := serveRssWithItems(t,
		rssItem("Ubuntu 24.04 LTS", "https://rulesite.com/dl?id=801", "801", "ee555ff666aa777bb88cc99dd00ee11ff22", "1000000000")+
			rssItem("Spam Content", "https://rulesite.com/dl?id=802", "802", "ff666aa777bb88cc99dd00ee11ff22aa33", "2000000000")+
			rssItem("Other Content", "https://rulesite.com/dl?id=803", "803", "aa777bb88cc99dd00ee11ff22aa33bb44", "3000000000"),
	)

	sub := makeSub(db, t, "rule-sub", "rulesite", []string{srv.URL})
	sub.AcceptRuleIDs = []uint{acceptRule.ID}
	sub.RejectRuleIDs = []uint{rejectRule.ID}
	require.NoError(t, db.Save(sub).Error)

	filterEng := filter.NewEngine(filter.NewRepository(db), zap.NewNop())
	eng.filterEng = filterEng
	eng.fetcher = NewFetcherWithClient(srv.Client(), zap.NewNop())

	var dispatched []model.TorrentEvent
	d := event.NewDispatcher(zap.NewNop())
	d.Register("rss_new", &mocks.EventHandler{
		Fn: func(ctx context.Context, events []model.TorrentEvent) error {
			dispatched = append(dispatched, events...)
			return nil
		},
	})
	eng.SetDispatcher(d)

	eng.fetchOnce(context.Background(), sub)

	require.Len(t, dispatched, 1)
	require.Equal(t, "Ubuntu 24.04 LTS", dispatched[0].Title)
	require.Equal(t, "801", dispatched[0].TorrentID)
}
