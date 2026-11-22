package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/publish"
	"go.uber.org/zap"
)

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
	if !decodeJSONBody(w, r, &req) {
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
// §59.275→§59.288 定位演进: ids 形态=运维内部（簇级批审/数据修复）；
// info_hashes 形态=前端 UI 批量审核（列表行天然主键是 hash——与批量清除同
// 钥匙形态；服务端解析 hash→ids 走原路径，簇同步 §59.94 对两形态统一生效）。
func (h *PublishTorrentsHandler) handleBatchReview(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs        []uint   `json:"ids"`
		InfoHashes []string `json:"info_hashes"`
		Reviewed   bool     `json:"reviewed"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	ids := req.IDs
	if len(req.InfoHashes) > 0 {
		// §59.288→§59.316 修订: hash→id 解析走【簇口径】——输入是列表行簇代表 hash，
		// 元数据可能在兄弟 hash 下（列表状态机按 name join 命中=显示待审核；直连
		// info_hash 解析在簇代表换站后失配→空→"ids required"误拒，243 八连案：
		// 簇 9 hash 元数据 1 行在兄弟站）。hash → 快照簇键 → 全簇 hash → ids，
		// 与列表状态机/簇同步 §59.94 同口径。
		clusterSub := h.db.WithContext(r.Context()).
			Table("torrent_snapshots").
			Select("DISTINCT client_uid, save_path, name").
			Where("hash IN ? AND is_hidden = 0 AND name != ''", req.InfoHashes)
		var clusterHashes []string
		if err := h.db.WithContext(r.Context()).
			Table("torrent_snapshots").
			Where("is_hidden = 0 AND name != '' AND (client_uid, save_path, name) IN (?)", clusterSub).
			Pluck("hash", &clusterHashes).Error; err == nil && len(clusterHashes) > 0 {
			req.InfoHashes = clusterHashes
		}
		var resolved []uint
		if err := h.db.WithContext(r.Context()).
			Model(&model.TorrentMetadata{}).
			Where("info_hash IN ?", req.InfoHashes).
			Pluck("id", &resolved).Error; err != nil {
			Error(w, http.StatusInternalServerError, 50000, "db error")
			return
		}
		ids = append(ids, resolved...)
	}
	if len(ids) == 0 {
		Error(w, http.StatusBadRequest, 40001, "ids required")
		return
	}
	result := h.db.WithContext(r.Context()).
		Model(&model.TorrentMetadata{}).
		Where("id IN ?", ids).
		Update("reviewed", req.Reviewed)
	if result.Error != nil {
		h.logger.Warn("batch review failed", zap.Error(result.Error))
		Error(w, http.StatusInternalServerError, 50000, "db error")
		return
	}
	// §59.94: 审核簇同步（公共方法——批量 ID 去重簇键后逐簇传播）
	h.syncClusterReviewedByIDs(context.Background(), ids, req.Reviewed)
	Success(w, map[string]interface{}{"updated": result.RowsAffected})
}

// handleBatchDelete §56.40: 批量删除元数据（硬删除，仅删快照不删种子）。
func (h *PublishTorrentsHandler) handleBatchDelete(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []uint `json:"ids"`
	}
	if !decodeJSONBody(w, r, &req) {
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
		TodayPublish     int64 `json:"todayPublish"`
		TodaySuccess     int64 `json:"todaySuccess"`
		TodayFailed      int64 `json:"todayFailed"`
		PendingCount     int64 `json:"pendingCount"`
		ReviewedCount    int64 `json:"reviewedCount"`
		TotalMetadata    int64 `json:"totalMetadata"`
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
	defer func() { _ = rows.Close() }()

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
		"stats":               stats,
		"recent":              recent,
		"trend":               trend,
		"target_site_top":     targetSiteTop,
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
	if !decodeJSONBody(w, r, &req) {
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
	if !decodeJSONBody(w, r, &req) {
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
		ClientUID uint `json:"clientId"`
		// §59.479: force=true 批量重获——绕过 hasCompleteMetadata 簇跳过
		// + forcePTGen 强刷（§59.173）。来源：禁转/无映射筛选视图「批量重获」
		// 按钮（该视图行多为已获取簇·常规批量获取会被簇跳过全部 no-op）。
		Force bool `json:"force"`
		Items []struct {
			Hash     string `json:"hash"`
			Name     string `json:"name"`
			Size     int64  `json:"size"`
			SavePath string `json:"savePath"`
		} `json:"items"`
	}
	if !decodeJSONBody(w, r, &req) {
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
	h.batchFetch.stopRequested = false // §59.495: 新批次重置停止请求
	h.batchFetch.total = len(req.Items)
	h.batchFetch.done = 0
	h.batchFetch.failed = 0
	h.batchFetch.items = make([]batchFetchItem, len(req.Items))
	for i, item := range req.Items {
		h.batchFetch.items[i] = batchFetchItem{Hash: item.Hash, Name: item.Name, Size: item.Size, SavePath: item.SavePath, Status: "pending"}
	}
	h.batchFetch.mu.Unlock()

	go h.runBatchFetch(req.ClientUID, req.Items, req.Force)

	Success(w, map[string]interface{}{"message": "已开始", "total": len(req.Items)})
}
