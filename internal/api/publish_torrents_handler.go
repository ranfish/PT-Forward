package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ranfish/pt-forward/internal/coverage"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/publish"
	"github.com/ranfish/pt-forward/internal/reseed"
	"github.com/ranfish/pt-forward/internal/site"
	"github.com/ranfish/pt-forward/internal/titleparser"
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
	db             *gorm.DB
	coverage       *coverage.Service
	clientMgr      MFClientProvider
	siteProvider   SiteProviderGetter
	sourceDetector *publish.SourceSiteDetector
	declFilter     *publish.DeclarationFilter
	reseedEngine   *reseed.Engine
	logger         *zap.Logger
	bgState        backgroundQueryState
}

type backgroundQueryState struct {
	mu          sync.Mutex
	active      map[uint]bool // clientID → querying
	total       int
	done        int
}

func NewPublishTorrentsHandler(db *gorm.DB, logger *zap.Logger) *PublishTorrentsHandler {
	return &PublishTorrentsHandler{
		db:      db,
		logger:  logger,
		bgState: backgroundQueryState{active: make(map[uint]bool)},
	}
}

func (h *PublishTorrentsHandler) SetCoverageService(s *coverage.Service) { h.coverage = s }
func (h *PublishTorrentsHandler) SetReseedEngine(e *reseed.Engine)     { h.reseedEngine = e }
func (h *PublishTorrentsHandler) SetClientProvider(c MFClientProvider)  { h.clientMgr = c }
func (h *PublishTorrentsHandler) SetSiteProvider(s SiteProviderGetter)  { h.siteProvider = s }
func (h *PublishTorrentsHandler) SetSourceDetector(d *publish.SourceSiteDetector) { h.sourceDetector = d }
func (h *PublishTorrentsHandler) SetDeclarationFilter(f *publish.DeclarationFilter) { h.declFilter = f }

func (h *PublishTorrentsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimRight(r.URL.Path, "/")

	switch {
	case strings.HasSuffix(path, "/publish/torrents") && r.Method == http.MethodGet:
		h.handleListTorrents(w, r)
	case strings.HasSuffix(path, "/publish/torrents/batch-publish") && r.Method == http.MethodPost:
		h.handleBatchPublish(w, r)
	case strings.HasSuffix(path, "/publish/torrents/coverage") && r.Method == http.MethodPost:
		h.handleQueryCoverage(w, r)
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

	items := make([]coverage.BatchItem, 0, len(torrents))
	for _, t := range torrents {
		if queried[t.Hash] {
			h.bgState.incDone()
			continue
		}

		// L0: 获取 trackers
		trackers, err := client.GetTrackers(ctx, t.Hash)
		if err != nil {
			trackers = nil
		}

		items = append(items, coverage.BatchItem{
			InfoHash:   t.Hash,
			Trackers:   trackers,
			TorrentDir: extractTorrentDir(cfg.Config),
		})
	}

	if len(items) == 0 {
		return
	}

	// 快车道：L0 + L1(缓存) + L2(IYUU 批量)
	queryCtx, queryCancel := context.WithTimeout(ctx, 5*time.Minute)
	defer queryCancel()

	err = h.coverage.QueryBatchCoverage(queryCtx, items)
	if err != nil {
		h.logger.Error("bg query: batch coverage failed", zap.Error(err))
	}

	// L1 fresh: 批量 pieces-hash 本地计算 + 站点 API 查询
	if h.siteProvider != nil {
		h.batchPiecesHashQuery(ctx, items, cfg)
	}

	h.bgState.setDone(len(torrents))
}

type coverageQueryRequest struct {
	ClientID  uint   `json:"client_id"`
	InfoHash  string `json:"info_hash"`
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

func (h *PublishTorrentsHandler) queryPiecesHashSites(ctx context.Context, infoHash, piecesHash string) {
	var sites []model.Site
	h.db.WithContext(ctx).
		Where("enabled = ? AND is_target = ?", true, true).
		Find(&sites)

	now := time.Now()
	ttl := now.Add(24 * time.Hour)

	for _, site := range sites {
		adapter, err := h.siteProvider.GetAdapter(ctx, site.Domain)
		if err != nil || adapter == nil {
			continue
		}
		if !adapter.SupportsSearchByPiecesHash() {
			continue
		}
		searcher, ok := adapter.(piecesHashSearcher)
		if !ok {
			continue
		}
		config, err := h.siteProvider.GetSiteConfig(ctx, site.Domain)
		if err != nil || config == nil {
			continue
		}

		result, err := searcher.SearchByPiecesHash(ctx, config, []string{piecesHash})
		if err != nil {
			h.logger.Debug("pieces_hash query failed", zap.String("site", site.Name), zap.Error(err))
			continue
		}

		if tid, found := result[piecesHash]; found {
			h.coverage.UpsertCoverage(ctx, &model.SiteCoverageCache{
				InfoHash:   infoHash,
				SiteName:   site.Name,
				Status:     model.CoverageConfirmedHas,
				Source:     model.CoverageSourcePiecesHash,
				Confidence: 1.0,
				TorrentID:  strconv.Itoa(tid),
				QueriedAt:  now,
				ExpiresAt:  ttl,
			})
		} else {
			h.coverage.UpsertCoverage(ctx, &model.SiteCoverageCache{
				InfoHash:   infoHash,
				SiteName:   site.Name,
				Status:     model.CoverageConfirmedNot,
				Source:     model.CoverageSourcePiecesHash,
				Confidence: 0.95,
				QueriedAt:  now,
				ExpiresAt:  ttl,
			})
		}
	}
}

func (h *PublishTorrentsHandler) queryNameSizeSites(ctx context.Context, infoHash, name string, size int64) {
	coveredSites, _ := h.coverage.GetCoveredSiteNames(ctx, infoHash)

	var sites []model.Site
	query := h.db.WithContext(ctx).
		Where("enabled = ? AND is_target = ?", true, true)
	if len(coveredSites) > 0 {
		query = query.Where("name NOT IN ?", coveredSites)
	}
	if err := query.Find(&sites).Error; err != nil {
		h.logger.Warn("query failed", zap.Error(err))
	}

	now := time.Now()
	ttl := now.Add(24 * time.Hour)
	keyword := extractSearchKeyword(name)
	tolerance := size * 2 / 100

	for _, site := range sites {
		adapter, err := h.siteProvider.GetAdapter(ctx, site.Domain)
		if err != nil || adapter == nil {
			continue
		}
		config, err := h.siteProvider.GetSiteConfig(ctx, site.Domain)
		if err != nil || config == nil {
			continue
		}

		results, err := adapter.SearchTorrents(ctx, config, keyword, nil)
		if err != nil {
			h.logger.Debug("name_size search failed", zap.String("site", site.Name), zap.Error(err))
			continue
		}

		matched := false
		for _, result := range results {
			diff := result.Size - size
			if diff < 0 {
				diff = -diff
			}
			if diff <= tolerance {
				h.coverage.UpsertCoverage(ctx, &model.SiteCoverageCache{
					InfoHash:   infoHash,
					SiteName:   site.Name,
					Status:     model.CoverageProbablyHas,
					Source:     model.CoverageSourceNameSize,
					Confidence: 0.7,
					TorrentID:  result.TorrentID,
					QueriedAt:  now,
					ExpiresAt:  ttl,
				})
				matched = true
				break
			}
		}

		if !matched {
			h.coverage.UpsertCoverage(ctx, &model.SiteCoverageCache{
				InfoHash:   infoHash,
				SiteName:   site.Name,
				Status:     model.CoverageProbablyNot,
				Source:     model.CoverageSourceNameSize,
				Confidence: 0.7,
				QueriedAt:  now,
				ExpiresAt:  ttl,
			})
		}
	}
}

func extractSearchKeyword(name string) string {
	idx := strings.LastIndex(name, "-")
	keyword := name
	if idx > 10 {
		keyword = name[:idx]
	}
	return strings.ReplaceAll(strings.ReplaceAll(keyword, ".", " "), "_", " ")
}

func (h *PublishTorrentsHandler) ScheduledRefresh(ctx context.Context) error {
	h.logger.Info("scheduled coverage refresh started")
	var clients []model.ClientConfig
	if err := h.db.WithContext(ctx).Where("enabled = ?", true).Find(&clients).Error; err != nil {
		h.logger.Warn("query failed", zap.Error(err))
	}

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

		items := make([]coverage.BatchItem, 0, len(torrents))
		for _, t := range torrents {
			if queried[t.Hash] {
				continue
			}
			trkCtx, trkCancel := context.WithTimeout(ctx, 5*time.Second)
			trackers, _ := client.GetTrackers(trkCtx, t.Hash)
			trkCancel()
			items = append(items, coverage.BatchItem{
				InfoHash:   t.Hash,
				Trackers:   trackers,
				TorrentDir: extractTorrentDir(cfg.Config),
			})
		}

		if len(items) == 0 {
			continue
		}

		h.logger.Info("coverage refresh: querying",
			zap.String("client", cfg.Name),
			zap.Int("torrents", len(items)))

		batchCtx, batchCancel := context.WithTimeout(ctx, 5*time.Minute)
		h.coverage.QueryBatchCoverage(batchCtx, items)
		if h.siteProvider != nil {
			h.batchPiecesHashQuery(batchCtx, items, cfg)
		}
		batchCancel()
	}

	h.logger.Info("scheduled coverage refresh done")
	return nil
}

func (h *PublishTorrentsHandler) batchPiecesHashQuery(ctx context.Context, items []coverage.BatchItem, cfg model.ClientConfig) {
	// 从 content_fingerprints 表批量读 pieces_hash（复用辅种指纹，不计算种子文件）
	infoHashes := make([]string, 0, len(items))
	for _, item := range items {
		infoHashes = append(infoHashes, item.InfoHash)
	}

	var fps []model.ContentFingerprint
	h.db.WithContext(ctx).Where("info_hash IN ?", infoHashes).Find(&fps)

	hashToPieces := make(map[string]string, len(fps))
	for _, fp := range fps {
		if fp.PiecesHash != "" {
			hashToPieces[fp.InfoHash] = fp.PiecesHash
		}
	}
	if len(hashToPieces) == 0 {
		h.logger.Info("bg L1 fresh: no pieces_hash from fingerprints", zap.Int("torrents", len(items)))
		return
	}

	// 收集去重 pieces_hashes
	allPieces := make([]string, 0, len(hashToPieces))
	seen := make(map[string]bool)
	for _, ph := range hashToPieces {
		if !seen[ph] {
			seen[ph] = true
			allPieces = append(allPieces, ph)
		}
	}
	h.logger.Info("bg L1 fresh: starting",
		zap.Int("torrents", len(hashToPieces)),
		zap.Int("unique_pieces", len(allPieces)))

	// 获取全部目标站点
	var sites []model.Site
	if err := h.db.WithContext(ctx).Where("enabled = ? AND is_target = ?", true, true).Find(&sites).Error; err != nil {
		h.logger.Warn("query failed", zap.Error(err))
	}

	now := time.Now()
	ttl := now.Add(24 * time.Hour)

	for _, site := range sites {
		adapter, err := h.siteProvider.GetAdapter(ctx, site.Domain)
		if err != nil || adapter == nil {
			continue
		}
		if !adapter.SupportsSearchByPiecesHash() {
			continue
		}
		searcher, ok := adapter.(piecesHashSearcher)
		if !ok {
			continue
		}
		config, err := h.siteProvider.GetSiteConfig(ctx, site.Domain)
		if err != nil || config == nil {
			continue
		}

		// 批量查询（100/batch，NexusPHP 限制）
		for i := 0; i < len(allPieces); i += 100 {
			end := i + 100
			if end > len(allPieces) {
				end = len(allPieces)
			}
			batch := allPieces[i:end]

			result, err := searcher.SearchByPiecesHash(ctx, config, batch)
			if err != nil {
				h.logger.Debug("bg L1 fresh: site query failed",
					zap.String("site", site.Name), zap.Error(err))
				continue
			}

			// 将结果映射回 info_hash
			for infoHash, ph := range hashToPieces {
				if tid, found := result[ph]; found {
					h.coverage.UpsertCoverage(ctx, &model.SiteCoverageCache{
						InfoHash:   infoHash,
						SiteName:   site.Name,
						Status:     model.CoverageConfirmedHas,
						Source:     model.CoverageSourcePiecesHash,
						Confidence: 1.0,
						TorrentID:  strconv.Itoa(tid),
						QueriedAt:  now,
						ExpiresAt:  ttl,
					})
				} else {
					h.coverage.UpsertCoverage(ctx, &model.SiteCoverageCache{
						InfoHash:   infoHash,
						SiteName:   site.Name,
						Status:     model.CoverageConfirmedNot,
						Source:     model.CoverageSourcePiecesHash,
						Confidence: 0.95,
						QueriedAt:  now,
						ExpiresAt:  ttl,
					})
				}
			}
		}
	}

	h.logger.Info("bg L1 fresh: done", zap.Int("sites_checked", len(sites)))
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

func extractTorrentDir(configJSON string) string {
	if configJSON == "" {
		return ""
	}
	var cfg struct {
		TorrentDir string `json:"torrent_dir"`
	}
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return ""
	}
	return cfg.TorrentDir
}

type detectSourceRequest struct {
	InfoHash string `json:"info_hash"`
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
		SiteName  string `json:"site_name"`
		TorrentID string `json:"torrent_id"`
		HasCookie bool   `json:"has_cookie"`
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
		MatchedSite string `json:"matched_site"`
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
	TargetSite      string            `json:"target_site"`
	TitleComponents map[string]string `json:"title_components"`
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
		TargetSites     []string          `json:"target_sites"`
		TitleComponents map[string]string `json:"title_components"`
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
		GroupName  string `json:"group_name"`
		Domain     string `json:"domain"`
		SiteName   string `json:"site_name"`
		IsOfficial bool   `json:"is_official"`
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
		GroupName  string `json:"group_name"`
		Domain     string `json:"domain"`
		SiteName   string `json:"site_name"`
		IsOfficial bool   `json:"is_official"`
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
	ClientID   uint   `json:"client_id"`
	SourceSite string `json:"source_site"`
	TargetSite string `json:"target_site"`
	Items      []struct {
		InfoHash string `json:"info_hash"`
		Name     string `json:"name"`
		Size     int64  `json:"size"`
		SavePath string `json:"save_path"`
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

	for _, item := range req.Items {
		candidate := &model.PublishCandidate{
			SourceSite:        req.SourceSite,
			InfoHash:          item.InfoHash,
			TorrentName:       item.Name,
			ClientID:          fmt.Sprintf("%d", req.ClientID),
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
		SiteName  string `json:"site_name"`
		TorrentID string `json:"torrent_id"`
		Reviewed  bool   `json:"reviewed"`
		FetchedAt string `json:"fetched_at"`
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

	updates := map[string]interface{}{
		"title":       req.Title,
		"subtitle":    req.Subtitle,
		"description": req.Description,
		"screenshots": req.Screenshots,
		"poster":      req.Poster,
		"mediainfo":   req.MediaInfo,
		"tags":        req.Tags,
		"reviewed":    true,
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
		TodayPublish   int64 `json:"today_publish"`
		TodaySuccess   int64 `json:"today_success"`
		TodayFailed    int64 `json:"today_failed"`
		PendingCount   int64 `json:"pending_count"`
		ReviewedCount  int64 `json:"reviewed_count"`
		TotalMetadata  int64 `json:"total_metadata"`
		YesterdayPublish int64 `json:"yesterday_publish"`
		YesterdaySuccess int64 `json:"yesterday_success"`
		UnreviewedCount  int64 `json:"unreviewed_count"`
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
		SiteName string `json:"site_name"`
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
	groupCache := make(map[string]string)

	for _, item := range items {
		name, _ := item["name"].(string)
		size, _ := item["size"].(int64)
		key := name + "|" + strconv.FormatInt(size, 10)

		if idx, ok := seen[key]; ok {
			if officialSite := h.lookupOfficialSite(ctx, name, groupCache); officialSite != "" {
				if itemHasSite(item, officialSite) && !itemHasSite(result[idx], officialSite) {
					result[idx] = item
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

func (h *PublishTorrentsHandler) lookupOfficialSite(ctx context.Context, name string, cache map[string]string) string {
	groupName := publish.ExtractGroupName(name)
	if groupName == "" {
		return ""
	}
	if site, ok := cache[groupName]; ok {
		return site
	}
	var mapping model.ReleaseGroupMapping
	if err := h.db.WithContext(ctx).Where("LOWER(group_name) = LOWER(?)", groupName).First(&mapping).Error; err == nil {
		cache[groupName] = mapping.SiteName
		return mapping.SiteName
	}
	cache[groupName] = ""
	return ""
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
	var builtinSites []string
	h.db.WithContext(ctx).Model(&model.ReleaseGroupMapping{}).
		Distinct("site_name").
		Where("is_builtin = ?", true).
		Pluck("site_name", &builtinSites)

	var sourceSites []string
	h.db.WithContext(ctx).Model(&model.Site{}).
		Where("enabled = ? AND is_source = ?", true, true).
		Order("name").
		Pluck("name", &sourceSites)

	builtinSet := make(map[string]bool, len(builtinSites))
	for _, s := range builtinSites {
		builtinSet[s] = true
	}

	var result []string
	for _, s := range builtinSites {
		if s != "" {
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
