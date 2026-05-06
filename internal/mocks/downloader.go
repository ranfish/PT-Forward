package mocks

import (
	"context"

	"github.com/ranfish/pt-forward/internal/model"
)

type DownloaderClient struct {
	Name           string
	Role           string
	ReseedTargetID string
	ID             uint
	FreeSpaceVal   int64

	GetTorrentByHashFn       func(ctx context.Context, hash string) (*model.TorrentInfo, error)
	GetSeedingTorrentsFn     func(ctx context.Context) ([]*model.TorrentInfo, error)
	GetTorrentsByPathFn      func(ctx context.Context, savePath string) ([]*model.TorrentInfo, error)
	GetMainDataFn            func(ctx context.Context) (*model.Maindata, error)
	GetMainDataIncrementalFn func(ctx context.Context, rid int) (*model.Maindata, int, error)
	AddFromFileFn            func(ctx context.Context, data []byte, opts model.AddTorrentOptions) (*model.AddResult, error)
	ExportTorrentFn          func(ctx context.Context, hash string) ([]byte, error)
	DeleteTorrentFn          func(ctx context.Context, hash string, deleteFiles bool) error
	BatchDeleteTorrentsFn    func(ctx context.Context, hashes []string, deleteFiles bool) error
	PauseTorrentFn           func(ctx context.Context, hash string) error
	ResumeTorrentFn          func(ctx context.Context, hash string) error
	ReannounceFn             func(ctx context.Context, hash string) error
	RecheckFn                func(ctx context.Context, hash string) error
	SetTorrentTagsFn         func(ctx context.Context, hash string, tags []string) error
	RemoveTorrentTagsFn      func(ctx context.Context, hash string, tags []string) error
	SetCategoryFn            func(ctx context.Context, hash string, category string) error
	SetSavePathFn            func(ctx context.Context, hash string, savePath string) error
	SetSuperSeedingFn        func(ctx context.Context, hash string, enable bool) error
	SetUploadLimitFn         func(ctx context.Context, infoHash string, limitBytesPerSec int64) error
	PauseAllDownloadsFn      func(ctx context.Context) error
	ResumeAllDownloadsFn     func(ctx context.Context) error
	GetFreeSpaceFn           func(ctx context.Context) (int64, error)
	CheckExistsFn            func(ctx context.Context, infoHash string) (bool, error)
}

func (m *DownloaderClient) GetName() string                           { return m.Name }
func (m *DownloaderClient) GetRole() string                           { return m.Role }
func (m *DownloaderClient) GetReseedTargetID() string                 { return m.ReseedTargetID }
func (m *DownloaderClient) GetID() uint                               { return m.ID }
func (m *DownloaderClient) GetSharedPaths() []model.SharedPathMapping { return nil }

func (m *DownloaderClient) GetTorrentByHash(ctx context.Context, hash string) (*model.TorrentInfo, error) {
	if m.GetTorrentByHashFn != nil {
		return m.GetTorrentByHashFn(ctx, hash)
	}
	return nil, nil
}

func (m *DownloaderClient) GetSeedingTorrents(ctx context.Context) ([]*model.TorrentInfo, error) {
	if m.GetSeedingTorrentsFn != nil {
		return m.GetSeedingTorrentsFn(ctx)
	}
	return nil, nil
}

func (m *DownloaderClient) GetTorrentsByPath(ctx context.Context, savePath string) ([]*model.TorrentInfo, error) {
	if m.GetTorrentsByPathFn != nil {
		return m.GetTorrentsByPathFn(ctx, savePath)
	}
	return nil, nil
}

func (m *DownloaderClient) GetMainData(ctx context.Context) (*model.Maindata, error) {
	if m.GetMainDataFn != nil {
		return m.GetMainDataFn(ctx)
	}
	return nil, nil
}

func (m *DownloaderClient) GetMainDataIncremental(ctx context.Context, rid int) (*model.Maindata, int, error) {
	if m.GetMainDataIncrementalFn != nil {
		return m.GetMainDataIncrementalFn(ctx, rid)
	}
	return nil, 0, nil
}

func (m *DownloaderClient) AddFromFile(ctx context.Context, data []byte, opts model.AddTorrentOptions) (*model.AddResult, error) {
	if m.AddFromFileFn != nil {
		return m.AddFromFileFn(ctx, data, opts)
	}
	return nil, nil
}

func (m *DownloaderClient) ExportTorrent(ctx context.Context, hash string) ([]byte, error) {
	if m.ExportTorrentFn != nil {
		return m.ExportTorrentFn(ctx, hash)
	}
	return nil, nil
}

func (m *DownloaderClient) DeleteTorrent(ctx context.Context, hash string, deleteFiles bool) error {
	if m.DeleteTorrentFn != nil {
		return m.DeleteTorrentFn(ctx, hash, deleteFiles)
	}
	return nil
}

func (m *DownloaderClient) BatchDeleteTorrents(ctx context.Context, hashes []string, deleteFiles bool) error {
	if m.BatchDeleteTorrentsFn != nil {
		return m.BatchDeleteTorrentsFn(ctx, hashes, deleteFiles)
	}
	return nil
}

func (m *DownloaderClient) PauseTorrent(ctx context.Context, hash string) error {
	if m.PauseTorrentFn != nil {
		return m.PauseTorrentFn(ctx, hash)
	}
	return nil
}

func (m *DownloaderClient) ResumeTorrent(ctx context.Context, hash string) error {
	if m.ResumeTorrentFn != nil {
		return m.ResumeTorrentFn(ctx, hash)
	}
	return nil
}

func (m *DownloaderClient) Reannounce(ctx context.Context, hash string) error {
	if m.ReannounceFn != nil {
		return m.ReannounceFn(ctx, hash)
	}
	return nil
}

func (m *DownloaderClient) Recheck(ctx context.Context, hash string) error {
	if m.RecheckFn != nil {
		return m.RecheckFn(ctx, hash)
	}
	return nil
}

func (m *DownloaderClient) SetTorrentTags(ctx context.Context, hash string, tags []string) error {
	if m.SetTorrentTagsFn != nil {
		return m.SetTorrentTagsFn(ctx, hash, tags)
	}
	return nil
}

func (m *DownloaderClient) RemoveTorrentTags(ctx context.Context, hash string, tags []string) error {
	if m.RemoveTorrentTagsFn != nil {
		return m.RemoveTorrentTagsFn(ctx, hash, tags)
	}
	return nil
}

func (m *DownloaderClient) SetCategory(ctx context.Context, hash string, category string) error {
	if m.SetCategoryFn != nil {
		return m.SetCategoryFn(ctx, hash, category)
	}
	return nil
}

func (m *DownloaderClient) SetSavePath(ctx context.Context, hash string, savePath string) error {
	if m.SetSavePathFn != nil {
		return m.SetSavePathFn(ctx, hash, savePath)
	}
	return nil
}

func (m *DownloaderClient) SetSuperSeeding(ctx context.Context, hash string, enable bool) error {
	if m.SetSuperSeedingFn != nil {
		return m.SetSuperSeedingFn(ctx, hash, enable)
	}
	return nil
}

func (m *DownloaderClient) SetUploadLimit(ctx context.Context, infoHash string, limitBytesPerSec int64) error {
	if m.SetUploadLimitFn != nil {
		return m.SetUploadLimitFn(ctx, infoHash, limitBytesPerSec)
	}
	return nil
}

func (m *DownloaderClient) PauseAllDownloads(ctx context.Context) error {
	if m.PauseAllDownloadsFn != nil {
		return m.PauseAllDownloadsFn(ctx)
	}
	return nil
}

func (m *DownloaderClient) ResumeAllDownloads(ctx context.Context) error {
	if m.ResumeAllDownloadsFn != nil {
		return m.ResumeAllDownloadsFn(ctx)
	}
	return nil
}

func (m *DownloaderClient) GetFreeSpace(ctx context.Context) (int64, error) {
	if m.GetFreeSpaceFn != nil {
		return m.GetFreeSpaceFn(ctx)
	}
	return m.FreeSpaceVal, nil
}

func (m *DownloaderClient) CheckExists(ctx context.Context, infoHash string) (bool, error) {
	if m.CheckExistsFn != nil {
		return m.CheckExistsFn(ctx, infoHash)
	}
	return false, nil
}

type DownloaderProvider struct {
	Client        model.DownloaderClient
	GetFn         func(clientID string) (model.DownloaderClient, error)
	ListClientsFn func() []string
}

func (p *DownloaderProvider) Get(clientID string) (model.DownloaderClient, error) {
	if p.GetFn != nil {
		return p.GetFn(clientID)
	}
	if p.Client != nil {
		return p.Client, nil
	}
	return nil, nil
}

func (p *DownloaderProvider) ListClients() []string {
	if p.ListClientsFn != nil {
		return p.ListClientsFn()
	}
	return nil
}
