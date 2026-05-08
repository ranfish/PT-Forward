package integration

import (
	"context"
	"testing"

	"github.com/ranfish/pt-forward/internal/mocks"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := model.AutoMigrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func nopLogger() *zap.Logger {
	return zap.NewNop()
}

func countRecords(t *testing.T, db *gorm.DB, table string) int64 {
	t.Helper()
	var count int64
	if err := db.Table(table).Count(&count).Error; err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return count
}

func setupTestEnv(t *testing.T) (*gorm.DB, context.Context) {
	t.Helper()
	return setupDB(t), context.Background()
}

func makeSite(domain, name string) *model.Site {
	return &model.Site{
		Name: name, Domain: domain,
		BaseURL: "https://" + domain, AuthType: "cookie",
	}
}

func makeClient(name, role string) *model.ClientConfig {
	return &model.ClientConfig{
		Name: name, Type: "qbittorrent",
		Role: role, Enabled: true, URL: "http://localhost:8080",
	}
}

func makeTorrentEvent(siteName, torrentID, title string, size int64, infoHash string) model.TorrentEvent {
	return model.TorrentEvent{
		SiteName: siteName, TorrentID: torrentID,
		Title: title, Size: size, InfoHash: infoHash,
	}
}

func makeDefaultSiteProvider(uploadCalled *bool) *mocks.SiteInfoProvider {
	return &mocks.SiteInfoProvider{
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
						Title: "Test Torrent", Category: "movie",
						Source: "BluRay", Resolution: "2160p", Codec: "x264",
						ReleaseGroup: "GROUP", Description: "test desc",
						MediaInfo: "media info text",
					}, nil
				},
				SearchTorrentsFn: func(ctx context.Context, config *model.SiteConfig, query string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
					return nil, nil
				},
				UploadTorrentFn: func(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
					if uploadCalled != nil {
						*uploadCalled = true
					}
					return &model.PublishResponse{
						TorrentID: "pub-auto-001",
						DetailURL: "https://target.com/torrents/pub-auto-001",
					}, nil
				},
			}, nil
		},
	}
}
