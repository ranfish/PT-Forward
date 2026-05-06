package model

import (
	"context"
)

type SiteAdapter interface {
	Framework() string
	ParseRSS(ctx context.Context, feedURL string, config *SiteConfig) ([]*RSSTorrentEvent, error)
	DownloadTorrent(ctx context.Context, config *SiteConfig, torrentID string) ([]byte, error)
	GetTorrentDetail(ctx context.Context, config *SiteConfig, torrentID string) (*TorrentDetail, error)
	GetBatchSLData(ctx context.Context, config *SiteConfig, torrentIDs []string) (map[string]*SLData, error)
	GetPreciseSLData(ctx context.Context, config *SiteConfig, torrentID string) (*SLData, error)
	DetectDiscount(ctx context.Context, config *SiteConfig, torrentID string) (*DiscountResult, error)
	DetectHR(ctx context.Context, config *SiteConfig, torrentID string) (*HRResult, error)
	UploadTorrent(ctx context.Context, config *SiteConfig, req *PublishRequest) (*PublishResponse, error)
	SearchTorrents(ctx context.Context, config *SiteConfig, query string, opts *SearchOptions) ([]*SeedingSearchResult, error)
	GetTorrentInfoHash(ctx context.Context, config *SiteConfig, torrentID string) (string, error)
	SupportsSearchByPiecesHash() bool
	VerifyExists(ctx context.Context, config *SiteConfig, torrentID string) (bool, error)
}

type EventHandler interface {
	OnTorrents(ctx context.Context, events []TorrentEvent) error
}

type SiteInfoProvider interface {
	GetSiteInfo(ctx context.Context, siteName string) (*SiteInfo, error)
	GetSiteConfig(ctx context.Context, domain string) (*SiteConfig, error)
	GetSiteDefault(ctx context.Context, domain string) (*SiteDefault, error)
	GetAdapter(ctx context.Context, domain string) (SiteAdapter, error)
	ListSites(ctx context.Context) ([]*SiteInfo, error)
	GetSiteInfoByURL(ctx context.Context, baseURL string) (*SiteInfo, error)
	DetectFramework(ctx context.Context, domain string) (*DetectResult, error)
}

type CompletionWatcher interface {
	Start(ctx context.Context) error
	Stop()
	Watch(ctx context.Context, clientName, infoHash string, candidateID uint) error
	SubmitCandidate(ctx context.Context, candidate PublishCandidate) error
}

type IYUUService interface {
	QueryReseed(ctx context.Context, infoHashes []string) ([]*IYUUReseedResult, error)
	GetSeededSites(ctx context.Context, infoHash string) ([]string, error)
	GetSiteList(ctx context.Context) ([]IYUUSite, error)
	ReportExisting(ctx context.Context, sidList []int) error
	Ping(ctx context.Context) error
	SendNotification(ctx context.Context, text, desp string) error
}

type DownloaderClient interface {
	GetName() string
	GetRole() string
	GetReseedTargetID() string
	GetID() uint
	GetSharedPaths() []SharedPathMapping

	GetTorrentByHash(ctx context.Context, hash string) (*TorrentInfo, error)
	GetSeedingTorrents(ctx context.Context) ([]*TorrentInfo, error)
	GetTorrentsByPath(ctx context.Context, savePath string) ([]*TorrentInfo, error)
	GetMainData(ctx context.Context) (*Maindata, error)
	GetMainDataIncremental(ctx context.Context, rid int) (*Maindata, int, error)

	AddFromFile(ctx context.Context, data []byte, opts AddTorrentOptions) (*AddResult, error)
	ExportTorrent(ctx context.Context, hash string) ([]byte, error)

	DeleteTorrent(ctx context.Context, hash string, deleteFiles bool) error
	BatchDeleteTorrents(ctx context.Context, hashes []string, deleteFiles bool) error
	PauseTorrent(ctx context.Context, hash string) error
	ResumeTorrent(ctx context.Context, hash string) error
	Reannounce(ctx context.Context, hash string) error
	Recheck(ctx context.Context, hash string) error

	SetTorrentTags(ctx context.Context, hash string, tags []string) error
	RemoveTorrentTags(ctx context.Context, hash string, tags []string) error
	SetCategory(ctx context.Context, hash string, category string) error
	SetSavePath(ctx context.Context, hash string, savePath string) error
	SetSuperSeeding(ctx context.Context, hash string, enable bool) error
	SetUploadLimit(ctx context.Context, infoHash string, limitBytesPerSec int64) error

	PauseAllDownloads(ctx context.Context) error
	ResumeAllDownloads(ctx context.Context) error
	GetFreeSpace(ctx context.Context) (int64, error)

	CheckExists(ctx context.Context, infoHash string) (bool, error)
}

type DownloaderProvider interface {
	Get(clientID string) (DownloaderClient, error)
	ListClients() []string
}

type AuthRepository interface {
	GetByUsername(ctx context.Context, username string) (*User, error)
	Create(ctx context.Context, user *User) error
	Update(ctx context.Context, user *User) error
	UpdatePassword(ctx context.Context, userID uint, hash string) error
}

type SeedingEngine interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	AddSeedingRecord(ctx context.Context, record *SeedingTorrentRecord) error
	RemoveSeedingRecord(ctx context.Context, clientID, infoHash string) error
	GetActiveCount(clientID string) int
}

type ContentFingerprintRepository interface {
	GetByInfoHashAndSite(ctx context.Context, infoHash, siteName string) (*ContentFingerprint, error)
	Save(ctx context.Context, fp *ContentFingerprint) error
}

type SeedingCollector interface {
	Add(ctx context.Context, clientID string, event *TorrentEvent) error
	Flush(ctx context.Context, subscriptionID string) ([]*SeedingCandidate, error)
	Clear(ctx context.Context, subscriptionID string) error
}

type ChannelNotifier interface {
	Send(ctx context.Context, msg FormattedMessage) error
	Test(ctx context.Context) error
}

type SitePublisher interface {
	Publish(ctx context.Context, req *PublishRequest) (*PublishResponse, error)
	GetUploadForm(ctx context.Context, config *SiteConfig) (*UploadForm, error)
	SearchTorrent(ctx context.Context, keyword string, config *SiteConfig) ([]PublishDedupResult, error)
	GetEditForm(ctx context.Context, torrentID string, config *SiteConfig) (*EditForm, error)
	SubmitEdit(ctx context.Context, torrentID string, newDesc string, config *SiteConfig) error
}

type Publisher interface {
	Publish(ctx context.Context, req *PublishRequest) (*PublishResponse, error)
}

type SourceFetcher interface {
	FetchDetail(ctx context.Context, site *SiteConfig, torrentID string, cookie string) (*RawTorrent, error)
	DownloadTorrent(ctx context.Context, site *SiteConfig, torrentID string, cookie string, passkey string) ([]byte, error)
	VerifyExists(ctx context.Context, site *SiteConfig, torrentID string, cookie string) (bool, error)
}

type FingerprintRepository interface {
	GetByInfoHash(ctx context.Context, infoHash string) (*ContentFingerprint, error)
	Save(ctx context.Context, fp *ContentFingerprint) error
	GetSearchCache(ctx context.Context, site, cleanTitle string, totalSize int64) (*SearchCache, error)
	SaveSearchCache(ctx context.Context, site, cleanTitle string, totalSize int64, results []Candidate) error
	BatchSave(ctx context.Context, fps []*ContentFingerprint) error
}

type ReseedTaskRepository interface {
	GetByID(ctx context.Context, id uint) (*ReseedTask, error)
	List(ctx context.Context) ([]ReseedTask, error)
	ListByClientID(ctx context.Context, clientID string) ([]ReseedTask, error)
	ListEnabled(ctx context.Context) ([]ReseedTask, error)
	Create(ctx context.Context, task *ReseedTask) error
	Update(ctx context.Context, task *ReseedTask) error
	Delete(ctx context.Context, id uint) error
}

type ReseedMatchRepository interface {
	BatchSave(ctx context.Context, matches []*ReseedMatch) error
	FindByInfoHash(ctx context.Context, infoHash string) ([]ReseedMatch, error)
	FindPendingRetry(ctx context.Context, limit int) ([]ReseedMatch, error)
	UpdateStatus(ctx context.Context, id uint, status string, failReason string) error
}

type SeedingRuleRepository interface {
	List(ctx context.Context) ([]*SeedingClientConfig, error)
	GetByID(ctx context.Context, id uint) (*SeedingClientConfig, error)
	Create(ctx context.Context, cfg *SeedingClientConfig) error
	Update(ctx context.Context, cfg *SeedingClientConfig) error
	Delete(ctx context.Context, id uint) error
}

type PublishTaskRepository interface {
	List(ctx context.Context, opts ListOptions) ([]*PublishTask, int64, error)
	GetByID(ctx context.Context, id uint) (*PublishTask, error)
	Create(ctx context.Context, task *PublishTask) error
	Update(ctx context.Context, task *PublishTask) error
	UpdateStatus(ctx context.Context, id uint, status PublishTaskStatus) error
}
