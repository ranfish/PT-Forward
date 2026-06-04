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
	FetchUserStats(ctx context.Context, config *SiteConfig) (*UserStatsResult, error)
}

type CombinedHRDiscountDetector interface {
	DetectHRAndDiscount(ctx context.Context, config *SiteConfig, torrentID string) (*HRResult, *DiscountResult, error)
}

type UserStatsResult struct {
	Username      string
	UserClass     string
	UploadBytes   int64
	DownloadBytes int64
	Ratio         float64
	BonusPoints   float64
	SeedingPoints float64
	SeedingSize   int64
	SeedingCount  int
	Passkey       string
	RSSKey        string
	AuthKey       string
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

type GlobalTransferStats struct {
	AllTimeUpload   int64
	AllTimeDownload int64
}

type DownloaderClient interface {
	GetName() string
	GetRole() string
	GetReseedTargetID() string
	GetID() uint
	GetSharedPaths() []SharedPathMapping

	GetTorrentByHash(ctx context.Context, hash string) (*TorrentInfo, error)
	GetSeedingTorrents(ctx context.Context) ([]*TorrentInfo, error)
	GetAllTorrents(ctx context.Context) ([]*TorrentInfo, error)
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
	GetGlobalTransferStats(ctx context.Context) (*GlobalTransferStats, error)
	GetTrackerMessages(ctx context.Context, hash string) (string, error)
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

type FingerprintRepository interface {
	GetByInfoHash(ctx context.Context, infoHash string) (*ContentFingerprint, error)
	Save(ctx context.Context, fp *ContentFingerprint) error
	GetSearchCache(ctx context.Context, site, cleanTitle string, totalSize int64) (*SearchCache, error)
	SaveSearchCache(ctx context.Context, site, cleanTitle string, totalSize int64, results []Candidate) error
	BatchSave(ctx context.Context, fps []*ContentFingerprint) error
}

type ReseedTaskRepository interface {
	GetTask(ctx context.Context, id uint) (*ReseedTask, error)
	ListTasks(ctx context.Context) ([]ReseedTask, error)
	ListByClientID(ctx context.Context, clientID string) ([]ReseedTask, error)
	ListEnabled(ctx context.Context) ([]ReseedTask, error)
	CreateTask(ctx context.Context, task *ReseedTask) error
	UpdateTask(ctx context.Context, task *ReseedTask) error
	DeleteTask(ctx context.Context, id uint) error
}

type ReseedMatchRepository interface {
	BatchSaveMatches(ctx context.Context, matches []*ReseedMatch) error
	FindMatchesByInfoHash(ctx context.Context, infoHash string) ([]ReseedMatch, error)
	FindPendingRetry(ctx context.Context, limit int) ([]ReseedMatch, error)
	UpdateMatchStatus(ctx context.Context, id uint, status string, failReason string) error
}

type SeedingRuleRepository interface {
	ListConfigs(ctx context.Context) ([]*SeedingClientConfig, error)
	GetConfigByID(ctx context.Context, id uint) (*SeedingClientConfig, error)
	CreateConfig(ctx context.Context, cfg *SeedingClientConfig) error
	UpdateConfig(ctx context.Context, cfg *SeedingClientConfig) error
	DeleteConfig(ctx context.Context, id uint) error
}

type PublishTaskRepository interface {
	ListTasks(ctx context.Context, offset, limit int) ([]PublishTask, int64, error)
	GetTask(ctx context.Context, id uint) (*PublishTask, error)
	CreateTask(ctx context.Context, task *PublishTask) error
	Update(ctx context.Context, task *PublishTask) error
	UpdateTaskStatus(ctx context.Context, id uint, status PublishTaskStatus) error
}
