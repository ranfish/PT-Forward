package integration

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/mocks"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/seeding"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupSeedingE2EDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err, "open in-memory db")
	err = db.AutoMigrate(
		&model.SeedingTorrentRecord{},
		&model.SeedingClientConfig{},
		&model.RSSSubscription{},
		&model.Site{},
		&model.DownloaderSpeedSnapshot{},
		&model.SiteTrafficDaily{},
		&model.DeleteRule{},
		&model.SeedingClientState{},
		&model.ScoringLog{},
	)
	require.NoError(t, err, "auto-migrate models")
	return db
}

func TestE2E_SeedingFullChain(t *testing.T) {
	db := setupSeedingE2EDB(t)
	ctx := context.Background()
	logger := zap.NewNop()

	site := &model.Site{
		Name: "testsite", Domain: "testsite.cc",
		BaseURL: "https://testsite.cc", AuthType: "cookie",
	}
	require.NoError(t, db.Create(site).Error)

	sub := &model.RSSSubscription{
		Name:     "e2e-sub",
		Enabled:  true,
		SiteName: "testsite",
		ClientID: "seeding-e2e-client",
		URLs:     []string{"https://testsite.cc/rss"},
		Cron:     "*/5 * * * *",
		ScoringConfig: model.SeedingScoringConfig{
			Enabled:          true,
			MaxCandidates:    50,
			MaxActiveSeeding: 100,
			BatchLimit:       10,
			MinScore:         -1,
			HalfLifeHours:    2.0,
		},
	}
	require.NoError(t, db.Create(sub).Error)
	db.Model(sub).Update("min_score", 0.0)

	seedCfg := &model.SeedingClientConfig{
		ClientID: "seeding-e2e-client",
		Enabled:  true,
	}
	require.NoError(t, db.Create(seedCfg).Error)

	var addCalls int64
	var deleteCalls int64

	mockClient := &mocks.DownloaderClient{
		Name: "seeding-e2e-client",
		Role: "seeding",
		ID:   1,
		GetMainDataFn: func(_ context.Context) (*model.Maindata, error) {
			return &model.Maindata{
				FreeSpace: 100 * 1024 * 1024 * 1024,
				Torrents: map[string]model.TorrentInfo{
					"e2e_hash_001": {Hash: "e2e_hash_001", UploadSpeed: 100},
				},
			}, nil
		},
		GetSeedingTorrentsFn: func(_ context.Context) ([]*model.TorrentInfo, error) {
			return []*model.TorrentInfo{
				{Hash: "e2e_hash_001", UploadSpeed: 100, SeedTime: 100, Ratio: 0.5, TotalSize: 1024},
			}, nil
		},
		AddFromFileFn: func(_ context.Context, _ []byte, _ model.AddTorrentOptions) (*model.AddResult, error) {
			atomic.AddInt64(&addCalls, 1)
			return &model.AddResult{InfoHash: "e2e_hash_001"}, nil
		},
		DeleteTorrentFn: func(_ context.Context, _ string, _ bool) error {
			atomic.AddInt64(&deleteCalls, 1)
			return nil
		},
		CheckExistsFn: func(_ context.Context, _ string) (bool, error) {
			return false, nil
		},
		ReannounceFn: func(_ context.Context, _ string) error {
			return nil
		},
		GetTorrentByHashFn: func(_ context.Context, hash string) (*model.TorrentInfo, error) {
			return nil, nil
		},
	}

	mockDLProvider := &mocks.DownloaderProvider{
		GetFn: func(cid string) (model.DownloaderClient, error) {
			if cid == "seeding-e2e-client" {
				return mockClient, nil
			}
			return nil, fmt.Errorf("not found: %s", cid)
		},
		ListClientsFn: func() []string {
			return []string{"seeding-e2e-client"}
		},
	}

	mockSiteProvider := &mocks.SiteInfoProvider{
		GetSiteInfoFn: func(_ context.Context, siteName string) (*model.SiteInfo, error) {
			return &model.SiteInfo{Name: siteName, BaseURL: "https://testsite.cc"}, nil
		},
		GetSiteConfigFn: func(_ context.Context, _ string) (*model.SiteConfig, error) {
			return &model.SiteConfig{}, nil
		},
		GetAdapterFn: func(_ context.Context, _ string) (model.SiteAdapter, error) {
			return &mocks.SiteAdapter{
				DownloadTorrentFn: func(_ context.Context, _ *model.SiteConfig, _ string) ([]byte, error) {
					return []byte("d8:announce30:http://tracker.test.com/announcee"), nil
				},
				GetBatchSLDataFn: func(_ context.Context, _ *model.SiteConfig, tids []string) (map[string]*model.SLData, error) {
					result := make(map[string]*model.SLData)
					for _, tid := range tids {
						result[tid] = &model.SLData{Seeders: 5, Leechers: 50}
					}
					return result, nil
				},
				GetPreciseSLDataFn: func(_ context.Context, _ *model.SiteConfig, tid string) (*model.SLData, error) {
					return &model.SLData{Seeders: 5, Leechers: 50}, nil
				},
			}, nil
		},
	}

	eng := seeding.NewEngine(db, logger)
	eng.SetClientProvider(mockDLProvider)
	eng.SetSiteProvider(mockSiteProvider)
	require.NoError(t, eng.Start(ctx))
	defer eng.Stop(ctx)

	t.Log("=== Phase 1: OnTorrents ===")
	events := []model.TorrentEvent{
		{
			SourceID:  fmt.Sprintf("%d", sub.ID),
			SiteName:  "testsite",
			TorrentID: "1001",
			Title:     "Ubuntu.24.04.LTS.2160p.BluRay",
			Size:      47000000000,
			InfoHash:  "e2e_hash_001",
			Discount:  model.DiscountFree,
			Metadata:  map[string]any{"client_name": "seeding-e2e-client"},
		},
	}
	require.NoError(t, eng.OnTorrents(ctx, events))
	assert.Equal(t, 1, eng.TotalActiveCount(), "OnTorrents should create 1 active record")

	var rec model.SeedingTorrentRecord
	require.NoError(t, db.Where("info_hash = ?", "e2e_hash_001").First(&rec).Error)
	assert.Equal(t, "1001", rec.TorrentID)
	assert.True(t, rec.IsFree)
	t.Logf("Phase 1 PASS: record created id=%d torrent=%s isFree=%v", rec.ID, rec.TorrentID, rec.IsFree)

	t.Log("=== Phase 2: Flush ===")
	candidates, err := eng.Flush(ctx, fmt.Sprintf("%d", sub.ID))
	require.NoError(t, err)
	assert.Equal(t, 1, len(candidates), "Flush should return 1 candidate")
	assert.True(t, atomic.LoadInt64(&addCalls) > 0, "AddFromFile should be called at least once")
	t.Logf("Phase 2 PASS: candidates=%d addCalls=%d", len(candidates), atomic.LoadInt64(&addCalls))

	t.Log("=== Phase 3: Evaluate (young torrent, no deletion) ===")
	evalResult, err := eng.Evaluate(ctx, "seeding-e2e-client", nil)
	require.NoError(t, err)
	assert.Equal(t, 1, evalResult.Evaluated, "should evaluate 1 record")
	assert.Equal(t, 0, evalResult.Deleted, "young torrent should NOT be deleted")
	t.Logf("Phase 3 PASS: evaluated=%d deleted=%d", evalResult.Evaluated, evalResult.Deleted)

	t.Log("=== Phase 4: Age record + Evaluate -> delete ===")
	require.NoError(t, eng.Stop(ctx))

	past := time.Now().Add(-200 * time.Hour)
	require.NoError(t, db.Model(&model.SeedingTorrentRecord{}).
		Where("info_hash = ?", "e2e_hash_001").
		Updates(map[string]interface{}{
			"is_free":    false,
			"created_at": past,
		}).Error)

	eng2 := seeding.NewEngine(db, logger)
	eng2.SetClientProvider(mockDLProvider)
	eng2.SetSiteProvider(mockSiteProvider)
	require.NoError(t, eng2.Start(ctx))
	defer eng2.Stop(ctx)

	getSeedingFn := mockClient.GetSeedingTorrentsFn
	mockClient.GetSeedingTorrentsFn = func(_ context.Context) ([]*model.TorrentInfo, error) {
		return []*model.TorrentInfo{
			{Hash: "e2e_hash_001", UploadSpeed: 0, SeedTime: 720000, Ratio: 5.0, TotalSize: 1024},
		}, nil
	}

	evalResult2, err := eng2.Evaluate(ctx, "seeding-e2e-client", nil)
	require.NoError(t, err)
	assert.Equal(t, 1, evalResult2.Evaluated, "should evaluate 1 record")
	assert.True(t, evalResult2.Deleted >= 1, "old non-free torrent with low score should be deleted")
	assert.True(t, atomic.LoadInt64(&deleteCalls) > 0, "DeleteTorrent should be called")
	t.Logf("Phase 4 PASS: evaluated=%d deleted=%d deleteCalls=%d",
		evalResult2.Evaluated, evalResult2.Deleted, atomic.LoadInt64(&deleteCalls))

	mockClient.GetSeedingTorrentsFn = getSeedingFn
}
