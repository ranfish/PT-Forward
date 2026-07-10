package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ranfish/pt-forward/internal/coverage"
	"github.com/ranfish/pt-forward/internal/model"
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
	db           *gorm.DB
	coverage     *coverage.Service
	clientMgr    MFClientProvider
	siteProvider SiteProviderGetter
	logger       *zap.Logger
	bgState      backgroundQueryState
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
func (h *PublishTorrentsHandler) SetClientProvider(c MFClientProvider)  { h.clientMgr = c }
func (h *PublishTorrentsHandler) SetSiteProvider(s SiteProviderGetter)  { h.siteProvider = s }

func (h *PublishTorrentsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimRight(r.URL.Path, "/")

	switch {
	case strings.HasSuffix(path, "/publish/torrents") && r.Method == http.MethodGet:
		h.handleListTorrents(w, r)
	case strings.HasSuffix(path, "/publish/torrents/coverage") && r.Method == http.MethodPost:
		h.handleQueryCoverage(w, r)
	case strings.HasSuffix(path, "/publish/torrents/query-status") && r.Method == http.MethodGet:
		h.handleQueryStatus(w, r)
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
		h.db.Where("enabled = ?", true).Find(&clients)
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

	var totalSites int64
	h.db.Model(&model.Site{}).Where("enabled = ? AND is_target = ?", true, true).Count(&totalSites)

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

	items := make([]map[string]interface{}, 0, len(torrents))
	for _, t := range torrents {
		cov := coverageMap[t.Hash]
		hasCount := 0
		for _, c := range cov {
			if c.Status == model.CoverageConfirmedHas || c.Status == model.CoverageProbablyHas {
				hasCount++
			}
		}
		items = append(items, map[string]interface{}{
			"info_hash": t.Hash,
			"name":      t.Name,
			"size":      t.TotalSize,
			"save_path": t.SavePath,
			"state":     t.State,
			"uploaded":  t.Uploaded,
			"queried":   queriedMap[t.Hash],
			"coverage": map[string]interface{}{
				"has_count":    hasCount,
				"total_sites":  totalSites,
				"target_count": int(totalSites) - hasCount,
				"sites":        cov,
			},
		})
	}

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

	Success(w, map[string]interface{}{
		"items":       items,
		"total":       len(items),
		"total_sites": totalSites,
		"querying":    querying,
		"query_progress": map[string]int{
			"done":  h.bgState.done,
			"total": h.bgState.total,
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
			h.logger.Debug("bg query: get trackers failed", zap.String("hash", t.Hash[:8]), zap.Error(err))
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

	// L0: trackers
	trackers, err := client.GetTrackers(ctx, req.InfoHash)
	if err != nil {
		h.logger.Warn("query coverage: get trackers failed", zap.Error(err))
		trackers = nil
	}

	// 快车道：L0 + L1(缓存) + L2(IYUU)
	_, err = h.coverage.QueryCoverage(ctx, req.InfoHash, trackers)
	if err != nil {
		h.logger.Warn("query coverage: fast query failed", zap.Error(err))
	}

	// 慢车道：L1 fresh（pieces-hash 本地计算 + 站点 API 查询）
	torrentDir := extractTorrentDir(cfg.Config)
	if h.siteProvider != nil && torrentDir != "" {
		piecesHash, err := coverage.ComputePiecesHashFromDir(torrentDir, req.InfoHash)
		if err != nil {
			h.logger.Debug("query coverage: pieces_hash compute skipped", zap.String("hash", req.InfoHash[:8]), zap.Error(err))
		} else {
			h.queryPiecesHashSites(ctx, req.InfoHash, piecesHash)
		}
	}

	// 慢车道：L3（名称/体积搜索）
	if h.siteProvider != nil && req.Name != "" && req.Size > 0 {
		h.queryNameSizeSites(ctx, req.InfoHash, req.Name, req.Size)
	}

	// 返回最终结果
	cached, _ := h.coverage.GetCachedCoverage(ctx, req.InfoHash)

	var totalSites int64
	h.db.Model(&model.Site{}).Where("enabled = ? AND is_target = ?", true, true).Count(&totalSites)

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
	query.Find(&sites)

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
	h.db.WithContext(ctx).Where("enabled = ?", true).Find(&clients)

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
	torrentDir := extractTorrentDir(cfg.Config)
	if torrentDir == "" {
		return
	}

	// 批量计算 pieces_hash
	hashToPieces := make(map[string]string, len(items))
	for _, item := range items {
		ph, err := coverage.ComputePiecesHashFromDir(torrentDir, item.InfoHash)
		if err != nil {
			continue
		}
		hashToPieces[item.InfoHash] = ph
	}
	if len(hashToPieces) == 0 {
		h.logger.Info("bg L1 fresh: no pieces_hash computed", zap.String("torrent_dir", torrentDir))
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
	h.db.WithContext(ctx).Where("enabled = ? AND is_target = ?", true, true).Find(&sites)

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
	Success(w, map[string]interface{}{
		"querying": querying,
		"done":     h.bgState.done,
		"total":    h.bgState.total,
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
