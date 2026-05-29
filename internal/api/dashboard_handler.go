package api

import (
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/seeding"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type DashboardHandler struct {
	db            *gorm.DB
	logger        *zap.Logger
	version       string
	clientChecker clientOnlineChecker
	seedingEngine torrentCounter
}

type clientOnlineChecker interface {
	ConnectedCount() int
}

type torrentCounter interface {
	GetRealTorrentCounts() map[string]*seeding.RealTorrentCounts
}

func NewDashboardHandler(db *gorm.DB, logger *zap.Logger, version string, checker clientOnlineChecker) *DashboardHandler {
	return &DashboardHandler{db: db, logger: logger, version: version, clientChecker: checker}
}

func (h *DashboardHandler) SetSeedingEngine(engine torrentCounter) {
	h.seedingEngine = engine
}

func (h *DashboardHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	trimmed := strings.TrimRight(path, "/")

	switch {
	case strings.HasSuffix(trimmed, "/dashboard/overview"):
		h.handleOverview(w, r)
	case strings.HasSuffix(trimmed, "/dashboard/activities"):
		h.handleActivities(w, r)
	case strings.HasSuffix(trimmed, "/dashboard/trends"):
		h.handleTrends(w, r)
	case strings.HasPrefix(trimmed, "/api/v1/torrent-events"):
		h.handleTorrentEvents(w, r, trimmed)
	default:
		Error(w, http.StatusNotFound, 40400, "接口不存在")
	}
}

func (h *DashboardHandler) handleOverview(w http.ResponseWriter, _ *http.Request) {
	var rssCount int64
	if err := h.db.Model(&model.RSSSubscription{}).Where("enabled = ?", true).Count(&rssCount).Error; err != nil {
		h.logger.Warn("dashboard: query rss count failed", zap.Error(err))
	}

	var realSeeding, realDownloading, realPaused, realTotal int
	var realTotalSize int64
	if h.seedingEngine != nil {
		counts := h.seedingEngine.GetRealTorrentCounts()
		for _, c := range counts {
			realSeeding += c.Seeding
			realDownloading += c.Downloading
			realPaused += c.Paused
			realTotal += c.Total
			realTotalSize += c.TotalSize
		}
	}

	var reseedActive int64
	if err := h.db.Model(&model.ReseedTask{}).Where("enabled = ?", true).Count(&reseedActive).Error; err != nil {
		h.logger.Warn("dashboard: query reseed active count failed", zap.Error(err))
	}

	var publishPending int64
	if err := h.db.Model(&model.PublishCandidate{}).Where("publish_status = ?", "pending").Count(&publishPending).Error; err != nil {
		h.logger.Warn("dashboard: query publish pending count failed", zap.Error(err))
	}

	var publishToday int64
	today := time.Now().Truncate(24 * time.Hour)
	if err := h.db.Model(&model.PublishCandidate{}).Where("publish_status = ? AND updated_at >= ?", "done", today).Count(&publishToday).Error; err != nil {
		h.logger.Warn("dashboard: query publish today count failed", zap.Error(err))
	}

	var publishTotal int64
	if err := h.db.Model(&model.PublishCandidate{}).Where("publish_status = ?", "done").Count(&publishTotal).Error; err != nil {
		h.logger.Warn("dashboard: query publish total count failed", zap.Error(err))
	}

	var reseedToday int64
	if err := h.db.Model(&model.ReseedMatch{}).Where("status = ? AND injected_at >= ?", "injected", today).Count(&reseedToday).Error; err != nil {
		h.logger.Warn("dashboard: query reseed today count failed", zap.Error(err))
	}

	var reseedTotal int64
	if err := h.db.Model(&model.ReseedMatch{}).Where("status = ?", "injected").Count(&reseedTotal).Error; err != nil {
		h.logger.Warn("dashboard: query reseed total count failed", zap.Error(err))
	}

	var siteTotal int64
	if err := h.db.Model(&model.Site{}).Count(&siteTotal).Error; err != nil {
		h.logger.Warn("dashboard: query site total count failed", zap.Error(err))
	}

	var siteOnline int64
	if err := h.db.Model(&model.Site{}).Where("enabled = ?", true).Count(&siteOnline).Error; err != nil {
		h.logger.Warn("dashboard: query site online count failed", zap.Error(err))
	}

	var clientConfigs []model.ClientConfig
	if err := h.db.Where("enabled = ?", true).Find(&clientConfigs).Error; err != nil {
		h.logger.Warn("dashboard: query client configs failed", zap.Error(err))
	}

	onlineCount := len(clientConfigs)
	if h.clientChecker != nil {
		if count := h.clientChecker.ConnectedCount(); count >= 0 {
			onlineCount = count
		}
	}

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	Success(w, map[string]interface{}{
		"sites": map[string]interface{}{
			"total":  siteTotal,
			"online": siteOnline,
		},
		"downloaders": map[string]interface{}{
			"total":  len(clientConfigs),
			"online": onlineCount,
		},
		"torrents": map[string]interface{}{
			"seeding":     realSeeding,
			"paused":      realPaused,
			"downloading": realDownloading,
			"total":       realTotal,
			"totalSize":   realTotalSize,
		},
		"publish": map[string]interface{}{
			"todayCount":   publishToday,
			"totalCount":   publishTotal,
			"pendingCount": publishPending,
		},
		"reseed": map[string]interface{}{
			"todayCount":   reseedToday,
			"totalCount":   reseedTotal,
			"runningTasks": reseedActive,
		},
		"system": map[string]interface{}{
			"uptime":      time.Since(startTime).Seconds(),
			"version":     h.version,
			"goroutines":  runtime.NumGoroutine(),
			"memoryUsage": memStats.Alloc,
		},
		"rssSubscriptions": rssCount,
	})
}

func (h *DashboardHandler) handleActivities(w http.ResponseWriter, r *http.Request) {
	page := int64(1)
	size := int64(20)
	if v := r.URL.Query().Get("page"); v != "" {
		if n, err := parseInt64(v); err == nil && n > 0 {
			page = n
		}
	}
	if v := r.URL.Query().Get("size"); v != "" {
		if n, err := parseInt64(v); err == nil && n > 0 && n <= 200 {
			size = n
		}
	}

	var total int64
	h.db.Model(&model.RSSTorrentSeen{}).Count(&total)

	var seen []model.RSSTorrentSeen
	offset := int((page - 1) * size)
	if err := h.db.Order("created_at DESC").Offset(offset).Limit(int(size)).Find(&seen).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询最近种子失败")
		return
	}

	type seedingHash struct {
		SiteName  string
		TorrentID string
		InfoHash  string
	}
	var seedingHashes []seedingHash
	h.db.Model(&model.SeedingTorrentRecord{}).
		Select("site_name, torrent_id, info_hash").
		Find(&seedingHashes)

	realHashMap := make(map[string]string, len(seedingHashes))
	for _, sh := range seedingHashes {
		realHashMap[sh.SiteName+":"+sh.TorrentID] = sh.InfoHash
	}

	type siteInfo struct {
		BaseURL   string
		Framework string
	}
	var sites []model.Site
	h.db.Select("name, base_url, framework").Find(&sites)
	siteMap := make(map[string]siteInfo, len(sites))
	for _, s := range sites {
		siteMap[s.Name] = siteInfo{BaseURL: s.BaseURL, Framework: s.Framework}
	}

	type activityItem struct {
		model.RSSTorrentSeen
		DetailURL string `json:"detail_url"`
	}

	items := make([]activityItem, len(seen))
	for i := range seen {
		if len(seen[i].InfoHash) != 40 {
			if real, ok := realHashMap[seen[i].SiteName+":"+seen[i].TorrentID]; ok && len(real) == 40 {
				seen[i].InfoHash = real
			}
		}
		items[i] = activityItem{RSSTorrentSeen: seen[i]}
		if si, ok := siteMap[seen[i].SiteName]; ok && si.BaseURL != "" && seen[i].TorrentID != "" {
			items[i].DetailURL = buildDetailURL(si.BaseURL, si.Framework, seen[i].TorrentID)
		}
	}

	Success(w, map[string]interface{}{
		"items": items,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

func buildDetailURL(baseURL, framework, torrentID string) string {
	switch framework {
	case "tnode":
		return baseURL + "/torrent/info/" + torrentID
	case "unit3d", "rousi":
		return baseURL + "/torrent/" + torrentID
	default:
		return baseURL + "/details.php?id=" + torrentID
	}
}

func (h *DashboardHandler) handleTrends(w http.ResponseWriter, r *http.Request) {
	days := 7
	if v := r.URL.Query().Get("days"); v != "" {
		if n, err := parseInt64(v); err == nil && n > 0 && n <= 30 {
			days = int(n)
		}
	}

	now := time.Now()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -(days - 1))
	dayEnd := dayStart.AddDate(0, 0, days)

	type dayCount struct {
		Date  string `json:"date"`
		Count int64  `json:"count"`
	}

	var eventCounts []dayCount
	if err := h.db.Model(&model.TorrentEvent{}).
		Select("DATE(created_at) as date, COUNT(*) as count").
		Where("created_at >= ? AND created_at < ?", dayStart, dayEnd).
		Group("DATE(created_at)").Find(&eventCounts).Error; err != nil {
		h.logger.Warn("query event trend counts failed", zap.Error(err))
	}

	var seenCounts []dayCount
	if err := h.db.Model(&model.RSSTorrentSeen{}).
		Select("DATE(created_at) as date, COUNT(*) as count").
		Where("created_at >= ? AND created_at < ?", dayStart, dayEnd).
		Group("DATE(created_at)").Find(&seenCounts).Error; err != nil {
		h.logger.Warn("query seen trend counts failed", zap.Error(err))
	}

	var publishCounts []dayCount
	if err := h.db.Model(&model.PublishCandidate{}).
		Select("DATE(updated_at) as date, COUNT(*) as count").
		Where("updated_at >= ? AND updated_at < ? AND publish_status = ?", dayStart, dayEnd, "done").
		Group("DATE(updated_at)").Find(&publishCounts).Error; err != nil {
		h.logger.Warn("query publish trend counts failed", zap.Error(err))
	}

	var reseedCounts []dayCount
	if err := h.db.Model(&model.ReseedMatch{}).
		Select("DATE(injected_at) as date, COUNT(*) as count").
		Where("injected_at >= ? AND injected_at < ?", dayStart, dayEnd).
		Group("DATE(injected_at)").Find(&reseedCounts).Error; err != nil {
		h.logger.Warn("query reseed trend counts failed", zap.Error(err))
	}

	eventMap := make(map[string]int64, len(eventCounts))
	for _, e := range eventCounts {
		eventMap[e.Date] = e.Count
	}
	seenMap := make(map[string]int64, len(seenCounts))
	for _, s := range seenCounts {
		seenMap[s.Date] = s.Count
	}
	publishMap := make(map[string]int64, len(publishCounts))
	for _, p := range publishCounts {
		publishMap[p.Date] = p.Count
	}
	reseedMap := make(map[string]int64, len(reseedCounts))
	for _, r := range reseedCounts {
		reseedMap[r.Date] = r.Count
	}

	points := make([]map[string]interface{}, 0, days)
	for i := days - 1; i >= 0; i-- {
		d := now.AddDate(0, 0, -i)
		ds := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, d.Location()).Format("2006-01-02")
		points = append(points, map[string]interface{}{
			"date":    ds,
			"events":  eventMap[ds],
			"rss":     seenMap[ds],
			"publish": publishMap[ds],
			"reseed":  reseedMap[ds],
		})
	}

	Success(w, map[string]interface{}{
		"trends": points,
		"days":   days,
	})
}

func (h *DashboardHandler) handleTorrentEvents(w http.ResponseWriter, r *http.Request, trimmed string) {
	if trimmed == "/api/v1/torrent-events" || trimmed == "/api/v1/torrent-events/" {
		if r.Method != http.MethodGet {
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
			return
		}

		var events []model.TorrentEvent
		var total int64

		q := h.db.Model(&model.TorrentEvent{})
		if site := r.URL.Query().Get("site"); site != "" {
			q = q.Where("site_name = ?", site)
		}
		q.Count(&total)
		if err := q.Error; err != nil {
			Error(w, http.StatusInternalServerError, 50000, "查询种子事件总数失败")
			return
		}
		if err := q.Session(&gorm.Session{}).Order("created_at DESC").Limit(100).Find(&events).Error; err != nil {
			Error(w, http.StatusInternalServerError, 50000, "查询种子事件失败")
			return
		}

		Success(w, map[string]interface{}{
			"items": events,
			"total": total,
		})
		return
	}

	remaining := strings.TrimPrefix(trimmed, "/api/v1/torrent-events/")
	remaining = strings.TrimRight(remaining, "/")

	if remaining == "cleanup" {
		if r.Method != http.MethodPost {
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
			return
		}
		before := time.Now().AddDate(0, 0, -30)
		result := h.db.Where("created_at < ?", before).Delete(&model.TorrentEvent{})
		if result.Error != nil {
			Error(w, http.StatusInternalServerError, 50000, "清理事件失败")
			return
		}
		Success(w, map[string]interface{}{
			"deleted": result.RowsAffected,
		})
		return
	}

	if r.Method != http.MethodGet {
		Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		return
	}

	var event model.TorrentEvent
	if err := h.db.First(&event, remaining).Error; err != nil {
		Error(w, http.StatusNotFound, 40400, "事件不存在")
		return
	}
	Success(w, event)
}

func parseInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}
