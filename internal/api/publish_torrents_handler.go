package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/ranfish/pt-forward/internal/compliance"
	"github.com/ranfish/pt-forward/internal/coverage"
	"github.com/ranfish/pt-forward/internal/fingerprint"
	"github.com/ranfish/pt-forward/internal/metadata"
	"github.com/ranfish/pt-forward/internal/description"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/publish"
	"github.com/ranfish/pt-forward/internal/reseed"
	"github.com/ranfish/pt-forward/internal/site"
	"github.com/ranfish/pt-forward/internal/titleparser"
	"github.com/ranfish/pt-forward/internal/util"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type piecesHashSearcher interface {
	SearchByPiecesHash(ctx context.Context, config *model.SiteConfig, piecesHashes []string) (map[string]int, error)
}

type SiteProviderGetter interface {
	GetAdapter(ctx context.Context, domain string) (model.SiteAdapter, error)
	GetSiteConfig(ctx context.Context, domain string) (*model.SiteConfig, error)
}

type PublishTorrentsHandler struct {
	executor *publish.PublishExecutor // §59.156 切片 2 新发布执行器（复用 pipeline 组件）
	db             *gorm.DB
	coverage       *coverage.Service
	clientMgr      MFClientProvider
	siteProvider   SiteProviderGetter
	sourceDetector *publish.SourceSiteDetector
	declFilter     *publish.DeclarationFilter
	reseedEngine   *reseed.Engine
	metadataFetcher MetadataFetcherProvider
	complianceChecker *compliance.Checker
	seedPipeline       SeedArtifactAnalyzer
	shotStrategy       ScreenshotStrategyRunner
	screenshotCacheDays int // §59.63: 截图链接缓存观察期（天，<=0 关闭）
	ptgen              PTGenAnalyzer
	resourceResolver   *publish.ResourceResolver
	logger         *zap.Logger
	bgState        backgroundQueryState
	batchFetch     batchFetchState
	siteBatch      siteBatchState // §59.166 一站多种批量任务
	strategySem    chan struct{} // §59.58: 截图策略并发额度（批量链挤爆 CPU/代理实测定案）
	posterClusterCtx map[string]posterClusterContext // §59.61 附: infoHash → 簇上下文（异步修复回传用）
}

type backgroundQueryState struct {
	mu          sync.Mutex
	active      map[uint]bool // clientID → querying
	total       int
	done        int
}

type batchFetchState struct {
	mu     sync.Mutex
	active bool
	total  int
	done   int
	failed int
	items  []batchFetchItem
}

type batchFetchItem struct {
	Hash   string `json:"hash"`
	Name   string `json:"name"`
	Status string `json:"status"` // pending / done / failed
	Error  string `json:"error,omitempty"`
}

// ═══ §59.166 一站多种批量发布任务（N 种×1 站——串行+间隔，轮询进度）═══

type siteBatchState struct {
	mu     sync.Mutex
	tasks  map[string]*siteBatchTask // taskID → task（含已完成待 TTL 清理）
	active map[string]string         // targetSite → 运行中 taskID（同站互斥）
}

type siteBatchTask struct {
	ID           string            `json:"task_id"`
	TargetSite   string            `json:"target_site"`
	Total        int               `json:"total"`
	Done         int               `json:"done"`
	CurrentTitle string            `json:"current_title,omitempty"`
	Results      []siteBatchResult `json:"results"`
	Finished     bool              `json:"finished"`
	Error        string            `json:"error,omitempty"`
	StartedAt    time.Time         `json:"started_at"`
	FinishedAt   time.Time         `json:"finished_at,omitempty"`
}

type siteBatchResult struct {
	InfoHash  string `json:"info_hash"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
	TorrentID string `json:"torrent_id,omitempty"`
	URL       string `json:"url,omitempty"`
}

func NewPublishTorrentsHandler(db *gorm.DB, logger *zap.Logger, pipeline *publish.Pipeline) *PublishTorrentsHandler {
	// §59.58: 容量 5——8 核实测每路 mpv ~6 核，5 路摊薄后单种子 ~145s < 4min ctx（60% 余量）。
	// CPU 总量守恒：并发数不改变批量总时长，只消除无界并发导致的 ctx 超时作废功。
	// §59.51 手动截图按钮不经过此信号量（独立单例，不与批量竞争）。
	return &PublishTorrentsHandler{
		db:               db,
		logger:           logger,
		executor:         publish.NewPublishExecutor(pipeline),
		bgState:          backgroundQueryState{active: make(map[uint]bool)},
		siteBatch:        siteBatchState{tasks: map[string]*siteBatchTask{}, active: map[string]string{}},
		resourceResolver: publish.NewResourceResolver(db),
		strategySem:      make(chan struct{}, 5),
		screenshotCacheDays: 30, // §59.63: 观察期默认 30 天（SetScreenshotCacheDays 由 settings 覆盖）
	}
}

// StartObservingCleanup §59.38: 观察期定时清理——日级扫描超 7 天的观察组并
// 执行两级清理（立即清理与定时共用 purgeObservingResource）。启动 +5min 避开
// 启动 IO 高峰，此后每 24h 一轮。
func (h *PublishTorrentsHandler) StartObservingCleanup() {
	go func() {
		time.Sleep(5 * time.Minute)
		h.runObservingCleanup(context.Background())
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			h.runObservingCleanup(context.Background())
		}
	}()
}

func (h *PublishTorrentsHandler) runObservingCleanup(ctx context.Context) {
	cutoff := time.Now().Add(-observingGracePeriod)
	type obsGroup struct {
		ClientID string
		Name     string
	}
	var groups []obsGroup
	h.db.WithContext(ctx).
		Table("torrent_snapshots").
		Select("client_id, name").
		Where("name != '' AND last_seen < ?", cutoff).
		Group("client_id, name").
		Having("SUM(CASE WHEN is_hidden = 0 THEN 1 ELSE 0 END) = 0 AND SUM(CASE WHEN is_hidden = 1 THEN 1 ELSE 0 END) > 0").
		Find(&groups)
	if len(groups) == 0 {
		return
	}
	var totalSnaps, totalMetas int64
	for _, g := range groups {
		res := h.purgeObservingResource(ctx, g.ClientID, g.Name)
		totalSnaps += res.snapRows
		totalMetas += res.metaRows
	}
	h.logger.Info("observing cleanup completed",
		zap.Int("groups", len(groups)),
		zap.Int64("snaps", totalSnaps),
		zap.Int64("metas", totalMetas))
}

func (h *PublishTorrentsHandler) SetCoverageService(s *coverage.Service) { h.coverage = s }
func (h *PublishTorrentsHandler) SetReseedEngine(e *reseed.Engine)     { h.reseedEngine = e }
func (h *PublishTorrentsHandler) SetClientProvider(c MFClientProvider)  { h.clientMgr = c }
func (h *PublishTorrentsHandler) SetSiteProvider(s SiteProviderGetter)  { h.siteProvider = s }
func (h *PublishTorrentsHandler) SetSourceDetector(d *publish.SourceSiteDetector) { h.sourceDetector = d }
func (h *PublishTorrentsHandler) SetDeclarationFilter(f *publish.DeclarationFilter) { h.declFilter = f }
func (h *PublishTorrentsHandler) SetMetadataFetcher(f MetadataFetcherProvider)      { h.metadataFetcher = f }
func (h *PublishTorrentsHandler) SetComplianceChecker(c *compliance.Checker)        { h.complianceChecker = c }
func (h *PublishTorrentsHandler) SetSeedPipeline(p SeedArtifactAnalyzer)             { h.seedPipeline = p }
func (h *PublishTorrentsHandler) SetScreenshotStrategyRunner(p ScreenshotStrategyRunner) { h.shotStrategy = p }

// SetScreenshotCacheDays §59.63: 截图链接缓存观察期（天）。<=0 关闭。
func (h *PublishTorrentsHandler) SetScreenshotCacheDays(days int) { h.screenshotCacheDays = days }
func (h *PublishTorrentsHandler) SetPTGenAnalyzer(p PTGenAnalyzer)                    { h.ptgen = p }

// §59.21: 本地产物分析接口（只跑 mediainfo，不跑截图）
type SeedArtifactAnalyzer interface {
	AnalyzeLocalArtifacts(ctx context.Context, name, savePath string) (map[string]interface{}, error)
}

// §59.53: 采集链截图策略（pipeline 实现）——auto 全策略/远程只转存
type ScreenshotStrategyRunner interface {
	ApplyScreenshotStrategy(ctx context.Context, name, savePath string, sourceScreenshots []string, isLocal bool) []string
}

// PTGenAnalyzer PTGen 查询接口（§59.42 海报 fallback 链用）
type PTGenAnalyzer interface {
	AnalyzePTGen(ctx context.Context, name string) (*model.PTGenResult, error)
}

func (h *PublishTorrentsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimRight(r.URL.Path, "/")

	switch {
	case strings.HasSuffix(path, "/publish/torrents") && r.Method == http.MethodGet:
		h.handleListTorrents(w, r)
	case strings.HasSuffix(path, "/publish/torrents/batch-publish") && r.Method == http.MethodPost:
		h.handleBatchPublish(w, r)
	case strings.HasSuffix(path, "/publish/torrents/coverage") && r.Method == http.MethodPost:
		h.handleQueryCoverage(w, r)
	case strings.HasSuffix(path, "/publish/torrents/batch-coverage") && r.Method == http.MethodPost:
		h.handleBatchQueryCoverage(w, r)
	case strings.HasSuffix(path, "/publish/torrents/query-status") && r.Method == http.MethodGet:
		h.handleQueryStatus(w, r)
	case strings.HasSuffix(path, "/publish/torrents/detect-source") && r.Method == http.MethodPost:
		h.handleDetectSource(w, r)
	case strings.HasSuffix(path, "/publish/torrents/group-mappings") && r.Method == http.MethodGet:
		h.handleListGroupMappings(w, r)
	case strings.HasSuffix(path, "/publish/torrents/group-mappings/sites") && r.Method == http.MethodGet:
		h.handleListGroupedSiteNames(w, r)
	case strings.HasSuffix(path, "/publish/torrents/declaration-filters") && r.Method == http.MethodGet:
		h.handleGetDeclarationFilters(w, r)
	case strings.HasSuffix(path, "/publish/torrents/declaration-filters") && r.Method == http.MethodPut:
		h.handleSetDeclarationFilters(w, r)
	case strings.HasSuffix(path, "/publish/torrents/preview-title") && r.Method == http.MethodPost:
		h.handlePreviewTitle(w, r)
	case strings.HasSuffix(path, "/publish/torrents/preview-title-batch") && r.Method == http.MethodPost:
		h.handlePreviewTitleBatch(w, r)
	case strings.HasSuffix(path, "/publish/torrents/group-mappings") && r.Method == http.MethodPost:
		h.handleCreateGroupMapping(w, r)
	case strings.Contains(path, "/publish/torrents/group-mappings/") && r.Method == http.MethodPut:
		h.handleUpdateGroupMapping(w, r)
	case strings.Contains(path, "/publish/torrents/group-mappings/") && r.Method == http.MethodDelete:
		h.handleDeleteGroupMapping(w, r)
	case strings.HasSuffix(path, "/publish/cached-sites") && r.Method == http.MethodGet:
		h.handleCachedSites(w, r)
	case strings.HasSuffix(path, "/publish/seed-data") && r.Method == http.MethodGet:
		h.handleListSeedData(w, r)
	case strings.HasSuffix(path, "/publish/seed-data/batch-review") && r.Method == http.MethodPost:
		h.handleBatchReview(w, r)
	case strings.HasSuffix(path, "/publish/seed-data/batch-delete") && r.Method == http.MethodPost:
		h.handleBatchDelete(w, r)
	case strings.Contains(path, "/publish/seed-data/") && r.Method == http.MethodPut:
		h.handleSaveSeedData(w, r)
	case strings.HasSuffix(path, "/publish/stats") && r.Method == http.MethodGet:
		h.handleStats(w, r)
	case strings.HasSuffix(path, "/publish/coverage-cache") && r.Method == http.MethodGet:
		h.handleCoverageCache(w, r)
	case strings.HasSuffix(path, "/publish/source-priority") && r.Method == http.MethodGet:
		h.handleGetSourcePriority(w, r)
	case strings.HasSuffix(path, "/publish/source-priority") && r.Method == http.MethodPut:
		h.handleSetSourcePriority(w, r)
	case strings.HasSuffix(path, "/publish/fetch-priority") && r.Method == http.MethodGet:
		h.handleGetFetchPriority(w, r)
	case strings.HasSuffix(path, "/publish/fetch-priority") && r.Method == http.MethodPut:
		h.handleSetFetchPriority(w, r)
	case strings.HasSuffix(path, "/publish/seeds/audit-infohash") && r.Method == http.MethodPost:
		h.handleAuditInfoHash(w, r)
	case strings.HasSuffix(path, "/publish/seeds/recompute-profiles") && r.Method == http.MethodPost:
		h.handleRecomputeProfiles(w, r)
	case strings.HasSuffix(path, "/publish/seeds/execute") && r.Method == http.MethodPost:
		h.handleExecutePublish(w, r)
	case strings.HasSuffix(path, "/publish/seeds/execute-batch") && r.Method == http.MethodPost:
		h.handleExecutePublishBatch(w, r)

	case strings.HasSuffix(path, "/publish/seeds/execute-site-batch") && r.Method == http.MethodPost:
		h.handleExecuteSiteBatch(w, r)
	case strings.HasSuffix(path, "/publish/seeds/site-batch-progress") && r.Method == http.MethodGet:
		h.handleSiteBatchProgress(w, r)

	case strings.HasSuffix(path, "/publish/seeds/batch-fetch") && r.Method == http.MethodPost:
		h.handleBatchFetch(w, r)
	case strings.HasSuffix(path, "/publish/seeds/batch-fetch-progress") && r.Method == http.MethodGet:
		h.handleBatchFetchProgress(w, r)
	case strings.HasSuffix(path, "/publish/seeds") && r.Method == http.MethodGet:
		h.handleListSeeds(w, r)
	case strings.HasSuffix(path, "/publish/seeds/unique-paths") && r.Method == http.MethodGet:
		h.handleSeedUniquePaths(w, r)
	case strings.HasSuffix(path, "/publish/seeds/observing/purge") && r.Method == http.MethodPost:
		h.handlePurgeObserving(w, r)
	case strings.Contains(path, "/publish/seeds/") && strings.HasSuffix(path, "/fetch") && r.Method == http.MethodPost:
		h.handleFetchSingleSeed(w, r)
	case strings.Contains(path, "/publish/seeds/") && r.Method == http.MethodGet:
		h.handleGetSeed(w, r)
	case strings.Contains(path, "/publish/seeds/") && r.Method == http.MethodPut:
		h.handlePutSeed(w, r)
	case strings.Contains(path, "/publish/seeds/") && r.Method == http.MethodDelete:
		h.handleDeleteSeed(w, r)
	default:
		Error(w, http.StatusNotFound, 40400, "接口不存在")
	}
}

func (h *PublishTorrentsHandler) handleListTorrents(w http.ResponseWriter, r *http.Request) {
	if h.clientMgr == nil {
		Error(w, http.StatusServiceUnavailable, 50001, "客户端管理器未初始化")
		return
	}

	clientIDStr := r.URL.Query().Get("client_id")
	if clientIDStr == "" {
		var clients []model.ClientConfig
		if err := h.db.Where("enabled = ?", true).Find(&clients).Error; err != nil {
			h.logger.Warn("query failed", zap.Error(err))
		}

		if len(clients) == 0 {
			Success(w, map[string]interface{}{"items": []interface{}{}, "total": 0})
			return
		}
		clientIDStr = fmt.Sprintf("%d", clients[0].ID)
	}

	clientID, err := strconv.ParseUint(clientIDStr, 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的 client_id")
		return
	}

	var cfg model.ClientConfig
	if err := h.db.First(&cfg, clientID).Error; err != nil {
		Error(w, http.StatusNotFound, 40400, "下载器不存在")
		return
	}

	client, err := h.clientMgr.Get(cfg.Name)
	if err != nil {
		Error(w, http.StatusInternalServerError, 50000, fmt.Sprintf("连接下载器失败: %v", err))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	torrents, err := client.GetSeedingTorrents(ctx)
	if err != nil {
		Error(w, http.StatusInternalServerError, 50000, fmt.Sprintf("获取种子列表失败: %v", err))
		return
	}

	// v0.0.265: tracker → 站点匹配（用于列表展示"做种站点"列）
	trackerMatcher := site.NewTrackerMatcher(h.db)

	var totalSites int64
	if err := h.db.Model(&model.Site{}).Where("enabled = ? AND is_target = ?", true, true).Count(&totalSites).Error; err != nil {
		h.logger.Warn("query failed", zap.Error(err))
	}

	infoHashes := make([]string, 0, len(torrents))
	for _, t := range torrents {
		infoHashes = append(infoHashes, t.Hash)
	}

	var coverageMap map[string][]model.SiteCoverageCache
	if h.coverage != nil && len(infoHashes) > 0 {
		coverageMap, err = h.coverage.GetBatchCachedCoverage(ctx, infoHashes)
		if err != nil {
			h.logger.Warn("list torrents: coverage cache read failed", zap.Error(err))
		}
	}
	if coverageMap == nil {
		coverageMap = map[string][]model.SiteCoverageCache{}
	}

	var queriedMap map[string]bool
	if h.coverage != nil && len(infoHashes) > 0 {
		queriedMap, err = h.coverage.GetBatchQueryState(ctx, infoHashes)
		if err != nil {
			h.logger.Warn("list torrents: query state read failed", zap.Error(err))
		}
	}
	if queriedMap == nil {
		queriedMap = map[string]bool{}
	}

	metaTypeMap := map[string]string{}
	metaReviewedMap := map[string]bool{}
	if len(infoHashes) > 0 {
		var metas []model.TorrentMetadata
		h.db.WithContext(ctx).
			Where("info_hash IN ?", infoHashes).
			Order("updated_at DESC").
			Find(&metas)
		for _, m := range metas {
			if _, exists := metaTypeMap[m.InfoHash]; !exists {
				if m.StandardType != "" {
					metaTypeMap[m.InfoHash] = m.StandardType
				}
				if m.Reviewed {
					metaReviewedMap[m.InfoHash] = true
				}
			}
		}

		missingHashes := make([]string, 0)
		for _, hash := range infoHashes {
			if _, ok := metaTypeMap[hash]; !ok {
				missingHashes = append(missingHashes, hash)
			}
		}
		if len(missingHashes) > 0 {
			var seens []model.RSSTorrentSeen
			h.db.WithContext(ctx).
				Where("info_hash IN ? AND source_category != ''", missingHashes).
				Find(&seens)
			for _, s := range seens {
				if _, exists := metaTypeMap[s.InfoHash]; !exists && s.SourceCategory != "" {
					normalized := normalizeCategorySimple(s.SourceCategory)
					if normalized != "" {
						metaTypeMap[s.InfoHash] = normalized
					}
				}
			}
		}
	}

	items := make([]map[string]interface{}, 0, len(torrents))
	for _, t := range torrents {
		cov := coverageMap[t.Hash]
		hasCount := 0
		for _, c := range cov {
			if c.Status == model.CoverageConfirmedHas || c.Status == model.CoverageProbablyHas {
				hasCount++
			}
		}
		stdType := metaTypeMap[t.Hash]
		if stdType == "" {
			stdType = inferTypeFromName(t.Name)
		}
		reviewed := metaReviewedMap[t.Hash]
		// v0.0.265: tracker 匹配做种站点（支持多 tracker → 多站点）
		var sourceSites []string
		if len(t.TrackerURLs) > 0 {
			sourceSites = trackerMatcher.MatchAll(t.TrackerURLs)
		} else if t.TrackerURL != "" {
			sourceSites = trackerMatcher.MatchAll([]string{t.TrackerURL})
		}
		items = append(items, map[string]interface{}{
			"info_hash":         t.Hash,
			"name":              t.Name,
			"size":              t.TotalSize,
			"save_path":         t.SavePath,
			"state":             t.State,
			"uploaded":          t.Uploaded,
			"progress":          t.Progress * 100,
			"ratio":             t.Ratio,
			"queried":           queriedMap[t.Hash],
			"standard_type":     stdType,
			"metadata_reviewed": reviewed,
			"source_site":       strings.Join(sourceSites, ", "),
			"source_sites":      sourceSites,
			"coverage": map[string]interface{}{
				"has_count":    hasCount,
				"total_sites":  totalSites,
				"target_count": int(totalSites) - hasCount,
				"sites":        cov,
			},
		})
	}

	// §56.40: 按 name+size 去重，同组优先保留官方小组源站的行
	items = h.dedupTorrentItems(r.Context(), items)

	// 如果有未查询的种子，触发后台批量查询
	querying := h.bgState.isQuerying(uint(clientID))
	if !querying && h.coverage != nil {
		unqueried := 0
		for _, t := range torrents {
			if !queriedMap[t.Hash] {
				unqueried++
			}
		}
		if unqueried > 0 {
			go h.startBackgroundQuery(uint(clientID), cfg, torrents)
			querying = true
		}
	}

	_, progressDone, progressTotal := h.bgState.getProgress()

	Success(w, map[string]interface{}{
		"items":       items,
		"total":       len(items),
		"total_sites": totalSites,
		"querying":    querying,
		"query_progress": map[string]int{
			"done":  progressDone,
			"total": progressTotal,
		},
	})
}

func (h *PublishTorrentsHandler) startBackgroundQuery(clientID uint, cfg model.ClientConfig, torrents []*model.TorrentInfo) {
	h.bgState.start(clientID, len(torrents))
	defer h.bgState.stop(clientID)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	client, err := h.clientMgr.Get(cfg.Name)
	if err != nil {
		h.logger.Error("bg query: client connect failed", zap.Error(err))
		return
	}

	// 读 query_state 确定哪些需要查询
	peerCtx, peerCancel := context.WithTimeout(ctx, 10*time.Second)
	defer peerCancel()

	allHashes := make([]string, 0, len(torrents))
	for _, t := range torrents {
		allHashes = append(allHashes, t.Hash)
	}

	queried, _ := h.coverage.GetBatchQueryState(peerCtx, allHashes)

	unqueriedHashes := make([]string, 0, len(torrents))
	for _, t := range torrents {
		if queried[t.Hash] {
			h.bgState.incDone()
			continue
		}
		unqueriedHashes = append(unqueriedHashes, t.Hash)
	}

	if len(unqueriedHashes) == 0 {
		return
	}

	// 统一调辅种引擎（IYUU 批量 + pieces_hash API + tracker 映射）
	now := time.Now()
	ttl := now.Add(24 * time.Hour)

	if h.reseedEngine != nil {
		hitsByHash := h.reseedEngine.QueryBatchCoverage(ctx, unqueriedHashes, client)
		for infoHash, hits := range hitsByHash {
			for _, hit := range hits {
				source := model.CoverageSourceIYUU
				if hit.Source == "pieces_hash" {
					source = model.CoverageSourcePiecesHash
				}
				h.coverage.UpsertCoverage(ctx, &model.SiteCoverageCache{
					InfoHash: infoHash, SiteName: hit.SiteName,
					Status: model.CoverageConfirmedHas, Source: source,
					Confidence: 1.0, TorrentID: hit.TorrentID,
					QueriedAt: now, ExpiresAt: ttl,
				})
			}
		}
	}

	// tracker 映射（GetAllTorrents，和 handleBatchQueryCoverage 一致）
	h.bgTrackerCoverage(ctx, unqueriedHashes, client, now, ttl)

	for _, hash := range unqueriedHashes {
		h.coverage.MarkQueried(ctx, hash, now, ttl)
	}

	h.bgState.setDone(len(torrents))
}

// bgTrackerCoverage 后台查询的 tracker 映射（复用 handleBatchQueryCoverage 的逻辑）
func (h *PublishTorrentsHandler) bgTrackerCoverage(ctx context.Context, hashes []string, client model.DownloaderClient, now, ttl time.Time) {
	if h.db == nil {
		return
	}
	tm := site.NewTrackerMatcher(h.db)
	hashSet := make(map[string]bool, len(hashes))
	for _, ih := range hashes {
		hashSet[ih] = true
	}
	allTorrents, err := client.GetAllTorrents(ctx)
	if err != nil {
		return
	}
	for _, t := range allTorrents {
		if !hashSet[t.Hash] {
			continue
		}
		var trackerURLs []string
		if len(t.TrackerURLs) > 0 {
			trackerURLs = t.TrackerURLs
		} else if t.TrackerURL != "" {
			trackerURLs = []string{t.TrackerURL}
		}
		if len(trackerURLs) == 0 {
			continue
		}
		trackerSites := tm.MatchAll(trackerURLs)
		for _, sn := range trackerSites {
			result := h.db.WithContext(ctx).Model(&model.SiteCoverageCache{}).
				Where("info_hash = ? AND site_name = ?", t.Hash, sn).
				Update("source", model.CoverageSourceTracker)
			if result.RowsAffected == 0 {
				h.coverage.UpsertCoverage(ctx, &model.SiteCoverageCache{
					InfoHash: t.Hash, SiteName: sn,
					Status: model.CoverageConfirmedHas, Source: model.CoverageSourceTracker,
					Confidence: 1.0, QueriedAt: now, ExpiresAt: ttl,
				})
			}
		}
	}
}

type coverageQueryRequest struct {
	ClientID  uint   `json:"clientId"`
	InfoHash  string `json:"infoHash"`
	Name      string `json:"name"`
	Size      int64  `json:"size"`
}

func (h *PublishTorrentsHandler) handleQueryCoverage(w http.ResponseWriter, r *http.Request) {
	if h.coverage == nil || h.clientMgr == nil {
		Error(w, http.StatusServiceUnavailable, 50001, "覆盖服务未初始化")
		return
	}

	var req coverageQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}
	if req.InfoHash == "" || req.ClientID == 0 {
		Error(w, http.StatusBadRequest, 40001, "info_hash 和 client_id 必填")
		return
	}

	var cfg model.ClientConfig
	if err := h.db.First(&cfg, req.ClientID).Error; err != nil {
		Error(w, http.StatusNotFound, 40400, "下载器不存在")
		return
	}

	client, err := h.clientMgr.Get(cfg.Name)
	if err != nil {
		Error(w, http.StatusInternalServerError, 50000, fmt.Sprintf("连接下载器失败: %v", err))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	now := time.Now()
	ttl := now.Add(24 * time.Hour)

	// ① 辅种引擎查询：IYUU + pieces_hash API → 🟡 可辅种（先写）
	if h.reseedEngine != nil {
		hits := h.reseedEngine.QuerySingleCoverage(ctx, req.InfoHash, client)
		for _, hit := range hits {
			source := model.CoverageSourceIYUU
			if hit.Source == "pieces_hash" {
				source = model.CoverageSourcePiecesHash
			}
			h.coverage.UpsertCoverage(ctx, &model.SiteCoverageCache{
				InfoHash:   req.InfoHash,
				SiteName:   hit.SiteName,
				Status:     model.CoverageConfirmedHas,
				Source:     source,
				Confidence: 1.0,
				TorrentID:  hit.TorrentID,
				QueriedAt:  now,
				ExpiresAt:  ttl,
			})
		}
	}

	// ② tracker 映射 → 🟢 做种中（后写，覆盖同名 🟡）
	trackers, err := client.GetTrackers(ctx, req.InfoHash)
	if err == nil && len(trackers) > 0 {
		tm := site.NewTrackerMatcher(h.db)
		trackerSites := tm.MatchAll(trackers)
		for _, sn := range trackerSites {
			h.coverage.UpsertCoverage(ctx, &model.SiteCoverageCache{
				InfoHash:   req.InfoHash,
				SiteName:   sn,
				Status:     model.CoverageConfirmedHas,
				Source:     model.CoverageSourceTracker,
				Confidence: 1.0,
				QueriedAt:  now,
				ExpiresAt:  ttl,
			})
		}
	}

	// 标记查询状态
	h.coverage.MarkQueried(ctx, req.InfoHash, now, ttl)

	// 返回最终结果
	cached, _ := h.coverage.GetCachedCoverage(ctx, req.InfoHash)

	var totalSites int64
	if err := h.db.Model(&model.Site{}).Where("enabled = ? AND is_target = ?", true, true).Count(&totalSites).Error; err != nil {
		h.logger.Warn("query failed", zap.Error(err))
	}

	hasCount := 0
	for _, r := range cached {
		if r.Status == model.CoverageConfirmedHas || r.Status == model.CoverageProbablyHas {
			hasCount++
		}
	}

	Success(w, map[string]interface{}{
		"info_hash":    req.InfoHash,
		"sites":        cached,
		"has_count":    hasCount,
		"total_sites":  totalSites,
		"target_count": int(totalSites) - hasCount,
	})
}

// handleBatchQueryCoverage §56.40: 批量覆盖查询（一次请求查多个种子）。
func (h *PublishTorrentsHandler) handleBatchQueryCoverage(w http.ResponseWriter, r *http.Request) {
	if h.coverage == nil || h.clientMgr == nil || h.reseedEngine == nil {
		Error(w, http.StatusServiceUnavailable, 50001, "覆盖服务或辅种引擎未初始化")
		return
	}
	var req struct {
		ClientID  uint     `json:"clientId"`
		InfoHashes []string `json:"infoHashes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}
	if req.ClientID == 0 || len(req.InfoHashes) == 0 {
		Error(w, http.StatusBadRequest, 40001, "client_id 和 info_hashes 必填")
		return
	}

	var cfg model.ClientConfig
	if err := h.db.First(&cfg, req.ClientID).Error; err != nil {
		Error(w, http.StatusNotFound, 40400, "下载器不存在")
		return
	}
	client, err := h.clientMgr.Get(cfg.Name)
	if err != nil {
		Error(w, http.StatusInternalServerError, 50000, fmt.Sprintf("连接下载器失败: %v", err))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 300*time.Second)
	defer cancel()

	now := time.Now()
	ttl := now.Add(24 * time.Hour)

	// ① 辅种引擎批量查询 → 🟡
	hitsByHash := h.reseedEngine.QueryBatchCoverage(ctx, req.InfoHashes, client)
	for infoHash, hits := range hitsByHash {
		for _, hit := range hits {
			source := model.CoverageSourceIYUU
			if hit.Source == "pieces_hash" {
				source = model.CoverageSourcePiecesHash
			}
			h.coverage.UpsertCoverage(ctx, &model.SiteCoverageCache{
				InfoHash: infoHash, SiteName: hit.SiteName,
				Status: model.CoverageConfirmedHas, Source: source,
				Confidence: 1.0, TorrentID: hit.TorrentID,
				QueriedAt: now, ExpiresAt: ttl,
			})
		}
	}

	// ② tracker 映射批量 → 🟢（后写覆盖）
	// 使用 GetAllTorrents 一次获取所有种子的 TrackerURLs（和列表端点一致，避免 GetTrackers 对某些下载器的兼容问题）
	tm := site.NewTrackerMatcher(h.db)
	hashSet := make(map[string]bool, len(req.InfoHashes))
	for _, ih := range req.InfoHashes {
		hashSet[ih] = true
	}
	allTorrents, err := client.GetAllTorrents(ctx)
	if err == nil {
		for _, t := range allTorrents {
			if !hashSet[t.Hash] {
				continue
			}
			var trackerURLs []string
			if len(t.TrackerURLs) > 0 {
				trackerURLs = t.TrackerURLs
			} else if t.TrackerURL != "" {
				trackerURLs = []string{t.TrackerURL}
			}
			if len(trackerURLs) == 0 {
				continue
			}
			trackerSites := tm.MatchAll(trackerURLs)
			for _, sn := range trackerSites {
				result := h.db.WithContext(ctx).Model(&model.SiteCoverageCache{}).
					Where("info_hash = ? AND site_name = ?", t.Hash, sn).
					Update("source", model.CoverageSourceTracker)
				if result.RowsAffected == 0 {
					h.coverage.UpsertCoverage(ctx, &model.SiteCoverageCache{
						InfoHash: t.Hash, SiteName: sn,
						Status: model.CoverageConfirmedHas, Source: model.CoverageSourceTracker,
						Confidence: 1.0, QueriedAt: now, ExpiresAt: ttl,
					})
				}
			}
		}
	} else {
		// fallback: 逐个 GetTrackers
		for _, infoHash := range req.InfoHashes {
			trackers, err := client.GetTrackers(ctx, infoHash)
			if err != nil || len(trackers) == 0 {
				continue
			}
			trackerSites := tm.MatchAll(trackers)
			for _, sn := range trackerSites {
				h.coverage.UpsertCoverage(ctx, &model.SiteCoverageCache{
					InfoHash: infoHash, SiteName: sn,
					Status: model.CoverageConfirmedHas, Source: model.CoverageSourceTracker,
					Confidence: 1.0, QueriedAt: now, ExpiresAt: ttl,
				})
			}
		}
	}
	for _, infoHash := range req.InfoHashes {
		h.coverage.MarkQueried(ctx, infoHash, now, ttl)
	}

	Success(w, map[string]interface{}{
		"queried": len(req.InfoHashes),
	})
}

func (h *PublishTorrentsHandler) ScheduledRefresh(ctx context.Context) error {
	h.logger.Info("scheduled coverage refresh started")
	var clients []model.ClientConfig
	if err := h.db.WithContext(ctx).Where("enabled = ?", true).Find(&clients).Error; err != nil {
		h.logger.Warn("query failed", zap.Error(err))
	}

	now := time.Now()
	ttl := now.Add(24 * time.Hour)

	for _, cfg := range clients {
		client, err := h.clientMgr.Get(cfg.Name)
		if err != nil {
			h.logger.Warn("coverage refresh: client failed", zap.String("name", cfg.Name), zap.Error(err))
			continue
		}

		tCtx, tCancel := context.WithTimeout(ctx, 2*time.Minute)
		torrents, err := client.GetSeedingTorrents(tCtx)
		tCancel()
		if err != nil {
			h.logger.Warn("coverage refresh: get torrents failed", zap.String("name", cfg.Name), zap.Error(err))
			continue
		}

		allHashes := make([]string, 0, len(torrents))
		for _, t := range torrents {
			allHashes = append(allHashes, t.Hash)
		}

		qCtx, qCancel := context.WithTimeout(ctx, 10*time.Second)
		queried, _ := h.coverage.GetBatchQueryState(qCtx, allHashes)
		qCancel()

		var unqueriedHashes []string
		for _, t := range torrents {
			if !queried[t.Hash] {
				unqueriedHashes = append(unqueriedHashes, t.Hash)
			}
		}

		if len(unqueriedHashes) == 0 {
			continue
		}

		h.logger.Info("coverage refresh: querying",
			zap.String("client", cfg.Name),
			zap.Int("torrents", len(unqueriedHashes)))

		// 统一调辅种引擎
		batchCtx, batchCancel := context.WithTimeout(ctx, 5*time.Minute)
		if h.reseedEngine != nil {
			hitsByHash := h.reseedEngine.QueryBatchCoverage(batchCtx, unqueriedHashes, client)
			for infoHash, hits := range hitsByHash {
				for _, hit := range hits {
					source := model.CoverageSourceIYUU
					if hit.Source == "pieces_hash" {
						source = model.CoverageSourcePiecesHash
					}
					h.coverage.UpsertCoverage(batchCtx, &model.SiteCoverageCache{
						InfoHash: infoHash, SiteName: hit.SiteName,
						Status: model.CoverageConfirmedHas, Source: source,
						Confidence: 1.0, TorrentID: hit.TorrentID,
						QueriedAt: now, ExpiresAt: ttl,
					})
				}
			}
		}
		h.bgTrackerCoverage(batchCtx, unqueriedHashes, client, now, ttl)
		for _, hash := range unqueriedHashes {
			h.coverage.MarkQueried(batchCtx, hash, now, ttl)
		}
		batchCancel()
	}

	h.logger.Info("scheduled coverage refresh done")
	return nil
}

func (h *PublishTorrentsHandler) handleQueryStatus(w http.ResponseWriter, r *http.Request) {
	clientIDStr := r.URL.Query().Get("client_id")
	clientID, _ := strconv.ParseUint(clientIDStr, 10, 64)

	querying := h.bgState.isQuerying(uint(clientID))
	_, done, total := h.bgState.getProgress()
	Success(w, map[string]interface{}{
		"querying": querying,
		"done":     done,
		"total":    total,
	})
}

func (s *backgroundQueryState) start(clientID uint, total int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active[clientID] = true
	s.total = total
	s.done = 0
}

func (s *backgroundQueryState) stop(clientID uint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.active, clientID)
}

func (s *backgroundQueryState) isQuerying(clientID uint) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active[clientID]
}

func (s *backgroundQueryState) incDone() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.done++
}

func (s *backgroundQueryState) setDone(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.done = n
}

func (s *backgroundQueryState) getProgress() (querying bool, done, total int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.active) > 0, s.done, s.total
}

type detectSourceRequest struct {
	InfoHash string `json:"infoHash"`
	Name     string `json:"name"`
}

func (h *PublishTorrentsHandler) handleDetectSource(w http.ResponseWriter, r *http.Request) {
	if h.sourceDetector == nil {
		Error(w, http.StatusServiceUnavailable, 50001, "源站检测器未初始化")
		return
	}

	var req detectSourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}
	if req.InfoHash == "" || req.Name == "" {
		Error(w, http.StatusBadRequest, 40001, "info_hash 和 name 必填")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// 读覆盖数据
	coverageSites, _ := h.coverage.GetCachedCoverage(ctx, req.InfoHash)

	// 检测源头站
	detected := h.sourceDetector.Detect(ctx, req.Name, req.InfoHash, coverageSites)

	// 构建候选列表（用于前端降级选择）
	type candidate struct {
		SiteName  string `json:"siteName"`
		TorrentID string `json:"torrentId"`
		HasCookie bool   `json:"hasCookie"`
	}
	var candidates []candidate
	siteMap := make(map[string]string)
	for _, c := range coverageSites {
		if c.Status == model.CoverageConfirmedHas || c.Status == model.CoverageProbablyHas {
			siteMap[c.SiteName] = c.TorrentID
		}
	}
	if len(siteMap) > 0 {
		var sites []model.Site
		siteNames := make([]string, 0, len(siteMap))
		for name := range siteMap {
			siteNames = append(siteNames, name)
		}
		if err := h.db.Where("name IN ?", siteNames).Find(&sites).Error; err != nil {
			h.logger.Warn("query failed", zap.Error(err))
		}

		for _, site := range sites {
			candidates = append(candidates, candidate{
				SiteName:  site.Name,
				TorrentID: siteMap[site.Name],
				HasCookie: site.Cookie != "",
			})
		}
	}

	Success(w, map[string]interface{}{
		"source_site":    detected.SourceSite,
		"source_site_id": detected.SourceSiteID,
		"group_name":     detected.GroupName,
		"torrent_id":     detected.TorrentID,
		"auto_detected":  detected.AutoDetected,
		"candidates":     candidates,
	})
}

func (h *PublishTorrentsHandler) handleListGroupMappings(w http.ResponseWriter, r *http.Request) {
	var mappings []model.ReleaseGroupMapping
	if err := h.db.WithContext(r.Context()).Order("group_name ASC").Find(&mappings).Error; err != nil {
		h.logger.Warn("query failed", zap.Error(err))
	}

	var sites []model.Site
	if err := h.db.WithContext(r.Context()).Where("enabled = ?", true).Find(&sites).Error; err != nil {
		h.logger.Warn("query failed", zap.Error(err))
	}

	siteMap := make(map[string]string)
	for _, s := range sites {
		siteMap[strings.ToLower(s.Domain)] = s.Name
		if s.AlternativeDomains != "" {
			var altDomains []string
			if err := json.Unmarshal([]byte(s.AlternativeDomains), &altDomains); err == nil {
				for _, alt := range altDomains {
					siteMap[strings.ToLower(alt)] = s.Name
				}
			}
		}
	}

	type mappingWithMatch struct {
		model.ReleaseGroupMapping
		MatchedSite string `json:"matchedSite"`
	}
	result := make([]mappingWithMatch, 0, len(mappings))
	for _, m := range mappings {
		mw := mappingWithMatch{ReleaseGroupMapping: m}
		if m.SiteName != "" {
			mw.MatchedSite = m.SiteName
		} else if m.Domain != "" {
			barePattern := strings.TrimPrefix(strings.ToLower(m.Domain), "www.")
			for domain, name := range siteMap {
				bareDomain := strings.TrimPrefix(domain, "www.")
				if strings.Contains(bareDomain, barePattern) {
					mw.MatchedSite = name
					break
				}
			}
		}
		result = append(result, mw)
	}

	Success(w, map[string]interface{}{"items": result, "total": len(result)})
}

func (h *PublishTorrentsHandler) handleListGroupedSiteNames(w http.ResponseWriter, r *http.Request) {
	var siteNames []string
	h.db.WithContext(r.Context()).
		Model(&model.ReleaseGroupMapping{}).
		Where("site_name != ''").
		Distinct("site_name").
		Pluck("site_name", &siteNames)
	Success(w, map[string]interface{}{"sites": siteNames})
}

func (h *PublishTorrentsHandler) handleGetDeclarationFilters(w http.ResponseWriter, r *http.Request) {
	if h.declFilter == nil {
		Success(w, map[string]interface{}{"patterns": []string{}, "is_default": true})
		return
	}
	patterns := h.declFilter.GetPatterns(r.Context())
	// 检查是否是默认值（DB 中没有存储）
	val, err := h.declFilter.GetRawPatterns(r.Context())
	isDefault := err != nil || val == ""
	Success(w, map[string]interface{}{
		"patterns":   patterns,
		"is_default": isDefault,
	})
}

func (h *PublishTorrentsHandler) handleSetDeclarationFilters(w http.ResponseWriter, r *http.Request) {
	if h.declFilter == nil {
		Error(w, http.StatusServiceUnavailable, 50001, "声明过滤器未初始化")
		return
	}
	var req struct {
		Patterns []string `json:"patterns"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}
	if err := h.declFilter.SetPatterns(r.Context(), req.Patterns); err != nil {
		Error(w, http.StatusInternalServerError, 50000, fmt.Sprintf("保存失败: %v", err))
		return
	}
	Success(w, map[string]interface{}{
		"patterns": req.Patterns,
		"message":  "已保存",
	})
}

type previewTitleRequest struct {
	TargetSite      string            `json:"targetSite"`
	TitleComponents map[string]string `json:"titleComponents"`
}

func (h *PublishTorrentsHandler) handlePreviewTitle(w http.ResponseWriter, r *http.Request) {
	var req previewTitleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}
	if req.TargetSite == "" {
		Error(w, http.StatusBadRequest, 40001, "target_site 必填")
		return
	}

	// 查目标站的 title_format
	var site model.Site
	if err := h.db.WithContext(r.Context()).Where("name = ?", req.TargetSite).First(&site).Error; err != nil {
		Error(w, http.StatusNotFound, 40400, "站点不存在")
		return
	}

	// 构建 TitleComponents
	c := titleparser.TitleComponents{
		MainTitle:      req.TitleComponents["main_title"],
		SeasonEpisode:  req.TitleComponents["season_episode"],
		Year:           req.TitleComponents["year"],
		Resolution:     req.TitleComponents["resolution"],
		Medium:         req.TitleComponents["medium"],
		VideoCodec:     req.TitleComponents["video_codec"],
		AudioCodec:     req.TitleComponents["audio_codec"],
		HDRFormat:      req.TitleComponents["hdr_format"],
		SourcePlatform: req.TitleComponents["source_platform"],
		BitDepth:       req.TitleComponents["bit_depth"],
		ReleaseVersion: req.TitleComponents["release_version"],
		ReleaseGroup:   req.TitleComponents["release_group"],
		ChinesePrefix:  req.TitleComponents["chinese_prefix"],
	}

	// 解析 title_format
	var tf titleparser.TitleFormat
	if site.TitleFormat != "" {
		if err := json.Unmarshal([]byte(site.TitleFormat), &tf); err != nil {
			tf = titleparser.DefaultTitleFormat()
		}
	} else {
		tf = titleparser.DefaultTitleFormat()
	}

	result := titleparser.ReassembleFromTechProfile(titleparser.TechProfileFromTitle(c), tf)
	if result == "" {
		result = c.MainTitle
	}
	Success(w, map[string]interface{}{
		"title":       result,
		"target_site": req.TargetSite,
	})
}

func (h *PublishTorrentsHandler) handlePreviewTitleBatch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TargetSites     []string          `json:"targetSites"`
		TitleComponents map[string]string `json:"titleComponents"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}

	c := titleparser.TitleComponents{
		MainTitle:      req.TitleComponents["main_title"],
		SeasonEpisode:  req.TitleComponents["season_episode"],
		Year:           req.TitleComponents["year"],
		Resolution:     req.TitleComponents["resolution"],
		Medium:         req.TitleComponents["medium"],
		VideoCodec:     req.TitleComponents["video_codec"],
		AudioCodec:     req.TitleComponents["audio_codec"],
		HDRFormat:      req.TitleComponents["hdr_format"],
		SourcePlatform: req.TitleComponents["source_platform"],
		BitDepth:       req.TitleComponents["bit_depth"],
		ReleaseVersion: req.TitleComponents["release_version"],
		ReleaseGroup:   req.TitleComponents["release_group"],
		ChinesePrefix:  req.TitleComponents["chinese_prefix"],
	}

	results := make(map[string]string, len(req.TargetSites))
	for _, siteName := range req.TargetSites {
		var site model.Site
		if err := h.db.WithContext(r.Context()).Where("name = ?", siteName).First(&site).Error; err != nil {
			results[siteName] = ""
			continue
		}
		var tf titleparser.TitleFormat
		if site.TitleFormat != "" {
			if err := json.Unmarshal([]byte(site.TitleFormat), &tf); err != nil {
			 tf = titleparser.DefaultTitleFormat()
			}
		} else {
			tf = titleparser.DefaultTitleFormat()
		}
		title := titleparser.ReassembleFromTechProfile(titleparser.TechProfileFromTitle(c), tf)
		if title == "" {
			title = c.MainTitle
		}
		results[siteName] = title
	}

	Success(w, map[string]interface{}{"results": results})
}

func (h *PublishTorrentsHandler) handleCreateGroupMapping(w http.ResponseWriter, r *http.Request) {
	var req struct {
		GroupName  string `json:"groupName"`
		Domain     string `json:"domain"`
		SiteName   string `json:"siteName"`
		IsOfficial bool   `json:"isOfficial"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}
	if req.GroupName == "" {
		Error(w, http.StatusBadRequest, 40001, "group_name 必填")
		return
	}
	mapping := model.ReleaseGroupMapping{
		GroupName:  req.GroupName,
		Domain:     req.Domain,
		SiteName:   req.SiteName,
		IsOfficial: req.IsOfficial,
	}
	if err := h.db.Create(&mapping).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, fmt.Sprintf("创建失败: %v", err))
		return
	}
	if h.sourceDetector != nil {
		h.sourceDetector.RefreshCache(r.Context())
	}
	Success(w, mapping)
}

func (h *PublishTorrentsHandler) handleUpdateGroupMapping(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimRight(r.URL.Path, "/")
	parts := strings.Split(path, "/")
	idStr := parts[len(parts)-1]
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的 ID")
		return
	}
	var req struct {
		GroupName  string `json:"groupName"`
		Domain     string `json:"domain"`
		SiteName   string `json:"siteName"`
		IsOfficial bool   `json:"isOfficial"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}
	if err := h.db.Model(&model.ReleaseGroupMapping{}).Where("id = ?", id).Updates(map[string]interface{}{
		"group_name":  req.GroupName,
		"domain":      req.Domain,
		"site_name":   req.SiteName,
		"is_official": req.IsOfficial,
	}).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, fmt.Sprintf("更新失败: %v", err))
		return
	}
	if h.sourceDetector != nil {
		h.sourceDetector.RefreshCache(r.Context())
	}
	Success(w, map[string]interface{}{"message": "已更新"})
}

func (h *PublishTorrentsHandler) handleDeleteGroupMapping(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimRight(r.URL.Path, "/")
	parts := strings.Split(path, "/")
	idStr := parts[len(parts)-1]
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的 ID")
		return
	}

	var mapping model.ReleaseGroupMapping
	h.db.WithContext(r.Context()).First(&mapping, id)

	if mapping.IsBuiltin {
		Error(w, http.StatusForbidden, 40300, "内置官组不可删除")
		return
	}

	if err := h.db.Delete(&model.ReleaseGroupMapping{}, id).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, fmt.Sprintf("删除失败: %v", err))
		return
	}

	if h.sourceDetector != nil {
		h.sourceDetector.RefreshCache(r.Context())
	}

	// 删除后如果该站已无任何映射，自动关闭 is_source
	if mapping.SiteName != "" {
		var remaining int64
		h.db.WithContext(r.Context()).Model(&model.ReleaseGroupMapping{}).
			Where("site_name = ?", mapping.SiteName).Count(&remaining)
		if remaining == 0 {
			result := h.db.Model(&model.Site{}).
				Where("name = ? AND is_source = ?", mapping.SiteName, true).
				Update("is_source", false)
			if result.RowsAffected > 0 {
				h.logger.Info("site is_source auto-disabled: no group mappings left",
					zap.String("site", mapping.SiteName))
			}
		}
	}

	Success(w, map[string]interface{}{"message": "已删除"})
}

type batchPublishRequest struct {
	ClientID   uint   `json:"clientId"`
	SourceSite string `json:"sourceSite"`
	TargetSite string `json:"targetSite"`
	Items      []struct {
		InfoHash string `json:"infoHash"`
		Name     string `json:"name"`
		Size     int64  `json:"size"`
		SavePath string `json:"savePath"`
	} `json:"items"`
}

func (h *PublishTorrentsHandler) handleBatchPublish(w http.ResponseWriter, r *http.Request) {
	var req batchPublishRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}
	if len(req.Items) == 0 {
		Error(w, http.StatusBadRequest, 40001, "items 不能为空")
		return
	}
	if req.TargetSite == "" {
		Error(w, http.StatusBadRequest, 40001, "target_site 必填")
		return
	}
	if req.SourceSite == "" {
		Error(w, http.StatusBadRequest, 40001, "source_site 必填")
		return
	}

	// 查目标站是否启用
	var targetSite model.Site
	if err := h.db.Where("name = ? AND enabled = ?", req.TargetSite, true).First(&targetSite).Error; err != nil {
		Error(w, http.StatusBadRequest, 40001, "目标站点不存在或未启用")
		return
	}

	// 查排除规则
	var exclusions []model.PublishExclusion
	if err := h.db.Find(&exclusions).Error; err != nil {
		h.logger.Warn("query failed", zap.Error(err))
	}

	blockedTargets := []string{}
	for _, exc := range exclusions {
		if exc.SourceSite == req.SourceSite {
			blockedTargets = append(blockedTargets, exc.TargetSite)
		}
	}
	for _, bt := range blockedTargets {
		if bt == req.TargetSite {
			Error(w, http.StatusBadRequest, 40001, "目标站点被互斥规则排除")
			return
		}
	}

	targetsJSON, _ := json.Marshal([]string{req.TargetSite})
	createdIDs := make([]uint, 0, len(req.Items))
	failed := 0

	// §59.147: 数字下载器 id → 客户端名转换（pusher 按名 Get, 原数字串必失败）
	var clientName string
	if req.ClientID > 0 {
		var cl model.ClientConfig
		if err := h.db.Select("name").Where("id = ?", req.ClientID).First(&cl).Error; err == nil {
			clientName = cl.Name
		}
	}

	for _, item := range req.Items {
		candidate := &model.PublishCandidate{
			SourceSite: req.SourceSite,
			InfoHash:   item.InfoHash,
			TorrentName: item.Name,
			ClientID:   clientName,
			// §59.147: 源资源路径落库——链 A 加种 SavePath 依赖（原丢弃致加种落默认路径不做种）
			LocalSavePath:     item.SavePath,
			TargetSites:       string(targetsJSON),
			PublishStatus:     model.CandidatePending,
			DownloadCompleted: true,
			Role:              "manual",
		}
		if err := h.db.Create(candidate).Error; err != nil {
			h.logger.Warn("batch publish: create candidate failed",
				zap.String("hash", item.InfoHash[:8]),
				zap.Error(err))
			failed++
			continue
		}
		createdIDs = append(createdIDs, candidate.ID)

		// 回写覆盖缓存（该种子已在目标站发布）
		if h.coverage != nil {
			h.coverage.UpdateFromPublishResult(r.Context(), item.InfoHash, req.TargetSite)
		}
	}

	h.logger.Info("batch publish completed",
		zap.Int("created", len(createdIDs)),
		zap.Int("failed", failed),
		zap.String("target", req.TargetSite))

	Success(w, map[string]interface{}{
		"created":       len(createdIDs),
		"failed":        failed,
		"candidate_ids": createdIDs,
		"target_site":   req.TargetSite,
	})
}

func normalizeCategorySimple(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	lower := strings.ToLower(raw)
	switch {
	case strings.Contains(lower, "movie") || strings.Contains(raw, "电影"):
		return "category.movie"
	case strings.Contains(lower, "tv") || strings.Contains(lower, "series") || strings.Contains(raw, "电视剧") || strings.Contains(raw, "剧集"):
		return "category.tv_series"
	case strings.Contains(lower, "anim") || strings.Contains(raw, "动漫") || strings.Contains(raw, "动画"):
		return "category.animation"
	case strings.Contains(lower, "doc") || strings.Contains(raw, "纪录"):
		return "category.documentaries"
	case strings.Contains(lower, "variety") || strings.Contains(lower, "show") || strings.Contains(raw, "综艺"):
		return "category.tv_shows"
	case strings.Contains(lower, "music") || strings.Contains(raw, "音乐"):
		return "category.music"
	case strings.Contains(lower, "sport") || strings.Contains(raw, "体育"):
		return "category.sports"
	default:
		return ""
	}
}

func inferTypeFromName(name string) string {
	lower := strings.ToLower(name)

	// 动画/动漫（优先检测，避免被 S01E 误判为电视剧）
	animationPatterns := []string{
		"anime", "animation", "oad", "ova", "oav",
		"borgen.", "wiggenwald.", "naruto", "one.piece",
		"dragon.ball", "demon.slayer", "attack.on.titan",
		"my.hero", "jujutsu", "chainsaw", "dandadan",
		"solo.leveling", "re.zero", "spy.x.family",
	}
	for _, p := range animationPatterns {
		if strings.Contains(lower, p) {
			return "category.animation"
		}
	}

	// 综艺/真人秀
	varietyPatterns := []string{
		"running.man", "variety", "show.s01", "tv.shows",
		"h!6", "你好，星期六", "快乐大本营", "奔跑吧",
		"极限挑战", "王牌对王牌", "乘风破浪",
		"向往的生活", "密室大逃脱", "青春环游记",
	}
	for _, p := range varietyPatterns {
		if strings.Contains(lower, p) {
			return "category.tv_shows"
		}
	}

	// 纪录片
	docPatterns := []string{
		"documentary", "documentaries", "nat.geo", "national.geographic",
		"bbc.earth", "blue.planet", "our.planet", "nature",
		" historia", "探索", "档案",
	}
	for _, p := range docPatterns {
		if strings.Contains(lower, p) {
			return "category.documentaries"
		}
	}

	// 音乐/演唱会
	musicPatterns := []string{
		"concert", "live.", "mv.", "music.video",
		"tour.", "festival.", "symphony", "opera.",
	}
	for _, p := range musicPatterns {
		if strings.Contains(lower, p) {
			return "category.music"
		}
	}

	// 体育
	sportPatterns := []string{
		"nba.", "nfl.", "ufc.", "f1.", "motogp",
		"world.cup", "euro.", "premier.league",
		"champions.league", "olympic",
	}
	for _, p := range sportPatterns {
		if strings.Contains(lower, p) {
			return "category.sports"
		}
	}

	// 电视剧（季集模式）
	tvPatterns := []string{
		"s01e", "s02e", "s03e", "s04e", "s05e", "s06e",
		"s07e", "s08e", "s09e", "s1e", "s2e",
		".s01.", ".s02.", ".s03.", ".s04.", ".s05.",
		".s06.", ".s07.", ".s08.", ".s09.",
		"complete", "season.1", "season.2", "season.3",
		"season.4", "season.5", "ep01", "ep02", "ep03",
		"ep04", "ep05", "ep06", "ep07", "ep08",
		"e01.", "e02.", "e03.", "e04.", "e05.",
		"e06.", "e07.", "e08.", "e09.", "e10.",
		"episode.1", "episode.2", "episode.3",
	}
	for _, p := range tvPatterns {
		if strings.Contains(lower, p) {
			return "category.tv_series"
		}
	}

	// 中文电视剧标题特征
	cnTVPrefixes := []string{
		"第.季", "第.部", "连续剧", "电视剧",
	}
	for _, p := range cnTVPrefixes {
		matched, _ := regexp.MatchString(p, name)
		if matched {
			return "category.tv_series"
		}
	}

	// 默认电影
	return "category.movie"
}

// handleCachedSites §56.37: 查询 info_hash 在 torrent_metadata 中已缓存的源站列表。
// 用于 CrossSeedPanel 的源站选择器（遗漏 E：源站切换）。
func (h *PublishTorrentsHandler) handleCachedSites(w http.ResponseWriter, r *http.Request) {
	infoHash := r.URL.Query().Get("info_hash")
	if infoHash == "" {
		Error(w, http.StatusBadRequest, 40001, "info_hash 必填")
		return
	}

	var metas []model.TorrentMetadata
	h.db.WithContext(r.Context()).
		Where("info_hash = ?", infoHash).
		Select("id", "site_name", "torrent_id", "reviewed", "fetched_at", "subtitle", "title").
		Find(&metas)

	type cachedSite struct {
		ID        uint   `json:"id"`
		SiteName  string `json:"siteName"`
		TorrentID string `json:"torrentId"`
		Reviewed  bool   `json:"reviewed"`
		FetchedAt string `json:"fetchedAt"`
		Title     string `json:"title"`
		Subtitle  string `json:"subtitle"`
	}
	sites := make([]cachedSite, 0, len(metas))
	for _, m := range metas {
		sites = append(sites, cachedSite{
			ID:        m.ID,
			SiteName:  m.SiteName,
			TorrentID: m.TorrentID,
			Reviewed:  m.Reviewed,
			FetchedAt: m.FetchedAt.Format("2006-01-02 15:04"),
			Title:     m.Title,
			Subtitle:  m.Subtitle,
		})
	}

	Success(w, map[string]interface{}{
		"info_hash": infoHash,
		"sites":     sites,
	})
}

// handleListSeedData §56.37: /publish/seed-data 列表（torrent_metadata 已 review 的记录）。
// 用于 /publish/data 页面（一站多种）。P1 优先级，P0 先实现基本列表。
func (h *PublishTorrentsHandler) handleListSeedData(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	if page == 0 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(q.Get("page_size"))
	if pageSize == 0 {
		pageSize = 20
	}
	search := q.Get("search")
	sourceSite := q.Get("source_site")
	reviewStatus := q.Get("review_status") // all（默认）/ reviewed / unreviewed

	fetchSource := q.Get("fetch_source")
	if fetchSource == "" {
		fetchSource = "batch_fetch"
	}

	query := h.db.WithContext(r.Context()).Model(&model.TorrentMetadata{}).
		Where("torrent_id != '' AND torrent_id != '0'")
	if fetchSource != "all" {
		query = query.Where("fetch_source = ?", fetchSource)
	}

	if search != "" {
		query = query.Where("title LIKE ? OR subtitle LIKE ? OR info_hash LIKE ?", "%"+search+"%", "%"+search+"%", "%"+search+"%")
	}
	if sourceSite != "" {
		query = query.Where("site_name = ?", sourceSite)
	}
	switch reviewStatus {
	case "reviewed":
		query = query.Where("reviewed = ?", true)
	case "unreviewed":
		query = query.Where("reviewed = ?", false)
	}

	var total int64
	query.Count(&total)

	var metas []model.TorrentMetadata
	query.Order("updated_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&metas)

	Success(w, map[string]interface{}{
		"items":     metas,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// handleSaveSeedData §56.37: PUT /publish/seed-data/{id} 保存用户编辑。
// 写平铺字段 + reviewed=true（遗漏 D：CrossSeedPanel 保存时机）。
func (h *PublishTorrentsHandler) handleSaveSeedData(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimRight(r.URL.Path, "/")
	parts := strings.Split(path, "/")
	idStr := parts[len(parts)-1]
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的 ID")
		return
	}

	var req struct {
		Title       string `json:"title"`
		Subtitle    string `json:"subtitle"`
		Description string `json:"description"`
		Screenshots string `json:"screenshots"`
		Poster      string `json:"poster"`
		MediaInfo   string `json:"mediainfo"`
		Tags        string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}

	// §59.94: 空值不覆盖（§59.89 同款——旧编辑链无条件覆盖会清空未传字段）
	updates := map[string]interface{}{"reviewed": true}
	if req.Title != "" {
		updates["title"] = req.Title
	}
	if req.Subtitle != "" {
		updates["subtitle"] = req.Subtitle
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Screenshots != "" {
		updates["screenshots"] = model.NormalizeScreenshotColumn(req.Screenshots) // §59.59 附二: 透传写点归一
	}
	if req.Poster != "" {
		updates["poster"] = req.Poster
	}
	if req.MediaInfo != "" {
		updates["media_info"] = req.MediaInfo // §59.56: 列名笔误同族第四处
	}
	if req.Tags != "" {
		updates["tags"] = req.Tags
	}

	result := h.db.WithContext(r.Context()).
		Model(&model.TorrentMetadata{}).
		Where("id = ?", id).
		Updates(updates)
	if result.Error != nil {
		Error(w, http.StatusInternalServerError, 50000, fmt.Sprintf("保存失败: %v", result.Error))
		return
	}
	if result.RowsAffected == 0 {
		Error(w, http.StatusNotFound, 40400, "记录不存在")
		return
	}
	// §59.94: 审核簇同步（公共方法）
	h.syncClusterReviewedByIDs(context.Background(), []uint{uint(id)}, true)

	Success(w, map[string]interface{}{"success": true, "id": id})
}

// handleBatchReview §56.40: 批量审核（标记 reviewed）。
func (h *PublishTorrentsHandler) handleBatchReview(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs      []uint `json:"ids"`
		Reviewed bool   `json:"reviewed"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "invalid body")
		return
	}
	if len(req.IDs) == 0 {
		Error(w, http.StatusBadRequest, 40001, "ids required")
		return
	}
	result := h.db.WithContext(r.Context()).
		Model(&model.TorrentMetadata{}).
		Where("id IN ?", req.IDs).
		Update("reviewed", req.Reviewed)
	if result.Error != nil {
		h.logger.Warn("batch review failed", zap.Error(result.Error))
		Error(w, http.StatusInternalServerError, 50000, "db error")
		return
	}
	// §59.94: 审核簇同步（公共方法——批量 ID 去重簇键后逐簇传播）
	h.syncClusterReviewedByIDs(context.Background(), req.IDs, req.Reviewed)
	Success(w, map[string]interface{}{"updated": result.RowsAffected})
}

// handleBatchDelete §56.40: 批量删除元数据（硬删除，仅删快照不删种子）。
func (h *PublishTorrentsHandler) handleBatchDelete(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []uint `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "invalid body")
		return
	}
	if len(req.IDs) == 0 {
		Error(w, http.StatusBadRequest, 40001, "ids required")
		return
	}
	result := h.db.WithContext(r.Context()).
		Where("id IN ?", req.IDs).
		Delete(&model.TorrentMetadata{})
	if result.Error != nil {
		h.logger.Warn("batch delete failed", zap.Error(result.Error))
		Error(w, http.StatusInternalServerError, 50000, "db error")
		return
	}
	h.logger.Info("batch delete metadata", zap.Int("count", int(result.RowsAffected)))
	Success(w, map[string]interface{}{"deleted": result.RowsAffected})
}
func (h *PublishTorrentsHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	var stats struct {
		TodayPublish   int64 `json:"todayPublish"`
		TodaySuccess   int64 `json:"todaySuccess"`
		TodayFailed    int64 `json:"todayFailed"`
		PendingCount   int64 `json:"pendingCount"`
		ReviewedCount  int64 `json:"reviewedCount"`
		TotalMetadata  int64 `json:"totalMetadata"`
		YesterdayPublish int64 `json:"yesterdayPublish"`
		YesterdaySuccess int64 `json:"yesterdaySuccess"`
		UnreviewedCount  int64 `json:"unreviewedCount"`
	}

	yesterdayStart := todayStart.AddDate(0, 0, -1)

	// 今日发布数
	h.db.WithContext(ctx).Model(&model.PublishResultRecord{}).
		Where("created_at >= ?", todayStart).Count(&stats.TodayPublish)
	h.db.WithContext(ctx).Model(&model.PublishResultRecord{}).
		Where("created_at >= ? AND status = ?", todayStart, "completed").Count(&stats.TodaySuccess)
	h.db.WithContext(ctx).Model(&model.PublishResultRecord{}).
		Where("created_at >= ? AND status = ?", todayStart, "failed").Count(&stats.TodayFailed)

	// 昨日数据（环比）
	h.db.WithContext(ctx).Model(&model.PublishResultRecord{}).
		Where("created_at >= ? AND created_at < ?", yesterdayStart, todayStart).Count(&stats.YesterdayPublish)
	h.db.WithContext(ctx).Model(&model.PublishResultRecord{}).
		Where("created_at >= ? AND created_at < ? AND status = ?", yesterdayStart, todayStart, "completed").Count(&stats.YesterdaySuccess)

	// 未审核元数据
	h.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
		Where("reviewed = ? AND torrent_id != '' AND torrent_id != '0'", false).
		Count(&stats.UnreviewedCount)

	// 队列深度
	h.db.WithContext(ctx).Model(&model.PublishCandidate{}).
		Where("publish_status IN ?", []string{"active", "pending"}).Count(&stats.PendingCount)

	// 已审核数据
	h.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
		Where("reviewed = ? AND torrent_id != '' AND torrent_id != '0'", true).
		Count(&stats.ReviewedCount)

	// 总元数据数
	h.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
		Where("torrent_id != '' AND torrent_id != '0'").
		Count(&stats.TotalMetadata)

	// 最近 10 条发布记录
	var recent []model.PublishResultRecord
	h.db.WithContext(ctx).Order("created_at DESC").Limit(10).Find(&recent)

	// 趋势图（可切换 7/30 天）
	days := 7
	if d := r.URL.Query().Get("days"); d == "30" {
		days = 30
	}
	weekAgo := todayStart.AddDate(0, 0, -(days - 1))
	type dayStat struct {
		Day     string `json:"day"`
		Success int64  `json:"success"`
		Failed  int64  `json:"failed"`
	}
	var trend []dayStat
	rows, _ := h.db.WithContext(ctx).
		Model(&model.PublishResultRecord{}).
		Select("DATE(created_at) as day, status, COUNT(*) as count").
		Where("created_at >= ?", weekAgo).
		Group("DATE(created_at), status").
		Order("day").
		Rows()
	defer rows.Close()

	dayMap := make(map[string]*dayStat)
	for i := 0; i < days; i++ {
		d := weekAgo.AddDate(0, 0, i).Format("2006-01-02")
		dayMap[d] = &dayStat{Day: d}
	}
	for rows.Next() {
		var day, status string
		var count int64
		if err := rows.Scan(&day, &status, &count); err != nil {
			continue
		}
		if ds, ok := dayMap[day]; ok {
			if status == "completed" || status == "edited" {
				ds.Success += count
			} else if status == "failed" {
				ds.Failed += count
			}
		}
	}
	for i := 0; i < days; i++ {
		d := weekAgo.AddDate(0, 0, i).Format("2006-01-02")
		trend = append(trend, *dayMap[d])
	}

	// 目标站分布 Top 10
	type siteCount struct {
		Site  string `json:"site"`
		Count int64  `json:"count"`
	}
	var targetSiteTop []siteCount
	h.db.WithContext(ctx).
		Model(&model.PublishResultRecord{}).
		Select("target_site as site, COUNT(*) as count").
		Where("target_site != ''").
		Group("target_site").
		Order("count DESC").
		Limit(10).
		Find(&targetSiteTop)

	// 状态分布
	type statusCount struct {
		Status string `json:"status"`
		Count  int64  `json:"count"`
	}
	var statusDist []statusCount
	h.db.WithContext(ctx).
		Model(&model.PublishResultRecord{}).
		Select("status, COUNT(*) as count").
		Where("status != ''").
		Group("status").
		Order("count DESC").
		Find(&statusDist)

	Success(w, map[string]interface{}{
		"stats":             stats,
		"recent":            recent,
		"trend":             trend,
		"target_site_top":   targetSiteTop,
		"status_distribution": statusDist,
	})
}

// handleCoverageCache §56.37 遗漏 C: 轻量级覆盖缓存查询（只读 DB，不触发 L0/L1/L2/L3 查询）。
// 用于 CrossSeedPanel Step 2 目标站三色排除。
func (h *PublishTorrentsHandler) handleCoverageCache(w http.ResponseWriter, r *http.Request) {
	infoHash := r.URL.Query().Get("info_hash")
	if infoHash == "" {
		Error(w, http.StatusBadRequest, 40001, "info_hash 必填")
		return
	}

	if h.coverage == nil {
		Success(w, map[string]interface{}{"info_hash": infoHash, "sites": []interface{}{}})
		return
	}

	cached, _ := h.coverage.GetCachedCoverage(r.Context(), infoHash)

	type siteStatus struct {
		SiteName string `json:"siteName"`
		Status   string `json:"status"`
		Source   string `json:"source"`
	}
	sites := make([]siteStatus, 0, len(cached))
	for _, c := range cached {
		if c.Status == model.CoverageConfirmedHas || c.Status == model.CoverageProbablyHas {
			sites = append(sites, siteStatus{
			SiteName: c.SiteName,
			Status:   c.Status,
			Source:   c.Source,
		})
		}
	}

	Success(w, map[string]interface{}{
		"info_hash": infoHash,
		"sites":     sites,
	})
}

// dedupTorrentItems §56.40: 按 name+size 去重，同组优先保留官方小组源站的行。
func (h *PublishTorrentsHandler) dedupTorrentItems(ctx context.Context, items []map[string]interface{}) []map[string]interface{} {
	seen := make(map[string]int)
	var result []map[string]interface{}

	for _, item := range items {
		name, _ := item["name"].(string)
		size, _ := item["size"].(int64)
		key := name + "|" + strconv.FormatInt(size, 10)

		if idx, ok := seen[key]; ok {
			if h.sourceDetector != nil {
				groupName := publish.ExtractGroupName(name)
				if groupName != "" {
					if officialSite := h.sourceDetector.LookupGroup(ctx, groupName); officialSite != "" {
						if itemHasSite(item, officialSite) && !itemHasSite(result[idx], officialSite) {
							result[idx] = item
						}
					}
				}
			}
			continue
		}
		seen[key] = len(result)
		result = append(result, item)
	}
	return result
}

func itemHasSite(item map[string]interface{}, site string) bool {
	sites, ok := item["source_sites"].([]string)
	if !ok {
		return false
	}
	for _, s := range sites {
		if s == site {
			return true
		}
	}
	return false
}

// handleGetSourcePriority §56.40: 读取源站点优先级配置。
func (h *PublishTorrentsHandler) handleGetSourcePriority(w http.ResponseWriter, r *http.Request) {
	priority := h.getSourcePriority(r.Context())
	if len(priority) == 0 {
		priority = h.defaultSourcePriority(r.Context())
	}
	Success(w, map[string]interface{}{"priority": priority})
}

// handleSetSourcePriority §56.40: 保存源站点优先级配置。
func (h *PublishTorrentsHandler) handleSetSourcePriority(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Priority []string `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "invalid body")
		return
	}
	data, _ := json.Marshal(req.Priority)
	h.db.WithContext(r.Context()).Exec(
		"INSERT INTO system_settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
		"source_priority", string(data))
	Success(w, map[string]interface{}{"priority": req.Priority})
}

func (h *PublishTorrentsHandler) getSourcePriority(ctx context.Context) []string {
	var val string
	if err := h.db.WithContext(ctx).Raw("SELECT value FROM system_settings WHERE key = 'source_priority' LIMIT 1").Row().Scan(&val); err != nil {
		return nil
	}
	var priority []string
	if err := json.Unmarshal([]byte(val), &priority); err != nil {
		return nil
	}
	return priority
}

// defaultSourcePriority 生成默认优先级：官组映射站在前，其余 is_source 站按名称排序。
func (h *PublishTorrentsHandler) defaultSourcePriority(ctx context.Context) []string {
	// 只返回 is_source=true 且有 cookie 的站点（对齐 PTNexus）
	var builtinSites []string
	h.db.WithContext(ctx).Model(&model.ReleaseGroupMapping{}).
		Distinct("site_name").
		Where("is_builtin = ?", true).
		Pluck("site_name", &builtinSites)

	var sourceSites []string
	h.db.WithContext(ctx).Model(&model.Site{}).
		Where("enabled = ? AND is_source = ? AND cookie != ''", true, true).
		Order("name").
		Pluck("name", &sourceSites)

	builtinSet := make(map[string]bool, len(builtinSites))
	for _, s := range builtinSites {
		builtinSet[s] = true
	}

	var result []string
	// builtin 官组站排前面（且必须在 sourceSites 列表里，即有 cookie）
	sourceSet := make(map[string]bool, len(sourceSites))
	for _, s := range sourceSites {
		sourceSet[s] = true
	}
	for _, s := range builtinSites {
		if s != "" && sourceSet[s] {
			result = append(result, s)
		}
	}
	for _, s := range sourceSites {
		if !builtinSet[s] {
			result = append(result, s)
		}
	}
	return result
}

// handleGetFetchPriority §59.20: 读取"获取数据"站点优先级。
func (h *PublishTorrentsHandler) handleGetFetchPriority(w http.ResponseWriter, r *http.Request) {
	priority := h.getFetchPrioritySetting(r.Context())
	if len(priority) == 0 {
		priority = h.defaultSourcePriority(r.Context())
	}
	Success(w, map[string]interface{}{"priority": priority})
}

// handleSetFetchPriority §59.20: 保存"获取数据"站点优先级。
func (h *PublishTorrentsHandler) handleSetFetchPriority(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Priority []string `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "invalid body")
		return
	}
	data, _ := json.Marshal(req.Priority)
	h.db.WithContext(r.Context()).Exec(
		"INSERT INTO system_settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
		"fetch_priority", string(data))
	Success(w, map[string]interface{}{"priority": req.Priority})
}

func (h *PublishTorrentsHandler) getFetchPrioritySetting(ctx context.Context) []string {
	var val string
	if err := h.db.WithContext(ctx).Raw("SELECT value FROM system_settings WHERE key = 'fetch_priority' LIMIT 1").Row().Scan(&val); err != nil {
		return nil
	}
	var priority []string
	if err := json.Unmarshal([]byte(val), &priority); err != nil {
		return nil
	}
	return priority
}

// ==================== 种子配置页 API（§59.20） ====================

// handleBatchFetch §59.20: 异步批量获取种子 metadata（"获取数据"按钮）。
func (h *PublishTorrentsHandler) handleBatchFetch(w http.ResponseWriter, r *http.Request) {
	if h.metadataFetcher == nil {
		Error(w, http.StatusServiceUnavailable, 50001, "metadata fetcher 未初始化")
		return
	}
	if h.sourceDetector == nil {
		Error(w, http.StatusServiceUnavailable, 50001, "source detector 未初始化")
		return
	}

	var req struct {
		ClientID string `json:"clientId"`
		Items    []struct {
			Hash     string `json:"hash"`
			Name     string `json:"name"`
			Size     int64  `json:"size"`
			SavePath string `json:"savePath"`
		} `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}
	if len(req.Items) == 0 {
		Error(w, http.StatusBadRequest, 40001, "items 为空")
		return
	}

	h.batchFetch.mu.Lock()
	if h.batchFetch.active {
		h.batchFetch.mu.Unlock()
		Error(w, http.StatusConflict, 40901, "已有批量获取任务在运行")
		return
	}
	h.batchFetch.active = true
	h.batchFetch.total = len(req.Items)
	h.batchFetch.done = 0
	h.batchFetch.failed = 0
	h.batchFetch.items = make([]batchFetchItem, len(req.Items))
	for i, item := range req.Items {
		h.batchFetch.items[i] = batchFetchItem{Hash: item.Hash, Name: item.Name, Status: "pending"}
	}
	h.batchFetch.mu.Unlock()

	go h.runBatchFetch(req.ClientID, req.Items)

	Success(w, map[string]interface{}{"message": "已开始", "total": len(req.Items)})
}

func (h *PublishTorrentsHandler) runBatchFetch(clientID string, items []struct {
	Hash     string `json:"hash"`
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	SavePath string `json:"savePath"`
}) {
	defer func() {
		if r := recover(); r != nil {
			h.logger.Error("runBatchFetch panic", zap.Any("panic", r))
		}
		h.batchFetch.mu.Lock()
		h.batchFetch.active = false
		h.batchFetch.mu.Unlock()
	}()
	ctx := context.Background()
	// §59.21: 查下载器 is_local
	isLocal := false
	if clientID != "" {
		var client model.ClientConfig
		if err := h.db.WithContext(ctx).Where("name = ?", clientID).First(&client).Error; err == nil {
			isLocal = client.IsLocal
		}
	}
	for i, item := range items {
		h.batchFetch.mu.Lock()
		h.batchFetch.items[i].Status = "pending"
		h.batchFetch.mu.Unlock()

		// §59.61 第 4 步: 簇内已有完整数据（先前副本获取后传播）→ 跳过（同簇去重:
		// 站点数据/MI/截图各一次；显式单种子获取不受影响）
		if h.hasCompleteMetadata(ctx, item.Hash) {
			h.logger.Info("cluster skip: metadata exists",
				zap.String("hash", item.Hash[:min(10, len(item.Hash))]))
			h.batchFetch.mu.Lock()
			h.batchFetch.done++
			h.batchFetch.items[i].Status = "done"
			h.batchFetch.mu.Unlock()
			time.Sleep(10 * time.Millisecond)
			continue
		}

		err := h.fetchSingleTorrent(ctx, clientID, item.Hash, item.Name, item.Size, item.SavePath, isLocal)

		h.batchFetch.mu.Lock()
		h.batchFetch.done++
		if err != nil {
			h.batchFetch.failed++
			h.batchFetch.items[i].Status = "failed"
			h.batchFetch.items[i].Error = err.Error()
		} else {
			h.batchFetch.items[i].Status = "done"
		}
		h.batchFetch.mu.Unlock()

		time.Sleep(50 * time.Millisecond)
	}
}

type posterClusterContext struct {
	clientID string
	savePath string
	name     string
}

// clusterCtxFor §59.61 附2: 簇上下文获取——map 加速, miss 从 snapshots 反查
// （4005 批次实锤: map 容量清空丢尾部上下文 → PTGen 修复不回传。反查为权威来源）。
func (h *PublishTorrentsHandler) clusterCtxFor(ctx context.Context, hash string) (posterClusterContext, bool) {
	if h.posterClusterCtx != nil {
		if c, ok := h.posterClusterCtx[hash]; ok {
			return c, true
		}
	}
	var snap model.TorrentSnapshot
	if err := h.db.WithContext(ctx).Where("hash = ? AND is_hidden = 0", hash).First(&snap).Error; err != nil {
		return posterClusterContext{}, false
	}
	c := posterClusterContext{clientID: snap.ClientID, savePath: snap.SavePath, name: snap.Name}
	if h.posterClusterCtx == nil {
		h.posterClusterCtx = make(map[string]posterClusterContext, 256)
	}
	h.posterClusterCtx[hash] = c
	return c, true
}

func (h *PublishTorrentsHandler) fetchSingleTorrent(ctx context.Context, clientID, hash, name string, size int64, savePath string, isLocal bool) error {
	// §59.61 附: 注册簇上下文（applyPosterFallback 异步回传用；固定小容量防泄漏——批量串行覆盖）
	if h.posterClusterCtx == nil {
		h.posterClusterCtx = make(map[string]posterClusterContext, 256)
	}
	h.posterClusterCtx[hash] = posterClusterContext{clientID: clientID, savePath: savePath, name: name}
	var coverageSites []model.SiteCoverageCache
	if h.coverage != nil {
		cs, err := h.coverage.GetCachedCoverage(ctx, hash)
		if err == nil {
			coverageSites = cs
		}
	}

	// §59.61: 簇 comment 直达候选——(client_id, save_path, name) 群聚合（管道 b 快照表）
	clusterTargets := h.buildClusterTargets(ctx, clientID, savePath, name, hash)

	result := h.sourceDetector.SelectFetchSite(ctx, name, coverageSites, clusterTargets)
	if result.SourceSite == "" {
		return fmt.Errorf("无可用源站（制作组未映射 + 无覆盖）")
	}

	fetchCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// §59.36 修订: is_local=true 时本地 MI 提前获取（搜索反查前）——
	// 音频 token 冲突时作为 MI 仲裁的源侧证据（真测量）。失败不阻断（降级②盲放行）。
	var localMI string
	if isLocal && savePath != "" && h.seedPipeline != nil {
		artifacts, artErr := h.seedPipeline.AnalyzeLocalArtifacts(ctx, name, savePath)
		if artErr != nil {
			h.logger.Warn("batch-fetch: pre-fetch local mediainfo failed", zap.String("hash", hash), zap.Error(artErr))
		} else if mi, ok := artifacts["media_info"]; ok {
			localMI, _ = mi.(string)
		}
	}

	var meta *model.TorrentMetadata
	var err error
	if result.TorrentID != "" {
		// §59.61: tid 来源含 coverage_tid/comment_tid——统一走 D3 轻校验入口
		// （coverage_tid 历史也校验：错缓存宁可降级搜索，符合审计精神）
		meta, err = h.metadataFetcher.FetchAndStoreDirect(fetchCtx, hash, result.SourceSite, result.TorrentID, name)
		if err == nil {
			goto fetched
		}
		// 直达失败（D3 拒绝/tid 失效）→ 降级链 §59.65（方案 B，The.Boys 实锤重排——
		// 老 FetchAndStore 内嵌 IYUU 兜底在此抢跑，克隆 iyuu_cache 覆盖源站语义）:
		//   ② 同站搜索（五闸门验证）→ ③ ②a 簇内其他 comment 直达（D3 校验）
		//   → ④ IYUU coverage 末位兜底（保数据完整性，fetch_source=iyuu_cache 审计可辨）
		h.logger.Info("direct fetch failed, fallback chain start",
			zap.String("hash", hash[:min(10, len(hash))]),
			zap.String("site", result.SourceSite),
			zap.Error(err))
		meta, err = h.metadataFetcher.FetchAndStoreBySearch(fetchCtx, hash, result.SourceSite, name, size, localMI)
		if err != nil {
			h.logger.Info("same-site search failed, trying cluster comments",
				zap.String("hash", hash[:min(10, len(hash))]),
				zap.Error(err))
			// ③ ②a: 簇内其他副本的 comment 凭证逐个直达（排除已试源站；D3 校验内建）
			for _, tgt := range clusterTargets {
				if tgt.TorrentID == "" || tgt.SiteName == result.SourceSite {
					continue
				}
				meta, err = h.metadataFetcher.FetchAndStoreDirect(fetchCtx, hash, tgt.SiteName, tgt.TorrentID, name)
				if err == nil {
					h.logger.Info("cluster comment direct hit",
						zap.String("hash", hash[:min(10, len(hash))]),
						zap.String("site", tgt.SiteName),
						zap.String("tid", tgt.TorrentID))
					goto fetched
				}
			}
			// ④ IYUU coverage 末位兜底
			h.logger.Info("cluster comments exhausted, IYUU fallback",
				zap.String("hash", hash[:min(10, len(hash))]))
			meta, err = h.metadataFetcher.FetchAndStoreIYUU(fetchCtx, hash, result.SourceSite)
			if err != nil {
				return err
			}
			h.logger.Info("IYUU fallback hit",
				zap.String("hash", hash[:min(10, len(hash))]),
				zap.String("site", meta.SiteName))
		}
	} else {
		meta, err = h.metadataFetcher.FetchAndStoreBySearch(fetchCtx, hash, result.SourceSite, name, size, localMI)
	}
	if err != nil {
		return err
	}
fetched:

	// §59.42: 海报可信图源白名单替换（异步；§59.61 附5: 尾部 finalize 会等其终局
	// 再传播——INSERT 与回传 UPDATE 的竞态已由 WaitGroup 消除）
	var posterFallbackWg sync.WaitGroup
	if meta != nil && meta.Poster != "" {
		posterFallbackWg.Add(1)
		go func() {
			defer posterFallbackWg.Done()
			h.applyPosterFallback(meta.InfoHash, meta.SiteName, meta.Poster, name)
		}()
	}
	// §59.49+§59.57: 截图探活与策略——strategy 可用时探活内联进其前序（串行消除竞态：
	// 原双 goroutine 并发，strategy 读到 purge 前死链 → rehost 失败保源 → same 早退 → 永不捕获）；
	// strategy 不可用（远程/无 savePath）时探活独立异步执行。
	if meta != nil {
		if h.shotStrategy != nil && savePath != "" {
			go h.applyScreenshotStrategy(clientID, meta.InfoHash, meta.SiteName, name, savePath, isLocal)
		} else if meta.Screenshots != "" && meta.Screenshots != "[]" {
			go h.purgeDeadScreenshots(meta.InfoHash, meta.SiteName)
		}
	}

	// §59.21: is_local=true 时落库本地 mediainfo（localMI 已在搜索前获取，直接复用）
	// §59.56: 列名笔误修复（mediainfo_source → media_info_source，v0.0.650 同族第三处）
	// + .Error 检查（原静默吞错——243 实测 760 次失败无声，148 组本地 MI 全空）
	if isLocal && localMI != "" && meta != nil {
		if err := h.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
			Where("info_hash = ? AND site_name = ?", meta.InfoHash, meta.SiteName).
			Updates(map[string]interface{}{
				"media_info":        localMI,
				"media_info_source": "local",
			}).Error; err != nil {
			h.logger.Error("local mediainfo persist failed",
				zap.String("hash", meta.InfoHash[:10]),
				zap.Error(err))
		}
	}

	// §59.26: TechProfile 三源合并 + 分类推断 + 声明过滤（与 runAnalyze ⑫ 统一管线）
	if meta != nil {
		var finalMeta model.TorrentMetadata
		h.db.WithContext(ctx).Where("info_hash = ? AND site_name = ?", meta.InfoHash, meta.SiteName).First(&finalMeta)

		if finalMeta.Title != "" {
			// §59.26: MI 优先 local（MediaInfo），fallback 源站（SourceMediaInfo）
			miForProfile := finalMeta.MediaInfo
			if miForProfile == "" {
				miForProfile = finalMeta.SourceMediaInfo
			}
			// §59.28 D1: DOM 源接入（DetailSourceJSON 的 medium/codec/resolution），
			// 种子配置页与 runAnalyze 走同一套三源合并管线（§59.26 设计）
			domMedium, domRes, domVideo, domAudio := domFieldsFromDetailSource(finalMeta.DetailSourceJSON)
			profile := titleparser.BuildTechProfile(finalMeta.Title, miForProfile, domMedium, domRes, domVideo, domAudio)
			// §59.77: 评论音轨扣减（v1.05 不计入——副标题声明提取）
			profile.AudioTracks = titleparser.AdjustCommentaryTracks(profile.AudioTracks, finalMeta.Subtitle, miForProfile)
			components := titleparser.TechProfileToComponents(profile)
			category := titleparser.InferCategory(components, finalMeta.SourceCategory, "", "")

			updates := map[string]interface{}{
				"category":        category,
				"resolution":      profile.Resolution,
				"video_codec":     profile.VideoCodec,
				"audio_codec":     profile.AudioCodec,
				"audio_channels":  profile.AudioChannels,
				"audio_tech":      profile.AudioTechnology,
				"audio_tracks":    profile.AudioTracks, // §59.76: v1.05 #16
				"hdr":             profile.HDR,
				"bit_depth":       profile.BitDepth,
				"source_type":     profile.SourceType,
				"specification":   profile.Specification,
				"source_platform": profile.SourcePlatform,
				"edition_info":    profile.EditionInfo,
				"region_code":     profile.RegionCode,
			}

			if h.declFilter != nil && finalMeta.Description != "" {
				patterns := h.declFilter.GetPatterns(ctx)
				fr := h.declFilter.Filter(finalMeta.Description, patterns)
				updates["description"] = fr.CleanedText
			}

			// §59.27: flags 以本次 detail 为准（detail_source_json.flags 是新提取值，
			// 但 GORM Assign 忽略零值导致 DB 旧 flags 残留——map 显式覆盖空值）
			var detailFlags []string
			if finalMeta.DetailSourceJSON != "" {
				var ds struct {
					Flags []string `json:"flags"`
				}
				if json.Unmarshal([]byte(finalMeta.DetailSourceJSON), &ds) == nil {
					detailFlags = ds.Flags
				}
			}
			if detailFlags == nil {
				detailFlags = []string{}
			}
			if newFlags, err := json.Marshal(detailFlags); err == nil {
				updates["flags"] = string(newFlags)
			}

			// §59.26 §59.28: 声明末尾追加转载致谢（幂等——重获不累积）
			// 第一段用 GenerateThanksQuote（含"转自{source_site}"权威模板），
			// 第二段禁转PTT 是用户指定的固定引言（v0.0.597）。
			if profile.ReleaseGroup != "" && profile.ReleaseGroup != "NOGROUP" {
				thanksLine := description.GenerateThanksQuote(meta.SiteName, profile.ReleaseGroup, false, nil)
				noTransferLine := "[quote][b][color=red][size=5]请遵守PT互相遵重共识，禁转PTT[/size][/color][/b][/quote]"
				thanks := "[quote][b][color=blue][size=5]" + thanksLine + "[/size][/color][/b][/quote]\n" + noTransferLine

				// 幂等：先剥离历史追加的致谢块（v0.0.607 修复重获累积）
				base := stripAppendedThanks(finalMeta.Statement)
				// §59.80: 空 base 不接 \n\n 前缀（米仔睡着了实锤前导空行）
				if base == "" {
					updates["statement"] = thanks
				} else {
					updates["statement"] = base + "\n\n" + thanks
				}
			}

			// §59.26: 标签推断（对齐 auto_feed：副标题+标题+简介+MI 多源关键词匹配）
			inferer := publish.NewMediaTagInferer()
			inferredTags := inferer.InferFull(publish.TagInput{
				MediaInfo:   finalMeta.MediaInfo,
				Title:       finalMeta.Title,
				Subtitle:    finalMeta.Subtitle,
				Description: finalMeta.Description,
				NFO:         finalMeta.BDInfo,
				Size:        size, // §59.72 B2: big_pack >1TB
			})
			// 源站显式标签优先，推断只补空（§59.28 G：坏 JSON 不静默——记录并按空处理）
			var existingTags []string
			if finalMeta.Tags != "" {
				if err := json.Unmarshal([]byte(finalMeta.Tags), &existingTags); err != nil {
					h.logger.Warn("seed fetch: tags column invalid json, resetting",
						zap.String("hash", hash), zap.Error(err))
					existingTags = nil
				}
			}
			if len(inferredTags) > 0 {
				// §59.74: MergeTags 单点——归一+直采优先+互斥/覆盖仲裁于合并结果
				all := publish.MergeTags(existingTags, inferredTags)
				if data, err := json.Marshal(all); err == nil {
					updates["tags"] = string(data)
				}
			}

			h.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
				Where("info_hash = ? AND site_name = ?", meta.InfoHash, meta.SiteName).
				Updates(updates)
		}
	}

	// §59.61 第 4 步 + 附5: 簇终局传播——等海报 fallback 终局后 INSERT 缺行
	// （携带终态）+ 终态回传（幂等）
	h.finalizeClusterPropagation(ctx, &posterFallbackWg, clientID, savePath, name, hash, meta.SiteName)

	return nil
}

// domFieldsFromDetailSource §59.28 D1：从 DetailSourceJSON 解析 DOM 源四字段
// （medium/resolution/video_codec/audio_codec），供 BuildTechProfile 三源合并。
func domFieldsFromDetailSource(detailJSON string) (medium, resolution, videoCodec, audioCodec string) {
	if detailJSON == "" {
		return
	}
	var ds struct {
		Medium     string `json:"medium"`
		Resolution string `json:"resolution"`
		VideoCodec string `json:"video_codec"`
		AudioCodec string `json:"audio_codec"`
	}
	if json.Unmarshal([]byte(detailJSON), &ds) != nil {
		return
	}
	// §59.34 审计: detail 提取器存 standard key（medium.webdl/UNK*），
	// ReverseLookup 归一化；未映射 key → 空（MergeDOMInto 跳过，保留 title 值）
	return titleparser.ReverseLookup(ds.Medium),
		titleparser.ReverseLookup(ds.Resolution),
		titleparser.ReverseLookup(ds.VideoCodec),
		titleparser.ReverseLookup(ds.AudioCodec)
}

// thanksAppendMarker §59.28 致谢追加分隔标记：statement 中该标记之后的内容
// 是我们追加的致谢块（重获时剥离，保证幂等）。
const thanksAppendMarker = "FRDS官组作品"

// stripAppendedThanks 剥离历史追加的致谢块（§59.28 幂等修复）。
// 追加格式固定为 "\n\n[quote]...官组作品...[/quote]\n[quote]...禁转PTT...[/quote]"，
// 识别第二个 quote 块（禁转PTT）定位追加起点。
var noTransferQuoteRe = regexp.MustCompile(`(?s)\s*\[quote\]\[b\]\[color=red\]\[size=5\]请遵守PT互相遵重共识，禁转PTT\[/size\]\[/color\]\[/b\]\[/quote\]`)

func stripAppendedThanks(statement string) string {
	if !strings.Contains(statement, thanksAppendMarker) {
		return statement
	}
	// 从最后一个 "官组作品" quote 块的起始 [quote] 剥离到末尾
	idx := strings.LastIndex(statement, thanksAppendMarker)
	if idx < 0 {
		return statement
	}
	// 回溯到包裹它的 [quote] 起点
	start := strings.LastIndex(statement[:idx], "[quote]")
	if start < 0 {
		return statement
	}
	return strings.TrimSpace(statement[:start])
}

// extractChineseFromSubtitle 从副标题提取【中文名】（§59.26 朋友站格式）。
func extractChineseFromSubtitle(subtitle string) string {
	re := regexp.MustCompile(`^[【\[]([^】\]]+)[】\]]`)
	if m := re.FindStringSubmatch(strings.TrimSpace(subtitle)); len(m) > 1 {
		return m[1]
	}
	return ""
}

// handleBatchFetchProgress §59.20: 查询批量获取进度。
func (h *PublishTorrentsHandler) handleBatchFetchProgress(w http.ResponseWriter, r *http.Request) {
	h.batchFetch.mu.Lock()
	defer h.batchFetch.mu.Unlock()
	Success(w, map[string]interface{}{
		"active": h.batchFetch.active,
		"total":  h.batchFetch.total,
		"done":   h.batchFetch.done,
		"failed": h.batchFetch.failed,
		"items":  h.batchFetch.items,
	})
}

// handleListSeeds §59.20 §59.29: 种子配置页列表 API。
// 数据流：snapshots(可选 client/path 过滤) → name 去重 → metadata → 状态标注 →
// 状态/搜索后端过滤 → 分页。
// §59.29: client_id/save_path 全可选（空=全部），进页面即有数据（方案 A 全部视图）。
// observingGracePeriod §59.38: 观察期滞后期——(client,name) 全变体 hidden 持续
// 满 7 天自动清理（TR 维护/移路径/误删重加的恢复窗口）。
const observingGracePeriod = 7 * 24 * time.Hour

// handleListObserving §59.38: 观察期视图——辅种组（client+name）全部变体 hidden 的资源。
// 行内容：name + 变体数 + 消失时间（max last_seen）+ 清理倒计时 + 下载器/路径。
// 资源实体语义（用户定案）：辅种 = 同下载器同资源不同站点；跨下载器为独立副本互不影响。
func (h *PublishTorrentsHandler) handleListObserving(w http.ResponseWriter, r *http.Request, clientID, savePath, search string) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page == 0 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize == 0 {
		pageSize = 50
	}
	if pageSize > 100 {
		pageSize = 100 // §59.166 单批发布上限 100 对齐（localStorage 脏值防线）
	}

	// 观察组 = (client, name) 聚合：全变体 hidden（组内无活跃行）且 name 非空
	type obsRow struct {
		ClientID string
		Name     string
		Variants int64
		LastSeen string // MAX() 聚合返回 string（driver 层），解析用
		SavePath string
		Size     int64
	}
	q := h.db.WithContext(r.Context()).
		Table("torrent_snapshots").
		Select(`client_id, name, COUNT(*) AS variants, MAX(last_seen) AS last_seen,
			(SELECT save_path FROM torrent_snapshots s2 WHERE s2.client_id = torrent_snapshots.client_id AND s2.name = torrent_snapshots.name ORDER BY last_seen DESC LIMIT 1) AS save_path,
			MAX(size) AS size`).
		// §59.38 审计修正: WHERE 不限 hidden——HAVING 活跃检测需全组行进聚合域
		//（原 is_hidden=1 使 SUM(active) 恒 0，组内仍有活跃行的资源被误列观察期）
		Where("name != ''").
		Group("client_id, name").
		Having("SUM(CASE WHEN is_hidden = 0 THEN 1 ELSE 0 END) = 0 AND SUM(CASE WHEN is_hidden = 1 THEN 1 ELSE 0 END) > 0")
	if clientID != "" {
		q = q.Where("client_id = ?", clientID)
	}
	if savePath != "" {
		// §59.38 审计修正: 按 (client_id, name) 组过滤——原 IN client 放大返回该下载器全部组
		q = q.Where("client_id || '|' || name IN (SELECT client_id || '|' || name FROM torrent_snapshots WHERE save_path = ?)", savePath)
	}
	if search != "" {
		q = q.Where("name LIKE ?", "%"+search+"%")
	}
	var rows []obsRow
	if err := q.Order("last_seen DESC").Find(&rows).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询观察期失败: "+err.Error())
		return
	}

	items := make([]map[string]interface{}, 0, len(rows))
	now := time.Now()
	for _, row := range rows {
		lastSeen, _ := time.Parse("2006-01-02 15:04:05.999999999-07:00", row.LastSeen)
		if lastSeen.IsZero() {
			lastSeen, _ = time.Parse(time.RFC3339, row.LastSeen)
		}
		remainingDays := 0
		if !lastSeen.IsZero() {
			remaining := observingGracePeriod - now.Sub(lastSeen)
			remainingDays = int(remaining.Hours() / 24)
			if remainingDays < 0 {
				remainingDays = 0
			}
		}
		items = append(items, map[string]interface{}{
			"client_id":       row.ClientID,
			"name":            row.Name,
			"variants":        row.Variants,
			"last_seen":       row.LastSeen,
			"save_path":       row.SavePath,
			"size":            row.Size,
			"cleanup_in_days": remainingDays,
			"status":          "observing",
		})
	}

	total := len(items)
	start := (page - 1) * pageSize
	if start >= total {
		items = []map[string]interface{}{}
	} else {
		end := start + pageSize
		if end > total {
			end = total
		}
		items = items[start:end]
	}
	Success(w, map[string]interface{}{"items": items, "total": total})
}

// handlePurgeObserving §59.38: 立即清理——观察期资源主动确认不等 7 天。
// 两级判定（防跨下载器 metadata 误删）：
//   ① 删该 (client, name) 的全部 hidden 快照行
//   ② metadata 仅当 info_hash 不被任何其他下载器活跃/观察期快照引用才删
func (h *PublishTorrentsHandler) handlePurgeObserving(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ClientID string `json:"clientId"`
		Name     string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ClientID == "" || req.Name == "" {
		Error(w, http.StatusBadRequest, 40001, "clientId 和 name 必填")
		return
	}
	deleted := h.purgeObservingResource(r.Context(), req.ClientID, req.Name)
	Success(w, map[string]interface{}{
		"message":        "已清理",
		"deleted_snaps":  deleted.snapRows,
		"deleted_metas":  deleted.metaRows,
	})
}

type purgeResult struct {
	snapRows  int64
	metaRows  int64
}

// purgeObservingResource 两级清理（立即清理与定时任务共用）。
// 安全前置：仅当该组确无活跃行才执行（活跃=误判，拒绝清理）。
func (h *PublishTorrentsHandler) purgeObservingResource(ctx context.Context, clientID, name string) purgeResult {
	var active int64
	h.db.WithContext(ctx).Model(&model.TorrentSnapshot{}).
		Where("client_id = ? AND name = ? AND is_hidden = ?", clientID, name, false).
		Count(&active)
	if active > 0 {
		return purgeResult{}
	}

	// ① 该组全部 hash
	var hashes []string
	h.db.WithContext(ctx).Model(&model.TorrentSnapshot{}).
		Where("client_id = ? AND name = ?", clientID, name).
		Pluck("hash", &hashes)
	if len(hashes) == 0 {
		return purgeResult{}
	}

	// ② metadata 两级判定：hash 被其他下载器（或本下载器其他组）活跃引用则跳过
	var referenced []string
	h.db.WithContext(ctx).
		Table("torrent_snapshots").
		Where("hash IN ? AND is_hidden = ? AND (client_id != ? OR name != ?)", hashes, false, clientID, name).
		Distinct("hash").
		Pluck("hash", &referenced)
	refSet := make(map[string]bool, len(referenced))
	for _, h2 := range referenced {
		refSet[h2] = true
	}
	purgeHashes := make([]string, 0, len(hashes))
	for _, h2 := range hashes {
		if !refSet[h2] {
			purgeHashes = append(purgeHashes, h2)
		}
	}

	var res purgeResult
	// 删 metadata（发布记录不动——发布域史实）
	if len(purgeHashes) > 0 {
		d := h.db.WithContext(ctx).Where("info_hash IN ?", purgeHashes).
			Delete(&model.TorrentMetadata{})
		res.metaRows = d.RowsAffected
	}
	// 删该组快照行
	d := h.db.WithContext(ctx).Where("client_id = ? AND name = ?", clientID, name).
		Delete(&model.TorrentSnapshot{})
	res.snapRows = d.RowsAffected
	h.logger.Info("observing resource purged",
		zap.String("client", clientID),
		zap.String("name", name[:min(len(name), 50)]),
		zap.Int64("snaps", res.snapRows),
		zap.Int64("metas", res.metaRows))
	return res
}


func (h *PublishTorrentsHandler) handleListSeeds(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("client_id")
	savePath := r.URL.Query().Get("save_path")
	statusFilter := r.URL.Query().Get("status")
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	// §59.131 ②: ready 过滤——发布页（一种多站）簇口径 data_ready 筛选（R3-3: 簇级=reviewed）
	readyFilter := r.URL.Query().Get("ready")
	// §59.143: 发布页排除源站禁转——forbidden（flags 内存判定零成本）直接隐藏；
	// system_forbidden 精判在分页后（排除致页不齐）——保留红标, Eligibility 提交兜底
	excludeForbidden := r.URL.Query().Get("exclude_forbidden") == "true"

	// §59.166 一站多种：目标站发布维度筛选（publish_state=publishable/published
	// + target_site）。口径（用户定案）：已发布=终态有记录（pushed/pushed_existing/
	// uploaded/duplicate/existing——uploaded=上传成功站上已有，§59.166 补入：
	// 漏种簇不再留在"可发布"防整体重传）；可发布=已审核（reviewed 蕴含资产完备——种配审核
	// 流程权威保证）AND 无四终态记录（failed 算未发布可重试）；关联键=簇成员 hash
	// 集合反查簇键（宁漏勿错——早期空 source_info_hash 记录匹配不上→落可发布，
	// dedup 兜底）。
	publishState := r.URL.Query().Get("publish_state")
	publishTargetSite := r.URL.Query().Get("target_site")
	var publishedClusters map[string]bool
	if (publishState == "publishable" || publishState == "published") && publishTargetSite != "" {
		var pubHashes []string
		h.db.WithContext(r.Context()).
			Table("publish_result_records").
			Where("target_site = ? AND source_info_hash != '' AND status IN ?",
				publishTargetSite, []string{"pushed", "pushed_existing", "uploaded", "duplicate", "existing"}).
			Distinct("source_info_hash").
			Pluck("source_info_hash", &pubHashes)
		publishedClusters = make(map[string]bool)
		if len(pubHashes) > 0 {
			type ckRow struct {
				ClientID string
				SavePath string
				Name     string
			}
			var cks []ckRow
			h.db.WithContext(r.Context()).
				Table("torrent_snapshots").
				Select("DISTINCT client_id, save_path, name").
				Where("hash IN ? AND is_hidden = 0 AND name != ''", pubHashes).
				Find(&cks)
			for _, c := range cks {
				publishedClusters[c.ClientID+"|"+c.SavePath+"|"+c.Name] = true
			}
		}
	}

	// §59.38: 观察期视图——独立分支（hidden 组按 (client,name) 聚合，
	// 不与活跃视图共用查询/状态机）
	if statusFilter == "observing" {
		h.handleListObserving(w, r, clientID, savePath, search)
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page == 0 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize == 0 {
		pageSize = 50
	}
	if pageSize > 100 {
		pageSize = 100 // §59.166 单批发布上限 100 对齐（localStorage 脏值防线）
	}

	// 1. 查 snapshots——§59.26: 先去重再分页；§59.29: SQL 层分组去重（20w+ 行时
	// 避免 Go 全量载入）。同 client+path+name 的多站点 hash 只取 updated_at 最新一行。
	// 无 name 的行按 hash 自成一组（name='' 时 MAX(hash) 无意义但保持行为：空名行退化
	// 为按 hash 去重等价于不去重）。
	snapQuery := h.db.WithContext(r.Context()).
		Table("torrent_snapshots AS s").
		Select("s.*").
		Where("s.is_hidden = ?", false)

	// SQLite/MySQL 兼容的"组内最新行"：子查询取每组 MAX(id)。
	// name 为空时用 hash 作为组键（等效不去重）。
	sub := h.db.WithContext(r.Context()).
		Table("torrent_snapshots").
		Select("MAX(id) AS id").
		Where("is_hidden = ?", false).
		Group("client_id, save_path, CASE WHEN name = '' THEN hash ELSE name END")
	if clientID != "" {
		snapQuery = snapQuery.Where("s.client_id = ?", clientID)
		sub = sub.Where("client_id = ?", clientID)
	}
	if savePath != "" {
		snapQuery = snapQuery.Where("s.save_path = ?", savePath)
		sub = sub.Where("save_path = ?", savePath)
	}
	// §59.29: 搜索下推 SQL，20w+ 行时避免 Go 层全量过滤
	// §59.138: 三域搜索——CJK 词跨表扩 title/subtitle（Yumi 纯英文簇的中文剧名在
	// subtitle 域）；拉丁词纯 name LIKE。回归审核定案：29 大库 FLAC 全表跨表子查询
	// 69.8s 爆炸（大命中集 O(N×M)），CJK 门控消解（中文剧名命中集天然小）。
	// snapQuery 不再重复 search 过滤——代表行组键=name，sub 已筛组（§59.29 双保险收敛）。
	// sub/hashQ 两查询点必须同域——漏一处列表假阴性（§59.28 H1 同族）。
	searchHasCJK := false
	for _, r := range search {
		if unicode.Is(unicode.Han, r) {
			searchHasCJK = true
			break
		}
	}
	var searchConds []string
	var searchArgs []interface{}
	if search != "" {
		if searchHasCJK {
			searchConds = []string{"name LIKE ?", "OR name IN (SELECT s2.name FROM torrent_snapshots s2 JOIN torrent_metadata m ON m.info_hash = s2.hash WHERE m.title LIKE ? OR m.subtitle LIKE ?)"}
			searchArgs = []interface{}{"%" + search + "%", "%" + search + "%", "%" + search + "%"}
		} else {
			searchConds = []string{"name LIKE ?"}
			searchArgs = []interface{}{"%" + search + "%"}
		}
		sub = sub.Where(strings.Join(searchConds, " "), searchArgs...)
	}
	var rawSnapshots []model.TorrentSnapshot
	if err := snapQuery.
		Where("s.id IN (?)", sub).
		Order("s.updated_at DESC").
		Find(&rawSnapshots).Error; err != nil {
		Success(w, map[string]interface{}{"items": []interface{}{}, "total": 0})
		return
	}

	// §59.26: 按 name 去重（SQL 分组已去重，此处为双保险 + 保序语义）。
	// §59.29: 全部视图下 key 为 client+path+name（不同下载器/路径的同名资源是不同实体）
	snapshots := util.DedupByKey(rawSnapshots, func(s model.TorrentSnapshot) string {
		if s.Name != "" {
			return s.ClientID + "|" + s.SavePath + "|" + s.Name
		}
		return s.Hash
	})

	if len(snapshots) == 0 {
		Success(w, map[string]interface{}{"items": []interface{}{}, "total": 0})
		return
	}

	// 2. 收集 hash → 查 metadata（§59.29 性能：2.2w hash 的 IN 查询过慢，
	// 改为子查询 JOIN——同名资源的所有站点 hash 的 metadata 一并取出，
	// 解决去重保留行 hash 与 metadata 行 hash 不一致时丢数据的问题（§59.28 H1））
	nameList := make([]string, len(snapshots))
	for i, s := range snapshots {
		nameList[i] = s.Name
	}
	var metas []model.TorrentMetadata
	h.db.WithContext(r.Context()).
		Where("info_hash IN (SELECT hash FROM torrent_snapshots WHERE name IN ? AND is_hidden = 0)", nameList).
		Find(&metas)

	// 3. 按 name 关联 metadata（§59.29: name → 同名所有 hash 的 metadata 行，
	// 解决去重保留行与 metadata 行 hash 不一致时丢数据 §59.28 H1）
	// §59.38 修复: nameByHash 域放宽到 (client,path) 全部活跃行（原 rawSnapshots
	// 是分组去重保留行——同资源多变体场景 metadata 挂在非保留行上时映射丢失，
	// 列表假阴性标 unfetched。H1 只放宽了 metas 查询没放宽映射域，半修）
	nameByHash := make(map[string]string, len(rawSnapshots)*2)
	{
		var hashNameRows []struct {
			Hash string
			Name string
		}
		hashQ := h.db.WithContext(r.Context()).
			Table("torrent_snapshots").
			Select("hash, name").
			Where("is_hidden = ? AND name != ''", false)
		if clientID != "" {
			hashQ = hashQ.Where("client_id = ?", clientID)
		}
		if savePath != "" {
			hashQ = hashQ.Where("save_path = ?", savePath)
		}
		if search != "" {
			// §59.138: nameByHash 域同步搜索域（CJK 三域/拉丁 name——漏则假阴性）
			hashQ = hashQ.Where(strings.Join(searchConds, " "), searchArgs...)
		}
		hashQ.Find(&hashNameRows)
		for _, r := range hashNameRows {
			nameByHash[r.Hash] = r.Name
		}
	}
	metaByName := make(map[string][]model.TorrentMetadata, len(snapshots))
	for _, m := range metas {
		if name, ok := nameByHash[m.InfoHash]; ok {
			metaByName[name] = append(metaByName[name], m)
		}
	}

	// §59.131 ②: 簇副本数——发布页（一种多站）增量字段。
	// 按 (client,path,name) 聚合活跃快照行数；不施加 search 过滤（簇成员数与
	// 搜索无关，search 只决定哪些簇可见）。name='' 行不入此表，副本数退化 1。
	type clusterCountRow struct {
		ClientID string
		SavePath string
		Name     string
		Cnt      int
	}
	var countRows []clusterCountRow
	countQ := h.db.WithContext(r.Context()).
		Table("torrent_snapshots").
		Select("client_id, save_path, name, COUNT(*) AS cnt").
		Where("is_hidden = ? AND name != ''", false)
	if clientID != "" {
		countQ = countQ.Where("client_id = ?", clientID)
	}
	if savePath != "" {
		countQ = countQ.Where("save_path = ?", savePath)
	}
	countQ.Group("client_id, save_path, name").Find(&countRows)
	copyCountByKey := make(map[string]int, len(countRows))
	for _, cr := range countRows {
		copyCountByKey[cr.ClientID+"|"+cr.SavePath+"|"+cr.Name] = cr.Cnt
	}
	clusterKey := func(c, p, n string) string { return c + "|" + p + "|" + n }

	// 4. 组装结果（§59.29 性能：轻量状态标注全量执行——flags/映射/reviewed 纯内存或
	// 带缓存查询；compliance（含 per-row DB 查询）延迟到分页后的当前页执行，
	// 未筛选 system_forbidden 时默认降级为轻量判定，筛该态时对全量补跑）
	needFullCompliance := statusFilter == "system_forbidden"
	items := make([]map[string]interface{}, 0, len(snapshots))
	for _, snap := range snapshots {
		item := map[string]interface{}{
			"hash":      snap.Hash,
			"name":      snap.Name,
			"size":      snap.Size,
			"client_id": snap.ClientID,
			"save_path": snap.SavePath,
		}

		// 查找源站行 metadata（按 name 关联，含同名兄弟 hash 的行）
		var meta *model.TorrentMetadata
		if metas, ok := metaByName[snap.Name]; ok && len(metas) > 0 {
			meta = h.selectSourceMeta(metas)
		}

		if meta != nil {
			item["site_name"] = meta.SiteName
			item["title"] = meta.Title
			item["subtitle"] = meta.Subtitle
			item["poster"] = meta.Poster
			item["reviewed"] = meta.Reviewed
			item["flags"] = meta.Flags
			item["source_category"] = meta.SourceCategory
			item["fetch_source"] = meta.FetchSource
			item["has_mediainfo"] = meta.MediaInfo != ""
			item["has_description"] = meta.Description != ""
			item["has_screenshots"] = meta.Screenshots != ""
			item["fetched_at"] = meta.FetchedAt
		// 技术参数（列表展示用）
		item["resolution"] = meta.Resolution
		item["video_codec"] = meta.VideoCodec
		item["audio_codec"] = meta.AudioCodec
		item["audio_channels"] = meta.AudioChannels
		item["audio_tech"] = meta.AudioTech
		item["hdr"] = meta.HDR
		item["source_type"] = meta.SourceType
		item["specification"] = meta.Specification
		// §59.34: Encode 派生（v1.05 Encode 规格为空，由片源写法/编码族区分）
		item["encode"] = titleparser.IsEncode(titleparser.TechProfile{
			SourceType:    meta.SourceType,
			Specification: meta.Specification,
			VideoCodec:    meta.VideoCodec,
		})
		item["category"] = meta.Category
		item["form"] = meta.Form
		} else {
			item["reviewed"] = false
			item["fetched"] = false
			item["title"] = ""
		}

		// 5 态标注（轻量：跳过 compliance）
		status := h.classifySeedStatusLite(r.Context(), snap.Name, meta)
		item["status"] = status

		// §59.131 ②: 副本数 + 站点列表（簇内已有站点——发布页"已存在"标注数据源）
		cc := copyCountByKey[clusterKey(snap.ClientID, snap.SavePath, snap.Name)]
		if cc == 0 {
			cc = 1
		}
		item["copy_count"] = cc
		siteSet := make(map[string]bool)
		for _, m := range metaByName[snap.Name] {
			if m.SiteName != "" {
				siteSet[m.SiteName] = true
			}
		}
		sites := make([]string, 0, len(siteSet))
		for s := range siteSet {
			sites = append(sites, s)
		}
		sort.Strings(sites)
		item["sites"] = sites

		// §59.143: 发布页隐藏源站禁转簇（种子配置页不传此参数——数据维护视角可见）
		if excludeForbidden && status == "forbidden" {
			continue
		}

		// §59.131 ②: ready 过滤（data_ready=reviewed 口径，R3-3）
		if readyFilter != "" {
			isReady := meta != nil && meta.Reviewed
			if readyFilter == "true" && !isReady {
				continue
			}
			if readyFilter == "false" && isReady {
				continue
			}
		}

		// §59.166 一站多种：发布维度过滤（publishable 自含 reviewed——三态筛选
		// 无独立 ready 选项；published=历史事实回溯）
		if (publishState == "publishable" || publishState == "published") && publishTargetSite != "" {
			isPub := publishedClusters[snap.ClientID+"|"+snap.SavePath+"|"+snap.Name]
			if publishState == "published" && !isPub {
				continue
			}
			if publishState == "publishable" {
				if isPub || meta == nil || !meta.Reviewed {
					continue
				}
			}
		}

		// §59.29: 后端过滤（状态 + 搜索），与分页解耦
		if statusFilter != "" && statusFilter != "system_forbidden" {
			if statusFilter == "issues" {
				// 复合态：禁转/系统禁转/无映射
				if status != "forbidden" && status != "no_mapping" {
					continue
				}
			} else if status != statusFilter {
				continue
			}
		}
		if search != "" {
			sLower := strings.ToLower(search)
			// §59.138: Go 层双保险同步三域（name/title/subtitle）
			subtitle := ""
			if meta != nil {
				subtitle = meta.Subtitle
			}
			if !strings.Contains(strings.ToLower(snap.Name), sLower) &&
				!strings.Contains(strings.ToLower(item["title"].(string)), sLower) &&
				!strings.Contains(strings.ToLower(subtitle), sLower) {
				continue
			}
		}

		items = append(items, item)
	}

	// §59.29: 筛 system_forbidden 时对全量补跑 compliance（结果集已缩小）
	if needFullCompliance {
		filtered := items[:0]
		for _, item := range items {
			name, _ := item["name"].(string)
			siteName, _ := item["site_name"].(string)
			if r2 := h.complianceChecker.CheckWithSite(r.Context(), name, siteName); r2 != nil && !r2.Passed {
				item["status"] = "system_forbidden"
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}

	total := int64(len(items))

	// 去重+过滤后再分页
	start := (page - 1) * pageSize
	if start > len(items) {
		items = nil
	} else {
		end := start + pageSize
		if end > len(items) {
			end = len(items)
		}
		items = items[start:end]
	}

	// §59.29: 当前页补跑 compliance（未筛选时全量循环已跳过）——
	// 只影响本页状态标注精度，50 行可接受
	if !needFullCompliance && h.complianceChecker != nil {
		for _, item := range items {
			name, _ := item["name"].(string)
			siteName, _ := item["site_name"].(string)
			status, _ := item["status"].(string)
			// 轻量态非禁转/无映射的行才需要 compliance 精化
			if status != "forbidden" && status != "no_mapping" {
				if r2 := h.complianceChecker.CheckWithSite(r.Context(), name, siteName); r2 != nil && !r2.Passed {
					item["status"] = "system_forbidden"
				}
			}
		}
	}

	Success(w, map[string]interface{}{
		"items": items,
		"total": total,
	})
}

// handleRecomputeProfiles §59.151: 批量重推 MI 派生列+tags（存量自愈端点）。
// 对全部有本地 MI 的 metadata 行：读 MI → BuildTechProfile → 更新 v1.05 技术列
// + 重推 tags（MI 层判据版）。幂等——MI 是唯一真相，重推收敛同一值。
func (h *PublishTorrentsHandler) handleRecomputeProfiles(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var rows []model.TorrentMetadata
	if err := h.db.WithContext(ctx).
		Where("media_info != ''").Find(&rows).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询失败: "+err.Error())
		return
	}
	recomputed, skipped := 0, 0
	for _, m := range rows {
		mi := m.MediaInfo
		if mi == "" {
			mi = m.SourceMediaInfo
		}
		if mi == "" {
			skipped++
			continue
		}
		domMedium, domRes, domVideo, domAudio := domFieldsFromDetailSource(m.DetailSourceJSON)
		profile := titleparser.BuildTechProfile(m.Title, mi, domMedium, domRes, domVideo, domAudio)
		profile.AudioTracks = titleparser.AdjustCommentaryTracks(profile.AudioTracks, m.Subtitle, mi)
		components := titleparser.TechProfileToComponents(profile)
		category := titleparser.InferCategory(components, m.SourceCategory, "", "")
		updates := map[string]interface{}{
			"category":        category,
			"resolution":      profile.Resolution,
			"video_codec":     profile.VideoCodec,
			"audio_codec":     profile.AudioCodec,
			"audio_channels":  profile.AudioChannels,
			"audio_tech":      profile.AudioTechnology,
			"audio_tracks":    profile.AudioTracks,
			"hdr":             profile.HDR,
			"bit_depth":       profile.BitDepth,
			"source_type":     profile.SourceType,
			"specification":   profile.Specification,
			"source_platform": profile.SourcePlatform,
			"edition_info":    profile.EditionInfo,
			"region_code":     profile.RegionCode,
		}
		// tags 重推（MI 层判据版——§59.151 HDR/语言族结构化）
		inferer := publish.NewMediaTagInferer()
		inferred := inferer.Infer(mi, m.Title)
		if sub := m.Subtitle; sub != "" {
			// 副标题语义补充（infer 全输入）
			inferred = inferer.InferFull(publish.TagInput{
				MediaInfo: mi, Title: m.Title, Subtitle: sub,
				Description: m.Description, Statement: m.Statement,
				Region: regionLabelsOfMeta(m),
			})
		}
		if data, err := json.Marshal(inferred); err == nil {
			updates["tags"] = string(data)
		}
		if err := h.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
			Where("info_hash = ? AND site_name = ?", m.InfoHash, m.SiteName).
			Updates(updates).Error; err != nil {
			h.logger.Warn("recompute profile failed",
				zap.String("hash", m.InfoHash[:10]), zap.Error(err))
			skipped++
			continue
		}
		recomputed++
	}
	Success(w, map[string]interface{}{
		"recomputed": recomputed,
		"skipped":    skipped,
	})
}

// handleSeedUniquePaths §59.29: 返回有快照的 下载器→路径 树（筛选弹层数据源）。
func (h *PublishTorrentsHandler) handleSeedUniquePaths(w http.ResponseWriter, r *http.Request) {
	type pathRow struct {
		ClientID string
		SavePath string
		Count    int64
	}
	var rows []pathRow
	h.db.WithContext(r.Context()).Model(&model.TorrentSnapshot{}).
		Select("client_id, save_path, COUNT(*) as count").
		Where("is_hidden = ?", false).
		Group("client_id, save_path").
		Order("client_id, save_path").
		Find(&rows)

	clients := make([]map[string]interface{}, 0, len(rows))
	byClient := map[string]*map[string]interface{}{}
	for _, row := range rows {
		c, ok := byClient[row.ClientID]
		if !ok {
			entry := map[string]interface{}{
				"client_id": row.ClientID,
				"paths":     make([]map[string]interface{}, 0, 4),
			}
			byClient[row.ClientID] = &entry
			clients = append(clients, entry)
			c = &entry
		}
		(*c)["paths"] = append((*c)["paths"].([]map[string]interface{}), map[string]interface{}{
			"save_path": row.SavePath,
			"count":     row.Count,
		})
	}

	Success(w, map[string]interface{}{"clients": clients})
}

// selectSourceMeta 从同一 hash 的多行 metadata 中选出源站行（制作组映射命中优先，否则 updated_at 最新）。
func (h *PublishTorrentsHandler) selectSourceMeta(metas []model.TorrentMetadata) *model.TorrentMetadata {
	if len(metas) == 0 {
		return nil
	}
	ctx := context.Background()
	var best *model.TorrentMetadata
	for i := range metas {
		if best == nil || metas[i].UpdatedAt.After(best.UpdatedAt) {
			best = &metas[i]
		}
	}
	// 尝试找制作组映射命中行
	if h.sourceDetector != nil {
		for i := range metas {
			group := publish.ExtractGroupName(metas[i].Title)
			if group != "" {
				site := h.sourceDetector.LookupGroup(ctx, group)
				if site == metas[i].SiteName {
					return &metas[i]
				}
			}
		}
	}
	return best
}

// classifySeedStatusLite §59.29: 轻量状态标注（跳过 compliance 的 per-row DB 查询）。
// 全量循环用；compliance 由调用方对当前页（或筛选 system_forbidden 时对全量）补跑。
func (h *PublishTorrentsHandler) classifySeedStatusLite(ctx context.Context, name string, meta *model.TorrentMetadata) string {
	// ① flags 检查
	if meta != nil && meta.Flags != "" {
		var flags []string
		if err := json.Unmarshal([]byte(meta.Flags), &flags); err == nil {
			forbiddenSet := map[string]bool{
				"禁转": true, "禁止转载": true, "谢绝转载": true, "严禁转载": true,
				"谢绝搬运": true, "独占": true, "限转": true,
			}
			for _, f := range flags {
				if forbiddenSet[f] {
					return "forbidden"
				}
				// §59.162: 限时禁转两态——now<until 才拦（keepfrds 24h 窗到期自动放行）
				if f == "限时禁转" {
					if meta.NoTransferUntil == nil || meta.NoTransferUntil.After(time.Now()) {
						return "forbidden"
					}
				}
			}
		}
	}

	// ③ 制作组映射检查
	if h.sourceDetector != nil {
		group := publish.ExtractGroupName(name)
		if group == "" {
			return "no_mapping"
		}
		site := h.sourceDetector.LookupGroup(ctx, group)
		if site == "" {
			return "no_mapping"
		}
	}

	// 无 metadata → 未获取
	if meta == nil {
		return "unfetched"
	}

	// ④ reviewed=true
	if meta.Reviewed {
		return "reviewed"
	}

	// ⑤⑥ 9 字段校验
	missing := h.checkRequiredFields(meta)
	if len(missing) > 0 {
		return "incomplete"
	}
	return "pending"
}

// handleAuditInfoHash §59.59 审计: 诊断端点——下载指定站点的 .torrent 并计算
// infohash 与期望值比对（获取链路正确性的确定性判据）。独立于获取链，不写库。
func (h *PublishTorrentsHandler) handleAuditInfoHash(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Site       string `json:"site"`
		TorrentID  string `json:"torrent_id"`
		ExpectHash string `json:"expect_hash"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Site == "" || req.TorrentID == "" {
		Error(w, http.StatusBadRequest, 40001, "site/torrent_id 必填")
		return
	}
	if h.siteProvider == nil {
		Error(w, http.StatusServiceUnavailable, 50001, "site provider 未配置")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	config, err := h.siteProvider.GetSiteConfig(ctx, req.Site)
	if err != nil || config == nil {
		Error(w, http.StatusBadGateway, 50201, fmt.Sprintf("获取站点配置失败: %v", err))
		return
	}
	adapter, err := h.siteProvider.GetAdapter(ctx, req.Site)
	if err != nil || adapter == nil {
		Error(w, http.StatusBadGateway, 50202, fmt.Sprintf("获取适配器失败: %v", err))
		return
	}
	data, err := adapter.DownloadTorrent(ctx, config, req.TorrentID)
	if err != nil {
		Error(w, http.StatusBadGateway, 50203, fmt.Sprintf("下载 .torrent 失败: %v", err))
		return
	}
	meta, err := fingerprint.ComputeFromTorrent(data)
	if err != nil || meta == nil {
		Error(w, http.StatusInternalServerError, 50002, fmt.Sprintf("解析 .torrent 失败: %v", err))
		return
	}
	noSrc, _ := fingerprint.ComputeNoSourceHash(data)
	Success(w, map[string]interface{}{
		"site":              req.Site,
		"torrent_id":        req.TorrentID,
		"info_hash":         meta.InfoHash,
		"expect_hash":       req.ExpectHash,
		"match":             strings.EqualFold(meta.InfoHash, req.ExpectHash),
		"info_hash_no_source": noSrc,
	})
}

// checkRequiredFields §59.20: 9 必需字段校验，返回缺失字段名列表。
func (h *PublishTorrentsHandler) checkRequiredFields(meta *model.TorrentMetadata) []string {
	var missing []string
	if meta.Title == "" {
		missing = append(missing, "title")
	}
	if meta.Poster == "" {
		missing = append(missing, "poster")
	}
	// §59.53: 门槛升级——非空 → 解析后 < MinScreenshots 即缺失（凑数+审核双保险）
	if len(model.ParseScreenshotColumn(meta.Screenshots)) < publish.MinScreenshots {
		missing = append(missing, "screenshots")
	}
	if meta.Description == "" {
		missing = append(missing, "description")
	}
	if meta.MediaInfo == "" {
		missing = append(missing, "mediainfo")
	}
	if meta.Resolution == "" {
		missing = append(missing, "resolution")
	}
	if meta.VideoCodec == "" {
		missing = append(missing, "video_codec")
	}
	if meta.AudioCodec == "" {
		missing = append(missing, "audio_codec")
	}
	// 制作组从标题解析
	if h.sourceDetector != nil {
		if g := publish.ExtractGroupName(meta.Title); g == "" {
			missing = append(missing, "release_group")
		}
	}
	return missing
}

// extractSeedHash 从 URL 路径 /api/v1/publish/seeds/:info_hash 提取 info_hash。
func extractSeedHash(r *http.Request) string {
	path := strings.TrimRight(r.URL.Path, "/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

// handleGetSeed §59.20: 读取单个种子 metadata（GET /publish/seeds/:info_hash）。
// 返回 DB 14 平铺字段 + ParseTitleTech 解析 5 字段 = 完整 18 TechProfile + 编辑字段。
func (h *PublishTorrentsHandler) handleGetSeed(w http.ResponseWriter, r *http.Request) {
	infoHash := extractSeedHash(r)
	if infoHash == "" || infoHash == "seeds" {
		Error(w, http.StatusBadRequest, 40001, "缺少 info_hash")
		return
	}

	// §59.44: 资源视图解析——按 (client,path,name) 圈全部活跃 hash 聚合 metadata，
	// 修复挂载 hash≠保留行 hash 的 404（49% 资源组）；hidden 兜底覆盖编辑期间删种
	rv := h.resourceResolver.ResolveResource(r.Context(), infoHash)
	if rv == nil || rv.Meta == nil {
		Error(w, http.StatusNotFound, 40401, "未找到 metadata")
		return
	}
	meta := h.selectSourceMeta(rv.AllMetas)
	if meta == nil {
		meta = rv.Meta
	}
	// 后续更新定位用资源视图的元数据行 hash（数据在谁名下就更新谁）
	infoHash = meta.InfoHash

	// §59.47: screenshots 列统一解析（JSON 数组优先/换行回退）——
	// 原 strings.Split("\n") 对 JSON 格式列拆出整个 JSON 串当 URL（1 张损坏图）
	screenshots := model.ParseScreenshotColumn(meta.Screenshots)

	// §59.26: BuildTechProfile 三源合并（标题 + MediaInfo + DOM），5 标题字段始终用 profile，
	// 14 平铺字段 DB 为空时 fallback 到 profile（兼容历史数据）
	// MI 优先 local（MediaInfo），fallback 源站（SourceMediaInfo）
	miForProfile := meta.MediaInfo
	if miForProfile == "" {
		miForProfile = meta.SourceMediaInfo
	}
	// §59.28 D1: DOM 源接入（与 fetchSingleTorrent 同一套三源合并）
	domMedium, domRes, domVideo, domAudio := domFieldsFromDetailSource(meta.DetailSourceJSON)
	profile := titleparser.BuildTechProfile(meta.Title, miForProfile, domMedium, domRes, domVideo, domAudio)
	components := titleparser.TechProfileToComponents(profile)
	inferredCategory := titleparser.InferCategory(components, meta.SourceCategory, "", "")

	// §59.34: Encode 派生（基于最终展示组合值：DB 优先 fallback profile）
	displayProfile := titleparser.TechProfile{
		SourceType:    pickNonEmpty(meta.SourceType, profile.SourceType),
		Specification: pickNonEmpty(meta.Specification, profile.Specification),
		VideoCodec:    pickNonEmpty(meta.VideoCodec, profile.VideoCodec),
	}
	// §59.151: MI 铁证传递（displayProfile 原列值构造丢 MIEncoded——
	// WEB-DL 剧集 Encode 误判根因：IsEncode 走不进 MI 分支回落旧 spec 判定）
	displayProfile.MIEncoded = profile.MIEncoded
	displayProfile.MIHasVideo = profile.MIHasVideo

	// §59.46: 展示优先级兜底——description 列为空（源站无简介形态）时按
	// douban_url 查 ptgen_cache（format BBCode）；非空不覆盖（尊重源站/用户编辑）
	descriptionOut := meta.Description
	if descriptionOut == "" && meta.DoubanURL != "" {
		var cached model.PTGenCache
		if err := h.db.WithContext(r.Context()).
			Where("query_key = ?", meta.DoubanURL).
			Order("updated_at DESC").First(&cached).Error; err == nil {
			var pr struct {
				RawBBCode string `json:"raw_bbcode"`
			}
			if json.Unmarshal([]byte(cached.JSONData), &pr) == nil && pr.RawBBCode != "" {
				descriptionOut = pr.RawBBCode
			}
		}
	}

	result := map[string]interface{}{
		"info_hash":       meta.InfoHash,
		"site_name":       meta.SiteName,
		"title":           meta.Title,
		"subtitle":        meta.Subtitle,
		"poster":          meta.Poster,
		"description":     descriptionOut,
		"screenshots":     screenshots,
		"mediainfo":       miForProfile,
		"bdinfo":          meta.BDInfo,
		"statement":       meta.Statement,
		"imdb_url":        meta.IMDbURL,
		"douban_url":      meta.DoubanURL,
		"tmdb_url":        meta.TMDbURL,
		"flags":           meta.Flags,
		"source_category": meta.SourceCategory,
		"reviewed":        meta.Reviewed,
		"fetched_at":      meta.FetchedAt,
		"fetch_source":    meta.FetchSource,

		// 14 DB 平铺字段（DB 为空 → profile fallback）
		"category":        pickNonEmpty(meta.Category, inferredCategory),
		"form":            meta.Form,
		// §59.108: 编辑表单媒介输入框数据源（titleComponents.medium 曾恒空——
		// GET 无 medium 键, source_type+specification 合成 TitleComponents.Medium 形态）
		"medium": func() string {
			m := pickNonEmpty(meta.SourceType, profile.SourceType)
			s := pickNonEmpty(meta.Specification, profile.Specification)
			switch {
			case m != "" && s != "":
				return m + " " + s
			case m != "":
				return m
			default:
				return s
			}
		}(),
		"resolution":      pickNonEmpty(meta.Resolution, profile.Resolution),
		"video_codec":     pickNonEmpty(meta.VideoCodec, profile.VideoCodec),
		"audio_codec":     pickNonEmpty(meta.AudioCodec, profile.AudioCodec),
		"audio_channels":  pickNonEmpty(meta.AudioChannels, profile.AudioChannels),
		"audio_tech":      pickNonEmpty(meta.AudioTech, profile.AudioTechnology),
		// §59.116: 展示层零计算回归——列值唯一真相（计算只在 t0 落库层，有完整
		// titleparser 管道与段上下文）；fallback 只裸 profile 补位（列 0=异常诚实显示，
		// 刷列治本）。§59.115 曾在 fallback 加扣减——展示层第二套计算是分裂之源。
		"audio_tracks":    pickNonZero(meta.AudioTracks, profile.AudioTracks),
		"hdr":             pickNonEmpty(meta.HDR, profile.HDR),
		"bit_depth":       pickNonEmpty(meta.BitDepth, profile.BitDepth),
		"source_type":     displayProfile.SourceType,
		"specification":   displayProfile.Specification,
		// §59.34: Encode 派生标识（前端规格展示用，不参与重组）
		"encode":          titleparser.IsEncode(displayProfile),
		"source_platform": pickNonEmpty(meta.SourcePlatform, profile.SourcePlatform),
		"edition_info":    pickNonEmpty(meta.EditionInfo, profile.EditionInfo),
		"region_code":     pickNonEmpty(meta.RegionCode, profile.RegionCode),

		// 5 标题解析字段（chinese_prefix fallback 到副标题【中文名】）
		"main_title":     profile.MainTitle,
		"season_episode": profile.SeasonEpisode,
		"year":           profile.Year,
		"release_group":  profile.ReleaseGroup,
		"chinese_prefix": pickNonEmpty(profile.ChinesePrefix, extractChineseFromSubtitle(meta.Subtitle)),

		// 状态
		"missing_fields": h.checkRequiredFields(meta),
	}

	// §59.75: 产地/类型（PTGen 源归一——region.us/genre.drama + label 双形态，
	// 前端 Tab1 只读展示，发布映射消费 canonical）
	if src, err := metadata.UnmarshalPTGenSource(meta.PTGenSourceJSON); err == nil && src != nil {
		result["region"] = normalizeDomainValues("region", src.Region)
		result["genre"] = normalizeDomainValues("genre", src.Genre)
	}

	// §59.26: 返回 tags——双形态（§59.106: keys=canonical 供发布映射,
	// labels=dict 显示名——前端已选区不依赖 bundle 内 dict 新旧, 新词条上线
	// 旧缓存页面也不显示代码）
	if meta.Tags != "" {
		var tags []string
		if err := json.Unmarshal([]byte(meta.Tags), &tags); err == nil && len(tags) > 0 {
			result["tags"] = tags
			result["tag_labels"] = tagDisplayNames(tags)
		}
	}
	if result["tags"] == nil {
		result["tags"] = []string{}
	}

	// §59.21: 返回 is_local（从 client_id 查询）
	clientID := r.URL.Query().Get("client_id")
	if clientID != "" {
		var client model.ClientConfig
		if err := h.db.WithContext(r.Context()).Where("name = ?", clientID).First(&client).Error; err == nil {
			result["is_local"] = client.IsLocal
		}
	}

	Success(w, result)
}

// putSeedRequest §59.89: PUT /publish/seeds 请求体（提升命名——buildPutSeedUpdates 测试锚）。
type putSeedRequest struct {
	Poster       string   `json:"poster"`
	Screenshots  []string `json:"screenshots"`
	Description  string   `json:"description"`
	Tags         []string `json:"tags"`
	SiteName     string   `json:"siteName"`
}

// buildPutSeedUpdates §59.89: PUT 更新构造（空值不覆盖）——部分保存场景（只改
// tags 不带 description）不得清空简介/海报/截图；v0.0.738 验证脚本 PUT {} 污染
// 预览实锤的根因修复。清空语义: 显式传空串 poster=""/description="" 视为清空
// 仍生效（需求方主动清空 vs 漏传的区分——JSON 里 nil 与 "" 无法区分，取保守:
// 一律不覆盖空值；主动清空走"重新获取"链）。
func buildPutSeedUpdates(req putSeedRequest, existingPoster, existingDesc, existingShots string) map[string]interface{} {
	updates := map[string]interface{}{}
	if req.Poster != "" {
		updates["poster"] = req.Poster
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Screenshots != nil {
		updates["screenshots"] = model.FormatScreenshotColumn(req.Screenshots)
	}
	if req.Tags != nil {
		if data, err := json.Marshal(req.Tags); err == nil {
			updates["tags"] = string(data)
		}
	}
	return updates
}

// handlePutSeed §59.20: 保存种子配置（PUT /publish/seeds/:info_hash）。
// 写入编辑字段（poster/screenshots/description）→ 9 字段校验 → Reviewed → 预览渲染。
func (h *PublishTorrentsHandler) handlePutSeed(w http.ResponseWriter, r *http.Request) {
	infoHash := extractSeedHash(r)
	if infoHash == "" || infoHash == "seeds" {
		Error(w, http.StatusBadRequest, 40001, "缺少 info_hash")
		return
	}

	var req putSeedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}

	// 找到目标行
	// §59.44: 资源视图解析——前端传的 hash 可能是列表保留行（无 metadata 挂靠），
	// 按 (client,path,name) 圈 hash 后用挂载行 hash 定位更新目标
	if rv := h.resourceResolver.ResolveResource(r.Context(), infoHash); rv != nil && rv.Meta != nil {
		infoHash = rv.Meta.InfoHash
	}
	query := h.db.WithContext(r.Context()).Where("info_hash = ?", infoHash)
	if req.SiteName != "" {
		query = query.Where("site_name = ?", req.SiteName)
	}
	var metas []model.TorrentMetadata
	query.Find(&metas)
	if len(metas) == 0 {
		Error(w, http.StatusNotFound, 40401, "未找到 metadata")
		return
	}
	meta := h.selectSourceMeta(metas)
	if meta == nil {
		meta = &metas[0]
	}
	existingShots := meta.Screenshots

	// 写入编辑字段
	updates := buildPutSeedUpdates(req, meta.Poster, meta.Description, meta.Screenshots)

	// 9 字段校验 → Reviewed（有效值合成——空请求不把已有值算成缺失）
	meta.Poster = pickNonEmpty(req.Poster, meta.Poster)
	meta.Screenshots = model.FormatScreenshotColumn(req.Screenshots)
	if req.Screenshots == nil {
		meta.Screenshots = existingShots
	}
	meta.Description = pickNonEmpty(req.Description, meta.Description)
	missing := h.checkRequiredFields(meta)
	updates["reviewed"] = len(missing) == 0
	// §59.93/94: 审核状态簇同步（公共方法——含取消）
	defer h.syncClusterReviewedByHashes(context.Background(), []string{infoHash}, len(missing) == 0)

	h.db.WithContext(r.Context()).Model(&model.TorrentMetadata{}).
		Where("id = ?", meta.ID).
		Updates(updates)

	// 重新查询获取更新后的数据（§59.28 C：并发删除时返回明确错误而非零值）
	var updated model.TorrentMetadata
	if err := h.db.WithContext(r.Context()).Where("id = ?", meta.ID).First(&updated).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50001, "保存后读取失败: "+err.Error())
		return
	}

	// §59.104: description 展示兜底（与 GET §59.46 同款）——列空按 douban_url 查
	// ptgen_cache（format BBCode）；非空不覆盖
	descriptionOut := updated.Description
	if descriptionOut == "" && updated.DoubanURL != "" {
		var cached model.PTGenCache
		if err := h.db.WithContext(r.Context()).
			Where("query_key = ?", updated.DoubanURL).
			Order("updated_at DESC").First(&cached).Error; err == nil {
			var pr struct {
				RawBBCode string `json:"raw_bbcode"`
			}
			if json.Unmarshal([]byte(cached.JSONData), &pr) == nil && pr.RawBBCode != "" {
				descriptionOut = pr.RawBBCode
			}
		}
	}

	// §59.28 C（方案A ②）：ReassembleFromTechProfile 标准化重组标题（预览用，不落库——
	// 发布时按目标站 title_format 重组，这里用 v1.05 默认模板展示标准化效果）
	// §59.28 C（方案A ④）：Renderer.Render 生成完整描述（声明+致谢+海报+正文+截图）
	miForProfile := updated.MediaInfo
	if miForProfile == "" {
		miForProfile = updated.SourceMediaInfo
	}
	domMedium, domRes, domVideo, domAudio := domFieldsFromDetailSource(updated.DetailSourceJSON)
	profile := titleparser.BuildTechProfile(updated.Title, miForProfile, domMedium, domRes, domVideo, domAudio)
	reassembledTitle := titleparser.ReassembleFromTechProfile(profile, titleparser.V105TitleFormat())

	renderer := description.NewRenderer("")
	screenshots := model.ParseScreenshotColumn(updated.Screenshots) // §59.47
	descData := &model.DescriptionData{
		Statement:      updated.Statement,
		PosterURL:      updated.Poster,
		PTGenBody:      updated.Description,
		MediaInfoText:  miForProfile,
		BDInfoText:     updated.BDInfo,
		Screenshots:    screenshots,
		SourceSite:     updated.SiteName,
		Title:          updated.Title,
	}
	renderedDesc, renderErr := renderer.Render(descData, model.SiteDescConfig{})

	result := map[string]interface{}{
		"info_hash":    updated.InfoHash,
		"site_name":    updated.SiteName,
		"title":        updated.Title,
		"subtitle":     updated.Subtitle,
		"poster":       updated.Poster,
		"description":  descriptionOut, // §59.104: GET 同款兜底（列空按 douban_url 查 ptgen_cache——预览不漏缓存简介）
		"statement":    updated.Statement,
		"tags": func() interface{} {
			var tags []string
			if updated.Tags != "" {
				if err := json.Unmarshal([]byte(updated.Tags), &tags); err != nil {
					return []string{}
				}
			}
			return tags
		}(),
		"tag_labels": func() []string {
			// §59.110: 预览④与 Tab1 同源显示名（§59.106 双形态）
			var tags []string
			if updated.Tags != "" {
				if err := json.Unmarshal([]byte(updated.Tags), &tags); err != nil {
					return []string{}
				}
				return tagDisplayNames(tags)
			}
			return []string{}
		}(),
		"reviewed":     updated.Reviewed,
		"missing_fields": missing,

		// §59.103: v1.05 全字段——与 GET detail（Tab1 数据源）同款取值策略：
		// 列值优先（pickNonEmpty/pickNonZero），列空才 profile 现算补。预览=引用
		// Tab1 数据，不再独立计算（§59.102 音轨数分裂的结构性根除——两响应同源）。
		"season_episode": profile.SeasonEpisode, // transient（不落列，两处同源现算）
		"year":           profile.Year,          // transient
		"resolution":     pickNonEmpty(updated.Resolution, profile.Resolution),
		"hdr":            pickNonEmpty(updated.HDR, profile.HDR),
		"bit_depth":      pickNonEmpty(updated.BitDepth, profile.BitDepth),
		"video_codec":    pickNonEmpty(updated.VideoCodec, profile.VideoCodec),
		"audio_codec":    pickNonEmpty(updated.AudioCodec, profile.AudioCodec),
		"audio_channels": pickNonEmpty(updated.AudioChannels, profile.AudioChannels),
		"audio_tech":     pickNonEmpty(updated.AudioTech, profile.AudioTechnology),
		"audio_tracks":   pickNonZero(updated.AudioTracks, profile.AudioTracks), // §59.116: 展示层零计算（同 GET）
		"source_type":    pickNonEmpty(updated.SourceType, profile.SourceType),
		"specification":  pickNonEmpty(updated.Specification, profile.Specification),
		"source_platform": pickNonEmpty(updated.SourcePlatform, profile.SourcePlatform),
		"edition_info":   pickNonEmpty(updated.EditionInfo, profile.EditionInfo),
		"region_code":    pickNonEmpty(updated.RegionCode, profile.RegionCode),
		"chinese_prefix": pickNonEmpty(profile.ChinesePrefix, extractChineseFromSubtitle(updated.Subtitle)),
		"encode":         titleparser.IsEncode(profile),
		// §59.90: 对齐 Tab1——剧名/制作组/类型(InferCategory)
		"main_title":     profile.MainTitle,
		"release_group":  profile.ReleaseGroup,
		"category": func() string {
			comps := titleparser.TechProfileToComponents(profile)
			return titleparser.InferCategory(comps, updated.SourceCategory, "", "")
		}(),

		// §59.28 C（方案A ②④）：标准化重组标题 + 渲染后完整描述（预览）
		"reassembled_title": reassembledTitle,
		"rendered_description": renderedDesc,
	}
	// §59.81: 产地/类型 + 分段渲染素材（简介四段结构化展示）
	if src, err := metadata.UnmarshalPTGenSource(updated.PTGenSourceJSON); err == nil && src != nil {
		result["region"] = normalizeDomainValues("region", src.Region)
		result["genre"] = normalizeDomainValues("genre", src.Genre)
	}
	if renderErr != nil {
		result["render_error"] = renderErr.Error()
	}
	Success(w, result)
}

// handleFetchSingleSeed §59.26: 单种子获取（复用 fetchSingleTorrent + is_local 支持）
func (h *PublishTorrentsHandler) handleFetchSingleSeed(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimRight(r.URL.Path, "/")
	infoHash := strings.TrimSuffix(path, "/fetch")
	parts := strings.Split(infoHash, "/")
	if len(parts) > 0 {
		infoHash = parts[len(parts)-1]
	}
	if infoHash == "" || infoHash == "seeds" {
		Error(w, http.StatusBadRequest, 40001, "缺少 info_hash")
		return
	}
	clientID := r.URL.Query().Get("client_id")
	if clientID == "" {
		Error(w, http.StatusBadRequest, 40001, "client_id 为必填")
		return
	}

	var snap model.TorrentSnapshot
	if err := h.db.WithContext(r.Context()).Where("hash = ? AND client_id = ?", infoHash, clientID).First(&snap).Error; err != nil {
		Error(w, http.StatusNotFound, 40401, "快照中未找到该种子")
		return
	}

	isLocal := false
	var client model.ClientConfig
	if h.db.WithContext(r.Context()).Where("name = ?", clientID).First(&client).Error == nil {
		isLocal = client.IsLocal
	}

	// §59.28 nil 守卫（与 handleBatchFetch 对齐）
	if h.sourceDetector == nil || h.metadataFetcher == nil {
		Error(w, http.StatusInternalServerError, 50002, "服务未就绪（detector/fetcher 未注入）")
		return
	}

	if err := h.fetchSingleTorrent(r.Context(), clientID, infoHash, snap.Name, snap.Size, snap.SavePath, isLocal); err != nil {
		Error(w, http.StatusInternalServerError, 50000, fmt.Sprintf("获取失败: %v", err))
		return
	}

	// §59.26: 获取（含重新获取）后 reviewed=false，必须重新走预览审核
	// §59.44: 资源视图圈 hash——获取可能落在资源键内其他 hash（tid 反查），全组重置
	resetHashes := []string{infoHash}
	if rv := h.resourceResolver.ResolveResource(r.Context(), infoHash); rv != nil && len(rv.Hashes) > 0 {
		resetHashes = rv.Hashes
	}
	h.db.WithContext(r.Context()).Model(&model.TorrentMetadata{}).
		Where("info_hash IN ?", resetHashes).
		Update("reviewed", false)

	Success(w, map[string]interface{}{"message": "获取成功"})
}

// handleDeleteSeed §59.26 §59.32: 清除种子 metadata（资源级——删同名全部 hash）。
// coverage 不清除。多站聚合下载器同资源多 hash 变体，metadata 只挂其一；
// "未配置"判定是资源级（任一 hash 有 = 已配置），故清除也必须资源级，
// 否则资源永远无法回到未获取态（20 清除 → 12 未配置 的根因）。
func (h *PublishTorrentsHandler) handleDeleteSeed(w http.ResponseWriter, r *http.Request) {
	infoHash := extractSeedHash(r)
	if infoHash == "" || infoHash == "seeds" {
		Error(w, http.StatusBadRequest, 40001, "缺少 info_hash")
		return
	}

	// 查该 hash 的 name → 同名全部 hash
	var name string
	h.db.WithContext(r.Context()).
		Table("torrent_snapshots").
		Select("name").
		Where("hash = ? AND name != ''", infoHash).
		Limit(1).
		Row().Scan(&name)

	result := h.db.WithContext(r.Context()).Where("info_hash = ?", infoHash).Delete(&model.TorrentMetadata{})
	if name != "" {
		// 资源级：删同名全部 hash 的 metadata（含代表 hash 自身，幂等）
		var siblingHashes []string
		h.db.WithContext(r.Context()).
			Table("torrent_snapshots").
			Select("hash").
			Where("name = ?", name).
			Find(&siblingHashes)
		if len(siblingHashes) > 0 {
			result = h.db.WithContext(r.Context()).
				Where("info_hash IN ?", siblingHashes).
				Delete(&model.TorrentMetadata{})
		}
	}
	_ = result
	Success(w, map[string]interface{}{"message": "已清除"})
}

// tagDisplayNames §59.106: tag canonical 键 → dict 显示名列表（miss 保留原文）。
func tagDisplayNames(tags []string) []string {
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		if l := titleparser.ReverseLookup("tag." + t); l != "" {
			out = append(out, l)
		} else {
			out = append(out, t)
		}
	}
	return out
}

// normalizeDomainValues §59.75: 域值归一（PTGen 原词→canonical）+ label 映射。
// 返回 {"keys": ["region.us"], "labels": ["美国"]} 双形态——Tab1 显示 label，
// 发布映射消费 keys。miss（未收录国名/类型词）保留原文双形态。
func normalizeDomainValues(domain string, raws []string) map[string][]string {
	if len(raws) == 0 {
		return nil
	}
	keys, labels := make([]string, 0, len(raws)), make([]string, 0, len(raws))
	seen := map[string]bool{}
	for _, r := range raws {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		key := titleparser.LookupDictKey(domain, r)
		if key == "" {
			key = r // miss 保留原文
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		keys = append(keys, key)
		if label := titleparser.ReverseLookup(key); label != "" {
			labels = append(labels, label)
		} else {
			labels = append(labels, r)
		}
	}
	if len(keys) == 0 {
		return nil
	}
	return map[string][]string{"keys": keys, "labels": labels}
}

// persistPTGenSource §59.75: PTGen 源结构化持久化（ptgen_source_json）。
// Region/Genre（产地/类型）此前只在 querier 闭包里被取走 RawBBCode/PosterURL 后
// 丢弃——接入遗漏修复（列/Marshal/Unmarshal 机制 §56 时代已有）。
// Tab1 产地/类型展示与未来发布站点映射消费此列。
func (h *PublishTorrentsHandler) persistPTGenSource(ctx context.Context, infoHash, siteName string, r *model.PTGenResult) {
	if r == nil {
		return
	}
	data, err := json.Marshal(metadata.PTGenToSource(*r, time.Now()))
	if err != nil {
		return
	}
	if err := h.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
		Where("info_hash = ? AND site_name = ?", infoHash, siteName).
		Update("ptgen_source_json", string(data)).Error; err != nil {
		h.logger.Warn("ptgen source persist failed", zap.Error(err))
	}
}

// refreshInferredTags §59.70: t2 重推标签——PTGen 简介落库后评分行才存在
//（t0 InferFull 时 Description 尚无 "◎豆瓣评分" 行），此处重跑推断并合并
//（既有标签全保留——直采/用户标签优先，推断只补差）。
func (h *PublishTorrentsHandler) refreshInferredTags(ctx context.Context, infoHash, siteName string) {
	var m model.TorrentMetadata
	if err := h.db.WithContext(ctx).
		Where("info_hash = ? AND site_name = ?", infoHash, siteName).
		First(&m).Error; err != nil {
		return
	}
	// §59.72 B2: big_pack 需 size——从快照查（簇内同 size）
	var snapSize int64
	h.db.WithContext(ctx).Model(&model.TorrentSnapshot{}).
		Where("hash = ?", infoHash).Select("COALESCE(MAX(size),0)").Scan(&snapSize)
	inferred := publish.NewMediaTagInferer().InferFull(publish.TagInput{
		MediaInfo:   m.MediaInfo,
		Title:       m.Title,
		Subtitle:    m.Subtitle,
		Description: m.Description,
		NFO:         m.BDInfo,
		Size:        snapSize,
	})
	var existing []string
	if m.Tags != "" {
		if err := json.Unmarshal([]byte(m.Tags), &existing); err != nil {
			existing = nil
		}
	}
	// §59.74: MergeTags 单点——归一+直采优先+互斥/覆盖仲裁；无变化不写
	all := publish.MergeTags(existing, inferred)
	data, _ := json.Marshal(all)
	if string(data) == m.Tags {
		return
	}
	if err := h.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
		Where("info_hash = ? AND site_name = ?", infoHash, siteName).
		Update("tags", string(data)).Error; err != nil {
		h.logger.Warn("refresh inferred tags failed", zap.Error(err))
		return
	}
	h.logger.Info("inferred tags refreshed",
		zap.String("hash", infoHash[:min(10, len(infoHash))]),
		zap.Int("tags_total", len(existing)))
}

// applyPosterFallback §59.42: 海报替换链落库（goroutine 内执行）。
// 优先用已落库的 douban_url 作 query（精确），无则种子名；两级 PTGen + HEAD 探活。
func (h *PublishTorrentsHandler) applyPosterFallback(infoHash, siteName, sitePoster, name string) {
	if h.ptgen == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	query := name
	var m model.TorrentMetadata
	if err := h.db.WithContext(ctx).
		Where("info_hash = ? AND site_name = ?", infoHash, siteName).
		First(&m).Error; err == nil && m.DoubanURL != "" {
		query = m.DoubanURL
	}

	// §59.55: 双字段消费——querier 捕获完整 PTGenResult（PosterURL + RawBBCode），
	// 海报走 RunPosterFallback 语义不变；description 增量写（format 非空才覆盖）
	ptgenDesc := ""
	var queriers []publish.PTGenQuerier
	var ptgenResult *model.PTGenResult
	queriers = append(queriers, func(ctx context.Context, q string) (string, error) {
		r, err := h.ptgen.AnalyzePTGen(ctx, q)
		if err != nil || r == nil {
			return "", err
		}
		if r.RawBBCode != "" {
			ptgenDesc = r.RawBBCode
			ptgenResult = r // §59.75: 捕获完整 result——产地/类型结构化落库
		}
		return r.PosterURL, nil
	})
	res := publish.RunPosterFallback(ctx, sitePoster, query, queriers)

	// §59.55: description 增量写——PTGen format 非空即覆盖（PTGen 为准落库），
	// 失败/空不动（kdouban/descr 回退保留）。与海报分支结果无关（PTGen 可能
	// 成功返回了 format 但 PosterURL 恰为空/非白名单直信）。
	if ptgenDesc != "" {
		h.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
			Where("info_hash = ? AND site_name = ?", infoHash, siteName).
			Update("description", ptgenDesc)
		h.logger.Info("ptgen description applied",
			zap.String("hash", infoHash[:10]),
			zap.Int("length", len(ptgenDesc)))
		// §59.75: PTGen 源结构化持久化（region/genre 系统资产）
		h.persistPTGenSource(ctx, infoHash, siteName, ptgenResult)
		// §59.70: t2 重推标签——评分行此刻才进 Description（豆瓣评分≥8 → high_rating）
		h.refreshInferredTags(ctx, infoHash, siteName)
		// §59.61 附2: 简介终态同步回传簇（map miss 反查; 幂等可重复）
		if c, ok := h.clusterCtxFor(ctx, infoHash); ok {
			h.propagateClusterPosters(ctx, c.clientID, c.savePath, c.name, infoHash)
		}
	}

	if res.Source == "site" {
		return // 可信原图，无需更新（description 已在上面处理）
	}
	// §59.49: ptgen_dead（两级 PTGen 全失败且原站 URL 不可信）时探活原 URL——
	// 死链清空（poster=""，字段列诚实显红叉）；活链保留（下次重取再试 PTGen 替换）。
	// 原 URL 进日志可追溯。
	if res.Source == "ptgen_dead" {
		if publish.CheckPosterAlive(ctx, sitePoster) {
			h.logger.Info("poster fallback: ptgen dead but site URL alive, keeping",
				zap.String("hash", infoHash[:10]),
				zap.String("poster", sitePoster[:min(60, len(sitePoster))]))
			return
		}
		h.logger.Warn("poster fallback: dead link purged",
			zap.String("hash", infoHash[:10]),
			zap.String("dead_url", sitePoster[:min(80, len(sitePoster))]))
		h.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
			Where("info_hash = ? AND site_name = ?", infoHash, siteName).
			Update("poster", "")
		return
	}
	h.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
		Where("info_hash = ? AND site_name = ?", infoHash, siteName).
		Updates(map[string]interface{}{
			"poster": res.Poster,
		})
	// §59.61 附2: PTGen 终态回传簇（map miss 反查 snapshots——4005 批次实锤修复）
	if c, ok := h.clusterCtxFor(ctx, infoHash); ok {
		h.propagateClusterPosters(ctx, c.clientID, c.savePath, c.name, infoHash)
	}
	h.logger.Info("poster fallback applied",
		zap.String("hash", infoHash[:10]),
		zap.String("source", res.Source),
		zap.String("original", res.Original[:min(60, len(res.Original))]),
		zap.String("final", res.Poster[:min(60, len(res.Poster))]))
}

// purgeDeadScreenshots §59.49: 截图获取时探活——HEAD（含 1 次重试）逐张检查，
// 清死留活；全死全清（screenshots=[]）。只反映获取时点存活状态。
func (h *PublishTorrentsHandler) purgeDeadScreenshots(infoHash, siteName string) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	var meta model.TorrentMetadata
	if err := h.db.WithContext(ctx).
		Where("info_hash = ? AND site_name = ?", infoHash, siteName).
		First(&meta).Error; err != nil {
		return
	}
	urls := model.ParseScreenshotColumn(meta.Screenshots)
	if len(urls) == 0 {
		return
	}

	// 并发探活（每张 HEAD + 失败重试 1 次）
	type result struct {
		idx   int
		alive bool
	}
	ch := make(chan result, len(urls))
	sem := make(chan struct{}, 6) // 并发上限 6
	for i, u := range urls {
		go func(idx int, u string) {
			sem <- struct{}{}
			defer func() { <-sem }()
			ch <- result{idx, publish.CheckPosterAlive(ctx, u)}
		}(i, u)
	}
	alive := make([]bool, len(urls))
	for range urls {
		r := <-ch
		alive[r.idx] = r.alive
	}

	var kept []string
	var dead []string
	for i, u := range urls {
		if alive[i] {
			kept = append(kept, u)
		} else {
			dead = append(dead, u)
		}
	}
	if len(dead) == 0 {
		return // 全活，无需更新
	}
	for _, d := range dead {
		h.logger.Warn("screenshot dead link purged",
			zap.String("hash", infoHash[:10]),
			zap.String("dead_url", d[:min(80, len(d))]))
	}
	data, _ := json.Marshal(kept) // 全死时 kept=nil → "null" 需转 "[]"
	if kept == nil {
		data = []byte("[]")
	}
	h.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
		Where("info_hash = ? AND site_name = ?", infoHash, siteName).
		Update("screenshots", string(data))
	h.logger.Info("screenshot purge completed",
		zap.String("hash", infoHash[:10]),
		zap.Int("total", len(urls)),
		zap.Int("kept", len(kept)),
		zap.Int("purged", len(dead)))
}

// applyScreenshotStrategy §59.53: 采集链截图策略异步执行（goroutine 内）。
// §59.57 竞态修复: purgeDeadScreenshots 内联前序（串行）——原双 goroutine 并发时
// 本函数可能读到 purge 前的死链列表，rehost 失败保源后 final==source → same 早退
// → 永不触发 mpv 补图（243 实测 8 组 ptpimg.me 全死链复现）。探活先行，本函数
// 读到的必为活链集，再走策略（白名单/转存/差额补足/无图全量）→ 落库。
func (h *PublishTorrentsHandler) applyScreenshotStrategy(clientID, infoHash, siteName, name, savePath string, isLocal bool) {
	// §59.58: 并发额度闸门——批量链 N 路 fire-and-forget 无界并发挤爆 CPU/代理（243 实测
	// >20 路时 mpv 摊薄 15 倍 → 撞 4min ctx → 差额补足作废）。排队在闸门外等，不烧 ctx 预算
	// （ctx 在获得额度后才创建）。CPU 总量守恒：总时长不变，换来每单稳定完成。
	sem := h.strategySem
	if sem == nil {
		sem = make(chan struct{}, 5) // 兜底（零值 handler，如测试构造）
	}
	sem <- struct{}{}
	defer func() { <-sem }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// §59.63: 截图链接缓存（观察期）——簇键命中且未过期直接复用，跳过探活/转存/
	// mpv/上传全链（Q3 缓存优先：本批源站截图不消费）。锚点=最近一次成功写穿，
	// 手动捕获结果也刷新锚点（Q4）。过期=miss 走既有策略（Q2=A：源站充足仍转存
	// 源站，不足才 mpv——与 §59.53 语义一致）。
	if cached, ok := h.lookupScreenshotCache(clientID, savePath, name); ok {
		data, _ := json.Marshal(cached)
		if err := h.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
			Where("info_hash = ? AND site_name = ?", infoHash, siteName).
			Update("screenshots", string(data)).Error; err == nil {
			h.propagateClusterScreenshots(ctx, clientID, savePath, name, infoHash, string(data))
			h.logger.Info("screenshot cache hit",
				zap.String("hash", infoHash[:min(10, len(infoHash))]),
				zap.Int("shots", len(cached)))
		}
		return
	}

	// §59.57: 探活内联前序（自带 90s 独立 ctx，读自身快照；HEAD 秒级不占策略预算）
	h.purgeDeadScreenshots(infoHash, siteName)

	strategyCtx, scancel := context.WithTimeout(ctx, 4*time.Minute)
	defer scancel()

	var meta model.TorrentMetadata
	if err := h.db.WithContext(ctx).
		Where("info_hash = ? AND site_name = ?", infoHash, siteName).
		First(&meta).Error; err != nil {
		return
	}
	source := model.ParseScreenshotColumn(meta.Screenshots)
	final := h.shotStrategy.ApplyScreenshotStrategy(strategyCtx, name, savePath, source, isLocal)
	if len(final) == len(source) {
		// 无变化（无图且截图失败/全白名单保留/远程无图）——不覆盖
		same := true
		for i := range final {
			if final[i] != source[i] {
				same = false
				break
			}
		}
		if same {
			// §59.61 附4: 全失败可见性——source=0 且 final=0 是捕获/上传全灭
			// （243 实测: pixhost 早高峰 16 簇三层重试全打穿后静默退出，行永久
			// 空 12.5h 无任何日志）。打点 warn 让停摆可观测，人工/下轮重取补。
			if len(source) == 0 && isLocal {
				h.logger.Warn("screenshot strategy total failure",
					zap.String("hash", infoHash[:10]),
					zap.String("name", name))
			}
			return
		}
	}
	data, _ := json.Marshal(final)
	if final == nil {
		data = []byte("[]")
	}
	if err := h.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
		Where("info_hash = ? AND site_name = ?", infoHash, siteName).
		Update("screenshots", string(data)).Error; err != nil {
		h.logger.Error("screenshot strategy persist failed",
			zap.String("hash", infoHash[:10]), zap.Error(err))
		return
	}
	// §59.61 第 4 步: 截图二次传播（策略异步完成后补簇内空行——元数据传播时未就绪）
	h.propagateClusterScreenshots(ctx, clientID, savePath, name, infoHash, string(data))
	h.logger.Info("screenshot strategy applied",
		zap.String("hash", infoHash[:10]),
		zap.Bool("local", isLocal),
		zap.Int("source", len(source)),
		zap.Int("final", len(final)))

	// §59.63: 成功落库写穿缓存（final 非空才到达此处——same 早退/失败路径不写）
	upsertClusterScreenshotCache(h.db, h.logger, h.screenshotCacheDays, clientID, savePath, name, final)
}

// regionLabelsOfMeta §59.151: metadata 行 → PTGen 产地 labels 串（复合判据输入）。
func regionLabelsOfMeta(m model.TorrentMetadata) string {
	src, err := metadata.UnmarshalPTGenSource(m.PTGenSourceJSON)
	if err != nil || src == nil {
		return ""
	}
	return strings.Join(src.Region, " ")
}


// handleExecutePublish §59.156 切片 2: 新发布执行器入口（DB 供给——切读 publish_form_config）。
// body: {info_hash, target_site, anonymous, tag_overrides: [], dry_run}
func (h *PublishTorrentsHandler) handleExecutePublish(w http.ResponseWriter, r *http.Request) {
	var req struct {
		InfoHash     string   `json:"info_hash"`
		TargetSite   string   `json:"target_site"`
		Anonymous    bool     `json:"anonymous"`
		TagOverrides []string `json:"tag_overrides"`
		DryRun       bool     `json:"dry_run"`
		PushOnly     bool     `json:"push_only"`
		TorrentID    string   `json:"torrent_id"`
		PushClientID string   `json:"push_client_id"`
		PushSavePath string   `json:"push_save_path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.InfoHash == "" || req.TargetSite == "" {
		Error(w, http.StatusBadRequest, 40001, "info_hash 与 target_site 必填")
		return
	}
	if h.executor == nil {
		Error(w, http.StatusServiceUnavailable, 50301, "执行器未初始化")
		return
	}
	// §59.156 回归审核：发布长流程（渲染+预检+上传+加种）脱离 HTTP 请求生命周期——
	// §59.51 教训（243 实测上传 context canceled）；独立 Background+10min 上限
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	result := h.executor.Execute(ctx, publish.ExecuteInput{
		InfoHash:     req.InfoHash,
		TargetSite:   req.TargetSite,
		Anonymous:    req.Anonymous,
		TagOverrides: req.TagOverrides,
		DryRun:       req.DryRun,
		PushOnly:     req.PushOnly,
		TorrentID:    req.TorrentID,
		PushClientID: req.PushClientID,
		PushSavePath: req.PushSavePath,
	})
	// §59.158: Success 信封（与 form-config 同款教训——裸写致前端取值 undefined）
	Success(w, map[string]any{"result": result})
}


// handleExecutePublishBatch §59.159: 一种多站批量发布——每站独立 execute 并行
// （BatchGroupID 分组落库；网络白名单内并发安全——PTGen 等在线依赖已移除）。
func (h *PublishTorrentsHandler) handleExecutePublishBatch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		InfoHash     string   `json:"info_hash"`
		TargetSites  []string `json:"target_sites"`
		Anonymous    bool     `json:"anonymous"`
		TagOverrides []string `json:"tag_overrides"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.InfoHash == "" || len(req.TargetSites) == 0 {
		Error(w, http.StatusBadRequest, 40001, "info_hash 与 target_sites 必填")
		return
	}
	if h.executor == nil {
		Error(w, http.StatusServiceUnavailable, 50301, "执行器未初始化")
		return
	}
	batchID := fmt.Sprintf("%d", time.Now().UnixNano())
	type siteResult struct {
		Site    string                 `json:"site"`
		Status  string                 `json:"status"`
		Message string                 `json:"message"`
		TorrentID string               `json:"torrent_id,omitempty"`
		URL     string                 `json:"url,omitempty"`
		PreAudit *publish.PreAuditResult `json:"pre_audit,omitempty"`
	}
	results := make([]siteResult, len(req.TargetSites))
	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	for i, site := range req.TargetSites {
		wg.Add(1)
		go func(idx int, siteName string) {
			defer wg.Done()
			defer func() {
				if rec := recover(); rec != nil {
					results[idx] = siteResult{Site: siteName, Status: "failed", Message: fmt.Sprintf("panic: %v", rec)}
				}
			}()
			res := h.executor.Execute(ctx, publish.ExecuteInput{
				InfoHash:     req.InfoHash,
				TargetSite:   siteName,
				Anonymous:    req.Anonymous,
				TagOverrides: req.TagOverrides,
				BatchGroupID: batchID,
			})
			sr := siteResult{Site: siteName, Status: res.Status, Message: res.Message}
			if res.Upload != nil {
				sr.TorrentID = res.Upload.TorrentID
			}
			sr.URL = res.TargetTorrentURL
			sr.PreAudit = res.PreAudit
			results[idx] = sr
		}(i, site)
	}
	wg.Wait()
	Success(w, map[string]any{"batch_id": batchID, "results": results})
}

// handleExecuteSiteBatch §59.166 一站多种批量发布（N 种×1 站）：
// 串行+种间间隔（sites.publish_interval_seconds，clamp 1-60）——NP 站连续上传
// 反作弊风险，节奏拟人（用户定案）；单种失败不中断批次；统一 BatchGroupID。
// 任务化：立即返回 taskId，进度走 site-batch-progress 轮询（断线无伤）。
func (h *PublishTorrentsHandler) handleExecuteSiteBatch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		InfoHashes []string `json:"infoHashes"`
		TargetSite string   `json:"targetSite"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.InfoHashes) == 0 || req.TargetSite == "" {
		Error(w, http.StatusBadRequest, 40001, "infoHashes 与 targetSite 必填")
		return
	}
	if len(req.InfoHashes) > 100 {
		Error(w, http.StatusBadRequest, 40002, "单批最多 100 个种子（流控防护）")
		return
	}
	if h.executor == nil {
		Error(w, http.StatusServiceUnavailable, 50301, "执行器未初始化")
		return
	}

	// 目标站校验：存在 + 发布配置启用
	var site model.Site
	if err := h.db.WithContext(r.Context()).Where("name = ?", req.TargetSite).First(&site).Error; err != nil {
		Error(w, http.StatusBadRequest, 40003, "目标站不存在: "+req.TargetSite)
		return
	}
	cfg := model.ParseFormConfig(site.PublishFormConfig)
	if cfg == nil || !cfg.Enabled {
		Error(w, http.StatusBadRequest, 40004, "目标站未启用发布配置: "+req.TargetSite)
		return
	}
	interval := site.PublishIntervalSeconds
	if interval < 1 {
		interval = 1
	}
	if interval > 60 {
		interval = 60
	}

	// 同站互斥（运行中任务存在→拒）
	taskID := fmt.Sprintf("%d", time.Now().UnixNano())
	h.siteBatch.mu.Lock()
	if runID, running := h.siteBatch.active[req.TargetSite]; running {
		msg := "该站已有批量任务运行中，请等待完成"
		// §59.166 互斥提示带进度（用户定案 A 方案——信息透明）
		if t := h.siteBatch.tasks[runID]; t != nil {
			msg = fmt.Sprintf("该站已有批量任务运行中（%d/%d），请等待完成", t.Done, t.Total)
		}
		h.siteBatch.mu.Unlock()
		Error(w, http.StatusConflict, 40901, msg)
		return
	}
	task := &siteBatchTask{
		ID: taskID, TargetSite: req.TargetSite, Total: len(req.InfoHashes),
		Results: make([]siteBatchResult, 0, len(req.InfoHashes)),
		StartedAt: time.Now(),
	}
	h.siteBatch.tasks[taskID] = task
	h.siteBatch.active[req.TargetSite] = taskID
	h.siteBatch.mu.Unlock()

	// §59.51 长任务铁律：脱离 HTTP 生命周期；动态超时 total×(60s+间隔) 封顶 24h
	budget := time.Duration(len(req.InfoHashes)) * (60*time.Second + time.Duration(interval)*time.Second)
	if budget > 24*time.Hour {
		budget = 24 * time.Hour
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), budget)
		defer cancel()
		batchGroupID := taskID
		for i, hash := range req.InfoHashes {
			if i > 0 {
				select {
				case <-ctx.Done():
					h.finishSiteBatch(task, "任务超时中断（已完成 "+fmt.Sprint(task.Done)+"/"+fmt.Sprint(task.Total)+"）")
					return
				case <-time.After(time.Duration(interval) * time.Second):
				}
			}
			// 当前种标题（进度显示——DB 快查，失败留空）
			curTitle := ""
			var m model.TorrentMetadata
			if err := h.db.WithContext(ctx).Select("title").
				Where("info_hash = ?", hash).First(&m).Error; err == nil {
				curTitle = m.Title
			}
			h.siteBatch.mu.Lock()
			task.CurrentTitle = curTitle
			h.siteBatch.mu.Unlock()

			// §59.166 回归审核补：per-seed recover——executor panic 若穿透会把任务
			// goroutine 炸掉（finishSiteBatch 永不调用 → 同站互斥永久锁死直到重启）。
			// recover 后按单种失败继续（批量语义）。
			res := func() *publish.ExecuteResult {
				defer func() {
					if rec := recover(); rec != nil {
						h.logger.Error("site-batch executor panic",
							zap.String("site", req.TargetSite), zap.Any("panic", rec))
					}
				}()
				return h.executor.Execute(ctx, publish.ExecuteInput{
					InfoHash:     hash,
					TargetSite:   req.TargetSite,
					BatchGroupID: batchGroupID, // 匿名走站点级 form_config.Anonymous（§59.166 单源）
				})
			}()
			if res == nil {
				res = &publish.ExecuteResult{Status: "failed", Message: "执行器 panic（已隔离，继续后续种子）"}
			}
			sr := siteBatchResult{InfoHash: hash, Title: curTitle, Status: res.Status, Message: res.Message}
			if res.Upload != nil {
				sr.TorrentID = res.Upload.TorrentID
			}
			sr.URL = res.TargetTorrentURL
			h.siteBatch.mu.Lock()
			task.Results = append(task.Results, sr)
			task.Done++
			task.CurrentTitle = ""
			h.siteBatch.mu.Unlock()
		}
		h.finishSiteBatch(task, "")
	}()

	Success(w, map[string]any{"task_id": taskID, "total": len(req.InfoHashes), "interval_seconds": interval})
}

// finishSiteBatch 收尾（解锁同站互斥+终态时间戳）。
func (h *PublishTorrentsHandler) finishSiteBatch(task *siteBatchTask, errMsg string) {
	h.siteBatch.mu.Lock()
	defer h.siteBatch.mu.Unlock()
	task.Finished = true
	task.Error = errMsg
	task.FinishedAt = time.Now()
	if h.siteBatch.active[task.TargetSite] == task.ID {
		delete(h.siteBatch.active, task.TargetSite)
	}
}

// handleSiteBatchProgress §59.166 批量任务进度轮询：
// ?task_id= 指定任务；?site=X&active=1 查该站运行中任务（页面挂载恢复——断线无伤）。
// 惰性 TTL 清理：完成任务 30 分钟后移除（发布日志有全量，内存态只保近期）。
func (h *PublishTorrentsHandler) handleSiteBatchProgress(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	h.siteBatch.mu.Lock()
	defer h.siteBatch.mu.Unlock()

	// TTL 清理（30 分钟）
	now := time.Now()
	for id, t := range h.siteBatch.tasks {
		if t.Finished && now.Sub(t.FinishedAt) > 30*time.Minute {
			delete(h.siteBatch.tasks, id)
		}
	}

	if q.Get("active") == "1" {
		siteName := q.Get("site")
		if siteName != "" {
			// 指定站查询（原口径）
			if id, ok := h.siteBatch.active[siteName]; ok {
				Success(w, h.siteBatch.tasks[id])
				return
			}
			Success(w, nil) // 无运行中任务
			return
		}
		// §59.166 终稿回归审核补：不带 site → 全部活跃任务（选站不持久化的
		// 前提下，页面刷新后恢复进度需要无锚查询——active map 直接列出）
		actives := make([]*siteBatchTask, 0, len(h.siteBatch.active))
		for _, id := range h.siteBatch.active {
			if t, ok := h.siteBatch.tasks[id]; ok {
				actives = append(actives, t)
			}
		}
		sort.Slice(actives, func(i, j int) bool { return actives[i].StartedAt.Before(actives[j].StartedAt) })
		if len(actives) == 0 {
			Success(w, nil)
			return
		}
		Success(w, actives)
		return
	}
	id := q.Get("task_id")
	if t, ok := h.siteBatch.tasks[id]; ok {
		Success(w, t)
		return
	}
	Error(w, http.StatusNotFound, 40401, "任务不存在或已过期（30 分钟 TTL）")
}
