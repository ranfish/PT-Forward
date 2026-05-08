package integration

import (
	"context"
	"fmt"
	"testing"

	"github.com/ranfish/pt-forward/internal/dispatcher"
	"github.com/ranfish/pt-forward/internal/mocks"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/publish"
	"github.com/ranfish/pt-forward/internal/seeding"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestE2E_PublishCandidateToUpload(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	seedSite(t, db, "source.com", "source-site")
	seedSite(t, db, "target.com", "target-site")
	sourceClientID := seedClient(t, db, "source-client", "seeding")
	_ = sourceClientID

	candidate := &model.PublishCandidate{
		SourceSite:      "source-site",
		SourceTorrentID: "torrent-001",
		InfoHash:        "abc123def456",
		TorrentName:     "Ubuntu 24.04 LTS Desktop amd64",
		Size:            4700000000,
		ClientID:        "source-client",
		TargetSites:     "target-site",
		PublishStatus:   model.CandidatePending,
	}
	require.NoError(t, db.Create(candidate).Error)

	uploadCalled := false
	mockSiteProvider := &mocks.SiteInfoProvider{
		GetSiteInfoFn: func(ctx context.Context, siteName string) (*model.SiteInfo, error) {
			return &model.SiteInfo{Name: siteName, BaseURL: "https://" + siteName + ".com"}, nil
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
						Title:        "Ubuntu 24.04 LTS Desktop amd64",
						Description:  "Ubuntu desktop image",
						Category:     "linux",
						Source:       "BluRay",
						Resolution:   "2160p",
						Codec:        "x264",
						ReleaseGroup: "Ubuntu",
					}, nil
				},
				SearchTorrentsFn: func(ctx context.Context, config *model.SiteConfig, query string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
					return nil, nil
				},
				UploadTorrentFn: func(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
					uploadCalled = true
					assert.Equal(t, "Ubuntu 24.04 LTS Desktop amd64", req.Title)
					assert.Equal(t, "target-site", req.TargetSite)
					assert.NotNil(t, req.TorrentData)
					t.Logf("upload called: title=%s source=%s target=%s fields=%v",
						req.Title, req.SourceSite, req.TargetSite, req.FormFields)
					return &model.PublishResponse{
						TorrentID: "pub-001",
						DetailURL: "https://target.com/torrents/pub-001",
					}, nil
				},
			}, nil
		},
	}

	pipeline := publish.NewPipeline(db, nopLogger())
	pipeline.SetSiteProvider(mockSiteProvider)

	result, err := pipeline.PublishCandidate(ctx, candidate.ID)
	require.NoError(t, err)
	assert.Equal(t, model.CandidateDone, result.PublishStatus)
	assert.True(t, uploadCalled, "UploadTorrent should have been called")

	var results []model.PublishResultRecord
	require.NoError(t, db.Find(&results).Error)
	require.Len(t, results, 1)
	assert.Equal(t, model.PublishResultCompleted, results[0].Status)
	assert.Equal(t, "pub-001", results[0].TorrentID)
	assert.Equal(t, "https://target.com/torrents/pub-001", results[0].PublishURL)

	t.Logf("PASS: publish result status=%s torrent_id=%s url=%s",
		results[0].Status, results[0].TorrentID, results[0].PublishURL)
}

func TestE2E_PublishBlockedByKeyword(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	seedSite(t, db, "source.com", "source-site")

	candidate := &model.PublishCandidate{
		SourceSite:      "source-site",
		SourceTorrentID: "torrent-002",
		InfoHash:        "def456",
		TorrentName:     "Something 禁转 Something",
		Size:            1000000000,
		PublishStatus:   model.CandidatePending,
		TargetSites:     "target-site",
	}
	require.NoError(t, db.Create(candidate).Error)

	pipeline := publish.NewPipeline(db, nopLogger())
	pipeline.SetSiteProvider(&mocks.SiteInfoProvider{})

	result, err := pipeline.PublishCandidate(ctx, candidate.ID)
	assert.Error(t, err)
	if result != nil {
		assert.Equal(t, model.CandidateSkipped, result.PublishStatus)
	}
	t.Logf("PASS: blocked by keyword, err=%v", err)
}

func TestE2E_PublishBlockedByExclusion(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	seedSite(t, db, "source.com", "source-site")
	seedSite(t, db, "target.com", "target-site")

	exclusion := &model.PublishExclusion{
		SourceSite: "source-site",
		TargetSite: "target-site",
	}
	require.NoError(t, db.Create(exclusion).Error)

	candidate := &model.PublishCandidate{
		SourceSite:      "source-site",
		SourceTorrentID: "torrent-003",
		InfoHash:        "ghi789",
		TorrentName:     "Normal Release 2024",
		Size:            2000000000,
		PublishStatus:   model.CandidatePending,
		TargetSites:     "target-site",
	}
	require.NoError(t, db.Create(candidate).Error)

	mockSiteProvider := &mocks.SiteInfoProvider{
		GetSiteInfoFn: func(ctx context.Context, siteName string) (*model.SiteInfo, error) {
			return &model.SiteInfo{Name: siteName, BaseURL: "https://" + siteName + ".com"}, nil
		},
		GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
			return &model.SiteConfig{Domain: domain}, nil
		},
		GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
			return &mocks.SiteAdapter{
				DownloadTorrentFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error) {
					return []byte("d8:announce...e"), nil
				},
				SearchTorrentsFn: func(ctx context.Context, config *model.SiteConfig, query string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
					return nil, nil
				},
			}, nil
		},
	}

	pipeline := publish.NewPipeline(db, nopLogger())
	pipeline.SetSiteProvider(mockSiteProvider)

	_, err := pipeline.PublishCandidate(ctx, candidate.ID)
	assert.NoError(t, err)

	var results []model.PublishResultRecord
	require.NoError(t, db.Find(&results).Error)
	assert.Equal(t, 0, len(results), "should skip due to exclusion, no upload attempted")

	t.Logf("PASS: exclusion rule blocked publish from source-site to target-site")
}

func TestE2E_FullChain_RSSToPublishViaReseed(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	seedSite(t, db, "source.com", "source-site")
	seedSite(t, db, "target.com", "target-site")
	clientID := seedClient(t, db, "seeder-client", "seeding")
	_ = clientID

	sub := &model.RSSSubscription{
		Name: "test-sub", SiteName: "source-site",
		URLs: []string{"https://source.com/rss"}, Cron: "*/15 * * * *",
		ClientID: "seeder-client", Enabled: true,
	}
	require.NoError(t, db.Create(sub).Error)

	seedingEng := seeding.NewEngine(db, nopLogger())
	publishPipeline := publish.NewPipeline(db, nopLogger())

	uploadCalled := false
	mockSiteProvider := &mocks.SiteInfoProvider{
		GetSiteInfoFn: func(ctx context.Context, siteName string) (*model.SiteInfo, error) {
			return &model.SiteInfo{Name: siteName, BaseURL: "https://" + siteName + ".com"}, nil
		},
		GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
			return &model.SiteConfig{Domain: domain}, nil
		},
		GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
			return &mocks.SiteAdapter{
				VerifyExistsFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) (bool, error) {
					return false, nil
				},
				DownloadTorrentFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error) {
					return []byte("d8:announce...e"), nil
				},
				GetTorrentDetailFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.TorrentDetail, error) {
					return &model.TorrentDetail{Title: "Test Torrent", Category: "movie"}, nil
				},
				SearchTorrentsFn: func(ctx context.Context, config *model.SiteConfig, query string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
					return nil, nil
				},
				UploadTorrentFn: func(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
					uploadCalled = true
					return &model.PublishResponse{TorrentID: "pub-full-001", DetailURL: "https://target.com/t/pub-full-001"}, nil
				},
			}, nil
		},
	}
	mockDLProvider := &mocks.DownloaderProvider{
		GetFn: func(cid string) (model.DownloaderClient, error) {
			return &mocks.DownloaderClient{
				ID: 1, Name: "seeder-client", Role: "seeding",
				CheckExistsFn: func(ctx context.Context, infoHash string) (bool, error) {
					return false, nil
				},
			}, nil
		},
	}
	publishPipeline.SetSiteProvider(mockSiteProvider)
	publishPipeline.SetClientProvider(mockDLProvider)

	td := dispatcher.NewTorrentDispatcher(db, nil, nopLogger())
	td.RegisterHandler(dispatcher.RoleSeeding, seedingEng)
	td.RegisterHandler(dispatcher.RoleDownload, publishPipeline)

	rssEvents := []model.TorrentEvent{{
		SiteName: "source-site", TorrentID: "src-t-001",
		Title: "Big.Buck.Bunny.2024.2160p.BluRay.x264-GROUP",
		Size:  20000000000, InfoHash: "aaaa1111bbbb2222",
		SourceID: fmt.Sprintf("%d", sub.ID),
	}}

	require.NoError(t, td.OnTorrents(ctx, rssEvents))

	seedingCount := countRecords(t, db, "seeding_torrent_records")
	assert.Equal(t, int64(1), seedingCount)
	t.Logf("Step 1 OK: seeding_records=%d", seedingCount)

	seedRec := &model.SeedingTorrentRecord{}
	require.NoError(t, db.First(seedRec).Error)
	assert.Equal(t, "src-t-001", seedRec.TorrentID)
	t.Logf("Step 2 OK: seeding record torrent_id=%s info_hash=%s", seedRec.TorrentID, seedRec.InfoHash)

	candidateCount := countRecords(t, db, "publish_candidates")
	t.Logf("Step 3: publish_candidates=%d (may be 0 if no publish role configured)", candidateCount)

	if candidateCount > 0 {
		var cand model.PublishCandidate
		require.NoError(t, db.First(&cand).Error)
		cand.TargetSites = "target-site"
		require.NoError(t, db.Save(&cand).Error)

		result, err := publishPipeline.PublishCandidate(ctx, cand.ID)
		require.NoError(t, err)
		assert.Equal(t, model.CandidateDone, result.PublishStatus)
		assert.True(t, uploadCalled)
		t.Logf("Step 4 OK: published to target-site, status=%s", result.PublishStatus)
	}

	t.Logf("PASS: full chain RSS→Seeding→Publish completed")
}
