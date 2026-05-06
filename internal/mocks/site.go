package mocks

import (
	"context"

	"github.com/ranfish/pt-forward/internal/model"
)

type SiteAdapter struct {
	FrameworkVal string

	ParseRSSFn                    func(ctx context.Context, feedURL string, config *model.SiteConfig) ([]*model.RSSTorrentEvent, error)
	DownloadTorrentFn             func(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error)
	GetTorrentDetailFn            func(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.TorrentDetail, error)
	GetBatchSLDataFn              func(ctx context.Context, config *model.SiteConfig, torrentIDs []string) (map[string]*model.SLData, error)
	GetPreciseSLDataFn            func(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.SLData, error)
	DetectDiscountFn              func(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.DiscountResult, error)
	DetectHRFn                    func(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.HRResult, error)
	UploadTorrentFn               func(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error)
	SearchTorrentsFn              func(ctx context.Context, config *model.SiteConfig, query string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error)
	GetTorrentInfoHashFn          func(ctx context.Context, config *model.SiteConfig, torrentID string) (string, error)
	SupportsSearchByPiecesHashVal bool
	VerifyExistsFn                func(ctx context.Context, config *model.SiteConfig, torrentID string) (bool, error)
}

func (m *SiteAdapter) Framework() string {
	if m.FrameworkVal != "" {
		return m.FrameworkVal
	}
	return "mock"
}

func (m *SiteAdapter) ParseRSS(ctx context.Context, feedURL string, config *model.SiteConfig) ([]*model.RSSTorrentEvent, error) {
	if m.ParseRSSFn != nil {
		return m.ParseRSSFn(ctx, feedURL, config)
	}
	return nil, nil
}

func (m *SiteAdapter) DownloadTorrent(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error) {
	if m.DownloadTorrentFn != nil {
		return m.DownloadTorrentFn(ctx, config, torrentID)
	}
	return nil, nil
}

func (m *SiteAdapter) GetTorrentDetail(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.TorrentDetail, error) {
	if m.GetTorrentDetailFn != nil {
		return m.GetTorrentDetailFn(ctx, config, torrentID)
	}
	return nil, nil
}

func (m *SiteAdapter) GetBatchSLData(ctx context.Context, config *model.SiteConfig, torrentIDs []string) (map[string]*model.SLData, error) {
	if m.GetBatchSLDataFn != nil {
		return m.GetBatchSLDataFn(ctx, config, torrentIDs)
	}
	return nil, nil
}

func (m *SiteAdapter) GetPreciseSLData(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.SLData, error) {
	if m.GetPreciseSLDataFn != nil {
		return m.GetPreciseSLDataFn(ctx, config, torrentID)
	}
	return nil, nil
}

func (m *SiteAdapter) DetectDiscount(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.DiscountResult, error) {
	if m.DetectDiscountFn != nil {
		return m.DetectDiscountFn(ctx, config, torrentID)
	}
	return nil, nil
}

func (m *SiteAdapter) DetectHR(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.HRResult, error) {
	if m.DetectHRFn != nil {
		return m.DetectHRFn(ctx, config, torrentID)
	}
	return nil, nil
}

func (m *SiteAdapter) UploadTorrent(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
	if m.UploadTorrentFn != nil {
		return m.UploadTorrentFn(ctx, config, req)
	}
	return nil, nil
}

func (m *SiteAdapter) SearchTorrents(ctx context.Context, config *model.SiteConfig, query string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
	if m.SearchTorrentsFn != nil {
		return m.SearchTorrentsFn(ctx, config, query, opts)
	}
	return nil, nil
}

func (m *SiteAdapter) GetTorrentInfoHash(ctx context.Context, config *model.SiteConfig, torrentID string) (string, error) {
	if m.GetTorrentInfoHashFn != nil {
		return m.GetTorrentInfoHashFn(ctx, config, torrentID)
	}
	return "", nil
}

func (m *SiteAdapter) SupportsSearchByPiecesHash() bool { return m.SupportsSearchByPiecesHashVal }

func (m *SiteAdapter) VerifyExists(ctx context.Context, config *model.SiteConfig, torrentID string) (bool, error) {
	if m.VerifyExistsFn != nil {
		return m.VerifyExistsFn(ctx, config, torrentID)
	}
	return false, nil
}

type SiteInfoProvider struct {
	GetSiteInfoFn      func(ctx context.Context, siteName string) (*model.SiteInfo, error)
	GetSiteConfigFn    func(ctx context.Context, domain string) (*model.SiteConfig, error)
	GetSiteDefaultFn   func(ctx context.Context, domain string) (*model.SiteDefault, error)
	GetAdapterFn       func(ctx context.Context, domain string) (model.SiteAdapter, error)
	ListSitesFn        func(ctx context.Context) ([]*model.SiteInfo, error)
	GetSiteInfoByURLFn func(ctx context.Context, baseURL string) (*model.SiteInfo, error)
	DetectFrameworkFn  func(ctx context.Context, domain string) (*model.DetectResult, error)
}

func (m *SiteInfoProvider) GetSiteInfo(ctx context.Context, siteName string) (*model.SiteInfo, error) {
	if m.GetSiteInfoFn != nil {
		return m.GetSiteInfoFn(ctx, siteName)
	}
	return nil, nil
}

func (m *SiteInfoProvider) GetSiteConfig(ctx context.Context, domain string) (*model.SiteConfig, error) {
	if m.GetSiteConfigFn != nil {
		return m.GetSiteConfigFn(ctx, domain)
	}
	return nil, nil
}

func (m *SiteInfoProvider) GetSiteDefault(ctx context.Context, domain string) (*model.SiteDefault, error) {
	if m.GetSiteDefaultFn != nil {
		return m.GetSiteDefaultFn(ctx, domain)
	}
	return nil, nil
}

func (m *SiteInfoProvider) GetAdapter(ctx context.Context, domain string) (model.SiteAdapter, error) {
	if m.GetAdapterFn != nil {
		return m.GetAdapterFn(ctx, domain)
	}
	return nil, nil
}

func (m *SiteInfoProvider) ListSites(ctx context.Context) ([]*model.SiteInfo, error) {
	if m.ListSitesFn != nil {
		return m.ListSitesFn(ctx)
	}
	return nil, nil
}

func (m *SiteInfoProvider) GetSiteInfoByURL(ctx context.Context, baseURL string) (*model.SiteInfo, error) {
	if m.GetSiteInfoByURLFn != nil {
		return m.GetSiteInfoByURLFn(ctx, baseURL)
	}
	return nil, nil
}

func (m *SiteInfoProvider) DetectFramework(ctx context.Context, domain string) (*model.DetectResult, error) {
	if m.DetectFrameworkFn != nil {
		return m.DetectFrameworkFn(ctx, domain)
	}
	return nil, nil
}
