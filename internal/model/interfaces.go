package model

import (
	"context"
	"time"
)

// §33.2.1 SeedingCollector — Dispatcher → §18 核心桥梁（M1 补充）
type SeedingCollector interface {
	Add(ctx context.Context, clientID string, event *TorrentEvent) error
	Flush(ctx context.Context, subscriptionID string) ([]*SeedingCandidate, error)
	Clear(ctx context.Context, subscriptionID string) error
}

// §33.2.2 SeedingDataProvider — 刷流 S/L 数据提供者
type SeedingDataProvider interface {
	GetBatchSLData(ctx context.Context, torrentIDs []string) (map[string]*SLData, error)
	GetPreciseSLData(ctx context.Context, torrentID string) (*SLData, error)
}

// SiteDataProvider 将 SiteAdapter 的 SLData 方法包装为 SeedingDataProvider 接口
// 实现：internal/seeding/provider.go
type SiteDataProvider struct {
	adapter SiteAdapter
	config  *SiteConfig
}

func NewSiteDataProvider(adapter SiteAdapter, config *SiteConfig) *SiteDataProvider {
	return &SiteDataProvider{adapter: adapter, config: config}
}

func (p *SiteDataProvider) GetBatchSLData(ctx context.Context, torrentIDs []string) (map[string]*SLData, error) {
	return p.adapter.GetBatchSLData(ctx, p.config, torrentIDs)
}

func (p *SiteDataProvider) GetPreciseSLData(ctx context.Context, torrentID string) (*SLData, error) {
	return p.adapter.GetPreciseSLData(ctx, p.config, torrentID)
}

// §33.2.3 SiteAdapter — 站点框架适配器（合并 L14928）
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
}

// §33.2.4 EventHandler — RSS → 系统 的核心事件接口
type EventHandler interface {
	OnTorrents(ctx context.Context, events []TorrentEvent) error
}

// §33.2.5 SiteInfoProvider — 站点信息提供者（合并 L5572 + L14908）
type SiteInfoProvider interface {
	GetSiteInfo(ctx context.Context, siteName string) (*SiteInfo, error)
	GetSiteConfig(ctx context.Context, domain string) (*SiteConfig, error)
	GetSiteDefault(ctx context.Context, domain string) (*SiteDefault, error)
	GetAdapter(ctx context.Context, domain string) (SiteAdapter, error)
	ListSites(ctx context.Context) ([]*SiteInfo, error)
	GetSiteInfoByURL(ctx context.Context, baseURL string) (*SiteInfo, error)
	DetectFramework(ctx context.Context, domain string) (*DetectResult, error)
}

// §33.2.6 CompletionWatcher — 下载完成检测 + 发布触发
type CompletionWatcher interface {
	Start(ctx context.Context) error
	Stop()
	Watch(ctx context.Context, clientName, infoHash string, candidateID uint) error
	SubmitCandidate(ctx context.Context, candidate PublishCandidate) error
}

// §33.2.7 IYUUService — IYUU 全局服务
type IYUUService interface {
	QueryReseed(ctx context.Context, infoHashes []string) ([]*IYUUReseedResult, error)
	GetSeededSites(ctx context.Context, infoHash string) ([]string, error)
	GetSiteList(ctx context.Context) ([]IYUUSite, error)
	ReportExisting(ctx context.Context, sidList []int) error
	Ping(ctx context.Context) error
	SendNotification(ctx context.Context, text, desp string) error
}

// §33.2.8 ChannelNotifier — 单通道通知实现（per-channel 接口）
type ChannelNotifier interface {
	Send(ctx context.Context, msg FormattedMessage) error
	Test(ctx context.Context) error
}

// §33.2.9 DownloaderClient — 下载器操作 (Problem 6 统一返回值规范)
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

// SubscriptionRepository — 订阅数据访问
// Deprecated: 未实现，使用 rss.Repository 替代。
type SubscriptionRepository interface {
	GetByID(ctx context.Context, id string) (*RSSSubscription, error)
	ListBySite(ctx context.Context, siteName string) ([]*RSSSubscription, error)
	ListActive(ctx context.Context) ([]*RSSSubscription, error)
	Update(ctx context.Context, sub *RSSSubscription) error
}

// ListOptions — 通用列表查询选项
type ListOptions struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}

// §33.2.11 SitePublisher — 站点发布接口
// Deprecated: 未实现，站点发布逻辑通过 adapter.SiteAdapter 接口提供。
type SitePublisher interface {
	Publish(ctx context.Context, req *PublishRequest) (*PublishResponse, error)
	GetUploadForm(ctx context.Context, config *SiteConfig) (*UploadForm, error)
	SearchTorrent(ctx context.Context, keyword string, config *SiteConfig) ([]PublishDedupResult, error)
	GetEditForm(ctx context.Context, torrentID string, config *SiteConfig) (*EditForm, error)
	SubmitEdit(ctx context.Context, torrentID string, newDesc string, config *SiteConfig) error
}

// §33.2.12 Publisher — 发布执行接口（§31.10.10）
type Publisher interface {
	Publish(ctx context.Context, req *PublishRequest) (*PublishResponse, error)
}

// §33.2.13 SourceFetcher — 源站抓取接口
// Deprecated: 未实现，源站抓取逻辑通过 adapter.SiteAdapter.FetchDetail/DownloadTorrent 替代。
type SourceFetcher interface {
	FetchDetail(ctx context.Context, site *SiteConfig, torrentID string, cookie string) (*RawTorrent, error)
	DownloadTorrent(ctx context.Context, site *SiteConfig, torrentID string, cookie string, passkey string) ([]byte, error)
	VerifyExists(ctx context.Context, site *SiteConfig, torrentID string, cookie string) (bool, error)
}

// §33.2.14 TorrentInfoFetcher — 源站种子信息抓取（§7.10）
type TorrentInfoFetcher interface {
	FetchDetail(ctx context.Context, siteName, torrentID string) (*TorrentDetail, error)
}

// §33.2.15 PTGenProvider — PTGen 元数据查询（§10.4）
type PTGenProvider interface {
	Query(ctx context.Context, query string) (*PTGenResult, error)
	SupportsNameSearch() bool
}

// §33.2.16 DiscountDetector — 免费折扣检测（§17.9b.5）
type DiscountDetector interface {
	Detect(ctx context.Context, siteName string, torrentID string) (*DiscountResult, error)
}

// §33.2.17 SiteSearcher — 站点搜索（§27.4）
type SiteSearcher interface {
	Search(ctx context.Context, site *SiteInfo, query string, opts *SearchOptions) ([]*SeedingSearchResult, error)
}

// §33.2.18 DownloaderProvider — 下载器实例提供者（§22）
type DownloaderProvider interface {
	Get(clientID string) (DownloaderClient, error)
	ListClients() []string
}

// §33.2.19 SiteConfigProvider — 站点运行时配置提供者（§22）
type SiteConfigProvider interface {
	Get(domain string) (*SiteConfig, error)
}

// §33.2.20 SiteAdapterProvider — 站点框架适配器提供者（§22）
type SiteAdapterProvider interface {
	Get(domain string) (SiteAdapter, error)
}

// §33.2.21 DescriptionRenderer — BBCode/Markdown/HTML 描述渲染器（§31.10.22）
type DescriptionRenderer interface {
	Render(data *DescriptionData, config SiteDescConfig) (string, error)
	FormatMediaInfo(rawText string, format MediaInfoFormat) string
	FormatScreenshots(urls []string) string
	Format() string
}

// §33.2.22 FingerprintRepository — 内容指纹数据访问（Sprint 82 补齐）
type FingerprintRepository interface {
	GetByInfoHash(ctx context.Context, infoHash string) (*ContentFingerprint, error)
	Save(ctx context.Context, fp *ContentFingerprint) error
	GetSearchCache(ctx context.Context, site, cleanTitle string, totalSize int64) (*SearchCache, error)
	SaveSearchCache(ctx context.Context, site, cleanTitle string, totalSize int64, results []Candidate) error
	BatchSave(ctx context.Context, matches []*ReseedMatch) error
}

// §33.2.23 NegativeCache — 辅种负面清单缓存
// Deprecated: 未实现，负面清单通过 GORM 直接操作 ReseedNegativeCache 表替代。
type NegativeCache interface {
	Set(ctx context.Context, sourceInfoHash string, targetSite string, reason string, ttl time.Duration) error
	GetBySourceHashes(ctx context.Context, hashes []string) ([]ReseedNegativeCache, error)
	Flush(ctx context.Context) error
	DeleteBySourceHash(ctx context.Context, sourceInfoHash string) error
}

// §33.2.24 ReseedTaskRepository — 辅种任务数据访问（Sprint 89 P1-6 补齐定义）
type ReseedTaskRepository interface {
	GetByID(ctx context.Context, id uint) (*ReseedTask, error)
	List(ctx context.Context) ([]ReseedTask, error)
	ListByClientID(ctx context.Context, clientID string) ([]ReseedTask, error)
	ListEnabled(ctx context.Context) ([]ReseedTask, error)
	Create(ctx context.Context, task *ReseedTask) error
	Update(ctx context.Context, task *ReseedTask) error
	Delete(ctx context.Context, id uint) error
}

// §33.2.25 ReseedMatchRepository — 辅种匹配结果数据访问（Sprint 89 P1-6 补齐定义）
type ReseedMatchRepository interface {
	BatchSave(ctx context.Context, matches []*ReseedMatch) error
	FindByInfoHash(ctx context.Context, infoHash string) ([]ReseedMatch, error)
	FindPendingRetry(ctx context.Context, limit int) ([]ReseedMatch, error)
	UpdateStatus(ctx context.Context, id uint, status string, failReason string) error
}

// §33.2.26 AuthRepository — 认证数据访问（Sprint 97b：新增）
type AuthRepository interface {
	GetByUsername(ctx context.Context, username string) (*User, error)
	Create(ctx context.Context, user *User) error
	Update(ctx context.Context, user *User) error
	UpdatePassword(ctx context.Context, userID uint, hash string) error
}

// §33.2.27 NotificationChannelRepository — 通知通道 CRUD（Sprint 97b：新增）
type NotificationChannelRepository interface {
	List(ctx context.Context) ([]*NotificationChannel, error)
	GetByID(ctx context.Context, id uint) (*NotificationChannel, error)
	Create(ctx context.Context, ch *NotificationChannel) error
	Update(ctx context.Context, ch *NotificationChannel) error
	Delete(ctx context.Context, id uint) error
}

// §33.2.28 FilterRuleRepository — 过滤规则 CRUD（Sprint 97b：新增）
type FilterRuleRepository interface {
	List(ctx context.Context) ([]*FilterRule, error)
	GetByID(ctx context.Context, id uint) (*FilterRule, error)
	Create(ctx context.Context, rule *FilterRule) error
	Update(ctx context.Context, rule *FilterRule) error
	Delete(ctx context.Context, id uint) error
}

// §33.2.29 SeedingRuleRepository — 刷流规则 CRUD（Sprint 97b：新增）
type SeedingRuleRepository interface {
	List(ctx context.Context) ([]*SeedingClientConfig, error)
	GetByID(ctx context.Context, id uint) (*SeedingClientConfig, error)
	Create(ctx context.Context, cfg *SeedingClientConfig) error
	Update(ctx context.Context, cfg *SeedingClientConfig) error
	Delete(ctx context.Context, id uint) error
}

// §33.2.30 PublishTaskRepository — 发布任务数据访问（Sprint 97b：新增）
type PublishTaskRepository interface {
	List(ctx context.Context, opts ListOptions) ([]*PublishTask, int64, error)
	GetByID(ctx context.Context, id uint) (*PublishTask, error)
	Create(ctx context.Context, task *PublishTask) error
	Update(ctx context.Context, task *PublishTask) error
	UpdateStatus(ctx context.Context, id uint, status PublishTaskStatus) error
}

// §33.2.31 SettingRepository — 系统配置数据访问（Sprint 97b：新增）
type SettingRepository interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
	Delete(ctx context.Context, key string) error
	ListByPrefix(ctx context.Context, prefix string) (map[string]string, error)
}

// §33.2.32 TaskRegistryEntry — 定时任务条目
type TaskRegistryEntry struct {
	Name         string
	Type         string
	Schedule     string
	Handler      func(ctx context.Context) error
	Status       TaskStatus
	LastRunAt    *time.Time
	LastError    string
	SuccessCount int64
	ErrorCount   int64
}

// TaskRegistryInterface — 定时任务注册表接口（实现在 internal/scheduler/registry.go）
type TaskRegistryInterface interface {
	Register(name, taskType, schedule string, handler func(ctx context.Context) error) error
	Unregister(name string) error
	Trigger(ctx context.Context, name string) error
	Pause(name string) error
	Resume(name string) error
	PauseAll() error
	List() []*TaskRegistryEntry
	Get(name string) (*TaskRegistryEntry, error)
	Start(ctx context.Context) error
	Stop() error
}

// ── Helper Types ──────────────────────────────────────────────────────

// §33.1.30 NotifyService — 通知服务（Sprint 77 补齐）
type NotifyService interface {
	Send(ctx context.Context, msg FormattedMessage) error
	Dispatch(ctx context.Context, event string, msg FormattedMessage) error
	Test(ctx context.Context) error
}

// §33.1.31 SeedingEngine — 刷流引擎（Sprint 77 补齐）
type SeedingEngine interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	AddSeedingRecord(record *SeedingTorrentRecord) error
	RemoveSeedingRecord(clientID, infoHash string) error
	GetActiveCount(clientID string) int
}

// AdapterPoolInterface — 站点适配器生命周期管理接口
type AdapterPoolInterface interface {
	Start(ctx context.Context) error
	Get(ctx context.Context, domain string) (SiteAdapter, error)
	Rebuild(ctx context.Context, domain string) error
	Remove(domain string)
	Close(ctx context.Context) error
}
