package integration

import (
	"context"
	"errors"
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

func TestE2E_PublishCandidate_HRBlocked(t *testing.T) {
	db, ctx := setupTestEnv(t)
	seedSite(t, db, "source.com", "source-site")

	candidate := &model.PublishCandidate{
		SourceSite:      "source-site",
		SourceTorrentID: "torrent-hr",
		InfoHash:        "hr123",
		TorrentName:     "Normal.Release.2024",
		Size:            1000000000,
		PublishStatus:   model.CandidatePending,
		TargetSites:     "target-site",
		HasHR:           true,
	}
	require.NoError(t, db.Create(candidate).Error)

	pipeline := publish.NewPipeline(db, nopLogger())
	pipeline.SetSiteProvider(&mocks.SiteInfoProvider{})

	result, err := pipeline.PublishCandidate(ctx, candidate.ID)
	assert.Error(t, err)
	if result != nil {
		assert.Equal(t, model.CandidateSkipped, result.PublishStatus)
	}
}

func TestE2E_PublishCandidate_CatEDUBlocked(t *testing.T) {
	db, ctx := setupTestEnv(t)
	seedSite(t, db, "source.com", "source-site")

	candidate := &model.PublishCandidate{
		SourceSite:      "source-site",
		SourceTorrentID: "torrent-catedu",
		InfoHash:        "catedu123",
		TorrentName:     "Some.CatEDU.Course.2024",
		Size:            500000000,
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
}

func TestE2E_PublishCandidate_NotFound(t *testing.T) {
	db, ctx := setupTestEnv(t)

	pipeline := publish.NewPipeline(db, nopLogger())
	result, err := pipeline.PublishCandidate(ctx, 99999)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestE2E_PublishCandidate_NoSiteProvider(t *testing.T) {
	db, ctx := setupTestEnv(t)
	seedSite(t, db, "source.com", "source-site")

	candidate := &model.PublishCandidate{
		SourceSite:      "source-site",
		SourceTorrentID: "torrent-noprovider",
		InfoHash:        "noprovider123",
		TorrentName:     "Clean.Release.2024",
		Size:            2000000000,
		PublishStatus:   model.CandidatePending,
		TargetSites:     "target-site",
	}
	require.NoError(t, db.Create(candidate).Error)

	pipeline := publish.NewPipeline(db, nopLogger())
	result, err := pipeline.PublishCandidate(ctx, candidate.ID)
	require.NoError(t, err)
	assert.Equal(t, model.CandidatePublishing, result.PublishStatus)
}

func TestE2E_PublishCandidate_EmptyTargets(t *testing.T) {
	db, ctx := setupTestEnv(t)
	seedSite(t, db, "source.com", "source-site")

	candidate := &model.PublishCandidate{
		SourceSite:      "source-site",
		SourceTorrentID: "torrent-empty-target",
		InfoHash:        "empty123",
		TorrentName:     "Clean.Release.2024",
		Size:            2000000000,
		PublishStatus:   model.CandidatePending,
		TargetSites:     "",
	}
	require.NoError(t, db.Create(candidate).Error)

	pipeline := publish.NewPipeline(db, nopLogger())
	pipeline.SetSiteProvider(makeDefaultSiteProvider(nil))

	result, err := pipeline.PublishCandidate(ctx, candidate.ID)
	require.NoError(t, err)
	assert.Equal(t, model.CandidatePublishing, result.PublishStatus)
}

func TestE2E_PublishCandidate_UploadFailed(t *testing.T) {
	db, ctx := setupTestEnv(t)
	seedSite(t, db, "source.com", "source-site")
	seedSite(t, db, "target.com", "target-site")

	candidate := &model.PublishCandidate{
		SourceSite:      "source-site",
		SourceTorrentID: "torrent-upload-fail",
		InfoHash:        "uploadfail123",
		TorrentName:     "Clean.Release.2024",
		Size:            3000000000,
		PublishStatus:   model.CandidatePending,
		TargetSites:     "target-site",
	}
	require.NoError(t, db.Create(candidate).Error)

	mockProvider := &mocks.SiteInfoProvider{
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
					return &model.TorrentDetail{Title: "Clean.Release.2024", Category: "movie"}, nil
				},
				SearchTorrentsFn: func(ctx context.Context, config *model.SiteConfig, query string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
					return nil, nil
				},
				UploadTorrentFn: func(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
					return nil, errors.New("upload rejected by site")
				},
			}, nil
		},
	}

	pipeline := publish.NewPipeline(db, nopLogger())
	pipeline.SetSiteProvider(mockProvider)

	result, err := pipeline.PublishCandidate(ctx, candidate.ID)
	require.NoError(t, err)
	assert.Equal(t, model.CandidateFailed, result.PublishStatus)

	var results []model.PublishResultRecord
	require.NoError(t, db.Find(&results).Error)
	require.Len(t, results, 1)
	assert.Equal(t, model.PublishResultFailed, results[0].Status)
	assert.Contains(t, results[0].ErrorMessage, "upload rejected by site")
}

func TestE2E_PublishCandidate_DownloadFailed(t *testing.T) {
	db, ctx := setupTestEnv(t)
	seedSite(t, db, "source.com", "source-site")
	seedSite(t, db, "target.com", "target-site")

	candidate := &model.PublishCandidate{
		SourceSite:      "source-site",
		SourceTorrentID: "torrent-dl-fail",
		InfoHash:        "dlfail123",
		TorrentName:     "Clean.Release.2024",
		Size:            3000000000,
		PublishStatus:   model.CandidatePending,
		TargetSites:     "target-site",
	}
	require.NoError(t, db.Create(candidate).Error)

	mockProvider := &mocks.SiteInfoProvider{
		GetSiteInfoFn: func(ctx context.Context, siteName string) (*model.SiteInfo, error) {
			return &model.SiteInfo{Name: siteName, BaseURL: "https://" + siteName + ".com"}, nil
		},
		GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
			return &model.SiteConfig{Domain: domain}, nil
		},
		GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
			return &mocks.SiteAdapter{
				DownloadTorrentFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error) {
					return nil, errors.New("network timeout")
				},
				GetTorrentDetailFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.TorrentDetail, error) {
					return nil, nil
				},
				SearchTorrentsFn: func(ctx context.Context, config *model.SiteConfig, query string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
					return nil, nil
				},
			}, nil
		},
	}

	pipeline := publish.NewPipeline(db, nopLogger())
	pipeline.SetSiteProvider(mockProvider)

	result, err := pipeline.PublishCandidate(ctx, candidate.ID)
	assert.Error(t, err)
	if result != nil {
		assert.Equal(t, model.CandidateFailed, result.PublishStatus)
	}
}

func TestE2E_PublishCandidate_Dedup(t *testing.T) {
	db, ctx := setupTestEnv(t)
	seedSite(t, db, "source.com", "source-site")
	seedSite(t, db, "target.com", "target-site")

	candidate := &model.PublishCandidate{
		SourceSite:      "source-site",
		SourceTorrentID: "torrent-dedup",
		InfoHash:        "dedup123",
		TorrentName:     "Dedup.Release.2024",
		Size:            4000000000,
		PublishStatus:   model.CandidatePending,
		TargetSites:     "target-site",
	}
	require.NoError(t, db.Create(candidate).Error)

	uploadCalled := false
	mockProvider := &mocks.SiteInfoProvider{
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
					return &model.TorrentDetail{Title: "Dedup.Release.2024", Category: "movie"}, nil
				},
				SearchTorrentsFn: func(ctx context.Context, config *model.SiteConfig, query string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
					return []*model.SeedingSearchResult{{
						TorrentID: "existing-001",
						Title:     "Dedup.Release.2024",
						Size:      4000000000,
					}}, nil
				},
				UploadTorrentFn: func(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
					uploadCalled = true
					return &model.PublishResponse{TorrentID: "dup-001"}, nil
				},
			}, nil
		},
	}

	pipeline := publish.NewPipeline(db, nopLogger())
	pipeline.SetSiteProvider(mockProvider)

	_, err := pipeline.PublishCandidate(ctx, candidate.ID)
	require.NoError(t, err)
	assert.False(t, uploadCalled, "upload should not be called for dedup")

	var results []model.PublishResultRecord
	require.NoError(t, db.Find(&results).Error)
	if len(results) > 0 {
		assert.Equal(t, model.PublishResultSkipped, results[0].Status)
	}

	t.Logf("PASS: dedup detected, publish skipped, uploadCalled=%v results=%d", uploadCalled, len(results))
}

func TestE2E_OnTorrents_WithMatchedRule(t *testing.T) {
	db, ctx := setupTestEnv(t)
	seedSite(t, db, "source.com", "source-site")
	seedClient(t, db, "dl-client", "download")

	sub := &model.RSSSubscription{
		Name: "sub-matched", SiteName: "source-site",
		URLs: []string{"https://source.com/rss"}, Cron: "*/15 * * * *",
		ClientID: "dl-client", Enabled: true,
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
	publishPipeline.SetSiteProvider(mockSiteProvider)

	td := dispatcher.NewTorrentDispatcher(db, nil, nopLogger())
	td.RegisterHandler(dispatcher.RoleSeeding, seedingEng)
	td.RegisterHandler(dispatcher.RoleDownload, publishPipeline)

	events := []model.TorrentEvent{{
		SiteName: "source-site", TorrentID: "matched-001",
		Title: "Matched.Torrent.2024", Size: 10000000000,
		InfoHash:        "matched1234abcd",
		MatchedRuleName: "accept-all",
		SourceID:        fmt.Sprintf("%d", sub.ID),
	}}

	require.NoError(t, td.OnTorrents(ctx, events))

	candidateCount := countRecords(t, db, "publish_candidates")
	assert.Equal(t, int64(1), candidateCount)
	t.Logf("PASS: matched rule creates candidate, count=%d", candidateCount)
}

func TestE2E_OnTorrents_NoMatchedRule(t *testing.T) {
	db, ctx := setupTestEnv(t)
	seedSite(t, db, "source.com", "source-site")
	seedClient(t, db, "seeding-client", "seeding")

	sub := &model.RSSSubscription{
		Name: "sub-unmatched", SiteName: "source-site",
		URLs: []string{"https://source.com/rss"}, Cron: "*/15 * * * *",
		ClientID: "seeding-client", Enabled: true,
	}
	require.NoError(t, db.Create(sub).Error)

	seedingEng := seeding.NewEngine(db, nopLogger())
	publishPipeline := publish.NewPipeline(db, nopLogger())

	td := dispatcher.NewTorrentDispatcher(db, nil, nopLogger())
	td.RegisterHandler(dispatcher.RoleSeeding, seedingEng)
	td.RegisterHandler(dispatcher.RoleDownload, publishPipeline)

	events := []model.TorrentEvent{{
		SiteName: "source-site", TorrentID: "unmatched-001",
		Title: "Unmatched.Torrent.2024", Size: 5000000000,
		InfoHash: "unmatched1234",
		SourceID: fmt.Sprintf("%d", sub.ID),
	}}

	require.NoError(t, td.OnTorrents(ctx, events))

	seedingCount := countRecords(t, db, "seeding_torrent_records")
	assert.Equal(t, int64(1), seedingCount)

	candidateCount := countRecords(t, db, "publish_candidates")
	assert.Equal(t, int64(0), candidateCount, "no candidate when no matched rule")
	t.Logf("PASS: no matched rule → seeding only, candidates=%d", candidateCount)
}

func TestE2E_PublishCandidate_MultipleTargets(t *testing.T) {
	db, ctx := setupTestEnv(t)
	seedSite(t, db, "source.com", "source-site")
	seedSite(t, db, "target-a.com", "target-a")
	seedSite(t, db, "target-b.com", "target-b")

	candidate := &model.PublishCandidate{
		SourceSite:      "source-site",
		SourceTorrentID: "torrent-multi",
		InfoHash:        "multi123",
		TorrentName:     "Multi.Target.Release.2024",
		Size:            5000000000,
		PublishStatus:   model.CandidatePending,
		TargetSites:     "target-a,target-b",
	}
	require.NoError(t, db.Create(candidate).Error)

	uploadCount := 0
	mockProvider := &mocks.SiteInfoProvider{
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
					return &model.TorrentDetail{Title: "Multi.Target.Release.2024", Category: "movie"}, nil
				},
				SearchTorrentsFn: func(ctx context.Context, config *model.SiteConfig, query string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
					return nil, nil
				},
				UploadTorrentFn: func(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
					uploadCount++
					return &model.PublishResponse{
						TorrentID: fmt.Sprintf("pub-%s-%d", req.TargetSite, uploadCount),
						DetailURL: fmt.Sprintf("https://%s.com/t/%d", req.TargetSite, uploadCount),
					}, nil
				},
			}, nil
		},
	}

	pipeline := publish.NewPipeline(db, nopLogger())
	pipeline.SetSiteProvider(mockProvider)

	result, err := pipeline.PublishCandidate(ctx, candidate.ID)
	require.NoError(t, err)
	assert.Equal(t, model.CandidateDone, result.PublishStatus)
	assert.Equal(t, 2, uploadCount)

	var results []model.PublishResultRecord
	require.NoError(t, db.Find(&results).Error)
	assert.Len(t, results, 2)
	t.Logf("PASS: multi-target publish, uploads=%d results=%d", uploadCount, len(results))
}

func TestE2E_PublishCandidate_GetSiteConfigFails(t *testing.T) {
	db, ctx := setupTestEnv(t)
	seedSite(t, db, "source.com", "source-site")

	candidate := &model.PublishCandidate{
		SourceSite:      "source-site",
		SourceTorrentID: "torrent-cfg-fail",
		InfoHash:        "cfgfail123",
		TorrentName:     "Clean.Release.2024",
		Size:            2000000000,
		PublishStatus:   model.CandidatePending,
		TargetSites:     "target-site",
	}
	require.NoError(t, db.Create(candidate).Error)

	mockProvider := &mocks.SiteInfoProvider{
		GetSiteConfigFn: func(ctx context.Context, domain string) (*model.SiteConfig, error) {
			return nil, errors.New("config lookup failed")
		},
		GetAdapterFn: func(ctx context.Context, domain string) (model.SiteAdapter, error) {
			return nil, errors.New("adapter lookup failed")
		},
	}

	pipeline := publish.NewPipeline(db, nopLogger())
	pipeline.SetSiteProvider(mockProvider)

	result, err := pipeline.PublishCandidate(ctx, candidate.ID)
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestE2E_PublishCandidate_SourceDetailFails(t *testing.T) {
	db, ctx := setupTestEnv(t)
	seedSite(t, db, "source.com", "source-site")
	seedSite(t, db, "target.com", "target-site")

	candidate := &model.PublishCandidate{
		SourceSite:      "source-site",
		SourceTorrentID: "torrent-detail-fail",
		InfoHash:        "detailfail123",
		TorrentName:     "Clean.Release.2024",
		Size:            2000000000,
		PublishStatus:   model.CandidatePending,
		TargetSites:     "target-site",
	}
	require.NoError(t, db.Create(candidate).Error)

	uploadCalled := false
	mockProvider := &mocks.SiteInfoProvider{
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
					return nil, errors.New("detail page unavailable")
				},
				SearchTorrentsFn: func(ctx context.Context, config *model.SiteConfig, query string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
					return nil, nil
				},
				UploadTorrentFn: func(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
					uploadCalled = true
					return &model.PublishResponse{TorrentID: "pub-detail-fail-001", DetailURL: "https://target.com/t/pub-detail-fail-001"}, nil
				},
			}, nil
		},
	}

	pipeline := publish.NewPipeline(db, nopLogger())
	pipeline.SetSiteProvider(mockProvider)

	result, err := pipeline.PublishCandidate(ctx, candidate.ID)
	require.NoError(t, err)
	assert.Equal(t, model.CandidateDone, result.PublishStatus)
	assert.True(t, uploadCalled, "upload should proceed even if detail fails")
}

func TestE2E_ProcessPending_NoCandidates(t *testing.T) {
	db, ctx := setupTestEnv(t)
	pipeline := publish.NewPipeline(db, nopLogger())
	err := pipeline.ProcessPending(ctx)
	assert.NoError(t, err)
}

func TestE2E_ProcessPending_SkippedByKeyword(t *testing.T) {
	db, ctx := setupTestEnv(t)

	candidate := &model.PublishCandidate{
		SourceSite:      "source-site",
		SourceTorrentID: "torrent-pending-skip",
		InfoHash:        "pendingskip",
		TorrentName:     "Something 禁转 Something",
		Size:            1000000000,
		PublishStatus:   model.CandidatePending,
	}
	require.NoError(t, db.Create(candidate).Error)

	pipeline := publish.NewPipeline(db, nopLogger())
	require.NoError(t, pipeline.ProcessPending(ctx))

	var updated model.PublishCandidate
	require.NoError(t, db.First(&updated, candidate.ID).Error)
	assert.Equal(t, model.CandidateSkipped, updated.PublishStatus)
}

func TestE2E_PublishCandidate_UsingHelpers(t *testing.T) {
	db, ctx := setupTestEnv(t)
	site := makeSite("helper-test.com", "helper-site")
	require.NoError(t, db.Create(site).Error)

	client := makeClient("helper-client", "seeding")
	require.NoError(t, db.Create(client).Error)

	evt := makeTorrentEvent("helper-site", "ht-001", "Helper.Test.Release", 1234567890, "helperhash123")
	assert.Equal(t, "helper-site", evt.SiteName)
	assert.Equal(t, "ht-001", evt.TorrentID)
	assert.Equal(t, int64(1234567890), evt.Size)

	uploadCalled := false
	provider := makeDefaultSiteProvider(&uploadCalled)
	assert.NotNil(t, provider)

	si, err := provider.GetSiteInfo(ctx, "test-site")
	require.NoError(t, err)
	assert.Equal(t, "test-site", si.Name)

	cfg, err := provider.GetSiteConfig(ctx, "test.com")
	require.NoError(t, err)
	assert.Equal(t, "test.com", cfg.Domain)

	adapter, err := provider.GetAdapter(ctx, "test.com")
	require.NoError(t, err)

	data, err := adapter.DownloadTorrent(ctx, cfg, "t-001")
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	detail, err := adapter.GetTorrentDetail(ctx, cfg, "t-001")
	require.NoError(t, err)
	assert.Equal(t, "Test Torrent", detail.Title)

	search, err := adapter.SearchTorrents(ctx, cfg, "test", nil)
	require.NoError(t, err)
	assert.Nil(t, search)

	resp, err := adapter.UploadTorrent(ctx, cfg, &model.PublishRequest{Title: "test"})
	require.NoError(t, err)
	assert.Equal(t, "pub-auto-001", resp.TorrentID)
	assert.True(t, uploadCalled)

	t.Logf("PASS: all helper factory functions work correctly")
}

func TestE2E_DuplicateSeedingRecord(t *testing.T) {
	db, ctx := setupTestEnv(t)
	s := makeSite("dup-source.com", "dup-source-site")
	require.NoError(t, db.Create(s).Error)

	cid := seedClient(t, db, "dup-seeding-client", "seeding")
	_ = cid

	sub := &model.RSSSubscription{
		Name: "dup-sub", SiteName: "dup-source-site",
		URLs: []string{"https://dup-source.com/rss"}, Cron: "*/15 * * * *",
		ClientID: "dup-seeding-client", Enabled: true,
	}
	require.NoError(t, db.Create(sub).Error)

	seedingEng := seeding.NewEngine(db, nopLogger())
	publishPipeline := publish.NewPipeline(db, nopLogger())

	mockSP := &mocks.SiteInfoProvider{
		GetSiteInfoFn: func(ctx context.Context, sn string) (*model.SiteInfo, error) {
			return &model.SiteInfo{Name: sn, BaseURL: "https://dup-source.com"}, nil
		},
		GetSiteConfigFn: func(ctx context.Context, d string) (*model.SiteConfig, error) {
			return &model.SiteConfig{}, nil
		},
		GetAdapterFn: func(ctx context.Context, d string) (model.SiteAdapter, error) {
			return &mocks.SiteAdapter{}, nil
		},
	}
	mockDL := &mocks.DownloaderProvider{
		GetFn: func(cid string) (model.DownloaderClient, error) {
			return &mocks.DownloaderClient{ID: 1, Name: "dup-seeding-client", Role: "seeding"}, nil
		},
	}
	publishPipeline.SetSiteProvider(mockSP)
	publishPipeline.SetClientProvider(mockDL)

	td := dispatcher.NewTorrentDispatcher(db, nil, nopLogger())
	td.RegisterHandler(dispatcher.RoleSeeding, seedingEng)
	td.RegisterHandler(dispatcher.RoleDownload, publishPipeline)

	evt := makeTorrentEvent("dup-source-site", "dup-001", "Duplicate.Test", 8000000000, "duphash1111abcd")
	evt.SourceID = fmt.Sprintf("%d", sub.ID)
	events := []model.TorrentEvent{evt, evt}

	require.NoError(t, td.OnTorrents(ctx, events))

	seedingCount := countRecords(t, db, "seeding_torrent_records")
	assert.Equal(t, int64(1), seedingCount, "duplicate events should produce single record")
	t.Logf("PASS: duplicate events handled, records=%d", seedingCount)
}

func TestE2E_PublishGroup_CRUD(t *testing.T) {
	db, ctx := setupTestEnv(t)
	pipeline := publish.NewPipeline(db, nopLogger())

	group, err := pipeline.CreateGroup(ctx, 1, "hash123", "source-site", "torrent-001")
	require.NoError(t, err)
	assert.Equal(t, model.GroupActive, group.Status)
	assert.Equal(t, uint(1), group.CandidateID)

	fetched, err := pipeline.GetGroup(ctx, group.ID)
	require.NoError(t, err)
	assert.Equal(t, group.SourceHash, fetched.SourceHash)

	groups, total, err := pipeline.ListGroups(ctx, 0, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, groups, 1)

	member := &model.PublishGroupMember{
		SiteName: "target-site",
		Role:     "target",
		Status:   model.MemberStatusNew,
	}
	require.NoError(t, pipeline.AddGroupMember(ctx, group.ID, member))

	members, err := pipeline.ListGroupMembers(ctx, group.ID)
	require.NoError(t, err)
	assert.Len(t, members, 1)
	assert.Equal(t, "target-site", members[0].SiteName)

	require.NoError(t, pipeline.UpdateGroupStatus(ctx, group.ID, model.GroupPublishing, "test reason"))

	updated, err := pipeline.GetGroup(ctx, group.ID)
	require.NoError(t, err)
	assert.Equal(t, model.GroupPublishing, updated.Status)

	require.NoError(t, pipeline.UpdateMemberStatus(ctx, members[0].ID, model.MemberStatusUploaded, ""))

	var updatedMember model.PublishGroupMember
	require.NoError(t, db.First(&updatedMember, members[0].ID).Error)
	assert.Equal(t, model.MemberStatusUploaded, updatedMember.Status)

	t.Logf("PASS: group CRUD operations, groupID=%d members=%d", group.ID, len(members))
}

func TestE2E_PublishCandidate_Lifecycle(t *testing.T) {
	db, ctx := setupTestEnv(t)
	seedSite(t, db, "source.com", "source-site")

	candidate := &model.PublishCandidate{
		SourceSite:      "source-site",
		SourceTorrentID: "torrent-lifecycle",
		InfoHash:        "lifecycle123",
		TorrentName:     "Lifecycle.Test.2024",
		Size:            1000000000,
		PublishStatus:   model.CandidatePending,
		TargetSites:     "target-site",
	}
	require.NoError(t, db.Create(candidate).Error)

	pipeline := publish.NewPipeline(db, nopLogger())

	fetched, err := pipeline.GetCandidate(ctx, candidate.ID)
	require.NoError(t, err)
	assert.Equal(t, candidate.InfoHash, fetched.InfoHash)

	require.NoError(t, pipeline.MarkDownloadCompleted(ctx, candidate.ID, "/data/save", "/data/save/file.torrent"))

	updated, err := pipeline.GetCandidate(ctx, candidate.ID)
	require.NoError(t, err)
	assert.True(t, updated.DownloadCompleted)
	assert.Equal(t, model.CandidateCompleted, updated.PublishStatus)

	candidates, err := pipeline.ListPendingCandidates(ctx, 10)
	require.NoError(t, err)
	assert.Equal(t, 0, len(candidates), "completed candidate should not be pending")

	require.NoError(t, pipeline.DeleteCandidate(ctx, candidate.ID))

	_, err = pipeline.GetCandidate(ctx, candidate.ID)
	assert.Error(t, err)

	t.Logf("PASS: candidate lifecycle operations completed")
}

func TestE2E_PublishResult_Records(t *testing.T) {
	db, ctx := setupTestEnv(t)
	pipeline := publish.NewPipeline(db, nopLogger())

	result1 := &model.PublishResultRecord{
		CandidateID: 1, SourceSite: "source-site", TargetSite: "target-site",
		TorrentID: "pub-001", Status: model.PublishResultCompleted,
		PublishURL: "https://target.com/t/pub-001",
	}
	require.NoError(t, pipeline.CreateResult(ctx, result1))

	result2 := &model.PublishResultRecord{
		CandidateID: 1, SourceSite: "source-site", TargetSite: "target-b",
		TorrentID: "pub-002", Status: model.PublishResultFailed,
		ErrorMessage: "upload error",
	}
	require.NoError(t, pipeline.CreateResult(ctx, result2))

	results, err := pipeline.ListResults(ctx, 1, 10)
	require.NoError(t, err)
	assert.Len(t, results, 2)

	allResults, err := pipeline.ListResults(ctx, 0, 0)
	require.NoError(t, err)
	assert.Len(t, allResults, 2)

	t.Logf("PASS: result records created and listed, count=%d", len(results))
}

func TestE2E_TransitionGroupLifecycle(t *testing.T) {
	db, ctx := setupTestEnv(t)
	pipeline := publish.NewPipeline(db, nopLogger())

	group, err := pipeline.CreateGroup(ctx, 1, "lchash", "source-site", "t-001")
	require.NoError(t, err)

	m1 := &model.PublishGroupMember{SiteName: "target-a", Role: "target", Status: model.MemberStatusUploaded}
	require.NoError(t, pipeline.AddGroupMember(ctx, group.ID, m1))
	m2 := &model.PublishGroupMember{SiteName: "target-b", Role: "target", Status: model.MemberStatusUploaded}
	require.NoError(t, pipeline.AddGroupMember(ctx, group.ID, m2))

	require.NoError(t, pipeline.TransitionGroupLifecycle(ctx, group.ID))

	updated, err := pipeline.GetGroup(ctx, group.ID)
	require.NoError(t, err)
	assert.Equal(t, model.GroupMonitoring, updated.Status)
	t.Logf("PASS: all members uploaded → group monitoring, status=%s", updated.Status)
}

func TestE2E_TransitionGroupLifecycle_Failed(t *testing.T) {
	db, ctx := setupTestEnv(t)
	pipeline := publish.NewPipeline(db, nopLogger())

	group, err := pipeline.CreateGroup(ctx, 1, "failhash", "source-site", "t-002")
	require.NoError(t, err)

	m1 := &model.PublishGroupMember{SiteName: "target-a", Role: "target", Status: model.MemberStatusError}
	require.NoError(t, pipeline.AddGroupMember(ctx, group.ID, m1))

	require.NoError(t, pipeline.TransitionGroupLifecycle(ctx, group.ID))

	updated, err := pipeline.GetGroup(ctx, group.ID)
	require.NoError(t, err)
	assert.Equal(t, model.GroupPublishFailed, updated.Status)
	t.Logf("PASS: member error → group publish_failed, status=%s", updated.Status)
}

func TestE2E_PublishTask_CRUD(t *testing.T) {
	db, ctx := setupTestEnv(t)
	pipeline := publish.NewPipeline(db, nopLogger())

	task := &model.PublishTask{
		Type:         model.PublishTaskTypeManual,
		SourceSiteID: 1,
		TargetSites:  []string{"target-a", "target-b"},
		ManualCheck:  true,
	}
	require.NoError(t, pipeline.CreateTask(ctx, task))
	assert.Equal(t, model.PublishTaskPending, task.Status)

	fetched, err := pipeline.GetTask(ctx, task.ID)
	require.NoError(t, err)
	assert.Equal(t, model.PublishTaskTypeManual, fetched.Type)

	tasks, total, err := pipeline.ListTasks(ctx, 0, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, tasks, 1)

	require.NoError(t, pipeline.UpdateTaskStatus(ctx, task.ID, model.PublishTaskChecked))

	updated, err := pipeline.GetTask(ctx, task.ID)
	require.NoError(t, err)
	assert.Equal(t, model.PublishTaskChecked, updated.Status)

	task.ManualCheck = false
	require.NoError(t, pipeline.Update(ctx, task))

	updated2, err := pipeline.GetTask(ctx, task.ID)
	require.NoError(t, err)
	assert.False(t, updated2.ManualCheck)

	t.Logf("PASS: task CRUD operations, taskID=%d status=%s", task.ID, updated2.Status)
}

func TestE2E_UsingMakeDefaultSiteProvider(t *testing.T) {
	db, ctx := setupTestEnv(t)
	seedSite(t, db, "source.com", "source-site")
	seedSite(t, db, "target.com", "target-site")
	seedClient(t, db, "source-client", "seeding")

	candidate := &model.PublishCandidate{
		SourceSite:      "source-site",
		SourceTorrentID: "torrent-helper",
		InfoHash:        "helper123",
		TorrentName:     "Helper.Release.2024",
		Size:            4000000000,
		ClientID:        "source-client",
		TargetSites:     "target-site",
		PublishStatus:   model.CandidatePending,
	}
	require.NoError(t, db.Create(candidate).Error)

	uploadCalled := false
	pipeline := publish.NewPipeline(db, nopLogger())
	pipeline.SetSiteProvider(makeDefaultSiteProvider(&uploadCalled))

	result, err := pipeline.PublishCandidate(ctx, candidate.ID)
	require.NoError(t, err)
	assert.Equal(t, model.CandidateDone, result.PublishStatus)
	assert.True(t, uploadCalled)
	t.Logf("PASS: publish via makeDefaultSiteProvider, status=%s", result.PublishStatus)
}

func TestE2E_FieldMapping(t *testing.T) {
	db, ctx := setupTestEnv(t)
	seedSite(t, db, "source.com", "source-site")
	seedSite(t, db, "target.com", "target-site")

	mapping := &model.SiteFieldMapping{
		SiteName:    "target-site",
		FieldType:   "cat",
		SourceValue: "movie",
		TargetValue: "401",
	}
	require.NoError(t, db.Create(mapping).Error)

	candidate := &model.PublishCandidate{
		SourceSite:      "source-site",
		SourceTorrentID: "torrent-mapped",
		InfoHash:        "mapped123",
		TorrentName:     "Mapped.Release.2024",
		Size:            4000000000,
		PublishStatus:   model.CandidatePending,
		TargetSites:     "target-site",
	}
	require.NoError(t, db.Create(candidate).Error)

	var capturedCategory string
	mockProvider := &mocks.SiteInfoProvider{
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
					return &model.TorrentDetail{Title: "Mapped.Release.2024", Category: "movie"}, nil
				},
				SearchTorrentsFn: func(ctx context.Context, config *model.SiteConfig, query string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
					return nil, nil
				},
				UploadTorrentFn: func(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
					capturedCategory = req.FormFields["category"]
					return &model.PublishResponse{TorrentID: "pub-map-001", DetailURL: "https://target.com/t/pub-map-001"}, nil
				},
			}, nil
		},
	}

	pipeline := publish.NewPipeline(db, nopLogger())
	pipeline.SetSiteProvider(mockProvider)

	result, err := pipeline.PublishCandidate(ctx, candidate.ID)
	require.NoError(t, err)
	assert.Equal(t, model.CandidateDone, result.PublishStatus)
	assert.Equal(t, "401", capturedCategory, "category should be mapped from movie to 401")
	t.Logf("PASS: field mapping applied, category=%s", capturedCategory)
}
