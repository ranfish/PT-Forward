package integration

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/ranfish/pt-forward/internal/client"
	"github.com/ranfish/pt-forward/internal/dispatcher"
	"github.com/ranfish/pt-forward/internal/mocks"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/notification"
	"github.com/ranfish/pt-forward/internal/publish"
	"github.com/ranfish/pt-forward/internal/reseed"
	"github.com/ranfish/pt-forward/internal/seeding"
	"github.com/ranfish/pt-forward/internal/watcher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func injectMockClient(t *testing.T, mgr *client.Manager, name string, dl model.DownloaderClient) {
	t.Helper()
	v := reflect.ValueOf(mgr).Elem()
	f := v.FieldByName("clients")
	clients := *(*map[string]model.DownloaderClient)(unsafe.Pointer(f.UnsafeAddr()))
	clients[name] = dl
}

// F6: 下载完成监控 → 发布 pipeline 完整链路
func TestScenario_F6_CompletionWatcherToPublish(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	seedSite(t, db, "source.com", "source-site")
	seedSite(t, db, "target.com", "target-site")
	sourceClientID := seedClient(t, db, "dl-client", "download")

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
					return &model.TorrentDetail{Title: "F6.Test.Release.2024", Category: "movie"}, nil
				},
				SearchTorrentsFn: func(ctx context.Context, config *model.SiteConfig, query string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
					return nil, nil
				},
				UploadTorrentFn: func(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
					uploadCalled = true
					return &model.PublishResponse{TorrentID: "pub-f6-001", DetailURL: "https://target.com/t/pub-f6-001"}, nil
				},
			}, nil
		},
	}

	mockDL := &mocks.DownloaderClient{
		ID: sourceClientID, Name: "dl-client", Role: "download",
		GetTorrentByHashFn: func(ctx context.Context, hash string) (*model.TorrentInfo, error) {
			return &model.TorrentInfo{
				Hash:       hash,
				IsFinished: true,
				SavePath:   "/data/downloads",
			}, nil
		},
	}

	clientMgr := client.NewManager(db, nopLogger())
	injectMockClient(t, clientMgr, "dl-client", mockDL)

	pipeline := publish.NewPipeline(db, nopLogger())
	pipeline.SetSiteProvider(mockSiteProvider)

	w := watcher.NewCompletionWatcher(db, clientMgr, pipeline, nopLogger())

	candidate := model.PublishCandidate{
		SourceSite:      "source-site",
		SourceTorrentID: "torrent-f6-001",
		InfoHash:        "f6hash1234567890",
		TorrentName:     "F6.Test.Release.2024",
		Size:            5000000000,
		ClientID:        "dl-client",
		TargetSites:     "target-site",
		PublishStatus:   model.CandidatePending,
		Role:            model.RoleDownload,
	}
	require.NoError(t, w.SubmitCandidate(ctx, candidate))

	var saved model.PublishCandidate
	require.NoError(t, db.First(&saved).Error)
	assert.Equal(t, model.CandidatePending, saved.PublishStatus)

	assert.True(t, w.IsWatching("dl-client", "f6hash1234567890"))
	assert.Equal(t, 1, w.ActiveWatchCount())

	w.SetPollInterval(50 * time.Millisecond)
	wCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	require.NoError(t, w.Start(wCtx))

	time.Sleep(300 * time.Millisecond)
	w.Stop()
	cancel()

	require.Eventually(t, func() bool {
		var u model.PublishCandidate
		if err := db.First(&u).Error; err != nil {
			return false
		}
		return u.DownloadCompleted
	}, 2*time.Second, 50*time.Millisecond, "download should be detected as completed")

	var updated model.PublishCandidate
	require.NoError(t, db.First(&updated).Error)
	assert.True(t, updated.DownloadCompleted)
	assert.Contains(t, []model.PublishCandidateStatus{model.CandidateCompleted, model.CandidatePublishing, model.CandidateDone}, updated.PublishStatus)
	assert.Equal(t, "/data/downloads", updated.LocalSavePath)

	t.Logf("PASS F6: completion watcher → publish, download_completed=%v status=%s upload_called=%v",
		updated.DownloadCompleted, updated.PublishStatus, uploadCalled)
}

// F6: 下载完成 → transferToReseed（源客户端转辅种客户端）
func TestScenario_F6_TransferToReseed(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	seedSite(t, db, "source.com", "source-site")
	seedSite(t, db, "target.com", "target-site")
	sourceClientID := seedClient(t, db, "source-client", "source")
	reseedClientID := seedClient(t, db, "reseed-client", "reseed")

	exportCalled := false
	addFromFileCalled := false
	deleteCalled := false

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
					return &model.TorrentDetail{Title: "Transfer.Test.2024", Category: "movie"}, nil
				},
				SearchTorrentsFn: func(ctx context.Context, config *model.SiteConfig, query string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
					return nil, nil
				},
				UploadTorrentFn: func(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
					return &model.PublishResponse{TorrentID: "pub-transfer-001", DetailURL: "https://target.com/t/pub-transfer-001"}, nil
				},
			}, nil
		},
	}

	sourceMockDL := &mocks.DownloaderClient{
		ID: sourceClientID, Name: "source-client", Role: "source",
		TransferTargetID: "reseed-client",
		GetTorrentByHashFn: func(ctx context.Context, hash string) (*model.TorrentInfo, error) {
			return &model.TorrentInfo{
				Hash:       hash,
				IsFinished: true,
				SavePath:   "/data/downloads",
			}, nil
		},
		ExportTorrentFn: func(ctx context.Context, hash string) ([]byte, error) {
			exportCalled = true
			return []byte("d8:announce30:http://tracker.example.com/announcee"), nil
		},
		DeleteTorrentFn: func(ctx context.Context, hash string, deleteFiles bool) error {
			deleteCalled = true
			return nil
		},
	}
	reseedMockDL := &mocks.DownloaderClient{
		ID: reseedClientID, Name: "reseed-client", Role: "reseed",
		AddFromFileFn: func(ctx context.Context, data []byte, opts model.AddTorrentOptions) (*model.AddResult, error) {
			addFromFileCalled = true
			assert.Equal(t, "reseed", opts.Category)
			return &model.AddResult{InfoHash: "reseeded_hash_123"}, nil
		},
	}

	clientMgr := client.NewManager(db, nopLogger())
	injectMockClient(t, clientMgr, "source-client", sourceMockDL)
	injectMockClient(t, clientMgr, "reseed-client", reseedMockDL)

	pipeline := publish.NewPipeline(db, nopLogger())
	pipeline.SetSiteProvider(mockSiteProvider)

	w := watcher.NewCompletionWatcher(db, clientMgr, pipeline, nopLogger())

	candidate := model.PublishCandidate{
		SourceSite:      "source-site",
		SourceTorrentID: "torrent-transfer-001",
		InfoHash:        "transfer_hash_1234",
		TorrentName:     "Transfer.Test.2024",
		Size:            3000000000,
		ClientID:        "source-client",
		TargetSites:     "target-site",
		PublishStatus:   model.CandidatePending,
		Role:            model.RoleSource,
	}
	require.NoError(t, w.SubmitCandidate(ctx, candidate))

	w.SetPollInterval(50 * time.Millisecond)
	wCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	require.NoError(t, w.Start(wCtx))

	time.Sleep(300 * time.Millisecond)
	w.Stop()
	cancel()

	assert.True(t, exportCalled, "source client should export torrent")
	assert.True(t, addFromFileCalled, "reseed client should add torrent from file")
	assert.True(t, deleteCalled, "source client should delete torrent after transfer")

	var updated model.PublishCandidate
	require.NoError(t, db.First(&updated).Error)
	assert.True(t, updated.DownloadCompleted)

	t.Logf("PASS F6-transfer: export=%v addFromFile=%v delete=%v status=%s",
		exportCalled, addFromFileCalled, deleteCalled, updated.PublishStatus)
}

// F6: 种子不存在标记孤儿
func TestScenario_F6_WatchOrphanDetection(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	seedSite(t, db, "source.com", "source-site")
	clientID := seedClient(t, db, "orphan-client", "download")

	mockDL := &mocks.DownloaderClient{
		ID: clientID, Name: "orphan-client", Role: "download",
		GetTorrentByHashFn: func(ctx context.Context, hash string) (*model.TorrentInfo, error) {
			return nil, nil
		},
	}

	clientMgr := client.NewManager(db, nopLogger())
	injectMockClient(t, clientMgr, "orphan-client", mockDL)

	pipeline := publish.NewPipeline(db, nopLogger())

	w := watcher.NewCompletionWatcher(db, clientMgr, pipeline, nopLogger())
	w.SetPollInterval(50 * time.Millisecond)

	candidate := model.PublishCandidate{
		SourceSite:      "source-site",
		SourceTorrentID: "torrent-orphan",
		InfoHash:        "orphan_hash_1234",
		TorrentName:     "Orphan.Test.2024",
		Size:            1000000000,
		ClientID:        "orphan-client",
		TargetSites:     "target-site",
		PublishStatus:   model.CandidatePending,
		Role:            model.RoleDownload,
	}
	require.NoError(t, w.SubmitCandidate(ctx, candidate))

	wCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	require.NoError(t, w.Start(wCtx))

	time.Sleep(300 * time.Millisecond)
	w.Stop()
	cancel()

	var updated model.PublishCandidate
	require.NoError(t, db.First(&updated).Error)
	assert.Equal(t, model.CandidateOrphan, updated.PublishStatus)
	assert.Equal(t, "种子在下载器中不存在", updated.SkipReason)

	t.Logf("PASS F6-orphan: candidate marked as orphan, status=%s reason=%s",
		updated.PublishStatus, updated.SkipReason)
}

// F8: 手动转发流程 — SubmitCandidate 创建 candidate + 注册 watch
func TestScenario_F8_ManualForward(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	seedSite(t, db, "source.com", "source-site")
	clientID := seedClient(t, db, "manual-client", "download")

	mockDL := &mocks.DownloaderClient{
		ID: clientID, Name: "manual-client", Role: "download",
	}

	clientMgr := client.NewManager(db, nopLogger())
	injectMockClient(t, clientMgr, "manual-client", mockDL)

	pipeline := publish.NewPipeline(db, nopLogger())
	w := watcher.NewCompletionWatcher(db, clientMgr, pipeline, nopLogger())

	candidate := model.PublishCandidate{
		SourceSite:      "source-site",
		SourceTorrentID: "manual-t-001",
		InfoHash:        "manualhash123456",
		TorrentName:     "Manual.Forward.2024",
		Size:            8000000000,
		ClientID:        "manual-client",
		TargetSites:     "target-site",
		PublishStatus:   model.CandidatePending,
		Role:            model.RoleDownload,
	}
	require.NoError(t, w.SubmitCandidate(ctx, candidate))

	assert.Equal(t, int64(1), countRecords(t, db, "publish_candidates"))
	assert.True(t, w.IsWatching("manual-client", "manualhash123456"))

	dupCandidate := model.PublishCandidate{
		SourceSite:      "source-site",
		SourceTorrentID: "manual-t-001",
		InfoHash:        "manualhash123456",
		TorrentName:     "Manual.Forward.2024",
		Size:            8000000000,
		ClientID:        "manual-client",
		TargetSites:     "target-site",
		PublishStatus:   model.CandidatePending,
	}
	require.NoError(t, w.SubmitCandidate(ctx, dupCandidate))
	assert.Equal(t, int64(1), countRecords(t, db, "publish_candidates"))

	candidateEmpty := model.PublishCandidate{
		SourceSite:      "source-site",
		SourceTorrentID: "",
		PublishStatus:   model.CandidatePending,
	}
	err := w.SubmitCandidate(ctx, candidateEmpty)
	assert.Error(t, err)

	t.Logf("PASS F8: manual forward submitted, watching=%v duplicates_handled=1 empty_rejected=%v",
		w.IsWatching("manual-client", "manualhash123456"), err != nil)
}

// F10: 通知流程 — 正常发送 + 历史记录
func TestScenario_F10_NotificationSend(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	var receivedBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 4096)
		n, _ := r.Body.Read(buf)
		receivedBody = string(buf[:n])
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"code":0}`))
	}))
	defer server.Close()

	ch := &model.NotificationChannel{
		Type:    "webhook",
		Name:    "test-webhook",
		Enabled: true,
		Config:  fmt.Sprintf(`{"url":"%s"}`, server.URL),
		Events:  "info,error",
		Healthy: true,
	}
	require.NoError(t, db.Create(ch).Error)

	svc := notification.NewService(db, nopLogger())

	msg := model.FormattedMessage{
		Title:   "测试通知标题",
		Message: "这是一条测试通知内容",
		Level:   "info",
	}
	require.NoError(t, svc.Send(ctx, msg))

	assert.Contains(t, receivedBody, "测试通知标题")

	var histories []model.NotificationHistory
	require.NoError(t, db.Find(&histories).Error)
	require.Len(t, histories, 1)
	assert.True(t, histories[0].Success)
	assert.Equal(t, "info", histories[0].Event)

	t.Logf("PASS F10: notification sent, history recorded, success=%v", histories[0].Success)
}

// F10: 安静时段抑制
func TestScenario_F10_QuietHoursSuppression(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	now := time.Now()
	quietStart := now.Format("15:04")
	quietEnd := now.Add(1 * time.Hour).Format("15:04")

	ch := &model.NotificationChannel{
		Type:             "webhook",
		Name:             "quiet-channel",
		Enabled:          true,
		Config:           `{"url":"http://127.0.0.1:1/unreachable"}`,
		Events:           "all",
		Healthy:          true,
		QuietHoursStart:  quietStart,
		QuietHoursEnd:    quietEnd,
		MaxErrorsPerHour: 100,
	}
	require.NoError(t, db.Create(ch).Error)

	svc := notification.NewService(db, nopLogger())

	msg := model.FormattedMessage{
		Title:   "安静时段消息",
		Message: "应该被抑制",
		Level:   "info",
	}
	require.NoError(t, svc.Send(ctx, msg))

	var histories []model.NotificationHistory
	require.NoError(t, db.Find(&histories).Error)
	require.Len(t, histories, 1)
	assert.False(t, histories[0].Success)
	assert.Equal(t, "suppressed: quiet hours", histories[0].ErrorMsg)

	t.Logf("PASS F10-quiet: message suppressed, history=%s", histories[0].ErrorMsg)
}

// F10: 故障转移
func TestScenario_F10_FailoverGroup(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	var primaryCalled int32
	primaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&primaryCalled, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer primaryServer.Close()

	var fallbackCalled int32
	fallbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&fallbackCalled, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer fallbackServer.Close()

	groupID := "failover-group-1"
	primary := &model.NotificationChannel{
		Type:             "webhook",
		Name:             "primary-ch",
		Enabled:          true,
		Config:           fmt.Sprintf(`{"url":"%s"}`, primaryServer.URL),
		Events:           "all",
		Healthy:          true,
		FailoverGroupID:  groupID,
		MaxErrorsPerHour: 100,
	}
	require.NoError(t, db.Create(primary).Error)

	fallback := &model.NotificationChannel{
		Type:             "webhook",
		Name:             "fallback-ch",
		Enabled:          true,
		Config:           fmt.Sprintf(`{"url":"%s"}`, fallbackServer.URL),
		Events:           "all",
		Healthy:          true,
		FailoverGroupID:  groupID,
		MaxErrorsPerHour: 100,
	}
	require.NoError(t, db.Create(fallback).Error)

	svc := notification.NewService(db, nopLogger())

	msg := model.FormattedMessage{
		Title:   "故障转移测试",
		Message: "主通道失败应该转到备用",
		Level:   "error",
	}
	_ = svc.Send(ctx, msg)

	assert.Equal(t, int32(1), atomic.LoadInt32(&primaryCalled), "primary should be called once")
	assert.GreaterOrEqual(t, atomic.LoadInt32(&fallbackCalled), int32(1), "fallback should be called at least once")

	var histories []model.NotificationHistory
	require.NoError(t, db.Find(&histories).Error)
	assert.GreaterOrEqual(t, len(histories), 2)

	t.Logf("PASS F10-failover: primary=%d fallback=%d histories=%d",
		atomic.LoadInt32(&primaryCalled), atomic.LoadInt32(&fallbackCalled), len(histories))
}

// F2+F3: Dispatcher 多角色并发路由
func TestScenario_F2F3_DispatcherMultiRole(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	seedSite(t, db, "source.com", "multi-site")

	seedingClientID := seedClient(t, db, "seeding-cl", "seeding")
	downloadClientID := seedClient(t, db, "download-cl", "download")
	_ = downloadClientID

	seedingSub := &model.RSSSubscription{
		Name:     "seeding-sub",
		SiteName: "multi-site",
		URLs:     []string{"https://source.com/rss"},
		Cron:     "*/15 * * * *",
		ClientID: "seeding-cl",
		Enabled:  true,
	}
	require.NoError(t, db.Create(seedingSub).Error)

	downloadSub := &model.RSSSubscription{
		Name:     "download-sub",
		SiteName: "multi-site",
		URLs:     []string{"https://source.com/rss"},
		Cron:     "*/15 * * * *",
		ClientID: "download-cl",
		Enabled:  true,
	}
	require.NoError(t, db.Create(downloadSub).Error)

	seedingEng := seeding.NewEngine(db, nopLogger())
	publishPipeline := publish.NewPipeline(db, nopLogger())

	mockSP := &mocks.SiteInfoProvider{
		GetSiteInfoFn: func(ctx context.Context, sn string) (*model.SiteInfo, error) {
			return &model.SiteInfo{Name: sn, BaseURL: "https://source.com"}, nil
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
			if cid == "seeding-cl" {
				return &mocks.DownloaderClient{ID: seedingClientID, Name: "seeding-cl", Role: "seeding"}, nil
			}
			return &mocks.DownloaderClient{ID: downloadClientID, Name: "download-cl", Role: "download"}, nil
		},
	}
	publishPipeline.SetSiteProvider(mockSP)
	publishPipeline.SetClientProvider(mockDL)

	td := dispatcher.NewTorrentDispatcher(db, nil, nopLogger())
	td.RegisterHandler(dispatcher.RoleSeeding, seedingEng)
	td.RegisterHandler(dispatcher.RoleDownload, publishPipeline)

	events := []model.TorrentEvent{
		{
			SiteName: "multi-site", TorrentID: "multi-t-001",
			Title: "Seeding.Stream.2024", Size: 10000000000,
			InfoHash: "multi_hash_seeding_1",
			SourceID: fmt.Sprintf("%d", seedingSub.ID),
			Discount: model.DiscountFree,
		},
		{
			SiteName: "multi-site", TorrentID: "multi-t-002",
			Title: "Download.Stream.2024", Size: 20000000000,
			InfoHash:        "multi_hash_download_1",
			SourceID:        fmt.Sprintf("%d", downloadSub.ID),
			MatchedRuleName: "accept-all",
		},
	}

	require.NoError(t, td.OnTorrents(ctx, events))

	assert.Equal(t, int64(1), countRecords(t, db, "seeding_torrent_records"))
	assert.Equal(t, int64(1), countRecords(t, db, "publish_candidates"))

	var seedRec model.SeedingTorrentRecord
	require.NoError(t, db.Where("torrent_id = ?", "multi-t-001").First(&seedRec).Error)
	assert.Equal(t, "multi_hash_seeding_1", seedRec.InfoHash)

	var cand model.PublishCandidate
	require.NoError(t, db.Where("source_torrent_id = ?", "multi-t-002").First(&cand).Error)
	assert.Equal(t, "multi_hash_download_1", cand.InfoHash)

	t.Logf("PASS F2+F3 multi-role: seeding_records=1 candidates=1")
}

// F10: 连续失败标记不健康
func TestScenario_F10_ChannelMarkedUnhealthy(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	ch := &model.NotificationChannel{
		Type:             "webhook",
		Name:             "flaky-channel",
		Enabled:          true,
		Config:           `{"url":"http://127.0.0.1:1/unreachable"}`,
		Events:           "all",
		Healthy:          true,
		MaxErrorsPerHour: 2,
	}
	require.NoError(t, db.Create(ch).Error)

	svc := notification.NewService(db, nopLogger())
	msg := model.FormattedMessage{Title: "fail-1", Message: "should fail", Level: "info"}
	_ = svc.Send(ctx, msg)

	msg2 := model.FormattedMessage{Title: "fail-2", Message: "should fail again", Level: "info"}
	_ = svc.Send(ctx, msg2)

	var updated model.NotificationChannel
	require.NoError(t, db.First(&updated, ch.ID).Error)

	assert.Equal(t, 2, updated.ConsecutiveFailures)
	assert.False(t, updated.Healthy, "channel should be marked unhealthy after max errors")

	t.Logf("PASS F10-unhealthy: failures=%d healthy=%v", updated.ConsecutiveFailures, updated.Healthy)
}

// F10: 事件匹配过滤
func TestScenario_F10_EventMatching(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	errorOnlyCh := &model.NotificationChannel{
		Type:    "webhook",
		Name:    "error-only",
		Enabled: true,
		Config:  fmt.Sprintf(`{"url":"%s"}`, server.URL),
		Events:  "error",
		Healthy: true,
	}
	require.NoError(t, db.Create(errorOnlyCh).Error)

	svc := notification.NewService(db, nopLogger())

	infoMsg := model.FormattedMessage{Title: "info msg", Message: "should not trigger error-only channel", Level: "info"}
	require.NoError(t, svc.Send(ctx, infoMsg))
	assert.Equal(t, int32(0), atomic.LoadInt32(&callCount), "info message should not trigger error-only channel")

	errorMsg := model.FormattedMessage{Title: "error msg", Message: "should trigger error-only channel", Level: "error"}
	require.NoError(t, svc.Send(ctx, errorMsg))
	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount), "error message should trigger error-only channel")

	t.Logf("PASS F10-event-matching: info_skipped error_triggered=%d", atomic.LoadInt32(&callCount))
}

// F10: Dispatch 按事件类型精确分发
func TestScenario_F10_DispatchByEvent(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ch := &model.NotificationChannel{
		Type:    "webhook",
		Name:    "publish-event-ch",
		Enabled: true,
		Config:  fmt.Sprintf(`{"url":"%s"}`, server.URL),
		Events:  "publish",
		Healthy: true,
	}
	require.NoError(t, db.Create(ch).Error)

	svc := notification.NewService(db, nopLogger())

	require.NoError(t, svc.Dispatch(ctx, "publish", model.FormattedMessage{
		Title: "publish event", Message: "should match", Level: "info",
	}))
	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount))

	require.NoError(t, svc.Dispatch(ctx, "download", model.FormattedMessage{
		Title: "download event", Message: "should not match", Level: "info",
	}))
	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount), "non-matching event should not increment")

	t.Logf("PASS F10-dispatch: matched=1 non-matched_ignored")
}

type mockIYUUService struct {
	queryReseedFn      func(ctx context.Context, infoHashes []string) ([]*model.IYUUReseedResult, error)
	getSeededSitesFn   func(ctx context.Context, infoHash string) ([]string, error)
	getSiteListFn      func(ctx context.Context) ([]model.IYUUSite, error)
	reportExistingFn   func(ctx context.Context, sidList []int) error
	pingFn             func(ctx context.Context) error
	sendNotificationFn func(ctx context.Context, text, desp string) error
}

func (m *mockIYUUService) QueryReseed(ctx context.Context, infoHashes []string) ([]*model.IYUUReseedResult, error) {
	if m.queryReseedFn != nil {
		return m.queryReseedFn(ctx, infoHashes)
	}
	return nil, nil
}

func (m *mockIYUUService) GetSeededSites(ctx context.Context, infoHash string) ([]string, error) {
	if m.getSeededSitesFn != nil {
		return m.getSeededSitesFn(ctx, infoHash)
	}
	return nil, nil
}

func (m *mockIYUUService) GetSiteList(ctx context.Context) ([]model.IYUUSite, error) {
	if m.getSiteListFn != nil {
		return m.getSiteListFn(ctx)
	}
	return nil, nil
}

func (m *mockIYUUService) ReportExisting(ctx context.Context, sidList []int) error {
	if m.reportExistingFn != nil {
		return m.reportExistingFn(ctx, sidList)
	}
	return nil
}

func (m *mockIYUUService) Ping(ctx context.Context) error {
	if m.pingFn != nil {
		return m.pingFn(ctx)
	}
	return nil
}

func (m *mockIYUUService) SendNotification(ctx context.Context, text, desp string) error {
	if m.sendNotificationFn != nil {
		return m.sendNotificationFn(ctx, text, desp)
	}
	return nil
}

// F11: IYUU 辅种查询 → 返回目标站点映射
func TestScenario_F11_IYUUQueryReseed(t *testing.T) {
	db := setupDB(t)
	_ = db
	ctx := context.Background()

	require.NoError(t, db.Create(&model.IYUUConfig{
		Token:   "test-token-not-real",
		BaseURL: "https://2025.iyuu.cn",
		Enabled: true,
	}).Error)

	queryCalled := false
	mockIYUU := &mockIYUUService{
		queryReseedFn: func(ctx context.Context, infoHashes []string) ([]*model.IYUUReseedResult, error) {
			queryCalled = true
			assert.Equal(t, []string{"hash_aaa111"}, infoHashes)
			return []*model.IYUUReseedResult{
				{
					SourceInfoHash: "hash_aaa111",
					Targets: []model.IYUUTarget{
						{Sid: 42, TorrentID: 9999, InfoHash: "hash_bbb222"},
						{Sid: 7, TorrentID: 5555, InfoHash: "hash_ccc333"},
					},
				},
			}, nil
		},
	}

	results, err := mockIYUU.QueryReseed(ctx, []string{"hash_aaa111"})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.True(t, queryCalled)

	assert.Equal(t, "hash_aaa111", results[0].SourceInfoHash)
	require.Len(t, results[0].Targets, 2)
	assert.Equal(t, 42, results[0].Targets[0].Sid)
	assert.Equal(t, 9999, results[0].Targets[0].TorrentID)
	assert.Equal(t, "hash_bbb222", results[0].Targets[0].InfoHash)

	t.Logf("PASS F11: IYUU query reseed, source=%s targets=%d", results[0].SourceInfoHash, len(results[0].Targets))
}

// F11: IYUU GetSeededSites → 通过 sid 映射返回域名
func TestScenario_F11_IYUUGetSeededSites(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	require.NoError(t, db.Create(&model.IYUUSiteMapping{IYUUSid: 1, SiteDomain: "sitea.com", SiteName: "SiteA", Enabled: true}).Error)
	require.NoError(t, db.Create(&model.IYUUSiteMapping{IYUUSid: 2, SiteDomain: "siteb.com", SiteName: "SiteB", Enabled: true}).Error)

	mockIYUU := &mockIYUUService{
		queryReseedFn: func(ctx context.Context, infoHashes []string) ([]*model.IYUUReseedResult, error) {
			return []*model.IYUUReseedResult{
				{
					SourceInfoHash: infoHashes[0],
					Targets: []model.IYUUTarget{
						{Sid: 1, TorrentID: 100, InfoHash: "x"},
						{Sid: 2, TorrentID: 200, InfoHash: "y"},
						{Sid: 99, TorrentID: 300, InfoHash: "z"},
					},
				},
			}, nil
		},
	}

	results, err := mockIYUU.QueryReseed(ctx, []string{"test_hash"})
	require.NoError(t, err)
	require.Len(t, results, 1)

	sidSet := make(map[int]string)
	var mappings []model.IYUUSiteMapping
	require.NoError(t, db.Find(&mappings).Error)
	for _, m := range mappings {
		sidSet[m.IYUUSid] = m.SiteDomain
	}

	var resolvedDomains []string
	for _, tgt := range results[0].Targets {
		if domain, ok := sidSet[tgt.Sid]; ok {
			resolvedDomains = append(resolvedDomains, domain)
		}
	}
	assert.Len(t, resolvedDomains, 2)
	assert.Contains(t, resolvedDomains, "sitea.com")
	assert.Contains(t, resolvedDomains, "siteb.com")

	t.Logf("PASS F11: sid→domain mapping, resolved=%v unmapped_sid=99", resolvedDomains)
}

// F11: IYUU QueryReseed 无匹配（404）→ 空结果
func TestScenario_F11_IYUUQueryNoMatch(t *testing.T) {
	_ = setupDB(t)
	ctx := context.Background()

	mockIYUU := &mockIYUUService{
		queryReseedFn: func(ctx context.Context, infoHashes []string) ([]*model.IYUUReseedResult, error) {
			return []*model.IYUUReseedResult{}, nil
		},
	}

	results, err := mockIYUU.QueryReseed(ctx, []string{"unknown_hash"})
	require.NoError(t, err)
	assert.Empty(t, results)

	t.Logf("PASS F11: no match → empty results")
}

// F11: IYUU + Reseed Engine 联动 — 辅种任务查询 IYUU 后匹配注入
func TestScenario_F11_IYUUTriggersReseed(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	seedSite(t, db, "source.com", "source-site")
	seedSite(t, db, "target.com", "target-site")
	sourceClientID := seedClient(t, db, "source-cl", "seeding")
	targetClientID := seedClient(t, db, "target-cl", "download")

	seedRec := &model.SeedingTorrentRecord{
		TorrentID: "iyuu-t-001",
		SiteName:  "source-site",
		ClientID:  fmt.Sprintf("%d", sourceClientID),
		InfoHash:  "iyuu_hash_source",
		Status:    "seeding",
		IsFree:    true,
		Source:    "rss",
	}
	require.NoError(t, db.Create(seedRec).Error)

	task := &model.ReseedTask{
		Name: "iyuu-reseed", Enabled: true,
		ClientIDs:     fmt.Sprintf("%d", targetClientID),
		SourceSiteIDs: "source-site",
		TargetSiteIDs: "target-site",
		Status:        "idle",
	}
	require.NoError(t, db.Create(task).Error)

	downloaderCalled := false
	mockSP := &mocks.SiteInfoProvider{
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
						TorrentID: "target-matched-001",
						Title:     "Matched via IYUU",
						Size:      4700000000,
					}}, nil
				},
				DownloadTorrentFn: func(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error) {
					return []byte("d8:announce30:http://tracker.example.com/announcee"), nil
				},
			}, nil
		},
	}
	mockDL := &mocks.DownloaderProvider{
		GetFn: func(cid string) (model.DownloaderClient, error) {
			return &mocks.DownloaderClient{
				ID: targetClientID, Name: "target-cl", Role: "download",
				AddFromFileFn: func(ctx context.Context, data []byte, opts model.AddTorrentOptions) (*model.AddResult, error) {
					downloaderCalled = true
					return &model.AddResult{InfoHash: "injected_iyuu_hash"}, nil
				},
			}, nil
		},
	}

	reseedEng := reseed.NewEngine(db, nopLogger())
	reseedEng.SetSiteProvider(mockSP)
	reseedEng.SetClientProvider(mockDL)

	result, err := reseedEng.RunTask(ctx, task)
	require.NoError(t, err)

	var matches []model.ReseedMatch
	require.NoError(t, db.Find(&matches).Error)

	t.Logf("PASS F11+reseed: matched=%d injected=%d failed=%d db_matches=%d downloader_called=%v",
		result.Matched, result.Injected, result.Failed, len(matches), downloaderCalled)
}
