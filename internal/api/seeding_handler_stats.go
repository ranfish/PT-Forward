package api

import (
	"context"
	"net/http"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/seeding"
	"go.uber.org/zap"
)

func (h *SeedingHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	activeCount := 0
	var managedCounts *seeding.ManagedCounts
	if h.engine != nil {
		managedCounts = h.engine.GetManagedCounts()
		activeCount = managedCounts.Active
	}

	dbPausedCount := 0
	if managedCounts != nil {
		dbPausedCount = managedCounts.Paused
	}

	Success(w, map[string]interface{}{
		"activeRecords": activeCount,
		"dbPausedCount": dbPausedCount,
	})
}

func (h *SeedingHandler) handleStatsOverview(w http.ResponseWriter, r *http.Request) {
	activeTorrents := 0
	pausedTorrents := 0
	realSeeding := 0
	realDownloading := 0
	if h.engine != nil {
		mc := h.engine.GetManagedCounts()
		activeTorrents = mc.Active
		pausedTorrents = mc.Paused
		for _, rc := range h.engine.GetRealTorrentCounts() {
			realSeeding += rc.Seeding
			realDownloading += rc.Downloading
		}
	}

	var total int64
	if err := h.db.WithContext(r.Context()).Model(&model.SeedingTorrentRecord{}).Count(&total).Error; err != nil {
		h.logger.Warn("stats overview: query total count failed", zap.Error(err))
	}

	today := time.Now().Truncate(24 * time.Hour)
	var todayAdded int64
	if err := h.db.WithContext(r.Context()).Model(&model.SeedingTorrentRecord{}).Where("created_at >= ?", today).Count(&todayAdded).Error; err != nil {
		h.logger.Warn("stats overview: query today added count failed", zap.Error(err))
	}

	var todayDeleted int64
	if err := h.db.WithContext(r.Context()).Model(&model.SeedingTorrentRecord{}).Where("status = ? AND updated_at >= ?", "deleted", today).Count(&todayDeleted).Error; err != nil {
		h.logger.Warn("stats overview: query today deleted count failed", zap.Error(err))
	}

	var globalStats *model.GlobalTransferStats
	if h.engine != nil {
		globalStats = h.engine.GetGlobalTransferStats(context.Background())
	}
	if globalStats == nil {
		globalStats = &model.GlobalTransferStats{}
	}

	todayUpload := int64(0)
	todayDownload := int64(0)
	if h.engine != nil {
		todayStats := h.engine.GetTodayTransferDelta(context.Background())
		todayUpload = todayStats.AllTimeUpload
		todayDownload = todayStats.AllTimeDownload
	}

	var globalRatio float64
	if globalStats.AllTimeDownload > 0 {
		globalRatio = float64(globalStats.AllTimeUpload) / float64(globalStats.AllTimeDownload)
	} else if globalStats.AllTimeUpload > 0 {
		globalRatio = -1
	}

	Success(w, map[string]interface{}{
		"totalTorrents":      total,
		"activeTorrents":     activeTorrents,
		"pausedTorrents":     pausedTorrents,
		"realSeeding":        realSeeding,
		"realDownloading":    realDownloading,
		"totalUploadBytes":   globalStats.AllTimeUpload,
		"totalDownloadBytes": globalStats.AllTimeDownload,
		"todayUploadBytes":   todayUpload,
		"todayDownloadBytes": todayDownload,
		"globalRatio":        globalRatio,
		"todayDeleted":       todayDeleted,
		"todayAdded":         todayAdded,
		"statsScope":         "seeding",
		"statsScopeNote":     "仅统计刷流绑定且启用的下载器全局传输量（含该下载器内所有种子，含非刷流种子）",
	})
}

func (h *SeedingHandler) handleStatsBySite(w http.ResponseWriter, r *http.Request) {
	// §59.565 D2: limit 补上限（parseLimitQuery 单点·原 <1 回落 20 无上限）
	limit, clamped := parseLimitQuery(r)
	if clamped {
		h.logger.Warn("limit clamped to 200", zap.String("op", "stats_by_site"))
	}

	today := time.Now().Truncate(24 * time.Hour)

	type siteStat struct {
		SiteName      string `json:"siteName"`
		SeedingCount  int64  `json:"seedingCount"`
		TotalCount    int64  `json:"totalCount"`
		TodayAdded    int64  `json:"todayAdded"`
		TodayDeleted  int64  `json:"todayDeleted"`
		ActiveFree    int64  `json:"activeFree"`
		ActiveNonFree int64  `json:"activeNonFree"`
		TodayUpload   int64  `json:"todayUploadBytes"`
		HistoryUpload int64  `json:"historyUploadBytes"`
	}

	var stats []siteStat
	if err := h.db.WithContext(r.Context()).Model(&model.SeedingTorrentRecord{}).
		Select(`site_name,
			SUM(CASE WHEN status IN ('seeding','pending','paused_free_end','paused_rule') THEN 1 ELSE 0 END) as seeding_count,
			COUNT(*) as total_count,
			SUM(CASE WHEN created_at >= ? THEN 1 ELSE 0 END) as today_added,
			SUM(CASE WHEN status = 'deleted' AND updated_at >= ? THEN 1 ELSE 0 END) as today_deleted,
			SUM(CASE WHEN is_free = 1 AND status IN ('seeding','pending','paused_free_end','paused_rule') THEN 1 ELSE 0 END) as active_free,
			SUM(CASE WHEN is_free = 0 AND status IN ('seeding','pending','paused_free_end','paused_rule') THEN 1 ELSE 0 END) as active_non_free,
			0 as today_upload_bytes,
			SUM(final_uploaded) as history_upload_bytes`,
			today, today).
		Group("site_name").
		Order("seeding_count DESC").
		Limit(limit).
		Find(&stats).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询站点统计失败")
		return
	}

	siteUploadMap := map[string]int64{}
	type uploadRow struct {
		SiteName    string `json:"site_name"`
		UploadDelta int64  `json:"upload_delta"`
	}
	var uploadRows []uploadRow
	h.db.WithContext(r.Context()).Raw(`SELECT site_name, SUM(delta_upload) as upload_delta
		FROM (
			SELECT site_name, info_hash,
				MAX(uploaded) - MIN(uploaded) AS delta_upload
			FROM torrent_traffic
			WHERE recorded_at >= ?
			GROUP BY site_name, info_hash
		) sub
		GROUP BY site_name`, today).Scan(&uploadRows)
	for _, row := range uploadRows {
		siteUploadMap[row.SiteName] = row.UploadDelta
	}

	for i := range stats {
		if v, ok := siteUploadMap[stats[i].SiteName]; ok {
			stats[i].TodayUpload = v
		}
	}

	historyUploadMap := map[string]int64{}
	type historyRow struct {
		SiteName string `json:"site_name"`
		Uploaded int64  `json:"uploaded"`
	}
	// §59.152: 历史总量直读 current_uploaded 列（Vertex torrents 表模式——
	// 每 UPSERT 最新值；final_uploaded 兜底删除种子；traffic 全表扫 fallback 移除）
	var historyRows []historyRow
	h.db.WithContext(r.Context()).Raw(`SELECT site_name, SUM(uploaded) as uploaded
		FROM (
			SELECT str.site_name, str.info_hash,
				CASE WHEN str.current_uploaded > 0 THEN str.current_uploaded
					WHEN str.final_uploaded > 0 THEN str.final_uploaded
					ELSE 0 END as uploaded
			FROM seeding_torrent_records str
			WHERE str.status IN ('seeding','pending','paused_free_end','paused_rule')
		) sub
		GROUP BY site_name`).Scan(&historyRows)
	for _, row := range historyRows {
		historyUploadMap[row.SiteName] = row.Uploaded
	}

	for i := range stats {
		if v, ok := historyUploadMap[stats[i].SiteName]; ok {
			stats[i].HistoryUpload += v
		}
	}

	Success(w, map[string]interface{}{
		"items": stats,
		"total": len(stats),
	})
}

func (h *SeedingHandler) handleSiteTrend(w http.ResponseWriter, r *http.Request, site string) {
	days := 7
	if v := r.URL.Query().Get("range"); v == "30d" {
		days = 30
	} else if v == "24h" {
		days = 1
	}

	now := time.Now()
	points := make([]map[string]interface{}, 0, days)

	for i := days; i >= 0; i-- {
		day := now.AddDate(0, 0, -i)
		dayStart := day.Truncate(24 * time.Hour)
		dayEnd := dayStart.AddDate(0, 0, 1)

		var count int64
		if err := h.db.WithContext(r.Context()).Model(&model.SeedingTorrentRecord{}).
			Where("site_name = ? AND created_at >= ? AND created_at < ?", site, dayStart, dayEnd).
			Count(&count).Error; err != nil {
			h.logger.Warn("site trend: query count failed", zap.Error(err))
		}

		points = append(points, map[string]interface{}{
			"date":  dayStart.Format("2006-01-02"),
			"count": count,
		})
	}

	Success(w, map[string]interface{}{
		"site":   site,
		"trends": points,
	})
}

func (h *SeedingHandler) handleStatsTorrents(w http.ResponseWriter, r *http.Request) {
	page, pageSize := parsePagination(r) // §59.552 统一（100→200 有意放宽）
	sortBy := r.URL.Query().Get("sort")
	offset := (page - 1) * pageSize

	status := r.URL.Query().Get("status")
	q := h.db.WithContext(r.Context()).Model(&model.SeedingTorrentRecord{}).
		Where("status IN ?", []string{"seeding", "paused_free_end", "paused_rule"})
	if status != "" {
		q = q.Where("status = ?", status)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询种子统计总数失败")
		return
	}

	var orderClause string
	switch sortBy {
	case "size":
		orderClause = "torrent_size DESC"
	case "time":
		orderClause = "flushed_at DESC"
	default: // uploaded 及默认
		orderClause = "final_uploaded DESC"
	}

	var records []model.SeedingTorrentRecord
	if err := q.Order(orderClause).Offset(offset).Limit(pageSize).Find(&records).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询种子统计失败")
		return
	}

	type torrentWithTraffic struct {
		model.SeedingTorrentRecord
		LatestUpload int64  `json:"latest_upload"`
		Title        string `json:"title"`
		DetailURL    string `json:"detail_url"`
	}

	type siteInfo struct {
		BaseURL   string
		Framework string
	}
	siteMap := map[string]siteInfo{}
	var siteList []model.Site
	if err := h.db.WithContext(r.Context()).Select("name, base_url, framework").Find(&siteList).Error; err != nil {
		h.logger.Warn("query failed", zap.Error(err))
	}

	for _, s := range siteList {
		siteMap[s.Name] = siteInfo{BaseURL: s.BaseURL, Framework: s.Framework}
	}

	hashes := make([]string, 0, len(records))
	for _, rec := range records {
		hyield := rec.InfoHash
		if len(hyield) > 0 {
			hashes = append(hashes, hyield)
		}
	}

	latestMap := map[string]int64{}
	titleKeyMap := map[string]string{}
	if len(hashes) > 0 {
		type trafficRow struct {
			InfoHash string `json:"info_hash"`
			Uploaded int64  `json:"uploaded"`
		}
		var rows []trafficRow
		h.db.WithContext(r.Context()).Raw(`SELECT info_hash, MAX(uploaded) as uploaded
			FROM torrent_traffic
			WHERE info_hash IN (?) AND recorded_at >= ?
			GROUP BY info_hash`, hashes, time.Now().Truncate(24*time.Hour)).Scan(&rows)
		for _, row := range rows {
			latestMap[row.InfoHash] = row.Uploaded
		}

		type titleRow struct {
			SiteName  string `json:"site_name"`
			TorrentID string `json:"torrent_id"`
			Title     string `json:"title"`
		}
		var titles []titleRow
		siteTorrents := make([]string, 0, len(records))
		for _, rec := range records {
			siteTorrents = append(siteTorrents, rec.SiteName+"|"+rec.TorrentID)
		}
		h.db.WithContext(r.Context()).Raw(`SELECT site_name, torrent_id, title FROM rss_torrent_seen WHERE site_name || '|' || torrent_id IN (?)`, siteTorrents).Scan(&titles)
		for _, t := range titles {
			titleKeyMap[t.SiteName+"|"+t.TorrentID] = t.Title
		}
	}

	result := make([]torrentWithTraffic, 0, len(records))
	for _, rec := range records {
		r := torrentWithTraffic{SeedingTorrentRecord: rec}
		if v, ok := latestMap[rec.InfoHash]; ok {
			r.LatestUpload = v
		}
		if t, ok := titleKeyMap[rec.SiteName+"|"+rec.TorrentID]; ok {
			r.Title = t
		}
		if si, ok := siteMap[rec.SiteName]; ok && si.BaseURL != "" && rec.TorrentID != "" {
			r.DetailURL = buildDetailURL(si.BaseURL, si.Framework, rec.TorrentID)
		}
		result = append(result, r)
	}

	Success(w, map[string]interface{}{
		"items": result,
		"total": total,
		"page":  page,
	})
}

func (h *SeedingHandler) handleDownloaderSpeedTrend(w http.ResponseWriter, r *http.Request, clientUID uint) {
	hours := 24
	if v := r.URL.Query().Get("range"); v == "7d" {
		hours = 168
	} else if v == "30d" {
		hours = 720
	}

	now := time.Now()
	step := hours / 24
	if step < 1 {
		step = 1
	}

	rangeStart := now.Add(time.Duration(-hours) * time.Hour).Truncate(time.Hour)
	rangeEnd := now.Truncate(time.Hour).Add(time.Hour)

	var snapshots []model.DownloaderSpeedSnapshot
	if err := h.db.WithContext(r.Context()).Where("client_uid = ? AND recorded_at >= ? AND recorded_at < ?", clientUID, rangeStart, rangeEnd).
		Order("recorded_at ASC").Find(&snapshots).Error; err != nil {
		h.logger.Warn("query speed snapshots failed",
			zap.Uint("client_uid", clientUID),
			zap.Error(err))
	}

	bucketMap := make(map[string]model.DownloaderSpeedSnapshot)
	for _, s := range snapshots {
		key := s.RecordedAt.Truncate(time.Duration(step) * time.Hour).Format(time.RFC3339)
		bucketMap[key] = s
	}

	points := make([]map[string]interface{}, 0)
	for i := hours; i >= 0; i -= step {
		t := now.Add(time.Duration(-i) * time.Hour)
		tKey := t.Truncate(time.Duration(step) * time.Hour).Format(time.RFC3339)
		tDisplay := t.Truncate(time.Hour).Format(time.RFC3339)

		point := map[string]interface{}{
			"timestamp":     tDisplay,
			"uploadSpeed":   0,
			"downloadSpeed": 0,
		}
		if s, ok := bucketMap[tKey]; ok {
			point["uploadSpeed"] = s.UploadSpeed
			point["downloadSpeed"] = s.DownloadSpeed
		}
		points = append(points, point)
	}

	Success(w, map[string]interface{}{
		"clientId": clientUID,
		"points":   points,
	})
}
