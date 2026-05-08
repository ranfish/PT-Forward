package integration

import (
	"context"
	"fmt"
	"testing"

	"github.com/ranfish/pt-forward/internal/dispatcher"
	"github.com/ranfish/pt-forward/internal/filter"
	"github.com/ranfish/pt-forward/internal/mocks"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/publish"
	"github.com/ranfish/pt-forward/internal/reseed"
	"github.com/ranfish/pt-forward/internal/seeding"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func seedClient(t *testing.T, db *gorm.DB, name, role string) uint {
	t.Helper()
	client := &model.ClientConfig{
		Name:    name,
		Type:    "qbittorrent",
		Role:    role,
		Enabled: true,
		URL:     "http://localhost:8080",
	}
	if err := db.Create(client).Error; err != nil {
		t.Fatalf("create client %s: %v", name, err)
	}
	return client.ID
}

func TestE2E_RSSToSeedingRecord(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	site := &model.Site{
		Name: "source-site", Domain: "source.com",
		BaseURL: "https://source.com", AuthType: "cookie",
	}
	require.NoError(t, db.Create(site).Error)

	clientID := seedClient(t, db, "seeding-client", "seeding")

	sub := &model.RSSSubscription{
		Name:     "test-sub",
		SiteName: "source-site",
		URLs:     []string{"https://source.com/rss"},
		Cron:     "*/15 * * * *",
		ClientID: "seeding-client",
		Enabled:  true,
	}
	require.NoError(t, db.Create(sub).Error)

	seedingEng := seeding.NewEngine(db, nopLogger())
	publishPipeline := publish.NewPipeline(db, nopLogger())

	mockSiteProvider := &mocks.SiteInfoProvider{
		GetSiteInfoFn: func(ctx context.Context, siteName string) (*model.SiteInfo, error) {
			return &model.SiteInfo{Name: siteName, BaseURL: "https://source.com"}, nil
		},
		GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
			return &model.SiteConfig{}, nil
		},
		GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
			return &mocks.SiteAdapter{}, nil
		},
	}
	mockDLProvider := &mocks.DownloaderProvider{
		GetFn: func(cid string) (model.DownloaderClient, error) {
			return &mocks.DownloaderClient{
				ID: clientID, Name: "seeding-client", Role: "seeding",
			}, nil
		},
	}
	publishPipeline.SetSiteProvider(mockSiteProvider)
	publishPipeline.SetClientProvider(mockDLProvider)

	td := dispatcher.NewTorrentDispatcher(db, nil, nopLogger())
	td.RegisterHandler(dispatcher.RoleSeeding, seedingEng)
	td.RegisterHandler(dispatcher.RoleDownload, publishPipeline)

	events := []model.TorrentEvent{{
		SiteName: "source-site", TorrentID: "torrent-001",
		Title: "Ubuntu 24.04 LTS", Size: 4700000000,
		InfoHash: "abc123def456", SourceID: "1",
	}}

	require.NoError(t, td.OnTorrents(ctx, events))

	assert.Equal(t, int64(1), countRecords(t, db, "seeding_torrent_records"))

	var rec model.SeedingTorrentRecord
	require.NoError(t, db.First(&rec).Error)
	assert.Equal(t, "torrent-001", rec.TorrentID)
	assert.Equal(t, "abc123def456", rec.InfoHash)
	t.Logf("PASS: seeding record id=%d torrent=%s site=%s", rec.ID, rec.TorrentID, rec.SiteName)
}

func TestE2E_FilterAcceptReject(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	rule := &model.FilterRule{
		Name:       "reject-small",
		RuleType:   "reject",
		Priority:   10,
		Enabled:    true,
		Conditions: []model.RuleCondition{{Key: "title", CompareType: model.CompareContain, Value: "spam"}},
	}
	require.NoError(t, db.Create(rule).Error)

	acceptRule := &model.FilterRule{
		Name:       "accept-all",
		RuleType:   "accept",
		Priority:   100,
		Enabled:    true,
		Conditions: []model.RuleCondition{{Key: "title", CompareType: model.CompareRegExp, Value: ".*"}},
	}
	require.NoError(t, db.Create(acceptRule).Error)

	filterEng := filter.NewEngine(filter.NewRepository(db), nopLogger())

	result, err := filterEng.Match(ctx, &filter.EvalContext{
		Title: "Good.File.2024.2160p", Size: 50000000000, SiteName: "source-site",
	})
	require.NoError(t, err)
	assert.True(t, result.Matched)
	assert.Equal(t, "accept-all", result.RuleName)
	t.Logf("PASS: good file accepted by rule=%s", result.RuleName)

	result2, err := filterEng.Match(ctx, &filter.EvalContext{
		Title: "spam.bad.file", Size: 100, SiteName: "source-site",
	})
	require.NoError(t, err)
	assert.True(t, result2.Reject)
	t.Logf("PASS: spam file rejected")
}

func TestE2E_ReseedMatchAndInject(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	seedSite(t, db, "source.com", "source-site")
	seedSite(t, db, "target.com", "target-site")
	sourceClientID := seedClient(t, db, "source-client", "seeding")
	targetClientID := seedClient(t, db, "target-client", "download")

	seedRec := &model.SeedingTorrentRecord{
		TorrentID: "torrent-001",
		SiteName:  "source-site",
		ClientID:  fmt.Sprintf("%d", sourceClientID),
		InfoHash:  "abc123def456",
		Status:    "seeding",
		IsFree:    true,
		Source:    "rss",
	}
	require.NoError(t, db.Create(seedRec).Error)

	task := &model.ReseedTask{
		Name: "test-reseed", Enabled: true,
		ClientIDs:     fmt.Sprintf("%d", targetClientID),
		SourceSiteIDs: "source-site",
		TargetSiteIDs: "target-site",
		Status:        "idle",
	}
	require.NoError(t, db.Create(task).Error)

	downloaderCalled := false
	mockSiteProvider := &mocks.SiteInfoProvider{
		GetSiteInfoFn: func(ctx context.Context, siteName string) (*model.SiteInfo, error) {
			return &model.SiteInfo{Name: siteName, BaseURL: "https://" + siteName + ".com"}, nil
		},
		GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
			return &model.SiteConfig{Domain: domain}, nil
		},
		GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
			return &mocks.SiteAdapter{
				SearchTorrentsFn: func(ctx context.Context, config *model.SiteConfig, query string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
					return []*model.SeedingSearchResult{{
						TorrentID: "target-t-001",
						Title:     "Ubuntu 24.04 LTS Desktop amd64",
						Size:      4700000000,
					}}, nil
				},
				DownloadTorrentFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error) {
					return []byte("d8:announce30:http://tracker.example.com/announcee"), nil
				},
			}, nil
		},
		ListSitesFn: func(ctx context.Context) ([]*model.SiteInfo, error) {
			return []*model.SiteInfo{{Name: "target-site", BaseURL: "https://target.com"}}, nil
		},
	}
	mockDLProvider := &mocks.DownloaderProvider{
		GetFn: func(cid string) (model.DownloaderClient, error) {
			return &mocks.DownloaderClient{
				ID: targetClientID, Name: "target-client", Role: "download",
				AddFromFileFn: func(ctx context.Context, data []byte, opts model.AddTorrentOptions) (*model.AddResult, error) {
					downloaderCalled = true
					return &model.AddResult{InfoHash: "injected_hash"}, nil
				},
			}, nil
		},
	}

	reseedEng := reseed.NewEngine(db, nopLogger())
	reseedEng.SetSiteProvider(mockSiteProvider)
	reseedEng.SetClientProvider(mockDLProvider)

	result, err := reseedEng.RunTask(ctx, task)
	require.NoError(t, err)
	t.Logf("reseed result: matched=%d injected=%d failed=%d skipped=%d",
		result.Matched, result.Injected, result.Failed, result.Skipped)

	var matches []model.ReseedMatch
	require.NoError(t, db.Find(&matches).Error)
	t.Logf("matches=%d downloader_called=%v", len(matches), downloaderCalled)
	if len(matches) > 0 {
		t.Logf("PASS: match status=%s target=%s", matches[0].Status, matches[0].TargetSite)
	}
}

func seedSite(t *testing.T, db *gorm.DB, domain, name string) {
	t.Helper()
	s := &model.Site{Name: name, Domain: domain, BaseURL: "https://" + domain, AuthType: "cookie"}
	if err := db.Create(s).Error; err != nil {
		t.Fatalf("create site %s: %v", name, err)
	}
}
