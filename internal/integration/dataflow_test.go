package integration

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/mocks"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/publish"
	"github.com/ranfish/pt-forward/internal/reseed"
	"github.com/ranfish/pt-forward/internal/rss"
	"github.com/ranfish/pt-forward/internal/seeding"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestTorrentData(t *testing.T) []byte {
	t.Helper()
	pieces := make([]byte, 20)
	info := "d4:name8:test.txt6:lengthi1024e12:piece lengthi16384e6:pieces20:" + string(pieces) + "e"
	return []byte("d4:info" + info + "e")
}

// ── F1: RSS 抓取 → 解析 → 分发 ──────────────────────────────────

func TestScenario_F1_FetchParseDispatch(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	site := &model.Site{
		Name: "source-site", Domain: "source.com",
		BaseURL: "https://source.com", AuthType: "cookie",
		HashStrategy: "guid", SizeStrategy: "enclosure", IDStrategy: "query_param",
	}
	require.NoError(t, db.Create(site).Error)

	seedClient(t, db, "seeding-client", "seeding")

	sub := &model.RSSSubscription{
		Name: "f1-sub", SiteName: "source-site",
		URLs: []string{"https://source.com/rss"}, Enabled: true,
		ClientID: "seeding-client",
	}
	require.NoError(t, db.Create(sub).Error)

	rssXML := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel><title>Test</title><link>https://source.com</link>
<item><title>Ubuntu 24.04 LTS</title>
<link>https://source.com/download.php?id=501</link>
<guid>aa111bb222cc333dd444ee555ff666aa777bb88</guid>
<enclosure url="https://source.com/download.php?id=501" length="4700000000" type="application/x-bittorrent"/>
</item></channel></rss>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(rssXML))
	}))
	defer srv.Close()

	fetcher := rss.NewFetcherWithClient(srv.Client(), nopLogger())
	feed, err := fetcher.Fetch(ctx, srv.URL)
	require.NoError(t, err)
	require.Len(t, feed.Channel.Items, 1)

	events := fetcher.ParseItems(feed, sub, site)
	require.Len(t, events, 1)
	assert.Equal(t, "source-site", events[0].SiteName)
	assert.Equal(t, "Ubuntu 24.04 LTS", events[0].Title)
	t.Logf("F1 step1: parse OK TorrentID=%s InfoHash=%s Size=%d",
		events[0].TorrentID, events[0].InfoHash, events[0].Size)

	assert.NotEmpty(t, events[0].TorrentID)
	assert.NotEmpty(t, events[0].InfoHash)

	t.Logf("PASS F1: RSS fetch→parse OK TorrentID=%s InfoHash=%s",
		events[0].TorrentID, events[0].InfoHash)
}

func TestScenario_F1_MultipleItems(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	site := &model.Site{
		Name: "multi-site", Domain: "multi.com",
		BaseURL: "https://multi.com", AuthType: "cookie",
		HashStrategy: "guid", SizeStrategy: "enclosure", IDStrategy: "query_param",
	}
	require.NoError(t, db.Create(site).Error)

	seedClient(t, db, "seeding-client", "seeding")

	sub := &model.RSSSubscription{
		Name: "f1-multi", SiteName: "multi-site",
		URLs: []string{"https://multi.com/rss"}, Enabled: true,
		ClientID: "seeding-client",
	}
	require.NoError(t, db.Create(sub).Error)

	rssXML := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel><title>Multi</title><link>https://multi.com</link>
<item><title>Torrent A</title>
<link>https://multi.com/dl?id=101</link><guid>aaa111bbb222ccc333dd444ee555ff666aa777bb1</guid>
<enclosure url="https://multi.com/dl?id=101" length="1000" type="application/x-bittorrent"/></item>
<item><title>Torrent B</title>
<link>https://multi.com/dl?id=102</link><guid>aaa111bbb222ccc333dd444ee555ff666aa777bb2</guid>
<enclosure url="https://multi.com/dl?id=102" length="2000" type="application/x-bittorrent"/></item>
<item><title>Torrent C</title>
<link>https://multi.com/dl?id=103</link><guid>aaa111bbb222ccc333dd444ee555ff666aa777bb3</guid>
<enclosure url="https://multi.com/dl?id=103" length="3000" type="application/x-bittorrent"/></item>
</channel></rss>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(rssXML))
	}))
	defer srv.Close()

	fetcher := rss.NewFetcherWithClient(srv.Client(), nopLogger())
	feed, err := fetcher.Fetch(ctx, srv.URL)
	require.NoError(t, err)

	events := fetcher.ParseItems(feed, sub, site)
	require.Len(t, events, 3)

	titles := make(map[string]bool)
	for _, ev := range events {
		titles[ev.Title] = true
	}
	assert.True(t, titles["Torrent A"])
	assert.True(t, titles["Torrent B"])
	assert.True(t, titles["Torrent C"])

	t.Logf("PASS F1: multiple items parsed=%d", len(events))
}

// ── F4: 刷流推送 → Downloader ────────────────────────────────────

func TestScenario_F4_FlushPushToDownloader(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	site := &model.Site{
		Name: "f4-site", Domain: "f4.com",
		BaseURL: "https://f4.com", AuthType: "cookie",
	}
	require.NoError(t, db.Create(site).Error)

	seedClient(t, db, "seeding-client", "seeding")

	sub := &model.RSSSubscription{
		Name: "f4-sub", SiteName: "f4-site",
		URLs: []string{"https://f4.com/rss"}, Enabled: true,
		ClientID: "seeding-client",
		ScoringConfig: model.SeedingScoringConfig{
			MaxCandidates:    10,
			MinScore:         -1,
			BatchLimit:       5,
			MaxActiveSeeding: 100,
		},
	}
	require.NoError(t, db.Create(sub).Error)
	db.Model(sub).Update("min_score", 0.0)
	subID := fmt.Sprintf("%d", sub.ID)

	record := &model.SeedingTorrentRecord{
		ClientID:       "seeding-client",
		TorrentID:      "free-001",
		InfoHash:       "aa111bb222cc333dd444ee555ff666aa777bb88",
		SiteName:       "f4-site",
		SubscriptionID: subID,
		Status:         model.SeedingStatusSeeding,
		Source:         "rss",
		IsFree:         true,
		Discount:       model.DiscountFree,
	}
	require.NoError(t, db.Create(record).Error)

	var addFromFileCalled int32

	mockDLClient := &mocks.DownloaderClient{
		GetMainDataFn: func(ctx context.Context) (*model.Maindata, error) {
			return &model.Maindata{FreeSpace: 100 * 1024 * 1024 * 1024}, nil
		},
		AddFromFileFn: func(ctx context.Context, data []byte, opts model.AddTorrentOptions) (*model.AddResult, error) {
			atomic.AddInt32(&addFromFileCalled, 1)
			return &model.AddResult{InfoHash: "aa111bb222cc333dd444ee555ff666aa777bb88"}, nil
		},
		CheckExistsFn: func(ctx context.Context, infoHash string) (bool, error) {
			return false, nil
		},
	}

	mockDLProvider := &mocks.DownloaderProvider{
		GetFn: func(clientID string) (model.DownloaderClient, error) {
			return mockDLClient, nil
		},
	}

	torrentData := createTestTorrentData(t)
	mockSiteProvider := &mocks.SiteInfoProvider{
		GetSiteInfoFn: func(ctx context.Context, siteName string) (*model.SiteInfo, error) {
			return &model.SiteInfo{Name: siteName, Enabled: true}, nil
		},
		GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
			return &model.SiteConfig{Domain: domain}, nil
		},
		GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
			return &mocks.SiteAdapter{
				DownloadTorrentFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error) {
					return torrentData, nil
				},
				DetectDiscountFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.DiscountResult, error) {
					return &model.DiscountResult{Level: model.DiscountFree}, nil
				},
			}, nil
		},
	}

	eng := seeding.NewEngine(db, nopLogger())
	eng.SetClientProvider(mockDLProvider)
	eng.SetSiteProvider(mockSiteProvider)
	require.NoError(t, eng.Start(ctx))

	var dbRecords []model.SeedingTorrentRecord
	db.Where("client_id = ? AND status = ? AND source = ?", "seeding-client", model.SeedingStatusSeeding, "rss").Find(&dbRecords)
	t.Logf("F4 debug: db_records=%d discount=%s isFree=%v", len(dbRecords), dbRecords[0].Discount, dbRecords[0].IsFree)

	var dbSub model.RSSSubscription
	require.NoError(t, db.First(&dbSub).Error)
	t.Logf("F4 debug: sub.ID=%d Enabled=%v ClientID=%q ScoringCfg.MaxCandidates=%d ScoringCfg.MaxActiveSeeding=%d",
		dbSub.ID, dbSub.Enabled, dbSub.ClientID, dbSub.ScoringConfig.MaxCandidates, dbSub.ScoringConfig.MaxActiveSeeding)

	results, err := eng.Flush(ctx, fmt.Sprintf("%d", sub.ID))
	require.NoError(t, err)
	t.Logf("F4 debug: results=%d addFromFile=%d", len(results), atomic.LoadInt32(&addFromFileCalled))
	require.NotEmpty(t, results, "Flush should return at least one result")
	assert.Equal(t, int32(1), atomic.LoadInt32(&addFromFileCalled))

	t.Logf("PASS F4: flush results=%d addFromFile_called=%d", len(results), atomic.LoadInt32(&addFromFileCalled))
}

func TestScenario_F4_SkipsNonFree(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	site := &model.Site{
		Name: "f4skip-site", Domain: "f4skip.com",
		BaseURL: "https://f4skip.com", AuthType: "cookie",
	}
	require.NoError(t, db.Create(site).Error)

	seedClient(t, db, "seeding-client", "seeding")

	sub := &model.RSSSubscription{
		Name: "f4skip-sub", SiteName: "f4skip-site",
		URLs: []string{"https://f4skip.com/rss"}, Enabled: true,
		ClientID: "seeding-client",
		ScoringConfig: model.SeedingScoringConfig{
			MaxCandidates:    10,
			MinScore:         0,
			BatchLimit:       5,
			MaxActiveSeeding: 100,
		},
	}
	require.NoError(t, db.Create(sub).Error)

	nonFreeRecord := &model.SeedingTorrentRecord{
		ClientID:  "seeding-client",
		TorrentID: "nonfree-001",
		InfoHash:  "bb111bb222cc333dd444ee555ff666aa777bb99",
		SiteName:  "f4skip-site",
		Status:    model.SeedingStatusSeeding,
		Source:    "rss",
		IsFree:    false,
		Discount:  model.DiscountNone,
	}
	require.NoError(t, db.Create(nonFreeRecord).Error)

	mockDLClient := &mocks.DownloaderClient{
		GetMainDataFn: func(ctx context.Context) (*model.Maindata, error) {
			return &model.Maindata{FreeSpace: 100 * 1024 * 1024 * 1024}, nil
		},
	}

	mockDLProvider := &mocks.DownloaderProvider{
		GetFn: func(clientID string) (model.DownloaderClient, error) {
			return mockDLClient, nil
		},
	}

	eng := seeding.NewEngine(db, nopLogger())
	eng.SetClientProvider(mockDLProvider)

	results, err := eng.Flush(ctx, fmt.Sprintf("%d", sub.ID))
	require.NoError(t, err)
	assert.Len(t, results, 0)

	t.Logf("PASS F4-skip: non-free filtered, results=%d", len(results))
}

// ── F5: 转载发布 全链路 ──────────────────────────────────────────

func TestScenario_F5_PublishE2E(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	sourceSite := &model.Site{
		Name: "f5-source", Domain: "f5source.com",
		BaseURL: "https://f5source.com", AuthType: "cookie",
	}
	require.NoError(t, db.Create(sourceSite).Error)

	targetSite := &model.Site{
		Name: "f5-target", Domain: "f5target.com",
		BaseURL: "https://f5target.com", AuthType: "cookie",
	}
	require.NoError(t, db.Create(targetSite).Error)

	clientID := seedClient(t, db, "dl-client", "download")
	_ = clientID

	candidate := &model.PublishCandidate{
		SourceSite:      "f5-source",
		SourceTorrentID: "src-001",
		TorrentName:     "Clean Release 2024",
		Size:            1024,
		InfoHash:        "aa111bb222cc333dd444ee555ff666aa777bb88",
		PublishStatus:   "pending",
		TargetSites:     `["f5-target"]`,
	}
	require.NoError(t, db.Create(candidate).Error)

	var uploadCalled int32
	siteProvider := &mocks.SiteInfoProvider{
		GetSiteInfoFn: func(ctx context.Context, siteName string) (*model.SiteInfo, error) {
			return &model.SiteInfo{Name: siteName, Enabled: true}, nil
		},
		GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
			return &model.SiteConfig{Domain: domain}, nil
		},
		GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
			return &mocks.SiteAdapter{
				DownloadTorrentFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error) {
					return []byte("d8:announce30:http://tracker.example.com/announcee"), nil
				},
				GetTorrentDetailFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.TorrentDetail, error) {
					return &model.TorrentDetail{
						Title: "Test Torrent", Category: "movie",
						Source: "BluRay", Resolution: "2160p", Codec: "x264",
						ReleaseGroup: "GROUP", Description: "test desc",
					}, nil
				},
				SearchTorrentsFn: func(ctx context.Context, config *model.SiteConfig, query string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
					return nil, nil
				},
				UploadTorrentFn: func(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
					atomic.AddInt32(&uploadCalled, 1)
					return &model.PublishResponse{
						TorrentID: "pub-001",
						DetailURL: "https://f5target.com/torrents/pub-001",
					}, nil
				},
			}, nil
		},
	}

	pipeline := publish.NewPipeline(db, nopLogger())
	pipeline.SetSiteProvider(siteProvider)

	result, err := pipeline.PublishCandidate(ctx, candidate.ID)
	require.NoError(t, err)
	require.NotNil(t, result)

	var results []model.PublishResultRecord
	db.Find(&results)
	assert.True(t, len(results) > 0)

	t.Logf("PASS F5: publish E2E candidate_id=%d upload_called=%v results=%d",
		candidate.ID, atomic.LoadInt32(&uploadCalled) > 0, len(results))
}

// ── F7: 辅种引擎 Layer1 InfoHash 匹配 ────────────────────────────

func TestScenario_F7_Layer1InfoHashMatch(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	sourceSite := &model.Site{
		Name: "f7-source", Domain: "f7source.com",
		BaseURL: "https://f7source.com", AuthType: "cookie",
	}
	require.NoError(t, db.Create(sourceSite).Error)

	targetSite := &model.Site{
		Name: "f7-target", Domain: "f7target.com",
		BaseURL: "https://f7target.com", AuthType: "cookie",
	}
	require.NoError(t, db.Create(targetSite).Error)

	record := &model.SeedingTorrentRecord{
		ClientID:  "reseed-client",
		TorrentID: "src-001",
		InfoHash:  "aa111bb222cc333dd444ee555ff666aa777bb88",
		SiteName:  "f7-source",
		Status:    model.SeedingStatusSeeding,
		Source:    "rss",
	}
	require.NoError(t, db.Create(record).Error)

	task := &model.ReseedTask{
		Name:                 "f7-task",
		ClientIDs:            "reseed-client",
		TargetSiteIDs:        "f7-target",
		EngineMode:           "e1_manual",
		MatchMethods:         "infohash",
		SizeTolerancePercent: 1.0,
		MaxInjectionsPerRun:  10,
		Enabled:              true,
		Status:               "idle",
	}
	require.NoError(t, db.Create(task).Error)

	var downloadCalled, addFromFileCalled int32
	torrentData := createTestTorrentData(t)

	mockSiteProvider := &mocks.SiteInfoProvider{
		GetSiteInfoFn: func(ctx context.Context, siteName string) (*model.SiteInfo, error) {
			return &model.SiteInfo{Name: siteName, Enabled: true}, nil
		},
		GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
			return &model.SiteConfig{Domain: domain}, nil
		},
		GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
			return &mocks.SiteAdapter{
				SearchTorrentsFn: func(ctx context.Context, config *model.SiteConfig, query string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
					return []*model.SeedingSearchResult{
						{
							TorrentID: "tgt-001",
							Title:     "Ubuntu 24.04",
							Size:      4700000000,
						},
					}, nil
				},
				GetTorrentInfoHashFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) (string, error) {
					return "aa111bb222cc333dd444ee555ff666aa777bb88", nil
				},
				DownloadTorrentFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error) {
					atomic.AddInt32(&downloadCalled, 1)
					return torrentData, nil
				},
			}, nil
		},
	}

	mockDLProvider := &mocks.DownloaderProvider{
		GetFn: func(clientID string) (model.DownloaderClient, error) {
			return &mocks.DownloaderClient{
				AddFromFileFn: func(ctx context.Context, data []byte, opts model.AddTorrentOptions) (*model.AddResult, error) {
					atomic.AddInt32(&addFromFileCalled, 1)
					return &model.AddResult{InfoHash: "aa111bb222cc333dd444ee555ff666aa777bb88"}, nil
				},
				GetTorrentByHashFn: func(ctx context.Context, hash string) (*model.TorrentInfo, error) {
					return nil, nil
				},
			}, nil
		},
	}

	eng := reseed.NewEngine(db, nopLogger())
	eng.SetSiteProvider(mockSiteProvider)
	eng.SetClientProvider(mockDLProvider)

	result, err := eng.RunTask(ctx, task)
	require.NoError(t, err)

	assert.Equal(t, 0, result.Failed)
	assert.True(t, result.Injected > 0 || result.Matched > 0, "should have at least one match or injection")

	assert.Equal(t, int32(1), atomic.LoadInt32(&downloadCalled))
	assert.Equal(t, int32(1), atomic.LoadInt32(&addFromFileCalled))

	var matches []model.ReseedMatch
	db.Find(&matches)
	require.Len(t, matches, 1)
	assert.Equal(t, "tgt-001", matches[0].TargetTorrentID)

	t.Logf("PASS F7: Layer1 InfoHash match matched=%d injected=%d failed=%d",
		result.Matched, result.Injected, result.Failed)
}

func TestScenario_F7_NoMatchFound(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	sourceSite := &model.Site{
		Name: "f7nomatch-src", Domain: "f7nomatch-src.com",
		BaseURL: "https://f7nomatch-src.com", AuthType: "cookie",
	}
	require.NoError(t, db.Create(sourceSite).Error)

	targetSite := &model.Site{
		Name: "f7nomatch-tgt", Domain: "f7nomatch-tgt.com",
		BaseURL: "https://f7nomatch-tgt.com", AuthType: "cookie",
	}
	require.NoError(t, db.Create(targetSite).Error)

	record := &model.SeedingTorrentRecord{
		ClientID:  "reseed-client",
		TorrentID: "src-nm-001",
		InfoHash:  "cc111bb222cc333dd444ee555ff666aa777bb00",
		SiteName:  "f7nomatch-src",
		Status:    model.SeedingStatusSeeding,
		Source:    "rss",
	}
	require.NoError(t, db.Create(record).Error)

	task := &model.ReseedTask{
		Name:                 "f7-nomatch",
		ClientIDs:            "reseed-client",
		TargetSiteIDs:        "f7nomatch-tgt",
		EngineMode:           "e1_manual",
		MatchMethods:         "infohash",
		SizeTolerancePercent: 1.0,
		MaxInjectionsPerRun:  10,
		Enabled:              true,
		Status:               "idle",
	}
	require.NoError(t, db.Create(task).Error)

	mockSiteProvider := &mocks.SiteInfoProvider{
		GetSiteInfoFn: func(ctx context.Context, siteName string) (*model.SiteInfo, error) {
			return &model.SiteInfo{Name: siteName, Enabled: true}, nil
		},
		GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
			return &model.SiteConfig{Domain: domain}, nil
		},
		GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
			return &mocks.SiteAdapter{
				SearchTorrentsFn: func(ctx context.Context, config *model.SiteConfig, query string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
					return nil, nil
				},
			}, nil
		},
	}

	eng := reseed.NewEngine(db, nopLogger())
	eng.SetSiteProvider(mockSiteProvider)

	result, err := eng.RunTask(ctx, task)
	require.NoError(t, err)

	assert.Equal(t, 0, result.Matched)
	assert.Equal(t, 0, result.Injected)

	t.Logf("PASS F7-nomatch: no candidates found matched=%d", result.Matched)
}

// ── F12: SideLoad 哈希解析 ────────────────────────────────────────

func TestScenario_F12_SideLoadHashResolution(t *testing.T) {
	emitter := rss.NewSideLoadEventEmitter()
	ch := emitter.Subscribe()

	torrentData := createTestTorrentData(t)

	mockProvider := &mocks.SiteInfoProvider{
		GetSiteInfoFn: func(ctx context.Context, siteName string) (*model.SiteInfo, error) {
			return &model.SiteInfo{
				Name:    siteName,
				Passkey: "test-pk",
			}, nil
		},
		GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
			return &model.SiteConfig{Domain: domain, Passkey: "test-pk"}, nil
		},
		GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
			return &mocks.SiteAdapter{
				DownloadTorrentFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error) {
					return torrentData, nil
				},
			}, nil
		},
	}

	mgr := rss.NewSideLoadManager(mockProvider, emitter, nopLogger())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mgr.Start(ctx)
	defer mgr.Stop()

	event := &model.TorrentEvent{
		SiteName:            "testsite",
		TorrentID:           "501",
		InfoHash:            "fakehash501fakehash",
		Title:               "Test Torrent",
		Size:                1024,
		RequiresSideLoading: true,
		SideLoadStatus:      model.SideLoadPending,
		Metadata:            map[string]any{},
	}

	require.NoError(t, mgr.Enqueue(event, "testsite"))

	var completedEvent rss.SideLoadEvent
	timeout := time.After(5 * time.Second)
loop:
	for {
		select {
		case ev := <-ch:
			if ev.Status == "completed" {
				completedEvent = ev
				break loop
			}
		case <-timeout:
			t.Fatal("timeout waiting for side load completion")
		}
	}

	require.NotNil(t, completedEvent.TorrentEvent)
	assert.NotEqual(t, "fakehash501fakehash", completedEvent.TorrentEvent.InfoHash)
	assert.Len(t, completedEvent.TorrentEvent.InfoHash, 40)
	assert.Equal(t, model.SideLoadCompleted, completedEvent.TorrentEvent.SideLoadStatus)

	t.Logf("PASS F12: SideLoad resolved fakehash → %s", completedEvent.TorrentEvent.InfoHash)
}

func TestScenario_F12_SideLoadFailure(t *testing.T) {
	emitter := rss.NewSideLoadEventEmitter()
	ch := emitter.Subscribe()

	mockProvider := &mocks.SiteInfoProvider{
		GetSiteInfoFn: func(ctx context.Context, siteName string) (*model.SiteInfo, error) {
			return &model.SiteInfo{Name: siteName, Enabled: true}, nil
		},
		GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
			return &model.SiteConfig{Domain: domain}, nil
		},
		GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
			return &mocks.SiteAdapter{
				DownloadTorrentFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error) {
					return nil, fmt.Errorf("download failed: site unreachable")
				},
			}, nil
		},
	}

	mgr := rss.NewSideLoadManager(mockProvider, emitter, nopLogger())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mgr.Start(ctx)
	defer mgr.Stop()

	event := &model.TorrentEvent{
		SiteName:            "testsite",
		TorrentID:           "502",
		InfoHash:            "fakehash502fakehash",
		Title:               "Broken Torrent",
		Size:                2048,
		RequiresSideLoading: true,
		SideLoadStatus:      model.SideLoadPending,
		Metadata:            map[string]any{},
	}

	require.NoError(t, mgr.Enqueue(event, "testsite"))

	timeout := time.After(5 * time.Second)
loop:
	for {
		select {
		case ev := <-ch:
			if ev.Status == "failed" {
				assert.Contains(t, ev.FailedReason, "download failed")
				t.Logf("PASS F12-fail: download failure handled reason=%s", ev.FailedReason)
				break loop
			}
		case <-timeout:
			t.Fatal("timeout waiting for side load failure")
		}
	}
}
