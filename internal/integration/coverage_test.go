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
		Discount: model.DiscountFree,
	}}

	require.NoError(t, td.OnTorrents(ctx, events))

	seedingCount := countRecords(t, db, "seeding_torrent_records")
	assert.Equal(t, int64(1), seedingCount)

	candidateCount := countRecords(t, db, "publish_candidates")
	assert.Equal(t, int64(0), candidateCount, "no candidate when no matched rule")
	t.Logf("PASS: no matched rule → seeding only, candidates=%d", candidateCount)
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
	evt.Discount = model.DiscountFree
	events := []model.TorrentEvent{evt, evt}

	require.NoError(t, td.OnTorrents(ctx, events))

	seedingCount := countRecords(t, db, "seeding_torrent_records")
	assert.Equal(t, int64(1), seedingCount, "duplicate events should produce single record")
	t.Logf("PASS: duplicate events handled, records=%d", seedingCount)
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
