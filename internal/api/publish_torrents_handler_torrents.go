package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/publish"
	"github.com/ranfish/pt-forward/internal/site"
	"github.com/ranfish/pt-forward/internal/titleparser"
	"go.uber.org/zap"
)

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

	clientUID, err := strconv.ParseUint(clientIDStr, 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的 client_id")
		return
	}

	var cfg model.ClientConfig
	if err := h.db.First(&cfg, clientUID).Error; err != nil {
		Error(w, http.StatusNotFound, 40400, "下载器不存在")
		return
	}

	client, err := h.clientMgr.Get(cfg.ID)
	if err != nil {
		// §59.564 刀B: 5xx 业务日志（审计 03 §2.3）
		h.logger.Error("connect client failed", zap.String("op", "connect_client"), zap.Uint("client_id", cfg.ID), zap.Error(err))
		Error(w, http.StatusInternalServerError, 50000, fmt.Sprintf("连接下载器失败: %v", err))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	torrents, err := client.GetSeedingTorrents(ctx)
	if err != nil {
		// §59.564 刀B: 5xx 业务日志
		h.logger.Error("get seeding torrents failed", zap.String("op", "get_seeding_torrents"), zap.Uint("client_id", cfg.ID), zap.Error(err))
		Error(w, http.StatusInternalServerError, 50000, fmt.Sprintf("获取种子列表失败: %v", err))
		return
	}

	// v0.0.265: tracker → 站点匹配（用于列表展示"做种站点"列）
	trackerMatcher := site.NewTrackerMatcher(h.db)

	var totalSites int64
	if err := h.db.Model(&model.Site{}).Where("enabled = ? AND is_reseed_target = ?", true, true).Count(&totalSites).Error; err != nil {
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
	querying := h.bgState.isQuerying(uint(clientUID))
	if !querying && h.coverage != nil {
		unqueried := 0
		for _, t := range torrents {
			if !queriedMap[t.Hash] {
				unqueried++
			}
		}
		if unqueried > 0 {
			go h.startBackgroundQuery(uint(clientUID), cfg, torrents)
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

func (h *PublishTorrentsHandler) startBackgroundQuery(clientUID uint, cfg model.ClientConfig, torrents []*model.TorrentInfo) {
	h.bgState.start(clientUID, len(torrents))
	defer h.bgState.stop(clientUID)

	// §59.562 刀3: Task ctx 派生自 service ctx（baseCtx）——脱离 HTTP 请求
	// 合理（原 §59.50 根因）·但接入停机：停机 cancel 提前中止·结果状态
	// 不更新=下次重查（幂等·最终一致）。
	ctx, cancel := context.WithTimeout(h.baseCtx, 10*time.Minute)
	defer cancel()

	client, err := h.clientMgr.Get(cfg.ID)
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
				if cerr := h.coverage.UpsertCoverage(ctx, &model.SiteCoverageCache{
					InfoHash: infoHash, SiteName: hit.SiteName,
					Status: model.CoverageConfirmedHas, Source: source,
					Confidence: 1.0, TorrentID: hit.TorrentID,
					QueriedAt: now, ExpiresAt: ttl,
				}); cerr != nil {
					h.logger.Warn("UpsertCoverage failed", zap.String("site", hit.SiteName), zap.Error(cerr))
				}
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
				if cerr := h.coverage.UpsertCoverage(ctx, &model.SiteCoverageCache{
					InfoHash: t.Hash, SiteName: sn,
					Status: model.CoverageConfirmedHas, Source: model.CoverageSourceTracker,
					Confidence: 1.0, QueriedAt: now, ExpiresAt: ttl,
				}); cerr != nil {
					h.logger.Warn("UpsertCoverage failed", zap.String("site", sn), zap.Error(cerr))
				}
			}
		}
	}
}

type coverageQueryRequest struct {
	ClientUID uint   `json:"clientId"`
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
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if req.InfoHash == "" || req.ClientUID == 0 {
		Error(w, http.StatusBadRequest, 40001, "info_hash 和 client_id 必填")
		return
	}

	var cfg model.ClientConfig
	if err := h.db.First(&cfg, req.ClientUID).Error; err != nil {
		Error(w, http.StatusNotFound, 40400, "下载器不存在")
		return
	}

	client, err := h.clientMgr.Get(cfg.ID)
	if err != nil {
		// §59.564 刀B: 5xx 业务日志（审计 03 §2.3）
		h.logger.Error("connect client failed", zap.String("op", "connect_client"), zap.Uint("client_id", cfg.ID), zap.Error(err))
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
			if cerr := h.coverage.UpsertCoverage(ctx, &model.SiteCoverageCache{
				InfoHash:   req.InfoHash,
				SiteName:   hit.SiteName,
				Status:     model.CoverageConfirmedHas,
				Source:     source,
				Confidence: 1.0,
				TorrentID:  hit.TorrentID,
				QueriedAt:  now,
				ExpiresAt:  ttl,
			}); cerr != nil {
				h.logger.Warn("UpsertCoverage failed", zap.String("site", hit.SiteName), zap.Error(cerr))
			}
		}
	}

	// ② tracker 映射 → 🟢 做种中（后写，覆盖同名 🟡）
	trackers, err := client.GetTrackers(ctx, req.InfoHash)
	if err == nil && len(trackers) > 0 {
		tm := site.NewTrackerMatcher(h.db)
		trackerSites := tm.MatchAll(trackers)
		for _, sn := range trackerSites {
			if cerr := h.coverage.UpsertCoverage(ctx, &model.SiteCoverageCache{
				InfoHash:   req.InfoHash,
				SiteName:   sn,
				Status:     model.CoverageConfirmedHas,
				Source:     model.CoverageSourceTracker,
				Confidence: 1.0,
				QueriedAt:  now,
				ExpiresAt:  ttl,
			}); cerr != nil {
				h.logger.Warn("UpsertCoverage failed", zap.String("site", sn), zap.Error(cerr))
			}
		}
	}

	// 标记查询状态
	h.coverage.MarkQueried(ctx, req.InfoHash, now, ttl)

	// 返回最终结果
	cached, _ := h.coverage.GetCachedCoverage(ctx, req.InfoHash)

	var totalSites int64
	if err := h.db.Model(&model.Site{}).Where("enabled = ? AND is_reseed_target = ?", true, true).Count(&totalSites).Error; err != nil {
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
		ClientUID  uint     `json:"clientId"`
		InfoHashes []string `json:"infoHashes"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if req.ClientUID == 0 || len(req.InfoHashes) == 0 {
		Error(w, http.StatusBadRequest, 40001, "client_id 和 info_hashes 必填")
		return
	}

	var cfg model.ClientConfig
	if err := h.db.First(&cfg, req.ClientUID).Error; err != nil {
		Error(w, http.StatusNotFound, 40400, "下载器不存在")
		return
	}
	client, err := h.clientMgr.Get(cfg.ID)
	if err != nil {
		// §59.564 刀B: 5xx 业务日志（审计 03 §2.3）
		h.logger.Error("connect client failed", zap.String("op", "connect_client"), zap.Uint("client_id", cfg.ID), zap.Error(err))
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
			if cerr := h.coverage.UpsertCoverage(ctx, &model.SiteCoverageCache{
				InfoHash: infoHash, SiteName: hit.SiteName,
				Status: model.CoverageConfirmedHas, Source: source,
				Confidence: 1.0, TorrentID: hit.TorrentID,
				QueriedAt: now, ExpiresAt: ttl,
			}); cerr != nil {
				h.logger.Warn("UpsertCoverage failed", zap.String("site", hit.SiteName), zap.Error(cerr))
			}
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
					if cerr := h.coverage.UpsertCoverage(ctx, &model.SiteCoverageCache{
						InfoHash: t.Hash, SiteName: sn,
						Status: model.CoverageConfirmedHas, Source: model.CoverageSourceTracker,
						Confidence: 1.0, QueriedAt: now, ExpiresAt: ttl,
					}); cerr != nil {
						h.logger.Warn("UpsertCoverage failed", zap.String("site", sn), zap.Error(cerr))
					}
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
				if cerr := h.coverage.UpsertCoverage(ctx, &model.SiteCoverageCache{
					InfoHash: infoHash, SiteName: sn,
					Status: model.CoverageConfirmedHas, Source: model.CoverageSourceTracker,
					Confidence: 1.0, QueriedAt: now, ExpiresAt: ttl,
				}); cerr != nil {
					h.logger.Warn("UpsertCoverage failed", zap.String("site", sn), zap.Error(cerr))
				}
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

func (h *PublishTorrentsHandler) handleQueryStatus(w http.ResponseWriter, r *http.Request) {
	clientIDStr := r.URL.Query().Get("client_id")
	clientUID, _ := strconv.ParseUint(clientIDStr, 10, 64)

	querying := h.bgState.isQuerying(uint(clientUID))
	_, done, total := h.bgState.getProgress()
	Success(w, map[string]interface{}{
		"querying": querying,
		"done":     done,
		"total":    total,
	})
}

func (s *backgroundQueryState) start(clientUID uint, total int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active[clientUID] = true
	s.total = total
	s.done = 0
}

func (s *backgroundQueryState) stop(clientUID uint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.active, clientUID)
}

func (s *backgroundQueryState) isQuerying(clientUID uint) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active[clientUID]
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
	if !decodeJSONBody(w, r, &req) {
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
		Success(w, map[string]interface{}{"builtin": []string{}, "custom": []string{}})
		return
	}
	// §59.516 v0.0.1457：返回 builtin/custom 两组——前端内置锁定+自定义可管理
	Success(w, map[string]interface{}{
		"builtin": publish.BuiltinPatterns(),
		"custom":  h.declFilter.GetCustomPatterns(r.Context()),
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
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if err := h.declFilter.SetPatterns(r.Context(), req.Patterns); err != nil {
		// §59.564 刀B: 5xx 业务日志
		h.logger.Error("save decl patterns failed", zap.String("op", "save_decl_patterns"), zap.Int("count", len(req.Patterns)), zap.Error(err))
		Error(w, http.StatusInternalServerError, 50000, fmt.Sprintf("保存失败: %v", err))
		return
	}
	// 回显实际存储的自定义集（内置词剔除后）
	Success(w, map[string]interface{}{
		"custom":  h.declFilter.GetCustomPatterns(r.Context()),
		"builtin": publish.BuiltinPatterns(),
		"message": "已保存（系统内置词不受影响）",
	})
}

type previewTitleRequest struct {
	TargetSite      string            `json:"targetSite"`
	TitleComponents map[string]string `json:"titleComponents"`
}

func (h *PublishTorrentsHandler) handlePreviewTitle(w http.ResponseWriter, r *http.Request) {
	var req previewTitleRequest
	if !decodeJSONBody(w, r, &req) {
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
	if !decodeJSONBody(w, r, &req) {
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
	if !decodeJSONBody(w, r, &req) {
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
		// §59.564 刀B: 5xx 业务日志
		h.logger.Error("create group mapping failed", zap.String("op", "create_group_mapping"), zap.String("group_name", req.GroupName), zap.String("domain", req.Domain), zap.Error(err))
		Error(w, http.StatusInternalServerError, 50000, fmt.Sprintf("创建失败: %v", err))
		return
	}
	if h.sourceDetector != nil {
		h.sourceDetector.RefreshCache(r.Context())
		h.syncGroupLexicon(r.Context())
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
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if err := h.db.Model(&model.ReleaseGroupMapping{}).Where("id = ?", id).Updates(map[string]interface{}{
		"group_name":  req.GroupName,
		"domain":      req.Domain,
		"site_name":   req.SiteName,
		"is_official": req.IsOfficial,
	}).Error; err != nil {
		// §59.564 刀B: 5xx 业务日志
		h.logger.Error("update group mapping failed", zap.String("op", "update_group_mapping"), zap.Uint64("id", id), zap.String("group_name", req.GroupName), zap.Error(err))
		Error(w, http.StatusInternalServerError, 50000, fmt.Sprintf("更新失败: %v", err))
		return
	}
	if h.sourceDetector != nil {
		h.sourceDetector.RefreshCache(r.Context())
		h.syncGroupLexicon(r.Context())
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
		// §59.564 刀B: 5xx 业务日志
		h.logger.Error("delete group mapping failed", zap.String("op", "delete_group_mapping"), zap.Uint64("id", id), zap.Error(err))
		Error(w, http.StatusInternalServerError, 50000, fmt.Sprintf("删除失败: %v", err))
		return
	}

	if h.sourceDetector != nil {
		h.sourceDetector.RefreshCache(r.Context())
		h.syncGroupLexicon(r.Context())
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
	ClientUID  uint   `json:"clientId"`
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
	if !decodeJSONBody(w, r, &req) {
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
	var clientUID uint
	if req.ClientUID > 0 {
		clientUID = req.ClientUID
	}

	for _, item := range req.Items {
		candidate := &model.PublishCandidate{
			SourceSite:  req.SourceSite,
			InfoHash:    item.InfoHash,
			TorrentName: item.Name,
			ClientUID:   clientUID,
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
			if cerr := h.coverage.UpdateFromPublishResult(r.Context(), item.InfoHash, req.TargetSite); cerr != nil {
				h.logger.Warn("UpdateFromPublishResult failed", zap.String("hash", item.InfoHash), zap.Error(cerr))
			}
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

// normalizeCategorySimple §59.317: 委托 titleparser.NormalizeSourceCategory 单点
// （原平行实现——键族覆盖漂移的重复造轮子，§59.26 教训；行为变化：未知输入
// 由原样返回改为返回 ""——RSS seen 原始 ID 形态不再泄漏到展示层）。
func normalizeCategorySimple(raw string) string {
	return titleparser.NormalizeSourceCategory(raw)
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
